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

// Package projectpolicy contains validation for portable SPX projects.
package projectpolicy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configName = ".config"
	// Keep portable policy reads within the bundle's default per-file budget.
	maxPortableConfigBytes = 64 << 20

	portableConfigAbsentIdentity = "absent"
	portableConfigSHA256Prefix   = "sha256:"
)

// PortableConfigSnapshot binds the validated portable configuration to the
// exact file identity and bytes that were inspected. Its fields are private so
// callers cannot manufacture a snapshot that bypasses validation.
type PortableConfigSnapshot struct {
	ready    bool
	found    bool
	data     []byte
	digest   [sha256.Size]byte
	fileInfo os.FileInfo
}

// Present reports whether .config existed when the snapshot was captured.
func (s PortableConfigSnapshot) Present() bool {
	return s.ready && s.found
}

// Bytes returns an immutable copy of the validated .config contents. It
// returns nil when .config was absent.
func (s PortableConfigSnapshot) Bytes() []byte {
	if !s.Present() {
		return nil
	}
	data := make([]byte, len(s.data))
	copy(data, s.data)
	return data
}

// Identity binds both .config presence and its exact bytes. It is suitable for
// carrying the expected snapshot across a process boundary.
func (s PortableConfigSnapshot) Identity() (string, error) {
	if !s.ready {
		return "", fmt.Errorf("portable project config snapshot is uninitialized")
	}
	if !s.found {
		return portableConfigAbsentIdentity, nil
	}
	return portableConfigSHA256Prefix + hex.EncodeToString(s.digest[:]), nil
}

// ValidatePortableConfig rejects configuration that cannot be captured in a
// self-contained project snapshot for the driver. A missing .config is valid;
// when present it must be a stable regular non-symlink file.
func ValidatePortableConfig(projectDir string) error {
	_, err := SnapshotPortableConfig(projectDir)
	return err
}

// SnapshotPortableConfig validates .config and retains the exact identity and
// bytes for later use by a portable run or build.
func SnapshotPortableConfig(projectDir string) (PortableConfigSnapshot, error) {
	configPath := filepath.Join(projectDir, configName)
	root, err := os.OpenRoot(projectDir)
	if err != nil {
		return PortableConfigSnapshot{}, fmt.Errorf("validate project config %q: open project root: %w", configPath, err)
	}
	defer root.Close()
	data, info, found, err := readStableRegularRootFile(root, configName)
	if err != nil {
		return PortableConfigSnapshot{}, fmt.Errorf("validate project config %q: %w", configPath, err)
	}
	return makePortableConfigSnapshot(configPath, data, info, found)
}

// SnapshotPortableConfigRoot reads .config through a pinned directory handle.
// It is used at process boundaries where path containment would leave a
// replacement window between validation and read.
func SnapshotPortableConfigRoot(root *os.Root) (PortableConfigSnapshot, error) {
	if root == nil {
		return PortableConfigSnapshot{}, fmt.Errorf("portable project config root is nil")
	}
	data, info, found, err := readStableRegularRootFile(root, configName)
	if err != nil {
		return PortableConfigSnapshot{}, fmt.Errorf("read portable project config: %w", err)
	}
	return makePortableConfigSnapshot(configName, data, info, found)
}

func makePortableConfigSnapshot(configPath string, data []byte, info os.FileInfo, found bool) (PortableConfigSnapshot, error) {
	if !found {
		return PortableConfigSnapshot{ready: true}, nil
	}

	var config struct {
		ExtAsset json.RawMessage `json:"extasset"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return PortableConfigSnapshot{}, fmt.Errorf("parse project config %q: %w", configPath, err)
	}
	if config.ExtAsset != nil {
		return PortableConfigSnapshot{}, fmt.Errorf("project config %q uses unsupported extasset; move resources inside the project directory", configPath)
	}
	return PortableConfigSnapshot{
		ready: true, found: true, data: data, digest: sha256.Sum256(data), fileInfo: info,
	}, nil
}

// Verify confirms that the current .config still has the identity, metadata,
// and bytes captured by the snapshot. Once this succeeds, callers should use
// Bytes rather than reopen the user-controlled file.
func (s PortableConfigSnapshot) Verify(projectDir string) error {
	if !s.ready {
		return fmt.Errorf("portable project config snapshot is uninitialized")
	}
	configPath := filepath.Join(projectDir, configName)
	data, info, found, err := readStableRegularFile(configPath)
	if err != nil {
		return fmt.Errorf("revalidate project config %q: %w", configPath, err)
	}
	if found != s.found {
		return fmt.Errorf("project config %q changed after validation", configPath)
	}
	if !found {
		return nil
	}
	if !os.SameFile(s.fileInfo, info) || !stableMetadata(s.fileInfo, info) ||
		sha256.Sum256(data) != s.digest || !bytes.Equal(data, s.data) {
		return fmt.Errorf("project config %q changed after validation", configPath)
	}
	return nil
}
