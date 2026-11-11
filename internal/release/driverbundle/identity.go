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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// Target identifies one host supported by the published driver bundle.
type Target struct {
	GOOS   string
	GOARCH string
}

// Component identifies one file in a combined Engine/PCK/bridge bundle.
type Component struct {
	Name string
	Mode uint32
}

// HostSpec is the canonical platform and component naming contract shared by
// manifest validation, release packaging, and launcher materialization.
type HostSpec struct {
	Target
	RuntimeVersion string
	BundleName     string
	Engine         Component
	Pack           Component
	Bridge         Component
}

// SupportedTargets returns a copy of the supported host list in release order.
func SupportedTargets() []Target {
	result := make([]Target, len(supportedTargets))
	copy(result, supportedTargets[:])
	return result
}

// HostSpecFor returns the canonical component names and modes for one host.
func HostSpecFor(runtimeVersion, goos, goarch string) (HostSpec, error) {
	if err := validateRuntimeVersion(runtimeVersion); err != nil {
		return HostSpec{}, err
	}
	for _, target := range supportedTargets {
		if target.GOOS != goos || target.GOARCH != goarch {
			continue
		}
		names := expectedFileNames(runtimeVersion, goos, goarch)
		return HostSpec{
			Target:         Target{GOOS: goos, GOARCH: goarch},
			RuntimeVersion: runtimeVersion,
			BundleName:     expectedBundleName(goos, goarch),
			Engine:         Component{Name: names[0], Mode: 0o755},
			Pack:           Component{Name: names[1], Mode: 0o644},
			Bridge:         Component{Name: names[2], Mode: 0o755},
		}, nil
	}
	return HostSpec{}, fmt.Errorf("unsupported driver target %s/%s", goos, goarch)
}

func validateSPXVersion(value string) error {
	if !strings.HasPrefix(value, "v") || !semver.IsValid(value) || semver.Canonical(value) != value || module.IsPseudoVersion(value) {
		return fmt.Errorf("invalid SPX version %q", value)
	}
	return nil
}

func validateRuntimeVersion(value string) error {
	if !runtimeVersionPattern.MatchString(value) || !semver.IsValid("v"+value) {
		return fmt.Errorf("invalid runtime version %q", value)
	}
	return nil
}

func expectedBundleName(goos, goarch string) string {
	return "spx-driver-" + goos + "-" + goarch + ".zip"
}

func expectedFileNames(runtimeVersion, goos, goarch string) [3]string {
	engine := "gdspxrt" + runtimeVersion
	if goos == "windows" {
		engine += ".exe"
	}
	extension := map[string]string{"darwin": ".dylib", "linux": ".so", "windows": ".dll"}[goos]
	return [3]string{engine, "gdspxrt" + runtimeVersion + ".pck", "gdspx-" + goos + "-" + goarch + extension}
}

func validateSHA256(value string) error {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return errors.New("must be a lower-case 64-hex-character digest")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return errors.New("must be a lower-case 64-hex-character digest")
	}
	return nil
}

func validateBundleName(name string) error {
	if err := validateBaseName(name); err != nil || !strings.HasSuffix(name, ".zip") {
		return fmt.Errorf("invalid bundle name %q", name)
	}
	return nil
}

func validateBaseName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00<>:\"|?*") || name != strings.TrimSpace(name) {
		return errors.New("must be a portable basename")
	}
	for _, r := range name {
		if r < 0x20 {
			return errors.New("contains a control character")
		}
	}
	return nil
}
