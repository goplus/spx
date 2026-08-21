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

	"github.com/goplus/spx/v3/internal/runtimebundle"
)

// validateSourceLimits checks every bound that can be decided from the
// caller-provided source metadata. It deliberately runs before prepareSources
// hashes any ReaderAt, so a source that cannot possibly fit in a payload is
// rejected without touching its body.
func validateSourceLimits(cfg BuildConfig, sources []FileSource) error {
	// The manifest is an outer ZIP entry in addition to the caller's sources.
	if len(sources) >= runtimebundle.MaxEntries {
		return fmt.Errorf("runtimepayload: %d entries exceeds limit %d: %w", len(sources)+1, runtimebundle.MaxEntries, runtimebundle.ErrArchiveLimit)
	}

	ordered := append([]FileSource(nil), sources...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })
	seen := make(map[string]struct{}, len(ordered))
	entries := make([]runtimebundle.Entry, 0, len(ordered))
	var total int64
	placeholder := strings.Repeat("0", sha256.Size*2)
	for _, source := range ordered {
		if err := validateEntryName(source.Name); err != nil {
			return err
		}
		if source.Name == ManifestPath {
			return fmt.Errorf("runtimepayload: %q is reserved for the top-level manifest", source.Name)
		}
		if _, duplicate := seen[source.Name]; duplicate {
			return fmt.Errorf("runtimepayload: duplicate entry %q", source.Name)
		}
		seen[source.Name] = struct{}{}
		if source.ReaderAt == nil {
			return fmt.Errorf("runtimepayload: entry %q has a nil ReaderAt", source.Name)
		}
		if source.Size < 0 {
			return fmt.Errorf("runtimepayload: entry %q has negative size %d", source.Name, source.Size)
		}
		if source.Size > runtimebundle.MaxEntrySize {
			return fmt.Errorf("runtimepayload: entry %q size %d exceeds limit %d: %w", source.Name, source.Size, runtimebundle.MaxEntrySize, runtimebundle.ErrArchiveLimit)
		}
		if source.Size > runtimebundle.MaxTotalSize-total {
			return fmt.Errorf("runtimepayload: total size exceeds limit %d: %w", runtimebundle.MaxTotalSize, runtimebundle.ErrArchiveLimit)
		}
		if len(source.Name) > zipMaxNameBytes {
			return fmt.Errorf("runtimepayload: entry %q name is too long", source.Name)
		}
		total += source.Size
		entries = append(entries, runtimebundle.Entry{
			Name: source.Name, Mode: uint32(canonicalFileMode(source.Mode)), Size: source.Size, SHA256: placeholder,
		})
	}

	manifest := Manifest{
		Schema: SchemaV1, Protocol: ProtocolV1, SPX: cfg.SPX, Target: cfg.Target,
		Engine: cfg.Engine, Bridge: cfg.Bridge, Project: cfg.Project, Entries: entries,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("runtimepayload: encode manifest: %w", err)
	}
	return validatePreparedPayloadLimits(entries, int64(len(manifestBytes)))
}

// validatePreparedPayloadLimits validates the complete outer payload after
// the manifest is encoded. The same check is also used by the source preflight
// with placeholder digests; SHA-256 values are fixed-width, so that dry-run
// manifest has the exact same size as the eventual manifest.
func validatePreparedPayloadLimits(entries []runtimebundle.Entry, manifestSize int64) error {
	if len(entries) >= runtimebundle.MaxEntries {
		return fmt.Errorf("runtimepayload: %d entries exceeds limit %d: %w", len(entries)+1, runtimebundle.MaxEntries, runtimebundle.ErrArchiveLimit)
	}
	if manifestSize < 0 {
		return fmt.Errorf("runtimepayload: negative manifest size %d", manifestSize)
	}
	if manifestSize > maxPayloadManifestBytes {
		return fmt.Errorf("runtimepayload: manifest size %d exceeds limit %d: %w", manifestSize, maxPayloadManifestBytes, runtimebundle.ErrArchiveLimit)
	}

	var total int64
	for _, entry := range entries {
		if entry.Size < 0 {
			return fmt.Errorf("runtimepayload: entry %q has negative size %d", entry.Name, entry.Size)
		}
		if entry.Size > runtimebundle.MaxEntrySize {
			return fmt.Errorf("runtimepayload: entry %q size %d exceeds limit %d: %w", entry.Name, entry.Size, runtimebundle.MaxEntrySize, runtimebundle.ErrArchiveLimit)
		}
		if entry.Size > runtimebundle.MaxTotalSize-total {
			return fmt.Errorf("runtimepayload: total size exceeds limit %d: %w", runtimebundle.MaxTotalSize, runtimebundle.ErrArchiveLimit)
		}
		total += entry.Size
	}
	if manifestSize > runtimebundle.MaxEntrySize {
		return fmt.Errorf("runtimepayload: manifest size %d exceeds limit %d: %w", manifestSize, runtimebundle.MaxEntrySize, runtimebundle.ErrArchiveLimit)
	}
	if manifestSize > runtimebundle.MaxTotalSize-total {
		return fmt.Errorf("runtimepayload: total size exceeds limit %d: %w", runtimebundle.MaxTotalSize, runtimebundle.ErrArchiveLimit)
	}

	archiveEntries := make([]runtimebundle.Entry, 0, len(entries)+1)
	archiveEntries = append(archiveEntries, entries...)
	archiveEntries = append(archiveEntries, runtimebundle.Entry{Name: ManifestPath, Size: manifestSize})
	archiveSize, err := estimatePayloadArchiveSize(archiveEntries)
	if err != nil {
		return err
	}
	if archiveSize > runtimebundle.MaxArchiveBytes {
		return fmt.Errorf("runtimepayload: archive size %d exceeds limit %d: %w", archiveSize, runtimebundle.MaxArchiveBytes, runtimebundle.ErrArchiveLimit)
	}
	return nil
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
