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

	"github.com/goplus/spx/v3/internal/coroutine"
)

func TestSpriteMotionIgnoresMissingTarget(t *testing.T) {
	for _, action := range []struct {
		name string
		run  func(*cloneLimitSprite)
	}{
		{"step", func(sprite *cloneLimitSprite) { sprite.StepTo__1("missing") }},
		{"glide zero duration", func(sprite *cloneLimitSprite) { sprite.Glide__1("missing", 0) }},
		{"glide nonzero duration", func(sprite *cloneLimitSprite) { sprite.Glide__1("missing", 60) }},
		{"turn", func(sprite *cloneLimitSprite) { sprite.TurnTo__1("missing") }},
	} {
		t.Run(action.name, func(t *testing.T) {
			game := setupCloneLimitGame(t)
			sprite := newCloneLimitSprite(game, "source")
			sprite.transform().x, sprite.transform().y, sprite.transform().direction = 30, 40, 60
			dirtyVersion := sprite.spriteState.DirtyVersion
			finished := false
			thread := gco.Create(sprite, func(coroutine.Thread) {
				action.run(sprite)
				finished = true
			})
			gco.JoinYieldedOrDone(thread)

			if !finished || sprite.Xpos() != 30 || sprite.Ypos() != 40 || sprite.Heading() != 60 || sprite.spriteState.DirtyVersion != dirtyVersion {
				t.Fatalf("missing target altered sprite: finished=%v position=(%v,%v) heading=%v dirty=%v", finished, sprite.Xpos(), sprite.Ypos(), sprite.Heading(), sprite.spriteState.DirtyVersion)
			}
		})
	}
}

func TestSpriteTurnWithoutAnimationFinishesImmediately(t *testing.T) {
	for _, mode := range []struct {
		name  string
		debug bool
	}{
		{"normal", false},
		{"debug instructions", true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, action := range []struct {
				name string
				run  func(*cloneLimitSprite)
			}{
				{"named target", func(sprite *cloneLimitSprite) { sprite.TurnToTarget("target", nil) }},
				{"coordinates", func(sprite *cloneLimitSprite) { sprite.TurnToXYpos(-10, -10, nil) }},
				{"direction", func(sprite *cloneLimitSprite) { sprite.TurnToDir(945, nil) }},
			} {
				t.Run(action.name, func(t *testing.T) {
					game := setupCloneLimitGame(t)
					originalDebug := game.debugState.DebugInstr
					game.debugState.DebugInstr = mode.debug
					t.Cleanup(func() { game.debugState.DebugInstr = originalDebug })
					sprite := newCloneLimitSprite(game, "source")
					sprite.SetHeading(60)
					target := newCloneLimitSprite(game, "target")
					target.SetXYpos(-10, -10)
					wantDirty := sprite.spriteState.DirtyVersion + 1

					// Both the initial turn and the unchanged heading must return
					// without waiting; only the initial turn publishes a change.
					for attempt := 0; attempt < 2; attempt++ {
						finished := false
						thread := gco.Create(sprite, func(coroutine.Thread) {
							action.run(sprite)
							finished = true
						})
						gco.JoinYieldedOrDone(thread)
						if !finished {
							t.Fatalf("turn %d yielded without an animation", attempt+1)
						}
						gco.Join(thread)
						if sprite.Heading() != -135 || sprite.spriteState.DirtyVersion != wantDirty {
							t.Fatalf("turn %d: heading=%v dirty=%v, want -135 and %v", attempt+1, sprite.Heading(), sprite.spriteState.DirtyVersion, wantDirty)
						}
						if sprite.Xpos() != 0 || sprite.Ypos() != 0 {
							t.Fatalf("turn moved sprite to (%v, %v)", sprite.Xpos(), sprite.Ypos())
						}
					}
				})
			}
		})
	}
}

func TestSpriteDistanceToMissingTarget(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	if got := sprite.DistanceTo__1("missing"); got != 10000 {
		t.Fatalf("DistanceTo__1(missing) = %v, want 10000", got)
	}
}

func TestSpriteNamedTargetIgnoresClones(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	sprite.transform().x, sprite.transform().y = 30, 40
	target := newCloneLimitSprite(game, "target")
	clone := createRuntimeClone(&target.SpriteImpl)
	clone.transform().x = 300

	if got := sprite.DistanceTo__1("target"); got != 50 {
		t.Fatalf("DistanceTo__1(target) = %v, want 50", got)
	}
}

func TestSpriteNilTargetIsMissing(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	var target *cloneLimitSprite
	if got := sprite.DistanceTo__0(target); got != 10000 {
		t.Fatalf("DistanceTo__0(nil) = %v, want 10000", got)
	}
}

func TestSpriteDirectionToMissingTargetKeepsHeading(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	sprite.transform().direction = 60
	if got := sprite.DirectionTo__1("missing"); got != 60 {
		t.Fatalf("DirectionTo__1(missing) = %v, want 60", got)
	}
}
