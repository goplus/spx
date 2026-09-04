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
	"sort"
	"strings"
	"sync/atomic"
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
	endOffset  int64
	eocdOffset int64
	offset     uint64
	size       uint64
	entries    uint64
}

// preflightZipArchive bounds work before zip.NewReader allocates entries.
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
	// Header parsing differs across Go releases; reject an alternate base.
	if baseOffset > 0 && hasZipDirectoryHeader(reader, size, centralOffset) {
		return fmt.Errorf("zip: ambiguous central directory offset: %w", zip.ErrFormat)
	}
	return scanZipCentralDirectory(reader, size, start, directory.endOffset, directory.entries, limits)
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
		// Do not let preflight and archive/zip select different end records.
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
	info := zipDirectoryInfo{
		endOffset:  size - window + int64(endIndex),
		eocdOffset: size - window + int64(endIndex),
		offset:     uint64(directoryOffset),
		size:       uint64(directorySize),
		entries:    uint64(entries),
	}
	standardZip64 := entries == math.MaxUint16 || directorySize == math.MaxUint32 || directoryOffset == math.MaxUint32
	// Go 1.25 also probes 0xffff directory sizes; later releases do not.
	if standardZip64 || directorySize == math.MaxUint16 {
		zip64Info, found, err := readZip64DirectoryInfo(reader, size, info, limits)
		if err != nil {
			return zipDirectoryInfo{}, err
		}
		if found {
			if !standardZip64 {
				return zipDirectoryInfo{}, fmt.Errorf("zip: ambiguous ZIP64 directory size: %w", zip.ErrFormat)
			}
			return zip64Info, nil
		}
	}
	if entriesThisDisk != entries {
		return zipDirectoryInfo{}, fmt.Errorf("zip: inconsistent central directory entry count: %w", zip.ErrFormat)
	}
	if int(entries) > limits.maxEntries {
		return zipDirectoryInfo{}, fmt.Errorf("%w: %d entries exceeds limit %d", ErrArchiveLimit, entries, limits.maxEntries)
	}
	return info, nil
}

func readZip64DirectoryInfo(reader io.ReaderAt, size int64, info zipDirectoryInfo, limits resolvedZipLimits) (zipDirectoryInfo, bool, error) {
	locatorOffset := info.eocdOffset - zipDirectory64LocLen
	if locatorOffset < 0 || size < zipDirectory64EndLen {
		return zipDirectoryInfo{}, false, nil
	}
	var locator [zipDirectory64LocLen]byte
	if _, err := reader.ReadAt(locator[:], locatorOffset); err != nil {
		return zipDirectoryInfo{}, false, err
	}
	if binary.LittleEndian.Uint32(locator[0:4]) != zipDirectory64LocSignature ||
		binary.LittleEndian.Uint32(locator[4:8]) != 0 ||
		binary.LittleEndian.Uint32(locator[16:20]) != 1 {
		return zipDirectoryInfo{}, false, nil
	}
	recordOffset64 := binary.LittleEndian.Uint64(locator[8:16])
	recordOffset, record, ok := readZip64EndAt(reader, size, locatorOffset, recordOffset64)
	if !ok {
		return zipDirectoryInfo{}, true, fmt.Errorf("zip: invalid ZIP64 end record: %w", zip.ErrFormat)
	}
	if binary.LittleEndian.Uint32(record[16:20]) != 0 || binary.LittleEndian.Uint32(record[20:24]) != 0 {
		return zipDirectoryInfo{}, true, fmt.Errorf("zip: multi-disk ZIP64 archives are unsupported: %w", zip.ErrFormat)
	}
	entriesThisDisk := binary.LittleEndian.Uint64(record[24:32])
	entries := binary.LittleEndian.Uint64(record[32:40])
	if entriesThisDisk != entries {
		return zipDirectoryInfo{}, true, fmt.Errorf("zip: inconsistent ZIP64 entry count: %w", zip.ErrFormat)
	}
	if entries > uint64(limits.maxEntries) {
		return zipDirectoryInfo{}, true, fmt.Errorf("%w: %d entries exceeds limit %d", ErrArchiveLimit, entries, limits.maxEntries)
	}
	info.endOffset = recordOffset
	info.entries = entries
	info.size = binary.LittleEndian.Uint64(record[40:48])
	info.offset = binary.LittleEndian.Uint64(record[48:56])
	return info, true, nil
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

func hasZipDirectoryHeader(reader io.ReaderAt, size, offset int64) bool {
	if offset < 0 || offset > size-4 {
		return false
	}
	var signature [4]byte
	_, err := reader.ReadAt(signature[:], offset)
	return err == nil && binary.LittleEndian.Uint32(signature[:]) == zipDirectoryHeaderSignature
}

func zipDirectoryEntryLen(reader io.ReaderAt, archiveSize, offset, end int64) int64 {
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
		entryLen := zipDirectoryEntryLen(reader, size, offset, end)
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
