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
	"path"
	"strings"
	"syscall"

	spxfs "github.com/goplus/spx/v3/fs"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

type OpenedBuilderResources struct {
	AssetDir string
	FS       spxfs.Dir
	LoadedBuilderProject
}

func AssetDirFromResource(resource any) (string, bool) {
	switch v := resource.(type) {
	case string:
		return v, true
	case spxfs.GdDir:
		return strings.TrimSuffix(v.GetPath(), "/"), true
	default:
		return "", false
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

func OpenBuilderResources(resource any, gameConf *Config) (OpenedBuilderResources, error) {
	var opened OpenedBuilderResources
	opened.AssetDir, _ = AssetDirFromResource(resource)

	fs, err := ResourceDir(resource)
	if err != nil {
		return OpenedBuilderResources{}, err
	}
	fs = adaptConfigDir(fs)
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

func LoadJSON(ret any, fs spxfs.Dir, file string) error {
	fs = adaptConfigDir(fs)
	f, err := fs.Open(file)
	if err != nil {
		spxlog.Error("Failed to open file %s: %v", file, err)
		return err
	}
	defer f.Close()
	return decodeJSON(f, ret)
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

func LoadSpriteConfig(fs spxfs.Dir, name string) (SpriteConfig, error) {
	baseDir := path.Join("sprites", name)
	var conf SpriteConfig
	if err := LoadJSON(&conf, fs, baseDir+"/index.json"); err != nil {
		return SpriteConfig{}, err
	}
	normalizeSpriteConfigPaths(&conf, baseDir)
	return conf, nil
}

func LoadSoundConfig(fs spxfs.Dir, name string) (SoundConfig, error) {
	baseDir := path.Join("sounds", name)
	var conf SoundConfig
	if err := LoadJSON(&conf, fs, path.Join(baseDir, "index.json")); err != nil {
		return SoundConfig{}, err
	}
	conf.Path = NormalizeConfigPath(baseDir, conf.Path)
	return conf, nil
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

func gdAssetDir(fs spxfs.Dir) (string, bool) {
	gdDir, ok := fs.(spxfs.GdDir)
	if !ok {
		return "", false
	}
	assetDir := strings.TrimSuffix(gdDir.GetPath(), "/")
	return assetDir, assetDir != ""
}

func shouldReadConfigFromEngine(assetDir string) bool {
	schema, _ := spxfs.SplitSchema(assetDir)
	return schema != ""
}

// NormalizeConfigPath resolves a relative project config path from configDir.
func NormalizeConfigPath(configDir, relPath string) string {
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

func normalizeProjectConfigPaths(conf *ProjectConfig) {
	if conf == nil {
		return
	}

	for _, backdrop := range conf.Backdrops {
		if backdrop == nil {
			continue
		}
		backdrop.Path = NormalizeConfigPath("", backdrop.Path)
	}
	conf.Bgm = NormalizeConfigPath("", conf.Bgm)
	conf.TilemapPath = NormalizeConfigPath("", conf.TilemapPath)
}

func normalizeSpriteConfigPaths(conf *SpriteConfig, configDir string) {
	if conf == nil {
		return
	}

	for _, costume := range conf.Costumes {
		if costume == nil {
			continue
		}
		costume.Path = NormalizeConfigPath(configDir, costume.Path)
	}
	if conf.CostumeSet != nil {
		conf.CostumeSet.Path = NormalizeConfigPath(configDir, conf.CostumeSet.Path)
	}
	if conf.CostumeMPSet != nil {
		conf.CostumeMPSet.Path = NormalizeConfigPath(configDir, conf.CostumeMPSet.Path)
	}
}
