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
	"testing"

	"github.com/goplus/spbase/mathf"
	internalengine "github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type touchingSyncSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	positions               map[pkgengine.Object]mathf.Vec2
	texturePaths            map[pkgengine.Object]string
	expectedTexturePaths    map[pkgengine.Object]string
	checkedTexturePaths     map[pkgengine.Object]string
	setTransformCalls       map[pkgengine.Object]int
	setPositionCalls        map[pkgengine.Object]int
	setRotationCalls        map[pkgengine.Object]int
	setScaleCalls           map[pkgengine.Object]int
	setPivotCalls           map[pkgengine.Object]int
	setVisibleCalls         map[pkgengine.Object]int
	setTextureCalls         map[pkgengine.Object]int
	setTextureAtlasCalls    map[pkgengine.Object]int
	setRenderScaleCalls     map[pkgengine.Object]int
	setTextureDirectCalls   map[pkgengine.Object]int
	setTextureAtlasPathByID map[pkgengine.Object]string
}

func newTouchingSyncSpriteMgr() *touchingSyncSpriteMgr {
	return &touchingSyncSpriteMgr{
		positions:               map[pkgengine.Object]mathf.Vec2{},
		texturePaths:            map[pkgengine.Object]string{},
		expectedTexturePaths:    map[pkgengine.Object]string{},
		checkedTexturePaths:     map[pkgengine.Object]string{},
		setTransformCalls:       map[pkgengine.Object]int{},
		setPositionCalls:        map[pkgengine.Object]int{},
		setRotationCalls:        map[pkgengine.Object]int{},
		setScaleCalls:           map[pkgengine.Object]int{},
		setPivotCalls:           map[pkgengine.Object]int{},
		setVisibleCalls:         map[pkgengine.Object]int{},
		setTextureCalls:         map[pkgengine.Object]int{},
		setTextureAtlasCalls:    map[pkgengine.Object]int{},
		setRenderScaleCalls:     map[pkgengine.Object]int{},
		setTextureDirectCalls:   map[pkgengine.Object]int{},
		setTextureAtlasPathByID: map[pkgengine.Object]string{},
	}
}

func (s *touchingSyncSpriteMgr) SetTransform(
	obj pkgengine.Object,
	pos mathf.Vec2,
	rot float64,
	scale mathf.Vec2,
	visible bool,
	pivot mathf.Vec2,
) {
	s.positions[obj] = pos
	s.setTransformCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetPosition(obj pkgengine.Object, pos mathf.Vec2) {
	s.positions[obj] = pos
	s.setPositionCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetRotation(obj pkgengine.Object, rot float64) {
	s.setRotationCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetScale(obj pkgengine.Object, scale mathf.Vec2) {
	s.setScaleCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetPivot(obj pkgengine.Object, pivot mathf.Vec2) {
	s.setPivotCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetVisible(obj pkgengine.Object, visible bool) {
	s.setVisibleCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetTexture(obj pkgengine.Object, path string) {
	s.texturePaths[obj] = path
	s.setTextureCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetTextureAtlas(obj pkgengine.Object, path string, rect2 mathf.Rect2) {
	s.texturePaths[obj] = path
	s.setTextureAtlasPathByID[obj] = path
	s.setTextureAtlasCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetTextureDirect(obj pkgengine.Object, path string) {
	s.texturePaths[obj] = path
	s.setTextureDirectCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetRenderScale(obj pkgengine.Object, scale mathf.Vec2) {
	s.setRenderScaleCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetMaterialShader(obj pkgengine.Object, path string) {}

func (s *touchingSyncSpriteMgr) SetMaterialParamsVec4(obj pkgengine.Object, effect string, vec4 mathf.Vec4) {
}

func (s *touchingSyncSpriteMgr) CheckCollisionWithSprite(obj, objB pkgengine.Object, alphaThreshold float64, usePixelPerfect bool) bool {
	return s.positions[obj] == s.positions[objB]
}

func (s *touchingSyncSpriteMgr) CheckCollisionByColor(obj pkgengine.Object, color mathf.Color, colorThreshold, alphaThreshold float64) bool {
	s.checkedTexturePaths[obj] = s.texturePaths[obj]
	expected, ok := s.expectedTexturePaths[obj]
	return !ok || s.texturePaths[obj] == expected
}

func installTouchingSyncSpriteMgr(t *testing.T, mgr *touchingSyncSpriteMgr) {
	t.Helper()

	enginewrap.Init(func(call func()) {
		call()
	})

	original := pkgengine.SpriteMgr
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() {
		pkgengine.SpriteMgr = original
	})
}

func newTouchingTestSprite(name string, x, y float64, id pkgengine.Object) *SpriteImpl {
	sprite := newTestTransformSprite(x, y)
	sprite.name = name
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.baseObj.initWithSize(1, 1)
	sprite.runtimeState.Scale = 1
	sprite.spriteState.IsVisible = true
	sprite.runtimeState.SyncSprite = &internalengine.Sprite{}
	sprite.runtimeState.SyncSprite.SetId(id)
	return sprite
}

func TestTouchingSpriteSyncsDirtyTransformImmediately(t *testing.T) {
	mgr := newTouchingSyncSpriteMgr()
	installTouchingSyncSpriteMgr(t, mgr)

	mover := newTouchingTestSprite("mover", 0, 0, 1)
	target := newTouchingTestSprite("target", 10, 0, 2)

	mgr.positions[1] = mathf.NewVec2(0, 0)
	mgr.positions[2] = mathf.NewVec2(10, 0)

	mover.transform().direction = 90
	mover.Step__0(10)

	if !mover.touchingSprite(target) {
		t.Fatal("touchingSprite() = false, want true after immediate step sync")
	}
	if got := mgr.setTransformCalls[1]; got != 1 {
		t.Fatalf("SetTransform calls after first touching = %d, want 1", got)
	}
	if got := mgr.setPositionCalls[1]; got != 0 {
		t.Fatalf("SetPosition calls after first touching = %d, want 0", got)
	}
	if got := mgr.setRotationCalls[1]; got != 0 {
		t.Fatalf("SetRotation calls after first touching = %d, want 0", got)
	}
	if got := mgr.setScaleCalls[1]; got != 0 {
		t.Fatalf("SetScale calls after first touching = %d, want 0", got)
	}
	if got := mgr.setPivotCalls[1]; got != 0 {
		t.Fatalf("SetPivot calls after first touching = %d, want 0", got)
	}
	if got := mgr.setVisibleCalls[1]; got != 0 {
		t.Fatalf("SetVisible calls after first touching = %d, want 0", got)
	}

	if !mover.touchingSprite(target) {
		t.Fatal("second touchingSprite() = false, want cached proxy sync to remain valid")
	}
	if got := mgr.setTransformCalls[1]; got != 1 {
		t.Fatalf("SetTransform calls after second touching = %d, want 1", got)
	}

	buffer := internalengine.NewSpriteSyncBuffer(1)
	mover.collectProxyUpdate(buffer)
	if got := buffer.UpdateCount(); got != 0 {
		t.Fatalf("collectProxyUpdate() batched %d updates, want 0 after immediate query sync", got)
	}
	if mover.spriteState.IsDirty {
		t.Fatal("spriteState.IsDirty = true, want false after frame-end collection")
	}
}

func TestTouchingColorSyncsDirtyCostumeImmediately(t *testing.T) {
	mgr := newTouchingSyncSpriteMgr()
	installTouchingSyncSpriteMgr(t, mgr)

	sprite := newTouchingTestSprite("mover", 0, 0, 1)
	sprite.costumes = []*costume{
		{name: "idle", path: "idle.png", bitmapResolution: 1, width: 1, height: 1, setIndex: -1},
		{name: "hit", path: "hit.png", bitmapResolution: 1, width: 1, height: 1, setIndex: -1},
	}
	sprite.costumeIndex = 0
	sprite.runtimeState.IsCostumeDirty = false
	sprite.runtimeState.IsAnimating = false

	mgr.texturePaths[1] = internalengine.ToAssetPath("idle.png")
	mgr.expectedTexturePaths[1] = internalengine.ToAssetPath("hit.png")

	sprite.SetCostume__0("hit")

	if !sprite.touchingColor(mathf.Color{}) {
		t.Fatal("touchingColor() = false, want true after immediate costume sync")
	}
	if got := mgr.setTextureCalls[1]; got != 1 {
		t.Fatalf("SetTexture calls after first touchingColor = %d, want 1", got)
	}
	if got := mgr.checkedTexturePaths[1]; got != internalengine.ToAssetPath("hit.png") {
		t.Fatalf("collision used texture %q, want %q", got, internalengine.ToAssetPath("hit.png"))
	}
	if sprite.runtimeState.IsCostumeDirty {
		t.Fatal("runtimeState.IsCostumeDirty = true, want false after immediate query sync")
	}

	if !sprite.touchingColor(mathf.Color{}) {
		t.Fatal("second touchingColor() = false, want synced costume to remain valid")
	}
	if got := mgr.setTextureCalls[1]; got != 1 {
		t.Fatalf("SetTexture calls after second touchingColor = %d, want 1", got)
	}
}
