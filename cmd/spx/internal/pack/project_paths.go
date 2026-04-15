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
	"strings"

	spxfs "github.com/goplus/spx/v2/fs"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
)

const (
	projectConfigName = ".config"
	packedIndexName   = "index_pack.json"
	// engineExtAssetDir is the extasset zip root.
	engineExtAssetDir = "extasset"
	// sharedAssetEscapeDepth is the minimum "../" depth.
	sharedAssetEscapeDepth = 2
)

type assetProjectConfig struct {
	// ExtAsset is the external asset directory.
	ExtAsset string `json:"extasset"`
}

type assetPathRef struct {
	configDir string
	path      string
}

type packedAssetIndex struct {
	Project coreproject.ProjectConfig
	Sprites map[string]coreproject.SpriteConfig
	Sounds  map[string]coreproject.SoundConfig
}

// collectExternalAssetPaths matches runtime asset lookup.
func collectExternalAssetPaths(baseFolder string, existingZipPaths map[string]struct{}) ([]DirInfos, error) {
	assetRoot := filepath.Join(baseFolder, "assets")
	info, err := os.Stat(assetRoot)
	if os.IsNotExist(err) || (err == nil && !info.IsDir()) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	refs, err := collectAssetPathRefs(assetRoot)
	if err != nil {
		return nil, err
	}

	extAssetDir, err := readExtAssetDir(baseFolder)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(existingZipPaths))
	for zipPath := range existingZipPaths {
		seen[zipPath] = struct{}{}
	}

	assetRoot = cleanFilesystemPath(assetRoot)
	compatibilityRoot := sharedAssetCompatibilityRoot(assetRoot)

	var extraPaths []DirInfos
	for _, ref := range refs {
		normalized := normalizeConfigPath(ref.configDir, ref.path)
		sourcePath, zipPath, ok := resolveExternalAssetPath(assetRoot, compatibilityRoot, extAssetDir, normalized)
		if !ok {
			continue
		}
		if _, exists := seen[zipPath]; exists {
			continue
		}

		info, err := os.Stat(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("stat external asset %s referenced by %q: %w", sourcePath, normalized, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("external asset %s referenced by %q is a directory", sourcePath, normalized)
		}

		seen[zipPath] = struct{}{}
		extraPaths = append(extraPaths, DirInfos{path: sourcePath, info: info, zipPath: zipPath})
	}

	return extraPaths, nil
}

func collectAssetPathRefs(assetRoot string) ([]assetPathRef, error) {
	var refs []assetPathRef
	packed, hasPacked, err := readPackedAssetIndex(assetRoot)
	if err != nil {
		return nil, err
	}

	projectConfigPath := filepath.Join(assetRoot, "index.json")
	if _, err := os.Stat(projectConfigPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat %s: %w", projectConfigPath, err)
		}
		if hasPacked {
			refs = appendProjectAssetRefs(refs, packed.Project)
		}
	} else {
		var conf coreproject.ProjectConfig
		if err := readJSONFile(projectConfigPath, &conf); err != nil {
			return nil, fmt.Errorf("parse %s: %w", projectConfigPath, err)
		}
		refs = appendProjectAssetRefs(refs, conf)
	}

	spriteConfigPaths, err := filepath.Glob(filepath.Join(assetRoot, "sprites", "*", "index.json"))
	if err != nil {
		return nil, err
	}
	for _, spriteConfigPath := range spriteConfigPaths {
		configDir, err := relConfigDir(assetRoot, filepath.Dir(spriteConfigPath))
		if err != nil {
			return nil, err
		}

		var conf coreproject.SpriteConfig
		if err := readJSONFile(spriteConfigPath, &conf); err != nil {
			return nil, fmt.Errorf("parse %s: %w", spriteConfigPath, err)
		}
		refs = appendSpriteAssetRefs(refs, configDir, conf)
	}
	if hasPacked {
		for name, conf := range packed.Sprites {
			configPath := filepath.Join(assetRoot, "sprites", name, "index.json")
			if _, err := os.Stat(configPath); err == nil {
				continue
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("stat %s: %w", configPath, err)
			}
			refs = appendSpriteAssetRefs(refs, path.Join("sprites", name), conf)
		}
	}

	soundConfigPaths, err := filepath.Glob(filepath.Join(assetRoot, "sounds", "*", "index.json"))
	if err != nil {
		return nil, err
	}
	for _, soundConfigPath := range soundConfigPaths {
		configDir, err := relConfigDir(assetRoot, filepath.Dir(soundConfigPath))
		if err != nil {
			return nil, err
		}

		var conf coreproject.SoundConfig
		if err := readJSONFile(soundConfigPath, &conf); err != nil {
			return nil, fmt.Errorf("parse %s: %w", soundConfigPath, err)
		}
		refs = appendSoundAssetRefs(refs, configDir, conf)
	}
	if hasPacked {
		for name, conf := range packed.Sounds {
			configPath := filepath.Join(assetRoot, "sounds", name, "index.json")
			if _, err := os.Stat(configPath); err == nil {
				continue
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("stat %s: %w", configPath, err)
			}
			refs = appendSoundAssetRefs(refs, path.Join("sounds", name), conf)
		}
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

	var packed packedAssetIndex
	if err := decodePackedAssetSection(root, &packed.Project); err != nil {
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
	return packed, true, nil
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

func appendAssetPathRef(refs []assetPathRef, configDir, relPath string) []assetPathRef {
	if relPath == "" {
		return refs
	}
	return append(refs, assetPathRef{configDir: configDir, path: relPath})
}

// resolveExternalAssetPath resolves external assets for packing.
func resolveExternalAssetPath(assetRoot, compatibilityRoot, extAssetDir, relPath string) (string, string, bool) {
	if relPath == "" || strings.HasPrefix(relPath, "/") {
		return "", "", false
	}
	if schema, _ := spxfs.SplitSchema(relPath); schema != "" {
		return "", "", false
	}

	sourcePath := cleanFilesystemPath(filepath.Join(assetRoot, filepath.FromSlash(relPath)))
	if zipPath := rewriteExtAssetZipPath(relPath, extAssetDir); zipPath != "" {
		if isWithinRoot(sourcePath, compatibilityRoot) {
			return sourcePath, zipPath, true
		}
		return "", "", false
	}

	if isWithinRoot(sourcePath, assetRoot) {
		return "", "", false
	}
	// Allow legacy shared assets outside assets/.
	if leadingParentCount(relPath) < sharedAssetEscapeDepth || !isWithinRoot(sourcePath, compatibilityRoot) {
		return "", "", false
	}

	zipPath, err := filepath.Rel(compatibilityRoot, sourcePath)
	if err != nil {
		return "", "", false
	}
	zipPath = normalizeZipPath(zipPath)
	if zipPath == "." || strings.HasPrefix(zipPath, "../") {
		return "", "", false
	}
	return sourcePath, zipPath, true
}

// rewriteExtAssetZipPath rewrites extasset paths for the zip.
func rewriteExtAssetZipPath(relPath, extAssetDir string) string {
	if extAssetDir == "" {
		return ""
	}

	segments := strings.Split(cleanFilesystemPath(relPath), "/")
	leadingParents := 0
	for i, segment := range segments {
		if segment == "" {
			continue
		}
		if segment == ".." {
			leadingParents++
			continue
		}
		if segment != extAssetDir || leadingParents == 0 {
			return ""
		}

		suffix := filepath.Join(segments[i+1:]...)
		return normalizeZipPath(filepath.Join(engineExtAssetDir, suffix))
	}

	return ""
}

func readExtAssetDir(baseFolder string) (string, error) {
	configPath := filepath.Join(baseFolder, projectConfigName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	var conf assetProjectConfig
	if err := readJSONFile(configPath, &conf); err != nil {
		return "", fmt.Errorf("parse %s: %w", configPath, err)
	}
	return conf.ExtAsset, nil
}

func readJSONFile(filePath string, v any) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func relConfigDir(assetRoot, configDir string) (string, error) {
	rel, err := filepath.Rel(assetRoot, configDir)
	if err != nil {
		return "", err
	}
	return normalizeZipPath(rel), nil
}

// normalizeConfigPath matches runtime path rules.
func normalizeConfigPath(configDir, relPath string) string {
	if relPath == "" {
		return ""
	}
	if strings.HasPrefix(relPath, "/") {
		return relPath
	}
	if schema, _ := spxfs.SplitSchema(relPath); schema != "" {
		return relPath
	}
	return path.Clean(path.Join(configDir, relPath))
}

func cleanFilesystemPath(path string) string {
	return normalizeZipPath(filepath.Clean(path))
}

func sharedAssetCompatibilityRoot(assetRoot string) string {
	return cleanFilesystemPath(filepath.Join(assetRoot, "..", ".."))
}

func isWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = normalizeZipPath(rel)
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

func leadingParentCount(relPath string) int {
	segments := strings.Split(cleanFilesystemPath(relPath), "/")
	count := 0
	for _, segment := range segments {
		if segment != ".." {
			break
		}
		count++
	}
	return count
}
