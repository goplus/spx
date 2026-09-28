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
	"unsafe"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
	"github.com/goplus/spx/v3/internal/ui"
)

type stageEntryKind uint8

const (
	stageNamedSprite stageEntryKind = iota
	stageSprite
	stageSprites
	stageMonitor
	stageMeasure
)

// preparedStageEntry owns parsed values and stable field indices, never values
// pointing into the shadow game used by reload preflight.
type preparedStageEntry struct {
	kind       stageEntryKind
	name       string
	fieldIndex int
	spriteType reflect.Type
	properties []spriteProperties
	monitor    preparedMonitor
	measure    coreproject.MeasureShape
}

func prepareStageEntry(gamer reflect.Value, raw any) (preparedStageEntry, error) {
	if name, ok := raw.(string); ok {
		return preparedStageEntry{kind: stageNamedSprite, name: name}, nil
	}
	var entry preparedStageEntry
	shape, ok := raw.(coreproject.StageShape)
	if !ok {
		return entry, fmt.Errorf("invalid zorder entry type %T", raw)
	}
	err := coreproject.DispatchStageShape(shape, coreproject.StageShapeHandlers{
		StageMonitor: func(shape coreproject.StageShape) (err error) {
			entry.kind = stageMonitor
			entry.monitor, err = prepareMonitor(shape)
			return
		},
		Measure: func(shape coreproject.StageShape) (err error) {
			entry.kind = stageMeasure
			entry.measure, err = coreproject.ParseMeasureShape(shape)
			return
		},
		Sprite: func(shape coreproject.StageShape) (err error) {
			entry, err = prepareStageSprite(gamer, shape, false)
			return
		},
		Sprites: func(shape coreproject.StageShape) (err error) {
			entry, err = prepareStageSprite(gamer, shape, true)
			return
		},
	})
	return entry, err
}

func prepareStageSprite(gamer reflect.Value, shape coreproject.StageShape, multiple bool) (preparedStageEntry, error) {
	entry := preparedStageEntry{kind: stageSprite, fieldIndex: -1}
	target, err := stageShapeTarget(shape)
	if err != nil {
		return entry, err
	}
	entry.name = target
	var items []any
	if multiple {
		entry.kind = stageSprites
		items, err = stageShapeItems(shape)
		if err != nil {
			return entry, err
		}
	}
	for i := range gamer.NumField() {
		if gamer.Type().Field(i).Name == target {
			entry.fieldIndex = i
			break
		}
	}
	if entry.fieldIndex < 0 {
		kind := "sprite"
		if multiple {
			kind = "sprites"
		}
		return entry, fmt.Errorf("stage %s target %q is not defined", kind, target)
	}
	field := stageField(gamer, entry.fieldIndex)
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

func stageField(gamer reflect.Value, index int) reflect.Value {
	field := gamer.Field(index)
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
}

func stageSpriteValue(field reflect.Value) any {
	if field.Kind() == reflect.Pointer || field.Kind() == reflect.Interface {
		return field.Interface()
	}
	return field.Addr().Interface()
}

func (p *Game) loadAndInitSprites(gamer reflect.Value, project *coreproject.ProjectConfig, loadSprite spriteLoader, entries []preparedStageEntry) []Sprite {
	count := len(entries)
	if entries == nil {
		count = len(project.Zorder)
	}
	inits := make([]Sprite, 0, count)
	for layer := range count {
		var entry preparedStageEntry
		if entries == nil {
			// Cold loading preserves lazy loading and failure order between entries.
			var err error
			entry, err = prepareStageEntry(gamer, project.Zorder[layer])
			if err != nil {
				engine.Panic(fmt.Errorf("zorder[%d]: %w", layer, err))
				break
			}
		} else {
			entry = entries[layer]
		}
		inits = p.appendStageEntry(gamer, entry, loadSprite, layer, inits)
	}
	// Expanded stage groups occupy contiguous layers above the shared pen canvas.
	p.shapeMgr.updateRenderLayers()
	return inits
}

func (p *Game) appendStageEntry(gamer reflect.Value, entry preparedStageEntry, loadSprite spriteLoader, layer int, inits []Sprite) []Sprite {
	switch entry.kind {
	case stageNamedSprite:
		sp := p.getSpriteProtoByName(entry.name, gamer, loadSprite)
		spr := spriteOf(sp)
		spr.setLayer(layer + firstSpriteLayer)
		p.shapeMgr.add(spr)
		inits = append(inits, sp)
	case stageSprite:
		sp := stageSpriteValue(stageField(gamer, entry.fieldIndex)).(Sprite)
		dest := spriteOf(sp)
		applySpriteProperties(dest, entry.properties[0])
		p.shapeMgr.add(dest)
		inits = append(inits, sp)
	case stageSprites:
		field := stageField(gamer, entry.fieldIndex)
		items := reflect.MakeSlice(field.Type(), len(entry.properties), len(entry.properties))
		for i, properties := range entry.properties {
			item := items.Index(i)
			if item.Kind() == reflect.Pointer {
				item.Set(reflect.New(entry.spriteType))
				item = item.Elem()
			}
			prototype := p.getSpriteProto(entry.spriteType, gamer, loadSprite)
			dest, sp := instantiateStageSprite(item, prototype, properties)
			p.shapeMgr.add(dest)
			inits = append(inits, sp)
		}
		field.Set(items)
	case stageMonitor:
		monitor, err := newMonitor(gamer, entry.monitor)
		if err != nil {
			spxlog.Error("Skip monitor %q: %v", entry.monitor.config.Name, err)
			return inits
		}
		p.shapeMgr.add(monitor)
	case stageMeasure:
		p.shapeMgr.add(ui.NewMeasureShape(entry.measure))
	}
	return inits
}

func stageShapeTarget(shape coreproject.StageShape) (string, error) {
	target, ok := shape["target"].(string)
	if !ok || target == "" {
		return "", fmt.Errorf("stage shape target must be a non-empty string")
	}
	return target, nil
}

func stageShapeItems(shape coreproject.StageShape) ([]any, error) {
	items, ok := shape["items"].([]any)
	if !ok {
		return nil, fmt.Errorf("stage shape items must be an array")
	}
	return items, nil
}
