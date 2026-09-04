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
	"strings"
)

// The limits protect callers that open ZIPs supplied by projects or remote
// servers. They bound both metadata processing and the amount of data a
// successful entry read can produce.
const (
	// MaxZipEntries is the maximum number of central-directory entries.
	MaxZipEntries = 10_000
	// MaxZipEntrySize is the maximum uncompressed size of one regular entry.
	MaxZipEntrySize int64 = 512 << 20
	// MaxZipTotalSize is the maximum sum of uncompressed regular entries.
	MaxZipTotalSize int64 = 4 << 30
	// MaxZipCompressionRatio is the maximum declared uncompressed/compressed
	// ratio for a non-empty regular entry.
	MaxZipCompressionRatio uint64 = 200
)

var (
	// ErrArchiveLimit identifies an archive rejected because it could require
	// excessive metadata, decompression, or CPU work.
	ErrArchiveLimit = errors.New("zip: archive limit exceeded")
	// ErrZipArchiveLimit is an explicit package-qualified alias for callers
	// that want to distinguish this limit from other ZIP errors.
	ErrZipArchiveLimit = ErrArchiveLimit
	// These package variables are intentionally unexported so production code
	// cannot change process-wide limits. Tests use them to exercise boundaries.
	zipMaxEntries          = MaxZipEntries
	zipMaxEntrySize        = MaxZipEntrySize
	zipMaxTotalSize        = MaxZipTotalSize
	zipMaxCompressionRatio = MaxZipCompressionRatio
)

type resolvedZipLimits struct {
	maxEntries          int
	maxEntrySize        int64
	maxTotalSize        int64
	maxCompressionRatio uint64
}

func currentZipLimits() resolvedZipLimits {
	limits := resolvedZipLimits{
		maxEntries:          zipMaxEntries,
		maxEntrySize:        zipMaxEntrySize,
		maxTotalSize:        zipMaxTotalSize,
		maxCompressionRatio: zipMaxCompressionRatio,
	}
	// A test hook must not make the production path accidentally unbounded.
	// Falling back to defaults also keeps a zero value useful in tests.
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

// validateZipReader checks central-directory declarations before an archive
// is returned to a caller. archive/zip intentionally leaves expansion limits
// to its users, so UncompressedSize64 and the compression ratio must be
// checked here.
func validateZipReader(reader *zip.Reader) error {
	if reader == nil {
		return fmt.Errorf("%w: nil ZIP reader", ErrArchiveLimit)
	}
	limits := currentZipLimits()
	if len(reader.File) > limits.maxEntries {
		return fmt.Errorf("%w: %d entries exceeds limit %d", ErrArchiveLimit, len(reader.File), limits.maxEntries)
	}

	var total uint64
	for _, file := range reader.File {
		if err := validateZipFile(file, limits); err != nil {
			return err
		}
		if file == nil || isZipDirectory(file) {
			continue
		}
		size := file.UncompressedSize64
		if size > uint64(limits.maxTotalSize) || total > uint64(limits.maxTotalSize)-size {
			return fmt.Errorf("%w: total uncompressed size exceeds limit %d", ErrArchiveLimit, limits.maxTotalSize)
		}
		total += size
	}
	return nil
}

func validateZipFile(file *zip.File, limits resolvedZipLimits) error {
	if file == nil {
		return fmt.Errorf("%w: nil ZIP entry", ErrArchiveLimit)
	}
	if isZipDirectory(file) {
		// A directory has no logical payload. archive/zip permits a non-zero
		// compressed size for some producer quirks, so do not apply a ratio to
		// that storage data. A non-zero uncompressed declaration is still
		// rejected because it would bypass the archive's total-size accounting.
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
		name := "<unknown>"
		if file != nil {
			name = file.Name
		}
		return fmt.Errorf("%w: entry %q compression ratio exceeds %d:1", ErrArchiveLimit, name, maxRatio)
	}
	return nil
}

// openZipEntry validates a single entry again at read time and caps the
// decompressor's output with a one-byte sentinel. The sentinel ensures an
// entry whose header understates its output cannot silently terminate at the
// configured limit without letting archive/zip verify its checksum.
func openZipEntry(file *zip.File) (io.ReadCloser, error) {
	limits := currentZipLimits()
	if err := validateZipFile(file, limits); err != nil {
		return nil, err
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	return &zipLimitedReadCloser{
		ReadCloser: reader,
		remaining:  zipLimitWithSentinel(limits.maxEntrySize),
		max:        limits.maxEntrySize,
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
		// The extra byte was consumed only as a sentinel. Do not expose it to
		// callers, even though the underlying reader returned it in p.
		return int(allowed), r.limitErr
	}
	return n, err
}
