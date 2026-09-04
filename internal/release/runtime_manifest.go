/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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

package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goplus/spx/v3/internal/strictjson"
)

const (
	RuntimeManifestSchema = 1
	SHA256SumsFileName    = "SHA256SUMS"
)

// RuntimeManifest describes one complete, content-verifiable runtime release.
type RuntimeManifest struct {
	Schema            int               `json:"schema"`
	RuntimeVersion    string            `json:"runtime_version"`
	RuntimeABI        int               `json:"runtime_abi"`
	ReleaseRepository string            `json:"release_repository"`
	LockSHA256        string            `json:"lock_sha256"`
	Provenance        RuntimeProvenance `json:"provenance"`
	Assets            []RuntimeAsset    `json:"assets"`
}

// RuntimeProvenance identifies the source tree and build inputs behind a
// runtime manifest.
type RuntimeProvenance struct {
	SPXCommit               string        `json:"spx_commit"`
	GodotCommit             string        `json:"godot_commit"`
	ModuleTree              string        `json:"module_tree"`
	RuntimePackSourceSHA256 string        `json:"runtime_pack_source_sha256"`
	BuildRecipeSHA256       string        `json:"build_recipe_sha256"`
	Toolchain               ToolchainLock `json:"toolchain"`
}

// RuntimeAsset is a checksummed release asset. Name is always a basename.
type RuntimeAsset struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// RuntimeAssetInput maps a release asset name to the local file to hash.
type RuntimeAssetInput struct {
	Name string
	Path string
}

// GenerateRuntimeManifest hashes assets and creates a manifest sorted by asset
// name. It validates the result against lock before returning it.
func GenerateRuntimeManifest(lock RuntimeLock, provenance RuntimeProvenance, inputs []RuntimeAssetInput) (RuntimeManifest, error) {
	if err := lock.Validate(); err != nil {
		return RuntimeManifest{}, err
	}
	lockSHA256, err := lock.SHA256()
	if err != nil {
		return RuntimeManifest{}, err
	}

	assets := make([]RuntimeAsset, 0, len(inputs))
	for _, input := range inputs {
		if err := validateBaseName("runtime asset", input.Name); err != nil {
			return RuntimeManifest{}, err
		}
		if strings.TrimSpace(input.Path) == "" {
			return RuntimeManifest{}, fmt.Errorf("release: runtime asset %q has an empty path", input.Name)
		}
		size, digest, err := hashFile(input.Path)
		if err != nil {
			return RuntimeManifest{}, fmt.Errorf("release: hash runtime asset %q: %w", input.Name, err)
		}
		assets = append(assets, RuntimeAsset{Name: input.Name, Size: size, SHA256: digest})
	}
	slices.SortFunc(assets, func(a, b RuntimeAsset) int { return strings.Compare(a.Name, b.Name) })

	manifest := RuntimeManifest{
		Schema:            RuntimeManifestSchema,
		RuntimeVersion:    lock.RuntimeVersion,
		RuntimeABI:        lock.RuntimeABI,
		ReleaseRepository: lock.ReleaseRepository,
		LockSHA256:        lockSHA256,
		Provenance:        provenance,
		Assets:            assets,
	}
	if err := manifest.ValidateForLock(lock); err != nil {
		return RuntimeManifest{}, err
	}
	return manifest, nil
}

// ParseRuntimeManifest decodes and structurally validates a manifest.
func ParseRuntimeManifest(data []byte) (RuntimeManifest, error) {
	var manifest RuntimeManifest
	if err := strictjson.Decode(data, &manifest); err != nil {
		return RuntimeManifest{}, fmt.Errorf("decode runtime manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return RuntimeManifest{}, err
	}
	return manifest, nil
}

// ParseRuntimeManifestForRelease decodes a manifest and binds it to the
// selected runtime version and required release asset set without repeating
// structural validation.
func ParseRuntimeManifestForRelease(data []byte, runtimeVersion string, requiredAssets []string) (RuntimeManifest, error) {
	manifest, err := ParseRuntimeManifest(data)
	if err != nil {
		return RuntimeManifest{}, err
	}
	if err := manifest.validateForVersion(runtimeVersion); err != nil {
		return RuntimeManifest{}, err
	}
	if err := manifest.validateRequiredAssets(requiredAssets); err != nil {
		return RuntimeManifest{}, err
	}
	return manifest, nil
}

// LoadRuntimeManifest reads and parses a runtime manifest file.
func LoadRuntimeManifest(path string) (RuntimeManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RuntimeManifest{}, fmt.Errorf("read runtime manifest: %w", err)
	}
	return ParseRuntimeManifest(data)
}

// Validate checks the manifest's internal structure and canonical asset order.
func (m RuntimeManifest) Validate() error {
	if m.Schema != RuntimeManifestSchema {
		return fmt.Errorf("release: runtime manifest schema = %d, want %d", m.Schema, RuntimeManifestSchema)
	}
	if !runtimeVersionPattern.MatchString(m.RuntimeVersion) {
		return fmt.Errorf("release: invalid manifest runtime version %q", m.RuntimeVersion)
	}
	if m.RuntimeABI <= 0 {
		return errors.New("release: manifest runtime ABI must be positive")
	}
	if !releaseRepositoryPattern.MatchString(m.ReleaseRepository) {
		return fmt.Errorf("release: invalid manifest release repository %q", m.ReleaseRepository)
	}
	if !isLowerHexDigest(m.LockSHA256, sha256.Size*2) {
		return fmt.Errorf("release: invalid runtime lock SHA-256 %q", m.LockSHA256)
	}
	if !gitCommitPattern.MatchString(m.Provenance.SPXCommit) {
		return fmt.Errorf("release: invalid SPX provenance commit %q", m.Provenance.SPXCommit)
	}
	if !gitCommitPattern.MatchString(m.Provenance.GodotCommit) {
		return fmt.Errorf("release: invalid Godot provenance commit %q", m.Provenance.GodotCommit)
	}
	if !gitCommitPattern.MatchString(m.Provenance.ModuleTree) {
		return fmt.Errorf("release: invalid module provenance tree %q", m.Provenance.ModuleTree)
	}
	if !isLowerHexDigest(m.Provenance.RuntimePackSourceSHA256, sha256.Size*2) {
		return fmt.Errorf("release: invalid runtime pack source SHA-256 %q", m.Provenance.RuntimePackSourceSHA256)
	}
	if !isLowerHexDigest(m.Provenance.BuildRecipeSHA256, sha256.Size*2) {
		return fmt.Errorf("release: invalid build recipe SHA-256 %q", m.Provenance.BuildRecipeSHA256)
	}
	if err := validateToolchainLock(m.Provenance.Toolchain); err != nil {
		return err
	}
	if len(m.Assets) == 0 {
		return errors.New("release: runtime manifest assets must not be empty")
	}
	previousName := ""
	for i, asset := range m.Assets {
		if err := validateBaseName("runtime asset", asset.Name); err != nil {
			return err
		}
		if i > 0 && asset.Name <= previousName {
			return fmt.Errorf("release: runtime assets must be uniquely sorted by name: %q follows %q", asset.Name, previousName)
		}
		if asset.Size <= 0 {
			return fmt.Errorf("release: runtime asset %q size must be positive", asset.Name)
		}
		if !isLowerHexDigest(asset.SHA256, sha256.Size*2) {
			return fmt.Errorf("release: runtime asset %q has invalid SHA-256 %q", asset.Name, asset.SHA256)
		}
		previousName = asset.Name
	}
	return nil
}

// ValidateForVersion checks the version portion of a manifest's release
// identity. Reuse callers must also validate the lock and scoped provenance.
func (m RuntimeManifest) ValidateForVersion(runtimeVersion string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	return m.validateForVersion(runtimeVersion)
}

func (m RuntimeManifest) validateForVersion(runtimeVersion string) error {
	if !runtimeVersionPattern.MatchString(runtimeVersion) {
		return fmt.Errorf("release: invalid expected runtime version %q", runtimeVersion)
	}
	if m.RuntimeVersion != runtimeVersion {
		return fmt.Errorf("release: runtime manifest version %q does not match %q", m.RuntimeVersion, runtimeVersion)
	}
	return nil
}

// ValidateRequiredAssets checks that the manifest contains exactly the named
// release assets. Content size and SHA-256 validation remains part of Validate.
func (m RuntimeManifest) ValidateRequiredAssets(requiredAssets []string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	return m.validateRequiredAssets(requiredAssets)
}

func (m RuntimeManifest) validateRequiredAssets(requiredAssets []string) error {
	required := make(map[string]struct{}, len(requiredAssets))
	for _, name := range requiredAssets {
		if err := validateBaseName("required runtime asset", name); err != nil {
			return err
		}
		if _, exists := required[name]; exists {
			return fmt.Errorf("release: duplicate required runtime asset %q", name)
		}
		required[name] = struct{}{}
	}
	if len(m.Assets) != len(required) {
		return fmt.Errorf("release: runtime manifest has %d assets, release requires exactly %d", len(m.Assets), len(required))
	}
	for _, asset := range m.Assets {
		if _, ok := required[asset.Name]; !ok {
			return fmt.Errorf("release: runtime asset %q is not required", asset.Name)
		}
	}
	return nil
}

// ValidateForLock validates metadata generated from one exact build lock.
// Published reuse checks compare the scoped source provenance separately.
func (m RuntimeManifest) ValidateForLock(lock RuntimeLock) error {
	if err := lock.Validate(); err != nil {
		return err
	}
	if err := m.Validate(); err != nil {
		return err
	}
	if err := m.validateForVersion(lock.RuntimeVersion); err != nil {
		return err
	}
	lockSHA256, err := lock.SHA256()
	if err != nil {
		return err
	}
	if m.RuntimeABI != lock.RuntimeABI {
		return fmt.Errorf("release: runtime manifest identity does not match lock")
	}
	if m.ReleaseRepository != lock.ReleaseRepository {
		return fmt.Errorf("release: manifest repository %q does not match lock %q", m.ReleaseRepository, lock.ReleaseRepository)
	}
	if m.LockSHA256 != lockSHA256 {
		return fmt.Errorf("release: manifest lock SHA-256 %q does not match %q", m.LockSHA256, lockSHA256)
	}
	if m.Provenance.GodotCommit != lock.Godot.Commit {
		return fmt.Errorf("release: manifest Godot commit %q does not match lock %q", m.Provenance.GodotCommit, lock.Godot.Commit)
	}
	if m.Provenance.Toolchain != lock.Toolchain {
		return errors.New("release: manifest toolchain does not match lock")
	}

	return m.validateRequiredAssets(lock.RequiredAssets)
}

// JSON returns the canonical, human-readable representation of a manifest.
func (m RuntimeManifest) JSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode runtime manifest: %w", err)
	}
	return append(data, '\n'), nil
}

// WriteRuntimeManifest writes a validated runtime manifest to path.
func WriteRuntimeManifest(path string, manifest RuntimeManifest) error {
	data, err := manifest.JSON()
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write runtime manifest: %w", err)
	}
	return nil
}

// Asset returns the manifest entry for name. Assets are canonically sorted, so
// the lookup is deterministic and does not allocate.
func (m RuntimeManifest) Asset(name string) (RuntimeAsset, bool) {
	index, found := slices.BinarySearchFunc(m.Assets, RuntimeAsset{Name: name}, func(a, b RuntimeAsset) int {
		return strings.Compare(a.Name, b.Name)
	})
	if !found {
		return RuntimeAsset{}, false
	}
	return m.Assets[index], true
}

// VerifyAsset verifies one local file by exact size and SHA-256 against the
// named manifest entry. This supports download-then-verify consumers that do
// not materialize the complete release in one directory.
func (m RuntimeManifest) VerifyAsset(name, path string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	asset, ok := m.Asset(name)
	if !ok {
		return fmt.Errorf("release: runtime asset %q is not present in the manifest", name)
	}
	size, digest, err := hashFile(path)
	if err != nil {
		return fmt.Errorf("release: verify runtime asset %q: %w", name, err)
	}
	if size != asset.Size {
		return fmt.Errorf("release: runtime asset %q size = %d, want %d", name, size, asset.Size)
	}
	if digest != asset.SHA256 {
		return fmt.Errorf("release: runtime asset %q SHA-256 = %s, want %s", name, digest, asset.SHA256)
	}
	return nil
}

// VerifyFiles verifies every manifest asset below dir by exact size and SHA-256.
func (m RuntimeManifest) VerifyFiles(dir string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	for _, asset := range m.Assets {
		if err := m.VerifyAsset(asset.Name, filepath.Join(dir, asset.Name)); err != nil {
			return err
		}
	}
	return nil
}

// SHA256SUMS returns conventional, deterministically ordered checksum lines.
func (m RuntimeManifest) SHA256SUMS() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	var sums strings.Builder
	for _, asset := range m.Assets {
		fmt.Fprintf(&sums, "%s  %s\n", asset.SHA256, asset.Name)
	}
	return []byte(sums.String()), nil
}

// WriteSHA256SUMS writes conventional checksum lines for a manifest's assets.
func WriteSHA256SUMS(path string, manifest RuntimeManifest) error {
	data, err := manifest.SHA256SUMS()
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write SHA256SUMS: %w", err)
	}
	return nil
}

func hashFile(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, "", err
	}
	if !info.Mode().IsRegular() {
		return 0, "", fmt.Errorf("not a regular file")
	}
	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(hasher.Sum(nil)), nil
}

func isLowerHexDigest(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}
