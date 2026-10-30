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

	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	itime "github.com/goplus/spx/v4/internal/time"
)

type eventOrderLog struct {
	mu      sync.Mutex
	entries []string
}

func BenchmarkBroadcastAndWaitFanout(b *testing.B) {
	for _, handlers := range []int{1, 3, 10} {
		b.Run(fmt.Sprintf("300-targets/%d-handlers", handlers), func(b *testing.B) {
			co := coroutine.New(nil)
			installRuntimeScheduler(b, co, nil)
			game := new(Game)
			game.bindScriptEvents()
			game.initShapeMgr()
			for i := range 300 {
				sprite := newEventOrderSprite(game, fmt.Sprintf("target-%d", i))
				game.shapeMgr.add(sprite)
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
	clone.spriteState.cloned = true
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
			co, game := setupInputFrameGame(t)
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
				itime.Update(1.0/30, 30)
				batch := &inputFrame{}
				for _, key := range edges {
					batch.events = append(batch.events, &eventKeyDown{Key: key})
				}
				game.inputMgr.frame.pending = batch
				game.dispatchFrameEvents()
				co.Update()
				want = append(want, edges[0])
				if !slices.Equal(got, want) {
					t.Fatalf("handler keys = %v, want %v", got, want)
				}
			}
		})
	}
}

func TestFrameBatchSharedKeyHandlerIgnoresInputUntilPreviousInvocationFinishes(t *testing.T) {
	co, game := setupInputFrameGame(t)
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
		itime.Update(1.0/30, 30)
		game.inputMgr.frame.pending = &inputFrame{events: []event{&eventKeyDown{Key: key}}}
		game.dispatchFrameEvents()
		co.Update()
		if !slices.Equal(got, want[frame]) || finished != (frame > 0) {
			t.Fatalf("frame %d: handler keys = %v, first finished = %v, want %v and %v",
				frame, got, finished, want[frame], frame > 0)
		}
	}
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
	setupCaptureTest(t)

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

func TestBroadcastAndWaitResumesBeforeRestartedEarlierReceiver(t *testing.T) {
	for _, stopOthers := range []bool{false, true} {
		t.Run(fmt.Sprintf("stop-others=%t", stopOthers), func(t *testing.T) {
			co, game, actor, _ := setupEventOrderGame(t)
			blocked := co.NewLatch()
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
			co.Create(actor, func(coroutine.Thread) {
				game.Broadcast__0("child")
				engine.WaitNextFrame()
				game.BroadcastAndWait__0("outer")
				log.add("caller-resumed")
			})
			co.Update()
			requireEventOrder(t, &log, []string{"child-started"})

			// Restarting preserves child's earlier thread slot, which this
			// round already visited. The caller resumes before child runs again.
			itime.Update(0, 0)
			co.Update()
			want := []string{"child-started", "outer-returned", "caller-resumed", "child-started"}
			if stopOthers {
				want = append(want, "child-stopped-others")
			}
			requireEventOrder(t, &log, want)
		})
	}
}

func TestBroadcastAndWaitResumesSiblingReceiversInThreadOrder(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	var log eventOrderLog
	for _, name := range []string{"A", "B"} {
		game.OnMsg__1("fan-out", func() {
			log.add(name + "-entered")
			game.BroadcastAndWait__0("inner-" + name)
			log.add(name + "-resumed")
		})
		game.OnMsg__1("inner-"+name, func() { log.add("inner-" + name) })
	}
	co.Create(game, func(coroutine.Thread) {
		game.BroadcastAndWait__0("fan-out")
		log.add("caller-resumed")
	})
	co.Update()

	// Both inner receivers finish in one round. Their callers retain A/B
	// thread order when the next round visits their waiting broadcasts.
	requireEventOrder(t, &log, []string{
		"A-entered", "B-entered", "inner-A", "inner-B",
		"A-resumed", "B-resumed", "caller-resumed",
	})
}

func TestBroadcastAndWaitPreservesQueuedCleanupOrder(t *testing.T) {
	co, game, panel, transition := setupEventOrderGame(t)
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

	co.Create(game, func(coroutine.Thread) {
		game.BroadcastAndWait__0("prepare-level")
		log.add("caller-resumed")
		game.Broadcast__0("open-level")
	})
	co.Update()

	// Cleanup already queued by the completed receiver must get its turn
	// before the caller can register the panel's next script for cancellation.
	requireEventOrder(t, &log, []string{
		"preparation-returned", "panel-cleared", "caller-resumed", "level-opened",
	})
}

func TestBroadcastAndWaitKeepsAsyncSenderAtomic(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	var log eventOrderLog
	value := 0
	game.OnMsg__1("publish", func() {
		game.Broadcast__0("observe")
		value = 1
		log.add("sender-updated")
	})
	game.OnMsg__1("observe", func() { log.add(fmt.Sprintf("observed-%d", value)) })

	co.Create(game, func(coroutine.Thread) {
		game.BroadcastAndWait__0("publish")
	})
	co.Update()
	requireEventOrder(t, &log, []string{"sender-updated", "observed-1"})
}

func TestBroadcastAndWaitDoesNotExposeSenderToEarlyStop(t *testing.T) {
	co, game, actor, _ := setupEventOrderGame(t)
	var log eventOrderLog
	actor.OnMsg__1("work", func() {
		game.Broadcast__0("cancel-actor-work")
		log.add("sender-returned")
	})
	actor.OnMsg__1("cancel-actor-work", func() {
		log.add("other-scripts-stopped")
		actor.Stop(OtherScriptsInSprite)
	})

	co.Create(game, func(coroutine.Thread) {
		game.BroadcastAndWait__0("work")
	})
	co.Update()
	requireEventOrder(t, &log, []string{"sender-returned", "other-scripts-stopped"})
}

func TestBroadcastSiblingWaitDoesNotYieldAsyncSender(t *testing.T) {
	co, game, publisher, waiter := setupEventOrderGame(t)
	release := co.NewLatch()
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
	co.Update()
	requireEventOrder(t, &log, []string{"waiter-entered", "publisher-updated", "observed-1"})

	release.Open()
	co.Update()
	requireEventOrder(t, &log, []string{
		"waiter-entered", "publisher-updated", "observed-1", "waiter-returned",
	})
}

func TestBroadcastAndWaitRunsQueuedDescendantsWithoutJoiningThem(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	release := co.NewLatch()
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
	co.Create(game, func(coroutine.Thread) {
		game.BroadcastAndWait__0("parent")
		log.add("caller-resumed")
	})
	co.Update()

	// Every newly queued script gets its first turn, but the wait belongs
	// only to the direct receiver and must not join its blocked descendants.
	// A descendant can still resume and broadcast after the caller returns.
	requireEventOrder(t, &log, []string{
		"parent-returned", "child-returned", "grandchild-started", "caller-resumed",
	})
	release.Open()
	co.Update()
	requireEventOrder(t, &log, []string{
		"parent-returned", "child-returned", "grandchild-started", "caller-resumed", "grandchild-returned", "notification-ran",
	})
}

func TestExternalBroadcastAndWaitWaitsForReceiverCompletion(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	release := co.NewLatch()
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
	updateRuntimeEventSchedulerUntil(t, co, func() bool {
		select {
		case <-entered:
			return true
		default:
			return false
		}
	})
	co.Update()
	requireEventOrder(t, &log, []string{"child-ran"})
	select {
	case <-returned:
		t.Fatal("external broadcast returned while its receiver was blocked")
	default:
	}

	release.Open()
	co.Update()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("external broadcast did not return after its receiver completed")
	}
	requireEventOrder(t, &log, []string{"child-ran", "receiver-returned"})
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
	setupCaptureTest(t)

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
