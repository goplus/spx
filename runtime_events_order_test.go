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
	"sync"
	"testing"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

type scratchEventOrderLog struct {
	mu      sync.Mutex
	entries []string
}

func (l *scratchEventOrderLog) add(entry string) {
	l.mu.Lock()
	l.entries = append(l.entries, entry)
	l.mu.Unlock()
}

func (l *scratchEventOrderLog) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

func (l *scratchEventOrderLog) reset() {
	l.mu.Lock()
	l.entries = l.entries[:0]
	l.mu.Unlock()
}

func (l *scratchEventOrderLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.entries...)
}

func newScratchEventOrderSprite(game *Game, name string) *SpriteImpl {
	sprite := &SpriteImpl{name: name, g: game}
	sprite.scriptEventBindings.init(&game.scriptEvents, sprite)
	return sprite
}

func setupScratchEventOrderGame(t *testing.T) (*coroutine.Coroutines, *Game, *SpriteImpl, *SpriteImpl) {
	t.Helper()

	co, game := setupRuntimeEventGame(t)
	game.initShapeMgr()
	engine.ResetFrameRuntime()
	t.Cleanup(engine.ResetFrameRuntime)

	back := newScratchEventOrderSprite(game, "back")
	front := newScratchEventOrderSprite(game, "front")
	game.addShape(back)
	game.addShape(front)
	game.shapeMgr.updateRenderLayers()
	return co, game, back, front
}

func waitForScratchEventOrderEntries(t *testing.T, co *coroutine.Coroutines, log *scratchEventOrderLog, count int) {
	t.Helper()
	// Drain once before observing the log; a handler may write its final entry
	// while its coroutine is still tearing down.
	co.Update()
	updateRuntimeEventSchedulerUntil(t, co, func() bool {
		return log.len() >= count
	})
}

func requireScratchEventOrder(t *testing.T, log *scratchEventOrderLog, want []string) {
	t.Helper()
	if got := log.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("event order = %v, want %v", got, want)
	}
}

func TestScratchGlobalBroadcastRunsFrontToBackThenStageToFirstYield(t *testing.T) {
	co, game, back, front := setupScratchEventOrderGame(t)
	var log scratchEventOrderLog

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
	waitForScratchEventOrderEntries(t, co, &log, 4)
	requireScratchEventOrder(t, &log, []string{
		"front-1-before-yield",
		"front-2",
		"back",
		"stage",
	})

	itime.Update(0, 0)
	waitForScratchEventOrderEntries(t, co, &log, 5)
	requireScratchEventOrder(t, &log, []string{
		"front-1-before-yield",
		"front-2",
		"back",
		"stage",
		"front-1-after-yield",
	})
}

func TestScratchOnStartStopAllDrainsSnapshotOnly(t *testing.T) {
	co, game, back, front := setupScratchEventOrderGame(t)
	var log scratchEventOrderLog

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

	game.handleEvent(&eventStart{generation: game.currentBootstrapGeneration()})
	updateRuntimeEventSchedulerUntil(t, co, game.lifecycleState.StartDispatched.Load)
	waitForScratchEventOrderEntries(t, co, &log, 5)
	requireScratchEventOrder(t, &log, []string{
		"front-before-yield",
		"front-stop",
		"back-before-yield",
		"stage-start",
		"message-before-yield",
	})

	itime.Update(0, 0)
	waitForScratchEventOrderEntries(t, co, &log, 6)
	requireScratchEventOrder(t, &log, []string{
		"front-before-yield",
		"front-stop",
		"back-before-yield",
		"stage-start",
		"message-before-yield",
		"message-after-yield",
	})
}

func TestScratchGlobalBroadcastTracksDynamicLayerOrder(t *testing.T) {
	co, game, back, front := setupScratchEventOrderGame(t)
	var log scratchEventOrderLog

	game.OnMsg__1("layer-order", func() { log.add("stage") })
	back.OnMsg__1("layer-order", func() { log.add("back") })
	front.OnMsg__1("layer-order", func() { log.add("front") })

	game.Broadcast__0("layer-order")
	waitForScratchEventOrderEntries(t, co, &log, 3)
	requireScratchEventOrder(t, &log, []string{"front", "back", "stage"})

	log.reset()
	back.SetLayerTo(Front)
	game.Broadcast__0("layer-order")
	waitForScratchEventOrderEntries(t, co, &log, 3)
	requireScratchEventOrder(t, &log, []string{"back", "front", "stage"})
}

func TestScratchGlobalBroadcastIncludesCloneAtItsCurrentLayer(t *testing.T) {
	co, game, back, source := setupScratchEventOrderGame(t)
	var log scratchEventOrderLog

	front := newScratchEventOrderSprite(game, "front")
	game.addShape(front)
	game.shapeMgr.updateRenderLayers()

	game.OnMsg__1("clone-order", func() { log.add("stage") })
	back.OnMsg__1("clone-order", func() { log.add("back") })
	source.OnMsg__1("clone-order", func() { log.add("source") })
	front.OnMsg__1("clone-order", func() { log.add("front") })

	// Clone handlers are registered after the original targets, but the clone
	// itself is inserted immediately behind its source in the live layer list.
	clone := newScratchEventOrderSprite(game, "source-clone")
	clone.spriteState.Cloned = true
	clone.OnMsg__1("clone-order", func() { log.add("clone") })
	game.addClonedShape(source, clone)

	if got, want := game.getAllShapes(), []Shape{back, clone, source, front}; !reflect.DeepEqual(got, want) {
		t.Fatalf("test clone layer order = %v, want %v", got, want)
	}

	game.Broadcast__0("clone-order")
	waitForScratchEventOrderEntries(t, co, &log, 5)
	requireScratchEventOrder(t, &log, []string{"front", "source", "clone", "back", "stage"})
}

func TestScratchKeySpecificHandlersRunBeforeKeyAnyHandlers(t *testing.T) {
	co, game, back, front := setupScratchEventOrderGame(t)
	var log scratchEventOrderLog

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
	waitForScratchEventOrderEntries(t, co, &log, 6)
	requireScratchEventOrder(t, &log, []string{
		"front-specific",
		"back-specific",
		"stage-specific",
		"front-any",
		"back-any",
		"stage-any",
	})

	log.reset()
	game.handleEvent(&eventKeyDown{Key: KeyA})
	waitForScratchEventOrderEntries(t, co, &log, 3)
	requireScratchEventOrder(t, &log, []string{"front-any", "back-any", "stage-any"})
}

func TestScratchKeyListHandlersRouteKeyAnyToAnyPhase(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	var log scratchEventOrderLog

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
	waitForScratchEventOrderEntries(t, co, &log, 3)
	requireScratchEventOrder(t, &log, []string{"specific", "with-key-space", "without-key"})

	log.reset()
	game.handleEvent(&eventKeyDown{Key: KeyA})
	waitForScratchEventOrderEntries(t, co, &log, 2)
	requireScratchEventOrder(t, &log, []string{"with-key-a", "without-key"})
}

func TestScratchAsyncBroadcastCallerContinuesBeforeOrderedReceiverBatch(t *testing.T) {
	co, game, back, front := setupScratchEventOrderGame(t)
	var log scratchEventOrderLog

	register := func(events *scriptEventBindings, name string) {
		events.OnMsg__1("async-order", func() {
			log.add(name)
			engine.WaitNextFrame()
		})
	}
	register(&game.scriptEventBindings, "stage")
	register(&back.scriptEventBindings, "back")
	register(&front.scriptEventBindings, "front")

	co.CreateAndStart(false, game, func(coroutine.Thread) int {
		log.add("caller-before")
		game.Broadcast__0("async-order")
		log.add("caller-after")
		return 0
	})

	waitForScratchEventOrderEntries(t, co, &log, 5)
	requireScratchEventOrder(t, &log, []string{
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

func TestScratchBroadcastStopAllPreventsLaterReceivers(t *testing.T) {
	co, game, back, front := setupScratchEventOrderGame(t)
	var log scratchEventOrderLog

	game.OnMsg__1("stop-all", func() { log.add("stage") })
	back.OnMsg__1("stop-all", func() { log.add("back") })
	front.OnMsg__1("stop-all", func() {
		log.add("front")
		front.Stop(AllStop)
	})

	game.Broadcast__0("stop-all")
	co.Update()
	requireScratchEventOrder(t, &log, []string{"front"})
}

func TestScratchCanceledReceiverDoesNotBreakOrderedBatch(t *testing.T) {
	co, game, back, front := setupScratchEventOrderGame(t)
	var log scratchEventOrderLog

	front.OnMsg__1("cancel-middle", func() {
		log.add("front-first")
		front.Stop(OtherScriptsInSprite)
	})
	front.OnMsg__1("cancel-middle", func() { log.add("front-canceled") })
	back.OnMsg__1("cancel-middle", func() { log.add("back") })
	game.OnMsg__1("cancel-middle", func() { log.add("stage") })

	game.Broadcast__0("cancel-middle")
	waitForScratchEventOrderEntries(t, co, &log, 3)
	requireScratchEventOrder(t, &log, []string{"front-first", "back", "stage"})
}

func TestScratchConditionsEvaluateFrontToBackBeforeAnyHandler(t *testing.T) {
	co, game, back, front := setupScratchEventOrderGame(t)
	var log scratchEventOrderLog

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

	game.scriptEvents.doWhenCondition()
	waitForScratchEventOrderEntries(t, co, &log, 6)
	requireScratchEventOrder(t, &log, []string{
		"check-front",
		"check-back",
		"check-stage",
		"run-front",
		"run-back",
		"run-stage",
	})
}

func TestScratchNestedAsyncBroadcastKeepsOrderAfterFrameDeferral(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	co, game, back, front := setupScratchEventOrderGame(t)
	var log scratchEventOrderLog

	game.OnMsg__1("inner-order", func() { log.add("inner-stage") })
	back.OnMsg__1("inner-order", func() { log.add("inner-back") })
	front.OnMsg__1("inner-order", func() { log.add("inner-front") })
	game.OnMsg__1("outer-order", func() {
		log.add("outer-before")
		game.Broadcast__0("inner-order")
		log.add("outer-after")
	})

	game.Broadcast__0("outer-order")
	waitForScratchEventOrderEntries(t, co, &log, 2)
	requireScratchEventOrder(t, &log, []string{"outer-before", "outer-after"})
	updateRuntimeEventSchedulerUntil(t, co, func() bool {
		return co.GetLastUpdateStats().NextCount >= 3
	})

	itime.Update(0, 0)
	waitForScratchEventOrderEntries(t, co, &log, 5)
	requireScratchEventOrder(t, &log, []string{
		"outer-before",
		"outer-after",
		"inner-front",
		"inner-back",
		"inner-stage",
	})
}
