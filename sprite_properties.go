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

	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

type spriteProperty uint8

const (
	spritePropertyX spriteProperty = 1 << iota
	spritePropertyY
	spritePropertyHeading
	spritePropertyRotationStyle
	spritePropertyVisible
	spritePropertySize
	spritePropertyCostumeIndex
)

// spriteProperties is the validated subset of a stage shape that overrides a
// sprite. present keeps omitted fields distinct from explicit zero values.
type spriteProperties struct {
	present       spriteProperty
	x, y          float64
	heading       float64
	rotationStyle RotationStyle
	visible       bool
	size          float64
	costumeIndex  int
}

// stageSpriteProperties contains values prepared during reload preflight. A
// nil slice means this is a regular load and properties still need parsing.
type stageSpriteProperties [][]spriteProperties

func (p stageSpriteProperties) resolve(
	layer, item int,
	shape coreproject.StageShape,
) (spriteProperties, error) {
	if p == nil {
		return parseSpriteProperties(shape)
	}
	if layer < 0 || layer >= len(p) || item < 0 || item >= len(p[layer]) {
		return spriteProperties{}, fmt.Errorf(
			"prepared sprite properties are missing for zorder[%d] item[%d]", layer, item,
		)
	}
	return p[layer][item], nil
}

func parseSpriteProperties(shape coreproject.StageShape) (spriteProperties, error) {
	var props spriteProperties
	floatFields := [...]struct {
		name    string
		value   *float64
		present spriteProperty
	}{
		{"x", &props.x, spritePropertyX},
		{"y", &props.y, spritePropertyY},
		{"heading", &props.heading, spritePropertyHeading},
		{"size", &props.size, spritePropertySize},
	}
	for _, field := range floatFields {
		raw, ok := shape[field.name]
		if !ok {
			continue
		}
		value, ok := raw.(float64)
		if !ok {
			return spriteProperties{}, invalidSpritePropertyType(field.name, raw, "float64")
		}
		*field.value = value
		props.present |= field.present
	}

	if raw, ok := shape["rotationStyle"]; ok {
		value, ok := raw.(string)
		if !ok {
			return spriteProperties{}, invalidSpritePropertyType("rotationStyle", raw, "string")
		}
		props.rotationStyle = toRotationStyle(value)
		props.present |= spritePropertyRotationStyle
	}
	if raw, ok := shape["visible"]; ok {
		value, ok := raw.(bool)
		if !ok {
			return spriteProperties{}, invalidSpritePropertyType("visible", raw, "bool")
		}
		props.visible = value
		props.present |= spritePropertyVisible
	}
	if raw, ok := shape["costumeIndex"]; ok {
		value, ok := raw.(float64)
		if !ok {
			return spriteProperties{}, invalidSpritePropertyType("costumeIndex", raw, "float64")
		}
		props.costumeIndex = int(value)
		props.present |= spritePropertyCostumeIndex
	}
	return props, nil
}

func invalidSpritePropertyType(name string, value any, want string) error {
	return fmt.Errorf("stage shape field %q has type %T, want %s", name, value, want)
}

func (p spriteProperties) has(property spriteProperty) bool {
	return p.present&property != 0
}

func applySpriteProperties(dest *SpriteImpl, props spriteProperties) {
	transform := dest.transform()
	if props.has(spritePropertyX) {
		transform.x = props.x
	}
	if props.has(spritePropertyY) {
		transform.y = props.y
	}
	if props.has(spritePropertyHeading) {
		transform.direction = props.heading
	}
	if props.has(spritePropertyRotationStyle) {
		transform.rotationStyle = props.rotationStyle
	}
	if props.has(spritePropertyVisible) {
		dest.spriteState.IsVisible = props.visible
	}
	if props.has(spritePropertySize) {
		dest.runtimeState.Scale = props.size
	}
	if props.has(spritePropertyCostumeIndex) {
		dest.setCostumeIndex(props.costumeIndex)
	}
	dest.spriteState.Cloned = false
}
