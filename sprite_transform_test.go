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

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
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

func TestMotionInputsTreatNaNAsZero(t *testing.T) {
	tests := []struct {
		name         string
		apply        func(*SpriteImpl)
		wantX, wantY float64
	}{
		{
			name:  "set position",
			apply: func(sprite *SpriteImpl) { sprite.SetXYpos(math.NaN(), math.NaN()) },
			wantX: 0,
			wantY: 0,
		},
		{
			name:  "set x",
			apply: func(sprite *SpriteImpl) { sprite.SetXpos(math.NaN()) },
			wantX: 0,
			wantY: -8,
		},
		{
			name:  "set y",
			apply: func(sprite *SpriteImpl) { sprite.SetYpos(math.NaN()) },
			wantX: 12,
			wantY: 0,
		},
		{
			name:  "change position",
			apply: func(sprite *SpriteImpl) { sprite.ChangeXYpos(math.NaN(), math.NaN()) },
			wantX: 12,
			wantY: -8,
		},
		{
			name:  "change x",
			apply: func(sprite *SpriteImpl) { sprite.ChangeXpos(math.NaN()) },
			wantX: 12,
			wantY: -8,
		},
		{
			name:  "change y",
			apply: func(sprite *SpriteImpl) { sprite.ChangeYpos(math.NaN()) },
			wantX: 12,
			wantY: -8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sprite := newTestTransformSprite(12, -8)
			tt.apply(sprite)

			if gotX, gotY := sprite.getXY(); gotX != tt.wantX || gotY != tt.wantY {
				t.Fatalf("position = (%v, %v), want (%v, %v)", gotX, gotY, tt.wantX, tt.wantY)
			}
		})
	}
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

func TestMotionOptions(t *testing.T) {
	tests := []struct {
		name          string
		opts          *MotionOptions
		wantSpeed     Speed
		wantAnimation SpriteAnimationName
	}{
		{
			name:      "nil uses defaults",
			opts:      nil,
			wantSpeed: 1,
		},
		{
			name: "zero speed keeps default and animation",
			opts: &MotionOptions{
				Animation: "walk",
			},
			wantSpeed:     1,
			wantAnimation: "walk",
		},
		{
			name: "positive speed overrides default",
			opts: &MotionOptions{
				Speed:     2.5,
				Animation: "run",
			},
			wantSpeed:     2.5,
			wantAnimation: "run",
		},
		{
			name: "negative speed falls back to default",
			opts: &MotionOptions{
				Speed:     -3,
				Animation: "run",
			},
			wantSpeed:     1,
			wantAnimation: "run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpeed, gotAnimation := motionOptions(tt.opts)
			if gotSpeed != tt.wantSpeed {
				t.Fatalf("speed = %v, want %v", gotSpeed, tt.wantSpeed)
			}
			if gotAnimation != tt.wantAnimation {
				t.Fatalf("animation = %q, want %q", gotAnimation, tt.wantAnimation)
			}
		})
	}
}

func TestSpriteStepToRandomUsesRandomPositionTarget(t *testing.T) {
	sprite := newTestTransformSprite(1000, 1000)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360

	SetRandomSeed(1)
	defer ResetRandomSeed()
	sprite.StepTo__c(Random)

	gotX, gotY := sprite.getXY()
	if gotX == 1000 && gotY == 1000 {
		t.Fatal("StepTo__c(Random) did not move the sprite")
	}

	minX := float64(-(sprite.g.displayState.WorldWidth >> 1))
	maxX := float64(sprite.g.displayState.WorldWidth - (sprite.g.displayState.WorldWidth >> 1) - 1)
	minY := float64((sprite.g.displayState.WorldHeight >> 1) - (sprite.g.displayState.WorldHeight - 1))
	maxY := float64(sprite.g.displayState.WorldHeight >> 1)
	if gotX < minX || gotX > maxX || gotY < minY || gotY > maxY {
		t.Fatalf(
			"StepTo__c(Random) moved to (%v, %v), want x in [%v, %v], y in [%v, %v]",
			gotX,
			gotY,
			minX,
			maxX,
			minY,
			maxY,
		)
	}
}

func TestFixWorldRange(t *testing.T) {
	tests := []struct {
		name           string
		width, height  int
		scale          float64
		heading        Direction
		startX, startY float64
		x, y           float64
		wantX, wantY   float64
	}{
		{
			name:  "oversized stays centered",
			width: 480, height: 360, scale: 3,
			startY: 45, y: 45,
			wantY: 45,
		},
		{
			name:  "normal sprite is fenced",
			width: 100, height: 80, scale: 1, heading: 90,
			x: 500, y: 300,
			wantX: 275, wantY: 205,
		},
		{
			name:  "oversized is fenced when fully outside",
			width: 480, height: 360, scale: 3,
			startY: 45, x: 1000, y: 45, heading: 90,
			wantX: 945, wantY: 45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sprite := newTestTransformSprite(tt.startX, tt.startY)
			sprite.g.displayState.WorldWidth = 480
			sprite.g.displayState.WorldHeight = 360
			sprite.g.displayState.MinWorldX = -240
			sprite.g.displayState.MinWorldY = -180
			sprite.costumes = []*costume{{
				width:            tt.width,
				height:           tt.height,
				bitmapResolution: 1,
				center:           mathf.NewVec2(float64(tt.width)/2, float64(tt.height)/2),
			}}
			sprite.costumeIndex = 0
			sprite.runtimeState.Scale = tt.scale
			sprite.transform().direction = tt.heading

			x, y := sprite.transform().fixWorldRange(tt.x, tt.y)
			if x != tt.wantX || y != tt.wantY {
				t.Fatalf("fixWorldRange(%v, %v) = (%v, %v), want (%v, %v)", tt.x, tt.y, x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestFixWorldRangeUsesRenderedCostumeBoundsInsteadOfAutoTrigger(t *testing.T) {
	sprite := newTestTransformSprite(-186, 0)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.g.displayState.MinWorldX = -240
	sprite.g.displayState.MinWorldY = -180
	sprite.baseObj.costumes = []*costume{{
		width:            55,
		height:           54,
		bitmapResolution: 2,
		center:           mathf.NewVec2(52, 89),
	}}
	sprite.baseObj.costumeIndex = 0
	sprite.runtimeState.Scale = 1
	sprite.runtimeState.SyncSprite = &engine.Sprite{}
	sprite.transform().direction = 90
	sprite.physics().triggerInfo.Type = physicsColliderAuto
	sprite.physics().triggerInfo.Pivot = mathf.NewVec2(0.25, 0)
	sprite.physics().triggerInfo.Params = []float64{29.5, 29}

	gotX, gotY := sprite.transform().fixWorldRange(1000, 0)
	if gotX != 253 || gotY != 0 {
		t.Fatalf("fixWorldRange(1000, 0) = (%v, %v), want (253, 0)", gotX, gotY)
	}
}

func TestFenceBoundsIncludesCostumeRotation(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	sprite.baseObj.costumes = []*costume{{
		width:            100,
		height:           40,
		bitmapResolution: 1,
		center:           mathf.NewVec2(50, 20),
	}}
	sprite.baseObj.costumeIndex = 0
	sprite.runtimeState.Scale = 1
	sprite.transform().direction = 0

	got := sprite.fenceBounds()
	if got == nil {
		t.Fatal("fenceBounds() = nil")
	}
	if math.Abs(got.Position.X+20) > 1e-9 || math.Abs(got.Position.Y+50) > 1e-9 ||
		math.Abs(got.Size.X-40) > 1e-9 || math.Abs(got.Size.Y-100) > 1e-9 {
		t.Fatalf("fenceBounds() = %+v, want position (-20, -50), size (40, 100)", *got)
	}
}

func TestClampSpriteScaleToMaxScale(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.baseObj.costumes = []*costume{{
		width:            480,
		height:           360,
		bitmapResolution: 1,
		center:           mathf.NewVec2(240, 180),
	}}
	sprite.baseObj.costumeIndex = 0

	got := sprite.transform().clampSpriteScale(3)

	if got != 1.5 {
		t.Fatalf("clampSpriteScale(3) = %v, want 1.5", got)
	}
}

func TestFractionalCostumeFenceUsesLogicalGeometry(t *testing.T) {
	sprite := newTestTransformSprite(-393, 5)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.g.displayState.MinWorldX = -240
	sprite.g.displayState.MinWorldY = -180
	sprite.baseObj.costumes = []*costume{newCostume(&coreproject.CostumeConfig{
		ImageWidth: 247.24325, ImageHeight: 476.42829,
		X: 76.0323911558041, Y: 208.79720845345383,
		BitmapResolution: 1,
	})}
	sprite.baseObj.costumeIndex = 0
	sprite.runtimeState.Scale = 1
	sprite.transform().direction = 90
	sprite.transform().changeX(-5)
	if got := sprite.Xpos(); got != -396 {
		t.Fatalf("fenced X = %v, want -396", got)
	}
}
