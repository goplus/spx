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

package projectassets

import (
	"encoding/json"
	"fmt"
	"sort"
)

type resourceConfig struct {
	Path string `json:"path"`
}

type projectConfig struct {
	Backdrops   []*resourceConfig `json:"backdrops"`
	Bgm         string            `json:"bgm"`
	TilemapPath string            `json:"tilemapPath"`
}

type spriteConfig struct {
	Costumes     []*resourceConfig `json:"costumes"`
	CostumeSet   *resourceConfig   `json:"costumeSet"`
	CostumeMPSet *resourceConfig   `json:"costumeMPSet"`
}

type soundConfig struct {
	Path string `json:"path"`
}

type fontFamilyConfig struct {
	Faces []resourceConfig `json:"faces"`
}

type packedConfig struct {
	project  projectConfig
	sprites  map[string]spriteConfig
	sounds   map[string]soundConfig
	fonts    map[string]fontFamilyConfig
	hasFonts bool
}

func parseConfig(sourceName string, sourceData []byte, packedName string, packedData []byte) (packedConfig, error) {
	sourceRoot, err := parseRoot(sourceName, sourceData)
	if err != nil {
		return packedConfig{}, err
	}
	packedRoot, err := parseRoot(packedName, packedData)
	if err != nil {
		return packedConfig{}, err
	}

	merged := make(map[string]json.RawMessage, len(sourceRoot)+len(packedRoot))
	for key, value := range sourceRoot {
		merged[key] = value
	}
	for key, value := range packedRoot {
		merged[key] = value
	}
	var config packedConfig
	if err := decodeProjectConfig(merged, &config.project); err != nil {
		name := sourceName
		if len(packedData) != 0 {
			name = packedName
		}
		return packedConfig{}, fmt.Errorf("projectassets: decode merged project config at %q: %w", name, err)
	}
	config.sprites, err = decodePackedObjects[spriteConfig](packedName, "sprites", packedRoot["sprites"])
	if err != nil {
		return packedConfig{}, err
	}
	config.sounds, err = decodePackedObjects[soundConfig](packedName, "sounds", packedRoot["sounds"])
	if err != nil {
		return packedConfig{}, err
	}
	_, config.hasFonts = packedRoot["fonts"]
	config.fonts, err = decodePackedObjects[fontFamilyConfig](packedName, "fonts", packedRoot["fonts"])
	if err != nil {
		return packedConfig{}, err
	}
	return config, nil
}

func parseRoot(name string, data []byte) (map[string]json.RawMessage, error) {
	if data == nil {
		return nil, nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("projectassets: decode %q: %w", name, err)
	}
	return root, nil
}

func decodeProjectConfig(root map[string]json.RawMessage, dest *projectConfig) error {
	data, err := json.Marshal(root)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func decodePackedObjects[T any](indexName, section string, raw json.RawMessage) (map[string]T, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("projectassets: decode %q section %q: %w", indexName, section, err)
	}
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	configs := make(map[string]T, len(entries))
	for _, name := range names {
		if err := validateConfigEntryName(section, name); err != nil {
			return nil, err
		}
		var config T
		if err := json.Unmarshal(entries[name], &config); err != nil {
			return nil, fmt.Errorf("projectassets: decode %q section %q entry %q: %w", indexName, section, name, err)
		}
		configs[name] = config
	}
	return configs, nil
}
