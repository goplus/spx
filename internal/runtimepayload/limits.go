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

package runtimepayload

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/goplus/spx/v4/internal/runtimebundle"
)

// preparePayloadSources checks every bound that can be decided from source
// metadata before BuildTo hashes any ReaderAt. The returned sources are already
// ordered and canonicalized, so hashing does not repeat their preparation.
func preparePayloadSources(cfg BuildConfig, sources []FileSource) ([]preparedSource, error) {
	// The manifest is an outer ZIP entry in addition to the caller's sources.
	if len(sources) >= runtimebundle.MaxEntries {
		return nil, fmt.Errorf("runtimepayload: %d entries exceeds limit %d: %w", len(sources)+1, runtimebundle.MaxEntries, runtimebundle.ErrArchiveLimit)
	}
	prepared, err := prepareSources(sources, ManifestPath)
	if err != nil {
		return nil, err
	}
	entries := make([]runtimebundle.Entry, len(prepared), len(prepared)+1)
	placeholder := strings.Repeat("0", sha256.Size*2)
	var total int64
	for i, file := range prepared {
		// Reject oversized sources before allocating the encoded manifest.
		if file.entry.Size > runtimebundle.MaxEntrySize {
			return nil, fmt.Errorf("runtimepayload: entry %q size %d exceeds limit %d: %w", file.entry.Name, file.entry.Size, runtimebundle.MaxEntrySize, runtimebundle.ErrArchiveLimit)
		}
		if file.entry.Size > runtimebundle.MaxTotalSize-total {
			return nil, fmt.Errorf("runtimepayload: total size exceeds limit %d: %w", runtimebundle.MaxTotalSize, runtimebundle.ErrArchiveLimit)
		}
		total += file.entry.Size
		if len(file.entry.Name) > zipMaxNameBytes {
			return nil, fmt.Errorf("runtimepayload: entry %q name is too long", file.entry.Name)
		}
		entries[i] = file.entry
		entries[i].SHA256 = placeholder
	}
	manifestBytes, err := json.Marshal(newManifest(cfg, entries))
	if err != nil {
		return nil, fmt.Errorf("runtimepayload: encode manifest: %w", err)
	}
	// SHA-256 values are fixed-width, so the placeholder manifest has the same
	// size as the final manifest after hashing the sources.
	manifestSize := int64(len(manifestBytes))
	if manifestSize > maxPayloadManifestBytes {
		return nil, fmt.Errorf("runtimepayload: manifest size %d exceeds limit %d: %w", manifestSize, maxPayloadManifestBytes, runtimebundle.ErrArchiveLimit)
	}
	if manifestSize > runtimebundle.MaxTotalSize-total {
		return nil, fmt.Errorf("runtimepayload: total size exceeds limit %d: %w", runtimebundle.MaxTotalSize, runtimebundle.ErrArchiveLimit)
	}

	entries = append(entries, runtimebundle.Entry{Name: ManifestPath, Size: manifestSize})
	archiveSize, err := estimatePayloadArchiveSize(entries)
	if err != nil {
		return nil, err
	}
	if archiveSize > runtimebundle.MaxArchiveBytes {
		return nil, fmt.Errorf("runtimepayload: archive size %d exceeds limit %d: %w", archiveSize, runtimebundle.MaxArchiveBytes, runtimebundle.ErrArchiveLimit)
	}
	return prepared, nil
}

// estimatePayloadArchiveSize computes the exact size emitted by BuildTo's
// canonical archive/zip sequence from entry names and declared sizes. No
// source bytes are read. This also accounts for ZIP64 central-directory
// records, should the aggregate metadata push an offset over the 32-bit ZIP
// boundary.
func estimatePayloadArchiveSize(entries []runtimebundle.Entry) (int64, error) {
	ordered := append([]runtimebundle.Entry(nil), entries...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })
	var offset, centralSize int64
	for _, entry := range ordered {
		if len(entry.Name) > zipMaxNameBytes {
			return 0, fmt.Errorf("runtimepayload: entry %q name is too long", entry.Name)
		}
		if entry.Size < 0 {
			return 0, fmt.Errorf("runtimepayload: entry %q has negative size %d", entry.Name, entry.Size)
		}
		nameBytes := int64(len(entry.Name))
		localSize, err := addPayloadSize(zipLocalHeaderBytes+zipModTimeExtraBytes, nameBytes)
		if err != nil {
			return 0, fmt.Errorf("runtimepayload: payload archive size overflow: %w", err)
		}
		descriptorSize := zipDataDescriptorBytes
		centralExtraSize := zipModTimeExtraBytes
		zip64Entry := uint64(entry.Size) >= zip32Max
		if zip64Entry {
			descriptorSize = zipDataDescriptor64Bytes
		}
		if zip64Entry || uint64(offset) >= zip32Max {
			centralExtraSize += zip64CentralExtraBytes
		}
		centralEntrySize, err := addPayloadSize(zipCentralHeaderBytes+centralExtraSize, nameBytes)
		if err != nil {
			return 0, fmt.Errorf("runtimepayload: payload archive size overflow: %w", err)
		}
		localSize, err = addPayloadSize(localSize, entry.Size)
		if err != nil {
			return 0, fmt.Errorf("runtimepayload: payload archive size overflow: %w", err)
		}
		localSize, err = addPayloadSize(localSize, descriptorSize)
		if err != nil {
			return 0, fmt.Errorf("runtimepayload: payload archive size overflow: %w", err)
		}
		offset, err = addPayloadSize(offset, localSize)
		if err != nil {
			return 0, fmt.Errorf("runtimepayload: payload archive size overflow: %w", err)
		}
		centralSize, err = addPayloadSize(centralSize, centralEntrySize)
		if err != nil {
			return 0, fmt.Errorf("runtimepayload: payload archive size overflow: %w", err)
		}
	}
	zip64 := len(ordered) >= 1<<16 || uint64(centralSize) >= zip32Max || uint64(offset) >= zip32Max
	archiveSize, err := addPayloadSize(offset, centralSize)
	if err != nil {
		return 0, fmt.Errorf("runtimepayload: payload archive size overflow: %w", err)
	}
	archiveSize, err = addPayloadSize(archiveSize, zipEndBytes)
	if err != nil {
		return 0, fmt.Errorf("runtimepayload: payload archive size overflow: %w", err)
	}
	if zip64 {
		archiveSize, err = addPayloadSize(archiveSize, zip64EndBytes)
		if err != nil {
			return 0, fmt.Errorf("runtimepayload: payload archive size overflow: %w", err)
		}
	}
	return archiveSize, nil
}

func addPayloadSize(first, second int64) (int64, error) {
	const maxInt64 = int64(^uint64(0) >> 1)
	if first < 0 || second < 0 || first > maxInt64-second {
		return 0, errors.New("size exceeds MaxInt64")
	}
	return first + second, nil
}
