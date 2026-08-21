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

package release

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/strictjson"
)

const localRuntimeManifestSchema = "spx-local-engine/v1"

// LocalRuntimeFile declares one local Engine/PCK output.
type LocalRuntimeFile struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// LocalRuntimeManifest describes a locally built runtime, not a publication.
type LocalRuntimeManifest struct {
	Schema         string           `json:"schema"`
	Mode           string           `json:"mode"`
	RuntimeVersion string           `json:"runtime_version"`
	RuntimeABI     int              `json:"runtime_abi"`
	LockSHA256     string           `json:"lock_sha256"`
	GOOS           string           `json:"goos"`
	GOARCH         string           `json:"goarch"`
	Engine         LocalRuntimeFile `json:"engine"`
	Pack           LocalRuntimeFile `json:"pack"`
}

// ParseLocalRuntimeManifest decodes and validates a local manifest.
func ParseLocalRuntimeManifest(data []byte) (LocalRuntimeManifest, error) {
	var manifest LocalRuntimeManifest
	if err := strictjson.Decode(data, &manifest); err != nil {
		return LocalRuntimeManifest{}, fmt.Errorf("decode local runtime manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return LocalRuntimeManifest{}, err
	}
	return manifest, nil
}

// Validate checks the manifest's local structure and file declarations.
func (m LocalRuntimeManifest) Validate() error {
	if m.Schema != localRuntimeManifestSchema {
		return fmt.Errorf("release: local runtime manifest schema = %q, want %q", m.Schema, localRuntimeManifestSchema)
	}
	if m.Mode != "local" {
		return fmt.Errorf("release: local runtime manifest mode = %q, want local", m.Mode)
	}
	if !runtimeVersionPattern.MatchString(m.RuntimeVersion) {
		return fmt.Errorf("release: invalid local runtime version %q", m.RuntimeVersion)
	}
	if m.RuntimeABI <= 0 {
		return fmt.Errorf("release: local runtime ABI must be positive")
	}
	if !isLowerHexDigest(m.LockSHA256, sha256.Size*2) {
		return fmt.Errorf("release: invalid local runtime lock SHA-256 %q", m.LockSHA256)
	}
	if m.GOOS == "" || strings.ContainsAny(m.GOOS, `/\\`) || m.GOARCH == "" || strings.ContainsAny(m.GOARCH, `/\\`) {
		return fmt.Errorf("release: invalid local runtime target %q/%q", m.GOOS, m.GOARCH)
	}
	if err := validateLocalRuntimeFile("engine", m.Engine); err != nil {
		return err
	}
	if err := validateLocalRuntimeFile("pack", m.Pack); err != nil {
		return err
	}
	return nil
}

func validateLocalRuntimeFile(label string, file LocalRuntimeFile) error {
	if file.Name == "" || file.Name == "." || file.Name == ".." || filepath.Base(file.Name) != file.Name || strings.ContainsAny(file.Name, `/\\`) {
		return fmt.Errorf("release: local runtime %s has invalid name %q", label, file.Name)
	}
	if file.Size <= 0 {
		return fmt.Errorf("release: local runtime %s size must be positive", label)
	}
	if !isLowerHexDigest(file.SHA256, sha256.Size*2) {
		return fmt.Errorf("release: local runtime %s has invalid SHA-256 %q", label, file.SHA256)
	}
	return nil
}

// ValidateForLock verifies the local manifest against the complete lock.
func (m LocalRuntimeManifest) ValidateForLock(lock RuntimeLock, goos, goarch string) error {
	if err := lock.Validate(); err != nil {
		return err
	}
	if err := m.Validate(); err != nil {
		return err
	}
	lockSHA, err := lock.SHA256()
	if err != nil {
		return err
	}
	if m.RuntimeVersion != lock.RuntimeVersion || m.RuntimeABI != lock.RuntimeABI {
		return fmt.Errorf("release: local runtime identity does not match lock")
	}
	if m.LockSHA256 != lockSHA {
		return fmt.Errorf("release: local runtime lock SHA-256 %q does not match %q", m.LockSHA256, lockSHA)
	}
	return m.validateVersionAndTarget(lock, goos, goarch)
}

// ValidateForVersion verifies a source-mode runtime by version and target.
func (m LocalRuntimeManifest) ValidateForVersion(lock RuntimeLock, goos, goarch string) error {
	if err := lock.Validate(); err != nil {
		return err
	}
	if err := m.Validate(); err != nil {
		return err
	}
	if m.RuntimeVersion != lock.RuntimeVersion {
		return fmt.Errorf("release: local runtime version %q does not match %q", m.RuntimeVersion, lock.RuntimeVersion)
	}
	return m.validateVersionAndTarget(lock, goos, goarch)
}

func (m LocalRuntimeManifest) validateVersionAndTarget(lock RuntimeLock, goos, goarch string) error {
	if m.GOOS != goos || m.GOARCH != goarch {
		return fmt.Errorf("release: local runtime target %s/%s does not match host %s/%s", m.GOOS, m.GOARCH, goos, goarch)
	}
	spec, err := HostRuntimeSpecFor(lock, goos, goarch)
	if err != nil {
		return err
	}
	if m.Engine.Name != localRuntimeObjectName(spec.RuntimeName, m.Engine.SHA256) || m.Pack.Name != localRuntimeObjectName(spec.PackName, m.Pack.SHA256) {
		return fmt.Errorf("release: local runtime file names do not match locked host runtime")
	}
	return nil
}

// JSON returns the canonical local manifest representation.
func (m LocalRuntimeManifest) JSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode local runtime manifest: %w", err)
	}
	return append(data, '\n'), nil
}

// LocalRuntimeManifestPath returns the deterministic source-mode manifest
// location for one locked host runtime.
func LocalRuntimeManifestPath(repoRoot string, lock RuntimeLock, goos, goarch string) (string, error) {
	if repoRoot == "" || !filepath.IsAbs(repoRoot) || filepath.Clean(repoRoot) != repoRoot {
		return "", fmt.Errorf("release: local runtime repository root must be absolute and clean: %q", repoRoot)
	}
	spec, err := HostRuntimeSpecFor(lock, goos, goarch)
	if err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, ".spx", "runtime", lock.RuntimeVersion, spec.GOOS+"-"+spec.GOARCH, "engine-manifest.json"), nil
}
