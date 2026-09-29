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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	spxfs "github.com/goplus/spx/v3/fs"
	"github.com/goplus/spx/v3/internal/assetindex"
	"github.com/goplus/spx/v3/internal/engine"
)

const packedIndexJSON = "index_pack.json"

type packedConfigDir struct {
	spxfs.Dir
	index packedConfigIndex
}

type adaptedConfigDir interface {
	isAdaptedConfigDir()
}

// rawConfigDir keeps engine-specific lookup below packedConfigDir so all
// decoded configuration still passes through the installed overlay.
type rawConfigDir struct {
	spxfs.Dir
	assetDir string
}

type rawReadDirConfigDir struct {
	*rawConfigDir
	spxfs.ReadDirer
}

type packedConfigIndex struct {
	raw        []byte
	projectRaw []byte
	sprites    map[string]json.RawMessage
	sounds     map[string]json.RawMessage
	fonts      map[string]json.RawMessage
	hasFonts   bool
}

func (p *packedConfigDir) Open(name string) (io.ReadCloser, error) {
	switch normalizePackedConfigPath(name) {
	case "index.json":
		return io.NopCloser(bytes.NewReader(p.index.projectRaw)), nil
	case packedIndexJSON:
		return io.NopCloser(bytes.NewReader(p.index.raw)), nil
	}

	if raw, ok := p.lookupPackedChild(name); ok {
		return io.NopCloser(bytes.NewReader(raw)), nil
	}
	return openConfigReader(p.Dir, name)
}

func (*packedConfigDir) isAdaptedConfigDir() {}

func (p *packedConfigDir) GetPath() string {
	if gdDir, ok := p.Dir.(spxfs.GdDir); ok {
		return gdDir.GetPath()
	}
	return ""
}

func (p *packedConfigDir) ReadDir(name string) ([]spxfs.DirEntry, error) {
	reader, ok := p.Dir.(spxfs.ReadDirer)
	if !ok {
		return nil, nil
	}
	return reader.ReadDir(name)
}

func (p *packedConfigDir) lookupPackedChild(name string) (json.RawMessage, bool) {
	normalized := normalizePackedConfigPath(name)
	parts := strings.Split(normalized, "/")
	if len(parts) != 3 || parts[2] != "index.json" {
		return nil, false
	}

	switch parts[0] {
	case "sprites":
		raw, ok := p.index.sprites[parts[1]]
		return raw, ok
	case "sounds":
		raw, ok := p.index.sounds[parts[1]]
		return raw, ok
	case "fonts":
		raw, ok := p.index.fonts[parts[1]]
		return raw, ok
	default:
		return nil, false
	}
}

func (p *packedConfigDir) projectFontFamilyNames() ([]string, bool) {
	if !p.index.hasFonts {
		return nil, false
	}
	names := make([]string, 0, len(p.index.fonts))
	for name := range p.index.fonts {
		names = append(names, name)
	}
	sortFontFamilyNames(names)
	return names, true
}

func adaptConfigDir(fs spxfs.Dir) spxfs.Dir {
	if _, ok := fs.(adaptedConfigDir); ok {
		return fs
	}
	assetDir, ok := gdAssetDir(fs)
	if !ok || !shouldReadConfigFromEngine(assetDir) {
		return fs
	}
	raw := &rawConfigDir{Dir: fs, assetDir: assetDir}
	if reader, ok := fs.(spxfs.ReadDirer); ok {
		return &rawReadDirConfigDir{rawConfigDir: raw, ReadDirer: reader}
	}
	return raw
}

func (p *rawConfigDir) Open(name string) (io.ReadCloser, error) {
	for _, filePath := range configAssetPaths(p.assetDir, name) {
		if engine.HasFile(filePath) {
			return io.NopCloser(strings.NewReader(engine.ReadAllText(filePath))), nil
		}
	}
	return p.Dir.Open(name)
}

func (*rawConfigDir) isAdaptedConfigDir() {}

func (p *rawConfigDir) GetPath() string {
	return p.assetDir
}

func loadPackedConfigIndex(fs spxfs.Dir) (packedConfigIndex, bool, error) {
	data, ok, err := readConfigBytes(fs, packedIndexJSON)
	if err != nil {
		return packedConfigIndex{}, false, err
	}
	if !ok {
		return packedConfigIndex{}, false, nil
	}

	sourceData, _, err := readConfigBytes(fs, "index.json")
	if err != nil {
		return packedConfigIndex{}, false, err
	}

	index, err := parsePackedConfigIndex(data, sourceData)
	if err != nil {
		return packedConfigIndex{}, false, fmt.Errorf("parse %s: %w", packedIndexJSON, err)
	}
	return index, true, nil
}

func parsePackedConfigIndex(data []byte, sourceData []byte) (packedConfigIndex, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return packedConfigIndex{}, err
	}

	projectRaw, err := mergePackedProjectRoot(data, root, sourceData)
	if err != nil {
		return packedConfigIndex{}, err
	}

	sprites, err := parsePackedSection(root, "sprites")
	if err != nil {
		return packedConfigIndex{}, err
	}
	sounds, err := parsePackedSection(root, "sounds")
	if err != nil {
		return packedConfigIndex{}, err
	}
	fonts, err := parsePackedSection(root, "fonts")
	if err != nil {
		return packedConfigIndex{}, err
	}
	_, hasFonts := root["fonts"]

	return packedConfigIndex{
		raw:        data,
		projectRaw: projectRaw,
		sprites:    sprites,
		sounds:     sounds,
		fonts:      fonts,
		hasFonts:   hasFonts,
	}, nil
}

func mergePackedProjectRoot(data []byte, packedRoot map[string]json.RawMessage, sourceData []byte) ([]byte, error) {
	if len(sourceData) == 0 {
		return data, nil
	}

	var sourceRoot map[string]json.RawMessage
	if err := json.Unmarshal(sourceData, &sourceRoot); err != nil {
		return nil, fmt.Errorf("parse index.json: %w", err)
	}
	if len(sourceRoot) == 0 {
		return data, nil
	}

	return json.Marshal(assetindex.Merge(sourceRoot, packedRoot))
}

func parsePackedSection(root map[string]json.RawMessage, key string) (map[string]json.RawMessage, error) {
	section, err := assetindex.ParseEntries(root[key])
	if err != nil {
		return nil, fmt.Errorf("%s must be an object: %w", key, err)
	}
	return section, nil
}

func normalizePackedConfigPath(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "/")
	return path.Clean(name)
}

func openConfigReader(fs spxfs.Dir, file string) (io.ReadCloser, error) {
	if data, ok, err := readConfigBytes(fs, file); err != nil {
		return nil, err
	} else if ok {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, fmt.Errorf("load json failed: %s does not exist", file)
}

func readConfigBytes(fs spxfs.Dir, file string) ([]byte, bool, error) {
	f, err := fs.Open(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("open %s: %w", file, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", file, err)
	}
	return data, true, nil
}

func configAssetPaths(assetDir, file string) []string {
	normalizedFile := normalizePackedConfigPath(file)
	paths := []string{
		joinAssetConfigPath(assetDir, normalizedFile),
		joinAssetConfigPath(resourceAssetDir(assetDir), normalizedFile),
		engine.ToAssetPath(normalizedFile),
	}

	ret := paths[:0]
	for _, candidate := range paths {
		if candidate != "" && !slices.Contains(ret, candidate) {
			ret = append(ret, candidate)
		}
	}
	return ret
}

func joinAssetConfigPath(assetDir, file string) string {
	assetDir = strings.ReplaceAll(assetDir, "\\", "/")
	assetDir = strings.TrimSuffix(assetDir, "/")
	if assetDir == "" {
		return file
	}
	return assetDir + "/" + file
}

func resourceAssetDir(assetDir string) string {
	assetDir = strings.ReplaceAll(assetDir, "\\", "/")
	assetDir = strings.TrimSuffix(assetDir, "/")
	if assetDir == "" {
		return ""
	}
	if schema, _ := spxfs.SplitSchema(assetDir); schema != "" {
		return assetDir
	}
	return "res://" + strings.TrimPrefix(assetDir, "/")
}
