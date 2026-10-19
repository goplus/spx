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
	internalengine "github.com/goplus/spx/v3/internal/engine"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
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

	setupImmediateEngineCalls(t)

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

func TestNormalizeEdgeArea(t *testing.T) {
	for _, tt := range []struct {
		area, want string
	}{
		{"", edgeAreaStage},
		{"stage", edgeAreaStage},
		{"STAGE", edgeAreaStage},
		{"camera", edgeAreaCamera},
		{"CaMeRa", edgeAreaCamera},
		{"viewport", edgeAreaCamera},
		{"VIEWPORT", edgeAreaCamera},
		{"unknown", edgeAreaStage},
	} {
		t.Run(tt.area, func(t *testing.T) {
			if got := normalizeEdgeArea(tt.area); got != tt.want {
				t.Fatalf("normalizeEdgeArea(%q) = %q, want %q", tt.area, got, tt.want)
			}
		})
	}
}

func TestSpriteTouchingTargetDefaultsToStageArea(t *testing.T) {
	installEdgePhysicsMgr(t, &fakeEdgePhysicsMgr{
		stageTouching:  touchingScreenLeft,
		cameraTouching: touchingScreenRight,
	})

	sprite := newEdgeTestSprite(0)

	if !sprite.Touching__2(EdgeLeft) {
		t.Fatal("Touching__2(EdgeLeft) = false, want true for default stage area")
	}
	if sprite.Touching__2(EdgeRight) {
		t.Fatal("Touching__2(EdgeRight) = true, want false when only camera boundary is touched")
	}
	if sprite.checkTouchingScreen(touchingScreenRight, edgeAreaViewport) == 0 {
		t.Fatal("checkTouchingScreen(EdgeRight, viewport) = 0, want non-zero")
	}
}

func TestSpriteBounceOffEdgeDefaultsToStageArea(t *testing.T) {
	for _, tt := range []struct {
		name          string
		stage, camera int64
		from, want    Direction
	}{
		{"left", touchingScreenLeft, touchingScreenRight, -90, 90},
		{"right", touchingScreenRight, touchingScreenLeft, 90, -90},
		{"top", touchingScreenTop, touchingScreenBottom, 0, 180},
		{"bottom", touchingScreenBottom, touchingScreenTop, 180, 0},
		{"top diagonal", touchingScreenTop, touchingScreenBottom, 45, 135},
		{"bottom diagonal", touchingScreenBottom, touchingScreenTop, 135, 45},
	} {
		t.Run(tt.name, func(t *testing.T) {
			installEdgePhysicsMgr(t, &fakeEdgePhysicsMgr{
				stageNearest:  tt.stage,
				cameraNearest: tt.camera,
			})
			sprite := newEdgeTestSprite(tt.from)
			version := sprite.spriteState.DirtyVersion

			sprite.BounceOffEdge()

			if got := sprite.Heading(); math.IsNaN(got) || math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("BounceOffEdge() heading = %v, want %v", got, tt.want)
			}
			if sprite.spriteState.DirtyVersion == version {
				t.Fatal("bounce did not publish the transform change")
			}
		})
	}
}

func TestSpriteBounceOffEdgeWithCameraArea(t *testing.T) {
	installEdgePhysicsMgr(t, &fakeEdgePhysicsMgr{
		stageNearest:  touchingScreenLeft,
		cameraNearest: touchingScreenRight,
	})

	sprite := newEdgeTestSprite(-90)

	sprite.transform().bounceOffEdge(edgeAreaCamera)

	if got := sprite.Heading(); got != -90 {
		t.Fatalf("BounceOffEdge(camera) heading = %v, want -90", got)
	}
}
