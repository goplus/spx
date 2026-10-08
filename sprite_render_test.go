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
)

func newTestRenderSprite() *SpriteImpl {
	return &SpriteImpl{
		name: "TestSprite",
		baseObj: baseObj{
			costumes: []*costume{
				{name: "idle"},
				{name: "run"},
			},
			costumeIndex: 1,
		},
	}
}

func TestSpriteSetCostume(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		costume                 SpriteCostumeName
		defaultIndex, wantIndex int
		wantVersion             uint64
		wantDirty               bool
	}{
		{name: "missing name is no-op", costume: "missing", wantIndex: 1},
		{name: "valid name updates state", costume: "idle", defaultIndex: 1, wantVersion: 1, wantDirty: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite := newTestRenderSprite()
			sprite.spriteState.DefaultCostumeIndex = tt.defaultIndex

			sprite.setCostume(tt.costume)

			if got := sprite.costumeIndex; got != tt.wantIndex {
				t.Errorf("costumeIndex = %d, want %d", got, tt.wantIndex)
			}
			if got := sprite.spriteState.DefaultCostumeIndex; got != 0 {
				t.Errorf("DefaultCostumeIndex = %d, want 0", got)
			}
			if got := uint64(sprite.spriteState.DirtyVersion); got != tt.wantVersion {
				t.Errorf("DirtyVersion = %d, want %d", got, tt.wantVersion)
			}
			if got := sprite.runtimeState.IsCostumeDirty; got != tt.wantDirty {
				t.Errorf("IsCostumeDirty = %v, want %v", got, tt.wantDirty)
			}
		})
	}
}

func TestResolveCostumeIndex(t *testing.T) {
	sprite := newTestRenderSprite()

	tests := []struct {
		name    string
		costume string
		want    int
	}{
		{name: "name", costume: "run", want: 1},
		{name: "zero", costume: "0", want: -1},
		{name: "in range", costume: "2", want: 1},
		{name: "wrap", costume: "3", want: 0},
		{name: "wrap to zero", costume: "4", want: 1},
		{name: "invalid", costume: "missing", want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sprite.ResolveCostumeIndex(tt.costume); got != tt.want {
				t.Fatalf("ResolveCostumeIndex(%q) = %d, want %d", tt.costume, got, tt.want)
			}
		})
	}
}

func TestResolveCostumeIndexWithNoCostumes(t *testing.T) {
	sprite := &SpriteImpl{}

	if got := sprite.ResolveCostumeIndex("3"); got != 2 {
		t.Fatalf("ResolveCostumeIndex(%q) = %d, want %d", "3", got, 2)
	}
}

func newRenderOffsetTestSprite() *SpriteImpl {
	sprite := &SpriteImpl{g: &Game{}}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		RotationStyle: "normal",
		FAnimations:   map[SpriteAnimationName]*coreproject.AniConfig{},
		AnimBindings:  map[string]string{},
	})
	sprite.runtimeState.Scale = 1
	sprite.transform().pivot = mathf.NewVec2(3, 4)
	sprite.costumes = []*costume{{
		center:           mathf.NewVec2(10, 20),
		bitmapResolution: 1,
		width:            100,
		height:           80,
	}}
	return sprite
}

func TestGetCostumeRenderOffsetUsesPivot(t *testing.T) {
	costume := newBackdropCostume(&coreproject.BackdropConfig{
		CostumeConfig: coreproject.CostumeConfig{
			ImageWidth: 200, ImageHeight: 120, BitmapResolution: 2,
			X: 100, Y: 60,
		},
		Pivot: mathf.NewVec2(10, -5),
	})

	x, y := getCostumeRenderOffset(costume, costume.pivot, 1, 1)
	if x != -10 || y != 5 {
		t.Fatalf("getCostumeRenderOffset = (%v, %v), want (-10, 5)", x, y)
	}
}

func TestCostumeRenderAnchorConvertsAssetCoordinatesToSPX(t *testing.T) {
	costume := newCostume(&coreproject.CostumeConfig{
		ImageWidth: 201.5, ImageHeight: 121.25, BitmapResolution: 2,
		X: 80, Y: 40,
	})

	want := mathf.NewVec2(-10.375, 10.3125)
	if got := costume.renderAnchorInSPX(); got != want {
		t.Fatalf("renderAnchorInSPX = %v, want %v", got, want)
	}
	if width, height := costume.logicalSize(); width != 100.75 || height != 60.625 {
		t.Fatalf("logical size = (%v, %v), want (100.75, 60.625)", width, height)
	}
	if costume.isAtlas() {
		t.Fatal("standalone costume is marked as an atlas")
	}
}

func TestSizedCostumeUsesImageCenter(t *testing.T) {
	costume := newCostumeWithSize(7, 5)
	if width, height := costume.getSize(); width != 7 || height != 5 {
		t.Fatalf("sized costume = (%d, %d), want (7, 5)", width, height)
	}
	if got := costume.renderAnchorInSPX(); got != (mathf.Vec2{}) {
		t.Fatalf("renderAnchorInSPX = %v, want centered origin", got)
	}
	if costume.isAtlas() {
		t.Fatal("sized costume is marked as an atlas")
	}
}

func TestAtlasCostumePreservesFrameGeometry(t *testing.T) {
	c := newCostumeWith("frame1", &costumeSetImage{
		path: "atlas.png", width: 256, height: 128, nx: 2,
		rc: coreproject.CostumeSetRect{X: 16, Y: 8, W: 86, H: 43},
	}, 90, 1, 2)
	obj := baseObj{costumes: []*costume{c}}
	if !c.isAtlas() {
		t.Fatal("atlas costume is marked as a standalone image")
	}
	if got, want := obj.getCostumeAtlasRegion(), mathf.NewRect2(59, 8, 43, 43); got != want {
		t.Fatalf("atlas region = %v, want %v", got, want)
	}
	if got, want := obj.getCostumeAtlasUvRemap(), mathf.NewRect2(59.0/256, 8.0/128, 43.0/256, 43.0/128); got != want {
		t.Fatalf("atlas UV remap = %v, want %v", got, want)
	}
	if width, height := c.getSize(); width != 21 || height != 21 {
		t.Fatalf("integer size = (%d, %d), want (21, 21)", width, height)
	}
	if width, height := c.logicalSize(); width != 21.5 || height != 21.5 {
		t.Fatalf("logical size = (%v, %v), want (21.5, 21.5)", width, height)
	}
	if got := c.renderAnchorInSPX(); got != (mathf.Vec2{}) {
		t.Fatalf("renderAnchorInSPX = %v, want centered origin", got)
	}
}

func TestWorldRenderOffsetAppliesRootFlipAndRotation(t *testing.T) {
	sprite := newRenderOffsetTestSprite()
	sprite.runtimeState.Scale = 2
	sprite.transform().rotationStyle = LeftRight
	sprite.transform().direction = -30

	x, y := getWorldRenderOffset(sprite)
	if x != -74 || y != -48 {
		t.Fatalf("left-right world render offset = (%v, %v), want (-74, -48)", x, y)
	}

	sprite.runtimeState.Scale = 1
	sprite.transform().rotationStyle = Normal
	sprite.transform().direction = 45
	x, y = getWorldRenderOffset(sprite)
	wantX := 61 / math.Sqrt2
	wantY := 13 / math.Sqrt2
	if math.Abs(x-wantX) > 1e-9 || math.Abs(y-wantY) > 1e-9 {
		t.Fatalf("rotated world render offset = (%v, %v), want (%v, %v)", x, y, wantX, wantY)
	}
}

func TestLegacyLeftRightRotationStyleFlipsNegativeHeading(t *testing.T) {
	sprite := &SpriteImpl{g: &Game{}}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		Heading:       -9,
		RotationStyle: "leftRight",
		FAnimations:   map[SpriteAnimationName]*coreproject.AniConfig{},
		AnimBindings:  map[string]string{},
	})
	sprite.costumes = []*costume{{bitmapResolution: 1}}

	rotation, scaleX, scaleY := getRenderRotationAndScale(sprite)
	if rotation != 0 || scaleX != -1 || scaleY != 1 {
		t.Fatalf("legacy leftRight transform = (%v, %v, %v), want (0, -1, 1)", rotation, scaleX, scaleY)
	}
}

func TestPhysicsShapePivotSeparatesAutoAndExplicitShapes(t *testing.T) {
	sprite := newRenderOffsetTestSprite()
	sprite.runtimeState.Scale = 2

	config := physicConfig{Pivot: mathf.NewVec2(3, 4), Type: physicsColliderRect}
	if got, want := config.shapePivot(sprite), mathf.NewVec2(6, 8); got != want {
		t.Fatalf("explicit shape pivot = %v, want %v", got, want)
	}

	config.Type = physicsColliderAuto
	if got, want := config.shapePivot(sprite), mathf.NewVec2(80, -40); got != want {
		t.Fatalf("auto shape pivot = %v, want %v", got, want)
	}
}
