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
	"os"
	"path"
	"sort"
)

type resolver struct {
	root       *os.Root
	packDir    string
	referenced map[string]struct{}
}

func (r *resolver) collectProject(source string, config projectConfig) error {
	for _, backdrop := range config.Backdrops {
		if backdrop != nil {
			if err := r.addReference(source, r.packDir, backdrop.Path); err != nil {
				return err
			}
		}
	}
	for _, reference := range []string{config.Bgm, config.TilemapPath} {
		if err := r.addReference(source, r.packDir, reference); err != nil {
			return err
		}
	}
	return nil
}

func (r *resolver) collectSprites(packed map[string]spriteConfig) error {
	if err := forEachSorted(packed, func(name string, config spriteConfig) error {
		return r.addSpriteReferences(path.Join(r.packDir, packedIndexFile)+"#sprites/"+name, path.Join(r.packDir, "sprites", name), config)
	}); err != nil {
		return err
	}
	return collectSourceSection(r, "sprites", packed, func(source, base string, data []byte) error {
		var config spriteConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("projectassets: decode %q: %w", source, err)
		}
		return r.addSpriteReferences(source, base, config)
	})
}

func (r *resolver) addSpriteReferences(source, base string, config spriteConfig) error {
	for _, costume := range config.Costumes {
		if costume != nil {
			if err := r.addReference(source, base, costume.Path); err != nil {
				return err
			}
		}
	}
	for _, costume := range []*resourceConfig{config.CostumeSet, config.CostumeMPSet} {
		if costume != nil {
			if err := r.addReference(source, base, costume.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *resolver) collectSounds(packed map[string]soundConfig) error {
	if err := forEachSorted(packed, func(name string, config soundConfig) error {
		return r.addReference(path.Join(r.packDir, packedIndexFile)+"#sounds/"+name, path.Join(r.packDir, "sounds", name), config.Path)
	}); err != nil {
		return err
	}
	return collectSourceSection(r, "sounds", packed, func(source, base string, data []byte) error {
		var config soundConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("projectassets: decode %q: %w", source, err)
		}
		return r.addReference(source, base, config.Path)
	})
}

func (r *resolver) collectFonts(packed map[string]fontFamilyConfig, hasPackedCatalog bool) error {
	if hasPackedCatalog {
		return forEachSorted(packed, func(name string, config fontFamilyConfig) error {
			return r.addFontReferences(path.Join(r.packDir, packedIndexFile)+"#fonts/"+name, path.Join(r.packDir, "fonts", name), config)
		})
	}
	return collectSourceSection(r, "fonts", map[string]fontFamilyConfig(nil), func(source, base string, data []byte) error {
		var config fontFamilyConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("projectassets: decode %q: %w", source, err)
		}
		return r.addFontReferences(source, base, config)
	})
}

func (r *resolver) addFontReferences(source, base string, config fontFamilyConfig) error {
	for _, face := range config.Faces {
		if err := r.addReference(source, base, face.Path); err != nil {
			return err
		}
	}
	return nil
}

func collectSourceSection[T any](r *resolver, section string, packed map[string]T, collect func(source, base string, data []byte) error) error {
	directory := path.Join(r.packDir, section)
	entries, ok, err := readPinnedDir(r.root, directory)
	if err != nil || !ok {
		return err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("projectassets: config directory %q is a symlink", path.Join(directory, entry.Name()))
		}
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if err := validateConfigEntryName(section, name); err != nil {
			return err
		}
		if _, overridden := packed[name]; overridden {
			continue
		}
		base := path.Join(directory, name)
		source := path.Join(base, "index.json")
		data, found, err := readPinnedFile(r.root, source)
		if err != nil {
			return err
		}
		if found {
			if err := collect(source, base, data); err != nil {
				return err
			}
		}
	}
	return nil
}

func forEachSorted[T any](values map[string]T, visit func(string, T) error) error {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := visit(name, values[name]); err != nil {
			return err
		}
	}
	return nil
}
