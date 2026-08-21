//go:build !js || !wasm

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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validatePortableConfigDir(sessionDir, configDir string) error {
	if configDir == "" {
		return fmt.Errorf("ispxnative: portable config directory is empty")
	}
	if !filepath.IsAbs(configDir) {
		return fmt.Errorf("ispxnative: portable config directory %q is not absolute", configDir)
	}
	if clean := filepath.Clean(configDir); clean != configDir {
		return fmt.Errorf("ispxnative: portable config directory %q is not clean (want %q)", configDir, clean)
	}
	info, err := os.Lstat(configDir)
	if err != nil {
		return fmt.Errorf("ispxnative: inspect portable config directory %q: %w", configDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("ispxnative: portable config directory %q must be a real directory", configDir)
	}
	canonicalSession, err := filepath.EvalSymlinks(sessionDir)
	if err != nil {
		return fmt.Errorf("ispxnative: canonicalize session directory %q: %w", sessionDir, err)
	}
	canonicalConfig, err := filepath.EvalSymlinks(configDir)
	if err != nil {
		return fmt.Errorf("ispxnative: canonicalize portable config directory %q: %w", configDir, err)
	}
	rel, err := filepath.Rel(canonicalSession, canonicalConfig)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("ispxnative: portable config directory %q must be below session directory %q", configDir, sessionDir)
	}
	return nil
}

func openPortableConfigRoot(sessionDir, configDir string) (*os.Root, error) {
	rel, err := filepath.Rel(sessionDir, configDir)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return nil, fmt.Errorf("ispxnative: portable config directory %q is outside session directory %q", configDir, sessionDir)
	}
	sessionBefore, err := os.Lstat(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("ispxnative: inspect session directory: %w", err)
	}
	if sessionBefore.Mode()&os.ModeSymlink != 0 || !sessionBefore.IsDir() {
		return nil, fmt.Errorf("ispxnative: session directory %q must be a real directory", sessionDir)
	}
	sessionRoot, err := os.OpenRoot(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("ispxnative: open session directory: %w", err)
	}
	opened, statErr := sessionRoot.Stat(".")
	sessionAfter, pathErr := os.Lstat(sessionDir)
	if statErr != nil || pathErr != nil || opened == nil || !opened.IsDir() || !os.SameFile(sessionBefore, opened) || !os.SameFile(opened, sessionAfter) {
		_ = sessionRoot.Close()
		return nil, fmt.Errorf("ispxnative: session directory changed while opening")
	}
	before, err := sessionRoot.Lstat(rel)
	if err != nil {
		_ = sessionRoot.Close()
		return nil, fmt.Errorf("ispxnative: inspect portable config directory: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		_ = sessionRoot.Close()
		return nil, fmt.Errorf("ispxnative: portable config directory %q must be a real directory", configDir)
	}
	configRoot, err := sessionRoot.OpenRoot(rel)
	if err != nil {
		_ = sessionRoot.Close()
		return nil, fmt.Errorf("ispxnative: open portable config directory: %w", err)
	}
	openedConfig, statErr := configRoot.Stat(".")
	pathAfter, pathErr := sessionRoot.Lstat(rel)
	if statErr != nil || pathErr != nil || openedConfig == nil || !os.SameFile(before, openedConfig) || !os.SameFile(openedConfig, pathAfter) {
		_ = configRoot.Close()
		_ = sessionRoot.Close()
		return nil, fmt.Errorf("ispxnative: portable config directory changed while opening")
	}
	if err := sessionRoot.Close(); err != nil {
		_ = configRoot.Close()
		return nil, fmt.Errorf("ispxnative: close session directory: %w", err)
	}
	return configRoot, nil
}
