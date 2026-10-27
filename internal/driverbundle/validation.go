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

package driverbundle

import (
	"fmt"
	"regexp"
)

var supportedTargets = [...]Target{
	{GOOS: "darwin", GOARCH: "amd64"},
	{GOOS: "darwin", GOARCH: "arm64"},
	{GOOS: "linux", GOARCH: "amd64"},
	{GOOS: "windows", GOARCH: "amd64"},
}

var (
	runtimeVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	platformPattern       = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)
)

// Validate checks manifest identity and its four canonical host bundles.
func (m Manifest) Validate() error {
	if m.Schema != ManifestSchema {
		return fmt.Errorf("driverbundle: manifest schema = %d, want %d", m.Schema, ManifestSchema)
	}
	if err := validateSPXVersion(m.SPXVersion); err != nil {
		return fmt.Errorf("driverbundle: %w", err)
	}
	if err := validateRuntimeVersion(m.RuntimeVersion); err != nil {
		return fmt.Errorf("driverbundle: %w", err)
	}
	if len(m.Bundles) != len(supportedTargets) {
		return fmt.Errorf("driverbundle: bundles = %d, want %d", len(m.Bundles), len(supportedTargets))
	}
	for i, bundle := range m.Bundles {
		want := supportedTargets[i]
		if bundle.GOOS != want.GOOS || bundle.GOARCH != want.GOARCH {
			return fmt.Errorf("driverbundle: bundle %d target = %s/%s, want %s/%s", i, bundle.GOOS, bundle.GOARCH, want.GOOS, want.GOARCH)
		}
		if err := bundle.validateForRuntime(m.RuntimeVersion); err != nil {
			return fmt.Errorf("driverbundle: bundle %d: %w", i, err)
		}
	}
	return nil
}

// ValidateVersions binds a manifest to the selected driver and runtime
// releases. Content integrity is carried by each bundle and file digest.
func (m Manifest) ValidateVersions(spxVersion, runtimeVersion string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	return m.validateVersions(spxVersion, runtimeVersion)
}

func (m Manifest) validateVersions(spxVersion, runtimeVersion string) error {
	if err := validateSPXVersion(spxVersion); err != nil {
		return fmt.Errorf("driverbundle: expected %w", err)
	}
	if err := validateRuntimeVersion(runtimeVersion); err != nil {
		return fmt.Errorf("driverbundle: expected %w", err)
	}
	if m.SPXVersion != spxVersion {
		return fmt.Errorf("driverbundle: manifest SPX version = %q, want %q", m.SPXVersion, spxVersion)
	}
	if m.RuntimeVersion != runtimeVersion {
		return fmt.Errorf("driverbundle: manifest runtime version = %q, want %q", m.RuntimeVersion, runtimeVersion)
	}
	return nil
}

// Validate checks a bundle's generic structure.
func (b Bundle) Validate() error { return b.validateForRuntime("") }

// ValidateForRuntime checks the canonical target names and modes.
func (b Bundle) ValidateForRuntime(runtimeVersion string) error {
	if err := validateRuntimeVersion(runtimeVersion); err != nil {
		return fmt.Errorf("driverbundle: %w", err)
	}
	return b.validateForRuntime(runtimeVersion)
}

func (b Bundle) validateForRuntime(runtimeVersion string) error {
	if !platformPattern.MatchString(b.GOOS) || !platformPattern.MatchString(b.GOARCH) {
		return fmt.Errorf("invalid bundle platform %q/%q", b.GOOS, b.GOARCH)
	}
	if err := validateBundleName(b.Name); err != nil {
		return err
	}
	var spec HostSpec
	if runtimeVersion != "" {
		var err error
		spec, err = HostSpecFor(runtimeVersion, b.GOOS, b.GOARCH)
		if err != nil {
			return err
		}
		if b.Name != spec.BundleName {
			return fmt.Errorf("bundle name = %q, want %q", b.Name, spec.BundleName)
		}
	}
	if b.Size <= 0 {
		return fmt.Errorf("bundle %q size must be positive", b.Name)
	}
	if err := validateSHA256(b.SHA256); err != nil {
		return fmt.Errorf("bundle %q SHA-256: %w", b.Name, err)
	}
	if err := validateSHA256(b.EngineInterfaceDigest); err != nil {
		return fmt.Errorf("bundle %q engine interface digest: %w", b.Name, err)
	}
	if len(b.Files) != 3 {
		return fmt.Errorf("bundle %q must contain exactly three files", b.Name)
	}
	seen := make(map[string]struct{}, len(b.Files))
	for i, file := range b.Files {
		if err := file.Validate(); err != nil {
			return fmt.Errorf("bundle %q file %d: %w", b.Name, i, err)
		}
		if _, ok := seen[file.Name]; ok {
			return fmt.Errorf("bundle %q has duplicate file %q", b.Name, file.Name)
		}
		seen[file.Name] = struct{}{}
	}
	wantInterface, err := ComputeEngineInterfaceDigestFromSHA256(b.Files[0].SHA256, b.Files[1].SHA256)
	if err != nil {
		return fmt.Errorf("bundle %q Engine interface: %w", b.Name, err)
	}
	if b.EngineInterfaceDigest != wantInterface {
		return fmt.Errorf("bundle %q Engine interface digest does not match Engine and PCK", b.Name)
	}
	if runtimeVersion == "" {
		return nil
	}
	want := [...]Component{spec.Engine, spec.Pack, spec.Bridge}
	for i, component := range want {
		if b.Files[i].Name != component.Name || b.Files[i].Mode != component.Mode {
			return fmt.Errorf("bundle %q file %d identity does not match target", b.Name, i)
		}
	}
	return nil
}

// Validate checks one regular file record.
func (f File) Validate() error {
	if err := validateBaseName(f.Name); err != nil {
		return fmt.Errorf("file %q: %w", f.Name, err)
	}
	if f.Mode == 0 || f.Mode&^uint32(0o777) != 0 {
		return fmt.Errorf("file %q has invalid mode %#o", f.Name, f.Mode)
	}
	if f.Size <= 0 {
		return fmt.Errorf("file %q size must be positive", f.Name)
	}
	if err := validateSHA256(f.SHA256); err != nil {
		return fmt.Errorf("file %q SHA-256: %w", f.Name, err)
	}
	return nil
}
