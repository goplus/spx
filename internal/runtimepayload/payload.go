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

// Package runtimepayload implements the canonical, self-contained payload
// embedded in an XGo SPX launcher. It is internal because the driver argv,
// payload layout, and cache manifests are implementation details of SPX.
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
	"path"
	"sort"
	"strings"
	"time"

	"github.com/goplus/spx/v3/internal/runtimebundle"
	"github.com/goplus/spx/v3/internal/strictjson"
)

const (
	SchemaV1       = "spx-runtime-payload/v1"
	ProtocolV1     = "xgo-driver-v1"
	ManifestPath   = "META-INF/spx-runtime-v1.json"
	ProjectZipPath = "project/project.zip"
)

var canonicalTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// SourceIdentity records the logical and physical SPX identity selected by
// the application graph. Source mode intentionally records replacement/main
// provenance instead of pretending that a local build is a published release.
type SourceIdentity struct {
	SelectedPath     string `json:"selected_path"`
	SelectedVersion  string `json:"selected_version"`
	EffectivePath    string `json:"effective_path"`
	EffectiveVersion string `json:"effective_version"`
	Main             bool   `json:"main"`
	SourceMode       bool   `json:"source_mode"`
}

// Target identifies the host executable represented by the payload.
type Target struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
}

// Engine describes the verified engine component and its cache identity.
type Engine struct {
	RuntimeVersion        string `json:"runtime_version"`
	RuntimeABI            int    `json:"runtime_abi"`
	EngineInterfaceDigest string `json:"engine_interface_digest"`
	Executable            string `json:"executable"`
	Pack                  string `json:"pack"`
	BundleDigest          string `json:"bundle_digest"`
}

// Bridge describes the verified interpreter bridge component.
type Bridge struct {
	File         string `json:"file"`
	BundleDigest string `json:"bundle_digest"`
}

// Project describes the project materialized for launcher execution.
type Project struct {
	PackDirectory string `json:"pack_directory"`
	BundleDigest  string `json:"bundle_digest"`
	ArchiveSHA256 string `json:"archive_sha256"`
}

// Manifest is the only top-level payload manifest. Entries contains every ZIP
// entry except ManifestPath, avoiding a self-referential digest.
type Manifest struct {
	Schema   string                `json:"schema"`
	Protocol string                `json:"protocol"`
	SPX      SourceIdentity        `json:"spx"`
	Target   Target                `json:"target"`
	Engine   Engine                `json:"engine"`
	Bridge   Bridge                `json:"bridge"`
	Project  Project               `json:"project"`
	Entries  []runtimebundle.Entry `json:"entries"`
}

// File is one non-manifest payload entry. Name is a fixed slash path and Data
// is snapshotted by Build before it returns.
type File struct {
	Name string
	Mode fs.FileMode
	Data []byte
}

// FileSource is one repeatable, fixed-size payload entry. ReaderAt must remain
// readable and stable for the duration of BuildTo. BuildTo reads every source
// twice and rejects short reads or content changes between the two passes.
type FileSource struct {
	Name     string
	Mode     fs.FileMode
	ReaderAt io.ReaderAt
	Size     int64
}

// BuildConfig supplies the identities that cannot be inferred from entry
// bytes. Component bundle digests must be namespace-qualified identities from
// runtimebundle.Bundle.WithDigest.
type BuildConfig struct {
	SPX     SourceIdentity
	Target  Target
	Engine  Engine
	Bridge  Bridge
	Project Project
	Files   []File
}

// Build is the in-memory convenience wrapper used by small callers and tests.
// Production callers should use BuildTo so payload bytes are never materialized
// as one large heap allocation.
func Build(cfg BuildConfig) (payload []byte, payloadSHA256, manifestSHA256 string, err error) {
	sources := make([]FileSource, len(cfg.Files))
	snapshots := make([][]byte, len(cfg.Files))
	for i, file := range cfg.Files {
		snapshots[i] = append([]byte(nil), file.Data...)
		sources[i] = FileSource{
			Name: file.Name, Mode: file.Mode,
			ReaderAt: bytes.NewReader(snapshots[i]), Size: int64(len(snapshots[i])),
		}
	}
	cfg.Files = nil
	var output bytes.Buffer
	payloadSHA256, manifestSHA256, err = BuildTo(&output, cfg, sources)
	if err != nil {
		return nil, "", "", err
	}
	return output.Bytes(), payloadSHA256, manifestSHA256, nil
}

type preparedSource struct {
	source FileSource
	entry  runtimebundle.Entry
}

// BuildTo writes canonical payload bytes to dst and returns the full payload
// and manifest SHA-256 values. cfg.Files must be empty; sources is the streaming
// representation used by this API. An error may leave a partial payload in dst.
func BuildTo(dst io.Writer, cfg BuildConfig, sources []FileSource) (payloadSHA256, manifestSHA256 string, err error) {
	if dst == nil {
		return "", "", errors.New("runtimepayload: nil payload writer")
	}
	if len(cfg.Files) != 0 {
		return "", "", errors.New("runtimepayload: BuildTo requires FileSource inputs, not BuildConfig.Files")
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

func validateIdentity(cfg BuildConfig) error {
	for name, value := range map[string]string{
		"selected SPX path":       cfg.SPX.SelectedPath,
		"effective SPX path":      cfg.SPX.EffectivePath,
		"target GOOS":             cfg.Target.GOOS,
		"target GOARCH":           cfg.Target.GOARCH,
		"runtime version":         cfg.Engine.RuntimeVersion,
		"engine interface digest": cfg.Engine.EngineInterfaceDigest,
		"engine executable":       cfg.Engine.Executable,
		"engine pack":             cfg.Engine.Pack,
		"engine bundle digest":    cfg.Engine.BundleDigest,
		"bridge file":             cfg.Bridge.File,
		"bridge bundle digest":    cfg.Bridge.BundleDigest,
		"project pack directory":  cfg.Project.PackDirectory,
		"project bundle digest":   cfg.Project.BundleDigest,
		"project archive digest":  cfg.Project.ArchiveSHA256,
	} {
		if value == "" {
			return fmt.Errorf("runtimepayload: %s is empty", name)
		}
	}
	if cfg.Engine.RuntimeABI <= 0 {
		return fmt.Errorf("runtimepayload: runtime ABI must be positive")
	}
	for name, value := range map[string]string{
		"engine interface digest": cfg.Engine.EngineInterfaceDigest,
		"engine bundle digest":    cfg.Engine.BundleDigest,
		"bridge bundle digest":    cfg.Bridge.BundleDigest,
		"project bundle digest":   cfg.Project.BundleDigest,
		"project archive digest":  cfg.Project.ArchiveSHA256,
	} {
		if err := validateDigest(value); err != nil {
			return fmt.Errorf("runtimepayload: invalid %s: %w", name, err)
		}
	}
	for _, name := range []string{cfg.Engine.Executable, cfg.Engine.Pack, cfg.Bridge.File} {
		if path.Base(name) != name || name == "." || strings.ContainsAny(name, `\\:`) {
			return fmt.Errorf("runtimepayload: unsafe component basename %q", name)
		}
	}
	if cfg.Project.PackDirectory == "." || path.Clean(cfg.Project.PackDirectory) != cfg.Project.PackDirectory || strings.HasPrefix(cfg.Project.PackDirectory, "../") || path.IsAbs(cfg.Project.PackDirectory) {
		return fmt.Errorf("runtimepayload: invalid project pack directory %q", cfg.Project.PackDirectory)
	}
	return nil
}

func validateEntryName(name string) error {
	if name == "" || path.Clean(name) != name || path.IsAbs(name) || strings.ContainsRune(name, '\\') {
		return fmt.Errorf("runtimepayload: invalid entry path %q", name)
	}
	for _, component := range strings.Split(name, "/") {
		if component == "" || component == "." || component == ".." {
			return fmt.Errorf("runtimepayload: invalid entry path %q", name)
		}
	}
	return nil
}

func validateDigest(value string) error {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return errors.New("digest must be 64 lower-case hexadecimal characters")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return errors.New("digest must be 64 lower-case hexadecimal characters")
	}
	return nil
}

// Verified is an immutable payload whose complete archive, top-level manifest,
// entry table, component identities, and target have been checked.
type Verified struct {
	Manifest Manifest
	source   io.ReaderAt
	files    map[string]*zip.File
	entries  []runtimebundle.Entry
}

// Verify is the in-memory convenience wrapper around VerifyReaderAt.
func Verify(payload []byte, payloadSHA256, manifestSHA256, goos, goarch string) (*Verified, error) {
	reader := bytes.NewReader(payload)
	return VerifyReaderAt(reader, int64(len(payload)), payloadSHA256, manifestSHA256, goos, goarch)
}

const maxPayloadManifestBytes = 1 << 20

// The payload is a ZIP written with archive/zip's canonical Store headers.
// Keep these format sizes local to the preflight calculation below: the
// runtimebundle limits are the policy source of truth, while these constants
// describe the bytes BuildTo is about to emit.
const (
	zipLocalHeaderBytes      int64  = 30
	zipCentralHeaderBytes    int64  = 46
	zipDataDescriptorBytes   int64  = 16
	zipDataDescriptor64Bytes int64  = 24
	zipEndBytes              int64  = 22
	zip64EndBytes            int64  = 56 + 20
	zipModTimeExtraBytes     int64  = 9
	zip64CentralExtraBytes   int64  = 28
	zipMaxNameBytes                 = 1<<16 - 1
	zip32Max                 uint64 = 1<<32 - 1
)

// VerifyReaderAt authenticates and indexes a payload without retaining copies
// of its large entries. source must remain readable and unchanged while the
// returned Verified value is used.
func VerifyReaderAt(source io.ReaderAt, size int64, payloadSHA256, manifestSHA256, goos, goarch string) (*Verified, error) {
	if source == nil {
		return nil, fmt.Errorf("runtimepayload: nil payload reader")
	}
	if size < 0 {
		return nil, fmt.Errorf("runtimepayload: negative payload size %d", size)
	}
	if size > runtimebundle.MaxArchiveBytes {
		return nil, fmt.Errorf("runtimepayload: archive size %d exceeds limit %d: %w", size, runtimebundle.MaxArchiveBytes, runtimebundle.ErrArchiveLimit)
	}
	if err := compareReaderAtDigest(source, size, payloadSHA256, "payload"); err != nil {
		return nil, err
	}
	bundle, err := runtimebundle.VerifyZipReader(source, size)
	if err != nil {
		return nil, fmt.Errorf("runtimepayload: verify payload ZIP: %w", err)
	}
	reader, err := zip.NewReader(source, size)
	if err != nil {
		return nil, fmt.Errorf("runtimepayload: open verified payload ZIP: %w", err)
	}
	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		files[file.Name] = file
	}
	manifestFile, ok := files[ManifestPath]
	if !ok {
		return nil, fmt.Errorf("runtimepayload: missing %s", ManifestPath)
	}
	manifestBytes, err := readSmallZipEntry(manifestFile, maxPayloadManifestBytes)
	if err != nil {
		return nil, fmt.Errorf("runtimepayload: read manifest: %w", err)
	}
	if err := compareDigest(manifestBytes, manifestSHA256, "manifest"); err != nil {
		return nil, err
	}
	manifest, err := parseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	if manifest.Target.GOOS != goos || manifest.Target.GOARCH != goarch {
		return nil, fmt.Errorf("runtimepayload: payload target %s/%s does not match host %s/%s", manifest.Target.GOOS, manifest.Target.GOARCH, goos, goarch)
	}
	actualEntries := make([]runtimebundle.Entry, 0, len(bundle.Entries)-1)
	for _, entry := range bundle.Entries {
		if entry.Name != ManifestPath {
			actualEntries = append(actualEntries, entry)
		}
	}
	if !entriesEqual(manifest.Entries, actualEntries) {
		return nil, fmt.Errorf("runtimepayload: payload entry table does not match archive")
	}
	verified := &Verified{
		Manifest: manifest,
		source:   source,
		files:    files,
		entries:  append([]runtimebundle.Entry(nil), actualEntries...),
	}
	if err := verified.validateComponents(); err != nil {
		return nil, err
	}
	// Re-read the complete source after all structural checks. This catches a
	// mutable ReaderAt changing between the initial payload hash and indexing.
	if err := compareReaderAtDigest(source, size, payloadSHA256, "payload"); err != nil {
		return nil, fmt.Errorf("runtimepayload: payload source changed during verification: %w", err)
	}
	return verified, nil
}

func readSmallZipEntry(file *zip.File, limit int64) ([]byte, error) {
	if file.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("entry %q size %d exceeds limit %d", file.Name, file.UncompressedSize64, limit)
	}
	input, err := file.Open()
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(input, limit+1))
	closeErr := input.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("entry %q exceeds limit %d", file.Name, limit)
	}
	if uint64(len(data)) != file.UncompressedSize64 {
		return nil, fmt.Errorf("entry %q short read: got %d, want %d", file.Name, len(data), file.UncompressedSize64)
	}
	return data, nil
}

func parseManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := strictjson.Decode(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("runtimepayload: decode manifest: %w", err)
	}
	if manifest.Schema != SchemaV1 || manifest.Protocol != ProtocolV1 {
		return Manifest{}, fmt.Errorf("runtimepayload: unsupported manifest schema/protocol %q/%q", manifest.Schema, manifest.Protocol)
	}
	cfg := BuildConfig{SPX: manifest.SPX, Target: manifest.Target, Engine: manifest.Engine, Bridge: manifest.Bridge, Project: manifest.Project}
	if err := validateIdentity(cfg); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (v *Verified) validateComponents() error {
	required := []string{
		"engine/runtime-manifest.json",
		"engine/" + v.Manifest.Engine.Executable,
		"engine/" + v.Manifest.Engine.Pack,
		"bridge/bridge-manifest.json",
		"bridge/" + v.Manifest.Bridge.File,
		ProjectZipPath,
	}
	if len(v.files) != len(required)+1 {
		return fmt.Errorf("runtimepayload: payload has %d component entries, want %d", len(v.files)-1, len(required))
	}
	for _, name := range required {
		if _, ok := v.files[name]; !ok {
			return fmt.Errorf("runtimepayload: missing required entry %q", name)
		}
	}
	projectEntry, ok := findEntry(v.entries, ProjectZipPath)
	if !ok {
		return fmt.Errorf("runtimepayload: missing required entry %q", ProjectZipPath)
	}
	if projectEntry.SHA256 != v.Manifest.Project.ArchiveSHA256 {
		return fmt.Errorf("runtimepayload: project archive SHA-256 mismatch")
	}
	projectFile := v.files[ProjectZipPath]
	projectReader, projectSize, err := v.storedEntryReaderAt(projectFile)
	if err != nil {
		return err
	}
	project, err := ComponentBundleReaderAt(projectReader, projectSize, runtimebundle.NamespaceProject)
	if err != nil {
		return fmt.Errorf("runtimepayload: verify project archive: %w", err)
	}
	if project.Digest != v.Manifest.Project.BundleDigest {
		return fmt.Errorf("runtimepayload: project bundle digest mismatch")
	}
	for _, component := range []struct {
		prefix    string
		namespace runtimebundle.Namespace
		digest    string
	}{
		{prefix: "engine/", namespace: runtimebundle.NamespaceEngine, digest: v.Manifest.Engine.BundleDigest},
		{prefix: "bridge/", namespace: runtimebundle.NamespaceBridge, digest: v.Manifest.Bridge.BundleDigest},
	} {
		bundle, err := componentBundleFromEntries(v.entries, component.prefix, component.namespace)
		if err != nil {
			return fmt.Errorf("runtimepayload: verify %s component: %w", strings.TrimSuffix(component.prefix, "/"), err)
		}
		if bundle.Digest != component.digest {
			return fmt.Errorf("runtimepayload: %s bundle digest mismatch", strings.TrimSuffix(component.prefix, "/"))
		}
	}
	return nil
}

func (v *Verified) storedEntryReaderAt(file *zip.File) (io.ReaderAt, int64, error) {
	if v == nil || file == nil {
		return nil, 0, fmt.Errorf("runtimepayload: nil stored payload entry")
	}
	if file.Method != zip.Store || file.CompressedSize64 != file.UncompressedSize64 {
		return nil, 0, fmt.Errorf("runtimepayload: project archive entry must use canonical ZIP store mode")
	}
	offset, err := file.DataOffset()
	if err != nil {
		return nil, 0, fmt.Errorf("runtimepayload: locate project archive: %w", err)
	}
	size := int64(file.UncompressedSize64)
	return io.NewSectionReader(v.source, offset, size), size, nil
}

// WriteComponentZIP writes a deterministic archive containing entries below
// prefix with the prefix removed. Only engine/ and bridge/ are accepted.
func (v *Verified) WriteComponentZIP(prefix string, dst io.Writer) error {
	if v == nil || (prefix != "engine/" && prefix != "bridge/") {
		return fmt.Errorf("runtimepayload: invalid component prefix %q", prefix)
	}
	if dst == nil {
		return fmt.Errorf("runtimepayload: nil component writer")
	}
	var entries []runtimebundle.Entry
	for _, entry := range v.entries {
		if strings.HasPrefix(entry.Name, prefix) {
			entry.Name = strings.TrimPrefix(entry.Name, prefix)
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return fmt.Errorf("runtimepayload: empty component %q", prefix)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	writer := zip.NewWriter(dst)
	for _, entry := range entries {
		sourceName := prefix + entry.Name
		file := v.files[sourceName]
		if file == nil {
			_ = writer.Close()
			return fmt.Errorf("runtimepayload: missing component entry %q", sourceName)
		}
		header := &zip.FileHeader{Name: entry.Name, Method: zip.Store}
		header.SetMode(canonicalFileMode(fs.FileMode(entry.Mode)))
		header.SetModTime(canonicalTime)
		output, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return fmt.Errorf("runtimepayload: create component entry %q: %w", entry.Name, err)
		}
		if err := copyVerifiedZipEntry(output, file, runtimebundle.Entry{
			Name: sourceName, Mode: entry.Mode, Size: entry.Size, SHA256: entry.SHA256,
		}); err != nil {
			_ = writer.Close()
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("runtimepayload: close component ZIP: %w", err)
	}
	return nil
}

// ComponentZIP is the in-memory compatibility wrapper around WriteComponentZIP.
func (v *Verified) ComponentZIP(prefix string) ([]byte, error) {
	var output bytes.Buffer
	if err := v.WriteComponentZIP(prefix, &output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// WriteProjectZIP streams the embedded canonical project archive to dst.
func (v *Verified) WriteProjectZIP(dst io.Writer) error {
	if v == nil {
		return fmt.Errorf("runtimepayload: nil verified payload")
	}
	if dst == nil {
		return fmt.Errorf("runtimepayload: nil project writer")
	}
	file := v.files[ProjectZipPath]
	entry, ok := findEntry(v.entries, ProjectZipPath)
	if file == nil || !ok {
		return fmt.Errorf("runtimepayload: missing required entry %q", ProjectZipPath)
	}
	return copyVerifiedZipEntry(dst, file, entry)
}

// ProjectZIP is the in-memory compatibility wrapper around WriteProjectZIP.
func (v *Verified) ProjectZIP() []byte {
	var output bytes.Buffer
	if err := v.WriteProjectZIP(&output); err != nil {
		return nil
	}
	return output.Bytes()
}

func copyVerifiedZipEntry(dst io.Writer, file *zip.File, expected runtimebundle.Entry) error {
	input, err := file.Open()
	if err != nil {
		return fmt.Errorf("runtimepayload: open entry %q: %w", file.Name, err)
	}
	hasher := sha256.New()
	count, copyErr := io.Copy(io.MultiWriter(dst, hasher), input)
	closeErr := input.Close()
	if copyErr != nil {
		return fmt.Errorf("runtimepayload: copy entry %q: %w", file.Name, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("runtimepayload: close entry %q: %w", file.Name, closeErr)
	}
	if count != expected.Size {
		return fmt.Errorf("runtimepayload: entry %q size changed: got %d, want %d", file.Name, count, expected.Size)
	}
	if digest := hex.EncodeToString(hasher.Sum(nil)); digest != expected.SHA256 {
		return fmt.Errorf("runtimepayload: entry %q digest changed", file.Name)
	}
	return nil
}

func findEntry(entries []runtimebundle.Entry, name string) (runtimebundle.Entry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return runtimebundle.Entry{}, false
}

// ComponentBundle derives the namespace-qualified runtimebundle identity for
// canonical component ZIP bytes.
func ComponentBundle(data []byte, namespace runtimebundle.Namespace) (runtimebundle.Bundle, error) {
	return ComponentBundleReaderAt(bytes.NewReader(data), int64(len(data)), namespace)
}

// ComponentBundleReaderAt derives a namespace-qualified runtimebundle identity
// without materializing the archive bytes in memory.
func ComponentBundleReaderAt(reader io.ReaderAt, size int64, namespace runtimebundle.Namespace) (runtimebundle.Bundle, error) {
	bundle, err := runtimebundle.VerifyZipReader(reader, size)
	if err != nil {
		return runtimebundle.Bundle{}, err
	}
	bundle.Namespace = namespace
	return bundle.WithDigest()
}

// ComponentBundleSources derives the identity of the canonical component ZIP
// represented by sources without first constructing that ZIP in memory.
func ComponentBundleSources(sources []FileSource, namespace runtimebundle.Namespace) (runtimebundle.Bundle, error) {
	prepared, err := prepareSources(sources, "")
	if err != nil {
		return runtimebundle.Bundle{}, err
	}
	entries := make([]runtimebundle.Entry, len(prepared))
	for i := range prepared {
		entries[i] = prepared[i].entry
	}
	return componentBundleFromEntries(entries, "", namespace)
}

func componentBundleFromEntries(entries []runtimebundle.Entry, prefix string, namespace runtimebundle.Namespace) (runtimebundle.Bundle, error) {
	component := make([]runtimebundle.Entry, 0, len(entries))
	for _, entry := range entries {
		if prefix != "" && !strings.HasPrefix(entry.Name, prefix) {
			continue
		}
		entry.Name = strings.TrimPrefix(entry.Name, prefix)
		component = append(component, entry)
	}
	if len(component) == 0 {
		return runtimebundle.Bundle{}, fmt.Errorf("runtimepayload: empty %s component", strings.TrimSuffix(prefix, "/"))
	}
	bundle := runtimebundle.Bundle{
		Schema: runtimebundle.SchemaV1, Namespace: namespace, Entries: component,
	}
	return bundle.WithDigest()
}

// CanonicalComponentZIP builds an archive whose runtimebundle identity is
// stable across hosts. It is used by the driver before payload assembly.
func CanonicalComponentZIP(files []File) ([]byte, error) { return canonicalZIP(files) }

func canonicalZIP(files []File) ([]byte, error) {
	files = append([]File(nil), files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		if err := validateEntryName(file.Name); err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, duplicate := seen[file.Name]; duplicate {
			_ = writer.Close()
			return nil, fmt.Errorf("runtimepayload: duplicate component entry %q", file.Name)
		}
		seen[file.Name] = struct{}{}
		header := &zip.FileHeader{Name: file.Name, Method: zip.Store}
		if file.Mode.Perm()&0o111 != 0 {
			header.SetMode(0o755)
		} else {
			header.SetMode(0o644)
		}
		header.SetModTime(canonicalTime)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, err := entry.Write(file.Data); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func compareDigest(data []byte, expected, name string) error {
	if err := validateDigest(expected); err != nil {
		return fmt.Errorf("runtimepayload: invalid expected %s SHA-256: %w", name, err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		return fmt.Errorf("runtimepayload: %s SHA-256 mismatch", name)
	}
	return nil
}

func compareReaderAtDigest(reader io.ReaderAt, size int64, expected, name string) error {
	if err := validateDigest(expected); err != nil {
		return fmt.Errorf("runtimepayload: invalid expected %s SHA-256: %w", name, err)
	}
	digest, err := hashSource(FileSource{ReaderAt: reader, Size: size})
	if err != nil {
		return fmt.Errorf("runtimepayload: hash %s: %w", name, err)
	}
	if digest != expected {
		return fmt.Errorf("runtimepayload: %s SHA-256 mismatch", name)
	}
	return nil
}

func entriesEqual(first, second []runtimebundle.Entry) bool {
	if len(first) != len(second) {
		return false
	}
	left := append([]runtimebundle.Entry(nil), first...)
	right := append([]runtimebundle.Entry(nil), second...)
	sort.Slice(left, func(i, j int) bool { return left[i].Name < left[j].Name })
	sort.Slice(right, func(i, j int) bool { return right[i].Name < right[j].Name })
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
