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
	spxlog "github.com/goplus/spx/v3/internal/log"
	"github.com/goplus/spx/v3/internal/ui"
)

// preparedStageSprite keeps a validated live field and its parsed properties.
type preparedStageSprite struct {
	field      reflect.Value
	spriteType reflect.Type
	properties []spriteProperties
}

func prepareStageSprite(gamer reflect.Value, shape coreproject.StageShape, multiple bool) (preparedStageSprite, error) {
	var entry preparedStageSprite
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
	fieldPtr := coreproject.FindFieldPtr(gamer, target, 0)
	if fieldPtr == nil {
		kind := "sprite"
		if multiple {
			kind = "sprites"
		}
		return entry, fmt.Errorf("stage %s target %q is not defined", kind, target)
	}
	field := reflect.ValueOf(fieldPtr).Elem()
	entry.field = field
	if !multiple {
		if _, ok := stageSpriteValue(field).(Sprite); !ok {
			return entry, fmt.Errorf("stage sprite target %q is not a sprite field", target)
		}
		properties, err := parseSpriteProperties(shape)
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
	entry.spriteType = typ
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

func (p *Game) loadStageEntries(gamer reflect.Value, entries []any) ([]Sprite, error) {
	sprites := make([]Sprite, 0, len(entries))
	for layer, raw := range entries {
		// Load each entry before parsing the next to preserve error order.
		var err error
		sprites, err = p.loadStageEntry(gamer, raw, layer, sprites)
		if err != nil {
			return nil, fmt.Errorf("zorder[%d]: %w", layer, err)
		}
	}
	// Expanded groups keep contiguous layers above the pen canvas.
	p.shapeMgr.updateRenderLayers()
	return sprites, nil
}

func (p *Game) loadStageEntry(gamer reflect.Value, raw any, layer int, sprites []Sprite) ([]Sprite, error) {
	if name, ok := raw.(string); ok {
		sp, err := p.loadSpritePrototype(name, p.typs[name], gamer)
		if err != nil {
			return nil, err
		}
		spr := spriteOf(sp)
		spr.setLayer(layer + firstSpriteLayer)
		p.shapeMgr.add(spr)
		return append(sprites, sp), nil
	}
	shape, ok := raw.(coreproject.StageShape)
	if !ok {
		return nil, fmt.Errorf("invalid zorder entry type %T", raw)
	}
	typ, ok := shape["type"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid stage shape type")
	}
	switch typ {
	case "sprite", "sprites":
		entry, err := prepareStageSprite(gamer, shape, typ == "sprites")
		if err != nil {
			return nil, err
		}
		if typ == "sprite" {
			sp := stageSpriteValue(entry.field).(Sprite)
			dest := spriteOf(sp)
			applySpriteProperties(dest, entry.properties[0])
			p.shapeMgr.add(dest)
			return append(sprites, sp), nil
		}
		items := reflect.MakeSlice(entry.field.Type(), len(entry.properties), len(entry.properties))
		var prototype Sprite
		if len(entry.properties) != 0 {
			prototype, err = p.loadSpritePrototype(entry.spriteType.Name(), entry.spriteType, gamer)
			if err != nil {
				return nil, err
			}
		}
		for i, properties := range entry.properties {
			item := items.Index(i)
			if item.Kind() == reflect.Pointer {
				item.Set(reflect.New(entry.spriteType))
				item = item.Elem()
			}
			dest, sp := instantiateStageSprite(item, prototype, properties)
			p.shapeMgr.add(dest)
			sprites = append(sprites, sp)
		}
		entry.field.Set(items)
	case "stageMonitor", "monitor":
		prepared, err := prepareMonitor(shape)
		if err != nil {
			return nil, err
		}
		monitor, err := newMonitor(gamer, prepared)
		if err != nil {
			spxlog.Error("Skip monitor %q: %v", prepared.config.Name, err)
			return sprites, nil
		}
		p.shapeMgr.add(monitor)
	case "measure":
		measure, err := coreproject.ParseMeasureShape(shape)
		if err != nil {
			return nil, err
		}
		p.shapeMgr.add(ui.NewMeasureShape(measure))
	default:
		return nil, fmt.Errorf("unknown shape - %s", typ)
	}
	return sprites, nil
}

func (p *Game) loadSpritePrototype(name string, typ reflect.Type, gamer reflect.Value) (Sprite, error) {
	if spr, ok := p.sprs[name]; ok {
		return spr, nil
	}
	if typ == nil {
		return nil, fmt.Errorf("sprite %q is not defined", name)
	}
	spr := reflect.New(typ).Interface().(Sprite)
	if err := p.loadSprite(spr, name, gamer); err != nil {
		return nil, fmt.Errorf("load sprite %q: %w", name, err)
	}
	return spr, nil
}
