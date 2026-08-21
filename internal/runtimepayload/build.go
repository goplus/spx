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
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"

	"github.com/goplus/spx/v3/internal/runtimebundle"
)

// BuildTo writes canonical payload bytes to dst and returns the full payload
// and manifest SHA-256 values. An error may leave a partial payload in dst.
func BuildTo(dst io.Writer, cfg BuildConfig, sources []FileSource) (payloadSHA256, manifestSHA256 string, err error) {
	if dst == nil {
		return "", "", errors.New("runtimepayload: nil payload writer")
	}
	if err := validateIdentity(cfg); err != nil {
		return "", "", err
	}
	if err := validateSourceLimits(cfg, sources); err != nil {
		return "", "", err
	}
	prepared, err := prepareSources(sources, ManifestPath)
	if err != nil {
		return "", "", err
	}
	entries := make([]runtimebundle.Entry, len(prepared))
	for i := range prepared {
		entries[i] = prepared[i].entry
	}
	if err := validateComponentClaims(cfg, prepared, entries); err != nil {
		return "", "", err
	}

	manifest := Manifest{
		Schema: SchemaV1, Protocol: ProtocolV1, SPX: cfg.SPX, Target: cfg.Target,
		Engine: cfg.Engine, Bridge: cfg.Bridge, Project: cfg.Project, Entries: entries,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return "", "", fmt.Errorf("runtimepayload: encode manifest: %w", err)
	}
	if err := validatePreparedPayloadLimits(entries, int64(len(manifestBytes))); err != nil {
		return "", "", err
	}
	manifestSum := sha256.Sum256(manifestBytes)

	all := make([]preparedSource, 0, len(prepared)+1)
	all = append(all, prepared...)
	all = append(all, preparedSource{
		source: FileSource{
			Name: ManifestPath, Mode: 0o644,
			ReaderAt: bytes.NewReader(manifestBytes), Size: int64(len(manifestBytes)),
		},
		entry: runtimebundle.Entry{
			Name: ManifestPath, Mode: 0o644, Size: int64(len(manifestBytes)),
			SHA256: hex.EncodeToString(manifestSum[:]),
		},
	})
	sort.Slice(all, func(i, j int) bool { return all[i].source.Name < all[j].source.Name })

	payloadHasher := sha256.New()
	writer := zip.NewWriter(io.MultiWriter(dst, payloadHasher))
	for _, file := range all {
		header := &zip.FileHeader{Name: file.source.Name, Method: zip.Store}
		header.SetMode(file.source.Mode)
		header.SetModTime(canonicalTime)
		entry, createErr := writer.CreateHeader(header)
		if createErr != nil {
			_ = writer.Close()
			return "", "", fmt.Errorf("runtimepayload: create entry %q: %w", file.source.Name, createErr)
		}
		digest, writeErr := writeSource(entry, file.source)
		if writeErr != nil {
			_ = writer.Close()
			return "", "", fmt.Errorf("runtimepayload: write entry %q: %w", file.source.Name, writeErr)
		}
		if digest != file.entry.SHA256 {
			_ = writer.Close()
			return "", "", fmt.Errorf("runtimepayload: source %q changed between hash and write", file.source.Name)
		}
	}
	if err := writer.Close(); err != nil {
		return "", "", fmt.Errorf("runtimepayload: close payload ZIP: %w", err)
	}
	return hex.EncodeToString(payloadHasher.Sum(nil)), hex.EncodeToString(manifestSum[:]), nil
}

func prepareSources(sources []FileSource, reserved string) ([]preparedSource, error) {
	sources = append([]FileSource(nil), sources...)
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
	prepared := make([]preparedSource, 0, len(sources))
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		if err := validateEntryName(source.Name); err != nil {
			return nil, err
		}
		if reserved != "" && source.Name == reserved {
			return nil, fmt.Errorf("runtimepayload: %q is reserved for the top-level manifest", source.Name)
		}
		if _, duplicate := seen[source.Name]; duplicate {
			return nil, fmt.Errorf("runtimepayload: duplicate entry %q", source.Name)
		}
		seen[source.Name] = struct{}{}
		if source.ReaderAt == nil {
			return nil, fmt.Errorf("runtimepayload: entry %q has a nil ReaderAt", source.Name)
		}
		if source.Size < 0 {
			return nil, fmt.Errorf("runtimepayload: entry %q has negative size %d", source.Name, source.Size)
		}
		source.Mode = canonicalFileMode(source.Mode)
		digest, err := hashSource(source)
		if err != nil {
			return nil, fmt.Errorf("runtimepayload: hash entry %q: %w", source.Name, err)
		}
		prepared = append(prepared, preparedSource{
			source: source,
			entry: runtimebundle.Entry{
				Name: source.Name, Mode: uint32(source.Mode), Size: source.Size, SHA256: digest,
			},
		})
	}
	return prepared, nil
}

func canonicalFileMode(mode fs.FileMode) fs.FileMode {
	if mode.Perm()&0o111 != 0 {
		return 0o755
	}
	return 0o644
}

func hashSource(source FileSource) (string, error) {
	hasher := sha256.New()
	count, err := io.Copy(hasher, io.NewSectionReader(source.ReaderAt, 0, source.Size))
	if err != nil {
		return "", err
	}
	if count != source.Size {
		return "", fmt.Errorf("short read: read %d bytes, want %d: %w", count, source.Size, io.ErrUnexpectedEOF)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeSource(dst io.Writer, source FileSource) (string, error) {
	hasher := sha256.New()
	count, err := io.Copy(io.MultiWriter(dst, hasher), io.NewSectionReader(source.ReaderAt, 0, source.Size))
	if err != nil {
		return "", err
	}
	if count != source.Size {
		return "", fmt.Errorf("short read: read %d bytes, want %d: %w", count, source.Size, io.ErrUnexpectedEOF)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func validateComponentClaims(cfg BuildConfig, prepared []preparedSource, entries []runtimebundle.Entry) error {
	engine, err := componentBundleFromEntries(entries, "engine/", runtimebundle.NamespaceEngine)
	if err != nil {
		return err
	}
	if engine.Digest != cfg.Engine.BundleDigest {
		return fmt.Errorf("runtimepayload: engine bundle digest does not match payload entries")
	}
	bridge, err := componentBundleFromEntries(entries, "bridge/", runtimebundle.NamespaceBridge)
	if err != nil {
		return err
	}
	if bridge.Digest != cfg.Bridge.BundleDigest {
		return fmt.Errorf("runtimepayload: bridge bundle digest does not match payload entries")
	}
	for _, file := range prepared {
		if file.source.Name != ProjectZipPath {
			continue
		}
		if file.entry.SHA256 != cfg.Project.ArchiveSHA256 {
			return fmt.Errorf("runtimepayload: project archive digest does not match payload entry")
		}
		project, err := ComponentBundleReaderAt(file.source.ReaderAt, file.source.Size, runtimebundle.NamespaceProject)
		if err != nil {
			return fmt.Errorf("runtimepayload: verify project archive: %w", err)
		}
		if project.Digest != cfg.Project.BundleDigest {
			return fmt.Errorf("runtimepayload: project bundle digest does not match payload entry")
		}
		return nil
	}
	return fmt.Errorf("runtimepayload: missing required entry %q", ProjectZipPath)
}
