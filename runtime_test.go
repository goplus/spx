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
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/core/clock"
	coreevent "github.com/goplus/spx/v4/internal/core/event"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	"github.com/goplus/spx/v4/internal/engine/profiler"
	"github.com/goplus/spx/v4/internal/engine/ui"
	"github.com/goplus/spx/v4/internal/engine/wrap"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type engineStartTestGame struct {
	*Game
	mainCalls int
}

type captureFlushTestShape struct {
	updates int
}

type captureFlushSpriteMgr struct {
	wrap.SpriteMgrImpl
	batches [][]float32
}

type replayPauseTestExtMgr struct {
	pauses int
}

type debugPanelUiMgr struct {
	pkgengine.IUiMgr
	text      string
	destroyed []pkgengine.Object
}

// The backend advances independently of the Go sprite, as Godot physics does.
type frameSyncSpriteMgr struct {
	wrap.SpriteMgrImpl
	positions map[int64]mathf.Vec2
	batches   [][]float32
}

type frameUpdateFunc func(float64)

type frameMonitorWaitGame struct {
	*Game
	wait                       func()
	thread                     coroutine.Thread
	started, completed, active int
}

type stopAllAudioBackend struct {
	animationAudioBackend
	stopped   int
	destroyed []engine.Object
}

func TestRand0SwapsReversedBounds(t *testing.T) {
	const (
		from = 93
		to   = -220
	)

	sawNonUpperBound := false
	for i := 0; i < 100; i++ {
		got := Rand__0(from, to)
		if got < to || got > from {
			t.Fatalf("Rand__0(%d, %d) = %v, want value in [%d, %d]", from, to, got, to, from)
		}
		if got != float64(from) {
			sawNonUpperBound = true
		}
	}

	if !sawNonUpperBound {
		t.Fatalf("Rand__0(%d, %d) only returned the upper bound; reversed bounds should produce values across the range", from, to)
	}
}

func TestRand1AlwaysSamplesContinuousRange(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	for _, bounds := range [][2]float64{{1, 12}, {12, 1}, {-3, -1}, {-1, 1}, {0, 1}, {1.5, 12}, {1, 12.5}, {12.5, 1}, {-3, -1.5}} {
		SetRandomSeed(1042497735)
		low, high := math.Min(bounds[0], bounds[1]), math.Max(bounds[0], bounds[1])
		seenFraction := false
		for range 64 {
			got := Rand__1(bounds[0], bounds[1])
			if got < low || got >= high {
				t.Fatalf("Rand__1(%v, %v) = %v, outside continuous range", bounds[0], bounds[1], got)
			}
			seenFraction = seenFraction || math.Trunc(got) != got
		}
		if !seenFraction {
			t.Fatalf("Rand__1(%v, %v) only returned integers", bounds[0], bounds[1])
		}
	}
}

func TestRand1EqualBoundsStillConsumeRandomSource(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	SetRandomSeed(17)
	_ = Rand__1(0, 1)
	wantNext := Rand__1(0, 1)
	for _, bound := range []float64{7, 0.5, 1e20} {
		SetRandomSeed(17)
		if got := Rand__1(bound, bound); got != bound {
			t.Fatalf("Rand__1(%v, %v) = %v, want %v", bound, bound, got, bound)
		}
		if got := Rand__1(0, 1); got != wantNext {
			t.Fatalf("Rand__1(%v, %v) did not consume exactly one draw: next = %v, want %v", bound, bound, got, wantNext)
		}
	}
}

// Keep the legacy entry points wired to the core rules. Boundary matrices live
// with their implementation in internal/core/scratch.
func TestLegacyOperatorDelegation(t *testing.T) {
	if got := LimitPrecision(5e-10); got != 0 {
		t.Errorf("LimitPrecision = %v", got)
	}
	if got := Div(0, 0); got != 0 {
		t.Errorf("Div = %v", got)
	}
	if got := FloorMod(-7, 5); got != 3 {
		t.Errorf("FloorMod = %v", got)
	}
	if !Contains("Scratch", "RaTc") {
		t.Error("Contains lost case-insensitive matching")
	}
	value := NewValue("01")
	if got := Compare(value, 2); got != -1 {
		t.Errorf("Compare = %v", got)
	}
	if !Equal(value, 1) {
		t.Error("Equal lost wrapped numeric comparison")
	}
	if got := CharAt("A😀B", 0); got != "A" {
		t.Errorf("CharAt lost zero-based indexing: %q", got)
	}
}

func TestRandomSeedMakesScriptRandomDeterministic(t *testing.T) {
	SetRandomSeed(123)
	gotA := []float64{Rand__0(1, 10), Rand__1(0, 1), Rand__0(1, 10)}
	SetRandomSeed(123)
	gotB := []float64{Rand__0(1, 10), Rand__1(0, 1), Rand__0(1, 10)}
	defer ResetRandomSeed()

	for i := range gotA {
		if gotA[i] != gotB[i] {
			t.Fatalf("seeded random mismatch at %d: %v vs %v", i, gotA, gotB)
		}
	}
}

func TestNumericAndClockUtilities(t *testing.T) {
	for _, tt := range []struct {
		value float64
		want  int
	}{{0, 0}, {1.49, 1}, {1.5, 2}, {-1.49, -1}, {-1.5, -2}} {
		if got := Iround(tt.value); got != tt.want {
			t.Errorf("Iround(%v) = %d, want %d", tt.value, got, tt.want)
		}
	}
	SetRandomSeed(7)
	t.Cleanup(ResetRandomSeed)
	for range 20 {
		if got := Rand__1(5.5, -2); got < -2 || got >= 5.5 {
			t.Fatalf("reversed float bounds: %v", got)
		}
	}
	if got := Compare("z", "a"); got != 1 {
		t.Fatalf("lexical comparison = %d, want 1", got)
	}
	clock.Start()
	clock.Update(0.25, 30)
	if DeltaTime() != 0.25 || TimeSinceLevelLoad() != 0.25 {
		t.Fatalf("logical clock = %v/%v", DeltaTime(), TimeSinceLevelLoad())
	}
}

func TestDeterministicRandomIsolatedPerCoroutine(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	currentScheduler := setupRuntimeScheduler(t)

	runScenario := func(extraDraws int) []float64 {
		t.Helper()
		setDeterministicRandomSeed(123)

		doneA := make(chan struct{}, 1)
		currentScheduler.Create("extra-draws", func(me coroutine.Thread) {
			for range extraDraws {
				_ = Rand__1(0, 1)
			}
			doneA <- struct{}{}
		})
		select {
		case <-doneA:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for extra-draw coroutine")
		}

		doneB := make(chan []float64, 1)
		currentScheduler.Create("captured-sequence", func(me coroutine.Thread) {
			doneB <- []float64{Rand__1(0, 1), Rand__1(0, 1)}
		})
		select {
		case values := <-doneB:
			return values
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for captured-sequence coroutine")
			return nil
		}
	}

	gotA := runScenario(1)
	gotB := runScenario(5)
	if len(gotA) != len(gotB) {
		t.Fatalf("sequence length mismatch: %v vs %v", gotA, gotB)
	}
	for i := range gotA {
		if gotA[i] != gotB[i] {
			t.Fatalf("coroutine-local deterministic random mismatch at %d: %v vs %v", i, gotA, gotB)
		}
	}
}

func TestDeterministicRandomIgnoresWaitToDoGoroutineDraws(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	currentScheduler := setupRuntimeScheduler(t)

	runScenario := func(drawOutsideCoroutine bool) []float64 {
		t.Helper()
		setDeterministicRandomSeed(123)

		done := make(chan []float64, 1)
		currentScheduler.Create("wait-to-do-random", func(me coroutine.Thread) {
			if got := currentCoroutineID(); got != me.ID() {
				t.Errorf("managed random stream ID = %d, want %d", got, me.ID())
			}
			currentScheduler.WaitToDo(func() {
				if got := currentCoroutineID(); got != 0 {
					t.Errorf("native worker random stream ID = %d, want 0", got)
				}
				if drawOutsideCoroutine {
					_ = Rand__1(0, 1)
				}
			})
			done <- []float64{Rand__1(0, 1), Rand__1(0, 1)}
		})

		select {
		case values := <-done:
			return values
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for wait-to-do-random coroutine")
			return nil
		}
	}

	gotA := runScenario(false)
	gotB := runScenario(true)
	for i := range gotA {
		if gotA[i] != gotB[i] {
			t.Fatalf("wait-to-do goroutine shifted coroutine random stream at %d: %v vs %v", i, gotA, gotB)
		}
	}
}

func TestRandomWithoutSchedulerAndInt31Sequence(t *testing.T) {
	previous := scheduler
	scheduler = nil
	t.Cleanup(func() { scheduler = previous; ResetRandomSeed() })
	if currentCoroutineID() != 0 || lastAllocatedCoroutineID() != 0 {
		t.Fatal("scheduler-free random has a coroutine identity")
	}
	SetRandomSeed(91)
	first := randomInt31n(1000)
	SetRandomSeed(91)
	if second := randomInt31n(1000); second != first || second < 0 || second >= 1000 {
		t.Fatalf("int31 sequence changed: %d/%d", first, second)
	}
}

func TestListRandomPositionSharesScriptRandomSource(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	list := NewList(0, 1, 2, 3, 4, 5, 6)
	SetRandomSeed(91)
	var indices [32]int32
	for i := range indices {
		indices[i] = randomInt31n(int32(list.Len()))
	}
	wantNext := Rand__1(0, 1)
	SetRandomSeed(91)
	for i, want := range indices {
		if got := list.At(Random).Int(); got != int(want) {
			t.Fatalf("random item %d = %d, want %d", i, got, want)
		}
	}
	if got := Rand__1(0, 1); got != wantNext {
		t.Fatalf("random list reads shifted the shared stream: got %v, want %v", got, wantNext)
	}

	SetRandomSeed(91)
	wantFirst := Rand__1(0, 1)
	SetRandomSeed(91)
	list.Clear()
	if list.At(Random).Raw() != nil {
		t.Fatal("empty list has a random item")
	}
	if got := Rand__1(0, 1); got != wantFirst {
		t.Fatalf("empty random list read consumed the shared stream: got %v, want %v", got, wantFirst)
	}
}

func TestListRandomPositionKeepsCoroutineSource(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	currentScheduler := setupRuntimeScheduler(t)
	list := NewList(0, 1, 2, 3, 4, 5, 6)
	run := func(useList bool, extraDraws int) []float64 {
		t.Helper()
		setDeterministicRandomSeed(91)
		noiseDone := make(chan struct{}, 1)
		currentScheduler.Create("other-random-draws", func(me coroutine.Thread) {
			for range extraDraws {
				_ = Rand__1(0, 1)
			}
			noiseDone <- struct{}{}
		})
		select {
		case <-noiseDone:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for unrelated random draws")
		}

		done := make(chan []float64, 1)
		currentScheduler.Create("random-list-items", func(me coroutine.Thread) {
			values := make([]float64, 17)
			for i := range values[:16] {
				if useList {
					values[i] = list.At(Random).Float()
				} else {
					values[i] = float64(randomInt31n(int32(list.Len())))
				}
			}
			if useList {
				var empty List
				_ = empty.At(Random)
			}
			values[16] = Rand__1(0, 1)
			done <- values
		})
		select {
		case values := <-done:
			return values
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for random list reads")
			return nil
		}
	}
	want, got := run(false, 1), run(true, 5)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("coroutine draw %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestOnEngineStartSchedulesBootstrapOnce(t *testing.T) {
	for _, reset := range []bool{false, true} {
		t.Run(fmt.Sprintf("reset=%v", reset), func(t *testing.T) {
			previousBounds := cachedBounds
			t.Cleanup(func() { cachedBounds = previousBounds })
			currentScheduler, game := setupRuntimeEventGame(t)
			gamer := &engineStartTestGame{Game: game}
			game.gamer = gamer
			game.lifecycle.loaded.Store(true)
			bootstrapCalls := 0
			game.lifecycle.queue(game.lifecycle.generation(), func() { bootstrapCalls++ })

			// Hold execution until both host callbacks and any reset finish.
			currentScheduler.RunBetweenScripts(func() {
				game.OnEngineStart()
				game.OnEngineStart()
				if reset {
					game.lifecycle.reset()
				}
			})
			currentScheduler.Update()

			wantCalls := 1
			if reset {
				wantCalls = 0
			}
			if gamer.mainCalls != wantCalls || bootstrapCalls != wantCalls {
				t.Fatalf("MainEntry calls = %d, bootstrap calls = %d; want %d each", gamer.mainCalls, bootstrapCalls, wantCalls)
			}
			if game.lifecycle.loaded.Load() != !reset || game.lifecycle.bootstrapDone.Load() != !reset {
				t.Fatalf("running = %v, bootstrap done = %v; want %v", game.lifecycle.loaded.Load(), game.lifecycle.bootstrapDone.Load(), !reset)
			}
			if game.lifecycle.startDispatched.Load() {
				t.Fatal("bootstrap dispatched OnStart before the first frame")
			}
			threads := currentScheduler.LastThreadID()
			game.OnEngineStart()
			currentScheduler.Update()
			if currentScheduler.LastThreadID() != threads {
				t.Fatal("repeated OnEngineStart scheduled more work")
			}
		})
	}
}

func TestDebugPanelShowsLatestCoroutineStats(t *testing.T) {
	installAskTestUiMgr(t)
	mgr := &debugPanelUiMgr{IUiMgr: pkgengine.UiMgr}
	pkgengine.UiMgr = mgr
	previousDebug := profiler.Debug
	t.Cleanup(func() { profiler.Debug = previousDebug })
	currentScheduler := setupRuntimeScheduler(t)
	thread := currentScheduler.Create("debug", func(me coroutine.Thread) {
		currentScheduler.WaitYield(me)
		currentScheduler.WaitNextFrameFor(me)
	})
	currentScheduler.JoinFirstSlice(thread)
	var game Game
	game.debugState.Debug = true
	for range 2 {
		currentScheduler.Update()
		game.showDebugPanel()
		panel := game.debugState.DebugPanel
		game.showDebugPanel()
		if panel == nil || game.debugState.DebugPanel != panel || !profiler.Debug {
			t.Fatal("debug panel was not created and reused")
		}
		stats := currentScheduler.UpdateStats()
		for _, want := range []string{
			fmt.Sprintf("coro: MoveTime: %.2f\n", stats.MoveTime),
			fmt.Sprintf("coro: WaitTime: %.2f\n", stats.WaitTime),
			fmt.Sprintf("coro: TaskProcessing: %.2f\n", stats.JobTime),
			fmt.Sprintf("coro: TaskCounts: %d\n", stats.JobCount),
			fmt.Sprintf("coro: WaitFrameCount: %d\n", stats.FrameWaitCount),
			fmt.Sprintf("coro: WaitMainCount: %d\n", stats.MainThreadCount),
			fmt.Sprintf("coro: NextCount: %d\n", stats.DeferredCount),
		} {
			if !strings.Contains(mgr.text, want) {
				t.Errorf("debug panel missing %q: %s", want, mgr.text)
			}
		}
	}
	id := game.debugState.DebugPanel.GetId()
	game.debugState.Debug = false
	game.showDebugPanel()
	game.showDebugPanel()
	if profiler.Debug || game.debugState.DebugPanel != nil || !slices.Equal(mgr.destroyed, []pkgengine.Object{id}) {
		t.Fatal("disabling debug did not destroy the panel exactly once")
	}
}

func TestOnEngineRenderFlushesSpriteProxiesEveryFrame(t *testing.T) {
	spriteMgr := setupCaptureFlushSpriteMgr(t)

	var game Game
	game.lifecycle.loaded.Store(true)
	game.camera = &cameraImpl{g: &game}
	game.scene.init()
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)

	shape := &captureFlushTestShape{}
	game.scene.add(shape)
	destroyed := &SpriteImpl{}
	destroyed.runtimeState.proxy = &engine.Sprite{}
	game.scene.add(destroyed)
	game.scene.removeShape(destroyed)

	engine.SetGame(&game)
	defer engine.SetGame(nil)
	setupCaptureTest(t)
	engine.SetCaptureHandler(func(engine.CaptureRequest) error { return nil })

	game.OnEngineRender(0)
	if shape.updates != 0 {
		t.Fatalf("post-coroutine proxy flush advanced shape logic %d times, want 0", shape.updates)
	}
	if destroyed.runtimeState.proxy != nil {
		t.Fatal("pending sprite destroy was not flushed on an ordinary frame")
	}
	if len(spriteMgr.batches) != 1 {
		t.Fatalf("ordinary-frame proxy batches = %d, want 1", len(spriteMgr.batches))
	}

	destroyedWithCapture := &SpriteImpl{}
	destroyedWithCapture.runtimeState.proxy = &engine.Sprite{}
	game.scene.add(destroyedWithCapture)
	game.scene.removeShape(destroyedWithCapture)
	if err := engine.EnqueueCapture("after-coroutine", nil); err != nil {
		t.Fatal(err)
	}
	game.OnEngineRender(0)
	if shape.updates != 0 {
		t.Fatalf("capture-frame proxy flush advanced shape logic %d times, want 0", shape.updates)
	}
	if destroyedWithCapture.runtimeState.proxy != nil {
		t.Fatal("pending sprite destroy was not included in the capture-frame proxy flush")
	}
	if len(spriteMgr.batches) != 2 {
		t.Fatalf("proxy batches after capture frame = %d, want 2", len(spriteMgr.batches))
	}
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
}

func TestOnEngineRenderFlushesSpriteProxiesBeforeReplayEOFPause(t *testing.T) {
	resetInputSessionTest(t)
	spriteMgr := setupCaptureFlushSpriteMgr(t)

	var game Game
	game.camera = &cameraImpl{g: &game}
	game.scene.init()
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	shape := &captureFlushTestShape{}
	game.scene.add(shape)
	destroyed := &SpriteImpl{}
	destroyed.runtimeState.proxy = &engine.Sprite{}
	game.scene.add(destroyed)
	game.scene.removeShape(destroyed)
	if _, err := PrepareInputReplay(validRuntimeReplay(InputReplayFrame{Frame: 0, State: InputReplayState{}})); err != nil {
		t.Fatal(err)
	}

	session := claimPreparedSession(t, &game)
	if _, err := consumeInputTick(InputReplayState{}, nil, 1); err != nil {
		t.Fatal(err)
	}
	game.lifecycle.loaded.Store(true)
	game.OnEngineRender(0)

	if shape.updates != 0 {
		t.Fatalf("EOF-only proxy flush advanced shape logic %d times, want 0", shape.updates)
	}
	if destroyed.runtimeState.proxy != nil {
		t.Fatal("pending sprite destroy was not flushed before EOF pause")
	}
	if len(spriteMgr.batches) != 1 {
		t.Fatalf("EOF-only proxy batches = %d, want 1", len(spriteMgr.batches))
	}

	originalExtMgr := pkgengine.ExtMgr
	extMgr := &replayPauseTestExtMgr{}
	pkgengine.ExtMgr = extMgr
	t.Cleanup(func() { pkgengine.ExtMgr = originalExtMgr })
	game.OnEngineFrameEnd()
	game.OnEngineFrameEnd()
	if extMgr.pauses != 1 {
		t.Fatalf("EOF frame-end pauses = %d, want one", extMgr.pauses)
	}
	if status := session.status(); status.Phase != InputSessionPhaseCompleted || !status.Completed {
		t.Fatalf("EOF session status = %+v", status)
	}
}

func TestEngineResetClearsBootstrapAndAllowsStartupAgain(t *testing.T) {
	installAskTestUiMgr(t)
	game := setupStageLoadTestGame(t, gameTestFS{}).baseGame()
	// Reset owns both registries, even when prior tests left native proxies.
	pkgengine.CreateEmptySprite[engine.Sprite](mathf.Vec2{})
	pkgengine.CreateUI[pkgengine.UiNode]("reset")
	game.scene.add(&struct{}{})
	game.lifecycle.loaded.Store(true)
	game.lifecycle.bootstrapDone.Store(true)
	game.lifecycle.startDispatched.Store(true)
	generation := game.lifecycle.generation()
	calls := 0
	game.lifecycle.startOnce.Do(func() { calls++ })
	game.OnEnginePause(true)
	game.OnEnginePause(false)
	if !game.lifecycle.loaded.Load() || game.lifecycle.generation() != generation {
		t.Fatal("pause callback changed the managed lifecycle")
	}
	game.OnEngineReset()
	if game.lifecycle.generation() != generation+1 || game.lifecycle.loaded.Load() || game.lifecycle.bootstrapDone.Load() || game.lifecycle.startDispatched.Load() || game.scene.count() != 0 {
		t.Fatal("engine reset left startup state or shapes from the prior generation")
	}
	if len(pkgengine.Sprites()) != 0 || len(pkgengine.UiNodes()) != 0 {
		t.Fatal("engine reset left registered sprite or UI proxies")
	}
	game.lifecycle.startOnce.Do(func() { calls++ })
	if calls != 2 {
		t.Fatalf("startup calls after reset=%d,want2", calls)
	}
}

func TestScheduleStartEventRejectsUnreadyAndDuplicateDispatch(t *testing.T) {
	for _, tt := range []struct {
		name                        string
		done, dispatched, scheduled bool
	}{
		{"bootstrap incomplete", false, false, false},
		{"already dispatched", true, true, false},
		{"already scheduled", true, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game := &Game{}
			game.lifecycle.startScheduled = tt.scheduled
			game.lifecycle.bootstrapDone.Store(tt.done)
			game.lifecycle.startDispatched.Store(tt.dispatched)
			if event := game.lifecycle.scheduleStart(); event != nil || game.lifecycle.startScheduled != tt.scheduled {
				t.Fatalf("invalid start scheduling returned %v and changed flag to %v", event, game.lifecycle.startScheduled)
			}
		})
	}
	game := &Game{}
	game.lifecycle.generationID = 7
	game.lifecycle.bootstrapDone.Store(true)
	if event := game.lifecycle.scheduleStart(); event == nil || event.generation != 7 {
		t.Fatalf("ready start event=%v,want generation7", event)
	}
	if game.lifecycle.scheduleStart() != nil {
		t.Fatal("same bootstrap scheduled OnStart twice")
	}
}

func TestFrameSyncUpdatesCustomShapesAndMonitors(t *testing.T) {
	_, game := setupRuntimeEventGame(t)
	calls, delta := 0, 0.0
	updater := frameUpdateFunc(func(dt float64) { calls++; delta = dt })
	panel := &monitorPanelSpy{}
	monitor := &Monitor{visible: true, isDirty: true, panel: panel, eval: func() ui.MonitorValue { return ui.MonitorValue{Text: "42"} }}
	game.scene.items = []Shape{&SpriteImpl{}, &textBubble{}, &quoteBubble{}, monitor, updater, &struct{}{}}
	withEventRegistrationBarrier(game, game.dispatchShapeUpdates)
	if calls != 1 || delta != clock.DeltaTime() || panel.value.Text != "42" || monitor.isDirty {
		t.Fatalf("shape update calls=%d delta=%v monitor=%+v", calls, delta, panel)
	}
}

func TestFrameSyncReadsPhysicsBeforeConditionsAndPreservesVisualEdits(t *testing.T) {
	currentScheduler, game, mgr := setupFrameSyncGame(t)
	sprite := addFrameSyncSprite(game, 1, "moving")
	sprite.transform().setPositionRaw(10, 0)
	game.OnEngineRender(0)
	mgr.batches = nil
	mgr.positions[1] = mathf.NewVec2(20, 30)
	sprite.Hide()

	var sampled mathf.Vec2
	game.OnCond(func() bool {
		sampled = mathf.NewVec2(sprite.Xpos(), sprite.Ypos())
		return false
	}, func() {})
	runFrameSyncGame(currentScheduler, game)

	if want := mathf.NewVec2(20, 30); sampled != want || mgr.positions[1] != want {
		t.Fatalf("condition=%v backend=%v, want physical position %v", sampled, mgr.positions[1], want)
	}
	if len(mgr.batches) != 1 || mgr.batches[0][10] != 0 {
		t.Fatalf("frame submissions=%v, want one hidden sprite", mgr.batches)
	}
}

func TestFrameSyncCommitsCollisionSliceAndKeepsFutureWait(t *testing.T) {
	currentScheduler, game, mgr := setupFrameSyncGame(t)
	source := addFrameSyncSprite(game, 1, "source")
	target := addFrameSyncSprite(game, 2, "target")
	game.OnEngineRender(0)
	mgr.batches = nil
	source.addTouchStartHandler(func(Sprite) {
		source.SetXpos(42)
		engine.WaitNextFrame()
		source.SetXpos(84)
	})
	game.triggerEvents = []engine.TriggerEvent{{Src: source.runtimeState.proxy, Dst: target.runtimeState.proxy}}

	runFrameSyncGame(currentScheduler, game)
	if source.Xpos() != 42 || mgr.positions[1].X != 42 || len(mgr.batches) != 1 {
		t.Fatalf("collision frame: logical=%v backend=%v batches=%d", source.Xpos(), mgr.positions[1], len(mgr.batches))
	}
	runFrameSyncGame(currentScheduler, game)
	if source.Xpos() != 84 || mgr.positions[1].X != 84 || len(mgr.batches) != 2 {
		t.Fatalf("following frame: logical=%v backend=%v batches=%d", source.Xpos(), mgr.positions[1], len(mgr.batches))
	}
}

func TestFrameSyncRetainsBootstrapContactsUntilFirstActiveFrame(t *testing.T) {
	currentScheduler, game, mgr := setupFrameSyncGame(t)
	source := addFrameSyncSprite(game, 1, "source")
	target := addFrameSyncSprite(game, 2, "target")
	game.lifecycle.loaded.Store(false)
	game.lifecycle.startDispatched.Store(false)
	touches, starts := 0, 0
	source.addTouchStartHandler(func(Sprite) {
		touches++
		source.SetXpos(42)
	})
	game.OnStart(func() {
		starts++
		if len(game.triggerEvents) != 0 {
			t.Error("OnStart ran before bootstrap contacts were registered")
		}
	})
	game.triggerEvents = []engine.TriggerEvent{{Src: source.runtimeState.proxy, Dst: target.runtimeState.proxy}}
	game.OnEngineUpdate(0)
	currentScheduler.Update()
	game.OnEngineRender(0)
	if touches != 0 || starts != 0 || len(game.triggerEvents) != 1 {
		t.Fatal("incomplete bootstrap consumed contacts or started handlers")
	}
	game.lifecycle.loaded.Store(true)
	game.lifecycle.bootstrapDone.Store(true)
	runFrameSyncGame(currentScheduler, game)
	if touches != 1 || starts != 1 || mgr.positions[1].X != 42 {
		t.Fatalf("first active frame: touches=%d starts=%d position=%v", touches, starts, mgr.positions[1])
	}
	runFrameSyncGame(currentScheduler, game)
	if touches != 1 || starts != 1 {
		t.Fatal("startup events were repeated")
	}
}

func TestFrameSyncShapeCallbacksCanBroadcastBeforeCommit(t *testing.T) {
	currentScheduler, game, mgr := setupFrameSyncGame(t)
	sprite := addFrameSyncSprite(game, 1, "moving")
	game.OnMsg__1("move", func() { sprite.SetXpos(42) })
	game.scene.add(frameUpdateFunc(func(float64) {
		game.Broadcast__0("move")
	}))

	runFrameSyncGame(currentScheduler, game)
	if sprite.Xpos() != 42 || mgr.positions[1].X != 42 || len(mgr.batches) != 1 {
		t.Fatalf("shape callback: logical=%v backend=%v batches=%d", sprite.Xpos(), mgr.positions[1], len(mgr.batches))
	}
}

func TestFrameSyncShapeWaitPreservesRegistrationAndCommit(t *testing.T) {
	for _, test := range []struct {
		name   string
		wait   func()
		frames int
	}{
		{"zero", func() { engine.Wait(0) }, 1},
		{"next_frame", func() { engine.WaitNextFrame() }, 1},
		{"duration", func() { engine.Wait(2.0 / 60) }, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			currentScheduler, game, mgr := setupFrameSyncGame(t)
			source := addFrameSyncSprite(game, 1, "source")
			target := addFrameSyncSprite(game, 2, "target")
			game.lifecycle.startDispatched.Store(false)
			game.lifecycle.bootstrapDone.Store(true)
			started, completed, active, later, keys, touches, starts := 0, 0, 0, 0, 0, 0, 0
			var threads []coroutine.Thread
			game.scene.add(frameUpdateFunc(func(float64) {
				threads = append(threads, scheduler.Caller())
				started++
				active++
				defer func() { active-- }()
				test.wait()
				completed++
			}))
			game.scene.add(frameUpdateFunc(func(float64) {
				later++
				source.SetYpos(7)
			}))
			game.OnKey__0(KeyA, func() { keys++ })
			source.addTouchStartHandler(func(Sprite) { touches++ })
			game.OnStart(func() { starts++ })
			game.triggerEvents = []engine.TriggerEvent{{Src: source.runtimeState.proxy, Dst: target.runtimeState.proxy}}
			game.OnEngineBeforeUpdate(1.0 / 60)
			game.inputMgr.frame.pending.events = append(game.inputMgr.frame.pending.events, &eventKeyDown{Key: KeyA})
			clock.Update(1.0/60, 60)
			runFrameSyncCall(t, currentScheduler, func() {
				game.OnEngineUpdate(1.0 / 60)
				currentScheduler.Update()
				game.OnEngineRender(1.0 / 60)
			})
			if started != 1 || completed != 0 || active != 1 || later != 1 || keys != 1 || touches != 1 || starts != 1 {
				t.Fatalf("first frame: started/completed/active=%d/%d/%d, later/key/touch/start=%d/%d/%d/%d", started, completed, active, later, keys, touches, starts)
			}
			if len(mgr.batches) != 1 || mgr.positions[1].Y != 7 || game.inputMgr.frame.pending != nil || len(game.triggerEvents) != 0 {
				t.Fatal("waiting shape interrupted event registration or frame commit")
			}
			for frame := 1; frame <= test.frames; frame++ {
				runFrameSyncCall(t, currentScheduler, func() { runFrameSyncGame(currentScheduler, game) })
				wantCompleted := 0
				if frame == test.frames {
					wantCompleted = 1
				}
				if started != frame+1 || completed != wantCompleted || active != started-completed || later != frame+1 {
					t.Fatalf("frame %d: started/completed/active/later=%d/%d/%d/%d", frame+1, started, completed, active, later)
				}
			}
			runFrameSyncCall(t, currentScheduler, func() {
				game.Stop(AllOtherScripts)
				currentScheduler.JoinAll(threads)
				currentScheduler.Update()
			})
			if active != 0 {
				t.Fatalf("stopped shape updates still active: %d", active)
			}
		})
	}
}

func TestFrameSyncMonitorWaitSkipsBusyAndCleansStoppedGetter(t *testing.T) {
	for _, test := range []struct {
		name string
		wait func(*Game)
	}{
		{"zero", func(g *Game) { g.Wait(0) }},
		{"next_frame", func(*Game) { engine.WaitNextFrame() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			currentScheduler, game, _ := setupFrameSyncGame(t)
			getter := &frameMonitorWaitGame{Game: game, wait: func() { test.wait(game) }}
			monitor, panel := addFrameSyncMonitor(t, getter)
			for frame := range 2 {
				runFrameSyncCall(t, currentScheduler, func() { runFrameSyncGame(currentScheduler, game) })
				if getter.started != 1 || getter.completed != frame || getter.active != 1-frame {
					t.Fatalf("frame %d: getter started/completed/active=%d/%d/%d", frame+1, getter.started, getter.completed, getter.active)
				}
			}
			if panel.value.Text != "42.00" {
				t.Fatalf("resumed getter rendered %q", panel.value.Text)
			}
			monitor.isDirty = true
			getter.wait = func() { engine.Wait(10) }
			runFrameSyncCall(t, currentScheduler, func() { runFrameSyncGame(currentScheduler, game) })
			monitor.SetSize(2)
			runFrameSyncCall(t, currentScheduler, func() { runFrameSyncGame(currentScheduler, game) })
			if monitor.isDirty || panel.scale != 2 {
				t.Fatal("busy getter blocked the pending UI refresh")
			}
			runFrameSyncCall(t, currentScheduler, func() {
				game.Stop(AllOtherScripts)
				currentScheduler.Join(getter.thread)
				currentScheduler.Update()
			})
			if getter.started != 2 || getter.completed != 1 || getter.active != 0 {
				t.Fatalf("stop: getter started/completed/active=%d/%d/%d", getter.started, getter.completed, getter.active)
			}
			runFrameSyncCall(t, currentScheduler, func() { runFrameSyncGame(currentScheduler, game) })
			if getter.started != 3 || getter.active != 1 {
				t.Fatal("canceled getter retained its admission state")
			}
			currentScheduler.RunAfterStop(nil)
		})
	}
}

func TestFrameSyncMonitorWaitAppliesVisibilityAndLayout(t *testing.T) {
	for _, test := range []struct {
		name   string
		wait   func()
		frames int
	}{
		{"zero", func() { engine.Wait(0) }, 1},
		{"next_frame", func() { engine.WaitNextFrame() }, 1},
		{"duration", func() { engine.Wait(2.5 / 60) }, 3},
	} {
		for _, hide := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/hide=%v", test.name, hide), func(t *testing.T) {
				currentScheduler, game, _ := setupFrameSyncGame(t)
				defer currentScheduler.RunAfterStop(nil)
				getter := &frameMonitorWaitGame{Game: game}
				getter.wait = func() {
					if getter.started == 2 {
						test.wait()
					}
				}
				monitor, panel := addFrameSyncMonitor(t, getter)
				lateEdit := false
				game.scene.add(frameUpdateFunc(func(float64) {
					if lateEdit {
						monitor.SetXpos(99)
						lateEdit = false
					}
				}))
				frame := func() { runFrameSyncCall(t, currentScheduler, func() { runFrameSyncGame(currentScheduler, game) }) }
				frame()
				if !panel.visible || getter.started != 1 || getter.completed != 1 {
					t.Fatal("initial getter did not show the monitor")
				}
				monitor.isDirty = true
				frame()
				if getter.started != 2 || getter.active != 1 {
					t.Fatal("second getter did not wait")
				}
				panel.events = nil
				if hide {
					monitor.Hide()
				} else {
					monitor.SetXYpos(12, -34)
					monitor.SetSize(2)
				}
				for elapsed := 1; elapsed <= test.frames; elapsed++ {
					// This edit occurs after the UI refresh but before getter resumption.
					lateEdit = !hide && elapsed == test.frames
					frame()
					if panel.visible == hide || monitor.Visible() == hide {
						t.Fatalf("frame %d: panel visible=%v, logical visible=%v, events=%v", elapsed, panel.visible, monitor.Visible(), panel.events)
					}
					if hide && slices.Contains(panel.events, "show") {
						t.Fatalf("resumed getter restored stale visibility: %v", panel.events)
					}
					if !hide && (panel.pos != mathf.NewVec2(12, -34) || panel.scale != 2) {
						t.Fatalf("frame %d: waiting getter blocked layout: position=%v, scale=%v", elapsed, panel.pos, panel.scale)
					}
					wantCompleted := 1
					if elapsed == test.frames {
						wantCompleted = 2
					}
					if getter.started != 2 || getter.completed != wantCompleted || getter.active != 2-wantCompleted {
						t.Fatalf("frame %d: getter started/completed/active=%d/%d/%d", elapsed, getter.started, getter.completed, getter.active)
					}
				}
				if monitor.isDirty == hide {
					t.Fatalf("getter lost pending UI state: dirty=%v, hide=%v", monitor.isDirty, hide)
				}
				if !hide {
					frame()
					if monitor.isDirty || panel.pos != mathf.NewVec2(99, -34) {
						t.Fatal("late layout edit did not reach the next refresh")
					}
				}
			})
		}
	}
}

func TestFrameSyncMonitorWaitStopsBeforeGameDestroy(t *testing.T) {
	currentScheduler, game, _ := setupFrameSyncGame(t)
	getter := &frameMonitorWaitGame{Game: game, wait: func() { engine.WaitNextFrame() }}
	monitor, panel := addFrameSyncMonitor(t, getter)
	runFrameSyncCall(t, currentScheduler, func() { runFrameSyncGame(currentScheduler, game) })
	if getter.started != 1 || getter.active != 1 {
		t.Fatal("getter did not wait before destroy")
	}
	stoppedBeforeDestroy := false
	// The engine drains script work before destroying the game.
	currentScheduler.RunAfterStop(func() {
		stoppedBeforeDestroy = getter.active == 0 && !monitor.evaluating
		game.OnEngineDestroy()
	})
	runFrameSyncCall(t, currentScheduler, func() {
		clock.Update(1.0/60, 60)
		currentScheduler.Update()
	})
	if !stoppedBeforeDestroy || getter.completed != 0 || getter.active != 0 || len(panel.events) != 0 {
		t.Fatalf("destroy: stopped=%v, getter completed/active=%d/%d, UI events=%v", stoppedBeforeDestroy, getter.completed, getter.active, panel.events)
	}
}

func TestFrameSyncShapeBroadcastWaitResumesAfterReceiver(t *testing.T) {
	currentScheduler, game, mgr := setupFrameSyncGame(t)
	sprite := addFrameSyncSprite(game, 1, "moving")
	var order []string
	game.OnMsg__1("move", func() {
		order = append(order, "receiver started")
		engine.WaitNextFrame()
		sprite.SetXpos(42)
		order = append(order, "receiver finished")
	})
	game.scene.add(frameUpdateFunc(func(float64) {
		if len(order) != 0 {
			return
		}
		order = append(order, "sender started")
		game.BroadcastAndWait__0("move")
		order = append(order, "sender finished")
	}))
	runFrameSyncCall(t, currentScheduler, func() { runFrameSyncGame(currentScheduler, game) })
	if !slices.Equal(order, []string{"sender started", "receiver started"}) || sprite.Xpos() != 0 {
		t.Fatalf("first frame: order=%v position=%v", order, sprite.Xpos())
	}
	runFrameSyncCall(t, currentScheduler, func() { runFrameSyncGame(currentScheduler, game) })
	if !slices.Equal(order, []string{"sender started", "receiver started", "receiver finished", "sender finished"}) || mgr.positions[1].X != 42 {
		t.Fatalf("second frame: order=%v backend=%v", order, mgr.positions[1])
	}
}

func TestFrameSyncShapeBatchRejectsStaleGame(t *testing.T) {
	for _, reset := range []bool{false, true} {
		t.Run(fmt.Sprintf("reset=%v", reset), func(t *testing.T) {
			currentScheduler, game, _ := setupFrameSyncGame(t)
			later := 0
			game.scene.add(frameUpdateFunc(func(float64) {
				if reset {
					game.lifecycle.reset()
				} else {
					engine.SetGame(new(Game))
				}
			}))
			game.scene.add(frameUpdateFunc(func(float64) { later++ }))
			runFrameSyncCall(t, currentScheduler, func() { runFrameSyncGame(currentScheduler, game) })
			if later != 0 {
				t.Fatal("queued shape started after its game changed")
			}
		})
	}
}

func TestFrameSyncDestroyCancelsOnlyItsSpriteContinuations(t *testing.T) {
	currentScheduler, game, _ := setupFrameSyncGame(t)
	destroyed := addFrameSyncSprite(game, 1, "destroyed")
	survivor := addFrameSyncSprite(game, 2, "survivor")
	pkgengine.InitSpriteInstance(1, destroyed.runtimeState.proxy, nil)
	owners := []any{destroyed, survivor, game}
	threads := make([]coroutine.Thread, len(owners))
	continued := make([]bool, len(owners))
	cleaned := make([]bool, len(owners))
	for i, owner := range owners {
		threads[i] = currentScheduler.Create(owner, func(coroutine.Thread) {
			defer func() { cleaned[i] = true }()
			engine.WaitNextFrame()
			continued[i] = true
		})
		currentScheduler.JoinFirstSlice(threads[i])
	}
	runFrameSyncCall(t, currentScheduler, func() {
		destroyed.Destroy()
		currentScheduler.Join(threads[0])
	})
	if !cleaned[0] || continued[0] || !destroyed.isDestroyed() || destroyed.runtimeState.proxy == nil {
		t.Fatal("public Destroy did not cancel its continuation before proxy deletion committed")
	}
	runFrameSyncCall(t, currentScheduler, func() {
		runFrameSyncGame(currentScheduler, game)
		currentScheduler.JoinAll(threads)
	})
	if !slices.Equal(continued, []bool{false, true, true}) || !slices.Equal(cleaned, []bool{true, true, true}) {
		t.Fatalf("continued=%v cleaned=%v; want only the surviving sprite and global callbacks to continue", continued, cleaned)
	}
}

func TestFrameSyncPreservesDirectEnginePositionWrite(t *testing.T) {
	currentScheduler, game, mgr := setupFrameSyncGame(t)
	sprite := addFrameSyncSprite(game, 1, "moving")
	sprite.SetXpos(10)
	game.OnEngineBeforeUpdate(1.0 / 60)
	clock.Update(1.0/60, 60)
	// Low-level runtime tweens write the proxy after the physical snapshot.
	withEventRegistrationBarrier(frameEventDispatcher{}, func() {
		engine.WaitMainThread(func() {
			sprite.runtimeState.proxy.SetPosition(mathf.NewVec2(42, 43))
		})
		sprite.Hide()
	})
	game.OnEngineUpdate(1.0 / 60)
	currentScheduler.Update()
	game.OnEngineRender(1.0 / 60)
	if want := mathf.NewVec2(42, 43); mgr.positions[1] != want {
		t.Fatalf("visual batch replaced direct engine write: got %v, want %v", mgr.positions[1], want)
	}
	requirePosition(t, sprite, 42, 43)
	if sprite.transform().pendingPosition != (positionVersions{}) {
		t.Fatal("direct position write left superseded position intent pending")
	}
	mgr.positions[1] = mathf.NewVec2(50, 60)
	runFrameSyncGame(currentScheduler, game)
	requirePosition(t, sprite, 50, 60)
}

func TestCaptureQueuesRequestForActiveGame(t *testing.T) {
	var got []string
	engine.SetGame(struct{}{})
	defer engine.SetGame(nil)
	setupCaptureTest(t)
	engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
		got = append(got, req.Name)
		return nil
	})

	capture("step_001.png")
	if len(got) != 0 {
		t.Fatalf("capture ran immediately: %v", got)
	}

	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "step_001.png" {
		t.Fatalf("FlushCaptures = %v, want [step_001.png]", got)
	}
}

func TestRuntimeSchedulerFixtureDrainsBeforeRestoringBindings(t *testing.T) {
	original := setupRuntimeScheduler(t)
	stopped := make(chan struct{})
	cleaned := false
	t.Run("replacement", func(t *testing.T) {
		currentScheduler := coroutine.New(nil)
		installRuntimeScheduler(t, currentScheduler, func() {
			if scheduler != currentScheduler {
				t.Error("scheduler restored before resource cleanup")
			}
			select {
			case <-stopped:
			default:
				t.Error("resource cleanup ran before coroutine completion")
			}
			cleaned = true
		})
		started := make(chan struct{})
		currentScheduler.Create("fixture-drain", func(coroutine.Thread) {
			defer close(stopped)
			if !engine.IsInCoroutine() {
				t.Error("engine scheduler binding does not match the test scheduler")
			}
			close(started)
			engine.WaitNextFrame()
		})
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("test coroutine did not start")
		}
	})
	if !cleaned || scheduler != original {
		t.Fatal("scheduler cleanup did not complete and restore its previous binding")
	}
	bound := make(chan bool, 1)
	original.Create("fixture-restored", func(coroutine.Thread) {
		bound <- engine.IsInCoroutine()
	})
	select {
	case ok := <-bound:
		if !ok {
			t.Fatal("engine scheduler binding was not restored")
		}
	case <-time.After(time.Second):
		t.Fatal("restored scheduler did not start a coroutine")
	}
}

func TestNonVisualLoopsShareFrame(t *testing.T) {
	currentScheduler, _ := setupRuntimeEventGame(t)
	clock.Start()
	counts := [2]int{}
	for i := range counts {
		th := currentScheduler.Create(nil, func(coroutine.Thread) {
			Repeat(5, func() { counts[i]++ })
		})
		currentScheduler.JoinFirstSlice(th)
	}
	currentScheduler.Update()
	if counts != [2]int{5, 5} {
		t.Fatalf("same-frame iterations = %v, want [5 5]", counts)
	}
}

func TestRotationStyleChangesControlRedrawBoundary(t *testing.T) {
	for _, visible := range []bool{false, true} {
		t.Run(map[bool]string{false: "hidden", true: "visible"}[visible], func(t *testing.T) {
			currentScheduler, _ := setupRuntimeEventGame(t)
			clock.Start()
			sprite := new(SpriteImpl)
			sprite.spriteState.isVisible = visible
			sprite.components.transform = &transformComponent{
				sprite:    sprite,
				direction: 45, rotationStyle: Normal,
			}
			var frames []int64
			th := currentScheduler.Create(sprite, func(coroutine.Thread) {
				Repeat(3, func() {
					if len(frames)%2 == 0 {
						sprite.SetRotationStyle(None)
					} else {
						sprite.SetRotationStyle(Normal)
					}
					frames = append(frames, clock.Frame())
				})
			})
			currentScheduler.JoinFirstSlice(th)
			for range 3 {
				currentScheduler.Update()
				clock.Update(1.0/30, 30)
			}
			if len(frames) != 3 {
				t.Fatalf("iterations = %d, want 3", len(frames))
			}
			for i, frame := range frames {
				want := int64(0)
				if visible {
					want = int64(i)
				}
				if frame != want {
					t.Fatalf("iteration %d ran at frame %d, want %d", i, frame, want)
				}
			}
			if sprite.spriteState.dirtyVersion == 0 {
				t.Fatal("rotation style did not mark the render proxy dirty")
			}
		})
	}
}

func TestReplayFinalTickConditionBeforePause(t *testing.T) {
	resetInputSessionTest(t)
	currentScheduler, game := setupRuntimeEventGame(t)
	clock.Start()
	setupCaptureTest(t)
	setupImmediateEngineCalls(t)
	originalInputMgr, originalExtMgr := pkgengine.InputMgr, pkgengine.ExtMgr
	pkgengine.InputMgr = &gameTestInputMgr{}
	extMgr := &replayPauseTestExtMgr{}
	pkgengine.ExtMgr = extMgr
	t.Cleanup(func() {
		pkgengine.InputMgr, pkgengine.ExtMgr = originalInputMgr, originalExtMgr
	})
	replay := validRuntimeReplay(
		InputReplayFrame{Frame: 0, Time: 0},
		InputReplayFrame{Frame: 1, Time: 0.05,
			State:     InputReplayState{KeysDown: []int64{int64(KeyA)}},
			KeyEvents: []InputReplayKeyEvent{{Key: int64(KeyA), Pressed: true}}},
	)
	replay.FixedTimestep = 0.05
	if _, err := PrepareInputReplay(replay, InputSessionOptions{CaptureKey: KeyA}); err != nil {
		t.Fatal(err)
	}
	session := claimPreparedSession(t, game)
	game.inputMgr.init(game)
	game.camera = &cameraImpl{g: game}
	game.scene.init()
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	game.lifecycle.loaded.Store(true)
	game.lifecycle.startDispatched.Store(true)
	var sampled, handled float64
	var conditionCalls, keyCalls int
	game.OnCond(func() bool {
		if keyCalls != 0 {
			t.Error("key handler ran before condition sampling")
		}
		sampled = game.Timer()
		return game.KeyPressed(KeyA)
	}, func() {
		conditionCalls++
		handled = game.Timer()
	})
	game.OnKey__0(KeyA, func() { keyCalls++ })
	var captures []engine.CaptureRequest
	engine.SetCaptureHandler(func(request engine.CaptureRequest) error {
		captures = append(captures, request)
		return nil
	})
	for range replay.Frames {
		game.OnEngineBeforeUpdate(0.05)
		if engine.HasPendingCaptures() {
			t.Fatal("input capture was queued before the clock advanced")
		}
		clock.Update(0.05, 30)
		game.OnEngineUpdate(0.05)
		currentScheduler.Update()
		game.OnEngineRender(0.05)
		if err := engine.FlushCaptures(); err != nil {
			t.Fatal(err)
		}
		game.OnEngineFrameEnd()
	}
	if conditionCalls != 1 || keyCalls != 1 {
		t.Fatalf("condition/key calls = %d/%d, want 1/1", conditionCalls, keyCalls)
	}
	if sampled != 0.05 || handled != 0.1 {
		t.Fatalf("predicate/handler clock = %v/%v, want %v/%v", sampled, handled, 0.05, 0.1)
	}
	if len(captures) != 1 || captures[0].Frame != 2 || captures[0].InputTick == nil || *captures[0].InputTick != 1 {
		t.Fatalf("final tick captures = %+v", captures)
	}
	if extMgr.pauses != 1 || session.status().Phase != InputSessionPhaseCompleted {
		t.Fatalf("EOF pauses = %d, session = %+v", extMgr.pauses, session.status())
	}
}

func TestRedrawFinishesWholeRound(t *testing.T) {
	currentScheduler, _ := setupRuntimeEventGame(t)
	clock.Start()
	var visual, nonVisual int
	th := currentScheduler.Create(nil, func(coroutine.Thread) {
		Forever(func() {
			visual++
			engine.RequestRedraw()
		})
	})
	currentScheduler.JoinFirstSlice(th)
	th = currentScheduler.Create(nil, func(coroutine.Thread) {
		Forever(func() { nonVisual++ })
	})
	currentScheduler.JoinFirstSlice(th)
	currentScheduler.Update()
	for frame := 1; frame <= 4; frame++ {
		if visual != frame || nonVisual != frame {
			t.Fatalf("frame %d: visual/nonvisual = %d/%d", frame, visual, nonVisual)
		}
		clock.Update(1.0/30, 30)
		currentScheduler.Update()
	}
}

func TestExplicitWaitStillCrossesFrame(t *testing.T) {
	currentScheduler, _ := setupRuntimeEventGame(t)
	clock.Start()
	count := 0
	th := currentScheduler.Create(nil, func(coroutine.Thread) {
		Repeat(3, func() {
			count++
			engine.Wait(0)
		})
	})
	currentScheduler.JoinFirstSlice(th)
	for want := 1; want <= 3; want++ {
		currentScheduler.Update()
		if count != want {
			t.Fatalf("iterations = %d, want %d", count, want)
		}
		clock.Update(1.0/30, 30)
	}
}

func TestSpriteVisibilityControlsRedrawBoundary(t *testing.T) {
	for _, visible := range []bool{false, true} {
		t.Run(map[bool]string{false: "hidden", true: "visible"}[visible], func(t *testing.T) {
			currentScheduler, _ := setupRuntimeEventGame(t)
			clock.Start()
			sprite := new(SpriteImpl)
			sprite.spriteState.isVisible = visible
			count := 0
			th := currentScheduler.Create(sprite, func(coroutine.Thread) {
				Repeat(5, func() { count++; sprite.markProxyDirty() })
			})
			currentScheduler.JoinFirstSlice(th)
			currentScheduler.Update()
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

func TestCostumeChangesControlRedrawBoundary(t *testing.T) {
	for _, tt := range []struct {
		name       string
		visible    bool
		costume    any
		wantFrames [3]int64
	}{
		{"visible", true, Next, [3]int64{0, 1, 2}},
		{"hidden", false, Next, [3]int64{0, 0, 0}},
		{"invalid", true, SpriteCostumeName("missing"), [3]int64{0, 0, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			currentScheduler, _ := setupRuntimeEventGame(t)
			clock.Start()
			sprite := newTestRenderSprite()
			sprite.spriteState.isVisible = tt.visible
			var frames [3]int64
			count := 0
			th := currentScheduler.Create(sprite, func(coroutine.Thread) {
				Repeat(len(frames), func() {
					sprite.setCostume(tt.costume)
					frames[count] = clock.Frame()
					count++
				})
			})
			currentScheduler.JoinFirstSlice(th)
			for range frames {
				currentScheduler.Update()
				clock.Update(1.0/30, 30)
			}
			if count != len(frames) || frames != tt.wantFrames {
				t.Fatalf("costume iterations = %d at frames %v, want %d at %v", count, frames, len(frames), tt.wantFrames)
			}
		})
	}
}

func TestSoundControlsRequestRedrawBoundary(t *testing.T) {
	type soundControls interface {
		SetVolume(float64)
		ChangeVolume(float64)
		SetSoundEffect(SoundEffectKind, float64)
		ChangeSoundEffect(SoundEffectKind, float64)
	}
	controls := []struct {
		name string
		call func(soundControls)
	}{
		{"set volume", func(c soundControls) { c.SetVolume(50) }},
		{"change volume", func(c soundControls) { c.ChangeVolume(1) }},
		{"set effect", func(c soundControls) { c.SetSoundEffect(SoundPanEffect, 10) }},
		{"change effect", func(c soundControls) { c.ChangeSoundEffect(SoundPanEffect, 1) }},
	}
	for _, target := range []string{"game", "sprite"} {
		for _, control := range controls {
			t.Run(target+"/"+control.name, func(t *testing.T) {
				currentScheduler, _ := setupRuntimeEventGame(t)
				clock.Start()

				game := new(Game)
				game.soundMgr.Init(&fakeAudioBackend{})
				var sound soundControls = game
				if target == "sprite" {
					sprite := &SpriteImpl{g: game}
					sprite.components.sound = &soundComponent{sprite: sprite}
					sound = sprite
				}

				frames := [3]int64{}
				count := 0
				th := currentScheduler.Create(nil, func(coroutine.Thread) {
					Repeat(len(frames), func() {
						control.call(sound)
						frames[count] = clock.Frame()
						count++
					})
				})
				currentScheduler.JoinFirstSlice(th)
				for range frames {
					currentScheduler.Update()
					clock.Update(1.0/30, 30)
				}

				if count != len(frames) || frames != [3]int64{0, 1, 2} {
					t.Fatalf("iterations = %d at frames %v, want 3 at frames [0 1 2]", count, frames)
				}
			})
		}
	}
}

func TestTimerTrackingHatDoesNotFire(t *testing.T) {
	for _, redraw := range []bool{false, true} {
		t.Run(map[bool]string{false: "nonvisual", true: "redraw-every-round"}[redraw], func(t *testing.T) {
			currentScheduler, game, _ := setupInputSourceTest(t)
			game.lifecycle.markLoaded(game.lifecycle.generation())
			clock.Start()
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
			game.handleEvent(&eventStart{generation: game.lifecycle.generation()})
			currentScheduler.Update()
			// Slow frames must preserve the same sampling order.
			for _, delta := range []float64{1.0 / 30, 1.0 / 60, 0.25, 1.0 / 30} {
				game.OnEngineBeforeUpdate(1.0 / 30)
				clock.Update(delta, 30)
				withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
				currentScheduler.Update()
			}
			if fired != 0 {
				t.Fatalf("tracking hat fired %d times", fired)
			}
			// Stopping tracking allows a timer edge to fire.
			currentScheduler.StopIf(func(th coroutine.Thread) bool { return th.Owner == game })
			game.OnEngineBeforeUpdate(1.0 / 30)
			clock.Update(0.1, 30)
			withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
			currentScheduler.Update()
			game.OnEngineBeforeUpdate(1.0 / 30)
			clock.Update(0.1, 30)
			withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
			currentScheduler.Update()
			if fired != 1 {
				t.Fatalf("stopped tracking hat fired %d times, want 1", fired)
			}
		})
	}
}

func TestConditionSamplingSkipsBootstrapWhileScriptIsRunning(t *testing.T) {
	for _, startDispatched := range []bool{false, true} {
		t.Run(fmt.Sprintf("start_dispatched=%t", startDispatched), func(t *testing.T) {
			currentScheduler, game := setupRuntimeEventGame(t)
			game.lifecycle.startDispatched.Store(startDispatched)
			started, release := make(chan struct{}), make(chan struct{})
			var returned chan struct{}
			t.Cleanup(func() {
				close(release)
				if returned != nil {
					<-returned
				}
			})
			currentScheduler.Create(game, func(coroutine.Thread) {
				close(started)
				<-release
			})
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("bootstrap script did not start")
			}
			samples := 0
			game.OnCond(func() bool { samples++; return true }, func() {})
			returned = make(chan struct{})
			go func() {
				game.OnEngineBeforeUpdate(1.0 / 30)
				close(returned)
			}()
			select {
			case <-returned:
			case <-time.After(time.Second):
				t.Fatal("unstarted frame waited for a script sampling barrier")
			}
			if samples != 0 {
				t.Fatalf("bootstrap condition samples = %d, want 0", samples)
			}
		})
	}
}

func TestConditionSamplesBeforeClockAndHandlerUsesNewClock(t *testing.T) {
	currentScheduler, game, _ := setupInputSourceTest(t)
	game.lifecycle.markLoaded(game.lifecycle.generation())
	clock.Start()
	game.lifecycle.startDispatched.Store(true)
	var sampled, handled float64
	game.OnCond(func() bool { sampled = game.Timer(); return sampled > 0.01 }, func() { handled = game.Timer() })
	clock.Update(0.05, 30)
	game.OnEngineBeforeUpdate(1.0 / 30)
	if handled != 0 {
		t.Fatal("handler ran in predicate phase")
	}
	clock.Update(0.05, 30)
	withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
	currentScheduler.Update()
	if sampled != 0.05 || handled != 0.1 {
		t.Fatalf("predicate/handler time = %v/%v, want 0.05/0.1", sampled, handled)
	}
}

func TestConditionDoesNotReenterRunningHandler(t *testing.T) {
	currentScheduler, game, _ := setupInputSourceTest(t)
	game.lifecycle.markLoaded(game.lifecycle.generation())
	clock.Start()
	game.lifecycle.startDispatched.Store(true)
	value := true
	evaluations, calls := 0, 0
	release := make(chan struct{}, 2)
	completed := make(chan struct{}, 2)
	game.OnCond(func() bool { evaluations++; return value }, func() {
		calls++
		defer func() { completed <- struct{}{} }()
		engine.WaitForChan(release)
	})
	poll := func() {
		game.OnEngineBeforeUpdate(1.0 / 30)
		clock.Update(1.0/30, 30)
		withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
		currentScheduler.Update()
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

func TestLogicFramePreservesPhasesAndShapeSnapshot(t *testing.T) {
	setupRuntimeScheduler(t)
	clock.ClearTimers()
	clock.Start()
	t.Cleanup(clock.ClearTimers)
	game := &Game{sounds: make(map[string]sound)}
	backend := &animationAudioBackend{}
	game.soundMgr.Init(backend)

	newSprite := func(name string) *SpriteImpl {
		anim := newTestAnimationComponent()
		sprite := anim.sprite
		sprite.name, sprite.g = name, game
		game.sounds[name] = &coreproject.SoundConfig{Path: name + ".wav"}
		sprite.animation().pendingAudios = []string{name}
		anim.curAnimState = &animationPlayback{Name: name}
		anim.doneAnimations = []*animationPlayback{anim.curAnimState}
		return sprite
	}
	first, second, late := newSprite("first"), newSprite("second"), newSprite("late")
	destroyed, unbound := newSprite("destroyed"), newSprite("unbound")
	destroyed.markDestroyed()
	unbound.runtimeState.proxy = nil
	game.scene.items = []Shape{first, struct{}{}, destroyed, second, unbound}
	var steps []string
	backend.onPlay = func(id int64) {
		path := backend.plays[len(backend.plays)-1].path
		steps = append(steps, "play:"+path)
		switch id {
		case 1:
			first.animation().curAnimState.OnPlayAudioPlaybackID = id
			// Both passes must finish the original snapshot, even if playback
			// changes the live shape list.
			game.scene.items = []Shape{late}
		case 2:
			if first.animation().curAnimState == nil {
				t.Error("first animation completed before second audio played")
			}
			second.animation().curAnimState.OnPlayAudioPlaybackID = id
		case 3:
			late.animation().curAnimState.OnPlayAudioPlaybackID = id
		}
	}
	backend.onStop = func(id int64) {
		steps = append(steps, fmt.Sprintf("complete:%d", id))
	}
	clock.RegisterTimer(0.2)
	clock.RegisterTimer(0.1)
	clock.RegisterTimer(0.3)
	clock.Update(0.2, 30)
	audios, animations := make([]string, 0, 4), make([]*animationPlayback, 0, 4)
	audioStorage, animationStorage := &audios[:cap(audios)][0], &animations[:cap(animations)][0]
	audios, animations = game.processLogicFrame(audios, animations)
	want := []string{"play:" + engine.ToAssetPath("first.wav"), "play:" + engine.ToAssetPath("second.wav"), "complete:1", "complete:2"}
	if !slices.Equal(steps, want) {
		t.Fatalf("logic phases = %v, want %v", steps, want)
	}
	if first.animation().curAnimState != nil || second.animation().curAnimState != nil {
		t.Fatal("original snapshot did not complete both animations")
	}
	if len(late.animation().pendingAudios) != 1 || len(late.animation().doneAnimations) != 1 {
		t.Fatal("newly added sprite was processed in the old snapshot")
	}
	for _, sprite := range []*SpriteImpl{destroyed, unbound} {
		if len(sprite.animation().pendingAudios) != 0 || len(sprite.animation().doneAnimations) != 0 || sprite.animation().curAnimState == nil {
			t.Fatalf("inactive sprite %s was not drained without playback", sprite.name)
		}
	}
	// Timers belong to the frame event phase, not the audio/animation loop.
	for _, timestamp := range []int64{100, 200} {
		if timer, ok := clock.NextTimer(); !ok || timer != timestamp {
			t.Fatalf("pending timer = %d, %v, want %d", timer, ok, timestamp)
		}
	}
	if _, ok := clock.NextTimer(); ok {
		t.Fatal("future timer became due early")
	}
	steps = nil
	audios, animations = game.processLogicFrame(audios, animations)
	want = []string{"play:" + engine.ToAssetPath("late.wav"), "complete:3"}
	if !slices.Equal(steps, want) {
		t.Fatalf("next frame = %v, want %v", steps, want)
	}
	if len(audios) != 0 || cap(audios) != 4 || &audios[:cap(audios)][0] != audioStorage ||
		len(animations) != 0 || cap(animations) != 4 || &animations[:cap(animations)][0] != animationStorage {
		t.Fatal("logic frames did not preserve both reusable scratch buffers")
	}
}

func TestLogicFramePreservesEmptyScratch(t *testing.T) {
	clock.ClearTimers()
	t.Cleanup(clock.ClearTimers)
	for _, items := range [][]Shape{nil, {struct{}{}}} {
		game := &Game{}
		game.scene.items = items
		audios, animations := game.processLogicFrame([]string{}, []*animationPlayback{})
		if audios == nil || animations == nil || len(audios) != 0 || len(animations) != 0 {
			t.Fatalf("scratch = %#v, %#v, want non-nil empty slices", audios, animations)
		}
		audios, animations = game.processLogicFrame(nil, nil)
		if audios != nil || animations != nil {
			t.Fatalf("nil scratch = %#v, %#v, want nil slices", audios, animations)
		}
	}
}

func TestProcedureConsumesStopThisScript(t *testing.T) {
	Procedure(func() {
		panic(coroutine.ErrStopThisScript)
	})
}

func TestProcedurePropagatesOtherPanics(t *testing.T) {
	tests := []struct {
		name  string
		panic any
	}{
		{name: "abort thread", panic: coroutine.ErrAbortThread},
		{name: "user panic", panic: "user panic"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if got := recover(); got != test.panic {
					t.Fatalf("panic = %v, want %v", got, test.panic)
				}
			}()
			Procedure(func() {
				panic(test.panic)
			})
		})
	}
}

func TestProcedureScopesStopThisScriptAcrossRepeat(t *testing.T) {
	currentScheduler := setupRuntimeScheduler(t)

	done := make(chan string, 1)
	currentScheduler.Create("stop-this-script", func(coroutine.Thread) {
		trace := "outer-before;"
		Procedure(func() {
			trace += "inner-before;"
			Repeat(1, func() {
				// Match a stop nested in a converted repeat callback.
				currentScheduler.StopThisScript()
				trace += "inner-bad;"
			})
			trace += "inner-bad2;"
		})
		trace += "outer-after;"
		done <- trace
	})

	select {
	case got := <-done:
		if want := "outer-before;inner-before;outer-after;"; got != want {
			t.Fatalf("trace = %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("stop-this-script boundary did not return from procedure")
	}
}

func TestWarpSkipsControlFlowFrameWaits(t *testing.T) {
	currentScheduler := setupRuntimeScheduler(t)

	type result struct {
		repeatCalls int
		condCalls   int
		flagDuring  bool
		flagAfter   bool
	}

	done := make(chan result, 1)
	currentScheduler.Create("run-without-screen-refresh", func(me coroutine.Thread) {
		flagDuring := false
		repeatCalls := 0
		condCalls := 0
		Warp(func() {
			flagDuring = IsRunWithoutScreenRefresh()
			Repeat(3, func() {
				repeatCalls++
			})
			WaitUntil(func() bool {
				condCalls++
				return condCalls > 2
			})
		})

		done <- result{
			repeatCalls: repeatCalls,
			condCalls:   condCalls,
			flagDuring:  flagDuring,
			flagAfter:   IsRunWithoutScreenRefresh(),
		}
	})

	select {
	case got := <-done:
		if !got.flagDuring || got.flagAfter {
			t.Fatalf("flag state = during:%v after:%v, want true/false", got.flagDuring, got.flagAfter)
		}
		if got.repeatCalls != 3 {
			t.Fatalf("repeatCalls = %d, want 3", got.repeatCalls)
		}
		if got.condCalls != 3 {
			t.Fatalf("condCalls = %d, want 3", got.condCalls)
		}
	case <-time.After(time.Second):
		t.Fatal("control-flow wait points should not block when run without screen refresh is enabled")
	}
}

func TestWarpRestoresPreviousState(t *testing.T) {
	currentScheduler := setupRuntimeScheduler(t)

	type result struct {
		prev        bool
		outerDuring bool
		callDuring  bool
		callAfter   bool
		restored    bool
	}

	done := make(chan result, 1)
	currentScheduler.Create("run-without-screen-refresh-state", func(me coroutine.Thread) {
		prev := SetRunWithoutScreenRefresh(true)
		outerDuring := IsRunWithoutScreenRefresh()

		callDuring := false
		Warp(func() {
			callDuring = IsRunWithoutScreenRefresh()
		})

		callAfter := IsRunWithoutScreenRefresh()
		SetRunWithoutScreenRefresh(prev)
		done <- result{
			prev:        prev,
			outerDuring: outerDuring,
			callDuring:  callDuring,
			callAfter:   callAfter,
			restored:    IsRunWithoutScreenRefresh(),
		}
	})

	select {
	case got := <-done:
		if got.prev {
			t.Fatal("previous run-without-screen-refresh flag should default to false")
		}
		if !got.outerDuring || !got.callDuring || !got.callAfter {
			t.Fatalf("flag state = outer:%v call:%v after:%v, want true/true/true", got.outerDuring, got.callDuring, got.callAfter)
		}
		if got.restored {
			t.Fatal("run-without-screen-refresh flag should restore to false after outer reset")
		}
	case <-time.After(time.Second):
		t.Fatal("run-without-screen-refresh state should restore without blocking")
	}
}

func TestStopThisScriptAtEventBoundaryEndsThread(t *testing.T) {
	panicReported := make(chan struct{}, 1)
	currentScheduler := coroutine.New(func(coroutine.PanicReport) {
		panicReported <- struct{}{}
	})
	installRuntimeScheduler(t, currentScheduler, nil)

	var script scriptEventBindings
	script.bind(&scriptEventRegistry{}, "owner")
	thread := currentScheduler.Create("event-stop-this-script", func(coroutine.Thread) {
		script.Stop(ThisScript)
		panic("stop-this-script should not continue")
	})

	select {
	case <-thread.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("event thread did not terminate after stop-this-script")
	}
	select {
	case <-panicReported:
		t.Fatal("stop-this-script reached the panic reporter")
	default:
	}
}

func TestStopAllCleansResourcesBeforeAbortingCaller(t *testing.T) {
	for _, target := range []string{"stage", "clone"} {
		t.Run(target, func(t *testing.T) {
			game := setupCloneLimitGame(t)
			game.bindScriptEvents()
			backend := &stopAllAudioBackend{}
			game.soundMgr.Init(backend)
			source := newCloneLimitSprite(game, "source")
			source.transform().x = 42
			source.onInit = func(s *cloneLimitSprite) { s.OnKey__0(KeySpace, func() {}) }
			var clone *SpriteImpl
			doClone(source, nil, func(s *SpriteImpl) { clone = s })
			clone.sound().soundObj = 17
			playback := game.soundMgr.Play(17, "long.wav", false, false, 0, 0, 0)
			gate := scheduler.NewLatch()
			peerFinished := false
			peer := scheduler.Create(&source.SpriteImpl, func(coroutine.Thread) {
				gate.Wait()
				peerFinished = true
			})
			scheduler.JoinFirstSlice(peer)
			caller, stopScripts := any(game), game.Stop
			if target == "clone" {
				caller, stopScripts = clone, clone.Stop
			}
			afterStop := false
			stop := scheduler.Create(caller, func(coroutine.Thread) {
				stopScripts(AllStop)
				afterStop = true
			})
			scheduler.Join(stop)
			scheduler.Join(peer)
			if afterStop || peerFinished {
				t.Fatal("stopped script continued")
			}
			if !clone.isDestroyed() || game.scene.cloneCount != 0 || game.scene.findShapeIndex(clone) >= 0 {
				t.Fatal("stop all retained clone or its quota")
			}
			if len(game.scriptEvents.manager.Snapshot(coreevent.BucketKeyPressed)) != 0 {
				t.Fatal("clone event registration survived")
			}
			if backend.IsPlaying(playback) || backend.stopped != 1 {
				t.Fatal("stop all left audio playing")
			}
			if len(backend.destroyed) != 1 || backend.destroyed[0] != 17 {
				t.Fatalf("released sound objects = %v", backend.destroyed)
			}
			if source.isDestroyed() || source.transform().x != 42 || len(game.getAllShapes()) != 1 {
				t.Fatal("stop all reset or removed original")
			}
			// Repeated stops must not release resources twice.
			again := scheduler.Create(game, func(coroutine.Thread) { game.Stop(AllStop) })
			scheduler.Join(again)
			if game.scene.cloneCount != 0 || len(backend.destroyed) != 1 {
				t.Fatal("repeated stop duplicated cleanup")
			}
		})
	}
}

func TestStopAllCancelsWaitingCloneHandler(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "source")
	gate := scheduler.NewLatch()
	source.onClone = func(*cloneLimitSprite) {
		gate.Wait()
		t.Error("clone handler continued after stop all")
	}
	var clone *SpriteImpl
	parent := scheduler.Create(&source.SpriteImpl, func(coroutine.Thread) {
		doClone(source, nil, func(s *SpriteImpl) { clone = s })
		source.Stop(AllStop)
	})
	scheduler.Join(parent)
	scheduler.Update()
	if !clone.isDestroyed() || game.scene.cloneCount != 0 {
		t.Fatal("pending clone survived stop all")
	}
	flushCloneProxyUpdates(game)
	if clone.runtimeState.proxy != nil {
		t.Fatal("stopped clone retained its proxy")
	}
}

func TestStopAllClearsExistingSoundEffectsWithoutAllocating(t *testing.T) {
	for _, target := range []string{"stage", "sprite"} {
		t.Run(target, func(t *testing.T) {
			currentScheduler, game := setupRuntimeEventGame(t)
			backend := &fakeAudioBackend{pan: 0.5, pitch: 2}
			game.soundMgr.Init(backend)
			source := newCloneLimitSprite(game, "source")
			unused := newCloneLimitSprite(game, "unused")
			if target == "stage" {
				game.audioState.SoundObj = 9
			} else {
				source.sound().soundObj = 9
			}
			stop := currentScheduler.Create(game, func(coroutine.Thread) { game.Stop(AllStop) })
			currentScheduler.Join(stop)
			if backend.pan != 0 || backend.pitch != 1 || backend.createCalls != 0 {
				t.Fatalf("audio after stop: %+v", backend)
			}
			if unused.sound().soundObj != 0 {
				t.Fatal("stop allocated an unused sprite's sound object")
			}
		})
	}
}

func TestStopAllRemovesEveryCloneFromMixedShapes(t *testing.T) {
	game := setupCloneLimitGame(t)
	left := newCloneLimitSprite(game, "left")
	game.scene.add(&struct{}{})
	right := newCloneLimitSprite(game, "right")
	originals := slices.Clone(game.getAllShapes())
	var clones []*SpriteImpl
	for range 3 {
		for _, source := range []*cloneLimitSprite{left, right} {
			doClone(source, nil, func(clone *SpriteImpl) {
				clone.graphicEffects = map[EffectKind]float64{GhostEffect: 50}
				clones = append(clones, clone)
			})
		}
	}
	for range 2 {
		stop := scheduler.Create(&left.SpriteImpl, func(coroutine.Thread) { left.Stop(AllStop) })
		scheduler.Join(stop)
		if !slices.Equal(game.getAllShapes(), originals) || game.scene.cloneCount != 0 {
			t.Fatal("stop retained clones or changed original shape order")
		}
		for _, clone := range clones {
			if !clone.isDestroyed() || clone.graphicEffects[GhostEffect] != 0 {
				t.Fatal("stop skipped a clone's cleanup")
			}
		}
	}
}

func TestStopAllClearsGraphicEffects(t *testing.T) {
	var g Game
	g.scene.init()
	g.graphicEffects = map[EffectKind]float64{GhostEffect: 100}

	left := &SpriteImpl{g: &g, name: "left"}
	left.graphicEffects = map[EffectKind]float64{GhostEffect: 100}

	right := &SpriteImpl{g: &g, name: "right"}
	right.graphicEffects = map[EffectKind]float64{
		GhostEffect:      50,
		BrightnessEffect: 25,
	}

	g.scene.add(left)
	g.scene.add(right)

	g.stopAllResources()

	if got := g.graphicEffects[GhostEffect]; got != 0 {
		t.Fatalf("stage ghost effect = %v, want 0 after stop all reset", got)
	}
	if got := left.graphicEffects[GhostEffect]; got != 0 {
		t.Fatalf("left ghost effect = %v, want 0 after stop all reset", got)
	}
	if got := right.graphicEffects[GhostEffect]; got != 0 {
		t.Fatalf("right ghost effect = %v, want 0 after stop all reset", got)
	}
	if got := right.graphicEffects[BrightnessEffect]; got != 0 {
		t.Fatalf("right brightness effect = %v, want 0 after stop all reset", got)
	}
}

func (g *engineStartTestGame) MainEntry() { g.mainCalls++ }

func (s *captureFlushTestShape) onUpdate(float64) {
	s.updates++
}

func (*replayPauseTestExtMgr) RequestExit(int64) {}

func (*replayPauseTestExtMgr) RequestReset(int64) {}

func (*replayPauseTestExtMgr) RequestRestart() {}

func (*replayPauseTestExtMgr) OnRuntimePanic(string) {}

func (m *replayPauseTestExtMgr) Pause() { m.pauses++ }

func (*replayPauseTestExtMgr) Resume() {}

func (*replayPauseTestExtMgr) IsPaused() bool { return false }

func (*replayPauseTestExtMgr) NextFrame() {}

func (*replayPauseTestExtMgr) SetLayerSorterMode(int64) {}

func (m *captureFlushSpriteMgr) BatchUpdateTransforms(buffer []float32) {
	m.batches = append(m.batches, append([]float32(nil), buffer...))
}

func (m *debugPanelUiMgr) SetText(_ pkgengine.Object, text string) { m.text = text }

func (*debugPanelUiMgr) SetScale(pkgengine.Object, mathf.Vec2) {}

func (m *debugPanelUiMgr) DestroyNode(id pkgengine.Object) bool {
	m.destroyed = append(m.destroyed, id)
	return m.IUiMgr.DestroyNode(id)
}

func (m *frameSyncSpriteMgr) BatchRetrievePositions(ids []int64, out []float32) bool {
	for i, id := range ids {
		pos := m.positions[id]
		out[i*2], out[i*2+1] = float32(pos.X), float32(pos.Y)
	}
	return true
}

func (m *frameSyncSpriteMgr) SetPosition(id pkgengine.Object, pos mathf.Vec2) {
	m.positions[int64(id)] = pos
}

func (*frameSyncSpriteMgr) SetZIndex(pkgengine.Object, int64) {}

func (m *frameSyncSpriteMgr) BatchUpdateTransforms(data []float32) {
	m.batches = append(m.batches, append([]float32(nil), data...))
	for i := 0; i < int(data[0]); i++ {
		at := 2 + i*engine.SyncFieldsPerSprite
		m.positions[int64(data[at])] = mathf.NewVec2(float64(data[at+1]), float64(data[at+2]))
	}
}

func (f frameUpdateFunc) onUpdate(delta float64) { f(delta) }

func (g *frameMonitorWaitGame) Value() float64 {
	g.thread = scheduler.Caller()
	g.started++
	g.active++
	defer func() { g.active-- }()
	g.wait()
	g.completed++
	return 42
}

func setupCaptureFlushSpriteMgr(t *testing.T) *captureFlushSpriteMgr {
	t.Helper()
	setupImmediateEngineCalls(t)
	original := pkgengine.SpriteMgr
	mgr := &captureFlushSpriteMgr{}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() { pkgengine.SpriteMgr = original })
	return mgr
}

func setupFrameSyncGame(t *testing.T) (*coroutine.Scheduler, *Game, *frameSyncSpriteMgr) {
	t.Helper()
	currentScheduler, game, _ := setupInputSourceTest(t)
	original := pkgengine.SpriteMgr
	mgr := &frameSyncSpriteMgr{positions: make(map[int64]mathf.Vec2)}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() { pkgengine.SpriteMgr = original })
	game.scene.init()
	game.camera = &cameraImpl{g: game}
	game.syncBuffer = engine.NewSpriteSyncBuffer(2)
	game.lifecycle.loaded.Store(true)
	game.lifecycle.startDispatched.Store(true)
	return currentScheduler, game, mgr
}

func addFrameSyncSprite(game *Game, id int64, name string) *SpriteImpl {
	sprite := &newCloneAwakeOrderSprite(game, name).SpriteImpl
	sprite.runtimeState.proxy = &engine.Sprite{Target: sprite, MirrorPosition: sprite.mirrorPosition}
	sprite.runtimeState.proxy.SetId(id)
	sprite.runtimeState.isCostumeDirty = false
	sprite.spriteState.isVisible = true
	sprite.physics().physicsMode = DynamicPhysics
	sprite.markProxyDirty()
	game.scene.add(sprite)
	return sprite
}

func runFrameSyncGame(currentScheduler *coroutine.Scheduler, game *Game) {
	const delta = 1.0 / 60
	game.OnEngineBeforeUpdate(delta)
	clock.Update(delta, 60)
	game.OnEngineUpdate(delta)
	currentScheduler.Update()
	game.OnEngineRender(delta)
}

// A failed frame must release the host before fixture cleanup restores globals.
func runFrameSyncCall(t *testing.T, currentScheduler *coroutine.Scheduler, call func()) {
	t.Helper()
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		call()
	}()
	select {
	case recovered := <-done:
		if recovered != nil {
			t.Fatalf("frame panicked: %v", recovered)
		}
	case <-time.After(time.Second):
		currentScheduler.StopAll()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("frame did not exit after stopping its scripts")
		}
		t.Fatal("frame waited for a future-frame continuation")
	}
}

func addFrameSyncMonitor(t *testing.T, getter *frameMonitorWaitGame) (*Monitor, *monitorPanelSpy) {
	t.Helper()
	panel := &monitorPanelSpy{}
	monitor := &Monitor{
		visible: true, isDirty: true, panel: panel,
		eval: buildMonitorEval(reflect.ValueOf(getter).Elem(), 1, "value", ui.MonitorAppearanceDefault),
	}
	if monitor.eval == nil {
		t.Fatal("lowercase monitor property did not bind the Value getter")
	}
	getter.scene.add(monitor)
	return monitor, panel
}

func setupCaptureTest(t testing.TB) {
	t.Helper()
	clear := func() {
		t.Helper()
		engine.SetCaptureHandler(func(engine.CaptureRequest) error { return nil })
		if err := engine.FlushCaptures(); err != nil {
			t.Fatal(err)
		}
		engine.SetCaptureHandler(nil)
	}
	clear()
	t.Cleanup(clear)
}

func setupRuntimeScheduler(t *testing.T) *coroutine.Scheduler {
	t.Helper()
	currentScheduler := coroutine.New(nil)
	installRuntimeScheduler(t, currentScheduler, nil)
	return currentScheduler
}

// beforeRestore runs after the test's coroutines stop, while their scheduler
// remains bound. Register other resource cleanup before installing the scheduler.
func installRuntimeScheduler(t testing.TB, currentScheduler *coroutine.Scheduler, beforeRestore func()) {
	t.Helper()
	original := scheduler
	scheduler = currentScheduler
	engine.SetScheduler(currentScheduler)
	t.Cleanup(func() {
		currentScheduler.RunAfterStop(nil)
		if beforeRestore != nil {
			beforeRestore()
		}
		scheduler = original
		engine.SetScheduler(original)
	})
}

func (b *stopAllAudioBackend) StopAll() {
	b.stopped++
	clear(b.playing)
}

func (b *stopAllAudioBackend) DestroyAudio(obj engine.Object) { b.destroyed = append(b.destroyed, obj) }
