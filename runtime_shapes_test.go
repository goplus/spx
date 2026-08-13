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

type targetLookupSprite struct {
	SpriteImpl
}

func (*targetLookupSprite) Main() {}

func newTargetLookupSprite(g *Game, name string, cloned bool) *targetLookupSprite {
	sprite := &targetLookupSprite{}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.spriteState.Cloned = cloned
	return sprite
}

func TestGameGetTargetReturnsNamedSprite(t *testing.T) {
	var game Game
	clone := newTargetLookupSprite(&game, "Hero", true)
	target := newTargetLookupSprite(&game, "Hero", false)
	game.addShape(&clone.SpriteImpl)
	game.addShape(&target.SpriteImpl)

	if got := game.GetTarget("Hero"); got != &target.SpriteImpl {
		t.Fatalf("GetTarget(%q) = %v, want %v", "Hero", got, &target.SpriteImpl)
	}
}

func TestGameGetTargetReturnsNilWithoutMatch(t *testing.T) {
	var game Game
	game.addShape(&newTargetLookupSprite(&game, "Hero", true).SpriteImpl)

	for _, target := range []string{"", "Missing", "hero"} {
		if got := game.GetTarget(target); got != nil {
			t.Errorf("GetTarget(%q) = %v, want nil", target, got)
		}
	}
}

func TestGameGetTargetReturnsNilAfterTargetIsRemoved(t *testing.T) {
	var game Game
	target := newTargetLookupSprite(&game, "Hero", false)
	clone := newTargetLookupSprite(&game, "Hero", true)
	game.addShape(&target.SpriteImpl)
	game.addShape(&clone.SpriteImpl)

	game.removeShape(&target.SpriteImpl)

	if got := game.GetTarget("Hero"); got != nil {
		t.Fatalf("GetTarget(%q) after target removal = %v, want nil", "Hero", got)
	}
}
