//go:build !js

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

package zip

import (
	archivezip "archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type limitZipEntry struct {
	name   string
	data   string
	method uint16
}

func useZipTestLimits(t *testing.T, update func(*resolvedZipLimits)) {
	t.Helper()
	previous := zipTestLimits.Load()
	limits := defaultZipLimits()
	update(&limits)
	setZipTestLimits(limits)
	t.Cleanup(func() {
		if previous == nil {
			clearZipTestLimits()
			return
		}
		setZipTestLimits(*previous)
	})
}

func TestOpenRejectsTooManyZipEntries(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store}, limitZipEntry{name: "two", data: "2", method: archivezip.Store})
	useZipTestLimits(t, func(limits *resolvedZipLimits) { limits.maxEntries = 1 })

	_, err := Open(path)
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("Open error = %v, want ErrArchiveLimit", err)
	}
}

func TestOpenRejectsOversizedZipEntry(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "large", data: "12345", method: archivezip.Store})
	useZipTestLimits(t, func(limits *resolvedZipLimits) { limits.maxEntrySize = 4 })

	_, err := Open(path)
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("Open error = %v, want ErrArchiveLimit", err)
	}
	if !strings.Contains(err.Error(), "large") {
		t.Fatalf("Open error = %v, want entry name", err)
	}
}

func TestOpenRejectsExcessiveTotalZipSize(t *testing.T) {
	path := makeLimitZip(t,
		limitZipEntry{name: "one", data: "123", method: archivezip.Store},
		limitZipEntry{name: "two", data: "456", method: archivezip.Store},
	)
	useZipTestLimits(t, func(limits *resolvedZipLimits) { limits.maxTotalSize = 5 })

	_, err := Open(path)
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("Open error = %v, want ErrArchiveLimit", err)
	}
	if !strings.Contains(err.Error(), "total") {
		t.Fatalf("Open error = %v, want total-size detail", err)
	}
}

func TestOpenRejectsExcessiveZipCompressionRatio(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{
		name:   "repetitive",
		data:   strings.Repeat("a", 4096),
		method: archivezip.Deflate,
	})
	useZipTestLimits(t, func(limits *resolvedZipLimits) { limits.maxCompressionRatio = 2 })

	_, err := Open(path)
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("Open error = %v, want ErrArchiveLimit", err)
	}
	if !strings.Contains(err.Error(), "compression ratio") {
		t.Fatalf("Open error = %v, want compression-ratio detail", err)
	}
}

func TestOpenRejectsNonEmptyDirectoryDeclaration(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "dir/", data: "", method: archivezip.Store})
	reader, err := archivezip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	reader.File[0].UncompressedSize64 = 1
	// The changed declaration models a malformed central-directory entry.
	if err := validateZipReader(&reader.Reader); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("validateZipReader error = %v, want ErrArchiveLimit", err)
	}
}

func TestOpenAndReadZipAtLimits(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "exact", data: "1234", method: archivezip.Store})
	useZipTestLimits(t, func(limits *resolvedZipLimits) {
		limits.maxEntrySize = 4
		limits.maxTotalSize = 4
		limits.maxCompressionRatio = 1
	})

	dir, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	file, err := dir.Open("exact")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("read exact entry: data=%q err=%v close=%v", data, err, closeErr)
	}
	if string(data) != "1234" {
		t.Fatalf("entry = %q, want 1234", data)
	}
}

func TestOpenReadRejectsEntryThatGrowsPastLimit(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "grow", data: "123456", method: archivezip.Store})
	reader, err := archivezip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	// Mutating the exported header simulates an archive whose declaration was
	// tampered with after parsing. The wrapper must still fail closed.
	reader.File[0].UncompressedSize64 = 4
	useZipTestLimits(t, func(limits *resolvedZipLimits) { limits.maxEntrySize = 4 })
	zipFS := &FS{Reader: &reader.Reader}
	entry, err := zipFS.Open("grow")
	if err != nil {
		t.Fatal(err)
	}
	_, readErr := io.ReadAll(entry)
	_ = entry.Close()
	if readErr == nil {
		t.Fatal("reading tampered entry succeeded, want error")
	}
}

func TestPreflightRejectsForgedSmallEntryCount(t *testing.T) {
	data := makeLimitZipBytes(t,
		limitZipEntry{name: "one", data: "1", method: archivezip.Store},
		limitZipEntry{name: "two", data: "2", method: archivezip.Store},
		limitZipEntry{name: "three", data: "3", method: archivezip.Store},
	)
	end := findLimitZipEnd(t, data)
	binary.LittleEndian.PutUint16(data[end+8:end+10], 1)
	binary.LittleEndian.PutUint16(data[end+10:end+12], 1)
	useZipTestLimits(t, func(limits *resolvedZipLimits) { limits.maxEntries = 2 })

	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("preflightZipArchive error = %v, want ErrArchiveLimit", err)
	}
}

func TestOpenRejectsOversizedArchiveBeforeParsing(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store})
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	useZipTestLimits(t, func(limits *resolvedZipLimits) { limits.maxArchiveBytes = info.Size() - 1 })

	_, err = Open(path)
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("Open error = %v, want ErrArchiveLimit", err)
	}
}

func TestPreflightRejectsOversizedCentralDirectory(t *testing.T) {
	data := makeLimitZipBytes(t, limitZipEntry{name: "metadata", data: "1", method: archivezip.Store})
	useZipTestLimits(t, func(limits *resolvedZipLimits) { limits.maxCentralDirBytes = zipDirectoryHeaderLen - 1 })

	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("preflightZipArchive error = %v, want ErrArchiveLimit", err)
	}
}

func TestPreflightAcceptsZip64OffsetsWithSmallEntryCount(t *testing.T) {
	data := makeLimitZipBytes(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store})
	data = makeLimitZip64Offsets(t, data)
	reader := bytes.NewReader(data)
	if err := preflightZipArchive(reader, int64(len(data))); err != nil {
		t.Fatalf("preflightZipArchive error = %v", err)
	}
	if _, err := archivezip.NewReader(reader, int64(len(data))); err != nil {
		t.Fatalf("archive/zip rejected ZIP64 fixture: %v", err)
	}
}

func TestPreflightAcceptsZip64DirectorySizeSentinel(t *testing.T) {
	data := makeLimitZip64Offsets(t, makeLimitZipBytes(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store}))
	eocd := findLimitZipEnd(t, data)
	locator := eocd - zipDirectory64LocLen
	recordOffset := int(binary.LittleEndian.Uint64(data[locator+8 : locator+16]))
	records := binary.LittleEndian.Uint64(data[recordOffset+32 : recordOffset+40])
	centralOffset := binary.LittleEndian.Uint64(data[recordOffset+48 : recordOffset+56])
	binary.LittleEndian.PutUint16(data[eocd+8:eocd+10], uint16(records))
	binary.LittleEndian.PutUint16(data[eocd+10:eocd+12], uint16(records))
	binary.LittleEndian.PutUint32(data[eocd+12:eocd+16], math.MaxUint16)
	binary.LittleEndian.PutUint32(data[eocd+16:eocd+20], uint32(centralOffset))

	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("ZIP64 directory-size sentinel rejected: %v", err)
	}
	if _, err := archivezip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("archive/zip rejected ZIP64 directory-size sentinel: %v", err)
	}
}

func TestReadZipArchiveBodyRejectsOversize(t *testing.T) {
	useZipTestLimits(t, func(limits *resolvedZipLimits) { limits.maxArchiveBytes = 4 })

	if _, err := readZipArchiveBody(strings.NewReader("12345"), -1); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("streamed body error = %v, want ErrArchiveLimit", err)
	}
	if _, err := readZipArchiveBody(strings.NewReader(""), 5); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("declared body error = %v, want ErrArchiveLimit", err)
	}
}

func TestPreflightRequiresEOCDCommentToReachEOF(t *testing.T) {
	data := makeLimitZipBytesWithComment(t, "zip comment", limitZipEntry{name: "one", data: "1", method: archivezip.Store})
	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("valid comment archive rejected: %v", err)
	}
	trailing := append(append([]byte(nil), data...), 0x7f)
	if err := preflightZipArchive(bytes.NewReader(trailing), int64(len(trailing))); !errors.Is(err, archivezip.ErrFormat) {
		t.Fatalf("trailing bytes error = %v, want archive/zip format error", err)
	}
}

func TestPreflightRejectsLaterEOCDWithTrailingData(t *testing.T) {
	data := makeLimitZipBytes(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store})
	firstEnd := findLimitZipEnd(t, data)
	trailing := make([]byte, zipDirectoryEndLen+1)
	binary.LittleEndian.PutUint32(trailing[0:4], zipDirectoryEndSignature)
	data = append(data, trailing...)
	binary.LittleEndian.PutUint16(data[firstEnd+20:firstEnd+22], uint16(len(trailing)))

	if _, err := archivezip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("archive/zip did not select the later EOCD fixture: %v", err)
	}
	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data))); !errors.Is(err, archivezip.ErrFormat) {
		t.Fatalf("later EOCD with trailing data error = %v, want archive/zip format error", err)
	}
}

func TestPreflightAcceptsEmptyZip(t *testing.T) {
	data := makeLimitZipBytes(t)
	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("empty ZIP rejected: %v", err)
	}
	if _, err := archivezip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("archive/zip rejected empty ZIP: %v", err)
	}
}

func TestPreflightAcceptsSelfExtractingPrefix(t *testing.T) {
	data := makeLimitZipBytes(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store})
	prefix := []byte("#!/bin/sh\necho prefix\n")
	data = append(append([]byte(nil), prefix...), data...)
	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("self-extracting ZIP rejected: %v", err)
	}
	reader, err := archivezip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("archive/zip rejected self-extracting ZIP: %v", err)
	}
	if len(reader.File) != 1 || reader.File[0].Name != "one" {
		t.Fatalf("self-extracting ZIP files = %#v", reader.File)
	}
}

func TestPreflightRejectsZip64RelativeLocatorWithPrefix(t *testing.T) {
	data := makeLimitZip64Offsets(t, makeLimitZipBytes(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store}))
	prefix := []byte("self extracting prefix\x00")
	data = append(append([]byte(nil), prefix...), data...)
	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data))); !errors.Is(err, archivezip.ErrFormat) {
		t.Fatalf("relative ZIP64 locator error = %v, want archive/zip format error", err)
	}
	if _, err := archivezip.NewReader(bytes.NewReader(data), int64(len(data))); !errors.Is(err, archivezip.ErrFormat) {
		t.Fatalf("archive/zip relative ZIP64 locator error = %v, want format error", err)
	}
}

func TestPreflightAcceptsZip64AbsoluteLocatorWithPrefix(t *testing.T) {
	data := makeLimitZip64Offsets(t, makeLimitZipBytes(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store}))
	prefix := []byte("self extracting prefix\x00")
	data = append(append([]byte(nil), prefix...), data...)
	eocd := findLimitZipEnd(t, data)
	locator := eocd - zipDirectory64LocLen
	offset := binary.LittleEndian.Uint64(data[locator+8 : locator+16])
	binary.LittleEndian.PutUint64(data[locator+8:locator+16], offset+uint64(len(prefix)))
	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("self-extracting ZIP64 with absolute locator rejected: %v", err)
	}
	if _, err := archivezip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("archive/zip rejected self-extracting ZIP64: %v", err)
	}
}

func TestPreflightRejectsGapBeforeZip64Locator(t *testing.T) {
	data := makeLimitZip64Offsets(t, makeLimitZipBytes(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store}))
	eocd := findLimitZipEnd(t, data)
	locator := eocd - zipDirectory64LocLen
	withGap := make([]byte, 0, len(data)+1)
	withGap = append(withGap, data[:locator]...)
	withGap = append(withGap, 0)
	withGap = append(withGap, data[locator:]...)

	if err := preflightZipArchive(bytes.NewReader(withGap), int64(len(withGap))); !errors.Is(err, archivezip.ErrFormat) {
		t.Fatalf("ZIP64 locator gap error = %v, want archive/zip format error", err)
	}
}

func TestPreflightMatchesArchiveZipBaseOffsetProbe(t *testing.T) {
	data := makeLimitZipBytes(t,
		limitZipEntry{name: "one", data: "1", method: archivezip.Store},
		limitZipEntry{name: "two", data: "2", method: archivezip.Store},
		limitZipEntry{name: "three", data: "3", method: archivezip.Store},
	)
	end := findLimitZipEnd(t, data)
	centralOffset := int(binary.LittleEndian.Uint32(data[end+16 : end+20]))
	centralSize := int(binary.LittleEndian.Uint32(data[end+12 : end+16]))
	prefix := make([]byte, centralOffset+centralSize+16)
	decoy := prefix[centralOffset : centralOffset+centralSize]
	binary.LittleEndian.PutUint32(decoy[0:4], zipDirectoryHeaderSignature)
	binary.LittleEndian.PutUint32(decoy[20:24], math.MaxUint32)
	binary.LittleEndian.PutUint16(decoy[32:34], uint16(centralSize-zipDirectoryHeaderLen))
	data = append(prefix, data...)
	end = findLimitZipEnd(t, data)
	binary.LittleEndian.PutUint16(data[end+8:end+10], 1)
	binary.LittleEndian.PutUint16(data[end+10:end+12], 1)
	useZipTestLimits(t, func(limits *resolvedZipLimits) { limits.maxEntries = 2 })

	if completeZipDirectoryHeaderAt(bytes.NewReader(data), int64(len(data)), int64(centralOffset)) != 0 {
		t.Fatal("invalid ZIP64 decoy accepted as a complete directory header")
	}
	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("base-offset decoy error = %v, want ErrArchiveLimit", err)
	}
}

func TestPreflightRejectsHeaderPastDeclaredDirectorySize(t *testing.T) {
	data := makeLimitZipBytes(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store})
	end := findLimitZipEnd(t, data)
	centralOffset := int(binary.LittleEndian.Uint32(data[end+16 : end+20]))
	central := append([]byte(nil), data[centralOffset:end]...)
	withExtraHeader := make([]byte, 0, len(data)+len(central))
	withExtraHeader = append(withExtraHeader, data[:end]...)
	withExtraHeader = append(withExtraHeader, central...)
	withExtraHeader = append(withExtraHeader, data[end:]...)

	if err := preflightZipArchive(bytes.NewReader(withExtraHeader), int64(len(withExtraHeader))); !errors.Is(err, archivezip.ErrFormat) {
		t.Fatalf("extra directory header error = %v, want archive/zip format error", err)
	}
	if _, err := archivezip.NewReader(bytes.NewReader(withExtraHeader), int64(len(withExtraHeader))); err == nil {
		t.Fatal("archive/zip accepted a directory header past the declared size")
	}
}

func TestScanCentralDirectoryHonorsDeclaredEnd(t *testing.T) {
	data := makeLimitZipBytes(t,
		limitZipEntry{name: "one", data: "1", method: archivezip.Store},
		limitZipEntry{name: "two", data: "2", method: archivezip.Store},
	)
	end := findLimitZipEnd(t, data)
	centralOffset := int(binary.LittleEndian.Uint32(data[end+16 : end+20]))
	centralSize := int(binary.LittleEndian.Uint32(data[end+12 : end+16]))
	central := data[centralOffset : centralOffset+centralSize]
	firstLen := completeZipDirectoryHeaderAt(bytes.NewReader(central), int64(len(central)), 0)
	if firstLen == 0 {
		t.Fatal("first central-directory header missing")
	}
	if err := scanZipCentralDirectory(bytes.NewReader(central), int64(len(central)), 0, firstLen, 1, defaultZipLimits()); err != nil {
		t.Fatalf("bounded central-directory scan failed: %v", err)
	}
}

func TestPreflightRejectsCentralDirectorySizePastEOCD(t *testing.T) {
	data := makeLimitZipBytes(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store})
	end := findLimitZipEnd(t, data)
	size := binary.LittleEndian.Uint32(data[end+12 : end+16])
	binary.LittleEndian.PutUint32(data[end+12:end+16], size+uint32(zipDirectoryHeaderLen))
	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data))); !errors.Is(err, archivezip.ErrFormat) {
		t.Fatalf("oversized central directory error = %v, want archive/zip format error", err)
	}
}

func TestFSNilCloseIsSafe(t *testing.T) {
	var nilFS *FS
	if err := nilFS.Close(); err != nil {
		t.Fatalf("nil FS Close error = %v", err)
	}
	if err := (&FS{}).Close(); err != nil {
		t.Fatalf("empty FS Close error = %v", err)
	}
}

func TestZipLimitHookConcurrent(t *testing.T) {
	previous := zipTestLimits.Load()
	t.Cleanup(func() {
		if previous == nil {
			clearZipTestLimits()
		} else {
			setZipTestLimits(*previous)
		}
	})
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(2)
		go func(i int) {
			defer group.Done()
			limits := defaultZipLimits()
			limits.maxEntries = i + 1
			setZipTestLimits(limits)
		}(i)
		go func() {
			defer group.Done()
			_ = currentZipLimits()
		}()
	}
	group.Wait()
}

func TestZipLimitedReaderDoesNotExposeSentinelByte(t *testing.T) {
	reader := &zipLimitedReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader("12345")),
		remaining:  5,
		max:        4,
		name:       "sentinel",
	}
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("ReadAll error = %v, want ErrArchiveLimit", err)
	}
	if string(data) != "1234" {
		t.Fatalf("ReadAll data = %q, want 1234", data)
	}
}

func makeLimitZip(t *testing.T, entries ...limitZipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := archivezip.NewWriter(file)
	for _, item := range entries {
		header := &archivezip.FileHeader{Name: item.name, Method: item.method}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, item.data); err != nil {
			file.Close()
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func makeLimitZipBytes(t *testing.T, entries ...limitZipEntry) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := archivezip.NewWriter(&output)
	for _, item := range entries {
		header := &archivezip.FileHeader{Name: item.name, Method: item.method}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, item.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func makeLimitZipBytesWithComment(t *testing.T, comment string, entries ...limitZipEntry) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := archivezip.NewWriter(&output)
	for _, item := range entries {
		header := &archivezip.FileHeader{Name: item.name, Method: item.method}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, item.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.SetComment(comment); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func findLimitZipEnd(t *testing.T, data []byte) int {
	t.Helper()
	signature := []byte{'P', 'K', 0x05, 0x06}
	offset := bytes.LastIndex(data, signature)
	if offset < 0 || len(data)-offset < zipDirectoryEndLen {
		t.Fatal("ZIP end record not found")
	}
	return offset
}

func makeLimitZip64Offsets(t *testing.T, data []byte) []byte {
	t.Helper()
	end := findLimitZipEnd(t, data)
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
