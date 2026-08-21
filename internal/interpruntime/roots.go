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

// Package interpruntime prepares isolated native interpreter sessions without
// changing the process working directory or environment.
package interpruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ProjectDirEnv             = "SPX_PROJECT_DIR"
	AssetDirEnv               = "SPX_ASSET_DIR"
	SessionDirEnv             = "SPX_SESSION_DIR"
	PortableConfigDirEnv      = "SPX_PORTABLE_CONFIG_DIR"
	PortableConfigIdentityEnv = "SPX_PORTABLE_CONFIG_IDENTITY"
)

// Roots identifies the independent source, asset, and Engine session roots.
// Every path must be absolute, clean, and name an existing directory.
type Roots struct {
	ProjectDir string
	AssetDir   string
	SessionDir string
}

// Validate checks the interpreted runtime path contract.
func (r Roots) Validate() error {
	paths := []struct {
		name          string
		path          string
		rejectSymlink bool
	}{
		{name: "ProjectDir", path: r.ProjectDir},
		{name: "AssetDir", path: r.AssetDir, rejectSymlink: true},
		{name: "SessionDir", path: r.SessionDir, rejectSymlink: true},
	}
	for _, item := range paths {
		if err := validateAbsoluteCleanPath(item.name, item.path); err != nil {
			return err
		}
		if err := validateDirectory(item.name, item.path, item.rejectSymlink); err != nil {
			return err
		}
	}
	if err := validateDirectoryWithin("AssetDir", r.ProjectDir, r.AssetDir); err != nil {
		return err
	}
	return nil
}

func validateDirectoryWithin(name, root, directory string) error {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("interpruntime: canonicalize ProjectDir %q: %w", root, err)
	}
	canonicalDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return fmt.Errorf("interpruntime: canonicalize %s %q: %w", name, directory, err)
	}
	rel, err := filepath.Rel(canonicalRoot, canonicalDirectory)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("interpruntime: %s %q must be within ProjectDir %q", name, directory, root)
	}
	return nil
}

func validateDirectory(name, path string, rejectSymlink bool) error {
	var (
		info os.FileInfo
		err  error
	)
	if rejectSymlink {
		info, err = os.Lstat(path)
	} else {
		info, err = os.Stat(path)
	}
	if err != nil {
		return fmt.Errorf("interpruntime: stat %s %q: %w", name, path, err)
	}
	if rejectSymlink && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("interpruntime: %s %q must not be a symlink", name, path)
	}
	if !info.IsDir() {
		return fmt.Errorf("interpruntime: %s %q is not a directory", name, path)
	}
	return nil
}

func validateAbsoluteCleanPath(name, path string) error {
	if path == "" {
		return fmt.Errorf("interpruntime: %s is empty", name)
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("interpruntime: %s %q is not absolute", name, path)
	}
	if clean := filepath.Clean(path); clean != path {
		return fmt.Errorf("interpruntime: %s %q is not clean (want %q)", name, path, clean)
	}
	return nil
}

// RootsFromEnv parses and validates roots from a complete process environment.
// Duplicate root variables are rejected instead of relying on platform-specific
// duplicate-key precedence.
func RootsFromEnv(env []string) (Roots, error) {
	values := make(map[string]string, 3)
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		canonical, ok := rootEnvKey(key)
		if !ok {
			continue
		}
		if _, duplicate := values[canonical]; duplicate {
			return Roots{}, fmt.Errorf("interpruntime: duplicate environment variable %s", canonical)
		}
		values[canonical] = value
	}

	for _, key := range []string{ProjectDirEnv, AssetDirEnv, SessionDirEnv} {
		if _, ok := values[key]; !ok {
			return Roots{}, fmt.Errorf("interpruntime: required environment variable %s is not set", key)
		}
	}
	r := Roots{
		ProjectDir: values[ProjectDirEnv],
		AssetDir:   values[AssetDirEnv],
		SessionDir: values[SessionDirEnv],
	}
	if err := r.Validate(); err != nil {
		return Roots{}, err
	}
	return r, nil
}

// Environment replaces ambient runtime paths with validated roots.
func (r Roots) Environment(base []string) ([]string, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	env := make([]string, 0, len(base)+3)
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, root := rootEnvKey(key); root {
				continue
			}
		}
		env = append(env, entry)
	}
	return append(env,
		ProjectDirEnv+"="+r.ProjectDir,
		AssetDirEnv+"="+r.AssetDir,
		SessionDirEnv+"="+r.SessionDir,
	), nil
}

// PortableConfigFromEnv returns the optional driver-owned config directory and
// its presence/content identity. Both variables must be present together.
func PortableConfigFromEnv(env []string) (directory, identity string, found bool, err error) {
	values := make(map[string]string, 2)
	for _, entry := range env {
		key, candidate, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		canonical, ok := rootEnvKey(key)
		if !ok || (canonical != PortableConfigDirEnv && canonical != PortableConfigIdentityEnv) {
			continue
		}
		if _, duplicate := values[canonical]; duplicate {
			return "", "", false, fmt.Errorf("interpruntime: duplicate environment variable %s", canonical)
		}
		values[canonical] = candidate
	}
	directory, dirFound := values[PortableConfigDirEnv]
	identity, identityFound := values[PortableConfigIdentityEnv]
	if dirFound != identityFound {
		return "", "", false, fmt.Errorf("interpruntime: portable config environment is incomplete")
	}
	return directory, identity, dirFound, nil
}

func rootEnvKey(key string) (string, bool) {
	return rootEnvKeyForGOOS(key, runtime.GOOS)
}

func rootEnvKeyForGOOS(key, goos string) (string, bool) {
	for _, canonical := range []string{ProjectDirEnv, AssetDirEnv, SessionDirEnv, PortableConfigDirEnv, PortableConfigIdentityEnv} {
		if key == canonical || (goos == "windows" && strings.EqualFold(key, canonical)) {
			return canonical, true
		}
	}
	return "", false
}
