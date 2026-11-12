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

package spx

import (
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"

	spxfs "github.com/goplus/spx/v4/fs"
	"github.com/goplus/spx/v4/internal/base"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	tm "github.com/goplus/spx/v4/internal/core/tilemap"
)

// projectPlan owns parsed input. Preparing it neither changes live game
// fields nor allocates engine objects; applying it consumes the same input.
type projectPlan struct {
	sprites map[string]spritePlan
	stage   []stageEntry
	tilemap tm.LoadResult
}

type spritePlan struct {
	*spriteLayout
	preparedSprite
}

type preparedSprite struct {
	config coreproject.SpriteConfig
	layout *coreproject.CostumeLayout
}

type stageEntry struct {
	kind      string
	prototype string
	sprite    stageSprite
	monitor   preparedMonitor
	measure   coreproject.MeasureShape
}

// stageSprite binds the same direct field on the preparation copy and live game.
type stageSprite struct {
	field      int
	typ        reflect.Type
	properties []spriteProperties
}

func (p *Game) prepareProject(fs spxfs.Dir, proj *coreproject.ProjectConfig, gamer reflect.Value) (projectPlan, error) {
	plan := projectPlan{sprites: make(map[string]spritePlan)}
	if err := validateProjectConfig(proj); err != nil {
		return plan, fmt.Errorf("project config: %w", err)
	}
	// Allocate pointer/interface fields on a copy so stage target validation
	// sees their eventual types without publishing partially loaded fields.
	var preview reflect.Value
	if gamer.IsValid() {
		if !gamer.CanAddr() {
			return plan, fmt.Errorf("game value is not addressable")
		}
		preview = reflect.New(gamer.Type()).Elem()
		preview.Set(gamer)
		for i := range preview.NumField() {
			name, sprite := p.allocSpriteField(preview, i)
			if sprite == nil {
				continue
			}
			prepared, err := prepareSprite(fs, name, reflect.TypeOf(sprite).Elem(), gamer)
			if err != nil {
				return plan, fmt.Errorf("load sprite %q: %w", name, err)
			}
			plan.sprites[name] = prepared
		}
	}
	var err error
	plan.tilemap, err = tm.Load(fs, proj.TilemapPath)
	if err != nil {
		return plan, fmt.Errorf("load tilemap %q: %w", proj.TilemapPath, err)
	}
	plan.stage, err = p.prepareStageEntries(fs, preview, proj.Zorder, &plan)
	return plan, err
}

func (p *Game) prepareSpritePrototype(fs spxfs.Dir, name string, typ reflect.Type, gamer reflect.Value, plan *projectPlan) error {
	var actual reflect.Type
	if prepared, ok := plan.sprites[name]; ok {
		actual = prepared.typ
	} else if sprite, ok := p.sprs[name]; ok {
		if spriteOf(sprite) == nil {
			return fmt.Errorf("sprite %q has no valid prototype", name)
		}
		actual = reflect.TypeOf(sprite).Elem()
	} else {
		if typ == nil {
			return fmt.Errorf("sprite %q is not defined", name)
		}
		prepared, err := prepareSprite(fs, name, typ, gamer)
		if err != nil {
			return fmt.Errorf("load sprite %q: %w", name, err)
		}
		plan.sprites[name] = prepared
		return nil
	}
	if typ != nil && actual != typ {
		return fmt.Errorf("sprite %q prototype type %s does not match %s", name, actual, typ)
	}
	return nil
}

// Allocate all eligible fields, but return only registered sprites to load.
// Registered non-sprite interface fields are initialized too.
func (g *Game) allocSpriteField(v reflect.Value, i int) (string, Sprite) {
	name := v.Type().Field(i).Name
	field := coreproject.WritableField(v.Field(i))
	value := field.Addr()
	switch field.Kind() {
	case reflect.Pointer:
		if field.Type().Implements(tySprite) {
			value = reflect.New(field.Type().Elem())
			field.Set(value)
		}
	case reflect.Interface:
		if typ, ok := g.typs[name]; ok {
			value = reflect.New(typ)
			field.Set(value)
		}
	}
	if g.typs[name] == nil {
		return name, nil
	}
	sprite, _ := reflect.TypeAssert[Sprite](value)
	return name, sprite
}

func (p *Game) prepareStageEntries(fs spxfs.Dir, gamer reflect.Value, entries []any, plan *projectPlan) ([]stageEntry, error) {
	prepared := make([]stageEntry, len(entries))
	for layer, raw := range entries {
		// Prepare each prototype before parsing the next entry: a later shape
		// error must not hide an earlier missing or invalid sprite config.
		entry, err := p.prepareStageEntry(fs, gamer, raw, plan)
		if err != nil {
			return nil, fmt.Errorf("zorder[%d]: %w", layer, err)
		}
		prepared[layer] = entry
	}
	return prepared, nil
}

func (p *Game) prepareStageEntry(fs spxfs.Dir, gamer reflect.Value, raw any, plan *projectPlan) (stageEntry, error) {
	var entry stageEntry
	if name, ok := raw.(string); ok {
		entry.kind, entry.prototype = "prototype", name
		return entry, p.prepareSpritePrototype(fs, name, p.typs[name], gamer, plan)
	}
	shape, ok := raw.(coreproject.StageShape)
	if !ok {
		return entry, fmt.Errorf("invalid zorder entry type %T", raw)
	}
	entry.kind, ok = shape["type"].(string)
	if !ok {
		return entry, fmt.Errorf("invalid stage shape type")
	}
	var err error
	switch entry.kind {
	case "sprite", "sprites":
		entry.sprite, err = prepareStageSprite(gamer, shape, entry.kind == "sprites")
		if err != nil || entry.kind == "sprite" || len(entry.sprite.properties) == 0 {
			return entry, err
		}
		entry.prototype = entry.sprite.typ.Name()
		if err = p.prepareSpritePrototype(fs, entry.prototype, entry.sprite.typ, gamer, plan); err != nil {
			return entry, err
		}
		_, err = inspectCloneLayout(entry.sprite.typ)
	case "stageMonitor", "monitor":
		entry.monitor, err = prepareMonitor(shape)
	case "measure":
		entry.measure, err = coreproject.ParseMeasureShape(shape)
	default:
		err = fmt.Errorf("unknown shape - %s", entry.kind)
	}
	return entry, err
}

func prepareSprite(fs spxfs.Dir, name string, typ reflect.Type, gamer reflect.Value) (spritePlan, error) {
	config, err := coreproject.LoadSpriteConfig(fs, name)
	if err != nil {
		return spritePlan{}, err
	}
	prepared, err := prepareSpriteConfig(&config)
	if err != nil {
		return spritePlan{}, fmt.Errorf("sprite config %q: %w", name, err)
	}
	layout, err := inspectSpriteLayout(typ)
	if err != nil {
		return spritePlan{}, err
	}
	if err := layout.validateOwner(gamer.Type()); err != nil {
		return spritePlan{}, err
	}
	if !gamer.CanAddr() {
		return spritePlan{}, fmt.Errorf("sprite %s owner game is not addressable", typ)
	}
	return spritePlan{spriteLayout: layout, preparedSprite: prepared}, nil
}

func prepareSpriteConfig(config *coreproject.SpriteConfig) (preparedSprite, error) {
	preparedConfig := *config
	spine, err := prepareSpineAnimation(config.SpineAnim)
	if err != nil {
		return preparedSprite{}, err
	}
	preparedConfig.SpineAnim = spine
	var layout *coreproject.CostumeLayout
	if spine == nil {
		layout, err = coreproject.PrepareCostumeLayout(config)
		if err != nil {
			return preparedSprite{}, err
		}
	}
	var animations map[string]*coreproject.AniConfig
	if spine != nil {
		animations, err = prepareSpineAnimations(config.FAnimations, spine)
	} else {
		animations, err = prepareFrameAnimations(config.FAnimations, layout)
	}
	if err != nil {
		return preparedSprite{}, err
	}
	preparedConfig.FAnimations = animations
	return preparedSprite{config: preparedConfig, layout: layout}, nil
}

func prepareSpineAnimation(config *coreproject.SpineAnimConfig) (*coreproject.SpineAnimConfig, error) {
	if config == nil {
		return nil, nil
	}
	if config.Atlas == "" || config.Skeleton == "" {
		return nil, fmt.Errorf("spineAnim requires atlas and skeleton")
	}
	prepared := *config
	for _, field := range []struct {
		name     string
		value    *float64
		fallback float64
	}{
		{"defaultMix", &prepared.DefaultMix, 0.1},
		{"stepDuration", &prepared.StepDuration, 0.01},
		{"turnToDuration", &prepared.TurnToDuration, 1},
	} {
		if err := prepareSpineTiming("spineAnim."+field.name, field.value, field.fallback); err != nil {
			return nil, err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(config.AnimMap)) {
		if name == "" || config.AnimMap[name] == "" {
			return nil, fmt.Errorf("spineAnim.animMap requires non-empty aliases and animation names")
		}
	}
	prepared.AnimMap = maps.Clone(config.AnimMap)
	return &prepared, nil
}

func prepareSpineTiming(name string, value *float64, fallback float64) error {
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
		return fmt.Errorf("%s must be finite and nonnegative", name)
	}
	base.SetDefaultIfZero(value, fallback)
	return nil
}

func prepareSpineAnimations(animations map[string]*coreproject.AniConfig, spine *coreproject.SpineAnimConfig) (map[string]*coreproject.AniConfig, error) {
	if animations == nil && len(spine.AnimMap) == 0 {
		return nil, nil
	}
	prepared := make(map[string]*coreproject.AniConfig, len(animations))
	for _, name := range slices.Sorted(maps.Keys(animations)) {
		animation := animations[name]
		if animation == nil {
			return nil, fmt.Errorf("fAnimations[%q] is null", name)
		}
		definition := *animation
		if err := prepareSpineTiming(fmt.Sprintf("fAnimations[%q].stepDuration", name), &definition.StepDuration, 0.01); err != nil {
			return nil, err
		}
		if err := prepareSpineTiming(fmt.Sprintf("fAnimations[%q].turnToDuration", name), &definition.TurnToDuration, 1); err != nil {
			return nil, err
		}
		definition.Speed = 1
		prepared[name] = &definition
	}
	// Materialize aliases here so runtime initialization consumes only prepared
	// definitions. Explicit metadata keeps its own timing defaults.
	for name := range spine.AnimMap {
		if _, exists := prepared[name]; !exists {
			prepared[name] = &coreproject.AniConfig{
				StepDuration: spine.StepDuration, TurnToDuration: spine.TurnToDuration, Speed: 1,
			}
		}
	}
	return prepared, nil
}

func prepareFrameAnimations(animations map[string]*coreproject.AniConfig, layout *coreproject.CostumeLayout) (map[string]*coreproject.AniConfig, error) {
	if animations == nil {
		return nil, nil
	}
	prepared := make(map[string]*coreproject.AniConfig, len(animations))
	for _, name := range slices.Sorted(maps.Keys(animations)) {
		animation := animations[name]
		if animation == nil {
			return nil, fmt.Errorf("fAnimations[%q] is null", name)
		}
		frameFrom, err := prepareAnimationFrame(name, "frameFrom", animation.FrameFrom, layout)
		if err != nil {
			return nil, err
		}
		frameTo, err := prepareAnimationFrame(name, "frameTo", animation.FrameTo, layout)
		if err != nil {
			return nil, err
		}

		definition := *animation
		base.SetDefaultIfZero(&definition.FrameFps, 25)
		base.SetDefaultIfZero(&definition.TurnToDuration, 1.0)
		base.SetDefaultIfZero(&definition.StepDuration, 0.01)
		definition.IFrameFrom = frameFrom
		definition.IFrameTo = frameTo
		definition.Speed = 1
		definition.Duration = (math.Abs(float64(frameFrom-frameTo)) + 1) / float64(definition.FrameFps)
		prepared[name] = &definition
	}
	return prepared, nil
}

func prepareAnimationFrame(animation, field string, value any, layout *coreproject.CostumeLayout) (int, error) {
	index, ok := layout.ResolveFrameIndex(value)
	if !ok {
		return 0, fmt.Errorf("fAnimations[%q].%s references missing costume %q", animation, field, value)
	}
	costumeCount := len(layout.Frames)
	if index < 0 || index >= costumeCount {
		return 0, fmt.Errorf("fAnimations[%q].%s index %d is outside %d costumes", animation, field, index, costumeCount)
	}
	return index, nil
}

func validateProjectConfig(project *coreproject.ProjectConfig) error {
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

func prepareStageSprite(gamer reflect.Value, shape coreproject.StageShape, multiple bool) (stageSprite, error) {
	var entry stageSprite
	target, ok := shape["target"].(string)
	if !ok || target == "" {
		return entry, fmt.Errorf("stage shape target must be a non-empty string")
	}
	var items []any
	if multiple {
		items, ok = shape["items"].([]any)
		if !ok {
			return entry, fmt.Errorf("stage shape items must be an array")
		}
	}
	fieldType, ok := gamer.Type().FieldByName(target)
	if !ok || len(fieldType.Index) != 1 {
		kind := "sprite"
		if multiple {
			kind = "sprites"
		}
		return entry, fmt.Errorf("stage %s target %q is not defined", kind, target)
	}
	entry.field = fieldType.Index[0]
	field := coreproject.WritableField(gamer.Field(entry.field))
	if !multiple {
		sprite, ok := stageSpriteValue(field).(Sprite)
		if !ok {
			return entry, fmt.Errorf("stage sprite target %q is not a sprite field", target)
		}
		properties, err := parseSpriteProperties(shape)
		if err == nil && spriteOf(sprite) == nil {
			err = fmt.Errorf("stage sprite target %q has no sprite value", target)
		}
		entry.properties = []spriteProperties{properties}
		return entry, err
	}
	if field.Kind() != reflect.Slice {
		return entry, fmt.Errorf("stage sprites target %q is not a slice", target)
	}
	typ := field.Type().Elem()
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || !reflect.PointerTo(typ).Implements(tySprite) {
		return entry, fmt.Errorf("stage sprites target %q has invalid item type %s", target, field.Type().Elem())
	}
	entry.typ = typ
	entry.properties = make([]spriteProperties, len(items))
	for i, item := range items {
		shape, ok := item.(coreproject.StageShape)
		if !ok {
			return entry, fmt.Errorf("stage sprites target %q item[%d] has invalid type %T", target, i, item)
		}
		properties, err := parseSpriteProperties(shape)
		if err != nil {
			return entry, fmt.Errorf("stage sprites target %q item[%d]: %w", target, i, err)
		}
		entry.properties[i] = properties
	}
	return entry, nil
}

func stageSpriteValue(field reflect.Value) any {
	if field.Kind() == reflect.Pointer || field.Kind() == reflect.Interface {
		return field.Interface()
	}
	return field.Addr().Interface()
}
