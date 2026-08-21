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

package runtimepayload

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"
)

func validateIdentity(cfg BuildConfig) error {
	required := []struct {
		name  string
		value string
	}{
		{"selected SPX path", cfg.SPX.SelectedPath},
		{"effective SPX path", cfg.SPX.EffectivePath},
		{"target GOOS", cfg.Target.GOOS},
		{"target GOARCH", cfg.Target.GOARCH},
		{"runtime version", cfg.Engine.RuntimeVersion},
		{"engine interface digest", cfg.Engine.EngineInterfaceDigest},
		{"engine executable", cfg.Engine.Executable},
		{"engine pack", cfg.Engine.Pack},
		{"engine bundle digest", cfg.Engine.BundleDigest},
		{"bridge file", cfg.Bridge.File},
		{"bridge bundle digest", cfg.Bridge.BundleDigest},
		{"project pack directory", cfg.Project.PackDirectory},
		{"project bundle digest", cfg.Project.BundleDigest},
		{"project archive digest", cfg.Project.ArchiveSHA256},
	}
	for _, field := range required {
		if field.value == "" {
			return fmt.Errorf("runtimepayload: %s is empty", field.name)
		}
	}
	if cfg.Engine.RuntimeABI <= 0 {
		return fmt.Errorf("runtimepayload: runtime ABI must be positive")
	}
	digests := []struct {
		name  string
		value string
	}{
		{"engine interface digest", cfg.Engine.EngineInterfaceDigest},
		{"engine bundle digest", cfg.Engine.BundleDigest},
		{"bridge bundle digest", cfg.Bridge.BundleDigest},
		{"project bundle digest", cfg.Project.BundleDigest},
		{"project archive digest", cfg.Project.ArchiveSHA256},
	}
	for _, field := range digests {
		if err := validateDigest(field.value); err != nil {
			return fmt.Errorf("runtimepayload: invalid %s: %w", field.name, err)
		}
	}
	for _, name := range []string{cfg.Engine.Executable, cfg.Engine.Pack, cfg.Bridge.File} {
		if path.Base(name) != name || name == "." || strings.ContainsAny(name, `\\:`) {
			return fmt.Errorf("runtimepayload: unsafe component basename %q", name)
		}
	}
	if cfg.Project.PackDirectory == "." || path.Clean(cfg.Project.PackDirectory) != cfg.Project.PackDirectory || strings.HasPrefix(cfg.Project.PackDirectory, "../") || path.IsAbs(cfg.Project.PackDirectory) {
		return fmt.Errorf("runtimepayload: invalid project pack directory %q", cfg.Project.PackDirectory)
	}
	return nil
}

func validateEntryName(name string) error {
	if name == "" || path.Clean(name) != name || path.IsAbs(name) || strings.ContainsRune(name, '\\') {
		return fmt.Errorf("runtimepayload: invalid entry path %q", name)
	}
	for _, component := range strings.Split(name, "/") {
		if component == "" || component == "." || component == ".." {
			return fmt.Errorf("runtimepayload: invalid entry path %q", name)
		}
	}
	return nil
}

func validateDigest(value string) error {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return errors.New("digest must be 64 lower-case hexadecimal characters")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return errors.New("digest must be 64 lower-case hexadecimal characters")
	}
	return nil
}
