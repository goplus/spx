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

func TestSpriteSetCostumeInvalidNameIsNoOp(t *testing.T) {
	sprite := newTestRenderSprite()
	sprite.spriteState.DefaultCostumeIndex = 0

	sprite.setCostume(SpriteCostumeName("missing"))

	if got := sprite.costumeIndex; got != 1 {
		t.Fatalf("costumeIndex = %d, want 1", got)
	}
	if got := sprite.spriteState.DefaultCostumeIndex; got != 0 {
		t.Fatalf("DefaultCostumeIndex = %d, want 0", got)
	}
	if sprite.spriteState.IsDirty {
		t.Fatal("IsDirty = true, want false")
	}
	if sprite.runtimeState.IsCostumeDirty {
		t.Fatal("IsCostumeDirty = true, want false")
	}
}

func TestSpriteSetCostumeValidNameUpdatesState(t *testing.T) {
	sprite := newTestRenderSprite()
	sprite.spriteState.DefaultCostumeIndex = 1

	sprite.setCostume(SpriteCostumeName("idle"))

	if got := sprite.costumeIndex; got != 0 {
		t.Fatalf("costumeIndex = %d, want 0", got)
	}
	if got := sprite.spriteState.DefaultCostumeIndex; got != 0 {
		t.Fatalf("DefaultCostumeIndex = %d, want 0", got)
	}
	if !sprite.spriteState.IsDirty {
		t.Fatal("IsDirty = false, want true")
	}
	if !sprite.runtimeState.IsCostumeDirty {
		t.Fatal("IsCostumeDirty = false, want true")
	}
}
