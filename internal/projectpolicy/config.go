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

// Package projectpolicy contains mode-independent validation for SPX project
// metadata. Keeping it outside command packages makes interpreted, native and
// export entry points enforce the same project contract.
package projectpolicy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const configName = ".config"

// ValidateConfig rejects legacy external-resource configuration. A missing
// .config is valid; when present it must be a stable regular non-symlink file.
func ValidateConfig(projectDir string) error {
	configPath := filepath.Join(projectDir, configName)
	data, found, err := readStableRegularFile(configPath)
	if err != nil {
		return fmt.Errorf("validate project config %q: %w", configPath, err)
	}
	if !found {
		return nil
	}

	var config struct {
		ExtAsset json.RawMessage `json:"extasset"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse project config %q: %w", configPath, err)
	}
	if config.ExtAsset != nil {
		return fmt.Errorf("project config %q uses unsupported extasset; move resources inside the project directory", configPath)
	}
	return nil
}

func readStableRegularFile(name string) ([]byte, bool, error) {
	before, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, false, fmt.Errorf("must be a regular non-symlink file")
	}

	file, err := os.Open(name)
	if err != nil {
		return nil, false, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, false, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		file.Close()
		return nil, false, fmt.Errorf("changed while opening")
	}
	data, readErr := io.ReadAll(file)
	afterOpened, statErr := file.Stat()
	closeErr := file.Close()
	afterPath, lstatErr := os.Lstat(name)
	if readErr != nil {
		return nil, false, readErr
	}
	if statErr != nil {
		return nil, false, statErr
	}
	if closeErr != nil {
		return nil, false, closeErr
	}
	if lstatErr != nil || afterPath.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(opened, afterOpened) || !os.SameFile(afterOpened, afterPath) ||
		int64(len(data)) != opened.Size() || !stableMetadata(opened, afterOpened) {
		return nil, false, fmt.Errorf("changed while reading")
	}
	return data, true, nil
}

func stableMetadata(before, after os.FileInfo) bool {
	return before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime() == after.ModTime()
}
