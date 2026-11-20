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

package assets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	spxfs "github.com/goplus/spx/v4/fs"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/core/tilemap"
)

func (v *indexValidator) validateReferences(source, configDir string, config resourcePaths) error {
	resolve := coreproject.ResolveResourcePath
	if _, ok := config.(*coreproject.FontFamilyConfig); ok {
		resolve = coreproject.ResolveFontFacePath
	}
	for reference := range config.ResourcePaths {
		if !v.legacy && strings.Contains(*reference, ":") {
			return fmt.Errorf("projectassets: config %q has unsupported resource path %q", source, *reference)
		}
		name, err := resolve(configDir, *reference, v.legacy)
		if err != nil {
			return fmt.Errorf("projectassets: config %q: %w", source, err)
		}
		if v.legacy || name == "" {
			continue
		}
		if project, ok := config.(*projectConfig); ok && reference == &project.TilemapPath {
			name = tilemap.ConfigPath(name)
		}
		if err := verifyRegularFile(v.root, path.Join(v.packDir, name)); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("projectassets: config %q resource %q is missing", source, *reference)
			}
			return fmt.Errorf("projectassets: config %q resource %q: %w", source, *reference, err)
		}
	}
	return nil
}

func validateConfig(cfg Config) error {
	if cfg.ProjectDir == "" {
		return errors.New("projectassets: ProjectDir is empty")
	}
	if cfg.PackDir == "." || !fs.ValidPath(cfg.PackDir) || spxfs.IsAbsResourcePath(cfg.PackDir) || strings.ContainsAny(cfg.PackDir, "\\\x00") {
		return fmt.Errorf("projectassets: PackDir must be a clean non-empty relative slash path: %q", cfg.PackDir)
	}
	if cfg.PackIndex == "" || cfg.PackIndex == "." || cfg.PackIndex == ".." || strings.ContainsAny(cfg.PackIndex, "/\\\x00") {
		return fmt.Errorf("projectassets: PackIndex must be a plain file name: %q", cfg.PackIndex)
	}
	return nil
}
