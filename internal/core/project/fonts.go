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
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	spxfs "github.com/goplus/spx/v4/fs"
)

const fontsDir = "fonts"

const defaultFontFamilyName = "default"

type ProjectFontFamily struct {
	Name  string
	Faces []ProjectFontFace
}

// ProjectFontFace stores a resolved face; each family currently permits one regular face.
type ProjectFontFace struct {
	Path string
}

type ProjectFonts struct {
	Families    []ProjectFontFamily
	Preferences []string
}

type FontFaceConfig struct {
	Path string `json:"path"`
}

type FontFamilyConfig struct {
	Faces []FontFaceConfig `json:"faces"`
}

func LoadProjectFonts(fs spxfs.Dir, preferences []string) (ProjectFonts, error) {
	families, err := loadProjectFontFamilies(fs)
	if err != nil {
		return ProjectFonts{}, err
	}
	resolvedPreferences, err := ResolveFontPreferences(preferences, families)
	if err != nil {
		return ProjectFonts{}, err
	}
	return ProjectFonts{Families: families, Preferences: resolvedPreferences}, nil
}

func ResolveFontPreferences(value []string, families []ProjectFontFamily) ([]string, error) {
	if value == nil {
		return []string{defaultFontFamilyName}, nil
	}
	available := make(map[string]string, len(families)+1)
	available[defaultFontFamilyName] = defaultFontFamilyName
	for _, family := range families {
		available[asciiFold(family.Name)] = family.Name
	}
	resolved := make([]string, 0, len(value))
	seen := make(map[string]struct{}, len(value))
	for _, name := range value {
		if name == "" {
			return nil, errors.New("font preference must be non-empty")
		}
		folded := asciiFold(name)
		canonical, ok := available[folded]
		if !ok {
			return nil, fmt.Errorf("font preference %q is not an available font family", name)
		}
		if _, ok := seen[folded]; ok {
			return nil, fmt.Errorf("font preference %q is duplicated after ASCII case folding", name)
		}
		seen[folded] = struct{}{}
		resolved = append(resolved, canonical)
	}
	return resolved, nil
}

func loadProjectFontFamilies(fs spxfs.Dir) ([]ProjectFontFamily, error) {
	names, err := projectFontFamilyNames(fs)
	if err != nil {
		return nil, err
	}

	families := make([]ProjectFontFamily, 0, len(names))
	var previousFolded string
	for i, name := range names {
		folded, err := validateFontFamilyName(name)
		if err != nil {
			return nil, err
		}
		// Discovery sorts by ASCII fold, so conflicting families are adjacent.
		if i > 0 && folded == previousFolded {
			return nil, fmt.Errorf("font family %q conflicts with %q after ASCII case folding", name, names[i-1])
		}
		previousFolded = folded

		family, err := loadProjectFontFamily(fs, name)
		if err != nil {
			return nil, err
		}
		families = append(families, family)
	}
	return families, nil
}

func loadProjectFontFamily(fs spxfs.Dir, name string) (ProjectFontFamily, error) {
	baseDir := path.Join(fontsDir, name)
	var config FontFamilyConfig
	if err := LoadJSON(&config, fs, path.Join(baseDir, "index.json")); err != nil {
		return ProjectFontFamily{}, fmt.Errorf("load font family %q: %w", name, err)
	}
	if len(config.Faces) != 1 {
		return ProjectFontFamily{}, fmt.Errorf("font family %q must contain exactly one face, got %d", name, len(config.Faces))
	}

	facePath, err := resolveFontFacePath(baseDir, config.Faces[0].Path)
	if err != nil {
		return ProjectFontFamily{}, fmt.Errorf("font family %q: %w", name, err)
	}
	if err := spxfs.VerifyFile(fs, facePath); err != nil {
		return ProjectFontFamily{}, fmt.Errorf("open font face %q for family %q: %w", facePath, name, err)
	}
	return ProjectFontFamily{Name: name, Faces: []ProjectFontFace{{Path: facePath}}}, nil
}

func projectFontFamilyNames(fs spxfs.Dir) ([]string, error) {
	names, err := spxfs.ListDirectories(fs, fontsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", fontsDir, err)
	}
	sort.Slice(names, func(i, j int) bool {
		left, right := asciiFold(names[i]), asciiFold(names[j])
		if left == right {
			return names[i] < names[j]
		}
		return left < right
	})
	return names, nil
}

func validateFontFamilyName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("font family name must be non-empty")
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

func asciiFold(value string) string {
	bytes := []byte(value)
	for i, b := range bytes {
		if b >= 'A' && b <= 'Z' {
			bytes[i] = b + ('a' - 'A')
		}
	}
	return string(bytes)
}
