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
	internalengine "github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

type fakeEdgePhysicsMgr struct {
	stageTouching  int64
	cameraTouching int64
	stageNearest   int64
	cameraNearest  int64
}

func (f *fakeEdgePhysicsMgr) Raycast(from, to mathf.Vec2, collisionMask int64) pkgengine.Object {
	return 0
}

func (f *fakeEdgePhysicsMgr) CheckCollision(from, to mathf.Vec2, collisionMask int64, collideWithAreas, collideWithBodies bool) bool {
	return false
}

func (f *fakeEdgePhysicsMgr) CheckTouchedCameraBoundaries(obj pkgengine.Object) int64 {
	return f.cameraTouching
}

func (f *fakeEdgePhysicsMgr) CheckTouchedCameraBoundary(obj pkgengine.Object, boardType int64) bool {
	return f.cameraTouching&boardType != 0
}

func (f *fakeEdgePhysicsMgr) CheckNearestTouchedCameraBoundary(obj pkgengine.Object) int64 {
	return f.cameraNearest
}

func (f *fakeEdgePhysicsMgr) CheckTouchedStageBoundaries(obj pkgengine.Object) int64 {
	return f.stageTouching
}

func (f *fakeEdgePhysicsMgr) CheckTouchedStageBoundary(obj pkgengine.Object, boardType int64) bool {
	return f.stageTouching&boardType != 0
}

func (f *fakeEdgePhysicsMgr) CheckNearestTouchedStageBoundary(obj pkgengine.Object) int64 {
	return f.stageNearest
}

func (f *fakeEdgePhysicsMgr) SetCollisionSystemType(bool) {}

func (f *fakeEdgePhysicsMgr) SetGlobalGravity(float64) {}

func (f *fakeEdgePhysicsMgr) GetGlobalGravity() float64 {
	return 0
}

func (f *fakeEdgePhysicsMgr) SetGlobalFriction(float64) {}

func (f *fakeEdgePhysicsMgr) GetGlobalFriction() float64 {
	return 0
}

func (f *fakeEdgePhysicsMgr) SetGlobalAirDrag(float64) {}

func (f *fakeEdgePhysicsMgr) GetGlobalAirDrag() float64 {
	return 0
}

func (f *fakeEdgePhysicsMgr) CheckCollisionRect(pos, size mathf.Vec2, collisionMask int64) pkgengine.Array {
	return nil
}

func (f *fakeEdgePhysicsMgr) CheckCollisionCircle(pos mathf.Vec2, radius float64, collisionMask int64) pkgengine.Array {
	return nil
}

func (f *fakeEdgePhysicsMgr) RaycastWithDetails(from, to mathf.Vec2, ignoreSprites pkgengine.Array, collisionMask int64, collideWithAreas, collideWithBodies bool) pkgengine.Array {
	return nil
}

func installEdgePhysicsMgr(t *testing.T, mgr pkgengine.IPhysicsMgr) {
	t.Helper()

	enginewrap.Init(func(call func()) {
		call()
	})

	original := pkgengine.PhysicsMgr
	pkgengine.PhysicsMgr = mgr
	t.Cleanup(func() {
		pkgengine.PhysicsMgr = original
	})
}

func newEdgeTestSprite(direction Direction) *SpriteImpl {
	sprite := newTestTransformSprite(0, 0)
	sprite.spriteState.IsVisible = true
	sprite.transform().direction = direction
	sprite.runtimeState.SyncSprite = &internalengine.Sprite{}
	sprite.runtimeState.SyncSprite.SetId(42)
	return sprite
}

func TestSpriteTouchingTargetUsesRequestedEdgeArea(t *testing.T) {
	installEdgePhysicsMgr(t, &fakeEdgePhysicsMgr{
		stageTouching:  touchingScreenLeft,
		cameraTouching: touchingScreenRight,
	})

	sprite := newEdgeTestSprite(0)

	if !sprite.Touching__2(EdgeLeft) {
		t.Fatal("Touching__2(EdgeLeft) = false, want true for default stage area")
	}
	if sprite.touching(EdgeLeft, edgeAreaCamera) {
		t.Fatal("touching(EdgeLeft, camera) = true, want false")
	}
	if !sprite.touching(EdgeRight, edgeAreaViewport) {
		t.Fatal("touching(EdgeRight, viewport) = false, want true")
	}
	if !sprite.touching(EdgeRight, "Camera") {
		t.Fatal("touching(EdgeRight, Camera) = false, want true")
	}
}

func TestSpriteBounceOffEdgeDefaultsToStageArea(t *testing.T) {
	installEdgePhysicsMgr(t, &fakeEdgePhysicsMgr{
		stageNearest:  touchingScreenLeft,
		cameraNearest: touchingScreenRight,
	})

	sprite := newEdgeTestSprite(-90)

	sprite.BounceOffEdge()

	if got := sprite.Heading(); got != 90 {
		t.Fatalf("BounceOffEdge() heading = %v, want 90", got)
	}
}

func TestSpriteBounceOffEdgeWithCameraArea(t *testing.T) {
	installEdgePhysicsMgr(t, &fakeEdgePhysicsMgr{
		stageNearest:  touchingScreenLeft,
		cameraNearest: touchingScreenRight,
	})

	sprite := newEdgeTestSprite(-90)

	sprite.transform().BounceOffEdge(edgeAreaCamera)

	if got := sprite.Heading(); got != -90 {
		t.Fatalf("BounceOffEdge(camera) heading = %v, want -90", got)
	}
}
