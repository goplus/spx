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
	"fmt"
	"io"
	iofs "io/fs"
	"iter"
	"path"
	"runtime"
	"strings"
	"syscall"

	spxfs "github.com/goplus/spx/v4/fs"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
)

type OpenedBuilderResources struct {
	AssetDir string
	FS       spxfs.Dir
	LoadedBuilderProject
}

func OpenBuilderResources(resource any, gameConf *Config) (OpenedBuilderResources, error) {
	var opened OpenedBuilderResources
	opened.AssetDir = AssetDirFromResource(resource)

	fs, err := ResourceDir(resource)
	if err != nil {
		return OpenedBuilderResources{}, err
	}
	index, packed, err := loadPackedConfigIndex(fs)
	if err != nil {
		fs.Close()
		return OpenedBuilderResources{}, err
	}
	if packed {
		fs = &packedConfigDir{Dir: fs, index: index}
	}
	opened.FS = fs

	loaded, err := LoadBuilderProject(fs, gameConf)
	if err != nil {
		fs.Close()
		return OpenedBuilderResources{}, err
	}
	opened.LoadedBuilderProject = loaded
	return opened, nil
}

func AssetDirFromResource(resource any) string {
	switch v := resource.(type) {
	case string:
		return v
	case spxfs.GdDir:
		return strings.TrimSuffix(v.GetPath(), "/")
	default:
		return ""
	}
}

func ResourceDir(resource any) (spxfs.Dir, error) {
	if fs, ok := resource.(spxfs.Dir); ok {
		return fs, nil
	}
	path, ok := resource.(string)
	if !ok {
		return nil, fmt.Errorf("unsupported resource type %T", resource)
	}
	return spxfs.Open(path)
}

func LoadConfig(ret any, fs spxfs.Dir, index any) error {
	switch v := index.(type) {
	case io.Reader:
		return decodeJSON(v, ret)
	case string:
		return LoadJSON(ret, fs, v)
	case nil:
		return LoadJSON(ret, fs, "index.json")
	default:
		return syscall.EINVAL
	}
}

// ValidResourceName reports whether name is a single POSIX resource directory name.
func ValidResourceName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\x00")
}

// ValidateBackgroundSoundName checks the optional sound name selected by a project.
func ValidateBackgroundSoundName(name string) error {
	if name != "" && !ValidResourceName(name) {
		return fmt.Errorf("background sound name %q is not a valid path segment", name)
	}
	return nil
}

func LoadSpriteConfig(fs spxfs.Dir, name string) (SpriteConfig, error) {
	if !ValidResourceName(name) {
		return SpriteConfig{}, fmt.Errorf("sprite name %q is not a valid path segment", name)
	}
	baseDir := path.Join("sprites", name)
	var conf SpriteConfig
	if err := LoadJSON(&conf, fs, baseDir+"/index.json"); err != nil {
		return SpriteConfig{}, err
	}
	if err := resolveResourcePaths(fs, baseDir, conf.ResourcePaths); err != nil {
		return SpriteConfig{}, err
	}
	return conf, nil
}

func LoadSoundConfig(fs spxfs.Dir, name string) (SoundConfig, error) {
	if !ValidResourceName(name) {
		return SoundConfig{}, fmt.Errorf("sound name %q is not a valid path segment", name)
	}
	baseDir := path.Join("sounds", name)
	var conf SoundConfig
	if err := LoadJSON(&conf, fs, path.Join(baseDir, "index.json")); err != nil {
		return SoundConfig{}, err
	}
	if err := resolveResourcePaths(fs, baseDir, conf.ResourcePaths); err != nil {
		return SoundConfig{}, err
	}
	return conf, nil
}

func LoadJSON(ret any, fs spxfs.Dir, file string) error {
	file, err := cleanAssetPath(file)
	if err != nil {
		return err
	}
	f, err := fs.Open(file)
	if err != nil {
		spxlog.Error("Failed to open file %s: %v", file, err)
		return err
	}
	defer f.Close()
	return decodeJSON(f, ret)
}

// ResolveResourcePath resolves a configuration reference within the asset directory.
// Engine-owned resource URIs are not part of the project configuration format.
func ResolveResourcePath(configDir, reference string, literalResPrefix bool) (string, error) {
	if reference == "" {
		return "", nil
	}
	if strings.ContainsAny(reference, "\\\x00") || spxfs.IsAbsResourcePath(reference) || strings.Contains(reference, "://") || (runtime.GOOS == "windows" && strings.ContainsRune(reference, ':')) {
		return "", fmt.Errorf("resource path %q must be relative to the asset directory", reference)
	}
	if prefix, _, found := strings.Cut(reference, ":"); found {
		if len(prefix) == 1 && ((prefix[0] >= 'a' && prefix[0] <= 'z') || (prefix[0] >= 'A' && prefix[0] <= 'Z')) {
			return "", fmt.Errorf("resource path %q must not use a drive prefix", reference)
		}
		scheme, _ := spxfs.SplitSchema(reference)
		switch prefix {
		case "file", "http", "https":
			scheme = prefix
		case "res":
			if literalResPrefix {
				scheme = ""
			}
		}
		if scheme != "" {
			return "", fmt.Errorf("resource path %q must not use a URI scheme", reference)
		}
	}
	name := path.Join(configDir, reference)
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || spxfs.IsAbsResourcePath(name) {
		return "", fmt.Errorf("resource path %q escapes the asset directory", reference)
	}
	return name, nil
}

// cleanAssetPath preserves POSIX directory names while keeping file lookup
// relative to the supplied resource directory.
func cleanAssetPath(name string) (string, error) {
	clean := path.Clean(name)
	if spxfs.IsAbsResourcePath(name) || spxfs.IsAbsResourcePath(clean) ||
		strings.HasPrefix(name, `\`) || strings.HasPrefix(clean, `\`) ||
		clean == "." || !iofs.ValidPath(clean) || strings.ContainsRune(name, 0) || strings.Contains(name, "://") {
		return "", fmt.Errorf("resource file %q must be relative to the asset directory", name)
	}
	return clean, nil
}

func resolveResourcePaths(fs spxfs.Dir, configDir string, paths iter.Seq[*string]) error {
	literalResPrefix := spxfs.AllowsLiteralResPrefix(fs)
	for value := range paths {
		resolved, err := ResolveResourcePath(configDir, *value, literalResPrefix)
		if err != nil {
			return err
		}
		if err := spxfs.ValidateResourcePath(fs, resolved); err != nil {
			return err
		}
		// Keep root-level POSIX filenames distinct from engine resource URIs.
		if strings.HasPrefix(resolved, "res:") {
			resolved = "./" + resolved
		}
		*value = resolved
	}
	return nil
}

func decodeJSON(r io.Reader, ret any) error {
	dec := json.NewDecoder(r)
	if err := dec.Decode(ret); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected content after JSON value")
		}
		return err
	}
	return nil
}
