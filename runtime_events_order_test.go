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
	"reflect"
	"runtime"
	"slices"
	"sync"
	"testing"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

type eventOrderLog struct {
	mu      sync.Mutex
	entries []string
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

func setupEventOrderGame(t *testing.T) (*coroutine.Coroutines, *Game, *SpriteImpl, *SpriteImpl) {
	t.Helper()

	co, game := setupRuntimeEventGame(t)
	game.initShapeMgr()
	engine.ResetFrameRuntime()
	t.Cleanup(engine.ResetFrameRuntime)

	back := newEventOrderSprite(game, "back")
	front := newEventOrderSprite(game, "front")
	game.shapeMgr.add(back)
	game.shapeMgr.add(front)
	game.shapeMgr.updateRenderLayers()
	return co, game, back, front
}

func waitForEventOrderEntries(t *testing.T, co *coroutine.Coroutines, log *eventOrderLog, count int) {
	t.Helper()
	// Drain once before observing the log; a handler may write its final entry
	// while its coroutine is still tearing down.
	co.Update()
	updateRuntimeEventSchedulerUntil(t, co, func() bool {
		return log.len() >= count
	})
}

func advanceEventFrame(t *testing.T, co *coroutine.Coroutines, log *eventOrderLog, count int) {
	t.Helper()
	itime.Update(0, 0)
	waitForEventOrderEntries(t, co, log, count)
}

func requireEventOrder(t *testing.T, log *eventOrderLog, want []string) {
	t.Helper()
	if got := log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("event order = %v, want %v", got, want)
	}
}

func TestGlobalBroadcastRunsFrontToBackThenStageToFirstYield(t *testing.T) {
	co, game, back, front := setupEventOrderGame(t)
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
	waitForEventOrderEntries(t, co, &log, 4)
	requireEventOrder(t, &log, []string{
		"front-1-before-yield",
		"front-2",
		"back",
		"stage",
	})

	advanceEventFrame(t, co, &log, 5)
	requireEventOrder(t, &log, []string{
		"front-1-before-yield",
		"front-2",
		"back",
		"stage",
		"front-1-after-yield",
	})
}

func TestOnStartStopAllDrainsSnapshotOnly(t *testing.T) {
	co, game, back, front := setupEventOrderGame(t)
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

	game.handleEvent(&eventStart{generation: game.bootstrapGeneration()})
	updateRuntimeEventSchedulerUntil(t, co, game.lifecycleState.StartDispatched.Load)
	waitForEventOrderEntries(t, co, &log, 5)
	requireEventOrder(t, &log, []string{
		"front-before-yield",
		"front-stop",
		"back-before-yield",
		"stage-start",
		"message-before-yield",
	})

	advanceEventFrame(t, co, &log, 6)
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
	co, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	game.OnMsg__1("layer-order", func() { log.add("stage") })
	back.OnMsg__1("layer-order", func() { log.add("back") })
	front.OnMsg__1("layer-order", func() { log.add("front") })

	game.Broadcast__0("layer-order")
	waitForEventOrderEntries(t, co, &log, 3)
	requireEventOrder(t, &log, []string{"front", "back", "stage"})

	log.reset()
	back.SetLayerTo(Front)
	game.Broadcast__0("layer-order")
	waitForEventOrderEntries(t, co, &log, 3)
	requireEventOrder(t, &log, []string{"back", "front", "stage"})
}

func TestGlobalBroadcastIncludesCloneAtItsCurrentLayer(t *testing.T) {
	co, game, back, source := setupEventOrderGame(t)
	var log eventOrderLog

	front := newEventOrderSprite(game, "front")
	game.shapeMgr.add(front)
	game.shapeMgr.updateRenderLayers()

	game.OnMsg__1("clone-order", func() { log.add("stage") })
	back.OnMsg__1("clone-order", func() { log.add("back") })
	source.OnMsg__1("clone-order", func() { log.add("source") })
	front.OnMsg__1("clone-order", func() { log.add("front") })

	// Clone handlers are registered after the original targets, but the clone
	// itself is inserted immediately behind its source in the live layer list.
	clone := newEventOrderSprite(game, "source-clone")
	clone.spriteState.Cloned = true
	clone.OnMsg__1("clone-order", func() { log.add("clone") })
	game.shapeMgr.addClonedShape(source, clone)

	if got, want := game.getAllShapes(), []Shape{back, clone, source, front}; !reflect.DeepEqual(got, want) {
		t.Fatalf("test clone layer order = %v, want %v", got, want)
	}

	game.Broadcast__0("clone-order")
	waitForEventOrderEntries(t, co, &log, 5)
	requireEventOrder(t, &log, []string{"front", "source", "clone", "back", "stage"})
}

func TestKeySpecificHandlersRunBeforeKeyAnyHandlers(t *testing.T) {
	co, game, back, front := setupEventOrderGame(t)
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
	waitForEventOrderEntries(t, co, &log, 6)
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
	waitForEventOrderEntries(t, co, &log, 3)
	requireEventOrder(t, &log, []string{"front-any", "back-any", "stage-any"})
}

func TestKeyListHandlersRouteKeyAnyToAnyPhase(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
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
	waitForEventOrderEntries(t, co, &log, 3)
	requireEventOrder(t, &log, []string{"specific", "with-key-space", "without-key"})

	log.reset()
	game.handleEvent(&eventKeyDown{Key: KeyA})
	waitForEventOrderEntries(t, co, &log, 2)
	requireEventOrder(t, &log, []string{"with-key-a", "without-key"})
}

func TestAsyncBroadcastCallerContinuesBeforeOrderedReceiverBatch(t *testing.T) {
	co, game, back, front := setupEventOrderGame(t)
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

	co.Create(game, func(coroutine.Thread) {
		log.add("caller-before")
		game.Broadcast__0("async-order")
		log.add("caller-after")
	})

	waitForEventOrderEntries(t, co, &log, 5)
	requireEventOrder(t, &log, []string{
		"caller-before",
		"caller-after",
		"front",
		"back",
		"stage",
	})

	// Let the receiver first segments return instead of leaving their frame
	// waits for test cleanup to abort.
	itime.Update(0, 0)
	co.Update()
}

func TestBroadcastRestartsRunningReceiversIndependently(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	engine.ResetFrameRuntime()
	t.Cleanup(engine.ResetFrameRuntime)

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
	waitForEventOrderEntries(t, co, &log, 2)
	requireEventOrder(t, &log, []string{"first-start", "second-start"})

	// Scratch restarts a receiver instead of overlapping it.
	game.Broadcast__0("restart")
	waitForEventOrderEntries(t, co, &log, 4)
	requireEventOrder(t, &log, []string{
		"first-start",
		"second-start",
		"first-start",
		"second-start",
	})

	advanceEventFrame(t, co, &log, 6)
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
	co, game := setupRuntimeEventGame(t)
	var log eventOrderLog

	game.OnMsg__1("restart-pending", func() {
		log.add("receiver")
	})
	co.Create(game, func(coroutine.Thread) {
		log.add("caller-before")
		game.Broadcast__0("restart-pending")
		game.Broadcast__0("restart-pending")
		log.add("caller-after")
	})

	waitForEventOrderEntries(t, co, &log, 3)
	requireEventOrder(t, &log, []string{
		"caller-before",
		"caller-after",
		"receiver",
	})
}

func TestBroadcastAndWaitJoinsRestartedReceiver(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	engine.ResetFrameRuntime()
	t.Cleanup(engine.ResetFrameRuntime)

	var log eventOrderLog
	game.OnMsg__1("restart-and-wait", func() {
		log.add("receiver-start")
		engine.WaitNextFrame()
		log.add("receiver-finish")
	})

	game.Broadcast__0("restart-and-wait")
	waitForEventOrderEntries(t, co, &log, 1)

	co.Create(game, func(coroutine.Thread) {
		log.add("waiter-before")
		game.BroadcastAndWait__0("restart-and-wait")
		log.add("waiter-after")
	})
	waitForEventOrderEntries(t, co, &log, 3)
	requireEventOrder(t, &log, []string{
		"receiver-start",
		"waiter-before",
		"receiver-start",
	})

	advanceEventFrame(t, co, &log, 5)
	requireEventOrder(t, &log, []string{
		"receiver-start",
		"waiter-before",
		"receiver-start",
		"receiver-finish",
		"waiter-after",
	})
}

func TestBroadcastStopAllPreventsLaterReceivers(t *testing.T) {
	co, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	game.OnMsg__1("stop-all", func() { log.add("stage") })
	back.OnMsg__1("stop-all", func() { log.add("back") })
	front.OnMsg__1("stop-all", func() {
		log.add("front")
		front.Stop(AllStop)
	})

	game.Broadcast__0("stop-all")
	co.Update()
	requireEventOrder(t, &log, []string{"front"})
}

func TestCanceledReceiverDoesNotBreakOrderedBatch(t *testing.T) {
	co, game, back, front := setupEventOrderGame(t)
	var log eventOrderLog

	front.OnMsg__1("cancel-middle", func() {
		log.add("front-first")
		front.Stop(OtherScriptsInSprite)
	})
	front.OnMsg__1("cancel-middle", func() { log.add("front-canceled") })
	back.OnMsg__1("cancel-middle", func() { log.add("back") })
	game.OnMsg__1("cancel-middle", func() { log.add("stage") })

	game.Broadcast__0("cancel-middle")
	waitForEventOrderEntries(t, co, &log, 3)
	requireEventOrder(t, &log, []string{"front-first", "back", "stage"})
}

func TestConditionsEvaluateFrontToBackBeforeAnyHandler(t *testing.T) {
	co, game, back, front := setupEventOrderGame(t)
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
	waitForEventOrderEntries(t, co, &log, 6)
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

	co, game, back, front := setupEventOrderGame(t)
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
	waitForEventOrderEntries(t, co, &log, 5)
	requireEventOrder(t, &log, []string{
		"outer-before",
		"outer-after",
		"inner-front",
		"inner-back",
		"inner-stage",
	})
}
