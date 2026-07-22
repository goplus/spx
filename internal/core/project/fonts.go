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

package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	spxfs "github.com/goplus/spx/v2/fs"
	"github.com/goplus/spx/v2/internal/engine"
)

const fontsDir = "fonts"
const defaultFontFamilyName = "default"

type ProjectFontFamily struct {
	Name string
	Path string
}

type ProjectFonts struct {
	Families    []ProjectFontFamily
	Preferences []string
}

type projectFontCatalog interface {
	projectFontFamilyNames() (names []string, available bool)
}

func LoadProjectFonts(fs spxfs.Dir, preferences *string) (ProjectFonts, error) {
	names, catalogAvailable := projectFontFamilyNamesFromCatalog(fs)
	if !catalogAvailable {
		var err error
		names, err = scanProjectFontFamilyNames(fs)
		if err != nil {
			return ProjectFonts{}, err
		}
	}

	families := make([]ProjectFontFamily, 0, len(names))
	seen := make(map[string]string, len(names))
	for _, name := range names {
		folded, err := validateFontFamilyName(name)
		if err != nil {
			return ProjectFonts{}, err
		}
		if previous, ok := seen[folded]; ok {
			return ProjectFonts{}, fmt.Errorf("font family %q conflicts with %q after ASCII case folding", name, previous)
		}
		seen[folded] = name

		baseDir := path.Join(fontsDir, name)
		var config FontFamilyConfig
		if err := LoadJSON(&config, fs, path.Join(baseDir, "index.json")); err != nil {
			return ProjectFonts{}, fmt.Errorf("load font family %q: %w", name, err)
		}
		if len(config.Faces) != 1 {
			return ProjectFonts{}, fmt.Errorf("font family %q must contain exactly one face, got %d", name, len(config.Faces))
		}

		facePath, err := resolveFontFacePath(baseDir, config.Faces[0].Path)
		if err != nil {
			return ProjectFonts{}, fmt.Errorf("font family %q: %w", name, err)
		}
		if err := verifyFontFace(fs, facePath); err != nil {
			return ProjectFonts{}, fmt.Errorf("open font face %q for family %q: %w", facePath, name, err)
		}
		families = append(families, ProjectFontFamily{Name: name, Path: facePath})
	}

	return ProjectFonts{
		Families:    families,
		Preferences: ResolveFontPreferences(preferences, families),
	}, nil
}

func verifyFontFace(fs spxfs.Dir, facePath string) error {
	assetDir, hasAssetDir := gdAssetDir(fs)
	if hasAssetDir && shouldReadConfigFromEngine(assetDir) {
		return verifyFontFaceInEngine(assetDir, facePath)
	}
	file, err := fs.Open(facePath)
	if err == nil {
		return file.Close()
	}
	// The Web interpreter receives source and JSON files only. Font binaries
	// live in Godot's mounted project data, so retry the same logical asset path
	// through the engine after the interpreter filesystem misses it.
	if hasAssetDir {
		if engineErr := verifyFontFaceInEngine(assetDir, facePath); engineErr == nil {
			return nil
		}
	}
	return err
}

func verifyFontFaceInEngine(assetDir, facePath string) error {
	for _, candidate := range configAssetPaths(assetDir, facePath) {
		if engine.HasFile(candidate) {
			return nil
		}
	}
	return os.ErrNotExist
}

func projectFontFamilyNamesFromCatalog(fs spxfs.Dir) ([]string, bool) {
	catalog, ok := fs.(projectFontCatalog)
	if !ok {
		return nil, false
	}
	return catalog.projectFontFamilyNames()
}

func scanProjectFontFamilyNames(fs spxfs.Dir) ([]string, error) {
	if gdDir, ok := fs.(spxfs.GdDir); ok {
		base := strings.TrimSuffix(gdDir.GetPath(), "/")
		if schema, _ := spxfs.SplitSchema(base); schema != "" {
			encoded := engine.ListDirectories(base + "/" + fontsDir)
			var names []string
			if err := json.Unmarshal([]byte(encoded), &names); err != nil {
				return nil, fmt.Errorf("scan %s through engine: %w", fontsDir, err)
			}
			sortFontFamilyNames(names)
			return names, nil
		}
	}
	reader, ok := fs.(spxfs.ReadDirer)
	if !ok {
		return nil, nil
	}
	entries, err := reader.ReadDir(fontsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", fontsDir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir {
			names = append(names, entry.Name)
		}
	}
	sortFontFamilyNames(names)
	return names, nil
}

func sortFontFamilyNames(names []string) {
	sort.Slice(names, func(i, j int) bool {
		left, right := asciiFold(names[i]), asciiFold(names[j])
		if left == right {
			return names[i] < names[j]
		}
		return left < right
	})
}

func validateFontFamilyName(name string) (string, error) {
	if name == "" || strings.TrimSpace(name) != name {
		return "", fmt.Errorf("font family name %q must be non-empty and have no leading or trailing whitespace", name)
	}
	if strings.ContainsAny(name, ",\"'\\/") {
		return "", fmt.Errorf("font family name %q contains a forbidden character", name)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("font family name %q contains a control character", name)
		}
	}
	folded := asciiFold(name)
	if folded == defaultFontFamilyName {
		return "", fmt.Errorf("font family name %q is reserved", name)
	}
	return folded, nil
}

func resolveFontFacePath(baseDir, relativePath string) (string, error) {
	if relativePath == "" {
		return "", errors.New("font face path must not be empty")
	}
	if strings.Contains(relativePath, "\\") || strings.HasPrefix(relativePath, "/") {
		return "", fmt.Errorf("font face path %q must be a relative POSIX path", relativePath)
	}
	cleaned := path.Clean(relativePath)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("font face path %q escapes its family directory", relativePath)
	}
	resolved := path.Join(baseDir, cleaned)
	if resolved == baseDir || !strings.HasPrefix(resolved, baseDir+"/") {
		return "", fmt.Errorf("font face path %q escapes its family directory", relativePath)
	}
	return resolved, nil
}

func ParseFontPreferences(value string) []string {
	parts := strings.Split(value, ",")
	for i, part := range parts {
		name := strings.TrimSpace(part)
		if len(name) >= 2 && ((name[0] == '"' && name[len(name)-1] == '"') || (name[0] == '\'' && name[len(name)-1] == '\'')) {
			name = name[1 : len(name)-1]
		}
		parts[i] = name
	}
	return parts
}

func ResolveFontPreferences(value *string, families []ProjectFontFamily) []string {
	if value == nil {
		return []string{defaultFontFamilyName}
	}
	available := make(map[string]string, len(families)+1)
	available[defaultFontFamilyName] = defaultFontFamilyName
	for _, family := range families {
		available[asciiFold(family.Name)] = family.Name
	}
	parsed := ParseFontPreferences(*value)
	resolved := make([]string, 0, len(parsed))
	seen := make(map[string]struct{}, len(parsed))
	for _, name := range parsed {
		folded := asciiFold(name)
		canonical, ok := available[folded]
		if !ok {
			continue
		}
		if _, ok := seen[folded]; ok {
			continue
		}
		seen[folded] = struct{}{}
		resolved = append(resolved, canonical)
	}
	return resolved
}

func asciiFold(value string) string {
	bytes := []byte(value)
	for i, b := range bytes {
		if b >= 'A' && b <= 'Z' {
			bytes[i] = b + ('a' - 'A')
		}
	}
	return string(bytes)
}
