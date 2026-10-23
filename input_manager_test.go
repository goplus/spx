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
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine"
)

func TestKeyFromStringRecognizesExclam(t *testing.T) {
	if got := KeyFromString("!"); got != KeyExclam {
		t.Fatalf("KeyFromString(\"!\") = %v, want %v", got, KeyExclam)
	}
}

func TestMouseCoordinatesUseLatestLiveInput(t *testing.T) {
	var game Game
	game.inputMgr.init(&game)
	for _, point := range []mathf.Vec2{{X: 12.5, Y: -8}, {X: -20, Y: 31.25}} {
		game.inputMgr.setMousePos(point)
		if x, y := game.MouseX(), game.MouseY(); x != point.X || y != point.Y {
			t.Fatalf("mouse coordinates = (%v, %v), want (%v, %v)", x, y, point.X, point.Y)
		}
	}

	game.inputMgr.init(&game)
	if x, y := game.MouseX(), game.MouseY(); x != 0 || y != 0 {
		t.Fatalf("mouse coordinates after reinitialization = (%v, %v), want (0, 0)", x, y)
	}
}

func TestSwipeRoutingAllowsHandlerToBeginNextGesture(t *testing.T) {
	previous := gco
	gco = nil
	t.Cleanup(func() { gco = previous })
	for _, tt := range []struct {
		name        string
		spriteFirst bool
		debug       bool
	}{
		{"stage first with debug", false, true},
		{"sprite first without debug", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game := &Game{}
			game.debugState.DebugEvent = tt.debug
			previousGame := engine.GetGame()
			engine.SetGame(game)
			t.Cleanup(func() { engine.SetGame(previousGame) })
			game.bindScriptEvents()
			sprite := &SpriteImpl{name: "target", g: game}
			sprite.scriptEventBindings.bind(&game.scriptEvents, sprite)
			game.inputMgr.g = game
			now := time.Unix(100, 0)
			game.inputMgr.swipe.InitWithClock(func() time.Time { return now })
			var routes []string
			record := func(name string, next *SpriteImpl) {
				routes = append(routes, name)
				if len(routes) == 1 {
					game.inputMgr.swipe.Begin(mathf.Vec2{X: 100}, next)
				}
			}
			game.OnSwipe__0(90, func() { record("stage", sprite) })
			sprite.OnSwipe__0(90, func() { record("sprite", nil) })
			var target *SpriteImpl
			want := []string{"stage", "sprite"}
			if tt.spriteFirst {
				target = sprite
				want = []string{"sprite", "stage"}
			}
			game.inputMgr.swipe.Begin(mathf.Vec2{}, target)
			now = now.Add(100 * time.Millisecond)
			game.inputMgr.onLeftButtonUp(mathf.Vec2{X: 100})
			now = now.Add(100 * time.Millisecond)
			game.inputMgr.onLeftButtonUp(mathf.Vec2{X: 200})
			game.inputMgr.onLeftButtonUp(mathf.Vec2{X: 300})
			if !slices.Equal(routes, want) {
				t.Fatalf("routes = %v, want %v without duplicate finish dispatch", routes, want)
			}
		})
	}
}

func TestSwipeRoutingIgnoresFailedAndExpiredGestures(t *testing.T) {
	previous := gco
	gco = nil
	t.Cleanup(func() { gco = previous })
	for _, tt := range []struct {
		name     string
		elapsed  time.Duration
		distance float64
		expire   bool
	}{
		{"short", 100 * time.Millisecond, 10, false},
		{"zero elapsed", 0, 100, false},
		{"expired", 600 * time.Millisecond, 100, false},
		{"expired then clock rollback", 600 * time.Millisecond, 100, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game := &Game{}
			game.debugState.DebugEvent = true
			previousGame := engine.GetGame()
			engine.SetGame(game)
			t.Cleanup(func() { engine.SetGame(previousGame) })
			game.bindScriptEvents()
			sprite := &SpriteImpl{name: "target", g: game}
			sprite.scriptEventBindings.bind(&game.scriptEvents, sprite)
			game.inputMgr.g = game
			now := time.Unix(100, 0)
			game.inputMgr.swipe.InitWithClock(func() time.Time { return now })
			calls := 0
			game.OnSwipe__0(90, func() { calls++ })
			sprite.OnSwipe__0(90, func() { calls++ })
			for _, target := range []*SpriteImpl{nil, sprite} {
				game.inputMgr.swipe.Begin(mathf.Vec2{}, target)
				now = now.Add(tt.elapsed)
				if tt.expire {
					game.inputMgr.onMouseMove(mathf.Vec2{X: 50})
					now = now.Add(-500 * time.Millisecond)
				}
				game.inputMgr.onLeftButtonUp(mathf.Vec2{X: tt.distance})
				now = now.Add(100 * time.Millisecond)
				game.inputMgr.onLeftButtonUp(mathf.Vec2{X: 100})
			}
			if calls != 0 {
				t.Fatalf("invalid gestures dispatched %d handlers", calls)
			}
		})
	}
}

func TestKeyFromStringSpecialAndUnknownNames(t *testing.T) {
	if KeyFromString("Any") != KeyAny {
		t.Fatal("Any did not resolve to wildcard")
	}
	if KeyFromString("not a keyboard key") != KeyMax {
		t.Fatal("unknown key did not return KeyMax")
	}
}
