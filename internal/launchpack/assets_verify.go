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

package launchpack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/goplus/spx/v3/internal/driverbundle"
)

// Verify checks the files represented by the asset set. Published assets are
// checked against their immutable component identity; source assets are only
// checked for regular, non-symlink paths.
func (a Assets) Verify() error {
	for _, file := range []struct {
		label string
		path  string
	}{
		{"Engine", a.EnginePath}, {"runtime PCK", a.PackPath}, {"interpreter bridge", a.BridgePath},
	} {
		if file.path == "" {
			return fmt.Errorf("launchpack: %s path is required", file.label)
		}
		if err := validateRuntimeFile(file.path, file.label); err != nil {
			return err
		}
	}
	if a.Published == nil {
		return nil
	}
	if err := a.Published.validate(); err != nil {
		return err
	}
	engine, err := assetDigest("Engine", a.EnginePath, a.Published.EngineSHA256)
	if err != nil {
		return err
	}
	pack, err := assetDigest("runtime PCK", a.PackPath, a.Published.PackSHA256)
	if err != nil {
		return err
	}
	bridge, err := assetDigest("interpreter bridge", a.BridgePath, a.Published.BridgeSHA256)
	if err != nil {
		return err
	}
	interfaceDigest, err := driverbundle.ComputeEngineInterfaceDigestFromSHA256(engine, pack)
	if err != nil {
		return fmt.Errorf("launchpack: compute published Engine interface: %w", err)
	}
	return a.Published.verifyDigests(engine, pack, bridge, interfaceDigest)
}

func (p PublishedDriverIdentity) validate() error {
	for _, item := range []struct{ name, digest string }{
		{"manifest", p.ManifestSHA256}, {"bundle", p.BundleSHA256},
		{"Engine", p.EngineSHA256}, {"runtime PCK", p.PackSHA256},
		{"interpreter bridge", p.BridgeSHA256}, {"Engine interface", p.EngineInterfaceDigest},
	} {
		if !validSHA256(item.digest) {
			return fmt.Errorf("launchpack: published driver %s SHA-256 is invalid", item.name)
		}
	}
	for _, item := range []struct{ name, value string }{
		{"bundle name", p.BundleName}, {"SPX version", p.SPXVersion},
	} {
		if strings.TrimSpace(item.value) == "" {
			return fmt.Errorf("launchpack: published driver %s is required", item.name)
		}
	}
	return nil
}

func (p PublishedDriverIdentity) verifyDigests(engine, pack, bridge, interfaceDigest string) error {
	for _, values := range [][3]string{
		{"Engine", engine, p.EngineSHA256}, {"runtime PCK", pack, p.PackSHA256},
		{"interpreter bridge", bridge, p.BridgeSHA256},
	} {
		name, got, want := values[0], values[1], values[2]
		if got != want {
			return fmt.Errorf("launchpack: published %s SHA-256 = %s, want %s", name, got, want)
		}
	}
	if interfaceDigest != p.EngineInterfaceDigest {
		return fmt.Errorf("launchpack: published Engine interface digest = %s, want %s", interfaceDigest, p.EngineInterfaceDigest)
	}
	return nil
}

func assetDigest(label, path, want string) (string, error) {
	size, digest, err := hashRuntimeFile(path)
	if err != nil {
		return "", fmt.Errorf("launchpack: hash %s: %w", label, err)
	}
	if digest != want {
		return "", fmt.Errorf("launchpack: %s SHA-256 = %s, want %s (size %d)", label, digest, want, size)
	}
	return digest, nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}
