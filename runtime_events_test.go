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
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/core/clock"
	coreevent "github.com/goplus/spx/v4/internal/core/event"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	"github.com/goplus/spx/v4/internal/engine/wrap"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type clickThroughSpriteMgr struct {
	wrap.SpriteMgrImpl
	hits          map[pkgengine.Object]bool
	lastIsTrigger map[pkgengine.Object]bool
}

type disappearingClickTarget struct {
	proxy      engine.Sprite
	proxyReads int
	clicks     int
}

type touchEventSprite struct{ SpriteImpl }

func TestStopFromExternalCallerDoesNotExcludeActiveCoroutine(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	started := make(chan struct{})
	release := make(chan struct{})
	thread := currentScheduler.Create(game, func(coroutine.Thread) {
		close(started)
		<-release
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("active coroutine did not start")
	}

	game.Stop(AllOtherScripts)
	if !thread.Stopped() {
		t.Fatal("external Stop(AllOtherScripts) did not stop active coroutine")
	}
	close(release)
	currentScheduler.Join(thread)
}

func TestOnStartFromCoroutineWaitsForFirstSlices(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	game.debugState.DebugEvent = true
	release := currentScheduler.NewLatch()

	var order []string
	completed := 0
	for _, name := range []string{"first", "second"} {
		game.OnStart(func() {
			order = append(order, name)
			release.Wait()
			completed++
		})
	}

	dispatchedOnReturn := false
	currentScheduler.Create(game, func(coroutine.Thread) {
		game.handleEvent(&eventStart{generation: game.lifecycle.generation()})
		dispatchedOnReturn = game.lifecycle.startDispatched.Load()
		order = append(order, "caller")
	})
	currentScheduler.Update()

	if want := []string{"first", "second", "caller"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("startup order = %v, want %v", order, want)
	}
	if !dispatchedOnReturn {
		t.Fatal("caller resumed before start dispatch was marked complete")
	}
	if completed != 0 {
		t.Fatalf("completed handlers = %d, want 0 before release", completed)
	}

	release.Open()
	currentScheduler.Update()
	if completed != 2 {
		t.Fatalf("completed handlers = %d, want 2 after release", completed)
	}
}

func TestOnStartUsesTargetOrder(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	game.scene.init()

	newSprite := func(name string) *SpriteImpl {
		sprite := &SpriteImpl{name: name, g: game}
		sprite.scriptEventBindings.bind(&game.scriptEvents, sprite)
		return sprite
	}

	laser := newSprite("Laser")
	transition := newSprite("Transition")
	levels := newSprite("Levels")
	game.scene.add(laser)
	game.scene.add(transition)
	game.scene.add(levels)
	transition.spriteState.isVisible = true

	level := 7.0
	var order []string
	laser.OnStart(func() {
		order = append(order, "laser-stop-check")
		if level == 7 {
			laser.Stop(AllStop)
		}
	})
	laser.OnStart(func() {
		order = append(order, "laser-hide")
	})
	transition.OnStart(func() {
		order = append(order, "transition-hide")
		transition.Hide()
	})
	levels.OnStart(func() {
		order = append(order, "levels-init")
		level = 1
	})

	game.handleEvent(&eventStart{generation: game.lifecycle.generation()})
	currentScheduler.Update()

	want := []string{"levels-init", "transition-hide", "laser-stop-check", "laser-hide"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("OnStart order = %v, want %v", order, want)
	}
	if level != 1 || transition.Visible() {
		t.Fatalf("startup state = level %v, transition visible %v, want 1/false", level, transition.Visible())
	}
	if !game.lifecycle.startDispatched.Load() {
		t.Fatal("start event was not marked dispatched")
	}
}

func TestOnTimerMatchesMilliseconds(t *testing.T) {
	previous := scheduler
	scheduler = nil
	clock.Start()
	clock.ClearTimers()
	t.Cleanup(func() {
		scheduler = previous
		clock.ClearTimers()
	})

	var game Game
	game.bindScriptEvents()
	times := []float64{0.5009, 0.5011, 0.5001, 1.0019, 1.0021}
	calls := make([]int, len(times))
	for i, at := range times {
		game.OnTimer(at, func() { calls[i]++ })
	}
	game.OnTimer(0.5, nil)

	for round := 1; round <= 2; round++ {
		clock.Update(2, 60)
		for {
			at, ok := clock.NextTimer()
			if !ok {
				break
			}
			game.handleEvent(&eventTimer{Timestamp: at})
		}
		for i, got := range calls {
			if got != round {
				t.Errorf("timer %v after round %d: %d calls, want %d", times[i], round, got, round)
			}
		}
		clock.ResetTimer()
	}
}

func TestOnCondRisingEdgeAndOwnerIsolation(t *testing.T) {
	currentScheduler := setupRuntimeScheduler(t)

	var registry scriptEventRegistry
	var left, right scriptEventBindings
	left.bind(&registry, "left")
	right.bind(&registry, "right")

	var (
		leftValue, rightValue bool
		leftEvaluations       int
		rightEvaluations      int
		calls                 []string
	)
	left.OnCond(func() bool {
		leftEvaluations++
		return leftValue
	}, func() {
		calls = append(calls, "left")
	})
	right.OnCond(func() bool {
		rightEvaluations++
		return rightValue
	}, func() {
		calls = append(calls, "right")
	})

	states := [][2]bool{
		{false, false},
		{true, false},
		{true, true},
		{false, true},
		{true, false},
	}
	for _, state := range states {
		leftValue, rightValue = state[0], state[1]
		pollRuntimeConditions(&registry)
		currentScheduler.Update()
	}

	if want := []string{"left", "right", "left"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("condition calls = %v, want %v", calls, want)
	}
	if leftEvaluations != len(states) || rightEvaluations != len(states) {
		t.Fatalf("condition evaluations = %d/%d, want %d/%d", leftEvaluations, rightEvaluations, len(states), len(states))
	}

	left.clearHandlers()
	leftValue = false
	rightValue = false
	pollRuntimeConditions(&registry)
	leftValue = true
	pollRuntimeConditions(&registry)
	currentScheduler.Update()
	if leftEvaluations != len(states) {
		t.Fatalf("deleted owner evaluated %d times, want %d", leftEvaluations, len(states))
	}
	if want := []string{"left", "right", "left"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls after owner deletion = %v, want %v", calls, want)
	}
}

func TestOnCondIgnoresNilCallbacks(t *testing.T) {
	var game Game
	game.bindScriptEvents()
	game.OnCond(nil, func() {})
	game.OnCond(func() bool { return true }, nil)

	if got := game.scriptEvents.manager.Snapshot(coreevent.BucketCondition); len(got) != 0 {
		t.Fatalf("condition sinks = %d, want 0", len(got))
	}
}

func TestOnCondStartsAfterOnStartPhase(t *testing.T) {
	currentScheduler, game, _ := setupInputSourceTest(t)
	var order []string
	evaluations := 0
	game.OnCond(func() bool {
		evaluations++
		order = append(order, "evaluate")
		return true
	}, func() {
		order = append(order, "condition")
	})
	game.OnStart(func() {
		order = append(order, "start")
	})

	pollRuntimeEventConditions(game)
	if evaluations != 0 {
		t.Fatalf("condition evaluated %d times during bootstrap, want 0", evaluations)
	}

	game.lifecycle.markLoaded(game.lifecycle.generation())
	game.lifecycle.completeBootstrap(game.lifecycle.generation())
	pollRuntimeEventConditions(game)
	if evaluations != 0 {
		t.Fatalf("condition evaluated %d times before OnStart, want 0", evaluations)
	}

	game.dispatchStartEventIfNeeded()
	updateRuntimeEventSchedulerUntil(t, currentScheduler, game.lifecycle.startDispatched.Load)
	if want := []string{"start"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order before condition polling = %v, want %v", order, want)
	}

	pollRuntimeEventConditions(game)
	currentScheduler.Update()
	if evaluations != 1 {
		t.Fatalf("condition evaluated %d times after OnStart, want 1", evaluations)
	}
	if want := []string{"start", "evaluate", "condition"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("startup order = %v, want %v", order, want)
	}

	pollRuntimeEventConditions(game)
	currentScheduler.Update()
	if evaluations != 2 {
		t.Fatalf("condition evaluated %d times after two polls, want 2", evaluations)
	}
	if want := []string{"start", "evaluate", "condition", "evaluate"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("sustained-true order = %v, want %v", order, want)
	}
}

func TestOnCondObservesTopLevelAndOnStartInitialization(t *testing.T) {
	currentScheduler, game, _ := setupInputSourceTest(t)
	var order []string
	ready := false
	topLevelBlocked := make(chan struct{})
	onStartBlocked := make(chan struct{})

	generation := game.lifecycle.generation()
	game.lifecycle.markLoaded(generation)
	game.lifecycle.queue(generation, func() {
		runMainUntilYield(game, func() {
			order = append(order, "top-level")
			game.OnCond(func() bool {
				order = append(order, "evaluate")
				return ready
			}, func() {
				order = append(order, "condition")
			})
			game.OnStart(func() {
				ready = true
				order = append(order, "start")
				engine.WaitForChan(onStartBlocked)
				order = append(order, "start-resumed")
			})

			engine.WaitForChan(topLevelBlocked)
			order = append(order, "top-level-resumed")
		})
	})

	game.lifecycle.runTasks(generation)
	if want := []string{"top-level"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order after top-level first yield = %v, want %v", order, want)
	}
	pollRuntimeEventConditions(game)
	if want := []string{"top-level"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("condition ran before OnStart: %v", order)
	}

	game.lifecycle.completeBootstrap(generation)
	game.dispatchStartEventIfNeeded()
	updateRuntimeEventSchedulerUntil(t, currentScheduler, game.lifecycle.startDispatched.Load)
	if want := []string{"top-level", "start"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order after OnStart = %v, want %v", order, want)
	}

	pollRuntimeEventConditions(game)
	currentScheduler.Update()
	if want := []string{"top-level", "start", "evaluate", "condition"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("startup order = %v, want %v", order, want)
	}

	close(topLevelBlocked)
	close(onStartBlocked)
	currentScheduler.Update()
}

func TestOnStartIgnoresStaleBootstrapGeneration(t *testing.T) {
	currentScheduler := setupRuntimeScheduler(t)

	var game Game
	game.bindScriptEvents()
	stale := game.lifecycle.generation()
	game.lifecycle.reset()

	calls := 0
	game.OnStart(func() { calls++ })
	game.handleEvent(&eventStart{generation: stale})
	currentScheduler.Update()
	if calls != 0 || game.lifecycle.startDispatched.Load() {
		t.Fatal("stale start event affected the current lifecycle")
	}

	game.handleEvent(&eventStart{generation: game.lifecycle.generation()})
	updateRuntimeEventSchedulerUntil(t, currentScheduler, game.lifecycle.startDispatched.Load)
	if calls != 1 {
		t.Fatalf("current start calls = %d, want 1", calls)
	}
}

func TestOnStartWithoutSchedulerDoesNotCrossReset(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	scheduler = nil
	defer func() { scheduler = currentScheduler }()

	var calls []string
	game.OnStart(func() {
		calls = append(calls, "first")
		game.lifecycle.reset()
	})
	game.OnStart(func() { calls = append(calls, "stale") })
	game.handleEvent(&eventStart{generation: game.lifecycle.generation()})
	if want := []string{"first"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("start callbacks after reset = %v, want %v", calls, want)
	}
	if game.lifecycle.startDispatched.Load() {
		t.Fatal("stale OnStart completion reopened the start gate")
	}
}

func TestOnStartCompletionDoesNotCrossReset(t *testing.T) {
	currentScheduler := setupRuntimeScheduler(t)

	var game Game
	game.bindScriptEvents()
	entered := make(chan struct{})
	release := make(chan struct{})
	game.OnStart(func() {
		close(entered)
		<-release
	})
	staleCalls := 0
	game.OnStart(func() { staleCalls++ })

	game.handleEvent(&eventStart{generation: game.lifecycle.generation()})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("OnStart did not begin")
	}

	game.lifecycle.reset()
	game.scriptEvents.manager.Reset()
	close(release)
	currentScheduler.Update()
	if game.lifecycle.startDispatched.Load() {
		t.Fatal("stale OnStart completion reopened the start gate")
	}
	if staleCalls != 0 {
		t.Fatalf("stale pending OnStart calls = %d, want 0", staleCalls)
	}

	calls := 0
	game.OnStart(func() { calls++ })
	game.handleEvent(&eventStart{generation: game.lifecycle.generation()})
	updateRuntimeEventSchedulerUntil(t, currentScheduler, game.lifecycle.startDispatched.Load)
	if calls != 1 {
		t.Fatalf("new lifecycle start calls = %d, want 1", calls)
	}
}

func TestDistinctNestedAsyncBroadcastRunsReceiverBatchInCurrentFrame(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)

	var calls []string
	game.OnMsg__1("outer", func() {
		calls = append(calls, "outer")
		engine.RequestRedraw()
		game.Broadcast__0("inner")
	})
	game.OnMsg__1("inner", func() {
		calls = append(calls, "inner-1")
	})
	game.OnMsg__1("inner", func() {
		calls = append(calls, "inner-2")
	})

	game.Broadcast__0("outer")
	currentScheduler.Update()
	want := []string{"outer", "inner-1", "inner-2"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls in initial frame = %v, want %v", calls, want)
	}

	clock.Update(0, 0)
	currentScheduler.Update()
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls after next frame = %v, want no deferred handlers", calls)
	}
}

func TestSelfBroadcastWaitsForNextFrame(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)

	calls := 0
	game.OnMsg__1("tick", func() {
		calls++
		if calls < 3 {
			game.Broadcast__0("tick")
		}
	})

	game.Broadcast__0("tick")
	for frame := 1; frame <= 3; frame++ {
		if frame > 1 {
			clock.Update(0, 0)
		}
		currentScheduler.Update()
		if calls != frame {
			t.Fatalf("calls after frame %d = %d, want %d", frame, calls, frame)
		}
	}
}

func TestCatchAllMessageReceiverCannotReenterWithAnotherMessage(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)

	messages := []MsgName{"m0", "m1", "m2"}
	var calls []MsgName
	game.OnMsg__0(func(msg MsgName, _ any) {
		calls = append(calls, msg)
		engine.RequestRedraw()
		if len(calls) < len(messages) {
			game.Broadcast__0(messages[len(calls)])
		}
	})

	game.Broadcast__0(messages[0])
	for frame := 1; frame <= len(messages); frame++ {
		if frame > 1 {
			clock.Update(0, 0)
		}
		currentScheduler.Update()
		if want := messages[:frame]; !reflect.DeepEqual(calls, want) {
			t.Fatalf("calls after frame %d = %v, want %v", frame, calls, want)
		}
	}
}

func TestRepeatedReceiverDoesNotDeferNewSibling(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)

	var calls []string
	game.OnMsg__0(func(msg MsgName, _ any) {
		calls = append(calls, "any-"+string(msg))
		if msg == "outer" {
			engine.RequestRedraw()
			game.Broadcast__0("inner")
		}
	})
	game.OnMsg__1("inner", func() {
		calls = append(calls, "inner-new")
	})

	game.Broadcast__0("outer")
	currentScheduler.Update()
	want := []string{"any-outer", "inner-new"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls in initial frame = %v, want %v", calls, want)
	}

	clock.Update(0, 0)
	currentScheduler.Update()
	want = append(want, "any-inner")
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls after receiver restart = %v, want %v", calls, want)
	}
}

func TestNestedAsyncBroadcastCycleYieldsAtRoundBoundary(t *testing.T) {
	for _, test := range []struct {
		name          string
		requestRedraw bool
	}{
		{name: "redraw requested", requestRedraw: true},
		{name: "without loop continuation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			currentScheduler, game := setupRuntimeEventGame(t)
			var calls []string
			game.OnMsg__1("a", func() {
				calls = append(calls, "a")
				if test.requestRedraw {
					engine.RequestRedraw()
				}
				if len(calls) < 8 {
					game.Broadcast__0("b")
				}
			})
			game.OnMsg__1("b", func() {
				calls = append(calls, "b")
				if len(calls) < 8 {
					game.Broadcast__0("a")
				}
			})

			game.Broadcast__0("a")
			for frame := 0; frame < 4; frame++ {
				if frame > 0 {
					clock.Update(0, 0)
				}
				currentScheduler.Update()
				want := []string{"a", "b", "a", "b", "a", "b", "a", "b"}[:2*(frame+1)]
				if !reflect.DeepEqual(calls, want) {
					t.Fatalf("calls after frame %d = %v, want %v", frame, calls, want)
				}
				if test.requestRedraw && frame < 3 && currentScheduler.UpdateStats().DeferredCount == 0 {
					t.Fatalf("recursive receiver was not deferred after frame %d", frame)
				}
			}
		})
	}
}

func TestNestedAsyncBroadcastCycleHonorsScriptRoundBudget(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	currentScheduler, game := setupRuntimeEventGame(t)
	driver := currentScheduler.Create(nil, func(me coroutine.Thread) {
		for {
			currentScheduler.YieldLoopFor(me)
		}
	})
	currentScheduler.JoinFirstSlice(driver)

	calls := 0
	game.OnMsg__1("a", func() {
		calls++
		game.Broadcast__0("b")
	})
	game.OnMsg__1("b", func() {
		calls++
		game.Broadcast__0("a")
	})

	game.Broadcast__0("a")
	started := time.Now()
	currentScheduler.Update()
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("recursive broadcast update took %v, want script-round budget to stop it", elapsed)
	}
	if calls < 4 {
		t.Fatalf("recursive broadcast calls = %d, want multiple script rounds", calls)
	}
	if currentScheduler.UpdateStats().DeferredCount == 0 {
		t.Fatal("recursive broadcast did not leave its next round queued")
	}
}

func TestNestedAsyncBroadcastCycleResetsContextAtRoundBoundary(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	driver := currentScheduler.Create(nil, func(me coroutine.Thread) {
		currentScheduler.YieldLoopFor(me)
	})
	currentScheduler.JoinFirstSlice(driver)

	var calls []string
	game.OnMsg__1("a", func() {
		calls = append(calls, "a")
		if len(calls) == 3 {
			engine.RequestRedraw()
		}
		if len(calls) < 4 {
			game.Broadcast__0("b")
		}
	})
	game.OnMsg__1("b", func() {
		calls = append(calls, "b")
		if len(calls) < 3 {
			game.Broadcast__0("a")
		}
	})

	game.Broadcast__0("a")
	currentScheduler.Update()
	want := []string{"a", "b", "a", "b"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls after second script round = %v, want %v", calls, want)
	}
}

func TestNestedBroadcastChainExpiresAfterIndependentLoopRound(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)

	var calls []string
	aCalls := 0
	game.OnMsg__1("a", func() {
		aCalls++
		calls = append(calls, "a")
		if aCalls == 1 {
			game.Broadcast__0("b")
		}
	})
	game.OnMsg__1("b", func() {
		calls = append(calls, "b-before-yield")
		scheduler.YieldLoopFor(scheduler.Current())
		calls = append(calls, "b-after-yield")
		engine.RequestRedraw()
		game.Broadcast__0("a")
	})

	game.Broadcast__0("a")
	currentScheduler.Update()
	want := []string{"a", "b-before-yield", "b-after-yield", "a"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls across same-frame rounds = %v, want %v", calls, want)
	}
}

func TestAsyncBroadcastAfterFrameWaitRunsWithoutAnotherDelay(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)

	var calls []string
	game.OnMsg__1("outer", func() {
		calls = append(calls, "outer-before-wait")
		engine.WaitNextFrame()
		calls = append(calls, "outer-after-wait")
		game.Broadcast__0("inner")
	})
	game.OnMsg__1("inner", func() {
		calls = append(calls, "inner")
	})

	game.Broadcast__0("outer")
	currentScheduler.Update()
	if want := []string{"outer-before-wait"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls in initial frame = %v, want %v", calls, want)
	}

	clock.Update(0, 0)
	currentScheduler.Update()
	if want := []string{"outer-before-wait", "outer-after-wait", "inner"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls after handler resumed = %v, want %v", calls, want)
	}
}

func TestSelfBroadcastAfterFrameWaitNeedsAnotherRound(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)

	var calls []string
	game.OnMsg__1("self", func() {
		if len(calls) == 0 {
			calls = append(calls, "before-wait")
			engine.WaitNextFrame()
			calls = append(calls, "after-wait")
			game.Broadcast__0("self")
			return
		}
		calls = append(calls, "restarted")
	})

	game.Broadcast__0("self")
	currentScheduler.Update()
	if want := []string{"before-wait"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls in initial frame = %v, want %v", calls, want)
	}

	clock.Update(0, 0)
	currentScheduler.Update()
	if want := []string{"before-wait", "after-wait"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls after handler resumed = %v, want %v", calls, want)
	}

	clock.Update(0, 0)
	currentScheduler.Update()
	want := []string{"before-wait", "after-wait", "restarted"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls after restart round = %v, want %v", calls, want)
	}
}

func TestSelfBroadcastAfterFrameWaitWithRedrawDoesNotRunInSameFrame(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)

	var calls []string
	game.OnMsg__1("self-redraw", func() {
		switch len(calls) {
		case 0:
			calls = append(calls, "before-wait")
			engine.WaitNextFrame()
			calls = append(calls, "after-wait")
			engine.RequestRedraw()
			game.Broadcast__0("self-redraw")
		case 2:
			calls = append(calls, "restarted")
		default:
			calls = append(calls, "unexpected")
		}
	})

	game.Broadcast__0("self-redraw")
	currentScheduler.Update()
	if want := []string{"before-wait"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls in initial frame = %v, want %v", calls, want)
	}

	clock.Update(0, 0)
	currentScheduler.Update()
	if want := []string{"before-wait", "after-wait"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls after resumed frame = %v, want %v", calls, want)
	}

	clock.Update(0, 0)
	currentScheduler.Update()
	want := []string{"before-wait", "after-wait", "restarted"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls after redraw boundary = %v, want %v", calls, want)
	}
}

func TestNestedBroadcastAndWaitCompletesInCurrentFrame(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)

	var calls []string
	game.OnMsg__1("outer", func() {
		calls = append(calls, "outer-before")
		game.BroadcastAndWait__0("inner")
		calls = append(calls, "outer-after")
	})
	game.OnMsg__1("inner", func() {
		calls = append(calls, "inner")
	})

	game.Broadcast__0("outer")
	currentScheduler.Update()
	want := []string{"outer-before", "inner", "outer-after"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("synchronous nested broadcast calls = %v, want %v", calls, want)
	}

	clock.Update(0, 0)
	currentScheduler.Update()
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("synchronous broadcast left deferred handlers: calls = %v, want %v", calls, want)
	}
}

func TestRecursiveBroadcastAndWaitYieldsAtRoundBoundary(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)

	calls := 0
	game.OnMsg__1("self", func() {
		calls++
		engine.RequestRedraw()
		if calls < 3 {
			game.BroadcastAndWait__0("self")
		}
	})

	game.Broadcast__0("self")
	for frame := 1; frame <= 3; frame++ {
		if frame > 1 {
			clock.Update(0, 0)
		}
		currentScheduler.Update()
		if calls != frame {
			t.Fatalf("calls after frame %d = %d, want %d", frame, calls, frame)
		}
	}
}

func TestGameRunBootstrapTasksExecutesQueuedHooksOnce(t *testing.T) {
	var g Game
	generation := g.lifecycle.generation()

	var got []string
	g.lifecycle.queue(generation, func() {
		got = append(got, "first")
	})
	g.lifecycle.queue(generation, func() {
		got = append(got, "second")
	})

	g.lifecycle.runTasks(generation)
	g.lifecycle.runTasks(generation)

	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runBootstrapTasks got %v, want %v", got, want)
	}
}

func TestFindClickTargetRespectsLayerOrderAndVisibility(t *testing.T) {
	for _, test := range []struct {
		name       string
		handler    bool
		moveBack   bool
		ghost      bool
		wantBottom bool
	}{
		{name: "topmost without handler"},
		{name: "topmost with handler", handler: true},
		{name: "changed layer order", handler: true, moveBack: true, wantBottom: true},
		{name: "fully ghosted with handler", handler: true, ghost: true, wantBottom: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{1: true, 2: true})
			var game Game
			game.scene.init()
			bottom := newClickTestSprite(&game, "bottom", 1, true)
			top := newClickTestSprite(&game, "top", 2, test.handler)
			if test.ghost {
				top.graphicEffects = map[EffectKind]float64{GhostEffect: 100}
			}
			game.scene.add(bottom)
			game.scene.add(top)
			if test.moveBack {
				top.SetLayerTo(Back)
			}
			want := top
			if test.wantBottom {
				want = bottom
			}
			if target := game.findClickTarget(mathf.NewVec2(0, 0)); target != want {
				t.Fatalf("target = %p, want %p", target, want)
			}
		})
	}
}

func TestTouchPointUsesSensingQuery(t *testing.T) {
	mgr := setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{
		1: true,
	})

	var g Game
	sprite := newClickTestSprite(&g, "probe", 1, false)
	sprite.spriteState.isVisible = false

	if !sprite.touchPoint(12, 34) {
		t.Fatal("expected touchPoint hit")
	}
	if got := mgr.lastIsTrigger[1]; got {
		t.Fatalf("touchPoint used click query, want sensing query")
	}
}

func TestPointHitsClickTargetUsesClickQuery(t *testing.T) {
	mgr := setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{
		1: true,
	})

	var g Game
	sprite := newClickTestSprite(&g, "button", 1, true)

	if !g.pointHitsClickTarget(sprite, mathf.NewVec2(0, 0)) {
		t.Fatal("expected click hit")
	}
	if got := mgr.lastIsTrigger[1]; !got {
		t.Fatalf("pointHitsClickTarget used sensing query, want click query")
	}
}

func TestLeftButtonDownRoutesClicksAfterStartingSwipe(t *testing.T) {
	previous := scheduler
	scheduler = nil
	t.Cleanup(func() { scheduler = previous })

	for _, tt := range []struct {
		name       string
		hit        bool
		noHandler  bool
		blocked    []pkgengine.Object
		wantClicks []string
		wantGates  int
	}{
		{name: "target", hit: true, wantClicks: []string{"target"}, wantGates: 2},
		{name: "target without handler", hit: true, noHandler: true, wantGates: 2},
		{name: "stage", wantClicks: []string{"stage"}, wantGates: 2},
		{name: "global blocks target", hit: true, blocked: []pkgengine.Object{clickTimerGlobal}, wantGates: 1},
		{name: "target blocked", hit: true, blocked: []pkgengine.Object{7}, wantGates: 2},
		{name: "global blocks stage", blocked: []pkgengine.Object{clickTimerGlobal}, wantGates: 1},
		{name: "stage blocked", blocked: []pkgengine.Object{clickTimerStage}, wantGates: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{7: tt.hit})
			var game Game
			game.scene.init()
			game.bindScriptEvents()
			game.inputMgr.g = &game
			now := time.Unix(100, 0)
			var steps, clicks, swipes []string
			game.inputMgr.clickGate.InitWithClock(mouseClickInterval, func() time.Time {
				steps = append(steps, "gate")
				return now
			})
			game.inputMgr.swipe.InitWithClock(func() time.Time {
				steps = append(steps, "swipe")
				return now
			})
			sprite := newClickTestSprite(&game, "target", 7, false)
			if !tt.noHandler {
				sprite.OnClick(func() { clicks = append(clicks, "target") })
			}
			sprite.OnSwipe__0(90, func() { swipes = append(swipes, "target") })
			game.scene.add(sprite)
			game.OnClick(func() { clicks = append(clicks, "stage") })
			game.OnSwipe__0(90, func() { swipes = append(swipes, "stage") })
			for _, id := range tt.blocked {
				game.inputMgr.clickGate.Allow(id)
			}
			steps = nil

			game.handleEvent(&eventLeftButtonDown{Pos: mathf.Vec2{X: 10, Y: 20}})
			wantSteps := []string{"swipe"}
			for range tt.wantGates {
				wantSteps = append(wantSteps, "gate")
			}
			if !reflect.DeepEqual(steps, wantSteps) {
				t.Fatalf("dispatch steps = %v, want %v", steps, wantSteps)
			}
			if !reflect.DeepEqual(clicks, tt.wantClicks) {
				t.Fatalf("clicks = %v, want %v", clicks, tt.wantClicks)
			}
			// A blocked global gate must not consume the target/stage gate.
			routedID := pkgengine.Object(clickTimerStage)
			if tt.hit {
				routedID = 7
			}
			if allowed := game.inputMgr.clickGate.Allow(routedID); allowed != (tt.wantGates == 1) {
				t.Fatalf("routed gate remains available = %v, want %v", allowed, tt.wantGates == 1)
			}
			// The global gate is always consumed, including when the routed gate blocks.
			if game.inputMgr.clickGate.Allow(clickTimerGlobal) {
				t.Fatal("global gate was not consumed before routing")
			}
			now = now.Add(100 * time.Millisecond)
			game.handleEvent(&eventLeftButtonUp{Pos: mathf.Vec2{X: 110, Y: 20}})
			wantSwipe := "stage"
			if tt.hit {
				wantSwipe = "target"
			}
			if !reflect.DeepEqual(swipes, []string{wantSwipe}) {
				t.Fatalf("swipes = %v, want [%s]", swipes, wantSwipe)
			}
		})
	}
}

func TestLeftButtonDownDoesNotFallThroughWhenTargetProxyDisappears(t *testing.T) {
	previous := scheduler
	scheduler = nil
	t.Cleanup(func() { scheduler = previous })
	setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{7: true})

	for _, blocked := range []bool{false, true} {
		var game Game
		game.scene.init()
		game.bindScriptEvents()
		game.inputMgr.init(&game)
		game.inputMgr.clickGate.InitWithClock(mouseClickInterval, func() time.Time {
			return time.Unix(100, 0)
		})
		if blocked {
			game.inputMgr.clickGate.Allow(clickTimerGlobal)
		}
		target := &disappearingClickTarget{}
		target.proxy.SetId(7)
		game.scene.add(target)
		stageClicks := 0
		game.OnClick(func() { stageClicks++ })

		game.handleEvent(&eventLeftButtonDown{})
		wantReads := 2
		if blocked {
			wantReads = 1
		}
		if target.proxyReads != wantReads || target.clicks != 0 || stageClicks != 0 {
			t.Fatalf("global blocked %v: proxy reads / target clicks / stage clicks = %d / %d / %d, want %d / 0 / 0", blocked, target.proxyReads, target.clicks, stageClicks, wantReads)
		}
		if !game.inputMgr.clickGate.Allow(clickTimerStage) {
			t.Fatalf("global blocked %v: missing target proxy consumed the stage gate", blocked)
		}
	}
}

func TestTouchStartGate(t *testing.T) {
	previous := scheduler
	scheduler = nil
	defer func() { scheduler = previous }()

	game := new(Game)
	game.bindScriptEvents()
	source := newTouchEventSprite(game, "source")
	target := newTouchEventSprite(game, "target")
	source.doWhenTouchStart(&target.SpriteImpl)
	if source.spriteState.hasOnTouchStart {
		t.Fatal("touch without a handler enabled dispatch")
	}

	calls := 0
	source.addTouchStartHandler(func(Sprite) { calls++ })
	if !source.spriteState.hasOnTouchStart {
		t.Fatal("registered touch handler did not enable dispatch")
	}
	source.doWhenTouchStart(&target.SpriteImpl)
	if calls != 1 {
		t.Fatalf("registered touch calls = %d, want 1", calls)
	}

	clone := newTouchEventSprite(game, "source")
	clone.SpriteImpl.InitFrom(&source.SpriteImpl)
	clone.doWhenTouchStart(&target.SpriteImpl)
	if clone.spriteState.hasOnTouchStart || calls != 1 {
		t.Fatal("clone inherited the source handler gate")
	}
	clone.addTouchStartHandler(func(Sprite) { calls++ })
	if !clone.spriteState.hasOnTouchStart {
		t.Fatal("clone touch handler did not enable dispatch")
	}
	clone.doWhenTouchStart(&target.SpriteImpl)
	if calls != 2 {
		t.Fatalf("clone touch calls = %d, want 2", calls)
	}
	if got := len(game.scriptEvents.manager.Snapshot(coreevent.BucketTouchStart)); got != 2 {
		t.Fatalf("touch handlers = %d, want 2", got)
	}
}

func TestDebugEventBindingsAndBroadcastOverloads(t *testing.T) {
	_, game := setupRuntimeEventGame(t)
	previous := scheduler
	scheduler = nil
	t.Cleanup(func() { scheduler = previous })
	game.debugState.DebugEvent = true
	game.debugState.DebugInstr = true
	sprite := &SpriteImpl{g: game, name: "actor"}
	sprite.scriptEventBindings.bind(&game.scriptEvents, sprite)
	game.scene.add(sprite)
	var got []string
	game.OnTimer(0, func() { got = append(got, "timer") })
	game.OnKey__0(KeyA, func() { got = append(got, "key") })
	game.OnKey__1([]Key{KeyA}, func(Key) { got = append(got, "keys") })
	game.OnKey__1(nil, func(Key) { t.Error("empty key set invoked") })
	game.OnSwipe__0(90, func() { got = append(got, "swipe") })
	game.OnMsg__1("message", func() { got = append(got, "message") })
	game.OnBackdrop__1("scene", func() { got = append(got, "backdrop") })
	game.OnClick(func() { got = append(got, "click") })
	sprite.OnCloned__1(func(any) { got = append(got, "clone") })
	sprite.addTouchStartHandler(func(Sprite) { got = append(got, "touch") })
	game.doWhenTimer(0)
	game.doWhenKeyPressed(KeyA)
	game.doWhenSwipe(90)
	game.Broadcast__1("message", 123)
	game.BroadcastAndWait__1("message", 456)
	game.doWhenBackdropChanged("scene", true)
	game.doWhenClick()
	sprite.doWhenCloned(nil)
	sprite.doWhenTouchStart(&SpriteImpl{name: "peer"})
	want := []string{"timer", "key", "keys", "swipe", "message", "message", "backdrop", "click", "clone", "touch"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("debug handlers = %v, want %v", got, want)
	}
	game.scriptEvents.manager.SnapshotStartOnce()
	sprite.spriteState.cloned = true
	sprite.OnStart(func() { t.Error("late clone start ran") })
	game.OnStart(func() { t.Error("late game start ran") })
	if got := len(game.scriptEvents.manager.Snapshot(coreevent.BucketStart)); got != 0 {
		t.Fatalf("late start handlers registered = %d, want 0", got)
	}
	game.Stop(StopKind(-999))
}

func TestPointHitsClickTargetRejectsInvalidTarget(t *testing.T) {
	game := &Game{}
	for _, state := range []string{"missing proxy", "hidden", "destroyed"} {
		t.Run(state, func(t *testing.T) {
			sprite := newClickTestSprite(game, state, 1, false)
			switch state {
			case "missing proxy":
				sprite.runtimeState.proxy = nil
			case "hidden":
				sprite.spriteState.isVisible = false
			case "destroyed":
				sprite.markDestroyed()
			}
			// No backend: rejection must precede the hit query.
			if game.pointHitsClickTarget(sprite, mathf.Vec2{}) {
				t.Fatal("invalid target accepted a click")
			}
		})
	}
}

func TestTargetDispatchMutation(t *testing.T) {
	previous := scheduler
	scheduler = nil
	defer func() { scheduler = previous }()

	var registry scriptEventRegistry
	owner, other := &SpriteImpl{}, &SpriteImpl{}
	var calls []string
	enabled, nested := true, false
	var event scriptEventDispatch
	event.run = func(_ coroutine.Thread, sink *eventSink) {
		name := sink.Handler.(string)
		calls = append(calls, name)
		if name == "first" && !nested {
			nested = true
			registry.manager.Add(coreevent.BucketClick, eventSink{Owner: owner, Handler: "second"})
			registry.dispatchTarget(coreevent.BucketClick, owner, event)
		}
		if name == "delete" {
			registry.manager.DeleteOwner(owner)
		}
	}
	registry.manager.Add(coreevent.BucketClick, eventSink{Owner: other, Handler: "other"})
	registry.manager.Add(coreevent.BucketClick, eventSink{
		Owner: owner, Handler: "first", Cond: func(any) bool { return enabled },
	})

	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if want := []string{"first", "first", "second"}; !slices.Equal(calls, want) {
		t.Fatalf("nested dispatch = %v, want %v", calls, want)
	}
	enabled = false
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if want := []string{"first", "first", "second", "second"}; !slices.Equal(calls, want) {
		t.Fatalf("condition change = %v, want %v", calls, want)
	}
	registry.manager.DeleteOwner(owner)
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if len(calls) != 4 {
		t.Fatalf("deleted owner received event: %v", calls)
	}
	registry.manager.Add(coreevent.BucketClick, eventSink{Owner: owner, Handler: "delete"})
	registry.manager.Add(coreevent.BucketClick, eventSink{Owner: owner, Handler: "after"})
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if want := []string{"first", "first", "second", "second", "delete", "after"}; !slices.Equal(calls, want) {
		t.Fatalf("deleted during dispatch = %v, want %v", calls, want)
	}
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if len(calls) != 6 {
		t.Fatalf("deleted handlers received another event: %v", calls)
	}
}

func TestTargetDispatchCompletesMatchingBeforeHandlers(t *testing.T) {
	setupRuntimeScheduler(t)

	var registry scriptEventRegistry
	owner, other := &SpriteImpl{}, &SpriteImpl{}
	enabled := true
	var steps []string
	registry.manager.Add(coreevent.BucketClick, eventSink{Owner: owner, Handler: "first"})
	registry.manager.Add(coreevent.BucketClick, eventSink{
		Owner: other,
		Cond: func(any) bool {
			steps = append(steps, "check-other")
			return false
		},
	})
	registry.manager.Add(coreevent.BucketClick, eventSink{
		Owner: owner,
		Cond: func(any) bool {
			steps = append(steps, "check-second")
			return enabled
		},
		Handler: "second",
	})

	registry.dispatchTarget(coreevent.BucketClick, owner, scriptEventDispatch{
		mode: coroutine.BatchWaitDone,
		run: func(_ coroutine.Thread, sink *eventSink) {
			name := sink.Handler.(string)
			steps = append(steps, "run-"+name)
			if name == "first" {
				enabled = false
			}
		},
	})

	if want := []string{"check-second", "run-first", "run-second"}; !slices.Equal(steps, want) {
		t.Fatalf("dispatch steps = %v, want %v", steps, want)
	}
}

func TestStartBatchCleansPendingThreadCanceledBeforeRun(t *testing.T) {
	currentScheduler := setupRuntimeScheduler(t)
	var registry scriptEventRegistry
	var canceled coroutine.Thread
	var pendingBeforeCancel, runningMarkedPending, canceledRan bool
	dispatcher := currentScheduler.Create("dispatcher", func(coroutine.Thread) {
		registry.dispatchStartEventBatch([]eventSink{{Owner: "first"}, {Owner: "canceled"}}, scriptEventDispatch{
			mode: coroutine.BatchWaitFirstSlice,
			run: func(thread coroutine.Thread, sink *eventSink) {
				if sink.Owner == "canceled" {
					canceledRan = true
					return
				}
				runningMarkedPending = registry.isPendingStartThread(thread)
				currentScheduler.StopIf(func(thread coroutine.Thread) bool {
					if thread.Owner != "canceled" {
						return false
					}
					canceled = thread
					pendingBeforeCancel = registry.isPendingStartThread(thread)
					return true
				})
			},
		})
	})
	currentScheduler.Join(dispatcher)
	if canceled == nil || !pendingBeforeCancel {
		t.Fatal("later start thread was not registered as pending before cancellation")
	}
	currentScheduler.Join(canceled)
	if canceledRan {
		t.Fatal("canceled start handler ran")
	}
	if registry.isPendingStartThread(canceled) {
		t.Fatal("canceled start thread retained its pending marker after cleanup")
	}
	if runningMarkedPending {
		t.Fatal("running start handler retained its pending marker")
	}
}

func TestTargetOrderPreservesGroupsAndSnapshot(t *testing.T) {
	game := &Game{}
	game.scene.init()
	back, front := &SpriteImpl{g: game}, &SpriteImpl{g: game}
	removed := &SpriteImpl{g: game}
	game.scene.add(back)
	game.scene.add(front)
	sinks := []eventSink{
		{Owner: game, Handler: "stage-1"},
		{Owner: back, Handler: "back-1"},
		{Owner: removed, Handler: "removed"},
		{Owner: front, Handler: "front-1"},
		{Owner: []int{1}, Handler: "external"},
		{Owner: back, Handler: "back-2"},
		{Owner: game, Handler: "stage-2"},
		{Owner: front, Handler: "front-2"},
		{Owner: &Game{}, Handler: "other-game"},
	}
	snapshot := slices.Clone(sinks)
	ordered := sinksInTargetOrder(game, sinks)
	names := make([]string, len(ordered))
	for i, sink := range ordered {
		names[i] = sink.Handler.(string)
	}
	want := []string{"front-1", "front-2", "back-1", "back-2", "removed", "external", "other-game", "stage-1", "stage-2"}
	if !slices.Equal(names, want) {
		t.Fatalf("order = %v, want %v", names, want)
	}
	if !reflect.DeepEqual(sinks, snapshot) {
		t.Fatal("sorting modified the shared event snapshot")
	}
	ordered[0].Handler = "changed"
	if !reflect.DeepEqual(sinks, snapshot) {
		t.Fatal("ordered result aliases the shared event snapshot")
	}
	// A later broadcast must observe the new layer order, not a cached grouping.
	game.scene.items[0], game.scene.items[1] = front, back
	if got := sinksInTargetOrder(game, sinks)[0].Handler; got != "back-1" {
		t.Fatalf("first handler after layer change = %v", got)
	}
}

func TestEventHandlerNonMatchingTriggerPreservesInvocation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		register func(*Game, func())
		fire     func(*Game, bool)
	}{
		{"key", func(g *Game, run func()) { g.OnKey__0(KeySpace, run) }, func(g *Game, match bool) {
			key := KeyA
			if match {
				key = KeySpace
			}
			g.doWhenKeyPressed(key)
		}},
		{"click", func(g *Game, run func()) { g.OnClick(run) }, func(g *Game, match bool) {
			if match {
				g.doWhenClick()
			} else {
				sprite := &SpriteImpl{g: g}
				sprite.scriptEventBindings.bind(&g.scriptEvents, sprite)
				sprite.doWhenClick()
			}
		}},
		{"backdrop", func(g *Game, run func()) { g.OnBackdrop__1("scene", run) }, func(g *Game, match bool) {
			name := "other"
			if match {
				name = "scene"
			}
			g.doWhenBackdropChanged(name, false)
		}},
		{"message", func(g *Game, run func()) { g.OnMsg__1("msg", run) }, func(g *Game, match bool) {
			msg := "other"
			if match {
				msg = "msg"
			}
			g.Broadcast__0(msg)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			currentScheduler, game := setupRuntimeEventGame(t)
			gate := currentScheduler.NewLatch()
			started, finished := 0, 0
			tc.register(game, func() { started++; gate.Wait(); finished++ })
			tc.fire(game, true)
			currentScheduler.Update()
			tc.fire(game, false)
			currentScheduler.Update()
			gate.Open()
			currentScheduler.Update()
			if started != 1 || finished != 1 {
				t.Fatalf("non-matching trigger affected invocation: started=%d finished=%d", started, finished)
			}
		})
	}
}

func TestKeyHandlerSharesAdmissionAcrossRegisteredKeys(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	gate := currentScheduler.NewLatch()
	var received []Key
	keys := []Key{KeySpace, KeyA}
	game.OnKey__1(keys, func(key Key) {
		received = append(received, key)
		gate.Wait()
	})
	keys[1] = KeyB
	game.doWhenKeyPressed(KeySpace)
	currentScheduler.Update()
	game.doWhenKeyPressed(KeyA)
	currentScheduler.Update()
	if len(received) != 1 || received[0] != KeySpace {
		t.Fatalf("keys did not share one registration: %v", received)
	}
	gate.Open()
	currentScheduler.Update()
	game.doWhenKeyPressed(KeyB)
	currentScheduler.Update()
	game.doWhenKeyPressed(KeyA)
	currentScheduler.Update()
	if len(received) != 2 || received[1] != KeyA {
		t.Fatalf("registration did not retain its original keys: %v", received)
	}
}

func TestTargetEventHandlersStayWithTheirOwner(t *testing.T) {
	for _, tc := range []struct {
		name     string
		register func(*SpriteImpl, func())
		fire     func(*SpriteImpl, *SpriteImpl)
	}{
		{"click", func(s *SpriteImpl, run func()) { s.OnClick(run) }, func(s, _ *SpriteImpl) {
			s.doWhenClick()
		}},
		{"swipe", func(s *SpriteImpl, run func()) { s.OnSwipe__0(90, run) }, func(s, _ *SpriteImpl) {
			s.doWhenSwipe(90)
		}},
		{"cloned", func(s *SpriteImpl, run func()) { s.OnCloned__0(run) }, func(s, _ *SpriteImpl) {
			s.doWhenCloned(nil)
		}},
		{"touch", func(s *SpriteImpl, run func()) { s.addTouchStartHandler(func(Sprite) { run() }) }, func(s, other *SpriteImpl) {
			s.doWhenTouchStart(other)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			currentScheduler, game := setupRuntimeEventGame(t)
			first := &newTouchEventSprite(game, "first").SpriteImpl
			second := &newTouchEventSprite(game, "second").SpriteImpl
			var calls [2]int
			tc.register(first, func() { calls[0]++ })
			tc.register(second, func() { calls[1]++ })
			tc.fire(first, second)
			currentScheduler.Update()
			if calls != [2]int{1, 0} {
				t.Fatalf("first target: calls=%v, want [1 0]", calls)
			}
			tc.fire(second, first)
			currentScheduler.Update()
			if calls != [2]int{1, 1} {
				t.Fatalf("second target: calls=%v, want [1 1]", calls)
			}
		})
	}
}

func TestEventHandlerRetriggerPolicies(t *testing.T) {
	for _, tc := range []struct {
		kind         string
		wantStarted  int
		wantFinished int
	}{
		{"key", 1, 1},
		{"any key", 1, 1},
		{"click", 2, 1},
		{"backdrop", 2, 1},
		{"any backdrop", 2, 1},
		{"message", 2, 1},
		{"any message", 2, 1},
		{"timer", 2, 2},
		{"swipe", 2, 2},
		{"cloned", 2, 2},
		{"touch", 2, 2},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			currentScheduler, game := setupRuntimeEventGame(t)
			gate := currentScheduler.NewLatch()
			started, finished := 0, 0
			run := func() { started++; gate.Wait(); finished++ }
			var fire func()
			switch tc.kind {
			case "key":
				game.OnKey__0(KeySpace, run)
				fire = func() { game.doWhenKeyPressed(KeySpace) }
			case "any key":
				game.OnAnyKey(func(Key) { run() })
				fire = func() { game.doWhenKeyPressed(KeySpace) }
			case "click":
				game.OnClick(run)
				fire = func() { game.doWhenClick() }
			case "backdrop":
				game.OnBackdrop__1("scene", run)
				fire = func() { game.doWhenBackdropChanged("scene", false) }
			case "any backdrop":
				game.OnBackdrop__0(func(BackdropName) { run() })
				fire = func() { game.doWhenBackdropChanged("scene", false) }
			case "message":
				game.OnMsg__1("msg", run)
				fire = func() { game.Broadcast__0("msg") }
			case "any message":
				game.OnMsg__0(func(MsgName, any) { run() })
				fire = func() { game.Broadcast__0("msg") }
			case "timer":
				clock.ClearTimers()
				t.Cleanup(clock.ClearTimers)
				game.OnTimer(0, run)
				fire = func() { game.doWhenTimer(0) }
			case "swipe":
				game.OnSwipe__0(90, run)
				fire = func() { game.doWhenSwipe(90) }
			case "cloned":
				sprite := newTouchEventSprite(game, "clone")
				sprite.OnCloned__0(run)
				fire = func() { sprite.doWhenCloned(nil) }
			case "touch":
				sprite := newTouchEventSprite(game, "source")
				other := newTouchEventSprite(game, "target")
				sprite.addTouchStartHandler(func(Sprite) { run() })
				fire = func() { sprite.doWhenTouchStart(&other.SpriteImpl) }
			}
			fire()
			currentScheduler.Update()
			if started != 1 || finished != 0 {
				t.Fatalf("first trigger: started=%d finished=%d, want 1/0", started, finished)
			}
			fire()
			currentScheduler.Update()
			if started != tc.wantStarted || finished != 0 {
				t.Fatalf("retrigger: started=%d finished=%d, want %d/0", started, finished, tc.wantStarted)
			}
			gate.Open()
			currentScheduler.Update()
			if finished != tc.wantFinished {
				t.Fatalf("finished %d scripts, want %d", finished, tc.wantFinished)
			}
			fire()
			currentScheduler.Update()
			if started != tc.wantStarted+1 || finished != tc.wantFinished+1 {
				t.Fatalf("after completion: started=%d finished=%d, want %d/%d", started, finished, tc.wantStarted+1, tc.wantFinished+1)
			}
		})
	}
}

func TestKeyHandlerIdentityIncludesRegistrationAndOwner(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	gate := currentScheduler.NewLatch()
	started := 0
	run := func() { started++; gate.Wait() }
	game.OnKey__0(KeySpace, run)
	game.OnKey__0(KeySpace, run)
	clone := &SpriteImpl{g: game}
	clone.spriteState.cloned = true
	clone.scriptEventBindings.bind(&game.scriptEvents, clone)
	clone.OnKey__0(KeySpace, run)
	game.doWhenKeyPressed(KeySpace)
	currentScheduler.Update()
	game.doWhenKeyPressed(KeySpace)
	currentScheduler.Update()
	if started != 3 {
		t.Fatalf("started %d scripts, want one per registration and owner", started)
	}
	currentScheduler.StopIf(func(th coroutine.Thread) bool { return th.Owner == clone })
	game.doWhenKeyPressed(KeySpace)
	currentScheduler.Update()
	if started != 4 {
		t.Fatalf("canceled clone handler did not restart independently: %d", started)
	}
	gate.Open()
	currentScheduler.Update()
}

// Retriggers may arrive before a handler gets its first slice. Replacing a
// canceled registration must not let its delayed cleanup clear the new one.
func TestEventHandlerQueuedRetriggers(t *testing.T) {
	for _, key := range []bool{false, true} {
		t.Run(map[bool]string{false: "restart", true: "ignore"}[key], func(t *testing.T) {
			currentScheduler, game := setupRuntimeEventGame(t)
			gate := currentScheduler.NewLatch()
			started, finished := 0, 0
			run := func() { started++; gate.Wait(); finished++ }
			fire := func() { game.doWhenClick() }
			if key {
				game.OnKey__0(KeySpace, run)
				fire = func() { game.doWhenKeyPressed(KeySpace) }
			} else {
				game.OnClick(run)
			}
			parent := currentScheduler.Create(game, func(coroutine.Thread) {
				fire()
				fire()
				fire()
			})
			currentScheduler.Join(parent)
			currentScheduler.Update()
			if started != 1 {
				t.Fatalf("queued retriggers started %d handlers", started)
			}
			fire()
			currentScheduler.Update()
			want := 2
			if key {
				want = 1
			}
			if started != want {
				t.Fatalf("retrigger started %d, want %d", started, want)
			}
			gate.Open()
			currentScheduler.Update()
			if finished != 1 {
				t.Fatalf("finished %d handlers", finished)
			}
		})
	}
}

func TestIndependentBroadcastTreeDoesNotInheritReceiverTurn(t *testing.T) {
	for _, fromScript := range []bool{false, true} {
		name := "external"
		if fromScript {
			name = "script"
		}
		t.Run(name, func(t *testing.T) {
			currentScheduler, game := setupRuntimeEventGame(t)
			calls := 0
			game.OnMsg__1("cycle", func() {
				calls++
				if calls == 1 {
					game.Broadcast__0("cycle")
				}
			})

			game.Broadcast__0("cycle")
			currentScheduler.Update()
			if calls != 1 {
				t.Fatalf("nested broadcast calls = %d, want repeated receiver deferred", calls)
			}

			// A new root restarts the deferred receiver without advancing the
			// frame or sharing the first tree's claimed receiver turn.
			if fromScript {
				threads := currentScheduler.StartBatch([]coroutine.Task{{
					Owner: game,
					Run:   func(coroutine.Thread) { game.Broadcast__0("cycle") },
				}}, coroutine.BatchAsync)
				currentScheduler.JoinAll(threads)
			} else {
				game.Broadcast__0("cycle")
			}
			currentScheduler.Update()
			if calls != 2 {
				t.Fatalf("independent broadcast calls = %d, want new root to run in the same frame", calls)
			}
		})
	}
}

func TestMessageExecutionContextReleasedOnCancelAndReset(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	gate := currentScheduler.NewLatch()
	started, cleaned := 0, 0
	for range 3 {
		game.OnMsg__1("held", func() {
			started++
			defer func() { cleaned++ }()
			gate.Wait()
		})
	}

	game.Broadcast__0("held")
	currentScheduler.Update()
	if started != 3 {
		t.Fatalf("started receivers = %d, want 3", started)
	}
	currentScheduler.RunAfterStop(nil)
	if cleaned != 3 || len(game.scriptEvents.messageExecutions) != 0 {
		t.Fatalf("after cancellation: cleaned = %d, retained contexts = %d", cleaned, len(game.scriptEvents.messageExecutions))
	}

	game.scriptEvents.manager.Reset()
	calls := 0
	game.OnMsg__1("held", func() { calls++ })
	game.BroadcastAndWait__0("held")
	if calls != 1 || len(game.scriptEvents.messageExecutions) != 0 {
		t.Fatalf("after reset and completion: calls = %d, retained contexts = %d", calls, len(game.scriptEvents.messageExecutions))
	}
}

func TestMatchingEventSinks(t *testing.T) {
	for _, matches := range [][]int{nil, {2}, {0, 2}, {0, 1, 2}} {
		var calls []int
		sinks := make([]eventSink, 3)
		for i := range sinks {
			sinks[i] = eventSink{Owner: i, Cond: func(data any) bool {
				if data != "message" {
					t.Fatalf("predicate data = %v", data)
				}
				calls = append(calls, i)
				return slices.Contains(matches, i)
			}}
		}
		got := matchingEventSinks(sinks, "message")
		if !slices.Equal(calls, []int{0, 1, 2}) {
			t.Fatalf("predicate calls = %v", calls)
		}
		if len(got) != len(matches) {
			t.Fatalf("matches = %d, want %d", len(got), len(matches))
		}
		for i, owner := range matches {
			if got[i].Owner != owner {
				t.Fatalf("match %d owner = %v, want %d", i, got[i].Owner, owner)
			}
			got[i].Owner = "changed"
			if sinks[owner].Owner != owner {
				t.Fatal("result aliases the source snapshot")
			}
		}
	}
	unconditional := []eventSink{{Owner: "unconditional"}}
	if got := matchingEventSinks(unconditional, nil); len(got) != 1 || got[0].Owner != "unconditional" {
		t.Fatalf("unconditional match = %v", got)
	}
	if got := matchingEventSinks(nil, nil); len(got) != 0 {
		t.Fatalf("empty snapshot matched %d sinks", len(got))
	}
}

func TestMatchingEventSinksNoMatchAllocations(t *testing.T) {
	sinks := make([]eventSink, 64)
	for i := range sinks {
		sinks[i].Cond = coreevent.MatchValue("other")
	}
	if allocs := testing.AllocsPerRun(100, func() {
		if got := matchingEventSinks(sinks, "tick"); len(got) != 0 {
			t.Fatalf("unexpected matches: %d", len(got))
		}
	}); allocs != 0 {
		t.Fatalf("no-match filtering allocated %g times", allocs)
	}
}

// Stamp-driven games broadcast to many clones each frame.
func BenchmarkTargetOrder(b *testing.B) {
	game := &Game{}
	game.scene.init()
	sinks := make([]eventSink, 0, 202)
	for range 100 {
		sprite := &SpriteImpl{g: game}
		game.scene.add(sprite)
		sinks = append(sinks, eventSink{Owner: sprite}, eventSink{Owner: sprite})
	}
	sinks = append(sinks, eventSink{Owner: game}, eventSink{Owner: "external"})
	b.ReportAllocs()
	for b.Loop() {
		if got := sinksInTargetOrder(game, sinks); len(got) != len(sinks) {
			b.Fatal(len(got))
		}
	}
}

func BenchmarkTargetDispatch(b *testing.B) {
	previous := scheduler
	scheduler = nil
	defer func() { scheduler = previous }()
	for _, scenario := range []struct {
		name           string
		count, matches int
	}{
		{"1/missing", 1, 0},
		{"1/present", 1, 1},
		{"100/missing", 100, 0},
		{"100/present", 100, 1},
		{"300/missing", 300, 0},
		{"300/present", 300, 1},
		{"300/mixed-64", 300, 64},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			var registry scriptEventRegistry
			owner := &SpriteImpl{}
			for i := range scenario.count {
				sink := eventSink{Owner: &SpriteImpl{}}
				if i >= scenario.count-scenario.matches {
					sink.Owner = owner
					if scenario.matches > 1 && i%2 == 0 {
						sink.Cond = func(any) bool { return false }
					}
				}
				registry.manager.Add(coreevent.BucketTouchStart, sink)
			}
			event := scriptEventDispatch{run: func(coroutine.Thread, *eventSink) {}}
			b.ReportAllocs()
			for b.Loop() {
				registry.dispatchTarget(coreevent.BucketTouchStart, owner, event)
			}
		})
	}
}

func BenchmarkTargetDispatchMissingManaged(b *testing.B) {
	var registry scriptEventRegistry
	for range 300 {
		registry.manager.Add(coreevent.BucketTouchStart, eventSink{Owner: &SpriteImpl{}})
	}
	owner := &SpriteImpl{}
	event := scriptEventDispatch{mode: coroutine.BatchAsync}
	b.ReportAllocs()
	for b.Loop() {
		registry.dispatchTarget(coreevent.BucketTouchStart, owner, event)
	}
}

// Australia broadcasts "Front Waypoints" every frame without a matching hat.
// Include successful dispatches to guard against shifting cost to normal broadcasts.
func BenchmarkBroadcastMatching(b *testing.B) {
	previous := scheduler
	scheduler = nil
	defer func() { scheduler = previous }()
	for _, scenario := range []struct {
		name    string
		matches int
	}{{"missing", 0}, {"one", 1}, {"all", 64}} {
		b.Run(scenario.name, func(b *testing.B) {
			game := new(Game)
			game.bindScriptEvents()
			game.scene.init()
			for i := range 16 {
				sprite := newEventOrderSprite(game, "waypoint")
				game.scene.add(sprite)
				for j := range 4 {
					message := "other"
					if i*4+j < scenario.matches {
						message = "tick"
					}
					sprite.OnMsg__1(message, func() {})
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				game.Broadcast__0("tick")
			}
		})
	}
}

func (s *clickThroughSpriteMgr) CheckCollisionWithPoint(obj pkgengine.Object, point mathf.Vec2, isTrigger bool) bool {
	if s.lastIsTrigger != nil {
		s.lastIsTrigger[obj] = isTrigger
	}
	return s.hits[obj]
}

func (s *clickThroughSpriteMgr) SetZIndex(obj pkgengine.Object, z int64) {}

func (c *disappearingClickTarget) Visible() bool { return true }

func (c *disappearingClickTarget) getProxy() *engine.Sprite {
	c.proxyReads++
	if c.proxyReads > 1 {
		return nil
	}
	return &c.proxy
}

func (c *disappearingClickTarget) doWhenClick() { c.clicks++ }

func (*touchEventSprite) Main() {}

func setupRuntimeEventGame(t *testing.T) (*coroutine.Scheduler, *Game) {
	t.Helper()

	currentScheduler := setupRuntimeScheduler(t)
	game := new(Game)
	game.bindScriptEvents()
	game.inputMgr.init(game)
	engine.SetGame(game)
	t.Cleanup(func() { engine.SetGame(nil) })
	return currentScheduler, game
}

func updateRuntimeEventSchedulerUntil(t *testing.T, currentScheduler *coroutine.Scheduler, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("coroutine scheduler did not reach the expected state")
		}
		currentScheduler.Update()
		runtime.Gosched()
	}
}

func pollRuntimeConditions(registry *scriptEventRegistry) {
	scheduler.RunBetweenScripts(registry.sampleConditions)
	withEventRegistrationBarrier(registry.game, registry.dispatchConditions)
}

func pollRuntimeEventConditions(game *Game) {
	game.OnEngineBeforeUpdate(1.0 / 30)
	withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
}

func setupClickThroughSpriteMgr(t *testing.T, hits map[pkgengine.Object]bool) *clickThroughSpriteMgr {
	t.Helper()

	setupImmediateEngineCalls(t)

	original := pkgengine.SpriteMgr
	mgr := &clickThroughSpriteMgr{
		hits:          hits,
		lastIsTrigger: make(map[pkgengine.Object]bool),
	}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() {
		pkgengine.SpriteMgr = original
	})
	return mgr
}

func newClickTestSprite(g *Game, name string, id pkgengine.Object, registerClick bool) *SpriteImpl {
	sprite := &SpriteImpl{}
	sprite.g = g
	sprite.name = name
	sprite.spriteState.isVisible = true
	sprite.runtimeState.proxy = &engine.Sprite{}
	sprite.runtimeState.proxy.SetId(id)
	sprite.scriptEventBindings.bind(&g.scriptEvents, sprite)
	if registerClick {
		sprite.OnClick(func() {})
	}
	return sprite
}

func newTouchEventSprite(game *Game, name string) *touchEventSprite {
	sprite := &touchEventSprite{}
	sprite.g, sprite.name, sprite.sprite = game, name, sprite
	sprite.scriptEventBindings.bind(&game.scriptEvents, &sprite.SpriteImpl)
	return sprite
}

type eventOrderLog struct {
	mu      sync.Mutex
	entries []string
}

func BenchmarkBroadcastAndWaitFanout(b *testing.B) {
	for _, handlers := range []int{1, 3, 10} {
		b.Run(fmt.Sprintf("300-targets/%d-handlers", handlers), func(b *testing.B) {
			currentScheduler := coroutine.New(nil)
			installRuntimeScheduler(b, currentScheduler, nil)
			game := new(Game)
			game.bindScriptEvents()
			game.scene.init()
			for i := range 300 {
				sprite := newEventOrderSprite(game, fmt.Sprintf("target-%d", i))
				game.scene.add(sprite)
				for range handlers {
					sprite.OnMsg__1("fan-out", func() {})
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				// Include dispatch and completion; no receiver overlaps the
				// next broadcast. B.Loop excludes fixture setup and cleanup.
				game.BroadcastAndWait__0("fan-out")
			}
		})
	}
}

func TestGlobalBroadcastRunsFrontToBackThenStageToFirstYield(t *testing.T) {
	currentScheduler, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	// Registration follows SPX bootstrap order: stage first, then sprites from
	// back to front. Scratch discovers global hats in the opposite target order.
	game.OnMsg__1("ordered", func() {
		log.add("stage")
	})
	back.OnMsg__1("ordered", func() {
		log.add("back")
	})
	front.OnMsg__1("ordered", func() {
		log.add("front-1-before-yield")
		engine.WaitNextFrame()
		log.add("front-1-after-yield")
	})
	front.OnMsg__1("ordered", func() {
		log.add("front-2")
	})

	game.Broadcast__0("ordered")
	waitForEventOrderEntries(t, currentScheduler, &log, 4)
	requireEventOrder(t, &log, []string{
		"front-1-before-yield",
		"front-2",
		"back",
		"stage",
	})

	advanceEventFrame(t, currentScheduler, &log, 5)
	requireEventOrder(t, &log, []string{
		"front-1-before-yield",
		"front-2",
		"back",
		"stage",
		"front-1-after-yield",
	})
}

func TestOnStartStopAllDrainsSnapshotOnly(t *testing.T) {
	currentScheduler, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	front.OnStart(func() {
		log.add("front-before-yield")
		engine.WaitNextFrame()
		log.add("front-after-yield")
	})
	front.OnStart(func() {
		log.add("front-stop")
		front.Stop(AllStop)
		log.add("front-after-stop")
	})
	back.OnStart(func() {
		log.add("back-before-yield")
		engine.WaitNextFrame()
		log.add("back-after-yield")
	})
	game.OnStart(func() {
		log.add("stage-start")
		game.Broadcast__0("after-stop")
	})
	game.OnMsg__1("after-stop", func() {
		log.add("message-before-yield")
		engine.WaitNextFrame()
		log.add("message-after-yield")
	})

	game.handleEvent(&eventStart{generation: game.lifecycle.generation()})
	updateRuntimeEventSchedulerUntil(t, currentScheduler, game.lifecycle.startDispatched.Load)
	waitForEventOrderEntries(t, currentScheduler, &log, 5)
	requireEventOrder(t, &log, []string{
		"front-before-yield",
		"front-stop",
		"back-before-yield",
		"stage-start",
		"message-before-yield",
	})

	advanceEventFrame(t, currentScheduler, &log, 6)
	requireEventOrder(t, &log, []string{
		"front-before-yield",
		"front-stop",
		"back-before-yield",
		"stage-start",
		"message-before-yield",
		"message-after-yield",
	})
}

func TestGlobalBroadcastTracksDynamicLayerOrder(t *testing.T) {
	currentScheduler, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	game.OnMsg__1("layer-order", func() { log.add("stage") })
	back.OnMsg__1("layer-order", func() { log.add("back") })
	front.OnMsg__1("layer-order", func() { log.add("front") })

	game.Broadcast__0("layer-order")
	waitForEventOrderEntries(t, currentScheduler, &log, 3)
	requireEventOrder(t, &log, []string{"front", "back", "stage"})

	log.reset()
	back.SetLayerTo(Front)
	game.Broadcast__0("layer-order")
	waitForEventOrderEntries(t, currentScheduler, &log, 3)
	requireEventOrder(t, &log, []string{"back", "front", "stage"})
}

func TestGlobalBroadcastIncludesCloneAtItsCurrentLayer(t *testing.T) {
	currentScheduler, game, back, source := setupEventOrderGame(t)
	var log eventOrderLog

	front := newEventOrderSprite(game, "front")
	game.scene.add(front)
	game.scene.updateRenderLayers()

	game.OnMsg__1("clone-order", func() { log.add("stage") })
	back.OnMsg__1("clone-order", func() { log.add("back") })
	source.OnMsg__1("clone-order", func() { log.add("source") })
	front.OnMsg__1("clone-order", func() { log.add("front") })

	// Clone handlers are registered after the original targets, but the clone
	// itself is inserted immediately behind its source in the live layer list.
	clone := newEventOrderSprite(game, "source-clone")
	clone.spriteState.cloned = true
	clone.OnMsg__1("clone-order", func() { log.add("clone") })
	game.scene.addClonedShape(source, clone)

	if got, want := game.getAllShapes(), []Shape{back, clone, source, front}; !reflect.DeepEqual(got, want) {
		t.Fatalf("test clone layer order = %v, want %v", got, want)
	}

	game.Broadcast__0("clone-order")
	waitForEventOrderEntries(t, currentScheduler, &log, 5)
	requireEventOrder(t, &log, []string{"front", "source", "clone", "back", "stage"})
}

func TestKeySpecificHandlersRunBeforeKeyAnyHandlers(t *testing.T) {
	currentScheduler, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	register := func(events *scriptEventBindings, name string) {
		// Register KeyAny first to ensure dispatch uses Scratch's two event
		// phases rather than the flat sink registration order.
		events.OnKey__0(KeyAny, func() { log.add(name + "-any") })
		events.OnKey__0(KeySpace, func() { log.add(name + "-specific") })
	}
	register(&game.scriptEventBindings, "stage")
	register(&back.scriptEventBindings, "back")
	register(&front.scriptEventBindings, "front")

	game.handleEvent(&eventKeyDown{Key: KeySpace})
	waitForEventOrderEntries(t, currentScheduler, &log, 6)
	requireEventOrder(t, &log, []string{
		"front-specific",
		"back-specific",
		"stage-specific",
		"front-any",
		"back-any",
		"stage-any",
	})

	log.reset()
	game.handleEvent(&eventKeyDown{Key: KeyA})
	waitForEventOrderEntries(t, currentScheduler, &log, 3)
	requireEventOrder(t, &log, []string{"front-any", "back-any", "stage-any"})
}

func TestKeyListHandlersRouteKeyAnyToAnyPhase(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	var log eventOrderLog

	game.OnKey__1([]Key{KeyAny, KeySpace, KeyAny}, func(key Key) {
		switch key {
		case KeySpace:
			log.add("with-key-space")
		case KeyA:
			log.add("with-key-a")
		default:
			log.add("with-key-unexpected")
		}
	})
	game.OnKey__2([]Key{KeySpace, KeyAny}, func() { log.add("without-key") })
	game.OnKey__1([]Key{KeySpace, KeySpace}, func(Key) { log.add("specific") })
	game.OnKey__1(nil, func(Key) { log.add("nil") })
	game.OnKey__2([]Key{}, func() { log.add("empty") })

	game.handleEvent(&eventKeyDown{Key: KeySpace})
	waitForEventOrderEntries(t, currentScheduler, &log, 3)
	requireEventOrder(t, &log, []string{"specific", "with-key-space", "without-key"})

	log.reset()
	game.handleEvent(&eventKeyDown{Key: KeyA})
	waitForEventOrderEntries(t, currentScheduler, &log, 2)
	requireEventOrder(t, &log, []string{"with-key-a", "without-key"})
}

func TestFrameBatchSharedKeyHandlerAdmitsFirstMatchingEdge(t *testing.T) {
	for _, tt := range []struct {
		name  string
		keys  []Key
		edges []Key
	}{
		{"any key", nil, []Key{KeyA, KeyB}},
		{"multiple keys", []Key{KeyA, KeyB}, []Key{KeyA, KeyB}},
		{"repeated key", []Key{KeyA}, []Key{KeyA, KeyA}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			currentScheduler, game := setupInputFrameGame(t)
			var got []Key
			handler := func(key Key) { got = append(got, key) }
			if tt.keys == nil {
				game.OnAnyKey(handler)
			} else {
				game.OnKey__1(tt.keys, handler)
			}
			var want []Key
			// Once the first invocation finishes, the registration accepts a new frame.
			for _, edges := range [][]Key{tt.edges, {tt.edges[len(tt.edges)-1]}} {
				clock.Update(1.0/30, 30)
				batch := &inputFrame{}
				for _, key := range edges {
					batch.events = append(batch.events, &eventKeyDown{Key: key})
				}
				game.inputMgr.frame.pending = batch
				game.dispatchFrameEvents()
				currentScheduler.Update()
				want = append(want, edges[0])
				if !slices.Equal(got, want) {
					t.Fatalf("handler keys = %v, want %v", got, want)
				}
			}
		})
	}
}

func TestFrameBatchSharedKeyHandlerIgnoresInputUntilPreviousInvocationFinishes(t *testing.T) {
	currentScheduler, game := setupInputFrameGame(t)
	var got []Key
	finished := false
	game.OnAnyKey(func(key Key) {
		got = append(got, key)
		if key == KeyA {
			engine.WaitNextFrame()
			finished = true
		}
	})
	want := [][]Key{{KeyA}, {KeyA}, {KeyA, KeyB}}
	for frame, key := range []Key{KeyA, KeyB, KeyB} {
		clock.Update(1.0/30, 30)
		game.inputMgr.frame.pending = &inputFrame{events: []event{&eventKeyDown{Key: key}}}
		game.dispatchFrameEvents()
		currentScheduler.Update()
		if !slices.Equal(got, want[frame]) || finished != (frame > 0) {
			t.Fatalf("frame %d: handler keys = %v, first finished = %v, want %v and %v",
				frame, got, finished, want[frame], frame > 0)
		}
	}
}

func TestAsyncBroadcastCallerContinuesBeforeOrderedReceiverBatch(t *testing.T) {
	currentScheduler, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	register := func(events *scriptEventBindings, name string) {
		events.OnMsg__1("async-order", func() {
			log.add(name)
			engine.WaitNextFrame()
		})
	}
	register(&game.scriptEventBindings, "stage")
	register(&back.scriptEventBindings, "back")
	register(&front.scriptEventBindings, "front")

	currentScheduler.Create(game, func(coroutine.Thread) {
		log.add("caller-before")
		game.Broadcast__0("async-order")
		log.add("caller-after")
	})

	waitForEventOrderEntries(t, currentScheduler, &log, 5)
	requireEventOrder(t, &log, []string{
		"caller-before",
		"caller-after",
		"front",
		"back",
		"stage",
	})

	// Let the receiver first segments return instead of leaving their frame
	// waits for test cleanup to abort.
	clock.Update(0, 0)
	currentScheduler.Update()
}

func TestAsyncBroadcastReceiversFollowEarlierScriptsInRound(t *testing.T) {
	for _, test := range []struct {
		name    string
		wait    func(*coroutine.Scheduler, coroutine.Thread)
		advance float64
		redraw  bool
	}{
		{
			name:    "next frame",
			wait:    (*coroutine.Scheduler).WaitNextFrameFor,
			advance: 1.0 / 30,
		},
		{
			name: "same-frame loop round",
			wait: (*coroutine.Scheduler).YieldLoopFor,
		},
		{
			name:   "same-frame loop round with redraw",
			wait:   (*coroutine.Scheduler).YieldLoopFor,
			redraw: true,
		},
		{
			name: "elapsed wait",
			wait: func(currentScheduler *coroutine.Scheduler, _ coroutine.Thread) {
				currentScheduler.Wait(1)
			},
			advance: 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			currentScheduler, game := setupRuntimeEventGame(t)
			clock.Start()
			var log eventOrderLog
			game.OnMsg__1("new-receiver", func() { log.add("C") })

			a := currentScheduler.Create(game, func(me coroutine.Thread) {
				test.wait(currentScheduler, me)
				log.add("A-before")
				if test.redraw {
					engine.RequestRedraw()
				}
				game.Broadcast__0("new-receiver")
				log.add("A-after")
			})
			currentScheduler.JoinFirstSlice(a)
			b := currentScheduler.Create(game, func(me coroutine.Thread) {
				test.wait(currentScheduler, me)
				log.add("B")
			})
			currentScheduler.JoinFirstSlice(b)

			// Both old scripts are eligible together. A's broadcast must not
			// let its new receiver overtake B's continuation.
			if test.advance > 0 {
				clock.Update(test.advance, 30)
			}
			currentScheduler.Update()
			requireEventOrder(t, &log, []string{"A-before", "A-after", "B", "C"})
		})
	}
}

func TestAsyncBroadcastDoesNotWaitForUnexpiredEarlierScript(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	clock.Start()
	var log eventOrderLog
	game.OnMsg__1("new-receiver", func() { log.add("C") })

	a := currentScheduler.Create(game, func(coroutine.Thread) {
		engine.WaitNextFrame()
		log.add("A-before")
		game.Broadcast__0("new-receiver")
		log.add("A-after")
	})
	currentScheduler.JoinFirstSlice(a)
	b := currentScheduler.Create(game, func(coroutine.Thread) {
		engine.Wait(1)
		log.add("B")
	})
	currentScheduler.JoinFirstSlice(b)

	clock.Update(0.5, 30)
	currentScheduler.Update()
	requireEventOrder(t, &log, []string{"A-before", "A-after", "C"})

	clock.Update(0.5, 30)
	currentScheduler.Update()
	requireEventOrder(t, &log, []string{"A-before", "A-after", "C", "B"})
}

func TestBroadcastAndWaitResumesOlderScriptBeforeNewReceiver(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	clock.Start()
	var log eventOrderLog
	game.OnMsg__1("new-receiver", func() {
		log.add("C-before")
		engine.WaitNextFrame()
		log.add("C-after")
	})

	a := currentScheduler.Create(game, func(coroutine.Thread) {
		engine.WaitNextFrame()
		log.add("A-before")
		game.BroadcastAndWait__0("new-receiver")
		log.add("A-after")
	})
	currentScheduler.JoinFirstSlice(a)
	b := currentScheduler.Create(game, func(coroutine.Thread) {
		engine.WaitNextFrame()
		log.add("B")
	})
	currentScheduler.JoinFirstSlice(b)

	clock.Update(1.0/30, 30)
	currentScheduler.Update()
	requireEventOrder(t, &log, []string{"A-before", "B", "C-before"})

	clock.Update(1.0/30, 30)
	currentScheduler.Update()
	requireEventOrder(t, &log, []string{"A-before", "B", "C-before", "C-after", "A-after"})
}

func TestBroadcastRestartsRunningReceiversIndependently(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	setupCaptureTest(t)

	var log eventOrderLog
	register := func(name string) {
		game.OnMsg__1("restart", func() {
			log.add(name + "-start")
			engine.WaitNextFrame()
			log.add(name + "-finish")
		})
	}
	register("first")
	register("second")

	game.Broadcast__0("restart")
	waitForEventOrderEntries(t, currentScheduler, &log, 2)
	requireEventOrder(t, &log, []string{"first-start", "second-start"})

	// Scratch restarts a receiver instead of overlapping it.
	game.Broadcast__0("restart")
	waitForEventOrderEntries(t, currentScheduler, &log, 4)
	requireEventOrder(t, &log, []string{
		"first-start",
		"second-start",
		"first-start",
		"second-start",
	})

	advanceEventFrame(t, currentScheduler, &log, 6)
	got := log.snapshot()
	if len(got) != 6 {
		t.Fatalf("event order = %v, want four starts and two finishes", got)
	}
	if wantStarts := []string{
		"first-start",
		"second-start",
		"first-start",
		"second-start",
	}; !reflect.DeepEqual(got[:4], wantStarts) {
		t.Fatalf("event starts = %v, want %v", got[:4], wantStarts)
	}
	finishes := slices.Clone(got[4:])
	slices.Sort(finishes)
	if wantFinishes := []string{"first-finish", "second-finish"}; !reflect.DeepEqual(finishes, wantFinishes) {
		t.Fatalf("event finishes = %v, want %v", finishes, wantFinishes)
	}
}

func TestSameSliceBroadcastKeepsLatestPendingReceiver(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	var log eventOrderLog

	game.OnMsg__1("restart-pending", func() {
		log.add("receiver")
	})
	currentScheduler.Create(game, func(coroutine.Thread) {
		log.add("caller-before")
		game.Broadcast__0("restart-pending")
		game.Broadcast__0("restart-pending")
		log.add("caller-after")
	})

	waitForEventOrderEntries(t, currentScheduler, &log, 3)
	requireEventOrder(t, &log, []string{
		"caller-before",
		"caller-after",
		"receiver",
	})
}

func TestBroadcastAndWaitJoinsRestartedReceiver(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	setupCaptureTest(t)

	var log eventOrderLog
	game.OnMsg__1("restart-and-wait", func() {
		log.add("receiver-start")
		engine.WaitNextFrame()
		log.add("receiver-finish")
	})

	game.Broadcast__0("restart-and-wait")
	waitForEventOrderEntries(t, currentScheduler, &log, 1)

	currentScheduler.Create(game, func(coroutine.Thread) {
		log.add("waiter-before")
		game.BroadcastAndWait__0("restart-and-wait")
		log.add("waiter-after")
	})
	waitForEventOrderEntries(t, currentScheduler, &log, 3)
	requireEventOrder(t, &log, []string{
		"receiver-start",
		"waiter-before",
		"receiver-start",
	})

	advanceEventFrame(t, currentScheduler, &log, 5)
	requireEventOrder(t, &log, []string{
		"receiver-start",
		"waiter-before",
		"receiver-start",
		"receiver-finish",
		"waiter-after",
	})
}

func TestBroadcastAndWaitResumesBeforeRestartedEarlierReceiver(t *testing.T) {
	for _, stopOthers := range []bool{false, true} {
		t.Run(fmt.Sprintf("stop-others=%t", stopOthers), func(t *testing.T) {
			currentScheduler, game, actor, _ := setupEventOrderGame(t)
			blocked := currentScheduler.NewLatch()
			var log eventOrderLog
			childRuns := 0
			actor.OnMsg__1("child", func() {
				childRuns++
				log.add("child-started")
				if stopOthers && childRuns == 2 {
					actor.Stop(OtherScriptsInSprite)
					log.add("child-stopped-others")
				}
				blocked.Wait()
			})
			game.OnMsg__1("outer", func() {
				game.Broadcast__0("child")
				log.add("outer-returned")
			})
			currentScheduler.Create(actor, func(coroutine.Thread) {
				game.Broadcast__0("child")
				engine.WaitNextFrame()
				game.BroadcastAndWait__0("outer")
				log.add("caller-resumed")
			})
			currentScheduler.Update()
			requireEventOrder(t, &log, []string{"child-started"})

			// Restarting preserves child's earlier thread slot, which this
			// round already visited. The caller resumes before child runs again.
			clock.Update(0, 0)
			currentScheduler.Update()
			want := []string{"child-started", "outer-returned", "caller-resumed", "child-started"}
			if stopOthers {
				want = append(want, "child-stopped-others")
			}
			requireEventOrder(t, &log, want)
		})
	}
}

func TestBroadcastAndWaitResumesSiblingReceiversInThreadOrder(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	var log eventOrderLog
	for _, name := range []string{"A", "B"} {
		game.OnMsg__1("fan-out", func() {
			log.add(name + "-entered")
			game.BroadcastAndWait__0("inner-" + name)
			log.add(name + "-resumed")
		})
		game.OnMsg__1("inner-"+name, func() { log.add("inner-" + name) })
	}
	currentScheduler.Create(game, func(coroutine.Thread) {
		game.BroadcastAndWait__0("fan-out")
		log.add("caller-resumed")
	})
	currentScheduler.Update()

	// Both inner receivers finish in one round. Their callers retain A/B
	// thread order when the next round visits their waiting broadcasts.
	requireEventOrder(t, &log, []string{
		"A-entered", "B-entered", "inner-A", "inner-B",
		"A-resumed", "B-resumed", "caller-resumed",
	})
}

func TestBroadcastAndWaitPreservesQueuedCleanupOrder(t *testing.T) {
	currentScheduler, game, panel, transition := setupEventOrderGame(t)
	var log eventOrderLog

	transition.OnMsg__1("prepare-level", func() {
		game.Broadcast__0("clear-panel")
		log.add("preparation-returned")
	})
	panel.OnMsg__1("clear-panel", func() {
		log.add("panel-cleared")
		panel.Stop(OtherScriptsInSprite)
	})
	panel.OnMsg__1("open-level", func() { log.add("level-opened") })

	currentScheduler.Create(game, func(coroutine.Thread) {
		game.BroadcastAndWait__0("prepare-level")
		log.add("caller-resumed")
		game.Broadcast__0("open-level")
	})
	currentScheduler.Update()

	// Cleanup already queued by the completed receiver must get its turn
	// before the caller can register the panel's next script for cancellation.
	requireEventOrder(t, &log, []string{
		"preparation-returned", "panel-cleared", "caller-resumed", "level-opened",
	})
}

func TestBroadcastAndWaitKeepsAsyncSenderAtomic(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	var log eventOrderLog
	value := 0
	game.OnMsg__1("publish", func() {
		game.Broadcast__0("observe")
		value = 1
		log.add("sender-updated")
	})
	game.OnMsg__1("observe", func() { log.add(fmt.Sprintf("observed-%d", value)) })

	currentScheduler.Create(game, func(coroutine.Thread) {
		game.BroadcastAndWait__0("publish")
	})
	currentScheduler.Update()
	requireEventOrder(t, &log, []string{"sender-updated", "observed-1"})
}

func TestBroadcastAndWaitDoesNotExposeSenderToEarlyStop(t *testing.T) {
	currentScheduler, game, actor, _ := setupEventOrderGame(t)
	var log eventOrderLog
	actor.OnMsg__1("work", func() {
		game.Broadcast__0("cancel-actor-work")
		log.add("sender-returned")
	})
	actor.OnMsg__1("cancel-actor-work", func() {
		log.add("other-scripts-stopped")
		actor.Stop(OtherScriptsInSprite)
	})

	currentScheduler.Create(game, func(coroutine.Thread) {
		game.BroadcastAndWait__0("work")
	})
	currentScheduler.Update()
	requireEventOrder(t, &log, []string{"sender-returned", "other-scripts-stopped"})
}

func TestBroadcastSiblingWaitDoesNotYieldAsyncSender(t *testing.T) {
	currentScheduler, game, publisher, waiter := setupEventOrderGame(t)
	release := currentScheduler.NewLatch()
	var log eventOrderLog
	value := 0

	waiter.OnMsg__1("fan-out", func() {
		log.add("waiter-entered")
		game.BroadcastAndWait__0("hold-open")
		log.add("waiter-returned")
	})
	publisher.OnMsg__1("fan-out", func() {
		game.Broadcast__0("observe-sibling")
		value = 1
		log.add("publisher-updated")
	})
	game.OnMsg__1("hold-open", release.Wait)
	game.OnMsg__1("observe-sibling", func() { log.add(fmt.Sprintf("observed-%d", value)) })

	game.Broadcast__0("fan-out")
	currentScheduler.Update()
	requireEventOrder(t, &log, []string{"waiter-entered", "publisher-updated", "observed-1"})

	release.Open()
	currentScheduler.Update()
	requireEventOrder(t, &log, []string{
		"waiter-entered", "publisher-updated", "observed-1", "waiter-returned",
	})
}

func TestBroadcastAndWaitRunsQueuedDescendantsWithoutJoiningThem(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	release := currentScheduler.NewLatch()
	var log eventOrderLog
	game.OnMsg__1("parent", func() {
		game.Broadcast__0("child")
		log.add("parent-returned")
	})
	game.OnMsg__1("child", func() {
		game.Broadcast__0("grandchild")
		log.add("child-returned")
	})
	game.OnMsg__1("grandchild", func() {
		log.add("grandchild-started")
		release.Wait()
		game.Broadcast__0("notify-complete")
		log.add("grandchild-returned")
	})
	game.OnMsg__1("notify-complete", func() { log.add("notification-ran") })
	currentScheduler.Create(game, func(coroutine.Thread) {
		game.BroadcastAndWait__0("parent")
		log.add("caller-resumed")
	})
	currentScheduler.Update()

	// Every newly queued script gets its first turn, but the wait belongs
	// only to the direct receiver and must not join its blocked descendants.
	// A descendant can still resume and broadcast after the caller returns.
	requireEventOrder(t, &log, []string{
		"parent-returned", "child-returned", "grandchild-started", "caller-resumed",
	})
	release.Open()
	currentScheduler.Update()
	requireEventOrder(t, &log, []string{
		"parent-returned", "child-returned", "grandchild-started", "caller-resumed", "grandchild-returned", "notification-ran",
	})
}

func TestExternalBroadcastAndWaitWaitsForReceiverCompletion(t *testing.T) {
	currentScheduler, game := setupRuntimeEventGame(t)
	release := currentScheduler.NewLatch()
	entered := make(chan struct{})
	returned := make(chan struct{})
	var log eventOrderLog
	game.OnMsg__1("external-request", func() {
		game.Broadcast__0("external-child")
		close(entered)
		release.Wait()
		log.add("receiver-returned")
	})
	game.OnMsg__1("external-child", func() { log.add("child-ran") })
	go func() {
		game.BroadcastAndWait__0("external-request")
		close(returned)
	}()
	updateRuntimeEventSchedulerUntil(t, currentScheduler, func() bool {
		select {
		case <-entered:
			return true
		default:
			return false
		}
	})
	currentScheduler.Update()
	requireEventOrder(t, &log, []string{"child-ran"})
	select {
	case <-returned:
		t.Fatal("external broadcast returned while its receiver was blocked")
	default:
	}

	release.Open()
	currentScheduler.Update()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("external broadcast did not return after its receiver completed")
	}
	requireEventOrder(t, &log, []string{"child-ran", "receiver-returned"})
}

func TestExternalBroadcastOrdersSnapshotBetweenScriptSlices(t *testing.T) {
	currentScheduler, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog
	back.OnMsg__1("ordered", func() { log.add("back") })
	front.OnMsg__1("ordered", func() { log.add("front") })
	entered, changeLayers := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(changeLayers) })
	t.Cleanup(release)
	worker := currentScheduler.Create(back, func(coroutine.Thread) {
		close(entered)
		<-changeLayers
		back.SetLayerTo(Front)
		back.OnMsg__1("ordered", func() { log.add("late") })
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("layer-changing script did not start")
	}
	lastID := currentScheduler.LastThreadID()
	returned := make(chan struct{})
	go func() {
		game.BroadcastAndWait__0("ordered")
		close(returned)
	}()
	// The dispatcher can be created only after capturing the receiver snapshot.
	// Keep the running script's slice until that dispatcher is waiting for it.
	deadline := time.Now().Add(time.Second)
	for currentScheduler.LastThreadID() == lastID {
		if time.Now().After(deadline) {
			t.Fatal("external broadcast did not create its dispatcher")
		}
		runtime.Gosched()
	}
	release()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("external broadcast did not complete")
	}
	currentScheduler.Join(worker)
	// Ordering observes the completed script slice; membership remains the
	// original snapshot, so the newly registered handler is not included.
	requireEventOrder(t, &log, []string{"back", "front"})
}

func TestExternalBroadcastConcurrentLayerChanges(t *testing.T) {
	currentScheduler, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog
	back.OnMsg__1("ordered", func() { log.add("back") })
	front.OnMsg__1("ordered", func() { log.add("front") })
	entered, start := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(start) })
	t.Cleanup(release)
	worker := currentScheduler.Create(back, func(coroutine.Thread) {
		close(entered)
		<-start
		for range 1000 {
			back.SetLayerTo(Front)
			back.SetLayerTo(Back)
		}
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("layer-changing script did not start")
	}
	// Neither operation is ordered after the other by a test-only lock. The
	// dispatcher's script slice must protect both the slice header and elements.
	release()
	game.BroadcastAndWait__0("ordered")
	currentScheduler.Join(worker)
	requireEventOrder(t, &log, []string{"front", "back"})
}

func TestBroadcastStopAllPreventsLaterReceivers(t *testing.T) {
	currentScheduler, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	game.OnMsg__1("stop-all", func() { log.add("stage") })
	back.OnMsg__1("stop-all", func() { log.add("back") })
	front.OnMsg__1("stop-all", func() {
		log.add("front")
		front.Stop(AllStop)
	})

	game.Broadcast__0("stop-all")
	currentScheduler.Update()
	requireEventOrder(t, &log, []string{"front"})
}

func TestCanceledReceiverDoesNotBreakOrderedBatch(t *testing.T) {
	currentScheduler, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	front.OnMsg__1("cancel-middle", func() {
		log.add("front-first")
		front.Stop(OtherScriptsInSprite)
	})
	front.OnMsg__1("cancel-middle", func() { log.add("front-canceled") })
	back.OnMsg__1("cancel-middle", func() { log.add("back") })
	game.OnMsg__1("cancel-middle", func() { log.add("stage") })

	game.Broadcast__0("cancel-middle")
	waitForEventOrderEntries(t, currentScheduler, &log, 3)
	requireEventOrder(t, &log, []string{"front-first", "back", "stage"})
}

func TestConditionsEvaluateFrontToBackBeforeAnyHandler(t *testing.T) {
	currentScheduler, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	register := func(events *scriptEventBindings, name string) {
		events.OnCond(func() bool {
			log.add("check-" + name)
			return true
		}, func() {
			log.add("run-" + name)
		})
	}
	register(&game.scriptEventBindings, "stage")
	register(&back.scriptEventBindings, "back")
	register(&front.scriptEventBindings, "front")

	pollRuntimeConditions(&game.scriptEvents)
	waitForEventOrderEntries(t, currentScheduler, &log, 6)
	requireEventOrder(t, &log, []string{
		"check-front",
		"check-back",
		"check-stage",
		"run-front",
		"run-back",
		"run-stage",
	})
}

func TestDistinctNestedAsyncBroadcastRunsInCurrentFrame(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	currentScheduler, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	game.OnMsg__1("inner-order", func() { log.add("inner-stage") })
	back.OnMsg__1("inner-order", func() { log.add("inner-back") })
	front.OnMsg__1("inner-order", func() { log.add("inner-front") })
	game.OnMsg__1("outer-order", func() {
		log.add("outer-before")
		engine.RequestRedraw()
		game.Broadcast__0("inner-order")
		log.add("outer-after")
	})

	game.Broadcast__0("outer-order")
	waitForEventOrderEntries(t, currentScheduler, &log, 5)
	requireEventOrder(t, &log, []string{
		"outer-before",
		"outer-after",
		"inner-front",
		"inner-back",
		"inner-stage",
	})
}

func (l *eventOrderLog) add(entry string) {
	l.mu.Lock()
	l.entries = append(l.entries, entry)
	l.mu.Unlock()
}

func (l *eventOrderLog) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

func (l *eventOrderLog) reset() {
	l.mu.Lock()
	l.entries = l.entries[:0]
	l.mu.Unlock()
}

func (l *eventOrderLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.entries...)
}

func newEventOrderSprite(game *Game, name string) *SpriteImpl {
	sprite := &SpriteImpl{name: name, g: game}
	sprite.scriptEventBindings.bind(&game.scriptEvents, sprite)
	return sprite
}

func setupEventOrderGame(t *testing.T) (*coroutine.Scheduler, *Game, *SpriteImpl, *SpriteImpl) {
	t.Helper()

	currentScheduler, game := setupRuntimeEventGame(t)
	game.scene.init()
	setupCaptureTest(t)

	back := newEventOrderSprite(game, "back")
	front := newEventOrderSprite(game, "front")
	game.scene.add(back)
	game.scene.add(front)
	game.scene.updateRenderLayers()
	return currentScheduler, game, back, front
}

func waitForEventOrderEntries(t *testing.T, currentScheduler *coroutine.Scheduler, log *eventOrderLog, count int) {
	t.Helper()
	// Drain once before observing the log; a handler may write its final entry
	// while its coroutine is still tearing down.
	currentScheduler.Update()
	updateRuntimeEventSchedulerUntil(t, currentScheduler, func() bool {
		return log.len() >= count
	})
}

func advanceEventFrame(t *testing.T, currentScheduler *coroutine.Scheduler, log *eventOrderLog, count int) {
	t.Helper()
	clock.Update(0, 0)
	waitForEventOrderEntries(t, currentScheduler, log, count)
}

func requireEventOrder(t *testing.T, log *eventOrderLog, want []string) {
	t.Helper()
	if got := log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("event order = %v, want %v", got, want)
	}
}
