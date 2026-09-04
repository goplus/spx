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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync/atomic"
)

// The limits protect callers that open ZIPs supplied by projects or remote
// servers. They bound both metadata processing and the amount of data a
// successful entry read can produce.
const (
	// MaxZipArchiveBytes is the maximum compressed ZIP byte length accepted.
	MaxZipArchiveBytes int64 = 512 << 20
	// MaxZipCentralDirectoryBytes bounds central-directory metadata parsed by
	// archive/zip before its Reader is returned.
	MaxZipCentralDirectoryBytes int64 = 64 << 20
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
)

type resolvedZipLimits struct {
	maxArchiveBytes     int64
	maxCentralDirBytes  int64
	maxEntries          int
	maxEntrySize        int64
	maxTotalSize        int64
	maxCompressionRatio uint64
}

func defaultZipLimits() resolvedZipLimits {
	return normalizeZipLimits(resolvedZipLimits{
		maxArchiveBytes:     MaxZipArchiveBytes,
		maxCentralDirBytes:  MaxZipCentralDirectoryBytes,
		maxEntries:          MaxZipEntries,
		maxEntrySize:        MaxZipEntrySize,
		maxTotalSize:        MaxZipTotalSize,
		maxCompressionRatio: MaxZipCompressionRatio,
	})
}

func normalizeZipLimits(limits resolvedZipLimits) resolvedZipLimits {
	// A test hook must not make the production path accidentally unbounded.
	// Falling back to defaults also keeps a zero value useful in tests.
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

// The override is used only by package tests. An atomic pointer keeps tests
// that exercise concurrent readers race-free while production uses constants.
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

const (
	zipDirectoryHeaderSignature = 0x02014b50
	zipDirectoryEndSignature    = 0x06054b50
	zipDirectory64EndSignature  = 0x06064b50
	zipDirectory64LocSignature  = 0x07064b50
	zipDirectoryHeaderLen       = 46
	zipDirectoryEndLen          = 22
	zipDirectory64EndLen        = 56
	zipDirectory64LocLen        = 20
	zipDirectorySearchWindow    = 65 << 10
)

type zipDirectoryInfo struct {
	// endOffset is the absolute offset used by archive/zip to derive the
	// archive base (classic EOCD or ZIP64 EOCD, respectively).
	endOffset int64
	// eocdOffset is always the absolute offset of the classic EOCD record.
	eocdOffset int64
	offset     uint64
	size       uint64
	entries    uint64
}

// preflightZipArchive bounds archive/zip's central-directory work before
// zip.NewReader is allowed to allocate one File and its metadata per entry.
func preflightZipArchive(reader io.ReaderAt, size int64) error {
	return preflightZipArchiveWithLimits(reader, size, currentZipLimits())
}

func preflightZipArchiveWithLimits(reader io.ReaderAt, size int64, limits resolvedZipLimits) error {
	if reader == nil {
		return fmt.Errorf("%w: nil ZIP reader", ErrArchiveLimit)
	}
	limits = normalizeZipLimits(limits)
	if size < 0 {
		return fmt.Errorf("zip: negative archive size: %w", zip.ErrFormat)
	}
	if size > limits.maxArchiveBytes {
		return fmt.Errorf("%w: archive size %d exceeds limit %d", ErrArchiveLimit, size, limits.maxArchiveBytes)
	}

	directory, err := readZipDirectoryInfo(reader, size, limits)
	if err != nil {
		return err
	}
	if directory.size > uint64(limits.maxCentralDirBytes) {
		return fmt.Errorf("%w: central directory size %d exceeds limit %d", ErrArchiveLimit, directory.size, limits.maxCentralDirBytes)
	}
	if directory.size > uint64(size) || directory.offset > math.MaxInt64 {
		return fmt.Errorf("zip: invalid central directory bounds: %w", zip.ErrFormat)
	}
	if directory.endOffset < 0 || directory.endOffset > size || directory.size > uint64(directory.endOffset) {
		return fmt.Errorf("zip: invalid central directory offset: %w", zip.ErrFormat)
	}

	centralSize := int64(directory.size)
	centralOffset := int64(directory.offset)
	start := directory.endOffset - centralSize
	if start < 0 || start > size {
		return fmt.Errorf("zip: invalid central directory offset: %w", zip.ErrFormat)
	}
	baseOffset := start - centralOffset
	// Match archive/zip's compatibility handling for archives whose directory
	// offset is absolute despite a non-zero computed base offset. The header
	// predicate mirrors archive/zip's parser so both readers select the same
	// physical directory.
	if baseOffset > 0 && completeZipDirectoryHeaderAt(reader, size, centralOffset) > 0 {
		start = centralOffset
	}
	centralEnd := start + centralSize
	if centralEnd < start || centralEnd > size || centralEnd > directory.endOffset {
		return fmt.Errorf("zip: invalid central directory bounds: %w", zip.ErrFormat)
	}
	if err := scanZipCentralDirectory(reader, size, start, centralEnd, directory.entries, limits); err != nil {
		return err
	}
	// archive/zip reads directory headers until the first invalid signature,
	// even when a compatibility base offset makes the declared byte range end
	// before the end record. Reject an immediately following header so it
	// cannot make archive/zip allocate entries that the bounded scan missed.
	if centralEnd < directory.endOffset && completeZipDirectoryHeaderAt(reader, size, centralEnd) > 0 {
		return fmt.Errorf("zip: central directory extends beyond its declared size: %w", zip.ErrFormat)
	}
	return nil
}

func readZipDirectoryInfo(reader io.ReaderAt, size int64, limits resolvedZipLimits) (zipDirectoryInfo, error) {
	if size < zipDirectoryEndLen {
		return zipDirectoryInfo{}, fmt.Errorf("zip: archive too small: %w", zip.ErrFormat)
	}
	window := int64(zipDirectorySearchWindow)
	if window > size {
		window = size
	}
	buf := make([]byte, int(window))
	if _, err := reader.ReadAt(buf, size-window); err != nil {
		return zipDirectoryInfo{}, err
	}

	var endIndex = -1
	for i := len(buf) - zipDirectoryEndLen; i >= 0; i-- {
		if binary.LittleEndian.Uint32(buf[i:i+4]) != zipDirectoryEndSignature {
			continue
		}
		commentLen := int(binary.LittleEndian.Uint16(buf[i+20 : i+22]))
		absolute := size - window + int64(i)
		commentEnd := absolute + zipDirectoryEndLen + int64(commentLen)
		if commentEnd > size {
			return zipDirectoryInfo{}, fmt.Errorf("zip: invalid comment length: %w", zip.ErrFormat)
		}
		// archive/zip selects this first non-truncated candidate. Reject the
		// archive instead of searching for an earlier EOCD that would make the
		// preflight and archive/zip inspect different central directories.
		if commentEnd != size {
			return zipDirectoryInfo{}, fmt.Errorf("zip: trailing data after end record: %w", zip.ErrFormat)
		}
		endIndex = i
		break
	}
	if endIndex < 0 {
		return zipDirectoryInfo{}, fmt.Errorf("zip: end of central directory not found: %w", zip.ErrFormat)
	}

	record := buf[endIndex : endIndex+zipDirectoryEndLen]
	if binary.LittleEndian.Uint16(record[4:6]) != 0 || binary.LittleEndian.Uint16(record[6:8]) != 0 {
		return zipDirectoryInfo{}, fmt.Errorf("zip: multi-disk archives are unsupported: %w", zip.ErrFormat)
	}
	entriesThisDisk := binary.LittleEndian.Uint16(record[8:10])
	entries := binary.LittleEndian.Uint16(record[10:12])
	directorySize := binary.LittleEndian.Uint32(record[12:16])
	directoryOffset := binary.LittleEndian.Uint32(record[16:20])
	needsZip64 := entriesThisDisk == math.MaxUint16 || entries == math.MaxUint16 ||
		directorySize == math.MaxUint16 || directorySize == math.MaxUint32 ||
		directoryOffset == math.MaxUint32
	if !needsZip64 {
		if entriesThisDisk != entries {
			return zipDirectoryInfo{}, fmt.Errorf("zip: inconsistent central directory entry count: %w", zip.ErrFormat)
		}
		if int(entries) > limits.maxEntries {
			return zipDirectoryInfo{}, fmt.Errorf("%w: %d entries exceeds limit %d", ErrArchiveLimit, entries, limits.maxEntries)
		}
	}

	info := zipDirectoryInfo{
		endOffset:  size - window + int64(endIndex),
		eocdOffset: size - window + int64(endIndex),
		offset:     uint64(directoryOffset),
		size:       uint64(directorySize),
		entries:    uint64(entries),
	}
	if !needsZip64 {
		return info, nil
	}
	return readZip64DirectoryInfo(reader, size, info, limits)
}

func readZip64DirectoryInfo(reader io.ReaderAt, size int64, info zipDirectoryInfo, limits resolvedZipLimits) (zipDirectoryInfo, error) {
	locatorOffset := info.eocdOffset - zipDirectory64LocLen
	if locatorOffset < 0 || size < zipDirectory64EndLen {
		return zipDirectoryInfo{}, fmt.Errorf("zip: ZIP64 locator missing: %w", zip.ErrFormat)
	}
	var locator [zipDirectory64LocLen]byte
	if _, err := reader.ReadAt(locator[:], locatorOffset); err != nil {
		return zipDirectoryInfo{}, err
	}
	if binary.LittleEndian.Uint32(locator[0:4]) != zipDirectory64LocSignature ||
		binary.LittleEndian.Uint32(locator[4:8]) != 0 ||
		binary.LittleEndian.Uint32(locator[16:20]) != 1 {
		return zipDirectoryInfo{}, fmt.Errorf("zip: invalid ZIP64 locator: %w", zip.ErrFormat)
	}
	recordOffset64 := binary.LittleEndian.Uint64(locator[8:16])
	recordOffset, record, ok := readZip64EndAt(reader, size, locatorOffset, recordOffset64)
	if !ok {
		return zipDirectoryInfo{}, fmt.Errorf("zip: invalid ZIP64 end record: %w", zip.ErrFormat)
	}
	if binary.LittleEndian.Uint32(record[16:20]) != 0 || binary.LittleEndian.Uint32(record[20:24]) != 0 {
		return zipDirectoryInfo{}, fmt.Errorf("zip: multi-disk ZIP64 archives are unsupported: %w", zip.ErrFormat)
	}
	entriesThisDisk := binary.LittleEndian.Uint64(record[24:32])
	entries := binary.LittleEndian.Uint64(record[32:40])
	if entriesThisDisk != entries {
		return zipDirectoryInfo{}, fmt.Errorf("zip: inconsistent ZIP64 entry count: %w", zip.ErrFormat)
	}
	if entries > uint64(limits.maxEntries) {
		return zipDirectoryInfo{}, fmt.Errorf("%w: %d entries exceeds limit %d", ErrArchiveLimit, entries, limits.maxEntries)
	}
	info.endOffset = recordOffset
	info.entries = entries
	info.size = binary.LittleEndian.Uint64(record[40:48])
	info.offset = binary.LittleEndian.Uint64(record[48:56])
	return info, nil
}

func readZip64EndAt(reader io.ReaderAt, size, locatorOffset int64, offset uint64) (int64, [zipDirectory64EndLen]byte, bool) {
	var record [zipDirectory64EndLen]byte
	if offset > math.MaxInt64 {
		return 0, record, false
	}
	recordOffset := int64(offset)
	if recordOffset < 0 || recordOffset > size-zipDirectory64EndLen || recordOffset > locatorOffset-zipDirectory64EndLen {
		return 0, record, false
	}
	if _, err := reader.ReadAt(record[:], recordOffset); err != nil {
		return 0, record, false
	}
	if binary.LittleEndian.Uint32(record[0:4]) != zipDirectory64EndSignature {
		return 0, record, false
	}
	recordSize := binary.LittleEndian.Uint64(record[4:12])
	if recordSize < zipDirectory64EndLen-12 || recordSize > uint64(math.MaxInt64-12) {
		return 0, record, false
	}
	recordTotal := int64(recordSize) + 12
	if recordOffset > math.MaxInt64-recordTotal || recordOffset+recordTotal != locatorOffset || recordOffset+recordTotal > size {
		return 0, record, false
	}
	return recordOffset, record, true
}

func completeZipDirectoryHeaderAt(reader io.ReaderAt, size, offset int64) int64 {
	return completeZipDirectoryHeaderWithin(reader, size, offset, size)
}

func completeZipDirectoryHeaderWithin(reader io.ReaderAt, archiveSize, offset, end int64) int64 {
	if offset < 0 || end < offset || end > archiveSize || offset > end-zipDirectoryHeaderLen {
		return 0
	}
	var header [zipDirectoryHeaderLen]byte
	if _, err := reader.ReadAt(header[:], offset); err != nil {
		return 0
	}
	if binary.LittleEndian.Uint32(header[0:4]) != zipDirectoryHeaderSignature {
		return 0
	}
	filenameLen := int64(binary.LittleEndian.Uint16(header[28:30]))
	extraLen := int64(binary.LittleEndian.Uint16(header[30:32]))
	commentLen := int64(binary.LittleEndian.Uint16(header[32:34]))
	entryLen := int64(zipDirectoryHeaderLen) + filenameLen + extraLen + commentLen
	if entryLen > end-offset {
		return 0
	}

	needUncompressedSize := binary.LittleEndian.Uint32(header[24:28]) == math.MaxUint32
	needCompressedSize := binary.LittleEndian.Uint32(header[20:24]) == math.MaxUint32
	needHeaderOffset := binary.LittleEndian.Uint32(header[42:46]) == math.MaxUint32
	if !needUncompressedSize && !needCompressedSize && !needHeaderOffset {
		return entryLen
	}
	extra := make([]byte, int(extraLen))
	if extraLen > 0 {
		if _, err := reader.ReadAt(extra, offset+zipDirectoryHeaderLen+filenameLen); err != nil {
			return 0
		}
	}
	for len(extra) >= 4 {
		tag := binary.LittleEndian.Uint16(extra[0:2])
		fieldLen := int(binary.LittleEndian.Uint16(extra[2:4]))
		extra = extra[4:]
		if len(extra) < fieldLen {
			break
		}
		field := extra[:fieldLen]
		extra = extra[fieldLen:]
		if tag != 0x0001 {
			continue
		}
		if needUncompressedSize {
			if len(field) < 8 {
				return 0
			}
			field = field[8:]
			needUncompressedSize = false
		}
		if needCompressedSize {
			if len(field) < 8 {
				return 0
			}
			field = field[8:]
			needCompressedSize = false
		}
		if needHeaderOffset {
			if len(field) < 8 {
				return 0
			}
			needHeaderOffset = false
		}
	}
	// archive/zip tolerates a missing ZIP64 uncompressed size for historical
	// compatibility, but requires compressed size and local-header offset.
	if needCompressedSize || needHeaderOffset {
		return 0
	}
	return entryLen
}

func scanZipCentralDirectory(reader io.ReaderAt, size, start, end int64, declaredEntries uint64, limits resolvedZipLimits) error {
	if start < 0 || end < start || end > size {
		return fmt.Errorf("zip: invalid central directory bounds: %w", zip.ErrFormat)
	}
	offset := start
	var metadataBytes int64
	var entries uint64
	for offset < end {
		entryLen := completeZipDirectoryHeaderWithin(reader, size, offset, end)
		if entryLen == 0 {
			return fmt.Errorf("zip: invalid central directory entry: %w", zip.ErrFormat)
		}
		if entryLen > limits.maxCentralDirBytes-metadataBytes {
			return fmt.Errorf("%w: central directory metadata exceeds limit %d", ErrArchiveLimit, limits.maxCentralDirBytes)
		}
		metadataBytes += entryLen
		entries++
		if entries > uint64(limits.maxEntries) {
			return fmt.Errorf("%w: central directory contains more than %d entries", ErrArchiveLimit, limits.maxEntries)
		}
		offset += entryLen
	}
	if offset != end {
		return fmt.Errorf("zip: central directory scan did not reach its declared end: %w", zip.ErrFormat)
	}
	if entries != declaredEntries {
		return fmt.Errorf("zip: central directory declares %d entries but contains %d: %w", declaredEntries, entries, zip.ErrFormat)
	}
	return nil
}

func readZipArchiveBody(reader io.Reader, contentLength int64) ([]byte, error) {
	if reader == nil {
		return nil, fmt.Errorf("zip: HTTP response has no body")
	}
	limit := currentZipLimits().maxArchiveBytes
	if contentLength > limit {
		return nil, fmt.Errorf("%w: response length %d exceeds limit %d", ErrArchiveLimit, contentLength, limit)
	}
	body, err := io.ReadAll(io.LimitReader(reader, zipLimitWithSentinel(limit)))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: response exceeds limit %d", ErrArchiveLimit, limit)
	}
	return body, nil
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
