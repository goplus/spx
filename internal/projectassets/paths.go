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

package projectassets

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
)

func validateConfig(cfg Config) error {
	if cfg.ProjectDir == "" {
		return errors.New("projectassets: ProjectDir is empty")
	}
	if cfg.PackDir == "" || cfg.PackDir == "." || strings.Contains(cfg.PackDir, "\\") || path.IsAbs(cfg.PackDir) || path.Clean(cfg.PackDir) != cfg.PackDir || cfg.PackDir == ".." || strings.HasPrefix(cfg.PackDir, "../") {
		return fmt.Errorf("projectassets: PackDir must be a clean non-empty relative slash path: %q", cfg.PackDir)
	}
	if cfg.PackIndex == "" || cfg.PackIndex == "." || cfg.PackIndex == ".." || strings.ContainsAny(cfg.PackIndex, "/\\\x00") {
		return fmt.Errorf("projectassets: PackIndex must be a plain file name: %q", cfg.PackIndex)
	}
	return nil
}

func validateConfigEntryName(section, name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return fmt.Errorf("projectassets: section %q has unsafe entry name %q", section, name)
	}
	return nil
}

func (r *resolver) addReference(source, base, reference string) error {
	if reference == "" {
		return nil
	}
	if strings.ContainsAny(reference, "\\\x00") {
		return fmt.Errorf("projectassets: config %q has unsafe resource path %q", source, reference)
	}

	var name string
	switch {
	case strings.HasPrefix(reference, "res://"):
		name = strings.TrimPrefix(reference, "res://")
		if name == "" || path.IsAbs(name) {
			return fmt.Errorf("projectassets: config %q references absolute resource path %q", source, reference)
		}
		if strings.Contains(name, ":") {
			return fmt.Errorf("projectassets: config %q has unsupported resource path %q", source, reference)
		}
	case strings.Contains(reference, ":"):
		return fmt.Errorf("projectassets: config %q has unsupported resource path %q", source, reference)
	case path.IsAbs(reference):
		return fmt.Errorf("projectassets: config %q references absolute resource path %q", source, reference)
	default:
		name = path.Join(base, reference)
	}
	name = path.Clean(name)
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || path.IsAbs(name) {
		return fmt.Errorf("projectassets: config %q resource %q escapes ProjectDir (outside project directory)", source, reference)
	}
	if err := pinRegularFile(r.root, name); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("projectassets: config %q resource %q is missing", source, reference)
		}
		return fmt.Errorf("projectassets: config %q resource %q: %w", source, reference, err)
	}
	if name != r.packDir && !strings.HasPrefix(name, r.packDir+"/") {
		r.referenced[name] = struct{}{}
	}
	return nil
}
