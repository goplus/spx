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

package zippreflight

import (
	archivezip "archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"testing"
)

type testEntry struct {
	name    string
	data    string
	method  uint16
	comment string
}

type failingReaderAt struct {
	io.ReaderAt
	offset int64
	length int
	err    error
}

func TestCheckRejectsInvalidInput(t *testing.T) {
	valid := makeZip(t, "", testEntry{name: "one", data: "1"})
	var typedNil *bytes.Reader
	tests := []struct {
		name   string
		reader io.ReaderAt
		size   int64
		limits Limits
		format bool
	}{
		{name: "nil reader", size: int64(len(valid)), limits: testLimits(), format: true},
		{name: "typed nil reader", reader: typedNil, size: int64(len(valid)), limits: testLimits(), format: true},
		{name: "negative size", reader: bytes.NewReader(valid), size: -1, limits: testLimits(), format: true},
		{name: "missing end record", reader: bytes.NewReader([]byte("short")), size: 5, limits: testLimits(), format: true},
		{name: "invalid end signature", reader: bytes.NewReader(make([]byte, directoryEndLen)), size: directoryEndLen, limits: testLimits(), format: true},
		{name: "zero archive limit", reader: bytes.NewReader(valid), size: int64(len(valid)), limits: Limits{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Check(test.reader, test.size, test.limits)
			if err == nil {
				t.Fatal("Check succeeded")
			}
			if test.format && !errors.Is(err, archivezip.ErrFormat) {
				t.Fatalf("Check error = %v, want archive/zip format error", err)
			}
		})
	}
}

func TestCheckEnforcesMetadataLimits(t *testing.T) {
	data := makeZip(t, "",
		testEntry{name: "one", data: "1"},
		testEntry{name: "two", data: "2"},
		testEntry{name: "three", data: "3"},
	)
	t.Run("archive bytes", func(t *testing.T) {
		limits := testLimits()
		limits.MaxArchiveBytes = int64(len(data) - 1)
		if err := Check(bytes.NewReader(data), int64(len(data)), limits); !IsLimit(err) {
			t.Fatalf("Check error = %v, want limit error", err)
		}
	})
	t.Run("central directory bytes", func(t *testing.T) {
		limits := testLimits()
		limits.MaxCentralDirectoryBytes = directoryHeaderLen - 1
		if err := Check(bytes.NewReader(data), int64(len(data)), limits); !IsLimit(err) {
			t.Fatalf("Check error = %v, want limit error", err)
		}
	})
	t.Run("declared entries", func(t *testing.T) {
		limits := testLimits()
		limits.MaxEntries = 2
		if err := Check(bytes.NewReader(data), int64(len(data)), limits); !IsLimit(err) {
			t.Fatalf("Check error = %v, want limit error", err)
		}
	})
	t.Run("forged entry count", func(t *testing.T) {
		forged := append([]byte(nil), data...)
		end := findEnd(t, forged)
		binary.LittleEndian.PutUint16(forged[end+8:end+10], 1)
		binary.LittleEndian.PutUint16(forged[end+10:end+12], 1)
		limits := testLimits()
		limits.MaxEntries = 2
		if err := Check(bytes.NewReader(forged), int64(len(forged)), limits); !IsLimit(err) {
			t.Fatalf("Check error = %v, want limit error", err)
		}
	})
}

func TestIsLimitRecognizesWrappedErrors(t *testing.T) {
	data := makeZip(t, "", testEntry{name: "one"})
	limits := testLimits()
	limits.MaxArchiveBytes = int64(len(data) - 1)
	err := Check(bytes.NewReader(data), int64(len(data)), limits)
	wrapped := fmt.Errorf("read project bundle: %w", err)
	if !IsLimit(wrapped) || !strings.Contains(wrapped.Error(), "archive size") {
		t.Fatalf("wrapped error = %v, want identifiable archive-size limit", wrapped)
	}
	for _, err := range []error{nil, archivezip.ErrFormat, io.ErrUnexpectedEOF} {
		if IsLimit(err) {
			t.Errorf("IsLimit(%v) = true", err)
		}
	}
}

func TestCheckHandlesZip64Boundaries(t *testing.T) {
	base := makeZip(t, "", testEntry{name: "one", data: "1"})
	t.Run("standard offsets", func(t *testing.T) {
		assertAccepted(t, makeZip64(t, base))
	})
	t.Run("ZIP64 size marker without space for locator", func(t *testing.T) {
		data := makeZip(t, "")
		binary.LittleEndian.PutUint32(data[12:16], math.MaxUint16)
		assertFormatError(t, data)
	})
	for _, test := range []struct {
		name string
		size uint32
	}{
		{name: "ffff directory size", size: math.MaxUint16},
		{name: "ffffffff directory size", size: math.MaxUint32},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := makeZip64(t, base)
			end := findEnd(t, data)
			locator := end - directory64LocLen
			recordOffset := int(binary.LittleEndian.Uint64(data[locator+8 : locator+16]))
			entries := binary.LittleEndian.Uint64(data[recordOffset+32 : recordOffset+40])
			centralOffset := binary.LittleEndian.Uint64(data[recordOffset+48 : recordOffset+56])
			binary.LittleEndian.PutUint16(data[end+8:end+10], uint16(entries))
			binary.LittleEndian.PutUint16(data[end+10:end+12], uint16(entries))
			binary.LittleEndian.PutUint32(data[end+12:end+16], test.size)
			binary.LittleEndian.PutUint32(data[end+16:end+20], uint32(centralOffset))
			assertFormatError(t, data)
		})
	}
	t.Run("standard probe with ffff directory size", func(t *testing.T) {
		data := makeZip64(t, base)
		end := findEnd(t, data)
		binary.LittleEndian.PutUint32(data[end+12:end+16], math.MaxUint16)
		assertAccepted(t, data)
	})
	t.Run("classic ffff directory size", func(t *testing.T) {
		data := makeZip(t, "", testEntry{
			name:    "x",
			data:    "x",
			comment: strings.Repeat("c", math.MaxUint16-directoryHeaderLen-len("x")),
		})
		end := findEnd(t, data)
		if got := binary.LittleEndian.Uint32(data[end+12 : end+16]); got != math.MaxUint16 {
			t.Fatalf("central directory size = %d, want %d", got, uint32(math.MaxUint16))
		}
		assertAccepted(t, data)
	})
}

func TestCheckRejectsMalformedEndRecords(t *testing.T) {
	base := makeZip(t, "comment", testEntry{name: "one", data: "1"})
	assertAccepted(t, base)

	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{
			name: "trailing data",
			mutate: func(data []byte) []byte {
				return append(data, 0x7f)
			},
		},
		{
			name: "later end record",
			mutate: func(data []byte) []byte {
				end := findEnd(t, data)
				trailing := make([]byte, directoryEndLen+1)
				binary.LittleEndian.PutUint32(trailing[0:4], directoryEndSignature)
				data = append(data, trailing...)
				binary.LittleEndian.PutUint16(data[end+20:end+22], uint16(len(trailing)))
				return data
			},
		},
		{
			name: "multi disk",
			mutate: func(data []byte) []byte {
				end := findEnd(t, data)
				binary.LittleEndian.PutUint16(data[end+4:end+6], 1)
				return data
			},
		},
		{
			name: "inconsistent entries",
			mutate: func(data []byte) []byte {
				end := findEnd(t, data)
				binary.LittleEndian.PutUint16(data[end+8:end+10], 0)
				return data
			},
		},
		{
			name: "directory past end",
			mutate: func(data []byte) []byte {
				end := findEnd(t, data)
				size := binary.LittleEndian.Uint32(data[end+12 : end+16])
				binary.LittleEndian.PutUint32(data[end+12:end+16], size+directoryHeaderLen)
				return data
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := test.mutate(append([]byte(nil), base...))
			assertFormatError(t, data)
		})
	}
}

func TestCheckRejectsEndRecordConfusionInComment(t *testing.T) {
	t.Run("alternate record", func(t *testing.T) {
		comment := "prefix" + string([]byte{'P', 'K', 0x05, 0x06}) + strings.Repeat("\x00", directoryEndLen-4) + "suffix"
		data := makeZip(t, comment, testEntry{name: "one", data: "1"})
		reader, err := archivezip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		if len(reader.File) != 0 {
			t.Fatalf("archive/zip entries = %d, want parser-confusion fixture", len(reader.File))
		}
		assertFormatError(t, data)
	})
	t.Run("truncated alternate record", func(t *testing.T) {
		end := make([]byte, directoryEndLen)
		binary.LittleEndian.PutUint32(end, directoryEndSignature)
		binary.LittleEndian.PutUint16(end[20:], math.MaxUint16)
		data := makeZip(t, "prefix"+string(end)+"suffix", testEntry{name: "one", data: "1"})
		if _, err := archivezip.NewReader(bytes.NewReader(data), int64(len(data))); !errors.Is(err, archivezip.ErrFormat) {
			t.Fatalf("archive/zip error = %v, want format error", err)
		}
		assertFormatError(t, data)
	})
}

func TestCheckHandlesSelfExtractingArchives(t *testing.T) {
	base := makeZip(t, "", testEntry{name: "one", data: "1"})
	prefix := []byte("#!/bin/sh\necho prefix\n")
	t.Run("classic", func(t *testing.T) {
		assertAccepted(t, append(append([]byte(nil), prefix...), base...))
	})
	t.Run("ZIP64 absolute locator", func(t *testing.T) {
		data := append(append([]byte(nil), prefix...), makeZip64(t, base)...)
		end := findEnd(t, data)
		locator := end - directory64LocLen
		offset := binary.LittleEndian.Uint64(data[locator+8 : locator+16])
		binary.LittleEndian.PutUint64(data[locator+8:locator+16], offset+uint64(len(prefix)))
		assertAccepted(t, data)
	})
	t.Run("ZIP64 relative locator", func(t *testing.T) {
		data := append(append([]byte(nil), prefix...), makeZip64(t, base)...)
		assertFormatError(t, data)
	})
}

func TestCheckRejectsZip64RecordConfusion(t *testing.T) {
	base := makeZip64(t, makeZip(t, "", testEntry{name: "one", data: "1"}))
	t.Run("gap before locator", func(t *testing.T) {
		end := findEnd(t, base)
		locator := end - directory64LocLen
		data := make([]byte, 0, len(base)+1)
		data = append(data, base[:locator]...)
		data = append(data, 0)
		data = append(data, base[locator:]...)
		assertFormatError(t, data)
	})
	for _, test := range []struct {
		name   string
		mutate func(record, locator []byte)
	}{
		{"locator offset overflow", func(_, locator []byte) { binary.LittleEndian.PutUint64(locator[8:16], math.MaxUint64) }},
		{"locator points into itself", func(_, locator []byte) {
			binary.LittleEndian.PutUint64(locator[8:16], uint64(len(base)-directoryEndLen-directory64LocLen))
		}},
		{"invalid end signature", func(record, _ []byte) { binary.LittleEndian.PutUint32(record[:4], directoryEndSignature) }},
		{"short end record", func(record, _ []byte) { binary.LittleEndian.PutUint64(record[4:12], directory64EndLen-13) }},
		{"end record size overflow", func(record, _ []byte) { binary.LittleEndian.PutUint64(record[4:12], math.MaxUint64) }},
		{"inconsistent entries", func(record, _ []byte) { binary.LittleEndian.PutUint64(record[24:32], 0) }},
		{"multi disk", func(record, _ []byte) { binary.LittleEndian.PutUint32(record[16:20], 1) }},
		{"central offset overflow", func(record, _ []byte) { binary.LittleEndian.PutUint64(record[48:56], math.MaxUint64) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := append([]byte(nil), base...)
			locator := findEnd(t, data) - directory64LocLen
			record := int(binary.LittleEndian.Uint64(data[locator+8 : locator+16]))
			test.mutate(data[record:record+directory64EndLen], data[locator:locator+directory64LocLen])
			assertFormatError(t, data)
		})
	}
	t.Run("entry limit", func(t *testing.T) {
		data := makeZip64(t, makeZip(t, "", testEntry{name: "one"}, testEntry{name: "two"}))
		limits := testLimits()
		limits.MaxEntries = 1
		if err := Check(bytes.NewReader(data), int64(len(data)), limits); !IsLimit(err) {
			t.Fatalf("Check error = %v, want ZIP64 entry limit", err)
		}
	})
}

func TestCheckRejectsMalformedCentralDirectory(t *testing.T) {
	base := makeZip(t, "", testEntry{name: "one", data: "1"})
	for _, test := range []struct {
		name   string
		mutate func(end, central []byte)
	}{
		{"size exceeds archive", func(end, _ []byte) { binary.LittleEndian.PutUint32(end[12:16], uint32(len(base)+1)) }},
		{"size crosses end record", func(end, _ []byte) { binary.LittleEndian.PutUint32(end[12:16], uint32(len(base))) }},
		{"truncated header", func(end, _ []byte) {
			binary.LittleEndian.PutUint32(end[12:16], directoryHeaderLen-1)
			binary.LittleEndian.PutUint32(end[16:20], uint32(len(base)-directoryEndLen-directoryHeaderLen+1))
		}},
		{"invalid header signature", func(_, central []byte) { binary.LittleEndian.PutUint32(central[:4], directoryEndSignature) }},
		{"filename exceeds directory", func(_, central []byte) { binary.LittleEndian.PutUint16(central[28:30], math.MaxUint16) }},
		{"extra exceeds directory", func(_, central []byte) { binary.LittleEndian.PutUint16(central[30:32], math.MaxUint16) }},
		{"comment exceeds directory", func(_, central []byte) { binary.LittleEndian.PutUint16(central[32:34], math.MaxUint16) }},
		{"declared entry missing", func(end, _ []byte) {
			binary.LittleEndian.PutUint16(end[8:10], 2)
			binary.LittleEndian.PutUint16(end[10:12], 2)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := append([]byte(nil), base...)
			end := findEnd(t, data)
			central := int(binary.LittleEndian.Uint32(data[end+16 : end+20]))
			test.mutate(data[end:], data[central:end])
			assertFormatError(t, data)
		})
	}
}

func TestCheckRejectsUnreadableMetadata(t *testing.T) {
	data := makeZip64(t, makeZip(t, "", testEntry{name: "one"}))
	locator := findEnd(t, data) - directory64LocLen
	record := int(binary.LittleEndian.Uint64(data[locator+8 : locator+16]))
	central := int(binary.LittleEndian.Uint64(data[record+48 : record+56]))
	readErr := errors.New("metadata storage unavailable")
	for _, test := range []struct {
		name           string
		offset, length int
		want           error
	}{
		{"end record search", 0, len(data), readErr},
		{"ZIP64 locator", locator, directory64LocLen, readErr},
		{"ZIP64 end record", record, directory64EndLen, archivezip.ErrFormat},
		{"central header", central, directoryHeaderLen, archivezip.ErrFormat},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := failingReaderAt{bytes.NewReader(data), int64(test.offset), test.length, readErr}
			if err := Check(reader, int64(len(data)), testLimits()); !errors.Is(err, test.want) {
				t.Fatalf("Check error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestCheckRejectsAmbiguousBaseOffset(t *testing.T) {
	data := makeZip(t, "",
		testEntry{name: "one", data: "1"},
		testEntry{name: "two", data: "2"},
	)
	end := findEnd(t, data)
	centralOffset := int(binary.LittleEndian.Uint32(data[end+16 : end+20]))
	centralSize := int(binary.LittleEndian.Uint32(data[end+12 : end+16]))
	prefix := make([]byte, centralOffset+centralSize+16)
	binary.LittleEndian.PutUint32(prefix[centralOffset:centralOffset+4], directoryHeaderSignature)
	assertFormatError(t, append(prefix, data...))
}

func TestCheckRejectsHeaderPastDeclaredDirectory(t *testing.T) {
	data := makeZip(t, "", testEntry{name: "one", data: "1"})
	end := findEnd(t, data)
	centralOffset := int(binary.LittleEndian.Uint32(data[end+16 : end+20]))
	central := append([]byte(nil), data[centralOffset:end]...)
	withExtra := make([]byte, 0, len(data)+len(central))
	withExtra = append(withExtra, data[:end]...)
	withExtra = append(withExtra, central...)
	withExtra = append(withExtra, data[end:]...)
	assertFormatError(t, withExtra)
}

func TestScanCentralDirectoryHonorsDeclaredEnd(t *testing.T) {
	data := makeZip(t, "", testEntry{name: "one"}, testEntry{name: "two"})
	end := findEnd(t, data)
	centralOffset := int(binary.LittleEndian.Uint32(data[end+16 : end+20]))
	centralSize := int(binary.LittleEndian.Uint32(data[end+12 : end+16]))
	central := data[centralOffset : centralOffset+centralSize]
	firstLen := directoryEntryLen(bytes.NewReader(central), int64(len(central)), 0, int64(len(central)))
	if firstLen == 0 {
		t.Fatal("first central-directory header missing")
	}
	for _, test := range []struct {
		name       string
		start, end int64
		maxBytes   int64
		format     bool
	}{
		{"first entry only", 0, firstLen, firstLen, false},
		{"metadata budget exhausted", 0, firstLen, firstLen - 1, false},
		{"negative start", -1, firstLen, firstLen, true},
		{"reversed bounds", firstLen, 0, firstLen, true},
		{"end beyond archive", 0, int64(len(central)) + 1, firstLen, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			limits := testLimits()
			limits.MaxCentralDirectoryBytes = test.maxBytes
			err := scanCentralDirectory(bytes.NewReader(central), int64(len(central)), test.start, test.end, 1, limits)
			switch {
			case test.format:
				if !errors.Is(err, archivezip.ErrFormat) {
					t.Fatalf("scanCentralDirectory = %v, want format error", err)
				}
			case test.maxBytes < firstLen:
				if !IsLimit(err) {
					t.Fatalf("scanCentralDirectory = %v, want metadata limit", err)
				}
			default:
				if err != nil {
					t.Fatalf("scanCentralDirectory returned error: %v", err)
				}
			}
		})
	}
}

func TestCheckAcceptsEmptyZip(t *testing.T) {
	assertAccepted(t, makeZip(t, ""))
}

func (r failingReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	if offset == r.offset && len(p) == r.length {
		return 0, r.err
	}
	return r.ReaderAt.ReadAt(p, offset)
}

func testLimits() Limits {
	return Limits{
		MaxArchiveBytes:          2 << 20,
		MaxCentralDirectoryBytes: 1 << 20,
		MaxEntries:               100,
	}
}

func assertAccepted(t *testing.T, data []byte) {
	t.Helper()
	reader := bytes.NewReader(data)
	if err := Check(reader, int64(len(data)), testLimits()); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if _, err := archivezip.NewReader(reader, int64(len(data))); err != nil {
		t.Fatalf("archive/zip rejected accepted fixture: %v", err)
	}
}

func assertFormatError(t *testing.T, data []byte) {
	t.Helper()
	err := Check(bytes.NewReader(data), int64(len(data)), testLimits())
	if !errors.Is(err, archivezip.ErrFormat) {
		t.Fatalf("Check error = %v, want archive/zip format error", err)
	}
}

func makeZip(t *testing.T, comment string, entries ...testEntry) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := archivezip.NewWriter(&output)
	for _, item := range entries {
		header := &archivezip.FileHeader{Name: item.name, Method: item.method, Comment: item.comment}
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

func findEnd(t *testing.T, data []byte) int {
	t.Helper()
	offset := bytes.LastIndex(data, []byte{'P', 'K', 0x05, 0x06})
	if offset < 0 || len(data)-offset < directoryEndLen {
		t.Fatal("ZIP end record not found")
	}
	return offset
}

func makeZip64(t *testing.T, data []byte) []byte {
	t.Helper()
	end := findEnd(t, data)
	entries := binary.LittleEndian.Uint16(data[end+10 : end+12])
	centralSize := binary.LittleEndian.Uint32(data[end+12 : end+16])
	centralOffset := binary.LittleEndian.Uint32(data[end+16 : end+20])

	zip64 := make([]byte, directory64EndLen+directory64LocLen)
	binary.LittleEndian.PutUint32(zip64[0:4], directory64EndSignature)
	binary.LittleEndian.PutUint64(zip64[4:12], directory64EndLen-12)
	binary.LittleEndian.PutUint16(zip64[12:14], 45)
	binary.LittleEndian.PutUint16(zip64[14:16], 45)
	binary.LittleEndian.PutUint64(zip64[24:32], uint64(entries))
	binary.LittleEndian.PutUint64(zip64[32:40], uint64(entries))
	binary.LittleEndian.PutUint64(zip64[40:48], uint64(centralSize))
	binary.LittleEndian.PutUint64(zip64[48:56], uint64(centralOffset))
	locator := zip64[directory64EndLen:]
	binary.LittleEndian.PutUint32(locator[0:4], directory64LocSignature)
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
