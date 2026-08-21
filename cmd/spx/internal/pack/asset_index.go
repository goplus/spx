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

package pack

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

type packedAssetIndex struct {
	Project  coreproject.ProjectConfig
	Sprites  map[string]coreproject.SpriteConfig
	Sounds   map[string]coreproject.SoundConfig
	Fonts    map[string]coreproject.FontFamilyConfig
	HasFonts bool
}

func collectAssetPathRefs(assetRoot string) ([]assetPathRef, error) {
	var refs []assetPathRef

	packed, hasPacked, err := readPackedAssetIndex(assetRoot)
	if err != nil {
		return nil, err
	}

	if hasPacked {
		refs = appendProjectAssetRefs(refs, packed.Project)
	} else {
		projectConfigPath := filepath.Join(assetRoot, sourceIndexName)
		if _, err := os.Stat(projectConfigPath); err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("stat %s: %w", projectConfigPath, err)
			}
		} else {
			var conf coreproject.ProjectConfig
			if err := readJSONFile(projectConfigPath, &conf); err != nil {
				return nil, fmt.Errorf("parse %s: %w", projectConfigPath, err)
			}
			refs = appendProjectAssetRefs(refs, conf)
		}
	}

	spriteRefs, err := collectIndexedAssetRefs(
		assetRoot,
		"sprites",
		packed.Sprites,
		appendSpriteAssetRefs,
		true,
	)
	if err != nil {
		return nil, err
	}
	refs = append(refs, spriteRefs...)

	soundRefs, err := collectIndexedAssetRefs(
		assetRoot,
		"sounds",
		packed.Sounds,
		appendSoundAssetRefs,
		true,
	)
	if err != nil {
		return nil, err
	}
	refs = append(refs, soundRefs...)

	fontRefs, err := collectIndexedAssetRefs(
		assetRoot,
		"fonts",
		packed.Fonts,
		appendFontAssetRefs,
		!hasPacked || !packed.HasFonts,
	)
	if err != nil {
		return nil, err
	}
	refs = append(refs, fontRefs...)

	return refs, nil
}

func collectIndexedAssetRefs[T any](
	assetRoot string,
	category string,
	packed map[string]T,
	appendRefs func([]assetPathRef, string, T) []assetPathRef,
	scanSource bool,
) ([]assetPathRef, error) {
	var refs []assetPathRef

	names := make([]string, 0, len(packed))
	for name := range packed {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		refs = appendRefs(refs, path.Join(category, name), packed[name])
	}

	if !scanSource {
		return refs, nil
	}

	configPaths, err := filepath.Glob(filepath.Join(assetRoot, category, "*", sourceIndexName))
	if err != nil {
		return nil, err
	}
	for _, configPath := range configPaths {
		name := filepath.Base(filepath.Dir(configPath))
		if _, exists := packed[name]; exists {
			continue
		}

		configDir, err := relConfigDir(assetRoot, filepath.Dir(configPath))
		if err != nil {
			return nil, err
		}

		var conf T
		if err := readJSONFile(configPath, &conf); err != nil {
			return nil, fmt.Errorf("parse %s: %w", configPath, err)
		}
		refs = appendRefs(refs, configDir, conf)
	}

	return refs, nil
}

func appendProjectAssetRefs(refs []assetPathRef, conf coreproject.ProjectConfig) []assetPathRef {
	for _, backdrop := range conf.Backdrops {
		if backdrop != nil {
			refs = appendAssetPathRef(refs, "", backdrop.Path)
		}
	}
	refs = appendAssetPathRef(refs, "", conf.Bgm)
	refs = appendAssetPathRef(refs, "", conf.TilemapPath)
	return refs
}

func appendSpriteAssetRefs(refs []assetPathRef, configDir string, conf coreproject.SpriteConfig) []assetPathRef {
	for _, costume := range conf.Costumes {
		if costume != nil {
			refs = appendAssetPathRef(refs, configDir, costume.Path)
		}
	}
	if conf.CostumeSet != nil && conf.CostumeSet.Path != "" {
		refs = appendAssetPathRef(refs, configDir, conf.CostumeSet.Path)
	}
	if conf.CostumeMPSet != nil && conf.CostumeMPSet.Path != "" {
		refs = appendAssetPathRef(refs, configDir, conf.CostumeMPSet.Path)
	}
	return refs
}

func appendSoundAssetRefs(refs []assetPathRef, configDir string, conf coreproject.SoundConfig) []assetPathRef {
	return appendAssetPathRef(refs, configDir, conf.Path)
}

func appendFontAssetRefs(refs []assetPathRef, configDir string, conf coreproject.FontFamilyConfig) []assetPathRef {
	for _, face := range conf.Faces {
		refs = appendAssetPathRef(refs, configDir, face.Path)
	}
	return refs
}

func readPackedAssetIndex(assetRoot string) (packedAssetIndex, bool, error) {
	packedPath := filepath.Join(assetRoot, packedIndexName)
	if _, err := os.Stat(packedPath); err != nil {
		if os.IsNotExist(err) {
			return packedAssetIndex{}, false, nil
		}
		return packedAssetIndex{}, false, fmt.Errorf("stat %s: %w", packedPath, err)
	}

	var root map[string]json.RawMessage
	if err := readJSONFile(packedPath, &root); err != nil {
		return packedAssetIndex{}, false, fmt.Errorf("parse %s: %w", packedPath, err)
	}

	sourceRoot, err := readSourceAssetIndexRoot(assetRoot)
	if err != nil {
		return packedAssetIndex{}, false, err
	}
	mergedRoot := mergePackedRootSections(root, sourceRoot)

	var packed packedAssetIndex
	if err := decodePackedAssetSection(mergedRoot, &packed.Project); err != nil {
		return packedAssetIndex{}, false, fmt.Errorf("parse %s root: %w", packedPath, err)
	}
	packed.Sprites = make(map[string]coreproject.SpriteConfig)
	if err := decodePackedAssetObjects(root["sprites"], packed.Sprites); err != nil {
		return packedAssetIndex{}, false, fmt.Errorf("parse %s sprites: %w", packedPath, err)
	}
	packed.Sounds = make(map[string]coreproject.SoundConfig)
	if err := decodePackedAssetObjects(root["sounds"], packed.Sounds); err != nil {
		return packedAssetIndex{}, false, fmt.Errorf("parse %s sounds: %w", packedPath, err)
	}
	packed.Fonts = make(map[string]coreproject.FontFamilyConfig)
	_, packed.HasFonts = root["fonts"]
	if err := decodePackedAssetObjects(root["fonts"], packed.Fonts); err != nil {
		return packedAssetIndex{}, false, fmt.Errorf("parse %s fonts: %w", packedPath, err)
	}
	return packed, true, nil
}

func readSourceAssetIndexRoot(assetRoot string) (map[string]json.RawMessage, error) {
	projectConfigPath := filepath.Join(assetRoot, sourceIndexName)
	if _, err := os.Stat(projectConfigPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", projectConfigPath, err)
	}

	var root map[string]json.RawMessage
	if err := readJSONFile(projectConfigPath, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", projectConfigPath, err)
	}
	return root, nil
}

func mergePackedRootSections(packedRoot, sourceRoot map[string]json.RawMessage) map[string]json.RawMessage {
	if len(sourceRoot) == 0 {
		return packedRoot
	}

	merged := make(map[string]json.RawMessage, len(sourceRoot)+len(packedRoot))
	for key, value := range sourceRoot {
		merged[key] = value
	}
	for key, value := range packedRoot {
		merged[key] = value
	}
	return merged
}

func decodePackedAssetSection(root map[string]json.RawMessage, dest *coreproject.ProjectConfig) error {
	if len(root) == 0 {
		return nil
	}
	raw, err := json.Marshal(root)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

func decodePackedAssetObjects[T any](raw json.RawMessage, dest map[string]T) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	entries := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &entries); err != nil {
		return err
	}
	for name, entry := range entries {
		var conf T
		if err := json.Unmarshal(entry, &conf); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		dest[name] = conf
	}
	return nil
}
