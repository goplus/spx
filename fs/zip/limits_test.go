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
	path := makeLimitZip(t, limitZipEntry{name: "one", data: "1", method: archivezip.Store}, limitZipEntry{name: "two", data: "2", method: archivezip.Store})
	old := zipMaxEntries
	zipMaxEntries = 1
	t.Cleanup(func() { zipMaxEntries = old })

	_, err := Open(path)
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("Open error = %v, want ErrArchiveLimit", err)
	}
}

func TestOpenRejectsOversizedZipEntry(t *testing.T) {
	path := makeLimitZip(t, limitZipEntry{name: "large", data: "12345", method: archivezip.Store})
	oldSize := zipMaxEntrySize
	zipMaxEntrySize = 4
	t.Cleanup(func() { zipMaxEntrySize = oldSize })

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
	oldTotal := zipMaxTotalSize
	zipMaxTotalSize = 5
	t.Cleanup(func() { zipMaxTotalSize = oldTotal })

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
	oldRatio := zipMaxCompressionRatio
	zipMaxCompressionRatio = 2
	t.Cleanup(func() { zipMaxCompressionRatio = oldRatio })

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
	oldSize, oldTotal, oldRatio := zipMaxEntrySize, zipMaxTotalSize, zipMaxCompressionRatio
	zipMaxEntrySize = 4
	zipMaxTotalSize = 4
	zipMaxCompressionRatio = 1
	t.Cleanup(func() {
		zipMaxEntrySize, zipMaxTotalSize, zipMaxCompressionRatio = oldSize, oldTotal, oldRatio
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
	oldSize := zipMaxEntrySize
	zipMaxEntrySize = 4
	t.Cleanup(func() { zipMaxEntrySize = oldSize })
	zipFS := (*FS)(reader)
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
