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

// Package driverbundle describes published SPX project-driver bundles.
package driverbundle

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/goplus/spx/v3/internal/strictjson"
)

const (
	ManifestSchema                    = 1
	ManifestName                      = "driver-manifest.json"
	SPXModulePath                     = "github.com/goplus/spx/v3"
	ReleaseRepository                 = "goplus/spx"
	EngineInterfaceDigestDomain       = "spx-engine-interface/v1\x00"
	MaxManifestSize             int64 = 16 << 20
)

var ErrBundleNotFound = errors.New("driverbundle: bundle not found")

// Manifest identifies all platform bundles shipped with one SPX release.
type Manifest struct {
	Schema         int      `json:"schema"`
	SPXVersion     string   `json:"spx_version"`
	RuntimeVersion string   `json:"runtime_version"`
	Bundles        []Bundle `json:"bundles"`
}

// Bundle identifies one Engine+PCK+bridge ZIP.
type Bundle struct {
	GOOS                  string `json:"goos"`
	GOARCH                string `json:"goarch"`
	Name                  string `json:"name"`
	Size                  int64  `json:"size"`
	SHA256                string `json:"sha256"`
	EngineInterfaceDigest string `json:"engine_interface_digest"`
	Files                 []File `json:"files"` // Engine, PCK, then bridge.
}

// File identifies one regular file in a bundle.
type File struct {
	Name   string `json:"name"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Parse strictly decodes and validates a manifest.
func Parse(data []byte) (Manifest, error) {
	if int64(len(data)) > MaxManifestSize {
		return Manifest{}, fmt.Errorf("driverbundle: manifest exceeds %d-byte limit", MaxManifestSize)
	}
	var manifest Manifest
	if err := strictjson.Decode(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("driverbundle: decode manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// ParseForVersions strictly decodes a manifest and binds it to the selected
// driver and runtime versions without repeating structural validation.
func ParseForVersions(data []byte, spxVersion, runtimeVersion string) (Manifest, error) {
	manifest, err := Parse(data)
	if err != nil {
		return Manifest{}, err
	}
	if err := manifest.validateVersions(spxVersion, runtimeVersion); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// JSON returns canonical, human-readable manifest bytes.
func (m Manifest) JSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("driverbundle: encode manifest: %w", err)
	}
	return append(data, '\n'), nil
}

// ParseBundle strictly decodes one platform descriptor.
func ParseBundle(data []byte) (Bundle, error) {
	if int64(len(data)) > MaxManifestSize {
		return Bundle{}, fmt.Errorf("driverbundle: bundle descriptor exceeds %d-byte limit", MaxManifestSize)
	}
	var bundle Bundle
	if err := strictjson.Decode(data, &bundle); err != nil {
		return Bundle{}, fmt.Errorf("driverbundle: decode bundle: %w", err)
	}
	if err := bundle.Validate(); err != nil {
		return Bundle{}, err
	}
	return bundle, nil
}

// JSON returns canonical, human-readable bundle bytes.
func (b Bundle) JSON() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("driverbundle: encode bundle: %w", err)
	}
	return append(data, '\n'), nil
}

// BundleFor returns the bundle for goos/goarch.
func (m Manifest) BundleFor(goos, goarch string) (Bundle, error) {
	if err := m.Validate(); err != nil {
		return Bundle{}, err
	}
	for _, bundle := range m.Bundles {
		if bundle.GOOS == goos && bundle.GOARCH == goarch {
			return bundle, nil
		}
	}
	return Bundle{}, fmt.Errorf("%w: %s/%s", ErrBundleNotFound, goos, goarch)
}

// DownloadURL returns the bundle URL in the owning SPX release.
func (m Manifest) DownloadURL(name string) (string, error) {
	return ReleaseAssetURL(m.SPXVersion, name)
}

// ManifestURL returns the manifest URL in the SPX release selected by an exact version.
func ManifestURL(spxVersion string) (string, error) {
	return ReleaseAssetURL(spxVersion, ManifestName)
}

// ReleaseAssetURL returns the canonical SPX release URL for one driver asset.
func ReleaseAssetURL(spxVersion, name string) (string, error) {
	if err := validateSPXVersion(spxVersion); err != nil {
		return "", err
	}
	if name == ManifestName {
		if err := validateBaseName(name); err != nil {
			return "", err
		}
	} else if err := validateBundleName(name); err != nil {
		return "", err
	}
	return "https://github.com/" + ReleaseRepository + "/releases/download/" + spxVersion + "/" + name, nil
}
