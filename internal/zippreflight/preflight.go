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

// Package zippreflight bounds ZIP metadata before archive/zip allocates entries.
package zippreflight

import (
	"archive/zip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
)

const (
	directoryHeaderSignature = 0x02014b50
	directoryEndSignature    = 0x06054b50
	directory64EndSignature  = 0x06064b50
	directory64LocSignature  = 0x07064b50
	directoryHeaderLen       = 46
	directoryEndLen          = 22
	directory64EndLen        = 56
	directory64LocLen        = 20
	directorySearchWindow    = 65 << 10
)

// Limits bounds ZIP input and metadata; every field must be positive.
type Limits struct {
	MaxArchiveBytes          int64
	MaxCentralDirectoryBytes int64
	MaxEntries               int
}

type limitError struct {
	message string
}

func (e *limitError) Error() string {
	return e.message
}

func limitErrorf(format string, args ...any) error {
	return &limitError{message: fmt.Sprintf(format, args...)}
}

// IsLimit reports whether err identifies a preflight resource limit.
func IsLimit(err error) bool {
	var target *limitError
	return errors.As(err, &target)
}

type directoryInfo struct {
	endOffset  int64
	eocdOffset int64
	offset     uint64
	size       uint64
	entries    uint64
}

// Check validates central-directory bounds before zip.NewReader is called.
func Check(reader io.ReaderAt, size int64, limits Limits) error {
	if reader == nil || isNilReaderAt(reader) {
		return fmt.Errorf("zip: nil reader: %w", zip.ErrFormat)
	}
	if err := validateLimits(limits); err != nil {
		return err
	}
	if size < 0 {
		return fmt.Errorf("zip: negative archive size: %w", zip.ErrFormat)
	}
	if size > limits.MaxArchiveBytes {
		return limitErrorf("archive size %d exceeds limit %d", size, limits.MaxArchiveBytes)
	}

	directory, err := readDirectoryInfo(reader, size, limits)
	if err != nil {
		return err
	}
	if directory.size > uint64(limits.MaxCentralDirectoryBytes) {
		return limitErrorf("central directory size %d exceeds limit %d", directory.size, limits.MaxCentralDirectoryBytes)
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
	// Header parsing differs across Go releases; reject an alternate base.
	if start-centralOffset > 0 && hasDirectoryHeader(reader, size, centralOffset) {
		return fmt.Errorf("zip: ambiguous central directory offset: %w", zip.ErrFormat)
	}
	return scanCentralDirectory(reader, size, start, directory.endOffset, directory.entries, limits)
}

func validateLimits(limits Limits) error {
	if limits.MaxArchiveBytes <= 0 || limits.MaxCentralDirectoryBytes <= 0 || limits.MaxEntries <= 0 {
		return fmt.Errorf("zip: preflight limits must be positive")
	}
	return nil
}

func isNilReaderAt(reader io.ReaderAt) bool {
	value := reflect.ValueOf(reader)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func readDirectoryInfo(reader io.ReaderAt, size int64, limits Limits) (directoryInfo, error) {
	if size < directoryEndLen {
		return directoryInfo{}, fmt.Errorf("zip: archive too small: %w", zip.ErrFormat)
	}
	window := int64(directorySearchWindow)
	if window > size {
		window = size
	}
	buf := make([]byte, int(window))
	if _, err := reader.ReadAt(buf, size-window); err != nil {
		return directoryInfo{}, err
	}

	endIndex := -1
	for i := len(buf) - directoryEndLen; i >= 0; i-- {
		if binary.LittleEndian.Uint32(buf[i:i+4]) != directoryEndSignature {
			continue
		}
		commentLen := int(binary.LittleEndian.Uint16(buf[i+20 : i+22]))
		absolute := size - window + int64(i)
		commentEnd := absolute + directoryEndLen + int64(commentLen)
		if commentEnd != size {
			return directoryInfo{}, fmt.Errorf("zip: invalid end record: %w", zip.ErrFormat)
		}
		endIndex = i
		break
	}
	if endIndex < 0 {
		return directoryInfo{}, fmt.Errorf("zip: end of central directory not found: %w", zip.ErrFormat)
	}

	record := buf[endIndex : endIndex+directoryEndLen]
	if binary.LittleEndian.Uint16(record[4:6]) != 0 || binary.LittleEndian.Uint16(record[6:8]) != 0 {
		return directoryInfo{}, fmt.Errorf("zip: multi-disk archives are unsupported: %w", zip.ErrFormat)
	}
	entriesThisDisk := binary.LittleEndian.Uint16(record[8:10])
	entries := binary.LittleEndian.Uint16(record[10:12])
	directorySize := binary.LittleEndian.Uint32(record[12:16])
	directoryOffset := binary.LittleEndian.Uint32(record[16:20])
	info := directoryInfo{
		endOffset:  size - window + int64(endIndex),
		eocdOffset: size - window + int64(endIndex),
		offset:     uint64(directoryOffset),
		size:       uint64(directorySize),
		entries:    uint64(entries),
	}
	unambiguousZip64 := entries == math.MaxUint16 || directoryOffset == math.MaxUint32
	// Go releases disagree on size-only ZIP64 probes.
	if unambiguousZip64 || directorySize == math.MaxUint16 || directorySize == math.MaxUint32 {
		zip64Info, found, err := readZip64DirectoryInfo(reader, size, info, limits)
		if err != nil {
			return directoryInfo{}, err
		}
		if found {
			if !unambiguousZip64 {
				return directoryInfo{}, fmt.Errorf("zip: ambiguous ZIP64 directory size: %w", zip.ErrFormat)
			}
			return zip64Info, nil
		}
	}
	if entriesThisDisk != entries {
		return directoryInfo{}, fmt.Errorf("zip: inconsistent central directory entry count: %w", zip.ErrFormat)
	}
	if uint64(entries) > uint64(limits.MaxEntries) {
		return directoryInfo{}, limitErrorf("%d entries exceeds limit %d", entries, limits.MaxEntries)
	}
	return info, nil
}

func readZip64DirectoryInfo(reader io.ReaderAt, size int64, info directoryInfo, limits Limits) (directoryInfo, bool, error) {
	locatorOffset := info.eocdOffset - directory64LocLen
	if locatorOffset < 0 || size < directory64EndLen {
		return directoryInfo{}, false, nil
	}
	var locator [directory64LocLen]byte
	if _, err := reader.ReadAt(locator[:], locatorOffset); err != nil {
		return directoryInfo{}, false, err
	}
	if binary.LittleEndian.Uint32(locator[0:4]) != directory64LocSignature ||
		binary.LittleEndian.Uint32(locator[4:8]) != 0 ||
		binary.LittleEndian.Uint32(locator[16:20]) != 1 {
		return directoryInfo{}, false, nil
	}
	recordOffset64 := binary.LittleEndian.Uint64(locator[8:16])
	recordOffset, record, ok := readZip64EndAt(reader, size, locatorOffset, recordOffset64)
	if !ok {
		return directoryInfo{}, true, fmt.Errorf("zip: invalid ZIP64 end record: %w", zip.ErrFormat)
	}
	if binary.LittleEndian.Uint32(record[16:20]) != 0 || binary.LittleEndian.Uint32(record[20:24]) != 0 {
		return directoryInfo{}, true, fmt.Errorf("zip: multi-disk ZIP64 archives are unsupported: %w", zip.ErrFormat)
	}
	entriesThisDisk := binary.LittleEndian.Uint64(record[24:32])
	entries := binary.LittleEndian.Uint64(record[32:40])
	if entriesThisDisk != entries {
		return directoryInfo{}, true, fmt.Errorf("zip: inconsistent ZIP64 entry count: %w", zip.ErrFormat)
	}
	if entries > uint64(limits.MaxEntries) {
		return directoryInfo{}, true, limitErrorf("%d entries exceeds limit %d", entries, limits.MaxEntries)
	}
	info.endOffset = recordOffset
	info.entries = entries
	info.size = binary.LittleEndian.Uint64(record[40:48])
	info.offset = binary.LittleEndian.Uint64(record[48:56])
	return info, true, nil
}

func readZip64EndAt(reader io.ReaderAt, size, locatorOffset int64, offset uint64) (int64, [directory64EndLen]byte, bool) {
	var record [directory64EndLen]byte
	if offset > math.MaxInt64 {
		return 0, record, false
	}
	recordOffset := int64(offset)
	if recordOffset < 0 || recordOffset > size-directory64EndLen || recordOffset > locatorOffset-directory64EndLen {
		return 0, record, false
	}
	if _, err := reader.ReadAt(record[:], recordOffset); err != nil {
		return 0, record, false
	}
	if binary.LittleEndian.Uint32(record[0:4]) != directory64EndSignature {
		return 0, record, false
	}
	recordSize := binary.LittleEndian.Uint64(record[4:12])
	if recordSize < directory64EndLen-12 || recordSize > uint64(math.MaxInt64-12) {
		return 0, record, false
	}
	recordTotal := int64(recordSize) + 12
	if recordOffset > math.MaxInt64-recordTotal || recordOffset+recordTotal != locatorOffset || recordOffset+recordTotal > size {
		return 0, record, false
	}
	return recordOffset, record, true
}

func hasDirectoryHeader(reader io.ReaderAt, size, offset int64) bool {
	if offset < 0 || offset > size-4 {
		return false
	}
	var signature [4]byte
	_, err := reader.ReadAt(signature[:], offset)
	return err == nil && binary.LittleEndian.Uint32(signature[:]) == directoryHeaderSignature
}

func directoryEntryLen(reader io.ReaderAt, archiveSize, offset, end int64) int64 {
	if offset < 0 || end < offset || end > archiveSize || offset > end-directoryHeaderLen {
		return 0
	}
	var header [directoryHeaderLen]byte
	if _, err := reader.ReadAt(header[:], offset); err != nil {
		return 0
	}
	if binary.LittleEndian.Uint32(header[0:4]) != directoryHeaderSignature {
		return 0
	}
	filenameLen := int64(binary.LittleEndian.Uint16(header[28:30]))
	extraLen := int64(binary.LittleEndian.Uint16(header[30:32]))
	commentLen := int64(binary.LittleEndian.Uint16(header[32:34]))
	entryLen := int64(directoryHeaderLen) + filenameLen + extraLen + commentLen
	if entryLen > end-offset {
		return 0
	}
	return entryLen
}

func scanCentralDirectory(reader io.ReaderAt, size, start, end int64, declaredEntries uint64, limits Limits) error {
	if start < 0 || end < start || end > size {
		return fmt.Errorf("zip: invalid central directory bounds: %w", zip.ErrFormat)
	}
	offset := start
	var metadataBytes int64
	var entries uint64
	for offset < end {
		entryLen := directoryEntryLen(reader, size, offset, end)
		if entryLen == 0 {
			return fmt.Errorf("zip: invalid central directory entry: %w", zip.ErrFormat)
		}
		if entryLen > limits.MaxCentralDirectoryBytes-metadataBytes {
			return limitErrorf("central directory metadata exceeds limit %d", limits.MaxCentralDirectoryBytes)
		}
		metadataBytes += entryLen
		entries++
		if entries > uint64(limits.MaxEntries) {
			return limitErrorf("central directory contains more than %d entries", limits.MaxEntries)
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
