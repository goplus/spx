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

	spxfs "github.com/goplus/spx/v4/fs"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	spxlog "github.com/goplus/spx/v4/internal/log"
	"github.com/goplus/spx/v4/internal/ui"
)

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

func (p *Game) applyStageEntries(gamer reflect.Value, plan *projectPlan) []Sprite {
	sprites := make([]Sprite, 0, len(plan.stage))
	for layer, entry := range plan.stage {
		switch entry.kind {
		case "prototype":
			prototype := p.applySpritePrototype(entry.prototype, gamer, plan)
			spr := spriteOf(prototype)
			spr.setLayer(layer + firstSpriteLayer)
			p.shapeMgr.add(spr)
			sprites = append(sprites, prototype)
		case "sprite":
			field := coreproject.WritableField(gamer.Field(entry.sprite.field))
			sp := stageSpriteValue(field).(Sprite)
			dest := spriteOf(sp)
			applySpriteProperties(dest, entry.sprite.properties[0])
			p.shapeMgr.add(dest)
			sprites = append(sprites, sp)
		case "sprites":
			field := coreproject.WritableField(gamer.Field(entry.sprite.field))
			items := reflect.MakeSlice(field.Type(), len(entry.sprite.properties), len(entry.sprite.properties))
			var prototype Sprite
			if len(entry.sprite.properties) != 0 {
				prototype = p.applySpritePrototype(entry.prototype, gamer, plan)
			}
			for i, properties := range entry.sprite.properties {
				item := items.Index(i)
				if item.Kind() == reflect.Pointer {
					item.Set(reflect.New(entry.sprite.typ))
					item = item.Elem()
				}
				dest, sp := instantiateStageSprite(item, prototype, properties)
				p.shapeMgr.add(dest)
				sprites = append(sprites, sp)
			}
			field.Set(items)
		case "stageMonitor", "monitor":
			monitor, err := newMonitor(gamer, entry.monitor)
			if err != nil {
				spxlog.Error("Skip monitor %q: %v", entry.monitor.config.Name, err)
				continue
			}
			p.shapeMgr.add(monitor)
		case "measure":
			p.shapeMgr.add(ui.NewMeasureShape(entry.measure))
		}
	}
	// Expanded groups keep contiguous layers above the pen canvas.
	p.shapeMgr.updateRenderLayers()
	return sprites
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
