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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type limitZipEntry struct {
	name   string
	data   string
	method uint16
}

func TestOpenRejectsTooManyZipEntries(t *testing.T) {
	entries := make([]limitZipEntry, MaxZipEntries+1)
	for i := range entries {
		entries[i].name = fmt.Sprintf("entry-%d", i)
	}
	path := makeLimitZip(t, entries...)

	_, err := Open(path)
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("Open error = %v, want ErrArchiveLimit", err)
	}
}

func TestValidateRejectsOversizedZipEntry(t *testing.T) {
	reader := &archivezip.Reader{File: []*archivezip.File{
		{FileHeader: archivezip.FileHeader{Name: "large", UncompressedSize64: uint64(MaxZipEntrySize) + 1}},
	}}
	if err := validateZipReader(reader); !errors.Is(err, ErrArchiveLimit) || !strings.Contains(err.Error(), "large") {
		t.Fatalf("validateZipReader error = %v, want named entry-size error", err)
	}
}

func TestValidateRejectsExcessiveTotalZipSize(t *testing.T) {
	reader := &archivezip.Reader{}
	for i := int64(0); i <= MaxZipTotalSize/MaxZipEntrySize; i++ {
		reader.File = append(reader.File, &archivezip.File{FileHeader: archivezip.FileHeader{
			Name:               fmt.Sprintf("entry-%d", i),
			UncompressedSize64: uint64(MaxZipEntrySize),
			CompressedSize64:   uint64(MaxZipEntrySize)/MaxZipCompressionRatio + 1,
		}})
	}
	if err := validateZipReader(reader); !errors.Is(err, ErrArchiveLimit) || !strings.Contains(err.Error(), "total uncompressed") {
		t.Fatalf("validateZipReader error = %v, want total-size error", err)
	}
}

func TestValidateRejectsRepeatedCompressedWork(t *testing.T) {
	reader := &archivezip.Reader{File: []*archivezip.File{
		{FileHeader: archivezip.FileHeader{Name: "one", CompressedSize64: uint64(MaxZipArchiveBytes)/2 + 1}},
		{FileHeader: archivezip.FileHeader{Name: "two", CompressedSize64: uint64(MaxZipArchiveBytes)/2 + 1}},
	}}
	if err := validateZipReader(reader); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("validateZipReader error = %v, want ErrArchiveLimit", err)
	}
}

func TestOpenRejectsExcessiveZipCompressionRatio(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{
		name:   "repetitive",
		data:   strings.Repeat("a", 65536),
		method: archivezip.Deflate,
	})

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

func TestOpenRejectsAmbiguousZipPaths(t *testing.T) {
	tests := []struct {
		name    string
		entries []limitZipEntry
	}{
		{"duplicate", []limitZipEntry{{name: "a"}, {name: "a"}}},
		{"duplicate directory", []limitZipEntry{{name: "a/"}, {name: "a/"}}},
		{"file then directory", []limitZipEntry{{name: "a"}, {name: "a/"}}},
		{"directory then file", []limitZipEntry{{name: "a/"}, {name: "a"}}},
		{"file before child", []limitZipEntry{{name: "a"}, {name: "a-b"}, {name: "a/b"}}},
		{"file after child", []limitZipEntry{{name: "a/b/c"}, {name: "a/b"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Open(makeLimitZip(t, test.entries...))
			if !errors.Is(err, archivezip.ErrFormat) {
				t.Fatalf("Open error = %v, want archive/zip format error", err)
			}
		})
	}
}

func TestOpenRejectsInvalidZipPaths(t *testing.T) {
	for _, name := range []string{"", "/a", `a\b`, "a//b", "a/./b", "a/../b", "../a", "C:/a", ".", "a//"} {
		t.Run(name, func(t *testing.T) {
			_, err := Open(makeLimitZip(t, limitZipEntry{name: name}))
			if !errors.Is(err, archivezip.ErrFormat) && !errors.Is(err, archivezip.ErrInsecurePath) {
				t.Fatalf("Open error = %v, want invalid path error", err)
			}
		})
	}
}

func TestOpenAcceptsUnambiguousZipPaths(t *testing.T) {
	tests := []struct {
		name    string
		entries []limitZipEntry
	}{
		{"directory before child", []limitZipEntry{{name: "a/"}, {name: "a/b", data: "b"}}},
		{"directory after child", []limitZipEntry{{name: "a/b", data: "b"}, {name: "a/"}}},
		{"adjacent prefix", []limitZipEntry{{name: "a"}, {name: "ab/c"}}},
		{"deep path", []limitZipEntry{{name: strings.Repeat("a/", 32_000) + "z"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir, err := Open(makeLimitZip(t, test.entries...))
			if err != nil {
				t.Fatal(err)
			}
			if err := dir.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOpenAndReadZip(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "exact", data: "1234", method: archivezip.Store})

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

func TestOpenReadRejectsEntryThatExceedsDeclaration(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "grow", data: "123456", method: archivezip.Store})
	reader, err := archivezip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	// Mutating the exported header simulates an archive whose declaration was
	// tampered with after parsing. The wrapper must still fail closed.
	reader.File[0].UncompressedSize64 = 4
	zipFS := &FS{Reader: &reader.Reader}
	entry, err := zipFS.Open("grow")
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(entry)
	_ = entry.Close()
	if readErr == nil {
		t.Fatal("reading tampered entry succeeded")
	}
	if len(data) > 4 {
		t.Fatalf("read %d bytes, want at most declared size", len(data))
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
