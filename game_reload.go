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
	project                  coreproject.ProjectConfig
	preparedSprites          map[string]preparedSprite
	configNames              []string
	directSprites            map[string]reflect.Type
	prototypeByName          map[string]reflect.Type
	costumeOverrides         []reloadCostumeOverride
	preparedSpriteProperties stageSpriteProperties
	tilemap                  tm.LoadResult
}

type preparedSprite struct {
	config coreproject.SpriteConfig
	layout *coreproject.CostumeLayout
}

type reloadCostumeOverride struct {
	sprite   string
	location string
	index    int
}

func (p *reloadPlan) requireSpriteConfig(name string) {
	if _, ok := p.preparedSprites[name]; ok {
		return
	}
	p.preparedSprites[name] = preparedSprite{}
	p.configNames = append(p.configNames, name)
}

func (p *reloadPlan) validateZOrder(g *Game, shadow reflect.Value) error {
	return coreproject.WalkZOrder(
		p.project.Zorder,
		func(layer int, name string) error {
			if _, ok := p.directSprites[name]; ok {
				return nil
			}
			typ, ok := g.typs[name]
			if !ok {
				return fmt.Errorf("zorder[%d]: sprite %q is not defined", layer, name)
			}
			return p.addPrototype(name, typ, shadow, layer)
		},
		func(layer int, shape coreproject.StageShape) error {
			err := coreproject.DispatchStageShape(shape, coreproject.StageShapeHandlers{
				StageMonitor: func(shape coreproject.StageShape) error {
					_, err := coreproject.ParseMonitorShape(shape)
					return err
				},
				Measure: func(shape coreproject.StageShape) error {
					_, err := coreproject.ParseMeasureShape(shape)
					return err
				},
				Sprite: func(shape coreproject.StageShape) error {
					return p.validateStageSprite(shape, shadow, layer)
				},
				Sprites: func(shape coreproject.StageShape) error {
					return p.validateStageSprites(shape, shadow, layer)
				},
			})
			if err != nil {
				return fmt.Errorf("zorder[%d]: %w", layer, err)
			}
			return nil
		},
	)
}

func (p *reloadPlan) validateStageSprite(shape coreproject.StageShape, shadow reflect.Value, layer int) error {
	target, err := stageShapeTarget(shape)
	if err != nil {
		return err
	}
	val := coreproject.FindObjectPtr(shadow, target, 0)
	sprite, ok := val.(Sprite)
	if !ok || sprite == nil {
		return fmt.Errorf("stage sprite target %q is not a sprite field", target)
	}
	if _, ok := p.directSprites[target]; !ok {
		return fmt.Errorf("stage sprite target %q is not reloadable", target)
	}
	properties, err := parseSpriteProperties(shape)
	if err != nil {
		return err
	}
	p.preparedSpriteProperties[layer] = []spriteProperties{properties}
	p.recordCostumeOverride(target, fmt.Sprintf("stage sprite target %q", target), properties)
	return nil
}

func (p *reloadPlan) validateStageSprites(shape coreproject.StageShape, shadow reflect.Value, layer int) error {
	target, err := stageShapeTarget(shape)
	if err != nil {
		return err
	}
	items, err := stageShapeItems(shape)
	if err != nil {
		return err
	}
	val := coreproject.FindFieldPtr(shadow, target, 0)
	if val == nil {
		return fmt.Errorf("stage sprites target %q is not defined", target)
	}

	sliceType := reflect.ValueOf(val).Elem().Type()
	if sliceType.Kind() != reflect.Slice {
		return fmt.Errorf("stage sprites target %q is not a slice", target)
	}
	itemType := sliceType.Elem()
	if itemType.Kind() == reflect.Pointer {
		itemType = itemType.Elem()
	}
	if itemType.Kind() != reflect.Struct {
		return fmt.Errorf("stage sprites target %q has invalid item type %s", target, sliceType.Elem())
	}
	if !reflect.PointerTo(itemType).Implements(tySprite) {
		return fmt.Errorf("stage sprites target %q has invalid item type %s", target, sliceType.Elem())
	}
	if len(items) == 0 {
		return nil
	}
	if err := p.addPrototype(itemType.Name(), itemType, shadow, layer); err != nil {
		return err
	}
	prototype := itemType.Name()
	properties := make([]spriteProperties, 0, len(items))
	for i, item := range items {
		itemShape, ok := item.(coreproject.StageShape)
		if !ok {
			return fmt.Errorf("stage sprites target %q item[%d] has invalid type %T", target, i, item)
		}
		itemProperties, err := parseSpriteProperties(itemShape)
		if err != nil {
			return fmt.Errorf("stage sprites target %q item[%d]: %w", target, i, err)
		}
		properties = append(properties, itemProperties)
		p.recordCostumeOverride(prototype, fmt.Sprintf("stage sprites target %q item[%d]", target, i), itemProperties)
	}
	p.preparedSpriteProperties[layer] = properties
	return nil
}

func (p *reloadPlan) recordCostumeOverride(sprite, location string, properties spriteProperties) {
	if properties.has(spritePropertyCostumeIndex) {
		p.costumeOverrides = append(p.costumeOverrides, reloadCostumeOverride{
			sprite: sprite, location: location, index: properties.costumeIndex,
		})
	}
}

func (p *reloadPlan) validateCostumeOverrides() error {
	for _, override := range p.costumeOverrides {
		prepared, ok := p.preparedSprites[override.sprite]
		if !ok {
			return fmt.Errorf("%s has no sprite configuration", override.location)
		}
		count := len(prepared.layout.Frames)
		if override.index < 0 || override.index >= count {
			return fmt.Errorf("%s costumeIndex %d is outside %d costumes", override.location, override.index, count)
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
		return g.loadSpriteConfigWithLayout(sprite, name, gamer, &prepared.config, prepared.layout)
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
	plan.preparedSpriteProperties = make(stageSpriteProperties, len(plan.project.Zorder))
	if err := validateReloadProjectConfig(&plan.project); err != nil {
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
		loaded, err := coreproject.LoadSpriteConfig(g.fs, name)
		if err != nil {
			return nil, fmt.Errorf("reload preflight: load sprite config %q: %w", name, err)
		}
		costumeLayout, err := validateReloadSpriteConfig(&loaded.Config)
		if err != nil {
			return nil, fmt.Errorf("reload preflight: sprite config %q: %w", name, err)
		}
		plan.preparedSprites[name] = preparedSprite{config: loaded.Config, layout: costumeLayout}
	}
	if err := plan.validateCostumeOverrides(); err != nil {
		return nil, fmt.Errorf("reload preflight: %w", err)
	}
	return plan, nil
}

func validateReloadProjectConfig(project *coreproject.ProjectConfig) error {
	for i, backdrop := range project.Backdrops {
		if backdrop == nil {
			return fmt.Errorf("backdrops[%d] is null", i)
		}
	}
	settings := coreproject.ResolveSystemSettings(project)
	if settings.AutoSetCollisionLayer == project.Physics {
		return fmt.Errorf("autoSetCollisionLayer and physics must have different enabled states")
	}
	return nil
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

// validateReloadSpriteConfig rejects values that would panic during init.
func validateReloadSpriteConfig(cfg *coreproject.SpriteConfig) (*coreproject.CostumeLayout, error) {
	layout, err := coreproject.PrepareCostumeLayout(cfg)
	if err != nil {
		return nil, err
	}
	if err := validateReloadAnimationMap("fAnimations", cfg.FAnimations, layout); err != nil {
		return nil, err
	}
	return layout, nil
}

func validateReloadAnimationMap(kind string, animations map[string]*coreproject.AniConfig, layout *coreproject.CostumeLayout) error {
	for name, animation := range animations {
		if animation == nil {
			return fmt.Errorf("%s[%q] is null", kind, name)
		}
		if err := validateReloadAnimationFrame(kind, name, "frameFrom", animation.FrameFrom, layout); err != nil {
			return err
		}
		if err := validateReloadAnimationFrame(kind, name, "frameTo", animation.FrameTo, layout); err != nil {
			return err
		}
	}
	return nil
}

func validateReloadAnimationFrame(kind, animation, field string, value any, layout *coreproject.CostumeLayout) error {
	if value == nil {
		return nil
	}

	index, ok := layout.ResolveFrameIndex(value)
	if !ok {
		return fmt.Errorf("%s[%q].%s references missing costume %q", kind, animation, field, value)
	}
	costumeCount := len(layout.Frames)
	if index < 0 || index >= costumeCount {
		return fmt.Errorf("%s[%q].%s index %d is outside %d costumes", kind, animation, field, index, costumeCount)
	}
	return nil
}
