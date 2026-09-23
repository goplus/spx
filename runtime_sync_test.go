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
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type pullPositionSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	calls     int
	ids       []int64
	positions []float32
}

func (m *pullPositionSpriteMgr) BatchRetrievePositions(ids []int64, out []float32) bool {
	m.calls++
	m.ids = append(m.ids[:0], ids...)
	copy(out, m.positions)
	return true
}

func setupPullPositionSpriteMgr(t *testing.T) *pullPositionSpriteMgr {
	t.Helper()
	enginewrap.Init(func(call func()) { call() })
	original := pkgengine.SpriteMgr
	mgr := &pullPositionSpriteMgr{}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() { pkgengine.SpriteMgr = original })
	return mgr
}

func newPullPhysicsSprite(id int64, mode PhysicsMode, x, y float64) *SpriteImpl {
	sprite := newPhysicsPositionTestSprite(x, y)
	sprite.runtimeState.SyncSprite = &engine.Sprite{}
	sprite.runtimeState.SyncSprite.Id = id
	sprite.components.physics = &physicsComponent{sprite: sprite, physicsMode: mode}
	return sprite
}

func setupSyncBufferCapture(t *testing.T) (*Game, *captureFlushSpriteMgr) {
	t.Helper()
	mgr := setupCaptureFlushSpriteMgr(t)
	return &Game{syncBuffer: engine.NewSpriteSyncBuffer(1)}, mgr
}

func TestFlushSyncBufferOnlySubmitsChanges(t *testing.T) {
	game, mgr := setupSyncBufferCapture(t)
	game.flushSyncBuffer()
	if len(mgr.batches) != 0 {
		t.Fatalf("empty buffer submitted %d batches, want 0", len(mgr.batches))
	}

	game.syncBuffer.Add(1, 2, 3, 4, 5, 6, 7, 8, true)
	game.flushSyncBuffer()
	if len(mgr.batches) != 1 {
		t.Fatalf("updated buffer submitted %d batches, want 1", len(mgr.batches))
	}

	game.syncBuffer.Clear()
	game.syncBuffer.AddDelete(1)
	game.flushSyncBuffer()
	if len(mgr.batches) != 2 {
		t.Fatalf("delete buffer submitted %d total batches, want 2", len(mgr.batches))
	}
}

func TestFlushSyncBufferDoesNotSubmitSerializationFailure(t *testing.T) {
	game, mgr := setupSyncBufferCapture(t)
	invalidID := int64(1<<24 + 1)
	game.syncBuffer.Add(invalidID, 0, 0, 0, 0, 0, 0, 0, false)

	defer func() {
		if recover() == nil {
			t.Fatal("invalid sprite ID did not panic during serialization")
		}
		if len(mgr.batches) != 0 {
			t.Fatalf("serialization failure submitted %d batches, want 0", len(mgr.batches))
		}
	}()
	game.flushSyncBuffer()
}

func TestProxyTransformQueryAndBatchMatch(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	sprite.spriteState.IsVisible = true
	sprite.transform().direction = 45
	sprite.transform().rotationStyle = Normal

	sprite.ensureProxyQueryStateSynced()
	if got, want := spy.spriteMgr.position, mathf.NewVec2(50, 60); got != want {
		t.Fatalf("query position = %v, want %v", got, want)
	}
	if got, want := spy.spriteMgr.rotation, engine.DegToRad(-45); got != want {
		t.Fatalf("query rotation = %v, want %v", got, want)
	}
	if got, want := spy.spriteMgr.scale, mathf.NewVec2(1, 1); got != want {
		t.Fatalf("query scale = %v, want %v", got, want)
	}
	if got, want := spy.spriteMgr.renderOffset, mathf.NewVec2(37, -24); got != want {
		t.Fatalf("query offset = %v, want %v", got, want)
	}
	if !spy.spriteMgr.visible {
		t.Fatal("query transform hid a visible sprite")
	}

	buffer := engine.NewSpriteSyncBuffer(1)
	sprite.collectProxyUpdate(buffer)
	if buffer.UpdateCount() != 0 {
		t.Fatal("batch repeated an already synchronized transform")
	}
	sprite.markProxyDirty()
	sprite.collectProxyUpdate(buffer)
	want := []float32{1, 0, 101, 50, 60, float32(spy.spriteMgr.rotation), 1, 1, 37, -24, 1}
	if got := buffer.Serialize(); !slices.Equal(got, want) {
		t.Fatalf("batch transform = %v, want %v", got, want)
	}
}

func newPhysicsPositionTestSprite(x, y float64) *SpriteImpl {
	sprite := &SpriteImpl{}
	sprite.components.transform = &transformComponent{
		sprite: sprite,
		x:      x,
		y:      y,
	}
	return sprite
}

func TestPullPhysicsPositionsFiltersAndKeepsIDOrder(t *testing.T) {
	mgr := setupPullPositionSpriteMgr(t)
	mgr.positions = []float32{11, 12, 31, 32}
	first := newPullPhysicsSprite(1, KinematicPhysics, 1, 2)
	noPhysics := newPullPhysicsSprite(2, NoPhysics, 2, 3)
	missing := newPullPhysicsSprite(4, DynamicPhysics, 4, 5)
	missing.runtimeState.SyncSprite = nil
	last := newPullPhysicsSprite(3, DynamicPhysics, 3, 4)
	game := &Game{syncBuffer: engine.NewSpriteSyncBuffer(4)}
	game.shapeMgr.items = []Shape{first, noPhysics, struct{}{}, missing, last}

	game.pullPhysicsPositions()
	if mgr.calls != 1 || !slices.Equal(mgr.ids, []int64{1, 3}) {
		t.Fatalf("position query calls=%d ids=%v, want one query for [1 3]", mgr.calls, mgr.ids)
	}
	for _, tt := range []struct {
		sprite  *SpriteImpl
		x, y    float64
		version uint64
	}{
		{first, 11, 12, 1},
		{noPhysics, 2, 3, 0},
		{missing, 4, 5, 0},
		{last, 31, 32, 1},
	} {
		x, y := tt.sprite.transform().getXY()
		if x != tt.x || y != tt.y || tt.sprite.spriteState.VisualVersion != tt.version {
			t.Errorf("sprite position=(%v,%v) visual version=%d, want (%v,%v) version %d", x, y, tt.sprite.spriteState.VisualVersion, tt.x, tt.y, tt.version)
		}
		if tt.sprite.spriteState.IsDirty || tt.sprite.spriteState.DirtyVersion != 0 {
			t.Errorf("physics pull dirtied proxy: %+v", tt.sprite.spriteState)
		}
	}
	game.pullPhysicsPositions()
	if first.spriteState.VisualVersion != 1 || last.spriteState.VisualVersion != 1 {
		t.Fatal("unchanged position incremented visual version")
	}
	mgr.positions = []float32{13, 14, float32(math.NaN()), 40}
	game.pullPhysicsPositions()
	if x, y := last.transform().getXY(); x != 31 || y != 32 || last.spriteState.VisualVersion != 1 {
		t.Fatalf("missing sentinel changed last sprite to (%v,%v), version %d", x, y, last.spriteState.VisualVersion)
	}
	if x, y := first.transform().getXY(); x != 13 || y != 14 || first.spriteState.VisualVersion != 2 {
		t.Fatalf("valid result not applied to first sprite: (%v,%v), version %d", x, y, first.spriteState.VisualVersion)
	}
	game.shapeMgr.items = []Shape{noPhysics, struct{}{}, missing}
	game.pullPhysicsPositions()
	if mgr.calls != 3 {
		t.Fatalf("empty physics pull made an engine query: %d total calls, want 3", mgr.calls)
	}
}

func TestApplyPhysicsPositionsStopsAtShortResponse(t *testing.T) {
	sprites := []*SpriteImpl{
		newPhysicsPositionTestSprite(1, 2),
		newPhysicsPositionTestSprite(3, 4),
	}
	applyPhysicsPositions(sprites, []float32{11, 12, 31})
	if x, y := sprites[0].transform().getXY(); x != 11 || y != 12 {
		t.Fatalf("first sprite position=(%v,%v), want (11,12)", x, y)
	}
	if x, y := sprites[1].transform().getXY(); x != 3 || y != 4 {
		t.Fatalf("short response changed second sprite to (%v,%v)", x, y)
	}
}

func TestApplyPhysicsPositionInvalidatesVisualsOnlyWhenChanged(t *testing.T) {
	sprite := newPhysicsPositionTestSprite(1, 2)

	sprite.applyPhysicsPosition(1, 2)
	if sprite.spriteState.VisualVersion != 0 {
		t.Fatalf("unchanged position visual version = %d, want 0", sprite.spriteState.VisualVersion)
	}

	sprite.applyPhysicsPosition(3, 4)
	if sprite.spriteState.VisualVersion != 1 {
		t.Fatalf("changed position visual version = %d, want 1", sprite.spriteState.VisualVersion)
	}
	if sprite.spriteState.IsDirty || sprite.spriteState.DirtyVersion != 0 {
		t.Fatalf(
			"physics writeback dirtied proxy state: IsDirty=%t, DirtyVersion=%d",
			sprite.spriteState.IsDirty,
			sprite.spriteState.DirtyVersion,
		)
	}
	if x, y := sprite.transform().getXY(); x != 3 || y != 4 {
		t.Fatalf("physics position = (%v, %v), want (3, 4)", x, y)
	}

	sprite.applyPhysicsPosition(3, 4)
	if sprite.spriteState.VisualVersion != 1 {
		t.Fatalf("repeated position visual version = %d, want 1", sprite.spriteState.VisualVersion)
	}
}

func TestCameraFollowObservesPhysicsPositionChanges(t *testing.T) {
	sprite := newPhysicsPositionTestSprite(1, 2)
	camera := &cameraImpl{
		followTarget:          sprite,
		observedFollowVersion: sprite.spriteState.VisualVersion,
	}

	if changed, _ := camera.getFollowPos(); changed {
		t.Fatal("unchanged follow target unexpectedly invalidated the camera")
	}

	sprite.applyPhysicsPosition(3, 4)
	changed, pos := camera.getFollowPos()
	if !changed {
		t.Fatal("physics position change did not invalidate the camera")
	}
	if pos.X != 3 || pos.Y != 4 {
		t.Fatalf("camera follow position = (%v, %v), want (3, 4)", pos.X, pos.Y)
	}

	camera.observedFollowVersion = sprite.spriteState.VisualVersion
	if changed, _ := camera.getFollowPos(); changed {
		t.Fatal("observed follow target still invalidated the camera")
	}
}
