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
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	coreruntime "github.com/goplus/spx/v4/internal/core/runtime"
	"github.com/goplus/spx/v4/internal/coroutine"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestSpriteSetDragMode(t *testing.T) {
	var sprite SpriteImpl
	if sprite.spriteState.isDraggable {
		t.Fatal("sprites must default to not draggable")
	}
	for _, mode := range []DragMode{NotDraggable, DragMode(-1), DragMode(2)} {
		sprite.SetDragMode(Draggable)
		if !sprite.spriteState.isDraggable {
			t.Fatal("draggable mode did not enable dragging")
		}
		sprite.SetDragMode(mode)
		if sprite.spriteState.isDraggable {
			t.Fatalf("mode %v did not disable dragging", mode)
		}
	}
}

func TestSpriteDragModeLoadsAndClones(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "not draggable"
		config := `{"costumes":[{"name":"idle","imageWidth":10,"imageHeight":10}],"isDraggable":false}`
		if enabled {
			name = "draggable"
			config = `{"costumes":[{"name":"idle","imageWidth":10,"imageHeight":10}],"isDraggable":true}`
		}
		t.Run(name, func(t *testing.T) {
			files := gameTestFS{"sprites/owner/index.json": config}
			game := setupStageLoadTestGame(t, files)
			gamer := reflect.ValueOf(&game.Game).Elem()
			sprite := &layoutInterfaceOwnerSprite{}
			prepared, err := prepareSprite(files, "owner", reflect.TypeOf(*sprite), gamer)
			if err != nil {
				t.Fatal(err)
			}
			game.loadPreparedSprite(sprite, "owner", gamer, &prepared)
			if sprite.spriteState.isDraggable != enabled {
				t.Fatalf("loaded draggable = %v, want %v", sprite.spriteState.isDraggable, enabled)
			}
			var clone SpriteImpl
			clone.InitFrom(&sprite.SpriteImpl)
			if clone.spriteState.isDraggable != enabled {
				t.Fatalf("cloned draggable = %v, want %v", clone.spriteState.isDraggable, enabled)
			}
			clone.SetDragMode(Draggable)
			sprite.SetDragMode(NotDraggable)
			if !clone.spriteState.isDraggable || sprite.spriteState.isDraggable {
				t.Fatal("changing the original's drag mode affected the clone")
			}
		})
	}
}

func TestSpriteDragFramesPreserveOffsetAndRelease(t *testing.T) {
	co, game, sprite := setupSpriteDragTest(t)
	other := newDragTestSprite(game, "foreground", 2)
	game.shapeMgr.add(other)
	other.greffUniforms = map[EffectKind]float64{GhostEffect: 100}
	start := mathf.NewVec2(12, 24)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	assertSpriteDragPosition(t, sprite, 10, 20)
	if game.inputMgr.drag.active {
		t.Fatal("stationary press started a drag")
	}
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(42, 64))
	assertSpriteDragPosition(t, sprite, 40, 60)
	if !game.inputMgr.drag.active {
		t.Fatal("pointer movement did not start dragging")
	}
	shapes := game.shapeMgr.getTempShapes()
	if shapes[len(shapes)-1] != sprite {
		t.Fatal("dragged sprite was not brought to the front")
	}
	end := mathf.NewVec2(52, 74)
	dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
	assertSpriteDragPosition(t, sprite, 50, 70)
	if game.inputMgr.drag.target != nil || game.inputMgr.drag.active {
		t.Fatal("release retained the drag")
	}
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(80, 90))
	assertSpriteDragPosition(t, sprite, 50, 70)
}

func TestSpriteDragClickAndSwipeRouting(t *testing.T) {
	for _, moved := range []bool{false, true} {
		name := "click"
		if moved {
			name = "drag"
		}
		t.Run(name, func(t *testing.T) {
			co, game, sprite := setupSpriteDragTest(t)
			clicks, swipes := 0, 0
			sprite.OnClick(func() { clicks++ })
			sprite.OnSwipe__0(90, func() { swipes++ })
			game.OnClick(func() { t.Error("sprite gesture fell through to stage click") })
			game.OnSwipe__0(90, func() { t.Error("sprite drag fell through to stage swipe") })
			point := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(co, game, point, &eventLeftButtonDown{Pos: point})
			if clicks != 0 {
				t.Fatal("draggable sprite clicked on mouse down")
			}
			if moved {
				point.X += 100
				dispatchSpriteDragFrame(co, game, point)
			}
			dispatchSpriteDragFrame(co, game, point, &eventLeftButtonUp{Pos: point})
			wantClicks := 1
			if moved {
				wantClicks = 0
			}
			if clicks != wantClicks || swipes != 0 {
				t.Fatalf("clicks/swipes = %d/%d, want %d/0", clicks, swipes, wantClicks)
			}
		})
	}
}

func TestSpriteDragNonDraggableForegroundDoesNotBlockDrag(t *testing.T) {
	co, game, sprite := setupSpriteDragTest(t)
	top := newDragTestSprite(game, "top", 2)
	game.shapeMgr.add(top)
	clicks := 0
	top.OnClick(func() { clicks++ })
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(30, 40))
	assertSpriteDragPosition(t, sprite, 30, 40)
	assertSpriteDragPosition(t, top, 10, 20)
	if clicks != 1 || game.inputMgr.drag.target != sprite || !game.inputMgr.drag.active {
		t.Fatal("foreground must click on mouse down without preventing a background drag")
	}
}

func TestSpriteDragCandidateDoesNotSuppressForegroundSwipe(t *testing.T) {
	for _, change := range []string{"hide", "disable", "viewport jitter"} {
		t.Run(change, func(t *testing.T) {
			co, game, candidate := setupSpriteDragTest(t)
			clock := &inputSession{}
			game.inputSession = clock
			foreground := newDragTestSprite(game, "foreground", 2)
			game.shapeMgr.add(foreground)
			clicks, swipes := 0, 0
			foreground.OnClick(func() { clicks++ })
			foreground.OnSwipe__0(90, func() { swipes++ })
			candidate.OnSwipe__0(90, func() { t.Error("swipe reached the drag candidate") })
			game.OnSwipe__0(90, func() { t.Error("foreground swipe reached the stage") })
			if change == "viewport jitter" {
				camera := pkgengine.CameraMgr.(*dragCameraMgr)
				camera.world.Size = camera.viewport.Size.Divf(.02)
			}
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
			switch change {
			case "hide":
				candidate.Hide()
			case "disable":
				candidate.SetDragMode(NotDraggable)
			}
			// 100 world units in 100 ms is a swipe; at .02 zoom it is only
			// two viewport pixels and does not activate the pending drag.
			clock.currentTime = .1
			end := start.Add(mathf.NewVec2(100, 0))
			dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
			if clicks != 1 || swipes != 1 {
				t.Fatalf("foreground clicks/swipes = %d/%d, want 1/1", clicks, swipes)
			}
			assertSpriteDragPosition(t, candidate, 10, 20)
			assertSpriteDragPosition(t, foreground, 10, 20)
		})
	}
}

func TestSpriteDragCandidateCancellationPreservesForegroundClick(t *testing.T) {
	for _, action := range []string{"hide", "disable"} {
		t.Run(action, func(t *testing.T) {
			co, game, candidate := setupSpriteDragTest(t)
			clock := &inputSession{}
			game.inputSession = clock
			foreground := newDragTestSprite(game, "foreground", 2)
			game.shapeMgr.add(foreground)
			clicks := 0
			foreground.OnClick(func() { clicks++ })
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
			if clicks != 1 {
				t.Fatalf("mouse-down clicks = %d, want 1", clicks)
			}
			if action == "hide" {
				candidate.Hide()
			} else {
				candidate.SetDragMode(NotDraggable)
			}
			foreground.SetDragMode(Draggable)
			clock.currentTime = .1
			end := start.Add(mathf.NewVec2(1, 0))
			dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
			if clicks != 2 {
				t.Fatalf("mouse-down and mouse-up clicks = %d, want 2", clicks)
			}
		})
	}
}

func TestSpriteDragCancellation(t *testing.T) {
	for _, test := range []struct {
		name   string
		cancel func(*Game, *SpriteImpl)
	}{
		{"disable and enable", func(_ *Game, s *SpriteImpl) { s.SetDragMode(NotDraggable); s.SetDragMode(Draggable) }},
		{"hide and show", func(_ *Game, s *SpriteImpl) { s.Hide(); s.Show() }},
		{"destroy", func(_ *Game, s *SpriteImpl) { s.destroy() }},
		{"reset input", func(g *Game, _ *SpriteImpl) { g.inputMgr.init(g) }},
	} {
		for _, active := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/active=%v", test.name, active), func(t *testing.T) {
				co, game, sprite := setupSpriteDragTest(t)
				clock := &inputSession{}
				game.inputSession = clock
				other := newDragTestSprite(game, "other", 2)
				other.SetDragMode(Draggable)
				game.shapeMgr.add(other)
				other.SetLayerTo(Back)
				clicks := 0
				sprite.OnClick(func() { clicks++ })
				other.OnClick(func() { clicks++ })
				start := mathf.NewVec2(10, 20)
				dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
				want := start
				if active {
					want = mathf.NewVec2(30, 40)
					dispatchSpriteDragFrame(co, game, want)
				}
				test.cancel(game, sprite)
				if game.inputMgr.drag != (spriteDragState{}) {
					t.Fatal("cancellation retained the drag")
				}
				other.SetLayerTo(Front)
				clock.currentTime = 1
				end := mathf.NewVec2(50, 60)
				dispatchSpriteDragFrame(co, game, end)
				if game.inputMgr.drag != (spriteDragState{}) {
					t.Fatal("cancelled drag resumed or transferred")
				}
				dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
				assertSpriteDragPosition(t, sprite, want.X, want.Y)
				assertSpriteDragPosition(t, other, 10, 20)
				if clicks != 0 {
					t.Fatal("cancelled drag generated a click")
				}
				dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
				dispatchSpriteDragFrame(co, game, mathf.NewVec2(30, 40))
				if game.inputMgr.drag.target != other || !game.inputMgr.drag.active {
					t.Fatal("fresh press did not capture the new foreground")
				}
			})
		}
	}
}

func TestSpriteDragMovementThresholdUsesViewportPixels(t *testing.T) {
	for _, scale := range []float64{0.5, 1, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			co, game, sprite := setupSpriteDragTest(t)
			game.inputSession = &inputSession{}
			camera := pkgengine.CameraMgr.(*dragCameraMgr)
			camera.world.Size = camera.viewport.Size.Divf(scale)
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
			for _, dx := range []float64{1, -2, 3} {
				dispatchSpriteDragFrame(co, game, start.Add(mathf.NewVec2(dx/scale, 0)))
				if game.inputMgr.drag.active {
					t.Fatalf("%v viewport pixels started a drag", dx)
				}
				assertSpriteDragPosition(t, sprite, 10, 20)
			}
			dispatchSpriteDragFrame(co, game, start.Add(mathf.NewVec2(3.1/scale, 0)))
			if !game.inputMgr.drag.active {
				t.Fatal("movement beyond 3 viewport pixels did not start a drag")
			}
		})
	}
}

func TestSpriteDragJitterStillClicks(t *testing.T) {
	co, game, sprite := setupSpriteDragTest(t)
	game.inputSession = &inputSession{}
	clicks := 0
	sprite.OnClick(func() { clicks++ })
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	end := mathf.NewVec2(12, 22)
	dispatchSpriteDragFrame(co, game, end)
	dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
	assertSpriteDragPosition(t, sprite, 10, 20)
	if clicks != 1 {
		t.Fatalf("jitter produced %d clicks, want 1", clicks)
	}
}

func TestSpriteDragLongPressUsesInputClock(t *testing.T) {
	co, game, sprite := setupSpriteDragTest(t)
	clock := &inputSession{currentTime: 1}
	game.inputSession = clock
	clicks := 0
	sprite.OnClick(func() { clicks++ })
	point := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, point, &eventLeftButtonDown{Pos: point})
	for _, elapsed := range []float64{-0.1, 0.399, 0.399998, 0.4} {
		clock.currentTime = 1 + elapsed
		dispatchSpriteDragFrame(co, game, point)
		if got, want := game.inputMgr.drag.active, elapsed >= 0.4; got != want {
			t.Fatalf("active after %v seconds = %v, want %v", elapsed, got, want)
		}
	}
	dispatchSpriteDragFrame(co, game, point, &eventLeftButtonUp{Pos: point})
	if clicks != 0 {
		t.Fatal("stationary long press generated a click")
	}
}

func TestSpriteDragLongPressAfterExtendedRecording(t *testing.T) {
	for _, tc := range []struct{ fps, start int }{{30, 16174}, {60, 32656}, {30, 21675}, {60, 21663}} {
		for _, releaseAtBoundary := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dfps/start=%d/release=%v", tc.fps, tc.start, releaseAtBoundary), func(t *testing.T) {
				co, game, sprite := setupSpriteDragTest(t)
				clock := &inputSession{}
				game.inputSession = clock
				clicks := 0
				sprite.OnClick(func() { clicks++ })
				point := mathf.NewVec2(10, 20)
				boundary := tc.start + tc.fps*2/5
				// Existing v1 recordings accumulated timestamps one tick at a time.
				for i := 0; i <= boundary; i++ {
					if i > 0 {
						clock.currentTime += 1 / float64(tc.fps)
					}
					if i == tc.start {
						dispatchSpriteDragFrame(co, game, point, &eventLeftButtonDown{Pos: point})
					} else if i > tc.start {
						var events []event
						if releaseAtBoundary && i == boundary {
							events = append(events, &eventLeftButtonUp{Pos: point})
						}
						dispatchSpriteDragFrame(co, game, point, events...)
						if got, want := game.inputMgr.drag.active, i == boundary && !releaseAtBoundary; got != want {
							t.Fatalf("long press after frame %d: active=%v, want %v", i, got, want)
						}
					}
				}
				if clicks != 0 {
					t.Fatal("long-press release at 400 ms generated a click")
				}
			})
		}
	}
}

func TestSpriteDragActivationKeepsPressedTarget(t *testing.T) {
	for _, change := range []string{"move", "new draggable foreground", "new non-draggable foreground"} {
		t.Run(change, func(t *testing.T) {
			co, game, sprite := setupSpriteDragTest(t)
			game.inputSession = &inputSession{}
			start := mathf.NewVec2(12, 24)
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
			if game.inputMgr.drag.target != sprite {
				t.Fatal("press did not capture the draggable sprite")
			}
			if change == "move" {
				sprite.SetXYpos(20, 30)
			} else {
				top := newDragTestSprite(game, "new top", 2)
				game.shapeMgr.add(top)
				if change == "new draggable foreground" {
					top.SetDragMode(Draggable)
				}
			}
			pkgengine.SpriteMgr.(*dragSpriteMgr).hit = func(_ pkgengine.Object, _ mathf.Vec2) bool {
				t.Error("activation repeated hit testing after the press")
				return false
			}
			dispatchSpriteDragFrame(co, game, mathf.NewVec2(32, 44))
			if game.inputMgr.drag.target != sprite || !game.inputMgr.drag.active {
				t.Fatal("activation changed or lost the pressed target")
			}
			wantX, wantY := 30.0, 40.0
			if change == "move" {
				wantX, wantY = 40, 50
			}
			assertSpriteDragPosition(t, sprite, wantX, wantY)
			shapes := game.shapeMgr.getTempShapes()
			if shapes[len(shapes)-1] != sprite {
				t.Fatal("activated target is not at the front")
			}
		})
	}
}

func TestSpriteDragActivationAfterSpriteLeavesPressPoint(t *testing.T) {
	co, game, sprite := setupSpriteDragTest(t)
	game.inputSession = &inputSession{}
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	sprite.SetXYpos(200, 100)
	pkgengine.SpriteMgr.(*dragSpriteMgr).hits[1] = false
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(30, 40))
	if !game.inputMgr.drag.active || game.inputMgr.drag.target != sprite {
		t.Fatal("activation lost the captured target after it moved")
	}
	assertSpriteDragPosition(t, sprite, 220, 120)
}

func TestSpriteDragActivationAfterLayerChange(t *testing.T) {
	co, game, sprite := setupSpriteDragTest(t)
	game.inputSession = &inputSession{}
	other := newDragTestSprite(game, "other", 2)
	other.SetDragMode(Draggable)
	game.shapeMgr.add(other)
	other.SetLayerTo(Back)
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	if game.inputMgr.drag.target != sprite {
		t.Fatal("press did not pick the original foreground")
	}
	other.SetLayerTo(Front)
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(30, 40))
	if game.inputMgr.drag.target != sprite || !game.inputMgr.drag.active {
		t.Fatal("layer change transferred the captured gesture")
	}
	assertSpriteDragPosition(t, sprite, 30, 40)
	assertSpriteDragPosition(t, other, 10, 20)
}

func TestSpriteDragReleaseRepicksTarget(t *testing.T) {
	for _, kind := range []string{"draggable", "not draggable", "miss"} {
		t.Run(kind, func(t *testing.T) {
			co, game, sprite := setupSpriteDragTest(t)
			game.inputSession = &inputSession{}
			originalClicks, topClicks := 0, 0
			sprite.OnClick(func() { originalClicks++ })
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
			top := newDragTestSprite(game, "release target", 2)
			top.OnClick(func() { topClicks++ })
			game.shapeMgr.add(top)
			if kind == "draggable" {
				top.SetDragMode(Draggable)
			}
			end := mathf.NewVec2(11, 20)
			pkgengine.SpriteMgr.(*dragSpriteMgr).hit = func(_ pkgengine.Object, point mathf.Vec2) bool {
				if point != end {
					t.Errorf("release picked at %v, want %v", point, end)
				}
				return kind != "miss"
			}
			dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
			wantTop := 0
			if kind == "draggable" {
				wantTop = 1
			}
			if originalClicks != 0 || topClicks != wantTop {
				t.Fatalf("release clicks = %d/%d, want 0/%d", originalClicks, topClicks, wantTop)
			}
		})
	}
}

func TestSpriteDragCrossTargetRelease(t *testing.T) {
	for _, origin := range []string{"sprite", "stage"} {
		t.Run(origin, func(t *testing.T) {
			co, game, sprite := setupSpriteDragTest(t)
			clock := &inputSession{}
			game.inputSession = clock
			sprite.SetDragMode(NotDraggable)
			if origin == "stage" {
				sprite.Hide()
			}
			downClicks, releaseClicks := 0, 0
			sprite.OnClick(func() { downClicks++ })
			game.OnClick(func() { downClicks++ })
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
			if downClicks != 1 {
				t.Fatalf("mousedown clicks = %d, want 1", downClicks)
			}
			top := newDragTestSprite(game, "release target", 2)
			top.SetDragMode(Draggable)
			top.OnClick(func() { releaseClicks++ })
			game.shapeMgr.add(top)
			clock.currentTime = 0.1
			end := mathf.NewVec2(11, 20)
			dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
			clock.currentTime = 0.2
			dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
			if releaseClicks != 1 {
				t.Fatalf("cross-target release clicks = %d, want 1", releaseClicks)
			}
		})
	}
}

func TestSpriteDragEmptyPressNeverCapturesLaterTarget(t *testing.T) {
	for _, change := range []string{"appears", "enabled"} {
		t.Run(change, func(t *testing.T) {
			co, game, sprite := setupSpriteDragTest(t)
			clock := &inputSession{}
			game.inputSession = clock
			if change == "appears" {
				sprite.Hide()
			} else {
				sprite.SetDragMode(NotDraggable)
			}
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
			if game.inputMgr.drag.target != nil {
				t.Fatal("press captured an unavailable target")
			}
			clock.currentTime = .1
			dispatchSpriteDragFrame(co, game, start.Add(mathf.NewVec2(4, 0)))
			sprite.Show()
			sprite.SetDragMode(Draggable)
			for i, point := range []mathf.Vec2{start, start.Add(mathf.NewVec2(4, 0)), start.Add(mathf.NewVec2(5, 0))} {
				clock.currentTime = float64(i+1) * .4
				dispatchSpriteDragFrame(co, game, point)
				if game.inputMgr.drag.target != nil || game.inputMgr.drag.active {
					t.Fatalf("empty press captured a later target at step %d", i)
				}
			}
			assertSpriteDragPosition(t, sprite, 10, 20)
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonUp{Pos: start})
			clock.currentTime = 2
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
			dispatchSpriteDragFrame(co, game, start.Add(mathf.NewVec2(10, 0)))
			if game.inputMgr.drag.target != sprite || !game.inputMgr.drag.active {
				t.Fatal("a fresh press could not capture the available target")
			}
			assertSpriteDragPosition(t, sprite, 20, 20)
		})
	}
}

func TestSpriteDragCameraMotionDoesNotActivate(t *testing.T) {
	co, game, sprite := setupSpriteDragTest(t)
	game.inputSession = &inputSession{}
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	camera := pkgengine.CameraMgr.(*dragCameraMgr)
	camera.world.Position.X += 100
	// The same display pixel now corresponds to a different world point.
	dispatchSpriteDragFrame(co, game, start.Add(mathf.NewVec2(100, 0)))
	if game.inputMgr.drag.active {
		t.Fatal("camera movement activated a stationary pointer")
	}
	assertSpriteDragPosition(t, sprite, 10, 20)
}

func TestSpriteDragRecordingJSONRoundTrip(t *testing.T) {
	for _, fps := range []int{10, 30, 60} {
		for _, gesture := range []string{"jitter", "movement", "long press", "same tick click", "camera motion", "foreground", "layer change", "cancelled candidate", "empty press", "camera long press", "camera old target"} {
			t.Run(fmt.Sprintf("%s/%dfps", gesture, fps), func(t *testing.T) {
				resetInputSessionTest(t)
				co, game, sprite := setupSpriteDragTest(t)
				live := setupInputSourceManager(t)
				camera := pkgengine.CameraMgr.(*dragCameraMgr)
				camera.world.Size = camera.world.Size.Divf(2)
				initialCamera := camera.world
				mgr := pkgengine.SpriteMgr.(*dragSpriteMgr)
				mgr.hit = func(id pkgengine.Object, point mathf.Vec2) bool {
					if id == 2 {
						return true
					}
					x, y := sprite.getXY()
					return math.Abs(point.X-x) <= 5 && math.Abs(point.Y-y) <= 5
				}
				if gesture == "foreground" {
					game.shapeMgr.add(newDragTestSprite(game, "foreground", 2))
				}
				var other *SpriteImpl
				if gesture == "layer change" {
					other = newDragTestSprite(game, "other", 2)
					other.SetDragMode(Draggable)
					game.shapeMgr.add(other)
					other.SetLayerTo(Back)
				}
				if gesture == "empty press" {
					sprite.SetXYpos(200, 20)
				}
				applySceneFrame := func(i int) {
					if gesture == "layer change" && i == 1 {
						other.SetLayerTo(Front)
					}
					if gesture == "cancelled candidate" && i == 1 {
						sprite.Hide()
						sprite.Show()
					}
					if (gesture == "camera motion" || gesture == "camera long press" || gesture == "camera old target") && i == 1 {
						camera.world.Position.X += 100
						if gesture != "camera old target" {
							sprite.SetXYpos(110, 20)
						}
					}
					if gesture == "empty press" && i == fps*2/5+1 {
						sprite.SetXYpos(10, 20)
					}
				}
				if _, err := PrepareInputRecording(float64(fps)); err != nil {
					t.Fatal(err)
				}
				claimPreparedSession(t, game)
				t.Cleanup(func() { game.abortInputSession("test finished") })
				clicks := 0
				sprite.OnClick(func() { clicks++ })
				type observation struct {
					position mathf.Vec2
					active   bool
					clicks   int
					target   string
				}
				observe := func() observation {
					var target string
					if candidate := game.inputMgr.drag.target; candidate != nil {
						target = candidate.name
					}
					return observation{mathf.NewVec2(sprite.getXY()), game.inputMgr.drag.active, clicks, target}
				}
				points := []mathf.Vec2{{X: 10, Y: 20}, {X: 11, Y: 20}, {X: 11, Y: 20}}
				switch gesture {
				case "movement", "foreground", "layer change", "cancelled candidate":
					points = []mathf.Vec2{{X: 10, Y: 20}, {X: 11.5, Y: 20}, {X: 12, Y: 20}, {X: 14, Y: 20}}
				case "long press":
					points = make([]mathf.Vec2, fps*2/5+2)
					for i := range points {
						points[i] = mathf.NewVec2(10, 20)
					}
				case "camera motion":
					points = []mathf.Vec2{{X: 10, Y: 20}, {X: 110, Y: 20}, {X: 110, Y: 20}}
				case "camera long press", "camera old target", "empty press":
					points = make([]mathf.Vec2, fps*2/5+5)
					for i := range points {
						points[i] = mathf.NewVec2(10, 20)
						if gesture != "empty press" && i > 0 {
							points[i].X = 110
						}
						if gesture == "empty press" && i >= fps*2/5+3 {
							points[i].X = 14
						}
					}
				case "same tick click":
					points = []mathf.Vec2{{X: 10, Y: 20}}
				}
				var recorded []observation
				for i, point := range points {
					applySceneFrame(i)
					state := InputReplayState{Mouse: InputReplayMouse{X: point.X, Y: point.Y}, Buttons: 1}
					var edges []InputReplayMouseEvent
					if i == 0 {
						edges = append(edges, InputReplayMouseEvent{Button: 1, Pressed: true})
					}
					if i == len(points)-1 {
						state.Buttons = 0
						edges = append(edges, InputReplayMouseEvent{Button: 1})
					}
					// Mock only the engine sampling boundary; record and dispatch each tick.
					tick, err := consumeInputTickWithMouseEvents(state, edges, nil, 1/float64(fps))
					if err != nil {
						t.Fatal(err)
					}
					batch := &inputFrame{}
					game.inputMgr.frame.history.Process(coreruntime.InputFrame{
						Point: point, LeftButtonPressed: state.Buttons != 0,
						MouseEvents: engineMouseEventsFromReplay(tick.frame.MouseEvents, nil),
					}, batch.eventHooks(0))
					dispatchSpriteDragFrame(co, game, point, batch.events...)
					recorded = append(recorded, observe())
					wantActive := false
					switch gesture {
					case "movement", "foreground", "layer change":
						wantActive = i == 2
					case "long press":
						wantActive = i == fps*2/5
					case "camera long press", "camera old target":
						wantActive = i >= fps*2/5 && i < len(points)-1
					}
					if gesture == "camera long press" {
						assertSpriteDragPosition(t, sprite, points[i].X, 20)
					}
					if gesture == "layer change" {
						assertSpriteDragPosition(t, other, 10, 20)
						if i < len(points)-1 && game.inputMgr.drag.target != sprite {
							t.Fatal("layer change transferred the recorded candidate")
						}
					}
					if gesture == "camera old target" || gesture == "cancelled candidate" {
						assertSpriteDragPosition(t, sprite, 10, 20)
					}
					if gesture == "empty press" && i >= fps*2/5+1 && i < fps*2/5+3 {
						assertSpriteDragPosition(t, sprite, 10, 20)
					}
					if game.inputMgr.drag.active != wantActive {
						t.Errorf("recording tick %d active=%v, want %v", i, game.inputMgr.drag.active, wantActive)
					}
				}
				wantClicks := 1
				switch gesture {
				case "movement", "foreground", "layer change", "cancelled candidate", "long press", "camera long press", "camera old target":
					wantClicks = 0
				}
				if clicks != wantClicks {
					t.Errorf("recorded clicks=%d, want %d", clicks, wantClicks)
				}
				encoded, err := FinishInputRecordingJSON()
				if err != nil {
					t.Fatal(err)
				}
				replay, err := DecodeInputReplay(encoded)
				if err != nil {
					t.Fatal(err)
				}
				for i, frame := range replay.Frames {
					if math.Abs(frame.Time-float64(i)/float64(fps)) > 1e-9 {
						t.Fatalf("recorded tick %d time=%v", i, frame.Time)
					}
					if frame.State.Mouse != (InputReplayMouse{X: points[i].X, Y: points[i].Y}) {
						t.Fatal("recording changed world coordinates")
					}
				}
				game.abortInputSession("recording finished")
				game.resetBootstrap()
				game.inputMgr.init(game)
				sprite.SetXYpos(10, 20)
				if gesture == "empty press" {
					sprite.SetXYpos(200, 20)
				}
				if gesture == "foreground" {
					sprite.SetLayerTo(Back)
				}
				if other != nil {
					other.SetLayerTo(Back)
				}
				camera.world = initialCamera
				clicks = 0
				live.point = mathf.NewVec2(-999, -999)
				if _, err := PrepareInputReplay(replay); err != nil {
					t.Fatal(err)
				}
				session := claimPreparedSession(t, game)
				for i := range replay.Frames {
					applySceneFrame(i)
					if !game.inputMgr.prepareInputFrame(0.001) {
						t.Fatalf("replay tick %d missing", i)
					}
					game.dispatchFrameEvents()
					co.Update()
					session.endFrame()
					if got := observe(); got != recorded[i] {
						t.Fatalf("replay tick %d = %+v, recorded %+v", i, got, recorded[i])
					}
				}
			})
		}
	}
}

func TestSpriteDragReplayUsesRecordedPointerAndButtonState(t *testing.T) {
	for _, withEdges := range []bool{false, true} {
		name := "held snapshots"
		if withEdges {
			name = "ordered edges"
		}
		t.Run(name, func(t *testing.T) {
			resetInputSessionTest(t)
			co, game, sprite := setupSpriteDragTest(t)
			live := setupInputSourceManager(t)
			live.point = mathf.NewVec2(-200, -100)
			points := []mathf.Vec2{{X: 12, Y: 24}, {X: 42, Y: 64}, {X: 52, Y: 74}, {X: 80, Y: 90}}
			frames := make([]InputReplayFrame, len(points))
			for i, point := range points {
				buttons := uint8(0)
				if i < 2 {
					buttons = 1
				}
				frames[i] = InputReplayFrame{
					Frame: int64(i), Time: float64(i) / 30,
					State: InputReplayState{Mouse: InputReplayMouse{X: point.X, Y: point.Y}, Buttons: buttons},
				}
			}
			if withEdges {
				frames[0].MouseEvents = []InputReplayMouseEvent{{Button: 1, Pressed: true}}
				frames[2].MouseEvents = []InputReplayMouseEvent{{Button: 1, Pressed: false}}
			}
			if _, err := PrepareInputReplay(validRuntimeReplay(frames...)); err != nil {
				t.Fatal(err)
			}
			session := claimPreparedSession(t, game)
			t.Cleanup(func() { game.abortInputSession("test finished") })
			want := []mathf.Vec2{{X: 10, Y: 20}, {X: 40, Y: 60}, {X: 50, Y: 70}, {X: 50, Y: 70}}
			for i := range frames {
				if !game.inputMgr.prepareInputFrame(1.0 / 30) {
					t.Fatalf("replay frame %d was not prepared", i)
				}
				game.dispatchFrameEvents()
				co.Update()
				session.endFrame()
				assertSpriteDragPosition(t, sprite, want[i].X, want[i].Y)
			}
			if game.inputMgr.drag.target != nil {
				t.Fatal("replayed release retained the drag")
			}
		})
	}
}

func TestSpriteDragResetsAcrossReplayTransitions(t *testing.T) {
	resetInputSessionTest(t)
	co, game, sprite := setupSpriteDragTest(t)
	live := setupInputSourceManager(t)
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(30, 40))
	initial := InputReplayState{Mouse: InputReplayMouse{X: 80, Y: 90}, Buttons: 1}
	replay := validRuntimeReplay(InputReplayFrame{Frame: 0, State: initial})
	replay.Initial = initial
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	session := claimPreparedSession(t, game)
	t.Cleanup(func() { game.abortInputSession("test finished") })
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("first replay frame was not prepared")
	}
	game.dispatchFrameEvents()
	co.Update()
	session.endFrame()
	if game.inputMgr.drag.target != nil {
		t.Fatal("first replay tick retained a drag from live input")
	}
	assertSpriteDragPosition(t, sprite, 30, 40)

	game.inputMgr.onLeftButtonDown(mathf.NewVec2(30, 40))
	game.inputMgr.updateDrag(mathf.NewVec2(50, 60))
	game.abortInputSession("return to live input")
	live.point = mathf.NewVec2(-200, -100)
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("live handoff frame was not prepared")
	}
	game.dispatchFrameEvents()
	co.Update()
	if game.inputMgr.drag.target != nil {
		t.Fatal("live handoff retained a drag from replay input")
	}
	assertSpriteDragPosition(t, sprite, 50, 60)
}

func TestSpriteDragTouchingIsDirectionalUntilRelease(t *testing.T) {
	for _, named := range []bool{false, true} {
		name := "sprite"
		if named {
			name = "sprite name"
		}
		t.Run(name, func(t *testing.T) {
			game, _, receiver, target := newTouchingPair(t, false)
			game.inputMgr.init(game)
			target.SetDragMode(Draggable)
			touchTarget := func() bool { return receiver.Touching__0(target) }
			touchReceiver := func() bool { return target.Touching__0(receiver) }
			if named {
				touchTarget = func() bool { return receiver.Touching__1("target") }
				touchReceiver = func() bool { return target.Touching__1("receiver") }
			}
			if !touchTarget() || !touchReceiver() {
				t.Fatal("overlapping sprites did not touch before dragging")
			}
			game.inputMgr.drag = spriteDragState{target: &target.SpriteImpl, active: true}
			if touchTarget() {
				t.Fatal("another sprite detected a dragged target")
			}
			if !touchReceiver() {
				t.Fatal("dragged sprite could not detect another sprite")
			}
			game.inputMgr.onLeftButtonUp(mathf.Vec2{})
			if !touchTarget() || !touchReceiver() {
				t.Fatal("sensing did not recover after release")
			}
		})
	}
}

func TestSpriteDragBlocksScriptMotionWithoutDrawingPen(t *testing.T) {
	spy := setupSpyPenMgr(t)
	co, game, sprite := setupSpriteDragTest(t)
	sprite.PenDown()
	initialPenMoves := spy.moveCalls
	initialPenDowns, initialPenUps := spy.penDownCalls, spy.penUpCalls
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(30, 40))
	for _, move := range []func(){
		func() { sprite.SetXYpos(90, 100) },
		func() { sprite.SetXpos(90) },
		func() { sprite.SetYpos(100) },
		func() { sprite.ChangeXYpos(90, 100) },
		func() { sprite.ChangeXpos(90) },
		func() { sprite.ChangeYpos(100) },
		func() { sprite.Move__0(10) },
	} {
		move()
		assertSpriteDragPosition(t, sprite, 30, 40)
	}
	if spy.moveCalls != initialPenMoves {
		t.Fatalf("drag produced %d pen movements, want none", spy.moveCalls-initialPenMoves)
	}
	end := mathf.NewVec2(50, 60)
	dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
	if spy.moveCalls != initialPenMoves {
		t.Fatal("drag release drew a pen stroke")
	}
	initialEvents := len(spy.events)
	sprite.SetXYpos(70, 80)
	assertSpriteDragPosition(t, sprite, 70, 80)
	if got, want := spy.events[initialEvents:], []string{"restore", "move"}; !slices.Equal(got, want) {
		t.Fatalf("pen commands after release = %v, want %v", got, want)
	}
	if spy.lastRestore != end || spy.lastMove != mathf.NewVec2(70, 80) {
		t.Fatalf("pen resumed from %v to %v, want %v to (70, 80)", spy.lastRestore, spy.lastMove, end)
	}
	if spy.lastRestoreObject != *sprite.pen().penObj || spy.lastMoveObject != *sprite.pen().penObj {
		t.Fatal("resuming after drag targeted a different pen")
	}
	if spy.penDownCalls != initialPenDowns || spy.penUpCalls != initialPenUps || !sprite.pen().isPenDown {
		t.Fatal("resuming after drag toggled the pen and could draw an extra point")
	}
	if spy.restoreCalls != 1 || !spy.lastRestoreDown {
		t.Fatal("resuming after drag did not restore the pen-down state")
	}
}

func TestSpriteDragExplicitPenDownStillDrawsPoint(t *testing.T) {
	spy := setupSpyPenMgr(t)
	co, game, sprite := setupSpriteDragTest(t)
	sprite.PenDown()
	initialDowns, initialUps := spy.penDownCalls, spy.penUpCalls
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	end := mathf.NewVec2(50, 60)
	dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
	initialEvents := len(spy.events)
	sprite.PenDown()
	if got, want := spy.events[initialEvents:], []string{"restore", "down"}; !slices.Equal(got, want) {
		t.Fatalf("explicit PenDown commands = %v, want %v", got, want)
	}
	if spy.restoreCalls != 1 || spy.lastRestore != end || !spy.lastRestoreDown {
		t.Fatalf("reset calls/position = %d/%v, want 1/%v", spy.restoreCalls, spy.lastRestore, end)
	}
	if spy.penDownCalls != initialDowns+1 || spy.penUpCalls != initialUps || !sprite.pen().isPenDown {
		t.Fatal("explicit PenDown after drag must draw one point without an implicit PenUp")
	}
	if sprite.pen().needsPositionReset {
		t.Fatal("explicit PenDown retained the pending reset")
	}
	resets := spy.restoreCalls
	sprite.SetXYpos(70, 80)
	if spy.restoreCalls != resets || spy.penDownCalls != initialDowns+1 {
		t.Fatal("movement repeated a completed pen reset or PenDown")
	}
}

func TestSpriteDragLazyClonePenRestoresDropOriginWithoutDrawingPoint(t *testing.T) {
	spy := setupSpyPenMgr(t)
	co, game, clone := setupSpriteDragTest(t)
	original := newPenTestSprite()
	original.PenDown()
	clone.components.pen = original.pen().cloneFor(clone)
	clone.spriteState.cloned = true
	if !clone.pen().isPenDown || clone.pen().penObj != nil {
		t.Fatal("clone must inherit pen-down state without allocating a drawing resource")
	}
	initialDowns, initialCreates, initialMoves := spy.penDownCalls, spy.createCalls, spy.moveCalls
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	end := mathf.NewVec2(50, 60)
	dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
	if spy.createCalls != initialCreates || spy.moveCalls != initialMoves || spy.penDownCalls != initialDowns || spy.restoreCalls != 0 {
		t.Fatal("dragging a lazy clone allocated or drew its pen")
	}
	initialEvents := len(spy.events)
	clone.SetXYpos(70, 80)
	if spy.createCalls != initialCreates+1 || spy.penDownCalls != initialDowns || spy.penUpCalls != 0 {
		t.Fatal("first clone movement must allocate its pen without an implicit PenDown or PenUp")
	}
	if spy.restoreCalls != 1 || spy.lastRestore != end || !spy.lastRestoreDown {
		t.Fatal("first clone movement did not restore its inherited pen-down state at the drop position")
	}
	if got, want := spy.events[initialEvents:], []string{"size", "color", "restore", "move"}; !slices.Equal(got, want) {
		t.Fatalf("clone pen initialization = %v, want %v", got, want)
	}
	if spy.lastMove != mathf.NewVec2(70, 80) {
		t.Fatalf("clone pen destination = %v, want (70, 80)", spy.lastMove)
	}
	clonePen := *clone.pen().penObj
	if clonePen == *original.pen().penObj || spy.lastRestoreObject != clonePen || spy.lastMoveObject != clonePen {
		t.Fatal("clone pen initialization or movement used the original's drawing resource")
	}
	if clone.pen().needsPositionReset {
		t.Fatal("first clone movement retained its pending position reset")
	}
}

func TestSpriteDragPenUpBeforeResumeKeepsDrawingStopped(t *testing.T) {
	spy := setupSpyPenMgr(t)
	co, game, sprite := setupSpriteDragTest(t)
	sprite.PenDown()
	initialDowns, initialUps := spy.penDownCalls, spy.penUpCalls
	initialMoves := spy.moveCalls
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	end := mathf.NewVec2(50, 60)
	dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
	initialEvents := len(spy.events)
	sprite.PenUp()
	sprite.SetXYpos(70, 80)
	if got, want := spy.events[initialEvents:], []string{"up"}; !slices.Equal(got, want) {
		t.Fatalf("PenUp and movement commands = %v, want %v", got, want)
	}
	if sprite.pen().needsPositionReset || sprite.pen().isPenDown || spy.restoreCalls != 0 || spy.moveCalls != initialMoves {
		t.Fatal("PenUp followed by movement resumed a pending drag stroke")
	}
	if spy.penDownCalls != initialDowns || spy.penUpCalls != initialUps+1 {
		t.Fatal("PenUp introduced an unexpected pen transition")
	}
	sprite.PenDown()
	if spy.penDownCalls != initialDowns+1 || spy.restoreCalls != 0 || spy.lastMove != mathf.NewVec2(70, 80) {
		t.Fatal("a later PenDown did not start at the sprite's current position")
	}
}

func TestSpriteDragCameraLongPressKeepsCandidateWithoutJump(t *testing.T) {
	for _, moveSprite := range []bool{false, true} {
		t.Run(fmt.Sprintf("move sprite=%v", moveSprite), func(t *testing.T) {
			co, game, sprite := setupSpriteDragTest(t)
			clock := &inputSession{}
			game.inputSession = clock
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
			camera := pkgengine.CameraMgr.(*dragCameraMgr)
			camera.world.Position.X += 100
			wantX := 10.0
			if moveSprite {
				wantX = 110
				sprite.SetXYpos(wantX, 20)
			}
			pkgengine.SpriteMgr.(*dragSpriteMgr).hits[1] = false
			clock.currentTime = .4
			dispatchSpriteDragFrame(co, game, mathf.NewVec2(110, 20))
			if !game.inputMgr.drag.active || game.inputMgr.drag.target != sprite {
				t.Fatal("camera motion lost the pressed candidate")
			}
			assertSpriteDragPosition(t, sprite, wantX, 20)
		})
	}
}

func TestSpriteDragFloat32CameraNeverCapturesAfterEmptyPress(t *testing.T) {
	co, g, sprite := setupSpriteDragTest(t)
	clock := &inputSession{}
	g.inputSession = clock
	mgr := pkgengine.SpriteMgr.(*dragSpriteMgr)
	mgr.hits[1] = false
	camera := pkgengine.CameraMgr.(*dragCameraMgr)
	camera.world.Position = mathf.NewVec2(-237.13, -180.27)
	camera.world.Size = mathf.NewVec2(480/1.37, 360/1.37)
	float32Point := func(v mathf.Vec2) mathf.Vec2 {
		return mathf.NewVec2(float64(float32(v.X)), float64(float32(v.Y)))
	}
	pointAtView := func(view mathf.Vec2) mathf.Vec2 {
		camera.world.Position = float32Point(camera.world.Position)
		camera.world.Size = float32Point(camera.world.Size)
		g.inputMgr.frame.dragProjection = g.inputMgr.sampleDragProjection()
		return float32Point(g.inputMgr.frame.dragProjection.worldPoint(view))
	}
	stationary := mathf.NewVec2(250.3, 160.7)
	start := pointAtView(stationary)
	dispatchSpriteDragFrame(co, g, start, &eventLeftButtonDown{Pos: start})
	mgr.hits[1] = true
	for i := 1; i <= 20; i++ {
		camera.world.Position = camera.world.Position.Add(mathf.NewVec2(.13, -.27))
		camera.world.Size = camera.world.Size.Mulf(1.001)
		clock.currentTime = float64(i) / 10
		dispatchSpriteDragFrame(co, g, pointAtView(stationary))
		if g.inputMgr.drag.target != nil || g.inputMgr.drag.active {
			t.Fatalf("float32 camera captured a late sprite at frame %d", i)
		}
	}
	dispatchSpriteDragFrame(co, g, pointAtView(stationary.Add(mathf.NewVec2(4, 0))))
	if g.inputMgr.drag.target != nil || g.inputMgr.drag.active {
		t.Fatal("later pointer movement captured a sprite after an empty press")
	}
	assertSpriteDragPosition(t, sprite, 10, 20)
}

// Exercise the real sampling/dispatch path without seeding drag state or
// replacing prepared events. Replay supplies button edges at the input boundary;
// this does not exercise the live engine's platform event queue.
func TestSpriteDragSamplingAndDispatch(t *testing.T) {
	for _, gesture := range []string{"first down", "same tick click", "held", "activate"} {
		t.Run(gesture, func(t *testing.T) {
			resetInputSessionTest(t)
			co, game, sprite := setupSpriteDragTest(t)
			setupInputSourceManager(t)
			frames := []InputReplayFrame{{Frame: 0, Time: 0, State: InputReplayState{Mouse: InputReplayMouse{X: 10, Y: 20}, Buttons: 1}, MouseEvents: []InputReplayMouseEvent{{Button: 1, Pressed: true}}}}
			if gesture == "same tick click" {
				frames[0].State.Buttons = 0
				frames[0].MouseEvents = append(frames[0].MouseEvents, InputReplayMouseEvent{Button: 1})
			}
			if gesture == "held" || gesture == "activate" {
				at := .1
				if gesture == "activate" {
					at = .4
				}
				frames = append(frames, InputReplayFrame{Frame: 1, Time: at, State: frames[0].State})
			}
			if _, err := PrepareInputReplay(validRuntimeReplay(frames...)); err != nil {
				t.Fatal(err)
			}
			session := claimPreparedSession(t, game)
			t.Cleanup(func() { game.abortInputSession("test finished") })
			clicks := 0
			sprite.OnClick(func() { clicks++ })
			camera := pkgengine.CameraMgr.(*dragCameraMgr)
			for i := range frames {
				sampledWorld := camera.world
				if !game.inputMgr.prepareInputFrame(.01) {
					t.Fatal("missing input frame")
				}
				projection := game.inputMgr.frame.dragProjection
				if projection == nil || projection.world != sampledWorld {
					t.Fatal("frame did not freeze its sampling projection")
				}
				if i == len(frames)-1 {
					camera.world.Position.X += 100
					if gesture == "activate" {
						sprite.SetXYpos(110, 20)
						pkgengine.SpriteMgr.(*dragSpriteMgr).hit = func(_ pkgengine.Object, point mathf.Vec2) bool { return point == mathf.NewVec2(110, 20) }
					}
				}
				game.dispatchFrameEvents()
				co.Update()
				session.endFrame()
			}
			drag := game.inputMgr.drag
			if gesture == "same tick click" {
				if drag.pressed || drag.active || drag.target != nil || clicks != 1 {
					t.Fatalf("short click retained state or lost click: drag=%+v clicks=%d", drag, clicks)
				}
				return
			}
			if drag.startView != mathf.NewVec2(250, 160) {
				t.Fatalf("press used dispatch-time camera: %v", drag.startView)
			}
			if drag.active != (gesture == "activate") {
				t.Fatalf("active=%v for %s", drag.active, gesture)
			}
			if clicks != 0 {
				t.Fatal("held press generated a click")
			}
			if gesture == "activate" {
				assertSpriteDragPosition(t, sprite, 110, 20)
			} else {
				assertSpriteDragPosition(t, sprite, 10, 20)
			}
		})
	}
}

func TestSpriteDragProjectionFallback(t *testing.T) {
	for _, dimension := range []string{"no camera", "world width", "world height", "viewport width", "viewport height"} {
		t.Run(dimension, func(t *testing.T) {
			_, game, _ := setupSpriteDragTest(t)
			camera := pkgengine.CameraMgr.(*dragCameraMgr)
			switch dimension {
			case "no camera":
				game.camera = nil
			case "world width":
				camera.world.Size.X = 0
			case "world height":
				camera.world.Size.Y = 0
			case "viewport width":
				camera.viewport.Size.X = 0
			case "viewport height":
				camera.viewport.Size.Y = 0
			}
			projection := game.inputMgr.sampleDragProjection()
			point := mathf.NewVec2(10, 20)
			if projection != nil || projection.viewportPoint(point) != point || projection.worldPoint(point) != point {
				t.Fatal("unavailable projection must preserve world input")
			}
		})
	}
}

func TestSpriteDragRejectsStaleTarget(t *testing.T) {
	// Public cancellation APIs clear drag state immediately. Also check the
	// dispatch guard when an invalid target is already retained in a snapshot.
	for _, invalid := range []string{"disabled", "hidden", "destroyed"} {
		t.Run(invalid, func(t *testing.T) {
			co, game, sprite := setupSpriteDragTest(t)
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
			switch invalid {
			case "disabled":
				sprite.spriteState.isDraggable = false
			case "hidden":
				sprite.spriteState.isVisible = false
			case "destroyed":
				sprite.markDestroyed()
			}
			dispatchSpriteDragFrame(co, game, mathf.NewVec2(30, 40))
			if game.inputMgr.drag != (spriteDragState{}) {
				t.Fatal("invalid target retained drag state")
			}
			assertSpriteDragPosition(t, sprite, 10, 20)
		})
	}
}

func setupSpriteDragTest(t *testing.T) (*coroutine.Coroutines, *Game, *SpriteImpl) {
	t.Helper()
	co, game := setupInputFrameGame(t)
	game.initShapeMgr()
	game.camera = &cameraImpl{g: game}
	previousCamera := pkgengine.CameraMgr
	pkgengine.CameraMgr = &dragCameraMgr{
		viewport: mathf.Rect2{Size: mathf.NewVec2(480, 360)},
		world:    mathf.Rect2{Position: mathf.NewVec2(-240, -180), Size: mathf.NewVec2(480, 360)},
	}
	t.Cleanup(func() { pkgengine.CameraMgr = previousCamera })
	mgr := setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{1: true, 2: true})
	pkgengine.SpriteMgr = &dragSpriteMgr{clickThroughSpriteMgr: mgr}
	sprite := newDragTestSprite(game, "dragged", 1)
	sprite.SetDragMode(Draggable)
	game.shapeMgr.add(sprite)
	return co, game, sprite
}

type dragSpriteMgr struct {
	*clickThroughSpriteMgr
	hit func(pkgengine.Object, mathf.Vec2) bool
}

func (m *dragSpriteMgr) CheckCollisionWithPoint(id pkgengine.Object, point mathf.Vec2, trigger bool) bool {
	if m.hit != nil {
		return m.hit(id, point)
	}
	return m.clickThroughSpriteMgr.CheckCollisionWithPoint(id, point, trigger)
}

type dragCameraMgr struct {
	pkgengine.ICameraMgr
	viewport, world mathf.Rect2
}

func (m *dragCameraMgr) GetViewportRect() mathf.Rect2     { return m.viewport }
func (m *dragCameraMgr) GetGlobalCameraRect() mathf.Rect2 { return m.world }

func (*dragSpriteMgr) SetTransform(pkgengine.Object, mathf.Vec2, float64, mathf.Vec2, bool, mathf.Vec2) {
}

func newDragTestSprite(game *Game, name string, id pkgengine.Object) *SpriteImpl {
	sprite := newClickTestSprite(game, name, id, false)
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{X: 10, Y: 20, RotationStyle: "normal"})
	sprite.costumes = []*costume{newCostumeWithSize(10, 10)}
	sprite.runtimeState.scale = 1
	sprite.runtimeState.syncSprite.OnAnimationLoopedEvent = &pkgengine.Event0{}
	sprite.runtimeState.syncSprite.OnAnimationFinishedEvent = &pkgengine.Event0{}
	return sprite
}

func dispatchSpriteDragFrame(co *coroutine.Coroutines, game *Game, point mathf.Vec2, events ...event) {
	game.inputMgr.frame.dragProjection = game.inputMgr.sampleDragProjection()
	game.inputMgr.mousePos = point
	game.inputMgr.frame.pending = &inputFrame{events: events}
	game.dispatchFrameEvents()
	co.Update()
}

func assertSpriteDragPosition(t *testing.T, sprite *SpriteImpl, x, y float64) {
	t.Helper()
	if gotX, gotY := sprite.getXY(); gotX != x || gotY != y {
		t.Fatalf("sprite position = (%v, %v), want (%v, %v)", gotX, gotY, x, y)
	}
}
