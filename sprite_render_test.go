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
