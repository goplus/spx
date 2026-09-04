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
	"strconv"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

type reloadPlan struct {
	project         coreproject.ProjectConfig
	spriteConfigs   map[string]coreproject.LoadedSpriteConfig
	configNames     []string
	directSprites   map[string]reflect.Type
	prototypeByName map[string]reflect.Type
}

func prepareReload(g *Game, gamer reflect.Value, index any) (*reloadPlan, error) {
	if g.fs == nil {
		return nil, fmt.Errorf("reload preflight: game resource directory is not initialized")
	}

	plan := &reloadPlan{
		spriteConfigs:   make(map[string]coreproject.LoadedSpriteConfig),
		directSprites:   make(map[string]reflect.Type),
		prototypeByName: make(map[string]reflect.Type),
	}
	if err := coreproject.LoadConfig(&plan.project, g.fs, index); err != nil {
		return nil, fmt.Errorf("reload preflight: load project config: %w", err)
	}
	if err := validateReloadProjectConfig(&plan.project); err != nil {
		return nil, fmt.Errorf("reload preflight: project config: %w", err)
	}

	// Use a throwaway value so pointer/interface allocation and owner validation
	// exactly match the rebuild path without changing the live game.
	shadow := reflect.New(gamer.Type()).Elem()
	err := coreproject.WalkFields(shadow, func(fieldIndex int) (string, any) {
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
		if err := validateReloadSpriteConfig(&loaded.Config); err != nil {
			return nil, fmt.Errorf("reload preflight: sprite config %q: %w", name, err)
		}
		plan.spriteConfigs[name] = loaded
	}
	return plan, nil
}

func (p *reloadPlan) requireSpriteConfig(name string) {
	if _, ok := p.spriteConfigs[name]; ok {
		return
	}
	p.spriteConfigs[name] = coreproject.LoadedSpriteConfig{}
	p.configNames = append(p.configNames, name)
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
				StageMonitor: validateReloadMonitor,
				Measure:      validateReloadMeasure,
				Sprite: func(shape coreproject.StageShape) error {
					return p.validateStageSprite(shape, shadow)
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

func (p *reloadPlan) validateStageSprite(shape coreproject.StageShape, shadow reflect.Value) error {
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
	return validateReloadSpriteProperties(shape)
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
	for i, item := range items {
		itemShape, ok := item.(coreproject.StageShape)
		if !ok {
			return fmt.Errorf("stage sprites target %q item[%d] has invalid type %T", target, i, item)
		}
		if err := validateReloadSpriteProperties(itemShape); err != nil {
			return fmt.Errorf("stage sprites target %q item[%d]: %w", target, i, err)
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

func validateReloadSprite(sprite Sprite, gamer reflect.Value) error {
	typ := reflect.TypeOf(sprite)
	if typ == nil || typ.Kind() != reflect.Pointer || typ.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("invalid sprite type %T", sprite)
	}
	v := reflect.ValueOf(sprite).Elem()
	if v.NumField() == 0 || v.Field(0).Type() != reflect.TypeFor[SpriteImpl]() {
		return fmt.Errorf("sprite %s is missing leading SpriteImpl field", typ)
	}
	return bindSpriteOwner(v, gamer)
}

// validateReloadSpriteConfig checks the parts of SpriteConfig that are consumed
// during SpriteImpl initialization. Those paths historically report malformed
// data through engine/log panics, after the live game has already been reset.
func validateReloadSpriteConfig(cfg *coreproject.SpriteConfig) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}

	costumeCount, costumeNames, err := reloadCostumeLayout(cfg)
	if err != nil {
		return err
	}
	if err := validateReloadAnimationMap("fAnimations", cfg.FAnimations, costumeCount, costumeNames); err != nil {
		return err
	}
	return nil
}

func reloadCostumeLayout(cfg *coreproject.SpriteConfig) (int, map[string]int, error) {
	names := make(map[string]int)
	switch {
	case cfg.Costumes != nil:
		if len(cfg.Costumes) == 0 {
			return 0, nil, fmt.Errorf("costumes must not be empty")
		}
		for i, costume := range cfg.Costumes {
			if costume == nil {
				return 0, nil, fmt.Errorf("costumes[%d] is null", i)
			}
			if _, exists := names[costume.Name]; !exists {
				names[costume.Name] = i
			}
		}
		return len(cfg.Costumes), names, nil

	case cfg.CostumeSet != nil:
		count, err := appendReloadCostumeSet(names, 0, cfg.CostumeSet.Nx, cfg.CostumeSet.Items)
		if err != nil {
			return 0, nil, fmt.Errorf("costumeSet: %w", err)
		}
		return count, names, nil

	case cfg.CostumeMPSet != nil:
		if len(cfg.CostumeMPSet.Parts) == 0 {
			return 0, nil, fmt.Errorf("costumeMPSet.parts must not be empty")
		}
		count := 0
		for i, part := range cfg.CostumeMPSet.Parts {
			partCount, err := appendReloadCostumeSet(names, count, part.Nx, part.Items)
			if err != nil {
				return 0, nil, fmt.Errorf("costumeMPSet.parts[%d]: %w", i, err)
			}
			count += partCount
		}
		return count, names, nil

	default:
		return 0, nil, fmt.Errorf("configuration must define costumes, costumeSet, or costumeMPSet")
	}
}

func appendReloadCostumeSet(names map[string]int, start, nx int, items []coreproject.CostumeSetItem) (int, error) {
	if nx <= 0 {
		return 0, fmt.Errorf("invalid frame count %d", nx)
	}

	// initCSPart intentionally ignores Items for a single-frame set.
	if nx == 1 || items == nil {
		for i := 0; i < nx; i++ {
			name := strconv.Itoa(start + i)
			if nx == 1 {
				name = strconv.Itoa(start)
			}
			if _, exists := names[name]; !exists {
				names[name] = start + i
			}
		}
		return nx, nil
	}

	frameIndex := 0
	for itemIndex, item := range items {
		if item.N < 0 {
			return 0, fmt.Errorf("items[%d] has negative frame count %d", itemIndex, item.N)
		}
		for i := 0; i < item.N; i++ {
			name := item.NamePrefix + strconv.Itoa(i)
			if _, exists := names[name]; !exists {
				names[name] = start + frameIndex
			}
			frameIndex++
		}
	}
	if frameIndex != nx {
		return 0, fmt.Errorf("incomplete frame loading (loaded=%d, expected=%d)", frameIndex, nx)
	}
	return nx, nil
}

func validateReloadAnimationMap(kind string, animations map[string]*coreproject.AniConfig, costumeCount int, costumeNames map[string]int) error {
	for name, animation := range animations {
		if animation == nil {
			return fmt.Errorf("%s[%q] is null", kind, name)
		}
		if err := validateReloadAnimationFrame(kind, name, "frameFrom", animation.FrameFrom, costumeCount, costumeNames); err != nil {
			return err
		}
		if err := validateReloadAnimationFrame(kind, name, "frameTo", animation.FrameTo, costumeCount, costumeNames); err != nil {
			return err
		}
	}
	return nil
}

func validateReloadAnimationFrame(kind, animation, field string, value any, costumeCount int, costumeNames map[string]int) error {
	if value == nil {
		return nil
	}
	if name, ok := value.(string); ok {
		if _, exists := costumeNames[name]; !exists {
			return fmt.Errorf("%s[%q].%s references missing costume %q", kind, animation, field, name)
		}
		return nil
	}

	index := reloadNumericIndex(value)
	if index < 0 || index >= costumeCount {
		return fmt.Errorf("%s[%q].%s index %d is outside %d costumes", kind, animation, field, index, costumeCount)
	}
	return nil
}

func reloadNumericIndex(value any) int {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return 0
	}
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return int(rv.Uint())
	case reflect.Float32, reflect.Float64:
		return int(rv.Float())
	default:
		// animationComponent.costumeIndex treats unsupported values as zero.
		return 0
	}
}

func validateReloadSpriteProperties(shape coreproject.StageShape) error {
	for _, key := range []string{"x", "y", "heading", "size", "costumeIndex"} {
		if err := validateReloadShapeField(shape, key, false, reflect.TypeFor[float64]()); err != nil {
			return err
		}
	}
	if err := validateReloadShapeField(shape, "rotationStyle", false, reflect.TypeFor[string]()); err != nil {
		return err
	}
	return validateReloadShapeField(shape, "visible", false, reflect.TypeFor[bool]())
}

func validateReloadMonitor(shape coreproject.StageShape) error {
	for _, key := range []string{"target", "val", "name", "label"} {
		if err := validateReloadShapeField(shape, key, true, reflect.TypeFor[string]()); err != nil {
			return err
		}
	}
	for _, key := range []string{"mode", "x", "y"} {
		if err := validateReloadShapeField(shape, key, true, reflect.TypeFor[float64]()); err != nil {
			return err
		}
	}
	return validateReloadShapeField(shape, "visible", true, reflect.TypeFor[bool]())
}

func validateReloadMeasure(shape coreproject.StageShape) error {
	for _, key := range []string{"size", "x", "y"} {
		if err := validateReloadShapeField(shape, key, true, reflect.TypeFor[float64]()); err != nil {
			return err
		}
	}
	for _, key := range []string{"scale", "heading"} {
		if err := validateReloadShapeField(shape, key, false, reflect.TypeFor[float64]()); err != nil {
			return err
		}
	}
	if color, ok := shape["color"]; ok {
		if _, err := mathf.NewColorAny(color); err != nil {
			return fmt.Errorf("stage shape field %q: %w", "color", err)
		}
	}
	return nil
}

func validateReloadShapeField(shape coreproject.StageShape, key string, required bool, want reflect.Type) error {
	value, ok := shape[key]
	if !ok {
		if required {
			return fmt.Errorf("stage shape field %q is required", key)
		}
		return nil
	}
	if reflect.TypeOf(value) != want {
		return fmt.Errorf("stage shape field %q has type %T, want %s", key, value, want)
	}
	return nil
}

func (p *reloadPlan) loadSprites(g *Game, gamer reflect.Value) error {
	loadSprite := p.spriteLoader(g)
	err := coreproject.WalkFields(gamer, func(fieldIndex int) (string, any) {
		return getFieldPtrOrAlloc(g, gamer, fieldIndex)
	}, func(name string, val any) error {
		sprite, ok := val.(Sprite)
		if !ok {
			return nil
		}
		return loadSprite(sprite, name, gamer)
	})
	return err
}

func (p *reloadPlan) spriteLoader(g *Game) spriteLoader {
	return func(sprite Sprite, name string, gamer reflect.Value) error {
		loaded, ok := p.spriteConfigs[name]
		if !ok {
			return fmt.Errorf("reload plan has no sprite config for %q", name)
		}
		return g.loadSpriteConfig(sprite, name, gamer, &loaded.Config)
	}
}
