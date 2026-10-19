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
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	coreevent "github.com/goplus/spx/v3/internal/core/event"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

func setupInputFrameGame(t *testing.T) (*coroutine.Coroutines, *Game) {
	t.Helper()
	itime.ClearTimers()
	itime.Start(nil)
	t.Cleanup(itime.ClearTimers)
	co, game := setupRuntimeEventGame(t)
	game.inputMgr.init(game)
	return co, game
}

func TestOnEngineBeforeUpdateSamplesInputBeforeConditions(t *testing.T) {
	for _, mode := range []string{"scheduler", "direct"} {
		t.Run(mode, func(t *testing.T) {
			co, game, input := setupInputSourceTest(t)
			if mode == "direct" {
				previous := gco
				gco = nil
				t.Cleanup(func() { gco = previous })
			}
			game.lifecycleState.IsRunned.Store(true)
			game.lifecycleState.StartDispatched.Store(true)
			input.point.X, input.point.Y = 23, -17
			frame := itime.Frame()
			samples, handlers := 0, 0
			game.OnCond(func() bool {
				samples++
				if game.MouseX() != input.point.X || game.MouseY() != input.point.Y || itime.Frame() != frame {
					t.Error("condition did not observe current input before the clock advanced")
				}
				return true
			}, func() { handlers++ })
			game.OnEngineBeforeUpdate(1.0 / 30)
			if samples != 1 || handlers != 0 || game.inputMgr.frame.pending == nil {
				t.Fatalf("samples=%d, handlers=%d, pending=%v, want 1, 0, and a prepared frame", samples, handlers, game.inputMgr.frame.pending != nil)
			}
			itime.Update(1.0/30, 30)
			game.dispatchFrameEvents()
			co.Update()
			if handlers != 1 {
				t.Fatalf("condition handlers = %d, want 1", handlers)
			}
		})
	}
}

func TestOnEngineUpdateKeepsInputPendingBeforeStart(t *testing.T) {
	var game Game
	frame := &inputFrame{events: []event{&eventKeyDown{Key: KeyA}}}
	game.inputMgr.frame.pending = frame
	game.OnEngineUpdate(1.0 / 30)
	if game.inputMgr.frame.pending != frame {
		t.Fatal("update consumed input before the game started")
	}
}

func TestOnEngineUpdateDispatchesEmptyInputFrame(t *testing.T) {
	co, game, _ := setupInputSourceTest(t)
	game.lifecycleState.IsRunned.Store(true)
	game.lifecycleState.StartDispatched.Store(true)
	game.camera = &cameraImpl{g: game}
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	samples, conditions, timers := 0, 0, 0
	game.OnCond(func() bool { samples++; return true }, func() { conditions++ })
	game.OnTimer(0.1, func() { timers++ })
	frame := itime.Frame() + 1

	game.OnEngineBeforeUpdate(0.1)
	if pending := game.inputMgr.frame.pending; pending == nil || len(pending.events) != 0 {
		t.Fatalf("prepared input = %+v, want a valid empty frame", pending)
	}
	if samples != 1 || conditions != 0 || timers != 0 {
		t.Fatalf("before clock: samples/conditions/timers = %d/%d/%d, want 1/0/0", samples, conditions, timers)
	}
	itime.Update(0.1, 30)
	for update := range 2 {
		game.OnEngineUpdate(0.1)
		co.Update()
		if samples != 1 || conditions != 1 || timers != 1 {
			t.Fatalf("update %d: samples/conditions/timers = %d/%d/%d, want 1/1/1", update+1, samples, conditions, timers)
		}
		if game.inputMgr.frame.pending != nil || itime.Frame() != frame {
			t.Fatal("update retained the consumed frame or advanced the clock")
		}
	}
}

func TestOnEngineUpdateStopsAfterReplayEOF(t *testing.T) {
	co, game, _ := setupInputSourceTest(t)
	if _, err := PrepareInputReplay(validRuntimeReplay(InputReplayFrame{
		Frame:     0,
		State:     InputReplayState{KeysDown: []int64{int64(KeyA)}},
		KeyEvents: []InputReplayKeyEvent{{Key: int64(KeyA), Pressed: true}},
	})); err != nil {
		t.Fatal(err)
	}
	session := claimPreparedSession(t, game)
	game.lifecycleState.IsRunned.Store(true)
	game.lifecycleState.StartDispatched.Store(true)
	samples, conditions, keys := 0, 0, 0
	game.OnCond(func() bool { samples++; return true }, func() { conditions++ })
	game.OnKey__0(KeyA, func() { keys++ })
	game.OnEngineBeforeUpdate(1.0 / 30)
	itime.Update(1.0/30, 30)
	game.dispatchFrameEvents()
	co.Update()
	session.endFrame()
	if completed, err := session.completeReplayFrame(nil); err != nil || !completed {
		t.Fatalf("replay completion = %v, %v", completed, err)
	}

	game.OnEngineBeforeUpdate(1.0 / 30)
	itime.Update(1.0/30, 30)
	game.OnEngineUpdate(1.0 / 30)
	co.Update()
	if samples != 1 || conditions != 1 || keys != 1 {
		t.Fatalf("samples/conditions/keys=%d/%d/%d, want 1/1/1", samples, conditions, keys)
	}
	if game.inputMgr.frame.pending != nil {
		t.Fatal("replay EOF prepared another input frame")
	}
}

func TestDispatchFrameEventsRegistersLargeInputBatchBeforeReturn(t *testing.T) {
	co, game := setupInputFrameGame(t)
	const count = 26 // More than the former 16-event channel capacity.
	calls := make(chan Key, count*2)
	var routed, want []Key
	game.scriptEvents.manager.Add(coreevent.BucketAnyKeyPressed, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, value.(Key))
		return false
	}))
	frame := &inputFrame{}
	for i := range count {
		key := KeyA + Key(i)
		want = append(want, key)
		game.OnKey__1([]Key{key}, func(key Key) { calls <- key })
		frame.events = append(frame.events, &eventKeyDown{Key: key})
	}
	game.inputMgr.frame.pending = frame
	game.dispatchFrameEvents()
	if !slices.Equal(routed, want) {
		t.Fatalf("routed keys before return = %v, want %v", routed, want)
	}
	if game.inputMgr.frame.pending != nil {
		t.Fatal("dispatch retained its consumed input batch")
	}
	co.Update()
	seen := make(map[Key]bool, count)
	for range count {
		select {
		case key := <-calls:
			if seen[key] {
				t.Fatalf("input key %v dispatched twice", key)
			}
			seen[key] = true
		default:
			t.Fatalf("received %d distinct keys, want %d", len(seen), count)
		}
	}
	game.dispatchFrameEvents()
	co.Update()
	if !slices.Equal(routed, want) {
		t.Fatalf("consumed input batch routed again: %v", routed)
	}
	select {
	case key := <-calls:
		t.Fatalf("consumed input batch dispatched key %v again", key)
	default:
	}
}

func TestDispatchFrameEventsDoesNotWaitForFutureHandlerFrames(t *testing.T) {
	co, game := setupInputFrameGame(t)
	started, finished := make(chan struct{}), make(chan struct{})
	game.OnKey__0(KeySpace, func() {
		close(started)
		engine.WaitNextFrame()
		close(finished)
	})
	game.inputMgr.frame.pending = &inputFrame{events: []event{&eventKeyDown{Key: KeySpace}}}
	frame := itime.Frame()
	returned := make(chan struct{})
	go func() {
		game.dispatchFrameEvents()
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("input dispatch waited for a handler's future frame")
	}
	if itime.Frame() != frame {
		t.Fatal("input dispatch advanced the frame clock")
	}
	co.Update()
	select {
	case <-started:
	default:
		t.Fatal("handler did not start in the current frame")
	}
	select {
	case <-finished:
		t.Fatal("handler resumed its next-frame wait in the current frame")
	default:
	}
	itime.Update(1.0/30, 30)
	co.Update()
	select {
	case <-finished:
	default:
		t.Fatal("handler did not resume in the following frame")
	}
}

func TestDispatchFrameEventsUsesAdvancedClockForDueTimers(t *testing.T) {
	_, game := setupInputFrameGame(t)
	itime.RegisterTimer(0.1)
	itime.RegisterTimer(0.3)
	var routed []int64
	var frames []int64
	game.scriptEvents.manager.Add(coreevent.BucketAnyKeyPressed, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, int64(value.(Key)))
		frames = append(frames, itime.Frame())
		return false
	}))
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, value.(int64))
		frames = append(frames, itime.Frame())
		return false
	}))
	game.inputMgr.frame.pending = &inputFrame{events: []event{&eventKeyDown{Key: KeyA}}}
	frame := itime.Frame() + 1
	itime.Update(0.2, 30)
	game.dispatchFrameEvents()
	if want := []int64{int64(KeyA), 100}; !slices.Equal(routed, want) {
		t.Fatalf("input and timer routing = %v, want %v", routed, want)
	}
	if !slices.Equal(frames, []int64{frame, frame}) {
		t.Fatalf("routing frames = %v, want both in frame %d", frames, frame)
	}
	if timestamp, ok := itime.NextTimer(); ok {
		t.Fatalf("future timer %d became eligible before its frame", timestamp)
	}
}

func TestDueTimerCanRunBeforePendingLogicFrame(t *testing.T) {
	co, game := setupInputFrameGame(t)
	backend := &animationAudioBackend{}
	game.soundMgr.Init(backend)
	game.sounds = map[string]sound{"step": &coreproject.SoundConfig{Path: "step.wav"}}
	anim := newTestAnimationComponent()
	anim.sprite.g = game
	anim.sprite.sound().pendingAudios = []string{"step"}
	anim.curAnimState = &animState{Name: "step"}
	anim.doneAnimations = []*animState{anim.curAnimState}
	game.shapeMgr.items = []Shape{anim.sprite}
	var phases eventOrderLog
	backend.onPlay = func(id int64) {
		phases.add("audio")
		anim.curAnimState.OnPlayAudioPlaybackID = id
	}
	backend.onStop = func(int64) { phases.add("animation-complete") }
	logic := co.Create("logicLoop", func(coroutine.Thread) {
		engine.WaitNextFrame()
		game.processLogicFrame(nil, nil)
	})
	co.JoinFirstSlice(logic)
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(any) bool {
		phases.add("timer-route")
		return false
	}))
	handled := make(chan struct{})
	game.OnTimer(0.1, func() {
		phases.add("timer-handler")
		close(handled)
	})
	itime.Update(0.1, 30)
	frame := itime.Frame()
	game.inputMgr.frame.pending = &inputFrame{}
	game.dispatchFrameEvents()
	// A newly registered timer can run while the previous frame's logic task
	// remains queued for Update; audio/animation completion is not a prerequisite.
	select {
	case <-handled:
	case <-time.After(time.Second):
		t.Fatal("timer handler waited for the pending logic frame")
	}
	requireEventOrder(t, &phases, []string{"timer-route", "timer-handler"})
	co.Update()
	co.Join(logic)
	requireEventOrder(t, &phases, []string{"timer-route", "timer-handler", "audio", "animation-complete"})
	if itime.Frame() != frame || anim.curAnimState != nil || len(anim.doneAnimations) != 0 {
		t.Fatal("pending logic did not finish its animation in the timer's frame")
	}
}

func TestZeroTimerRoutesBeforeStartRegistration(t *testing.T) {
	co, game := setupInputFrameGame(t)
	game.lifecycleState.IsRunned.Store(true)
	game.completeBootstrap(game.bootstrapGeneration())
	game.camera = &cameraImpl{g: game}
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	routed, timers, starts := 0, 0, 0
	frame := itime.Frame() + 1
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(any) bool {
		routed++
		if game.startScheduled || game.lifecycleState.StartDispatched.Load() || itime.Frame() != frame {
			t.Error("zero timer did not route before startup registration in the advanced frame")
		}
		return false
	}))
	game.OnTimer(0, func() { timers++ })
	game.OnStart(func() {
		starts++
		if routed != 1 {
			t.Error("OnStart ran before the zero timer was routed")
		}
	})
	game.inputMgr.frame.pending = &inputFrame{}
	itime.Update(1.0/30, 30)
	game.OnEngineUpdate(1.0 / 30)
	co.Update()
	// Both handlers run; their order relative to each other is not the contract.
	if routed != 1 || timers != 1 || starts != 1 || !game.lifecycleState.StartDispatched.Load() {
		t.Fatalf("timer routes/handlers/start handlers = %d/%d/%d, start dispatched = %v",
			routed, timers, starts, game.lifecycleState.StartDispatched.Load())
	}
}

func TestDispatchFrameEventsFreezesTimersBeforeRouting(t *testing.T) {
	_, game := setupInputFrameGame(t)
	itime.RegisterTimer(0)
	var routed []int64
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, value.(int64))
		if len(routed) == 1 {
			// A newly registered overdue timer and a reset are eligible for the
			// following batch, even though they happen during event routing.
			itime.RegisterTimer(-0.1)
			itime.ResetTimer()
		}
		return false
	}))
	game.inputMgr.frame.pending = &inputFrame{}
	game.dispatchFrameEvents()
	if !slices.Equal(routed, []int64{0}) {
		t.Fatalf("first timer batch = %v, want [0]", routed)
	}
	game.inputMgr.frame.pending = &inputFrame{}
	game.dispatchFrameEvents()
	if !slices.Equal(routed, []int64{0, -100, 0}) {
		t.Fatalf("timer batches = %v, want [0 -100 0]", routed)
	}
}

func TestDispatchFrameEventsSurvivesStopOtherScripts(t *testing.T) {
	co, game := setupInputFrameGame(t)
	started, unblock := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(unblock) })
	t.Cleanup(release)
	blocker := co.Create(game, func(coroutine.Thread) {
		close(started)
		<-unblock
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("script did not acquire execution before input dispatch")
	}
	routed := false
	game.scriptEvents.manager.Add(coreevent.BucketAnyKeyPressed, coreevent.NewSink(game, nil, func(any) bool {
		routed = true
		return false
	}))
	game.inputMgr.frame.pending = &inputFrame{events: []event{&eventKeyDown{Key: KeySpace}}}
	returned := make(chan struct{})
	go func() {
		game.dispatchFrameEvents()
		close(returned)
	}()

	// Observe registration while the blocker still owns execution. The filter
	// does not stop anything; it avoids relying on goroutine scheduling order.
	var dispatcher coroutine.Thread
	deadline := time.Now().Add(time.Second)
	for dispatcher == nil {
		co.StopIf(func(thread coroutine.Thread) bool {
			if thread.ID() > blocker.ID() {
				dispatcher = thread
			}
			return false
		})
		if time.Now().After(deadline) {
			t.Fatal("frame dispatcher was not registered")
		}
		runtime.Gosched()
	}
	game.Stop(AllOtherScripts)
	if !blocker.Stopped() {
		t.Error("Stop(AllOtherScripts) did not stop the game script")
	}
	if dispatcher.Stopped() {
		t.Error("Stop(AllOtherScripts) canceled the pending frame dispatcher")
	}
	release()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("frame dispatch did not finish after the script released execution")
	}
	if !routed {
		t.Fatal("stopping game scripts discarded the frame's input")
	}
}

func TestDispatchFrameEventsFreezesTimersBeforeConditions(t *testing.T) {
	_, game := setupInputFrameGame(t)
	previous := gco
	gco = nil
	t.Cleanup(func() { gco = previous })
	// Run callbacks inline so a condition handler deterministically mutates the
	// timer state during dispatch, rather than depending on scheduler timing.
	conditions := 0
	game.OnCond(func() bool { return true }, func() {
		conditions++
		itime.ResetTimer()
		itime.RegisterTimer(0)
	})
	itime.RegisterTimer(0.1)
	var routed []int64
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, value.(int64))
		return false
	}))
	game.scriptEvents.sampleConditions()
	itime.Update(0.2, 30)
	game.inputMgr.frame.pending = &inputFrame{}
	game.dispatchFrameEvents()
	if conditions != 1 || !slices.Equal(routed, []int64{100}) {
		t.Fatalf("condition calls=%d, first timer batch=%v, want 1 and [100]", conditions, routed)
	}
	game.inputMgr.frame.pending = &inputFrame{}
	game.dispatchFrameEvents()
	if conditions != 1 || !slices.Equal(routed, []int64{100, 0}) {
		t.Fatalf("condition calls=%d, timer batches=%v, want 1 and [100 0]", conditions, routed)
	}
}
