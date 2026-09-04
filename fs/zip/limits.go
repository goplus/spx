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
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/goplus/spx/v3/internal/zippreflight"
)

// Limits bound ZIP metadata and decompression work.
const (
	// MaxZipArchiveBytes limits compressed input.
	MaxZipArchiveBytes int64 = 512 << 20
	// MaxZipCentralDirectoryBytes limits central-directory metadata.
	MaxZipCentralDirectoryBytes int64 = 64 << 20
	// MaxZipEntries limits central-directory entries.
	MaxZipEntries = 10_000
	// MaxZipEntrySize limits one uncompressed entry.
	MaxZipEntrySize int64 = 512 << 20
	// MaxZipTotalSize limits total uncompressed data.
	MaxZipTotalSize int64 = 4 << 30
	// MaxZipCompressionRatio limits declared expansion.
	MaxZipCompressionRatio uint64 = 200
)

// ErrArchiveLimit identifies excessive ZIP work.
var ErrArchiveLimit = errors.New("zip: archive limit exceeded")

type resolvedZipLimits struct {
	maxArchiveBytes     int64
	maxCentralDirBytes  int64
	maxEntries          int
	maxEntrySize        int64
	maxTotalSize        int64
	maxCompressionRatio uint64
}

func defaultZipLimits() resolvedZipLimits {
	return resolvedZipLimits{
		maxArchiveBytes:     MaxZipArchiveBytes,
		maxCentralDirBytes:  MaxZipCentralDirectoryBytes,
		maxEntries:          MaxZipEntries,
		maxEntrySize:        MaxZipEntrySize,
		maxTotalSize:        MaxZipTotalSize,
		maxCompressionRatio: MaxZipCompressionRatio,
	}
}

func normalizeZipLimits(limits resolvedZipLimits) resolvedZipLimits {
	// Keep test overrides bounded when fields are omitted.
	if limits.maxArchiveBytes <= 0 {
		limits.maxArchiveBytes = MaxZipArchiveBytes
	}
	if limits.maxCentralDirBytes <= 0 {
		limits.maxCentralDirBytes = MaxZipCentralDirectoryBytes
	}
	if limits.maxEntries <= 0 {
		limits.maxEntries = MaxZipEntries
	}
	if limits.maxEntrySize <= 0 {
		limits.maxEntrySize = MaxZipEntrySize
	}
	if limits.maxTotalSize <= 0 {
		limits.maxTotalSize = MaxZipTotalSize
	}
	if limits.maxCompressionRatio == 0 {
		limits.maxCompressionRatio = MaxZipCompressionRatio
	}
	if limits.maxEntrySize > limits.maxTotalSize {
		limits.maxEntrySize = limits.maxTotalSize
	}
	return limits
}

// Package tests override limits concurrently.
var zipTestLimits atomic.Pointer[resolvedZipLimits]

func currentZipLimits() resolvedZipLimits {
	if limits := zipTestLimits.Load(); limits != nil {
		return *limits
	}
	return defaultZipLimits()
}

func setZipTestLimits(limits resolvedZipLimits) {
	limits = normalizeZipLimits(limits)
	zipTestLimits.Store(&limits)
}

func clearZipTestLimits() {
	zipTestLimits.Store(nil)
}

// preflightZipArchive bounds work before zip.NewReader allocates entries.
func preflightZipArchive(reader io.ReaderAt, size int64) error {
	return preflightZipArchiveWithLimits(reader, size, currentZipLimits())
}

func preflightZipArchiveWithLimits(reader io.ReaderAt, size int64, limits resolvedZipLimits) error {
	limits = normalizeZipLimits(limits)
	err := zippreflight.Check(reader, size, zippreflight.Limits{
		MaxArchiveBytes:          limits.maxArchiveBytes,
		MaxCentralDirectoryBytes: limits.maxCentralDirBytes,
		MaxEntries:               limits.maxEntries,
	})
	if zippreflight.IsLimit(err) {
		return fmt.Errorf("%w: %v", ErrArchiveLimit, err)
	}
	return err
}

// validateZipReader bounds declared expansion work.
func validateZipReader(reader *zip.Reader) error {
	if reader == nil {
		return fmt.Errorf("%w: nil ZIP reader", ErrArchiveLimit)
	}
	limits := currentZipLimits()
	if len(reader.File) > limits.maxEntries {
		return fmt.Errorf("%w: %d entries exceeds limit %d", ErrArchiveLimit, len(reader.File), limits.maxEntries)
	}

	paths := make([]zipPath, 0, len(reader.File))
	var compressedTotal, total uint64
	for _, file := range reader.File {
		if err := validateZipFile(file, limits); err != nil {
			return err
		}
		path, err := newZipPath(file)
		if err != nil {
			return err
		}
		paths = append(paths, path)
		compressed := file.CompressedSize64
		if compressed > uint64(limits.maxArchiveBytes) || compressedTotal > uint64(limits.maxArchiveBytes)-compressed {
			return fmt.Errorf("%w: total compressed size exceeds limit %d", ErrArchiveLimit, limits.maxArchiveBytes)
		}
		compressedTotal += compressed
		if isZipDirectory(file) {
			continue
		}
		size := file.UncompressedSize64
		if size > uint64(limits.maxTotalSize) || total > uint64(limits.maxTotalSize)-size {
			return fmt.Errorf("%w: total uncompressed size exceeds limit %d", ErrArchiveLimit, limits.maxTotalSize)
		}
		total += size
	}
	return validateZipPaths(paths)
}

type zipPath struct {
	name      string
	directory bool
}

func newZipPath(file *zip.File) (zipPath, error) {
	directory := isZipDirectory(file)
	name := file.Name
	if directory {
		name = strings.TrimSuffix(name, "/")
	}
	if !validZipPath(name) {
		return zipPath{}, fmt.Errorf("zip: invalid entry name %q: %w", file.Name, zip.ErrFormat)
	}
	return zipPath{name: name, directory: directory}, nil
}

func validZipPath(name string) bool {
	if name == "" || name[0] == '/' || name[len(name)-1] == '/' || strings.ContainsRune(name, '\\') || hasZipDrivePrefix(name) {
		return false
	}
	for {
		part := name
		if slash := strings.IndexByte(name, '/'); slash >= 0 {
			part, name = name[:slash], name[slash+1:]
		} else {
			name = ""
		}
		if part == "" || part == "." || part == ".." {
			return false
		}
		if name == "" {
			return true
		}
	}
}

func hasZipDrivePrefix(name string) bool {
	return len(name) >= 2 && name[1] == ':' &&
		(name[0] >= 'A' && name[0] <= 'Z' || name[0] >= 'a' && name[0] <= 'z')
}

func validateZipPaths(paths []zipPath) error {
	sort.Slice(paths, func(i, j int) bool { return paths[i].name < paths[j].name })
	for i, path := range paths {
		if i > 0 && paths[i-1].name == path.name {
			return fmt.Errorf("zip: duplicate entry %q: %w", path.name, zip.ErrFormat)
		}
		if path.directory {
			continue
		}
		prefix := path.name + "/"
		child := sort.Search(len(paths), func(i int) bool { return paths[i].name >= prefix })
		if child < len(paths) && strings.HasPrefix(paths[child].name, prefix) {
			return fmt.Errorf("zip: file entry %q is a parent path: %w", path.name, zip.ErrFormat)
		}
	}
	return nil
}

func validateZipFile(file *zip.File, limits resolvedZipLimits) error {
	if file == nil {
		return fmt.Errorf("%w: nil ZIP entry", ErrArchiveLimit)
	}
	if isZipDirectory(file) {
		// Some writers give empty directories non-zero compressed storage.
		if file.UncompressedSize64 != 0 {
			return fmt.Errorf("%w: directory entry %q has non-zero uncompressed size %d", ErrArchiveLimit, file.Name, file.UncompressedSize64)
		}
		return nil
	}
	uncompressed := file.UncompressedSize64
	if uncompressed > uint64(limits.maxEntrySize) {
		return fmt.Errorf("%w: entry %q uncompressed size %d exceeds limit %d", ErrArchiveLimit, file.Name, uncompressed, limits.maxEntrySize)
	}
	if err := checkZipCompressionRatio(file, limits.maxCompressionRatio); err != nil {
		return err
	}
	return nil
}

func isZipDirectory(file *zip.File) bool {
	return file != nil && strings.HasSuffix(file.Name, "/")
}

func checkZipCompressionRatio(file *zip.File, maxRatio uint64) error {
	if file == nil || file.UncompressedSize64 == 0 {
		return nil
	}
	compressed := file.CompressedSize64
	if compressed == 0 || maxRatio == 0 || compressed > math.MaxUint64/maxRatio || file.UncompressedSize64 > compressed*maxRatio {
		return fmt.Errorf("%w: entry %q compression ratio exceeds %d:1", ErrArchiveLimit, file.Name, maxRatio)
	}
	return nil
}

// openZipEntry caps output at the entry's declared size plus one sentinel byte.
func openZipEntry(file *zip.File) (io.ReadCloser, error) {
	limits := currentZipLimits()
	if err := validateZipFile(file, limits); err != nil {
		return nil, err
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	max := int64(file.UncompressedSize64)
	return &zipLimitedReadCloser{
		ReadCloser: reader,
		remaining:  zipLimitWithSentinel(max),
		max:        max,
		name:       file.Name,
	}, nil
}

func zipLimitWithSentinel(max int64) int64 {
	if max >= math.MaxInt64 {
		return math.MaxInt64
	}
	return max + 1
}

type zipLimitedReadCloser struct {
	io.ReadCloser
	remaining int64
	max       int64
	read      int64
	name      string
	limitErr  error
}

func (r *zipLimitedReadCloser) Read(p []byte) (int, error) {
	if r.limitErr != nil {
		return 0, r.limitErr
	}
	if r.remaining == 0 {
		r.limitErr = fmt.Errorf("%w: entry %q uncompressed output exceeds limit %d", ErrArchiveLimit, r.name, r.max)
		return 0, r.limitErr
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.ReadCloser.Read(p)
	if n < 0 || n > len(p) {
		r.limitErr = fmt.Errorf("%w: entry %q returned invalid read count %d", ErrArchiveLimit, r.name, n)
		return 0, r.limitErr
	}
	allowed := r.max - r.read
	r.remaining -= int64(n)
	r.read += int64(n)
	if int64(n) > allowed {
		r.limitErr = fmt.Errorf("%w: entry %q uncompressed output exceeds limit %d", ErrArchiveLimit, r.name, r.max)
		// Do not expose the sentinel byte.
		return int(allowed), r.limitErr
	}
	return n, err
}
