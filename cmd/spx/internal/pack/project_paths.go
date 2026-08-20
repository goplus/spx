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

	spxfs "github.com/goplus/spx/v3/fs"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

const (
	projectConfigName = ".config"
	packDirName       = "assets"
	sourceIndexName   = "index.json"
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
	Project  coreproject.ProjectConfig
	Sprites  map[string]coreproject.SpriteConfig
	Sounds   map[string]coreproject.SoundConfig
	Fonts    map[string]coreproject.FontFamilyConfig
	HasFonts bool
}

// validateLegacyPackInputs keeps the pack command's legacy input contract
// independent from the portable runtime-provider policy. In particular,
// .config may contain extasset here; the provider's portable policy is not
// applicable to an archive that deliberately preserves external resources.
func validateLegacyPackInputs(baseFolder string) error {
	rootInfo, err := os.Lstat(baseFolder)
	if err != nil {
		return fmt.Errorf("pack: inspect project directory %q: %w", baseFolder, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fmt.Errorf("pack: project directory %q must be a real directory", baseFolder)
	}

	if err := validateLegacyProjectConfig(baseFolder); err != nil {
		return err
	}

	assetRoot := filepath.Join(baseFolder, packDirName)
	assetInfo, err := os.Lstat(assetRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("projectassets: PackDir %q is missing", packDirName)
		}
		return fmt.Errorf("projectassets: inspect PackDir %q: %w", packDirName, err)
	}
	if assetInfo.Mode()&os.ModeSymlink != 0 || !assetInfo.IsDir() {
		return fmt.Errorf("projectassets: PackDir %q must be a real directory", packDirName)
	}

	hasIndex := false
	for _, name := range []string{sourceIndexName, packedIndexName} {
		indexPath := filepath.Join(assetRoot, name)
		info, statErr := os.Lstat(indexPath)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			return fmt.Errorf("projectassets: inspect %q: %w", indexPath, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("projectassets: asset index %q must be a regular non-symlink file", indexPath)
		}
		hasIndex = true
	}
	if !hasIndex {
		return fmt.Errorf("projectassets: PackDir %q contains neither %q nor %q", packDirName, sourceIndexName, packedIndexName)
	}

	// Parse the effective packed/source indexes using the legacy precedence,
	// while leaving reference resolution to the collector below. The collector
	// intentionally permits bounded external references.
	if _, err := collectAssetPathRefs(assetRoot); err != nil {
		return fmt.Errorf("projectassets: validate asset indexes: %w", err)
	}
	return nil
}

func validateLegacyProjectConfig(baseFolder string) error {
	configPath := filepath.Join(baseFolder, projectConfigName)
	info, err := os.Lstat(configPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("projectpolicy: inspect project config %q: %w", configPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("projectpolicy: project config %q must be a regular non-symlink file", configPath)
	}
	if _, err := readExtAssetDir(baseFolder); err != nil {
		return fmt.Errorf("projectpolicy: parse project config %q: %w", configPath, err)
	}
	return nil
}

// collectExternalAssetPaths matches runtime asset lookup.
func collectExternalAssetPaths(baseFolder string, existingZipPaths map[string]struct{}) (extraPaths []DirInfos, err error) {
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

	var externalRoot *os.Root
	defer func() {
		if err != nil && externalRoot != nil {
			_ = externalRoot.Close()
		}
	}()
	for _, ref := range refs {
		normalized := normalizeConfigPath(ref.configDir, ref.path)
		sourcePath, zipPath, ok := resolveExternalAssetPath(assetRoot, compatibilityRoot, extAssetDir, normalized)
		if !ok {
			continue
		}
		if _, exists := seen[zipPath]; exists {
			continue
		}

		if externalRoot == nil {
			externalRoot, err = openPackRoot(compatibilityRoot)
			if err != nil {
				return nil, err
			}
		}
		info, rootPath, err := inspectExternalAssetPath(externalRoot, compatibilityRoot, sourcePath)
		if err != nil {
			return nil, fmt.Errorf("stat external asset %s referenced by %q: %w", sourcePath, normalized, err)
		}

		seen[zipPath] = struct{}{}
		extraPaths = append(extraPaths, DirInfos{path: sourcePath, info: info, zipPath: zipPath, root: externalRoot, rootPath: rootPath})
	}

	return extraPaths, nil
}

func inspectExternalAssetPath(root *os.Root, rootPath, name string) (os.FileInfo, string, error) {
	rel, err := filepath.Rel(rootPath, name)
	if err != nil {
		return nil, "", err
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("path is outside the compatibility root")
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for i := range parts {
		candidate := filepath.Join(parts[:i+1]...)
		info, err := root.Lstat(candidate)
		if err != nil {
			return nil, "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, "", fmt.Errorf("must be a regular non-symlink path (symlink at %q)", candidate)
		}
		if i < len(parts)-1 {
			if !info.IsDir() {
				return nil, "", fmt.Errorf("path component %q is not a directory", candidate)
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return nil, "", fmt.Errorf("must be a regular non-symlink file")
		}
		return info, rel, nil
	}
	return nil, "", fmt.Errorf("empty external asset path")
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
		projectConfigPath := filepath.Join(assetRoot, "index.json")
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

	for name, conf := range packed {
		refs = appendRefs(refs, path.Join(category, name), conf)
	}

	if !scanSource {
		return refs, nil
	}

	configPaths, err := filepath.Glob(filepath.Join(assetRoot, category, "*", "index.json"))
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
	projectConfigPath := filepath.Join(assetRoot, "index.json")
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

func mergePackedRootSections(packedRoot map[string]json.RawMessage, sourceRoot map[string]json.RawMessage) map[string]json.RawMessage {
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
