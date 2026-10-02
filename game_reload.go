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

package spx

import (
	"fmt"
	"reflect"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	tm "github.com/goplus/spx/v3/internal/tilemap"
)

type reloadPlan struct {
	project         coreproject.ProjectConfig
	preparedSprites map[string]preparedSprite
	configNames     []string
	directSprites   map[string]reflect.Type
	prototypeByName map[string]reflect.Type
	stage           []preparedStageEntry
	tilemap         tm.LoadResult
}

func (p *reloadPlan) requireSpriteConfig(name string) {
	if _, ok := p.preparedSprites[name]; ok {
		return
	}
	p.preparedSprites[name] = preparedSprite{}
	p.configNames = append(p.configNames, name)
}

func (p *reloadPlan) validateZOrder(g *Game, shadow reflect.Value) error {
	p.stage = make([]preparedStageEntry, len(p.project.Zorder))
	for layer, raw := range p.project.Zorder {
		entry, err := prepareStageEntry(shadow, raw)
		if err != nil {
			return fmt.Errorf("zorder[%d]: %w", layer, err)
		}
		p.stage[layer] = entry
		switch entry.kind {
		case stageNamedSprite:
			if _, ok := p.directSprites[entry.name]; ok {
				continue
			}
			typ, ok := g.typs[entry.name]
			if !ok {
				return fmt.Errorf("zorder[%d]: sprite %q is not defined", layer, entry.name)
			}
			if err := p.addPrototype(entry.name, typ, shadow, layer); err != nil {
				return err
			}
		case stageSprite:
			if _, ok := p.directSprites[entry.name]; !ok {
				return fmt.Errorf("zorder[%d]: stage sprite target %q is not reloadable", layer, entry.name)
			}
		case stageSprites:
			if len(entry.properties) == 0 {
				continue
			}
			name := entry.spriteType.Name()
			if err := p.addPrototype(name, entry.spriteType, shadow, layer); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *reloadPlan) validateCostumeOverrides() error {
	for _, entry := range p.stage {
		for i, properties := range entry.properties {
			if !properties.has(spritePropertyCostumeIndex) {
				continue
			}
			name := entry.name
			location := fmt.Sprintf("stage sprite target %q", name)
			if entry.kind == stageSprites {
				name = entry.spriteType.Name()
				location = fmt.Sprintf("stage sprites target %q item[%d]", entry.name, i)
			}
			prepared, ok := p.preparedSprites[name]
			if !ok {
				return fmt.Errorf("%s has no sprite configuration", location)
			}
			count := len(prepared.layout.Frames)
			if properties.costumeIndex < 0 || properties.costumeIndex >= count {
				return fmt.Errorf("%s costumeIndex %d is outside %d costumes", location, properties.costumeIndex, count)
			}
		}
	}
	return nil
}

func (p *reloadPlan) addPrototype(name string, typ reflect.Type, shadow reflect.Value, layer int) error {
	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("zorder[%d]: sprite %q has invalid type %s", layer, name, typ)
	}
	sprite, ok := reflect.New(typ).Interface().(Sprite)
	if !ok {
		return fmt.Errorf("zorder[%d]: %q does not implement Sprite", layer, name)
	}
	if err := validateReloadSprite(sprite, shadow); err != nil {
		return fmt.Errorf("zorder[%d]: sprite %q: %w", layer, name, err)
	}

	spriteType := reflect.TypeOf(sprite)
	if directType, ok := p.directSprites[name]; ok {
		if directType != spriteType {
			return fmt.Errorf("zorder[%d]: sprite %q type %s conflicts with field type %s", layer, name, spriteType, directType)
		}
		return nil
	}
	if previous, ok := p.prototypeByName[name]; ok {
		if previous != spriteType {
			return fmt.Errorf("zorder[%d]: sprite %q has conflicting types %s and %s", layer, name, previous, spriteType)
		}
		return nil
	}

	p.prototypeByName[name] = spriteType
	p.requireSpriteConfig(name)
	return nil
}

func (p *reloadPlan) loadSprites(g *Game, gamer reflect.Value) error {
	loadSprite := p.spriteLoader(g)
	return coreproject.WalkFields(gamer, func(fieldIndex int) (string, any) {
		return getFieldPtrOrAlloc(g, gamer, fieldIndex)
	}, func(name string, val any) error {
		sprite, ok := val.(Sprite)
		if !ok {
			return nil
		}
		return loadSprite(sprite, name, gamer)
	})
}

func (p *reloadPlan) spriteLoader(g *Game) spriteLoader {
	return func(sprite Sprite, name string, gamer reflect.Value) error {
		prepared, ok := p.preparedSprites[name]
		if !ok {
			return fmt.Errorf("reload plan has no sprite config for %q", name)
		}
		return g.loadPreparedSprite(sprite, name, gamer, &prepared)
	}
}

func prepareReload(g *Game, gamer reflect.Value, index any) (*reloadPlan, error) {
	if g.fs == nil {
		return nil, fmt.Errorf("reload preflight: game resource directory is not initialized")
	}

	plan := &reloadPlan{
		preparedSprites: make(map[string]preparedSprite),
		directSprites:   make(map[string]reflect.Type),
		prototypeByName: make(map[string]reflect.Type),
	}
	if err := coreproject.LoadConfig(&plan.project, g.fs, index); err != nil {
		return nil, fmt.Errorf("reload preflight: load project config: %w", err)
	}
	if err := validateProjectConfig(&plan.project); err != nil {
		return nil, fmt.Errorf("reload preflight: project config: %w", err)
	}
	loadedTilemap, err := tm.Load(g.fs, plan.project.TilemapPath)
	if err != nil {
		return nil, fmt.Errorf("reload preflight: load tilemap %q: %w", plan.project.TilemapPath, err)
	}
	plan.tilemap = loadedTilemap

	// Mirror the field layout without changing the live game.
	shadow := reflect.New(gamer.Type()).Elem()
	err = coreproject.WalkFields(shadow, func(fieldIndex int) (string, any) {
		return getFieldPtrOrAlloc(g, shadow, fieldIndex)
	}, func(name string, val any) error {
		sprite, ok := val.(Sprite)
		if !ok {
			return nil
		}
		if err := validateReloadSprite(sprite, shadow); err != nil {
			return fmt.Errorf("sprite field %q: %w", name, err)
		}
		plan.directSprites[name] = reflect.TypeOf(sprite)
		plan.requireSpriteConfig(name)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reload preflight: %w", err)
	}
	if err := plan.validateZOrder(g, shadow); err != nil {
		return nil, fmt.Errorf("reload preflight: %w", err)
	}

	for _, name := range plan.configNames {
		config, err := coreproject.LoadSpriteConfig(g.fs, name)
		if err != nil {
			return nil, fmt.Errorf("reload preflight: load sprite config %q: %w", name, err)
		}
		prepared, err := prepareSpriteConfig(&config)
		if err != nil {
			return nil, fmt.Errorf("reload preflight: sprite config %q: %w", name, err)
		}
		plan.preparedSprites[name] = prepared
	}
	if err := plan.validateCostumeOverrides(); err != nil {
		return nil, fmt.Errorf("reload preflight: %w", err)
	}
	return plan, nil
}

func validateReloadSprite(sprite Sprite, gamer reflect.Value) error {
	typ := reflect.TypeOf(sprite)
	if typ == nil || typ.Kind() != reflect.Pointer || typ.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("invalid sprite type %T", sprite)
	}
	v := reflect.ValueOf(sprite).Elem()
	if !isSpriteBaseField(v.Type(), 0) {
		return fmt.Errorf("sprite %s is missing leading SpriteImpl field", typ)
	}
	return bindSpriteOwner(v, gamer)
}
