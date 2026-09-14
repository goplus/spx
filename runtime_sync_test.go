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

import "testing"

func newPhysicsPositionTestSprite(x, y float64) *SpriteImpl {
	sprite := &SpriteImpl{}
	sprite.components.transform = &transformComponent{
		componentBase: componentBase{sprite: sprite},
		x:             x,
		y:             y,
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
