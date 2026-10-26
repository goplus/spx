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
	"strings"

	spxfs "github.com/goplus/spx/v4/fs"
	"github.com/goplus/spx/v4/internal/assetindex"
)

const packedIndexJSON = "index_pack.json"

type packedConfigDir struct {
	spxfs.Dir
	index packedConfigIndex
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
	if raw, ok := p.lookupPackedConfig(name); ok {
		return io.NopCloser(bytes.NewReader(raw)), nil
	}
	file, err := p.Dir.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	return file, nil
}

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

func (p *packedConfigDir) VerifyFile(name string) error {
	if _, ok := p.lookupPackedConfig(name); ok {
		return nil
	}
	return spxfs.VerifyFile(p.Dir, name)
}

func (p *packedConfigDir) ListDirectories(name string) ([]string, error) {
	if name == fontsDir && p.index.hasFonts {
		names := make([]string, 0, len(p.index.fonts))
		for name := range p.index.fonts {
			names = append(names, name)
		}
		return names, nil
	}
	return spxfs.ListDirectories(p.Dir, name)
}

func (p *packedConfigDir) lookupPackedConfig(name string) ([]byte, bool) {
	normalized := normalizePackedConfigPath(name)
	switch normalized {
	case "index.json":
		return p.index.projectRaw, true
	case packedIndexJSON:
		return p.index.raw, true
	}
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
