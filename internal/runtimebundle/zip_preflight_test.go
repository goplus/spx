/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package runtimebundle

import (
	archivezip "archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
)

func readPreflightZip(t *testing.T, entries ...testZipEntry) []byte {
	t.Helper()
	data, err := os.ReadFile(writeTestZip(t, entries...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func resolvedPreflightLimits(t *testing.T, update func(*Limits)) Limits {
	t.Helper()
	limits := Limits{}
	if update != nil {
		update(&limits)
	}
	resolved, err := limits.withDefaults()
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestPreflightRejectsForgedSmallEntryCount(t *testing.T) {
	data := readPreflightZip(t,
		testZipEntry{name: "one", data: "1"},
		testZipEntry{name: "two", data: "2"},
		testZipEntry{name: "three", data: "3"},
	)
	end := findPreflightZipEnd(t, data)
	binary.LittleEndian.PutUint16(data[end+8:end+10], 1)
	binary.LittleEndian.PutUint16(data[end+10:end+12], 1)
	limits := resolvedPreflightLimits(t, func(limits *Limits) { limits.MaxEntries = 2 })

	err := preflightZipArchive(bytes.NewReader(data), int64(len(data)), limits)
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("preflightZipArchive error = %v, want ErrArchiveLimit", err)
	}
	if _, err := VerifyZipReader(bytes.NewReader(data), int64(len(data)), VerifyOptions{Limits: limits}); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("VerifyZipReader error = %v, want ErrArchiveLimit", err)
	}
}

func TestPreflightAcceptsZip64Directory(t *testing.T) {
	data := makePreflightZip64Offsets(t, readPreflightZip(t, testZipEntry{name: "one", data: "1"}))
	limits := resolvedPreflightLimits(t, nil)
	reader := bytes.NewReader(data)
	if err := preflightZipArchive(reader, int64(len(data)), limits); err != nil {
		t.Fatalf("preflightZipArchive error = %v", err)
	}
	if _, err := archivezip.NewReader(reader, int64(len(data))); err != nil {
		t.Fatalf("archive/zip rejected ZIP64 fixture: %v", err)
	}
}

func TestPreflightAcceptsZip64DirectorySizeFFFFProbe(t *testing.T) {
	data := makePreflightZip64Offsets(t, readPreflightZip(t, testZipEntry{name: "one", data: "1"}))
	eocd := findPreflightZipEnd(t, data)
	locator := eocd - zipDirectory64LocLen
	recordOffset := int(binary.LittleEndian.Uint64(data[locator+8 : locator+16]))
	records := binary.LittleEndian.Uint64(data[recordOffset+32 : recordOffset+40])
	centralOffset := binary.LittleEndian.Uint64(data[recordOffset+48 : recordOffset+56])
	binary.LittleEndian.PutUint16(data[eocd+8:eocd+10], uint16(records))
	binary.LittleEndian.PutUint16(data[eocd+10:eocd+12], uint16(records))
	binary.LittleEndian.PutUint32(data[eocd+12:eocd+16], math.MaxUint16)
	binary.LittleEndian.PutUint32(data[eocd+16:eocd+20], uint32(centralOffset))

	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data)), resolvedPreflightLimits(t, nil)); err != nil {
		t.Fatalf("ZIP64 directory-size probe rejected: %v", err)
	}
	if _, err := archivezip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("archive/zip rejected ZIP64 directory-size probe: %v", err)
	}
}

func TestPreflightAcceptsClassicDirectorySizeFFFF(t *testing.T) {
	var output bytes.Buffer
	writer := archivezip.NewWriter(&output)
	header := &archivezip.FileHeader{
		Name:    "x",
		Method:  archivezip.Store,
		Comment: strings.Repeat("c", math.MaxUint16-zipDirectoryHeaderLen-len("x")),
	}
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data := output.Bytes()
	end := findPreflightZipEnd(t, data)
	if got := binary.LittleEndian.Uint32(data[end+12 : end+16]); got != math.MaxUint16 {
		t.Fatalf("central directory size = %d, want %d", got, uint32(math.MaxUint16))
	}

	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data)), resolvedPreflightLimits(t, nil)); err != nil {
		t.Fatalf("classic directory size 0xffff rejected: %v", err)
	}
	if _, err := archivezip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("archive/zip rejected classic directory size 0xffff: %v", err)
	}
}

func TestPreflightAcceptsSelfExtractingArchives(t *testing.T) {
	base := readPreflightZip(t, testZipEntry{name: "one", data: "1"})
	prefix := []byte("#!/bin/sh\necho prefix\n")
	classic := append(append([]byte(nil), prefix...), base...)
	if err := preflightZipArchive(bytes.NewReader(classic), int64(len(classic)), resolvedPreflightLimits(t, nil)); err != nil {
		t.Fatalf("self-extracting ZIP rejected: %v", err)
	}
	if _, err := archivezip.NewReader(bytes.NewReader(classic), int64(len(classic))); err != nil {
		t.Fatalf("archive/zip rejected self-extracting ZIP: %v", err)
	}

	zip64 := makePreflightZip64Offsets(t, base)
	zip64 = append(append([]byte(nil), prefix...), zip64...)
	eocd := findPreflightZipEnd(t, zip64)
	locator := eocd - zipDirectory64LocLen
	offset := binary.LittleEndian.Uint64(zip64[locator+8 : locator+16])
	binary.LittleEndian.PutUint64(zip64[locator+8:locator+16], offset+uint64(len(prefix)))
	if err := preflightZipArchive(bytes.NewReader(zip64), int64(len(zip64)), resolvedPreflightLimits(t, nil)); err != nil {
		t.Fatalf("self-extracting ZIP64 rejected: %v", err)
	}
	if _, err := archivezip.NewReader(bytes.NewReader(zip64), int64(len(zip64))); err != nil {
		t.Fatalf("archive/zip rejected self-extracting ZIP64: %v", err)
	}
}

func TestPreflightMatchesArchiveZipBaseOffsetProbe(t *testing.T) {
	data := readPreflightZip(t,
		testZipEntry{name: "one", data: "1"},
		testZipEntry{name: "two", data: "2"},
		testZipEntry{name: "three", data: "3"},
	)
	end := findPreflightZipEnd(t, data)
	centralOffset := int(binary.LittleEndian.Uint32(data[end+16 : end+20]))
	centralSize := int(binary.LittleEndian.Uint32(data[end+12 : end+16]))
	prefix := make([]byte, centralOffset+centralSize+16)
	decoy := prefix[centralOffset : centralOffset+centralSize]
	binary.LittleEndian.PutUint32(decoy[0:4], zipDirectoryHeaderSignature)
	binary.LittleEndian.PutUint32(decoy[20:24], math.MaxUint32)
	binary.LittleEndian.PutUint16(decoy[32:34], uint16(centralSize-zipDirectoryHeaderLen))
	data = append(prefix, data...)
	end = findPreflightZipEnd(t, data)
	binary.LittleEndian.PutUint16(data[end+8:end+10], 1)
	binary.LittleEndian.PutUint16(data[end+10:end+12], 1)
	limits := resolvedPreflightLimits(t, func(limits *Limits) { limits.MaxEntries = 2 })

	if completeZipDirectoryHeaderAt(bytes.NewReader(data), int64(len(data)), int64(centralOffset)) != 0 {
		t.Fatal("invalid ZIP64 decoy accepted as a complete directory header")
	}
	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data)), limits); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("base-offset decoy error = %v, want ErrArchiveLimit", err)
	}
}

func TestPreflightRequiresEOCDCommentToReachEOF(t *testing.T) {
	data := readPreflightZip(t, testZipEntry{name: "one", data: "1"})
	data = append(data, 0x7f)
	if _, err := archivezip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("archive/zip rejected trailing-data fixture: %v", err)
	}
	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data)), resolvedPreflightLimits(t, nil)); !errors.Is(err, archivezip.ErrFormat) {
		t.Fatalf("trailing bytes error = %v, want archive/zip format error", err)
	}
}

func findPreflightZipEnd(t *testing.T, data []byte) int {
	t.Helper()
	offset := bytes.LastIndex(data, []byte{'P', 'K', 0x05, 0x06})
	if offset < 0 || len(data)-offset < zipDirectoryEndLen {
		t.Fatal("ZIP end record not found")
	}
	return offset
}

func makePreflightZip64Offsets(t *testing.T, data []byte) []byte {
	t.Helper()
	end := findPreflightZipEnd(t, data)
	records := binary.LittleEndian.Uint16(data[end+10 : end+12])
	centralSize := binary.LittleEndian.Uint32(data[end+12 : end+16])
	centralOffset := binary.LittleEndian.Uint32(data[end+16 : end+20])

	zip64 := make([]byte, zipDirectory64EndLen+zipDirectory64LocLen)
	binary.LittleEndian.PutUint32(zip64[0:4], zipDirectory64EndSignature)
	binary.LittleEndian.PutUint64(zip64[4:12], zipDirectory64EndLen-12)
	binary.LittleEndian.PutUint16(zip64[12:14], 45)
	binary.LittleEndian.PutUint16(zip64[14:16], 45)
	binary.LittleEndian.PutUint64(zip64[24:32], uint64(records))
	binary.LittleEndian.PutUint64(zip64[32:40], uint64(records))
	binary.LittleEndian.PutUint64(zip64[40:48], uint64(centralSize))
	binary.LittleEndian.PutUint64(zip64[48:56], uint64(centralOffset))
	locator := zip64[zipDirectory64EndLen:]
	binary.LittleEndian.PutUint32(locator[0:4], zipDirectory64LocSignature)
	binary.LittleEndian.PutUint64(locator[8:16], uint64(end))
	binary.LittleEndian.PutUint32(locator[16:20], 1)

	result := make([]byte, 0, len(data)+len(zip64))
	result = append(result, data[:end]...)
	result = append(result, zip64...)
	result = append(result, data[end:]...)
	newEnd := end + len(zip64)
	binary.LittleEndian.PutUint16(result[newEnd+8:newEnd+10], math.MaxUint16)
	binary.LittleEndian.PutUint16(result[newEnd+10:newEnd+12], math.MaxUint16)
	binary.LittleEndian.PutUint32(result[newEnd+12:newEnd+16], math.MaxUint32)
	binary.LittleEndian.PutUint32(result[newEnd+16:newEnd+20], math.MaxUint32)
	return result
}
