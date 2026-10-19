/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"maps"
	"testing"

	"github.com/goplus/spbase/mathf"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type cameraTestMgr struct {
	pkgengine.ICameraMgr
	position, zoom mathf.Vec2
	viewport       mathf.Rect2
	limits         map[int64]int64
	smoothing      bool
	positionWrites int
}

func (m *cameraTestMgr) GetCameraPosition() mathf.Vec2 { return m.position }
func (m *cameraTestMgr) SetCameraPosition(pos mathf.Vec2) {
	m.position = pos
	m.positionWrites++
}
func (m *cameraTestMgr) GetCameraZoom() mathf.Vec2        { return m.zoom }
func (m *cameraTestMgr) SetCameraZoom(zoom mathf.Vec2)    { m.zoom = zoom }
func (m *cameraTestMgr) GetGlobalCameraRect() mathf.Rect2 { return m.viewport }
func (m *cameraTestMgr) SetCameraLimit(side, value int64) { m.limits[side] = value }
func (m *cameraTestMgr) SetCameraSmoothing(enabled bool)  { m.smoothing = enabled }

func setupCameraTestMgr(t *testing.T) *cameraTestMgr {
	t.Helper()
	setupImmediateEngineCalls(t)
	previous := pkgengine.CameraMgr
	mgr := &cameraTestMgr{limits: make(map[int64]int64)}
	pkgengine.CameraMgr = mgr
	t.Cleanup(func() { pkgengine.CameraMgr = previous })
	return mgr
}

func TestCameraCoordinatesAndWindowScale(t *testing.T) {
	mgr := setupCameraTestMgr(t)
	game := &Game{}
	game.displayState.WindowScale = 2.5
	camera := &cameraImpl{g: game, followTarget: Mouse}

	camera.SetZoom(1.5)
	if mgr.zoom != mathf.NewVec2(3.75, 3.75) || camera.Zoom() != 1.5 {
		t.Fatalf("zoom = %v, logical zoom = %v; want (3.75, 3.75) and 1.5", mgr.zoom, camera.Zoom())
	}
	camera.SetXYpos(12, -8)
	if camera.Xpos() != 12 || camera.Ypos() != -8 {
		t.Fatalf("camera position = (%v, %v), want (12, -8)", camera.Xpos(), camera.Ypos())
	}
	camera.ChangeXYpos(-2, 3)
	if mgr.position != mathf.NewVec2(10, -5) || camera.followTarget != nil {
		t.Fatalf("manual movement = %v, follow = %v; want (10, -5) and no follow target", mgr.position, camera.followTarget)
	}
	if !camera.isDirty || camera.dirtyVersion != 3 {
		t.Fatalf("camera invalidation = (%v, %d), want (true, 3)", camera.isDirty, camera.dirtyVersion)
	}
	mgr.viewport = mathf.Rect2{Position: mathf.NewVec2(-90, 25), Size: mathf.NewVec2(320, 180)}
	x, y, width, height := camera.ViewportRect()
	if x != -90 || y != 25 || width != 320 || height != 180 {
		t.Fatalf("viewport = (%v, %v, %v, %v), want (-90, 25, 320, 180)", x, y, width, height)
	}
}

func TestCameraLimitsRespectOffsetWorldCoordinates(t *testing.T) {
	mgr := setupCameraTestMgr(t)
	game := &Game{}
	game.displayState.WindowScale = 2
	game.displayState.WorldWidth, game.displayState.WorldHeight = 480, 360
	game.displayState.MinWorldX, game.displayState.MinWorldY = -120, -70
	camera := &cameraImpl{}
	camera.init(game)
	want := map[int64]int64{
		int64(sideLeft): -120, int64(sideRight): 360,
		int64(sideTop): -290, int64(sideBottom): 70,
	}
	if !maps.Equal(mgr.limits, want) || !mgr.smoothing {
		t.Fatalf("camera limits = %v, smoothing = %v; want %v and true", mgr.limits, mgr.smoothing, want)
	}
	if camera.Zoom() != 1 {
		t.Fatalf("initial logical zoom = %v, want 1", camera.Zoom())
	}
}

func TestCameraFollowUpdatesOnlyWhenTargetChanges(t *testing.T) {
	mgr := setupCameraTestMgr(t)
	sprite := newTestTransformSprite(12, -8)
	camera := &cameraImpl{g: sprite.g}
	camera.follow(sprite)
	camera.onUpdate()
	if mgr.position != mathf.NewVec2(12, -8) || mgr.positionWrites != 1 {
		t.Fatalf("first follow update = %v, writes = %d", mgr.position, mgr.positionWrites)
	}
	camera.setDirtyFlag(false)
	camera.onUpdate()
	if mgr.positionWrites != 1 {
		t.Fatal("unchanged sprite caused another camera update")
	}
	sprite.SetXYpos(21, 34)
	camera.onUpdate()
	if mgr.position != mathf.NewVec2(21, 34) || mgr.positionWrites != 2 {
		t.Fatalf("changed follow update = %v, writes = %d", mgr.position, mgr.positionWrites)
	}
	if camera.observedFollowVersion != sprite.spriteState.VisualVersion {
		t.Fatal("camera did not acknowledge the target's visual version")
	}
	camera.follow(nil)
	camera.onUpdate()
	if mgr.positionWrites != 2 {
		t.Fatal("detaching the follow target moved the camera")
	}
}

func TestCameraFollowResolvesTargetsWithoutReplacingOnMissingName(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "target")
	camera := &cameraImpl{g: game}
	camera.Follow__0(sprite)
	if camera.followTarget != &sprite.SpriteImpl {
		t.Fatal("follow did not resolve a generated sprite to its implementation")
	}
	camera.Follow__1("target")
	version := camera.dirtyVersion
	for _, target := range []any{SpriteName("missing"), Edge} {
		camera.follow(target)
		if camera.followTarget != &sprite.SpriteImpl || camera.dirtyVersion != version {
			t.Fatalf("unsupported target %v changed the active follow target", target)
		}
	}
	camera.follow(Mouse)
	game.inputMgr.setMousePos(mathf.NewVec2(45, 67))
	if changed, pos := camera.getFollowPos(); !changed || pos != mathf.NewVec2(45, 67) {
		t.Fatalf("mouse follow = (%v, %v), want (true, (45, 67))", changed, pos)
	}
}

func TestCameraFollowRejectsUnsupportedType(t *testing.T) {
	camera := &cameraImpl{followTarget: Mouse}
	defer func() {
		if recover() == nil {
			t.Fatal("unsupported follow target did not panic")
		}
		if camera.followTarget != Mouse || camera.dirtyVersion != 0 {
			t.Fatal("invalid target changed camera state before failing")
		}
	}()
	camera.follow(42)
}

func TestCameraWithoutFollowTargetHasNoPositionUpdate(t *testing.T) {
	camera := &cameraImpl{}
	if changed, position := camera.getFollowPos(); changed || position != (mathf.Vec2{}) {
		t.Fatalf("detached camera follow position = (%v, %v), want false and origin", changed, position)
	}
}
