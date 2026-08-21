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
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

	"github.com/goplus/spx/v3/internal/runtimebundle"
	"github.com/goplus/spx/v3/internal/strictjson"
)

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
