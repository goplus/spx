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

// Package assets validates typed resource references in an SPX project.
package assets

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"slices"
	"strings"

	coreproject "github.com/goplus/spx/v4/internal/core/project"
)

// Config identifies a project and its asset indexes.
type Config struct {
	ProjectDir string
	PackDir    string
	PackIndex  string
}

type indexValidator struct {
	root    *os.Root
	packDir string
	legacy  bool
}

// Validate checks that the launcher can archive every configured resource.
func Validate(cfg Config) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}
	root, err := openProjectRoot(cfg.ProjectDir)
	if err != nil {
		return err
	}
	defer root.Close()
	v := indexValidator{root: root, packDir: cfg.PackDir}
	return v.validateIndexes(cfg.PackIndex)
}

// ValidateLegacy checks full project configs without requiring resource files
// to exist. The legacy packer already walks every physical project entry.
func ValidateLegacy(root *os.Root) error {
	v := indexValidator{root: root, packDir: "assets", legacy: true}
	return v.validateIndexes("index.json")
}

func (v *indexValidator) validateIndexes(index string) error {
	if _, err := inspectPath(v.root, v.packDir, true); os.IsNotExist(err) {
		return fmt.Errorf("projectassets: PackDir %q is missing", v.packDir)
	} else if err != nil {
		return err
	}

	packedPath := path.Join(v.packDir, coreproject.PackedIndexFile)
	packed, hasPacked, err := v.readIndex(packedPath)
	if err != nil {
		return err
	}
	if index == coreproject.PackedIndexFile {
		if !hasPacked {
			return fmt.Errorf("projectassets: required config %q is missing", packedPath)
		}
		index = "index.json"
	}
	sourcePath := path.Join(v.packDir, index)
	source, hasSource, err := v.readIndex(sourcePath)
	if err != nil {
		return err
	}
	rootSource := sourcePath
	if hasPacked {
		rootSource = packedPath
	} else if !hasSource {
		return fmt.Errorf("projectassets: PackDir %q contains neither %q nor %q", v.packDir, index, coreproject.PackedIndexFile)
	}

	if err := v.validateProject(rootSource, coreproject.MergeIndexRoot(source, packed)); err != nil {
		return err
	}
	for _, section := range []string{"sprites", "sounds", "fonts"} {
		raw, present := packed[section]
		entries, err := coreproject.ParseIndexEntries(raw)
		if err != nil {
			return fmt.Errorf("projectassets: decode %q section %q: %w", packedPath, section, err)
		}
		for _, name := range slices.Sorted(maps.Keys(entries)) {
			if !coreproject.ValidResourceName(name) {
				return fmt.Errorf("projectassets: section %q has unsafe entry name %q", section, name)
			}
			configDir := path.Join(section, name)
			if err := v.validateChild(section, packedPath+"#"+configDir, configDir, entries[name]); err != nil {
				return fmt.Errorf("projectassets: section %q entry %q: %w", section, name, err)
			}
		}
		if section != "fonts" || !present {
			if err := v.validateSourceChildren(section, entries); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *indexValidator) validateProject(source string, fields map[string]json.RawMessage) error {
	var config resourcePaths
	var bgm *string
	if v.legacy {
		project := new(coreproject.ProjectConfig)
		config, bgm = project, &project.Bgm
	} else {
		project := new(projectConfig)
		config, bgm = project, &project.Bgm
	}
	if err := coreproject.DecodeIndexRoot(fields, config); err != nil {
		return fmt.Errorf("projectassets: decode merged project root at %q: %w", source, err)
	}
	if err := coreproject.ValidateBackgroundSoundName(*bgm); err != nil {
		return fmt.Errorf("projectassets: config %q: %w", source, err)
	}
	return v.validateReferences(source, "", config)
}

func (v *indexValidator) readIndex(name string) (map[string]json.RawMessage, bool, error) {
	data, found, err := readPinnedFile(v.root, name)
	if !found || err != nil {
		return nil, false, err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, false, fmt.Errorf("projectassets: decode %q: %w", name, err)
	}
	return root, true, nil
}

func (v *indexValidator) validateSourceChildren(section string, packed map[string]json.RawMessage) error {
	dir := path.Join(v.packDir, section)
	if v.legacy {
		// A regular file named e.g. "sounds" contains no child indexes.
		if info, err := v.root.Lstat(dir); err == nil && info.Mode().IsRegular() {
			return nil
		}
	}
	children, ok, err := readPinnedDir(v.root, dir)
	if err != nil || !ok {
		return err
	}
	for _, child := range children {
		name := child.Name()
		if child.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("projectassets: config directory %q is a symlink", path.Join(dir, name))
		}
		if !child.IsDir() {
			continue
		}
		if strings.ContainsRune(name, '\\') {
			return fmt.Errorf("projectassets: section %q has unsafe entry name %q in archive path", section, name)
		}
		if _, overridden := packed[name]; overridden {
			continue
		}
		configDir := path.Join(section, name)
		source := path.Join(v.packDir, configDir, "index.json")
		data, found, err := readPinnedFile(v.root, source)
		if err != nil {
			return err
		}
		if found {
			if err := v.validateChild(section, source, configDir, data); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *indexValidator) validateChild(section, source, configDir string, data []byte) error {
	config := v.childConfig(section)
	if err := json.Unmarshal(data, config); err != nil {
		return fmt.Errorf("projectassets: decode %q: %w", source, err)
	}
	return v.validateReferences(source, configDir, config)
}
