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
	"strings"
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

func TestSpritePropertiesDistinguishMissingAndZeroValues(t *testing.T) {
	newSprite := func() *SpriteImpl {
		sprite := &SpriteImpl{}
		sprite.components.transform = &transformComponent{
			x:             10,
			y:             20,
			direction:     30,
			rotationStyle: LeftRight,
		}
		sprite.spriteState.IsVisible = true
		sprite.spriteState.Cloned = true
		sprite.runtimeState.Scale = 2
		sprite.costumeIndex = 3
		return sprite
	}

	missing, err := parseSpriteProperties(coreproject.StageShape{})
	if err != nil {
		t.Fatal(err)
	}
	unchanged := newSprite()
	applySpriteProperties(unchanged, missing)
	if transform := unchanged.transform(); transform.x != 10 || transform.y != 20 ||
		transform.direction != 30 || transform.rotationStyle != LeftRight {
		t.Fatalf("missing properties changed transform: %+v", transform)
	}
	if !unchanged.spriteState.IsVisible || unchanged.runtimeState.Scale != 2 || unchanged.costumeIndex != 3 {
		t.Fatalf("missing properties changed sprite: %+v", unchanged)
	}

	zero, err := parseSpriteProperties(coreproject.StageShape{
		"x": 0.0, "y": 0.0, "heading": 0.0,
		"rotationStyle": "none", "visible": false,
		"size": 0.0, "costumeIndex": 0.0,
	})
	if err != nil {
		t.Fatal(err)
	}
	reset := newSprite()
	applySpriteProperties(reset, zero)
	if transform := reset.transform(); transform.x != 0 || transform.y != 0 ||
		transform.direction != 0 || transform.rotationStyle != None {
		t.Fatalf("zero properties were not applied to transform: %+v", transform)
	}
	if reset.spriteState.IsVisible || reset.runtimeState.Scale != 0 || reset.costumeIndex != 0 {
		t.Fatalf("zero properties were not applied to sprite: %+v", reset)
	}
}

func TestParseSpritePropertiesRejectsWrongTypes(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"x", "0", "float64"},
		{"y", true, "float64"},
		{"heading", nil, "float64"},
		{"rotationStyle", 0.0, "string"},
		{"visible", 0.0, "bool"},
		{"size", "small", "float64"},
		{"costumeIndex", 0, "float64"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseSpriteProperties(coreproject.StageShape{test.name: test.value})
			if err == nil || !strings.Contains(err.Error(), "want "+test.want) {
				t.Fatalf("parseSpriteProperties error = %v, want type %s", err, test.want)
			}
		})
	}
}
