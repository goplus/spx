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
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/engine"
)

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
