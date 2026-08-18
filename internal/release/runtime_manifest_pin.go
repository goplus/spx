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
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/goplus/spx/v3/internal/strictjson"
)

const RuntimeManifestPinSchema = 1

// RuntimeManifestPin authenticates the release manifest which, in turn,
// authenticates every runtime asset. It is kept separate from RuntimeLock so
// adding a post-build release digest does not change the lock identity stored
// inside an already-published manifest.
type RuntimeManifestPin struct {
	Schema         int    `json:"schema"`
	RuntimeVersion string `json:"runtime_version"`
	Name           string `json:"name"`
	Size           int64  `json:"size"`
	SHA256         string `json:"sha256"`
}

var (
	//go:embed runtime_manifest_pins/*.json
	embeddedRuntimeManifestPins embed.FS
	runtimeManifestPins         = mustLoadRuntimeManifestPins(embeddedRuntimeManifestPins)
)

func ParseRuntimeManifestPin(data []byte) (RuntimeManifestPin, error) {
	var pin RuntimeManifestPin
	if err := strictjson.Decode(data, &pin); err != nil {
		return RuntimeManifestPin{}, fmt.Errorf("decode runtime manifest pin: %w", err)
	}
	if err := pin.Validate(); err != nil {
		return RuntimeManifestPin{}, err
	}
	return pin, nil
}

func (p RuntimeManifestPin) Validate() error {
	if p.Schema != RuntimeManifestPinSchema {
		return fmt.Errorf("release: runtime manifest pin schema = %d, want %d", p.Schema, RuntimeManifestPinSchema)
	}
	if !runtimeVersionPattern.MatchString(p.RuntimeVersion) {
		return fmt.Errorf("release: invalid pinned runtime version %q", p.RuntimeVersion)
	}
	if err := validateBaseName("pinned runtime manifest", p.Name); err != nil {
		return err
	}
	if p.Size <= 0 {
		return fmt.Errorf("release: pinned runtime manifest size must be positive")
	}
	if !isLowerHexDigest(p.SHA256, sha256.Size*2) {
		return fmt.Errorf("release: invalid pinned runtime manifest SHA-256 %q", p.SHA256)
	}
	return nil
}

func (p RuntimeManifestPin) ValidateForLock(lock RuntimeLock) error {
	if err := lock.Validate(); err != nil {
		return err
	}
	if err := p.Validate(); err != nil {
		return err
	}
	if p.RuntimeVersion != lock.RuntimeVersion || p.Name != lock.Manifest {
		return fmt.Errorf("release: runtime manifest pin does not match lock")
	}
	return nil
}

// RuntimeManifestPinForLock returns the immutable release-manifest identity
// paired with lock. Missing pins fail closed instead of accepting a manifest
// which merely claims to match the lock.
func RuntimeManifestPinForLock(lock RuntimeLock) (RuntimeManifestPin, error) {
	if err := lock.Validate(); err != nil {
		return RuntimeManifestPin{}, err
	}
	pin, ok := runtimeManifestPins[lock.RuntimeVersion]
	if !ok {
		return RuntimeManifestPin{}, fmt.Errorf("release: no runtime manifest pin for version %q", lock.RuntimeVersion)
	}
	if err := pin.ValidateForLock(lock); err != nil {
		return RuntimeManifestPin{}, err
	}
	return pin, nil
}

func mustLoadRuntimeManifestPins(fileSystem fs.FS) map[string]RuntimeManifestPin {
	files, err := fs.Glob(fileSystem, "runtime_manifest_pins/*.json")
	if err != nil {
		panic("release: list runtime manifest pins: " + err.Error())
	}
	pins := make(map[string]RuntimeManifestPin, len(files))
	for _, file := range files {
		data, err := fs.ReadFile(fileSystem, file)
		if err != nil {
			panic("release: read runtime manifest pin: " + err.Error())
		}
		pin, err := ParseRuntimeManifestPin(data)
		if err != nil {
			panic("release: invalid runtime manifest pin: " + err.Error())
		}
		version := strings.TrimSuffix(path.Base(file), ".json")
		if pin.RuntimeVersion != version {
			panic(fmt.Sprintf("release: runtime manifest pin %s declares version %q", file, pin.RuntimeVersion))
		}
		if _, exists := pins[version]; exists {
			panic("release: duplicate runtime manifest pin for " + version)
		}
		pins[version] = pin
	}
	return pins
}
