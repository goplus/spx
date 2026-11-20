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

package assets

import coreproject "github.com/goplus/spx/v4/internal/core/project"

type resourcePaths interface {
	ResourcePaths(func(*string) bool)
}

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

func (c *resourceConfig) ResourcePaths(yield func(*string) bool) {
	yield(&c.Path)
}

func (c *projectConfig) ResourcePaths(yield func(*string) bool) {
	for _, backdrop := range c.Backdrops {
		if backdrop != nil && !yield(&backdrop.Path) {
			return
		}
	}
	yield(&c.TilemapPath)
}

func (c *spriteConfig) ResourcePaths(yield func(*string) bool) {
	for _, costume := range c.Costumes {
		if costume != nil && !yield(&costume.Path) {
			return
		}
	}
	if c.CostumeSet != nil && !yield(&c.CostumeSet.Path) {
		return
	}
	if c.CostumeMPSet != nil {
		yield(&c.CostumeMPSet.Path)
	}
}

// The launcher inspects only fields that reference resources. Legacy packing
// also checks the types of all other fields in the project format.
func (v *indexValidator) childConfig(section string) resourcePaths {
	switch section {
	case "sprites":
		if v.legacy {
			return new(coreproject.SpriteConfig)
		}
		return new(spriteConfig)
	case "sounds":
		if v.legacy {
			return new(coreproject.SoundConfig)
		}
		return new(resourceConfig)
	default: // fonts
		return new(coreproject.FontFamilyConfig)
	}
}
