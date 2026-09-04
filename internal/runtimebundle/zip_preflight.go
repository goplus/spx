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
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

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
	// endOffset is the offset archive/zip uses to derive the archive base.
	endOffset  int64
	eocdOffset int64
	offset     uint64
	size       uint64
	entries    uint64
}

// preflightZipArchive bounds archive/zip's central-directory work before
// zip.NewReader can allocate one File and its metadata per directory header.
func preflightZipArchive(reader io.ReaderAt, size int64, limits Limits) error {
	if reader == nil || isNilReaderAt(reader) {
		return fmt.Errorf("zip: nil reader: %w", zip.ErrFormat)
	}
	if size < 0 {
		return fmt.Errorf("zip: negative archive size: %w", zip.ErrFormat)
	}
	if size > limits.MaxArchiveBytes {
		return fmt.Errorf("%w: archive size %d exceeds limit %d", ErrArchiveLimit, size, limits.MaxArchiveBytes)
	}

	directory, err := readZipDirectoryInfo(reader, size, limits)
	if err != nil {
		return err
	}
	if directory.size > uint64(limits.MaxCentralDirectoryBytes) {
		return fmt.Errorf("%w: central directory size %d exceeds limit %d", ErrArchiveLimit, directory.size, limits.MaxCentralDirectoryBytes)
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
	// Match archive/zip's compatibility probe for archives that declare an
	// absolute central-directory offset despite a non-zero computed base.
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
	// archive/zip reads headers until the first invalid signature rather than
	// stopping at the declared byte count. Do not let it parse a header that the
	// bounded scan omitted.
	if centralEnd < directory.endOffset && completeZipDirectoryHeaderAt(reader, size, centralEnd) > 0 {
		return fmt.Errorf("zip: central directory extends beyond its declared size: %w", zip.ErrFormat)
	}
	return nil
}

func readZipDirectoryInfo(reader io.ReaderAt, size int64, limits Limits) (zipDirectoryInfo, error) {
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

	endIndex := -1
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
		// archive/zip selects this first non-truncated candidate. Requiring the
		// comment to reach EOF avoids preflighting a different earlier record.
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
	// archive/zip treats these boundary values as a ZIP64 probe, then falls
	// back to the classic EOCD values when no locator is present. In
	// particular, 0xffff is a valid value for the 32-bit directory-size field.
	mayHaveZip64 := entries == math.MaxUint16 || directorySize == math.MaxUint16 ||
		directoryOffset == math.MaxUint32
	if mayHaveZip64 {
		zip64Info, found, err := readZip64DirectoryInfo(reader, size, info, limits)
		if err != nil {
			return zipDirectoryInfo{}, err
		}
		if found {
			return zip64Info, nil
		}
	}
	if entriesThisDisk != entries {
		return zipDirectoryInfo{}, fmt.Errorf("zip: inconsistent central directory entry count: %w", zip.ErrFormat)
	}
	if int(entries) > limits.MaxEntries {
		return zipDirectoryInfo{}, fmt.Errorf("%w: %d entries exceeds limit %d", ErrArchiveLimit, entries, limits.MaxEntries)
	}
	return info, nil
}

func readZip64DirectoryInfo(reader io.ReaderAt, size int64, info zipDirectoryInfo, limits Limits) (zipDirectoryInfo, bool, error) {
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
	if entries > uint64(limits.MaxEntries) {
		return zipDirectoryInfo{}, true, fmt.Errorf("%w: %d entries exceeds limit %d", ErrArchiveLimit, entries, limits.MaxEntries)
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
	// archive/zip tolerates a missing ZIP64 uncompressed size for compatibility,
	// but requires the compressed size and local-header offset.
	if needCompressedSize || needHeaderOffset {
		return 0
	}
	return entryLen
}

func scanZipCentralDirectory(reader io.ReaderAt, size, start, end int64, declaredEntries uint64, limits Limits) error {
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
		if entryLen > limits.MaxCentralDirectoryBytes-metadataBytes {
			return fmt.Errorf("%w: central directory metadata exceeds limit %d", ErrArchiveLimit, limits.MaxCentralDirectoryBytes)
		}
		metadataBytes += entryLen
		entries++
		if entries > uint64(limits.MaxEntries) {
			return fmt.Errorf("%w: central directory contains more than %d entries", ErrArchiveLimit, limits.MaxEntries)
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
