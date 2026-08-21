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

// Package projectassets resolves typed resource references in an SPX project.
package projectassets

import (
	"fmt"
	"path"
	"sort"
)

const packedIndexFile = "index_pack.json"

// Config identifies a project and its asset indexes.
type Config struct {
	ProjectDir string
	PackDir    string
	PackIndex  string
}

// Resolution is the stable set of referenced files outside PackDir.
type Resolution struct {
	Files          []string
	HasSourceIndex bool
	HasPackedIndex bool
}

// Resolve validates typed resource references and returns their sorted paths.
// A packed index overrides source root fields and same-name child entries.
func Resolve(cfg Config) (Resolution, error) {
	if err := validateConfig(cfg); err != nil {
		return Resolution{}, err
	}
	root, err := openProjectRoot(cfg.ProjectDir)
	if err != nil {
		return Resolution{}, err
	}
	defer root.Close()

	if _, ok, err := readPinnedDir(root, cfg.PackDir); err != nil {
		return Resolution{}, err
	} else if !ok {
		return Resolution{}, fmt.Errorf("projectassets: PackDir %q is missing", cfg.PackDir)
	}

	sourceName := path.Join(cfg.PackDir, cfg.PackIndex)
	packedName := path.Join(cfg.PackDir, packedIndexFile)
	var sourceData []byte
	var hasSource bool
	if sourceName != packedName {
		sourceData, hasSource, err = readPinnedFile(root, sourceName)
		if err != nil {
			return Resolution{}, err
		}
	}
	packedData, hasPacked, err := readPinnedFile(root, packedName)
	if err != nil {
		return Resolution{}, err
	}
	if !hasSource && !hasPacked {
		if sourceName == packedName {
			return Resolution{}, fmt.Errorf("projectassets: required config %q is missing", packedName)
		}
		return Resolution{}, fmt.Errorf("projectassets: PackDir %q contains neither %q nor %q", cfg.PackDir, cfg.PackIndex, packedIndexFile)
	}

	resolved, err := parseConfig(sourceName, sourceData, packedName, packedData)
	if err != nil {
		return Resolution{}, err
	}
	collector := resolver{
		root:       root,
		packDir:    cfg.PackDir,
		referenced: make(map[string]struct{}),
	}
	rootSource := sourceName
	if hasPacked {
		rootSource = packedName
	}
	if err := collector.collectProject(rootSource, resolved.project); err != nil {
		return Resolution{}, err
	}
	if err := collector.collectSprites(resolved.sprites); err != nil {
		return Resolution{}, err
	}
	if err := collector.collectSounds(resolved.sounds); err != nil {
		return Resolution{}, err
	}
	if err := collector.collectFonts(resolved.fonts, resolved.hasFonts); err != nil {
		return Resolution{}, err
	}

	files := make([]string, 0, len(collector.referenced))
	for name := range collector.referenced {
		files = append(files, name)
	}
	sort.Strings(files)
	return Resolution{Files: files, HasSourceIndex: hasSource, HasPackedIndex: hasPacked}, nil
}

// Collect returns the referenced files. Resolve is preferred when index
// presence is also needed.
func Collect(cfg Config) ([]string, error) {
	resolved, err := Resolve(cfg)
	if err != nil {
		return nil, err
	}
	return resolved.Files, nil
}
