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
	"math"
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

func newTestTransformSprite(x, y float64) *SpriteImpl {
	sprite := &SpriteImpl{
		g:    &Game{},
		name: "TestSprite",
	}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		X:             x,
		Y:             y,
		RotationStyle: "normal",
		FAnimations:   map[SpriteAnimationName]*coreproject.AniConfig{},
		AnimBindings:  map[string]string{},
	})
	return sprite
}

func TestSpriteDirectionToPosNormalizesAngle(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)

	tests := []struct {
		name string
		x    float64
		y    float64
		want Direction
	}{
		{name: "right", x: 10, y: 0, want: 90},
		{name: "up", x: 0, y: 10, want: 0},
		{name: "left", x: -10, y: 0, want: -90},
		{name: "down", x: 0, y: -10, want: 180},
		{name: "bottom left stays normalized", x: -10, y: -10, want: -135},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sprite.DirectionTo__4(tt.x, tt.y); math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("DirectionTo__4(%v, %v) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestSpriteTurnToXYposFacesCoordinateTarget(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)

	sprite.TurnToXYpos(-10, -10, nil)

	if got := sprite.Heading(); got != -135 {
		t.Fatalf("Heading() = %v, want -135", got)
	}
}
