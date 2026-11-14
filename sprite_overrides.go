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

	coreproject "github.com/goplus/spx/v4/internal/core/project"
)

type spriteOverride uint8

const (
	spriteOverrideX spriteOverride = 1 << iota
	spriteOverrideY
	spriteOverrideHeading
	spriteOverrideRotationStyle
	spriteOverrideVisible
	spriteOverrideSize
	spriteOverrideCostumeIndex
)

// spriteOverrides is the validated subset of a stage shape that overrides a
// sprite. present keeps omitted fields distinct from explicit zero values.
type spriteOverrides struct {
	present       spriteOverride
	x, y          float64
	heading       float64
	rotationStyle RotationStyle
	visible       bool
	size          float64
	costumeIndex  int
}

func (p spriteOverrides) has(property spriteOverride) bool {
	return p.present&property != 0
}

func parseSpriteOverrides(shape coreproject.StageShape) (spriteOverrides, error) {
	var props spriteOverrides
	floatFields := [...]struct {
		name    string
		value   *float64
		present spriteOverride
	}{
		{"x", &props.x, spriteOverrideX},
		{"y", &props.y, spriteOverrideY},
		{"heading", &props.heading, spriteOverrideHeading},
		{"size", &props.size, spriteOverrideSize},
	}
	for _, field := range floatFields {
		raw, ok := shape[field.name]
		if !ok {
			continue
		}
		value, ok := raw.(float64)
		if !ok {
			return spriteOverrides{}, invalidSpriteOverrideType(field.name, raw, "float64")
		}
		*field.value = value
		props.present |= field.present
	}

	if raw, ok := shape["rotationStyle"]; ok {
		value, ok := raw.(string)
		if !ok {
			return spriteOverrides{}, invalidSpriteOverrideType("rotationStyle", raw, "string")
		}
		props.rotationStyle = toRotationStyle(value)
		props.present |= spriteOverrideRotationStyle
	}
	if raw, ok := shape["visible"]; ok {
		value, ok := raw.(bool)
		if !ok {
			return spriteOverrides{}, invalidSpriteOverrideType("visible", raw, "bool")
		}
		props.visible = value
		props.present |= spriteOverrideVisible
	}
	if raw, ok := shape["costumeIndex"]; ok {
		value, ok := raw.(float64)
		if !ok {
			return spriteOverrides{}, invalidSpriteOverrideType("costumeIndex", raw, "float64")
		}
		props.costumeIndex = int(value)
		props.present |= spriteOverrideCostumeIndex
	}
	return props, nil
}

func invalidSpriteOverrideType(name string, value any, want string) error {
	return fmt.Errorf("stage shape field %q has type %T, want %s", name, value, want)
}

func applySpriteOverrides(dest *SpriteImpl, props spriteOverrides) {
	transform := dest.transform()
	if props.has(spriteOverrideX) {
		transform.x = props.x
	}
	if props.has(spriteOverrideY) {
		transform.y = props.y
	}
	if props.has(spriteOverrideHeading) {
		transform.direction = props.heading
	}
	if props.has(spriteOverrideRotationStyle) {
		transform.rotationStyle = props.rotationStyle
	}
	if props.has(spriteOverrideVisible) {
		dest.spriteState.isVisible = props.visible
	}
	if props.has(spriteOverrideSize) {
		dest.runtimeState.scale = props.size
	}
	if props.has(spriteOverrideCostumeIndex) {
		dest.setCostumeIndex(props.costumeIndex)
	}
	dest.spriteState.cloned = false
}
