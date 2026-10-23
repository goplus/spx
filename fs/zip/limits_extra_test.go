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
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"testing"
)

func TestValidateZipReaderBoundaries(t *testing.T) {
	entry := func(name string, compressed, uncompressed uint64) *archivezip.File {
		return &archivezip.File{FileHeader: archivezip.FileHeader{
			Name: name, CompressedSize64: compressed, UncompressedSize64: uncompressed,
		}}
	}
	maxEntries := make([]*archivezip.File, MaxZipEntries)
	for i := range maxEntries {
		maxEntries[i] = entry(fmt.Sprintf("entry-%d", i), 0, 0)
	}
	var maxTotal []*archivezip.File
	for i := int64(0); i < MaxZipTotalSize/MaxZipEntrySize; i++ {
		compressed := (uint64(MaxZipEntrySize) + MaxZipCompressionRatio - 1) / MaxZipCompressionRatio
		maxTotal = append(maxTotal, entry(fmt.Sprintf("entry-%d", i), compressed, uint64(MaxZipEntrySize)))
	}

	tests := []struct {
		name   string
		reader *archivezip.Reader
		detail string
	}{
		{name: "nil reader", detail: "nil ZIP reader"},
		{name: "empty archive", reader: &archivezip.Reader{}},
		{name: "nil entry", reader: &archivezip.Reader{File: []*archivezip.File{nil}}, detail: "nil ZIP entry"},
		{name: "entry count at limit", reader: &archivezip.Reader{File: maxEntries}},
		{name: "entry count above limit", reader: &archivezip.Reader{File: make([]*archivezip.File, MaxZipEntries+1)}, detail: "entries exceeds limit"},
		{name: "entry and total sizes at limit", reader: &archivezip.Reader{File: maxTotal}},
		{name: "compressed size at limit", reader: &archivezip.Reader{File: []*archivezip.File{entry("empty", uint64(MaxZipArchiveBytes), 0)}}},
		{name: "compressed size above limit", reader: &archivezip.Reader{File: []*archivezip.File{entry("empty", uint64(MaxZipArchiveBytes)+1, 0)}}, detail: "total compressed size"},
		{name: "directory storage at limit", reader: &archivezip.Reader{File: []*archivezip.File{entry("dir/", uint64(MaxZipArchiveBytes), 0)}}},
		{name: "directory storage counts toward limit", reader: &archivezip.Reader{File: []*archivezip.File{entry("dir/", uint64(MaxZipArchiveBytes)+1, 0)}}, detail: "total compressed size"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateZipReader(tt.reader)
			if tt.detail == "" {
				if err != nil {
					t.Fatalf("validateZipReader error = %v", err)
				}
				return
			}
			if !errors.Is(err, ErrArchiveLimit) || !strings.Contains(err.Error(), tt.detail) {
				t.Fatalf("validateZipReader error = %v, want ErrArchiveLimit containing %q", err, tt.detail)
			}
		})
	}
}

func TestCheckZipCompressionRatioBoundaries(t *testing.T) {
	tests := []struct {
		name         string
		compressed   uint64
		uncompressed uint64
		ratio        uint64
		wantLimit    bool
	}{
		{name: "empty entry with zero ratio"},
		{name: "ratio at limit", compressed: 2, uncompressed: 400, ratio: 200},
		{name: "ratio above limit", compressed: 2, uncompressed: 401, ratio: 200, wantLimit: true},
		{name: "zero compressed size", uncompressed: 1, ratio: 200, wantLimit: true},
		{name: "zero ratio", compressed: 1, uncompressed: 1, wantLimit: true},
		{name: "largest product", compressed: math.MaxUint64, uncompressed: math.MaxUint64, ratio: 1},
		{name: "product below overflow", compressed: math.MaxUint64 / 2, uncompressed: math.MaxUint64 - 1, ratio: 2},
		{name: "product would overflow", compressed: math.MaxUint64/2 + 1, uncompressed: 1, ratio: 2, wantLimit: true},
	}
	if err := checkZipCompressionRatio(nil, MaxZipCompressionRatio); err != nil {
		t.Fatalf("nil entry error = %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &archivezip.File{FileHeader: archivezip.FileHeader{
				Name: "ratio", CompressedSize64: tt.compressed, UncompressedSize64: tt.uncompressed,
			}}
			err := checkZipCompressionRatio(file, tt.ratio)
			if tt.wantLimit {
				if !errors.Is(err, ErrArchiveLimit) || !strings.Contains(err.Error(), "ratio") {
					t.Fatalf("checkZipCompressionRatio error = %v, want named ErrArchiveLimit", err)
				}
			} else if err != nil {
				t.Fatalf("checkZipCompressionRatio error = %v", err)
			}
		})
	}
}

func TestOpenZipEntryRejectsInvalidMetadata(t *testing.T) {
	for name, file := range map[string]*archivezip.File{
		"nil":       nil,
		"oversized": {FileHeader: archivezip.FileHeader{Name: "large", UncompressedSize64: uint64(MaxZipEntrySize) + 1}},
	} {
		t.Run(name, func(t *testing.T) {
			reader, err := openZipEntry(file)
			if reader != nil || !errors.Is(err, ErrArchiveLimit) {
				t.Fatalf("openZipEntry = %v, %v, want nil, ErrArchiveLimit", reader, err)
			}
		})
	}
}

func TestOpenZipEntryPreservesArchiveErrors(t *testing.T) {
	var contents bytes.Buffer
	writer := archivezip.NewWriter(&contents)
	entry, err := writer.CreateHeader(&archivezip.FileHeader{Name: "entry", Method: archivezip.Store})
	if err != nil {
		t.Fatal(err)
	}
	const payload = "archive contents"
	if _, err := io.WriteString(entry, payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name    string
		modify  func(*archivezip.File)
		openErr error
		readErr error
	}{
		{name: "valid entry"},
		{name: "unsupported method", modify: func(f *archivezip.File) { f.Method = math.MaxUint16 }, openErr: archivezip.ErrAlgorithm},
		{name: "checksum mismatch", modify: func(f *archivezip.File) { f.CRC32 ^= 1 }, readErr: archivezip.ErrChecksum},
	} {
		t.Run(tt.name, func(t *testing.T) {
			archive, err := archivezip.NewReader(bytes.NewReader(contents.Bytes()), int64(contents.Len()))
			if err != nil {
				t.Fatal(err)
			}
			file := archive.File[0]
			if tt.modify != nil {
				tt.modify(file)
			}
			reader, err := openZipEntry(file)
			if !errors.Is(err, tt.openErr) {
				t.Fatalf("openZipEntry error = %v, want %v", err, tt.openErr)
			}
			if tt.openErr != nil {
				if reader != nil {
					t.Fatalf("openZipEntry returned reader %v on error", reader)
				}
				return
			}
			data, readErr := io.ReadAll(reader)
			closeErr := reader.Close()
			if string(data) != payload || !errors.Is(readErr, tt.readErr) || closeErr != nil {
				t.Fatalf("read entry = %q, %v; close = %v; want %q, %v; nil", data, readErr, closeErr, payload, tt.readErr)
			}
		})
	}
}

func TestZipLimitedReaderBoundsSuccessiveReads(t *testing.T) {
	source := &trackedZipReadCloser{Reader: strings.NewReader("12345")}
	reader := &zipLimitedReadCloser{ReadCloser: source, max: 4, name: "chunked"}
	defer reader.Close()
	buf := make([]byte, 3)
	if n, err := reader.Read(buf); n != 3 || err != nil || string(buf[:n]) != "123" {
		t.Fatalf("first Read = %d, %v, %q, want 3, nil, 123", n, err, buf)
	}
	n, limitErr := reader.Read(buf)
	if n != 1 || !errors.Is(limitErr, ErrArchiveLimit) || string(buf[:n]) != "4" {
		t.Fatalf("second Read = %d, %v, %q, want 1, ErrArchiveLimit, 4", n, limitErr, buf)
	}
	if source.lastBufferSize != 2 {
		t.Fatalf("underlying buffer size = %d, want remaining budget 2", source.lastBufferSize)
	}
	if n, err := reader.Read(buf); n != 0 || err != limitErr || source.readCalls != 2 {
		t.Fatalf("Read after limit = %d, %v, calls = %d; want 0, same error, 2", n, err, source.readCalls)
	}
}

func TestZipLimitedReaderRejectsInvalidReadCounts(t *testing.T) {
	for _, count := range []int{-1, 4} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			source := &trackedZipReadCloser{Reader: zipReadFunc(func([]byte) (int, error) { return count, nil })}
			reader := &zipLimitedReadCloser{ReadCloser: source, max: 2, name: "invalid-count"}
			defer reader.Close()
			buf := make([]byte, 8)
			n, limitErr := reader.Read(buf)
			if n != 0 || !errors.Is(limitErr, ErrArchiveLimit) || !strings.Contains(limitErr.Error(), "invalid read count") {
				t.Fatalf("Read = %d, %v, want 0, invalid-read-count ErrArchiveLimit", n, limitErr)
			}
			if n, err := reader.Read(buf); n != 0 || err != limitErr || source.readCalls != 1 {
				t.Fatalf("Read after invalid count = %d, %v, calls = %d; want 0, same error, 1", n, err, source.readCalls)
			}
		})
	}
}

func TestZipLimitedReaderRejectsDataAfterDeclaredEnd(t *testing.T) {
	for _, max := range []int64{0, MaxZipEntrySize} {
		source := &trackedZipReadCloser{Reader: strings.NewReader("unexpected")}
		reader := &zipLimitedReadCloser{ReadCloser: source, max: max, read: max, name: "at-limit"}
		defer reader.Close()
		if n, err := reader.Read(make([]byte, 4)); n != 0 || !errors.Is(err, ErrArchiveLimit) || source.lastBufferSize != 1 {
			t.Fatalf("Read at %d = %d, %v, buffer = %d; want 0, ErrArchiveLimit, 1", max, n, err, source.lastBufferSize)
		}
	}
}

func TestZipLimitedReaderPreservesReadAndCloseErrors(t *testing.T) {
	readErr := errors.New("source read failed")
	closeErr := errors.New("source close failed")
	for _, tt := range []struct {
		name string
		data string
		err  error
	}{
		{name: "read error", err: readErr},
		{name: "data with error", data: "ok", err: readErr},
		{name: "data with EOF", data: "ok", err: io.EOF},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := &trackedZipReadCloser{
				Reader:   zipReadFunc(func(p []byte) (int, error) { return copy(p, tt.data), tt.err }),
				closeErr: closeErr,
			}
			reader := &zipLimitedReadCloser{ReadCloser: source, max: 4, name: "source-errors"}
			buf := make([]byte, 4)
			n, err := reader.Read(buf)
			if n != len(tt.data) || string(buf[:n]) != tt.data || err != tt.err {
				t.Fatalf("Read = %d, %q, %v, want %d, %q, %v", n, buf, err, len(tt.data), tt.data, tt.err)
			}
			if err := reader.Close(); err != closeErr || source.closeCalls != 1 {
				t.Fatalf("Close = %v, calls = %d; want source error, 1", err, source.closeCalls)
			}
		})
	}
}

type zipReadFunc func([]byte) (int, error)

func (f zipReadFunc) Read(p []byte) (int, error) { return f(p) }

type trackedZipReadCloser struct {
	io.Reader
	readCalls      int
	lastBufferSize int
	closeCalls     int
	closeErr       error
}

func (r *trackedZipReadCloser) Read(p []byte) (int, error) {
	r.readCalls++
	r.lastBufferSize = len(p)
	return r.Reader.Read(p)
}

func (r *trackedZipReadCloser) Close() error {
	r.closeCalls++
	return r.closeErr
}
