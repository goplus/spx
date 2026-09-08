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
	"testing"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

func TestScratchNonVisualLoopsShareFrame(t *testing.T) {
	co, _ := setupRuntimeEventGame(t)
	itime.Start(nil)
	counts := [2]int{}
	for i := range counts {
		th := co.Create(nil, func(coroutine.Thread) int {
			Repeat(5, func() { counts[i]++ })
			return 0
		})
		co.JoinYieldedOrDone(th)
	}
	co.Update()
	if counts != [2]int{5, 5} {
		t.Fatalf("same-frame iterations = %v, want [5 5]", counts)
	}
}

func TestScratchRedrawFinishesWholeRound(t *testing.T) {
	co, _ := setupRuntimeEventGame(t)
	itime.Start(nil)
	var visual, nonVisual int
	th := co.Create(nil, func(coroutine.Thread) int {
		Forever(func() {
			visual++
			engine.RequestRedraw()
		})
		return 0
	})
	co.JoinYieldedOrDone(th)
	th = co.Create(nil, func(coroutine.Thread) int {
		Forever(func() { nonVisual++ })
		return 0
	})
	co.JoinYieldedOrDone(th)
	co.Update()
	for frame := 1; frame <= 4; frame++ {
		if visual != frame || nonVisual != frame {
			t.Fatalf("frame %d: visual/nonvisual = %d/%d", frame, visual, nonVisual)
		}
		itime.Update(1.0/30, 30)
		co.Update()
	}
}

func TestScratchExplicitWaitStillCrossesFrame(t *testing.T) {
	co, _ := setupRuntimeEventGame(t)
	itime.Start(nil)
	count := 0
	th := co.Create(nil, func(coroutine.Thread) int {
		Repeat(3, func() {
			count++
			engine.Wait(0)
		})
		return 0
	})
	co.JoinYieldedOrDone(th)
	for want := 1; want <= 3; want++ {
		co.Update()
		if count != want {
			t.Fatalf("iterations = %d, want %d", count, want)
		}
		itime.Update(1.0/30, 30)
	}
}

func TestScratchSpriteVisibilityControlsRedrawBoundary(t *testing.T) {
	for _, visible := range []bool{false, true} {
		t.Run(map[bool]string{false: "hidden", true: "visible"}[visible], func(t *testing.T) {
			co, _ := setupRuntimeEventGame(t)
			itime.Start(nil)
			sprite := new(SpriteImpl)
			sprite.spriteState.IsVisible = visible
			count := 0
			th := co.Create(sprite, func(coroutine.Thread) int {
				Repeat(5, func() { count++; sprite.markProxyDirty() })
				return 0
			})
			co.JoinYieldedOrDone(th)
			co.Update()
			want := 5
			if visible {
				want = 1
			}
			if count != want {
				t.Fatalf("iterations = %d, want %d", count, want)
			}
		})
	}
}

func TestScratchTimerTrackingHatDoesNotFire(t *testing.T) {
	for _, redraw := range []bool{false, true} {
		t.Run(map[bool]string{false: "nonvisual", true: "redraw-every-round"}[redraw], func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			itime.Start(nil)
			var tracked float64
			fired := 0
			game.OnStart(func() {
				Forever(func() {
					tracked = game.Timer()
					if redraw {
						engine.RequestRedraw()
					}
				})
			})
			game.OnCond(func() bool { return game.Timer() > tracked+0.01 }, func() { fired++ })
			game.handleEvent(&eventStart{generation: game.currentBootstrapGeneration()})
			co.Update()
			// Slow frames must preserve the same sampling order.
			for _, delta := range []float64{1.0 / 30, 1.0 / 60, 0.25, 1.0 / 30} {
				game.OnEngineBeforeUpdate()
				itime.Update(delta, 30)
				game.scriptEvents.dispatchConditions()
				co.Update()
			}
			if fired != 0 {
				t.Fatalf("tracking hat fired %d times", fired)
			}
			// Stopping tracking allows a timer edge to fire.
			co.StopIf(func(th coroutine.Thread) bool { return th.Obj == game })
			game.OnEngineBeforeUpdate()
			itime.Update(0.1, 30)
			game.scriptEvents.dispatchConditions()
			co.Update()
			game.OnEngineBeforeUpdate()
			itime.Update(0.1, 30)
			game.scriptEvents.dispatchConditions()
			co.Update()
			if fired != 1 {
				t.Fatalf("stopped tracking hat fired %d times, want 1", fired)
			}
		})
	}
}

func TestScratchConditionSamplesBeforeClockAndHandlerUsesNewClock(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	itime.Start(nil)
	game.lifecycleState.StartDispatched.Store(true)
	var sampled, handled float64
	game.OnCond(func() bool { sampled = game.Timer(); return sampled > 0.01 }, func() { handled = game.Timer() })
	itime.Update(0.05, 30)
	game.OnEngineBeforeUpdate()
	if handled != 0 {
		t.Fatal("handler ran in predicate phase")
	}
	itime.Update(0.05, 30)
	game.scriptEvents.dispatchConditions()
	co.Update()
	if sampled != 0.05 || handled != 0.1 {
		t.Fatalf("predicate/handler time = %v/%v, want 0.05/0.1", sampled, handled)
	}
}

func TestScratchConditionDoesNotReenterRunningHandler(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	itime.Start(nil)
	game.lifecycleState.StartDispatched.Store(true)
	value := true
	evaluations, calls := 0, 0
	release := make(chan struct{}, 2)
	completed := make(chan struct{}, 2)
	game.OnCond(func() bool { evaluations++; return value }, func() {
		calls++
		defer func() { completed <- struct{}{} }()
		var signal struct{}
		engine.WaitForChan(release, &signal)
	})
	poll := func() {
		game.OnEngineBeforeUpdate()
		itime.Update(1.0/30, 30)
		game.scriptEvents.dispatchConditions()
		co.Update()
	}
	poll()
	value = false
	poll()
	value = true
	poll()
	if calls != 1 || evaluations != 1 {
		t.Fatalf("running hat: calls/evaluations = %d/%d, want 1/1", calls, evaluations)
	}
	release <- struct{}{}
	<-completed
	poll()
	if calls != 1 {
		t.Fatal("sustained true condition restarted a completed handler")
	}
	value = false
	poll()
	value = true
	poll()
	if calls != 2 {
		t.Fatalf("new rising edge calls = %d, want 2", calls)
	}
	release <- struct{}{}
}
