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
	"math/rand"
	"testing"

	"github.com/goplus/spbase/mathf"
)

func TestSpriteStepToRandomUsesRandomPositionTarget(t *testing.T) {
	sprite := newTestTransformSprite(1000, 1000)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360

	rand.Seed(1)
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

func TestFixWorldRangeLeavesOversizedSpriteCentered(t *testing.T) {
	sprite := newTestTransformSprite(0, 45)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.g.displayState.MinWorldX = -240
	sprite.g.displayState.MinWorldY = -180
	sprite.baseObj.costumes = []*costume{{
		width:            480,
		height:           360,
		bitmapResolution: 1,
		center:           mathf.NewVec2(240, 180),
	}}
	sprite.baseObj.costumeIndex = 0
	sprite.runtimeState.Scale = 3

	gotX, gotY := sprite.transform().fixWorldRange(0, 45)
	if gotX != 0 || gotY != 45 {
		t.Fatalf("fixWorldRange(0, 45) = (%v, %v), want (0, 45) for oversized sprite", gotX, gotY)
	}
}

func TestFixWorldRangeFencesSpriteLikeScratch(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.g.displayState.MinWorldX = -240
	sprite.g.displayState.MinWorldY = -180
	sprite.baseObj.costumes = []*costume{{
		width:            100,
		height:           80,
		bitmapResolution: 1,
		center:           mathf.NewVec2(50, 40),
	}}
	sprite.baseObj.costumeIndex = 0
	sprite.runtimeState.Scale = 1

	gotX, gotY := sprite.transform().fixWorldRange(500, 300)
	if gotX != 275 || gotY != 205 {
		t.Fatalf("fixWorldRange(500, 300) = (%v, %v), want (275, 205)", gotX, gotY)
	}
}

func TestFixWorldRangeFencesOversizedSpriteOnlyWhenFullyOutside(t *testing.T) {
	sprite := newTestTransformSprite(0, 45)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.g.displayState.MinWorldX = -240
	sprite.g.displayState.MinWorldY = -180
	sprite.baseObj.costumes = []*costume{{
		width:            480,
		height:           360,
		bitmapResolution: 1,
		center:           mathf.NewVec2(240, 180),
	}}
	sprite.baseObj.costumeIndex = 0
	sprite.runtimeState.Scale = 3

	gotX, gotY := sprite.transform().fixWorldRange(1000, 45)
	if gotX != 945 || gotY != 45 {
		t.Fatalf("fixWorldRange(1000, 45) = (%v, %v), want (945, 45)", gotX, gotY)
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
