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
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/core/clock"
	coreevent "github.com/goplus/spx/v4/internal/core/event"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	coreruntime "github.com/goplus/spx/v4/internal/core/runtime"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type inputSourceTestManager struct {
	pkgengine.IInputMgr
	point   mathf.Vec2
	samples int
}

type inputFrameFailureExtMgr struct {
	replayPauseTestExtMgr
	messages []string
	exits    []int64
}

type inputFrameFailurePlatform struct{ pkgengine.IPlatformMgr }

type replayQueryInputMgr struct{ *inputSourceTestManager }

type dragSpriteMgr struct {
	*clickThroughSpriteMgr
	hit func(pkgengine.Object, mathf.Vec2) bool
}

type dragCameraMgr struct {
	pkgengine.ICameraMgr
	viewport, world mathf.Rect2
}

func TestOnEngineBeforeUpdateSamplesInputBeforeConditions(t *testing.T) {
	for _, mode := range []string{"scheduler", "direct"} {
		t.Run(mode, func(t *testing.T) {
			currentScheduler, game, input := setupInputSourceTest(t)
			if mode == "direct" {
				previous := scheduler
				scheduler = nil
				t.Cleanup(func() { scheduler = previous })
			}
			game.lifecycle.loaded.Store(true)
			game.lifecycle.startDispatched.Store(true)
			input.point.X, input.point.Y = 23, -17
			frame := clock.Frame()
			samples, handlers := 0, 0
			game.OnCond(func() bool {
				samples++
				if game.MouseX() != input.point.X || game.MouseY() != input.point.Y || clock.Frame() != frame {
					t.Error("condition did not observe current input before the clock advanced")
				}
				return true
			}, func() { handlers++ })
			game.OnEngineBeforeUpdate(1.0 / 30)
			if samples != 1 || handlers != 0 || game.inputMgr.frame.pending == nil {
				t.Fatalf("samples=%d, handlers=%d, pending=%v, want 1, 0, and a prepared frame", samples, handlers, game.inputMgr.frame.pending != nil)
			}
			clock.Update(1.0/30, 30)
			game.dispatchFrameEvents()
			currentScheduler.Update()
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
	currentScheduler, game, _ := setupInputSourceTest(t)
	game.lifecycle.loaded.Store(true)
	game.lifecycle.startDispatched.Store(true)
	game.camera = &cameraImpl{g: game}
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	samples, conditions, timers := 0, 0, 0
	game.OnCond(func() bool { samples++; return true }, func() { conditions++ })
	game.OnTimer(0.1, func() { timers++ })
	frame := clock.Frame() + 1

	game.OnEngineBeforeUpdate(0.1)
	if pending := game.inputMgr.frame.pending; pending == nil || len(pending.events) != 0 {
		t.Fatalf("prepared input = %+v, want a valid empty frame", pending)
	}
	if samples != 1 || conditions != 0 || timers != 0 {
		t.Fatalf("before clock: samples/conditions/timers = %d/%d/%d, want 1/0/0", samples, conditions, timers)
	}
	clock.Update(0.1, 30)
	for update := range 2 {
		game.OnEngineUpdate(0.1)
		currentScheduler.Update()
		if samples != 1 || conditions != 1 || timers != 1 {
			t.Fatalf("update %d: samples/conditions/timers = %d/%d/%d, want 1/1/1", update+1, samples, conditions, timers)
		}
		if game.inputMgr.frame.pending != nil || clock.Frame() != frame {
			t.Fatal("update retained the consumed frame or advanced the clock")
		}
	}
}

func TestOnEngineUpdateStopsAfterReplayEOF(t *testing.T) {
	currentScheduler, game, _ := setupInputSourceTest(t)
	if _, err := PrepareInputReplay(validRuntimeReplay(InputReplayFrame{
		Frame:     0,
		State:     InputReplayState{KeysDown: []int64{int64(KeyA)}},
		KeyEvents: []InputReplayKeyEvent{{Key: int64(KeyA), Pressed: true}},
	})); err != nil {
		t.Fatal(err)
	}
	session := claimPreparedSession(t, game)
	game.lifecycle.loaded.Store(true)
	game.lifecycle.startDispatched.Store(true)
	samples, conditions, keys := 0, 0, 0
	game.OnCond(func() bool { samples++; return true }, func() { conditions++ })
	game.OnKey__0(KeyA, func() { keys++ })
	game.OnEngineBeforeUpdate(1.0 / 30)
	clock.Update(1.0/30, 30)
	game.dispatchFrameEvents()
	currentScheduler.Update()
	session.endFrame()
	if completed, err := session.completeReplayFrame(nil); err != nil || !completed {
		t.Fatalf("replay completion = %v, %v", completed, err)
	}

	game.OnEngineBeforeUpdate(1.0 / 30)
	clock.Update(1.0/30, 30)
	game.OnEngineUpdate(1.0 / 30)
	currentScheduler.Update()
	if samples != 1 || conditions != 1 || keys != 1 {
		t.Fatalf("samples/conditions/keys=%d/%d/%d, want 1/1/1", samples, conditions, keys)
	}
	if game.inputMgr.frame.pending != nil {
		t.Fatal("replay EOF prepared another input frame")
	}
}

func TestDispatchFrameEventsRegistersLargeInputBatchBeforeReturn(t *testing.T) {
	currentScheduler, game := setupInputFrameGame(t)
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
	currentScheduler.Update()
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
	currentScheduler.Update()
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
	currentScheduler, game := setupInputFrameGame(t)
	started, finished := make(chan struct{}), make(chan struct{})
	game.OnKey__0(KeySpace, func() {
		close(started)
		engine.WaitNextFrame()
		close(finished)
	})
	game.inputMgr.frame.pending = &inputFrame{events: []event{&eventKeyDown{Key: KeySpace}}}
	frame := clock.Frame()
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
	if clock.Frame() != frame {
		t.Fatal("input dispatch advanced the frame clock")
	}
	currentScheduler.Update()
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
	clock.Update(1.0/30, 30)
	currentScheduler.Update()
	select {
	case <-finished:
	default:
		t.Fatal("handler did not resume in the following frame")
	}
}

func TestDispatchFrameEventsUsesAdvancedClockForDueTimers(t *testing.T) {
	_, game := setupInputFrameGame(t)
	clock.RegisterTimer(0.1)
	clock.RegisterTimer(0.3)
	var routed []int64
	var frames []int64
	game.scriptEvents.manager.Add(coreevent.BucketAnyKeyPressed, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, int64(value.(Key)))
		frames = append(frames, clock.Frame())
		return false
	}))
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, value.(int64))
		frames = append(frames, clock.Frame())
		return false
	}))
	game.inputMgr.frame.pending = &inputFrame{events: []event{&eventKeyDown{Key: KeyA}}}
	frame := clock.Frame() + 1
	clock.Update(0.2, 30)
	game.dispatchFrameEvents()
	if want := []int64{int64(KeyA), 100}; !slices.Equal(routed, want) {
		t.Fatalf("input and timer routing = %v, want %v", routed, want)
	}
	if !slices.Equal(frames, []int64{frame, frame}) {
		t.Fatalf("routing frames = %v, want both in frame %d", frames, frame)
	}
	if timestamp, ok := clock.NextTimer(); ok {
		t.Fatalf("future timer %d became eligible before its frame", timestamp)
	}
}

func TestDueTimerCanRunBeforePendingLogicFrame(t *testing.T) {
	currentScheduler, game := setupInputFrameGame(t)
	backend := &animationAudioBackend{}
	game.soundMgr.Init(backend)
	game.sounds = map[string]sound{"step": &coreproject.SoundConfig{Path: "step.wav"}}
	anim := newTestAnimationComponent()
	anim.sprite.g = game
	anim.sprite.animation().pendingAudios = []string{"step"}
	anim.curAnimState = &animationPlayback{Name: "step"}
	anim.doneAnimations = []*animationPlayback{anim.curAnimState}
	game.scene.items = []Shape{anim.sprite}
	var phases eventOrderLog
	backend.onPlay = func(id int64) {
		phases.add("audio")
		anim.curAnimState.OnPlayAudioPlaybackID = id
	}
	backend.onStop = func(int64) { phases.add("animation-complete") }
	logic := currentScheduler.Create("logicLoop", func(coroutine.Thread) {
		engine.WaitNextFrame()
		game.processLogicFrame(nil, nil)
	})
	currentScheduler.JoinFirstSlice(logic)
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(any) bool {
		phases.add("timer-route")
		return false
	}))
	handled := make(chan struct{})
	game.OnTimer(0.1, func() {
		phases.add("timer-handler")
		close(handled)
	})
	clock.Update(0.1, 30)
	frame := clock.Frame()
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
	currentScheduler.Update()
	currentScheduler.Join(logic)
	requireEventOrder(t, &phases, []string{"timer-route", "timer-handler", "audio", "animation-complete"})
	if clock.Frame() != frame || anim.curAnimState != nil || len(anim.doneAnimations) != 0 {
		t.Fatal("pending logic did not finish its animation in the timer's frame")
	}
}

func TestZeroTimerRoutesBeforeStartRegistration(t *testing.T) {
	currentScheduler, game := setupInputFrameGame(t)
	game.lifecycle.loaded.Store(true)
	game.lifecycle.completeBootstrap(game.lifecycle.generation())
	game.camera = &cameraImpl{g: game}
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	routed, timers, starts := 0, 0, 0
	frame := clock.Frame() + 1
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(any) bool {
		routed++
		if game.lifecycle.startScheduled || game.lifecycle.startDispatched.Load() || clock.Frame() != frame {
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
	clock.Update(1.0/30, 30)
	game.OnEngineUpdate(1.0 / 30)
	currentScheduler.Update()
	// Both handlers run; their order relative to each other is not the contract.
	if routed != 1 || timers != 1 || starts != 1 || !game.lifecycle.startDispatched.Load() {
		t.Fatalf("timer routes/handlers/start handlers = %d/%d/%d, start dispatched = %v",
			routed, timers, starts, game.lifecycle.startDispatched.Load())
	}
}

func TestDispatchFrameEventsFreezesTimersBeforeRouting(t *testing.T) {
	_, game := setupInputFrameGame(t)
	clock.RegisterTimer(0)
	var routed []int64
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, value.(int64))
		if len(routed) == 1 {
			// A newly registered overdue timer and a reset are eligible for the
			// following batch, even though they happen during event routing.
			clock.RegisterTimer(-0.1)
			clock.ResetTimer()
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
	currentScheduler, game := setupInputFrameGame(t)
	started, unblock := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(unblock) })
	t.Cleanup(release)
	blocker := currentScheduler.Create(game, func(coroutine.Thread) {
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
		currentScheduler.StopIf(func(thread coroutine.Thread) bool {
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
	previous := scheduler
	scheduler = nil
	t.Cleanup(func() { scheduler = previous })
	// Run callbacks inline so a condition handler deterministically mutates the
	// timer state during dispatch, rather than depending on scheduler timing.
	conditions := 0
	game.OnCond(func() bool { return true }, func() {
		conditions++
		clock.ResetTimer()
		clock.RegisterTimer(0)
	})
	clock.RegisterTimer(0.1)
	var routed []int64
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, value.(int64))
		return false
	}))
	game.scriptEvents.sampleConditions()
	clock.Update(0.2, 30)
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

func TestKeyFromStringRecognizesExclam(t *testing.T) {
	if got := KeyFromString("!"); got != KeyExclam {
		t.Fatalf("KeyFromString(\"!\") = %v, want %v", got, KeyExclam)
	}
}

func TestMouseCoordinatesUseLatestLiveInput(t *testing.T) {
	_, game, mgr := setupInputSourceTest(t)
	for _, point := range []mathf.Vec2{{X: 12.5, Y: -8}, {X: -20, Y: 31.25}} {
		mgr.point = point
		if !game.inputMgr.prepareInputFrame(1.0 / 30) {
			t.Fatal("live input frame was not prepared")
		}
		if x, y := game.MouseX(), game.MouseY(); x != point.X || y != point.Y {
			t.Fatalf("mouse coordinates = (%v, %v), want (%v, %v)", x, y, point.X, point.Y)
		}
	}

	game.inputMgr.init(game)
	if x, y := game.MouseX(), game.MouseY(); x != 0 || y != 0 {
		t.Fatalf("mouse coordinates after reinitialization = (%v, %v), want (0, 0)", x, y)
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

func TestReplayMousePressedMatchesLiveButtonSemantics(t *testing.T) {
	resetInputSessionTest(t)
	replay := validRuntimeReplay()
	replay.Initial.Buttons = 1 << 2
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	game.inputMgr.g = game
	claimPreparedSession(t, game)
	if game.inputMgr.effectiveMousePressed() {
		t.Fatal("middle-only replay input was treated as live MousePressed")
	}
}

func TestPrepareInputFrameSharesLiveRecordingAndReplayRouting(t *testing.T) {
	points := []mathf.Vec2{{}, {X: 5, Y: 6}, {X: 5.5, Y: 6}, {X: 9, Y: 12}}
	for _, mode := range []string{"live", "recording", "replay"} {
		t.Run(mode, func(t *testing.T) {
			_, game, mgr := setupInputSourceTest(t)
			var session *inputSession
			switch mode {
			case "recording":
				if _, err := PrepareInputRecording(30); err != nil {
					t.Fatal(err)
				}
				session = claimPreparedSession(t, game)
			case "replay":
				frames := make([]InputReplayFrame, len(points))
				for i, point := range points {
					frames[i] = InputReplayFrame{
						Frame: int64(i), Time: float64(i) / 30,
						State: InputReplayState{Mouse: InputReplayMouse{X: point.X, Y: point.Y}},
					}
				}
				if _, err := PrepareInputReplay(validRuntimeReplay(frames...)); err != nil {
					t.Fatal(err)
				}
				session = claimPreparedSession(t, game)
			}
			for i, point := range points {
				mgr.point = point
				if mode == "replay" {
					// Playback must use the recording even when real input disagrees.
					mgr.point = mathf.Vec2{X: 100, Y: -100}
				}
				if !game.inputMgr.prepareInputFrame(1.0 / 30) {
					t.Fatalf("frame %d was not prepared", i)
				}
				batch := game.inputMgr.frame.pending
				if batch == nil {
					t.Fatalf("frame %d has no input batch", i)
				}
				if len(batch.events) != 0 {
					t.Fatalf("frame %d generated events for pointer movement: %+v", i, batch.events)
				}
				if got := game.inputMgr.mousePos; got != point {
					t.Fatalf("frame %d mouse = %v, want %v", i, got, point)
				}
				game.dispatchFrameEvents()
				if game.inputMgr.frame.pending != nil {
					t.Fatal("dispatch retained its prepared input frame")
				}
				if session != nil {
					session.endFrame()
				}
			}
			if mode == "recording" {
				recorded, err := session.controller.Recording()
				if err != nil {
					t.Fatal(err)
				}
				for i, frame := range recorded.Frames {
					if frame.MouseEvents == nil || frame.KeyEvents == nil {
						t.Fatalf("frame %d changed empty edge slices to nil", i)
					}
				}
			}
		})
	}
}

func TestPrepareInputFrameFreezesMouseUntilNextSample(t *testing.T) {
	_, game, mgr := setupInputSourceTest(t)
	first, next := mathf.Vec2{X: 7, Y: 8}, mathf.Vec2{X: 40, Y: -30}
	mgr.point = first
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("first frame was not prepared")
	}
	mgr.point = next
	game.dispatchFrameEvents()
	if game.MouseX() != first.X || game.MouseY() != first.Y || mgr.samples != 1 {
		t.Fatal("input arriving after prepare changed the frozen frame")
	}
	if game.inputMgr.frame.pending != nil {
		t.Fatal("dispatch retained its prepared input frame")
	}
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("next frame was not prepared")
	}
	if game.MouseX() != next.X || game.MouseY() != next.Y || mgr.samples != 2 {
		t.Fatal("next frame did not sample the latest position")
	}
}

func TestPrepareInputFrameRejectsCompletedSessionWithoutDispatchingPendingInput(t *testing.T) {
	currentScheduler, game, mgr := setupInputSourceTest(t)
	setupCaptureTest(t)
	replay := validRuntimeReplay(InputReplayFrame{
		Frame: 0,
		State: InputReplayState{
			Mouse: InputReplayMouse{X: 7, Y: 8}, KeysDown: []int64{int64(KeyA)},
		},
		KeyEvents: []InputReplayKeyEvent{{Key: int64(KeyA), Pressed: true}},
	})
	if _, err := PrepareInputReplay(replay, InputSessionOptions{CaptureKey: KeyA}); err != nil {
		t.Fatal(err)
	}
	session := claimPreparedSession(t, game)
	calls := 0
	game.OnKey__0(KeyA, func() { calls++ })
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("replay frame was not prepared")
	}
	if pending := game.inputMgr.frame.pending; len(pending.events) == 0 || pending.captures != 1 {
		t.Fatalf("pending frame = %+v, want input and a screenshot request", pending)
	}
	session.endFrame()
	if completed, err := session.completeReplayFrame(nil); err != nil || !completed {
		t.Fatalf("complete replay = %v, %v", completed, err)
	}
	history, status := game.inputMgr.frame.history, session.status()
	point, samples := game.inputMgr.mousePos, mgr.samples
	mgr.point = mathf.Vec2{X: 40, Y: -30}

	if game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("completed replay accepted another frame")
	}
	if game.inputMgr.frame.pending != nil {
		t.Fatal("rejected frame retained the previous input batch")
	}
	if mgr.samples != samples || session.status() != status {
		t.Fatal("rejected frame sampled live input or advanced the replay")
	}
	if game.inputMgr.frame.history != history || !game.inputMgr.frame.hadSession || game.inputMgr.mousePos != point {
		t.Fatal("rejected frame changed input history or mouse position")
	}
	game.dispatchFrameEvents()
	currentScheduler.Update()
	if calls != 0 || engine.HasPendingCaptures() {
		t.Fatal("rejected frame dispatched old input or screenshot requests")
	}
}

func TestPrepareInputFrameReportsInvalidRecordingSample(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("Web runtime errors reset the game asynchronously instead of requesting exit")
	}
	for _, tt := range []struct {
		name    string
		point   mathf.Vec2
		delta   float64
		message string
	}{
		{"invalid mouse", mathf.Vec2{X: math.NaN()}, 1.0 / 30, "mouse coordinates must be finite"},
		{"invalid delta", mathf.Vec2{X: 40, Y: -30}, math.Inf(1), "live delta must be finite and non-negative"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			currentScheduler, game, mgr := setupInputSourceTest(t)
			setupCaptureTest(t)
			originalExt, originalPlatform := pkgengine.ExtMgr, pkgengine.PlatformMgr
			recorder := &inputFrameFailureExtMgr{}
			pkgengine.ExtMgr = recorder
			pkgengine.PlatformMgr = inputFrameFailurePlatform{originalPlatform}
			t.Cleanup(func() {
				pkgengine.ExtMgr, pkgengine.PlatformMgr = originalExt, originalPlatform
			})
			if _, err := PrepareInputRecording(30); err != nil {
				t.Fatal(err)
			}
			session := claimPreparedSession(t, game)
			mgr.point = mathf.Vec2{X: 7, Y: 8}
			if !game.inputMgr.prepareInputFrame(1.0 / 30) {
				t.Fatal("initial recording frame was not prepared")
			}
			session.endFrame()
			history, status := game.inputMgr.frame.history, session.status()
			point := game.inputMgr.mousePos
			game.inputMgr.frame.pending = &inputFrame{
				events: []event{&eventKeyDown{Key: KeyA}}, captures: 1,
			}
			calls := 0
			game.OnKey__0(KeyA, func() { calls++ })
			mgr.point = tt.point

			if game.inputMgr.prepareInputFrame(tt.delta) {
				t.Fatal("invalid recording sample was accepted")
			}
			if len(recorder.messages) != 1 || !strings.Contains(recorder.messages[0], tt.message) || !reflect.DeepEqual(recorder.exits, []int64{1}) {
				t.Fatalf("runtime error = %v, exits = %v, want %q and exit 1", recorder.messages, recorder.exits, tt.message)
			}
			if game.inputMgr.frame.pending != nil {
				t.Fatal("failed frame retained the previous input batch")
			}
			if session.status() != status || game.inputMgr.frame.history != history || !game.inputMgr.frame.hadSession || game.inputMgr.mousePos != point {
				t.Fatal("failed sample advanced the recording or changed published input state")
			}
			if !session.beginFrame() {
				t.Fatal("failed sample left the session frame open")
			}
			session.endFrame()
			game.dispatchFrameEvents()
			currentScheduler.Update()
			if calls != 0 || engine.HasPendingCaptures() {
				t.Fatal("failed frame dispatched old input or screenshot requests")
			}
		})
	}
}

func TestPrepareInputFramePreservesInitialHeldStateAndLiveHandoff(t *testing.T) {
	_, game, mgr := setupInputSourceTest(t)
	initial := InputReplayState{
		Mouse: InputReplayMouse{X: 7, Y: 8}, Buttons: 0b111,
		KeysDown: []int64{int64(KeyA)},
	}
	replay := validRuntimeReplay(InputReplayFrame{Frame: 0, State: initial})
	replay.Initial = initial
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	claimPreparedSession(t, game)
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("initial replay frame was not prepared")
	}
	if got := game.inputMgr.frame.pending.events; len(got) != 0 {
		t.Fatalf("initial held state manufactured edges: %+v", got)
	}
	if !game.MousePressed() || !game.KeyPressed(KeyA) {
		t.Fatal("initial held state was not visible to game input queries")
	}
	game.inputMgr.swipe.Begin(mathf.Vec2{}, &SpriteImpl{name: "replay target"})
	game.dispatchFrameEvents()
	game.abortInputSession("return to live input")
	mgr.point = mathf.Vec2{X: 40, Y: -30}
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("live handoff frame was not prepared")
	}
	if got := game.inputMgr.frame.pending.events; len(got) != 0 {
		t.Fatalf("handoff manufactured release edges: %+v", got)
	}
	if game.inputMgr.mousePos != mgr.point {
		t.Fatal("handoff retained replay input state")
	}
	game.dispatchFrameEvents()
	if _, target, ok := game.inputMgr.swipe.Finish(mathf.Vec2{X: 100}); ok || target != nil {
		t.Fatal("live handoff retained a gesture from the replay clock")
	}
	mgr.point.X += 5
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("next live frame was not prepared")
	}
	if len(game.inputMgr.frame.pending.events) != 0 || game.inputMgr.mousePos != mgr.point {
		t.Fatal("live input after handoff lost its current position or created an event")
	}
}

func TestGameResetClearsPreparedInputFrame(t *testing.T) {
	resetInputSessionTest(t)
	game := setupStageLoadTestGame(t, gameTestFS{})
	game.inputMgr.frame.pending = &inputFrame{events: []event{&eventKeyDown{Key: KeyA}}}
	game.inputMgr.frame.history.Reset(true)
	game.reset()
	if game.inputMgr.frame.pending != nil {
		t.Fatal("game reset retained a pending frame")
	}
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("fresh input frame was not prepared after reset")
	}
	if got := game.inputMgr.frame.pending.events; len(got) != 0 {
		t.Fatalf("game reset leaked previous held state into new input: %+v", got)
	}
}

func TestInputFrameExpiresStationarySwipeAfterClockAdvances(t *testing.T) {
	_, game, _ := setupInputSourceTest(t)
	clockReads := 0
	game.inputMgr.swipe.InitWithClock(func() time.Time {
		clockReads++
		return time.Unix(0, int64(clock.TimeSinceLevelLoad()*float64(time.Second)))
	})
	game.inputMgr.swipe.Begin(mathf.Vec2{}, &SpriteImpl{name: "held"})
	if !game.inputMgr.prepareInputFrame(0.6) {
		t.Fatal("stationary frame was not prepared")
	}
	if len(game.inputMgr.frame.pending.events) != 0 || clockReads != 1 {
		t.Fatal("sampling manufactured an event or expired before advancing the clock")
	}
	clock.Update(0.6, 30)
	game.dispatchFrameEvents()
	game.dispatchFrameEvents()
	if clockReads != 2 {
		t.Fatalf("clock read %d times, want begin and one frame expiry", clockReads)
	}

	// An expired stationary gesture cannot revive when a logical clock rewinds.
	clock.Update(-0.5, 30)
	if _, target, ok := game.inputMgr.swipe.Finish(mathf.Vec2{X: 100}); ok || target != nil || clockReads != 2 {
		t.Fatal("stationary expiry retained its target or allowed a gesture after clock rollback")
	}
}

func TestInputFrameSwipePreservesReplayClockRollback(t *testing.T) {
	for _, test := range []struct {
		name      string
		frameTime float64
		want      bool
	}{
		{"backward frame retains gesture", 0.9, true},
		{"expired frame cannot revive gesture", 1.6, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, game, _ := setupInputSourceTest(t)
			session := &inputSession{currentTime: 1}
			game.inputMgr.swipe.InitWithClock(session.logicalClock)
			game.inputMgr.swipe.Begin(mathf.Vec2{}, nil)
			if !game.inputMgr.prepareInputFrame(0.1) {
				t.Fatal("stationary frame was not prepared")
			}
			session.currentTime = test.frameTime
			game.dispatchFrameEvents()
			session.currentTime = 1.1
			if _, _, ok := game.inputMgr.swipe.Finish(mathf.Vec2{X: 100}); ok != test.want {
				t.Fatalf("swipe after clock correction = %v, want %v", ok, test.want)
			}
		})
	}
}

func TestSwipeRoutingAllowsHandlerToBeginNextGesture(t *testing.T) {
	previous := scheduler
	scheduler = nil
	t.Cleanup(func() { scheduler = previous })
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
	previous := scheduler
	scheduler = nil
	t.Cleanup(func() { scheduler = previous })
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
					game.inputMgr.frame.pending = &inputFrame{}
					game.dispatchFrameEvents()
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
			prepared, err := prepareSprite(files, "owner", reflect.TypeOf(sprite).Elem(), gamer)
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
	currentScheduler, game, sprite := setupSpriteDragTest(t)
	other := newDragTestSprite(game, "foreground", 2)
	game.scene.add(other)
	other.graphicEffects = map[EffectKind]float64{GhostEffect: 100}
	start := mathf.NewVec2(12, 24)
	dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
	assertSpriteDragPosition(t, sprite, 10, 20)
	if game.inputMgr.drag.active {
		t.Fatal("stationary press started a drag")
	}
	dispatchSpriteDragFrame(currentScheduler, game, mathf.NewVec2(42, 64))
	assertSpriteDragPosition(t, sprite, 40, 60)
	if !game.inputMgr.drag.active {
		t.Fatal("pointer movement did not start dragging")
	}
	shapes := game.scene.getTempShapes()
	if shapes[len(shapes)-1] != sprite {
		t.Fatal("dragged sprite was not brought to the front")
	}
	end := mathf.NewVec2(52, 74)
	dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
	assertSpriteDragPosition(t, sprite, 50, 70)
	if game.inputMgr.drag.target != nil || game.inputMgr.drag.active {
		t.Fatal("release retained the drag")
	}
	dispatchSpriteDragFrame(currentScheduler, game, mathf.NewVec2(80, 90))
	assertSpriteDragPosition(t, sprite, 50, 70)
}

func TestSpriteDragClickAndSwipeRouting(t *testing.T) {
	for _, moved := range []bool{false, true} {
		name := "click"
		if moved {
			name = "drag"
		}
		t.Run(name, func(t *testing.T) {
			currentScheduler, game, sprite := setupSpriteDragTest(t)
			clicks, swipes := 0, 0
			sprite.OnClick(func() { clicks++ })
			sprite.OnSwipe__0(90, func() { swipes++ })
			game.OnClick(func() { t.Error("sprite gesture fell through to stage click") })
			game.OnSwipe__0(90, func() { t.Error("sprite drag fell through to stage swipe") })
			point := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(currentScheduler, game, point, &eventLeftButtonDown{Pos: point})
			if clicks != 0 {
				t.Fatal("draggable sprite clicked on mouse down")
			}
			if moved {
				point.X += 100
				dispatchSpriteDragFrame(currentScheduler, game, point)
			}
			dispatchSpriteDragFrame(currentScheduler, game, point, &eventLeftButtonUp{Pos: point})
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
	currentScheduler, game, sprite := setupSpriteDragTest(t)
	top := newDragTestSprite(game, "top", 2)
	game.scene.add(top)
	clicks := 0
	top.OnClick(func() { clicks++ })
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
	dispatchSpriteDragFrame(currentScheduler, game, mathf.NewVec2(30, 40))
	assertSpriteDragPosition(t, sprite, 30, 40)
	assertSpriteDragPosition(t, top, 10, 20)
	if clicks != 1 || game.inputMgr.drag.target != sprite || !game.inputMgr.drag.active {
		t.Fatal("foreground must click on mouse down without preventing a background drag")
	}
}

func TestSpriteDragCandidateDoesNotSuppressForegroundSwipe(t *testing.T) {
	for _, change := range []string{"hide", "disable", "viewport jitter"} {
		t.Run(change, func(t *testing.T) {
			currentScheduler, game, candidate := setupSpriteDragTest(t)
			clock := &inputSession{}
			game.inputSlot.session = clock
			foreground := newDragTestSprite(game, "foreground", 2)
			game.scene.add(foreground)
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
			dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
			switch change {
			case "hide":
				candidate.Hide()
			case "disable":
				candidate.SetDragMode(NotDraggable)
			}
			// This swipe spans only two viewport pixels at .02 zoom.
			clock.currentTime = .1
			end := start.Add(mathf.NewVec2(100, 0))
			dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
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
			currentScheduler, game, candidate := setupSpriteDragTest(t)
			clock := &inputSession{}
			game.inputSlot.session = clock
			foreground := newDragTestSprite(game, "foreground", 2)
			game.scene.add(foreground)
			clicks := 0
			foreground.OnClick(func() { clicks++ })
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
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
			dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
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
				currentScheduler, game, sprite := setupSpriteDragTest(t)
				clock := &inputSession{}
				game.inputSlot.session = clock
				other := newDragTestSprite(game, "other", 2)
				other.SetDragMode(Draggable)
				game.scene.add(other)
				other.SetLayerTo(Back)
				clicks := 0
				sprite.OnClick(func() { clicks++ })
				other.OnClick(func() { clicks++ })
				start := mathf.NewVec2(10, 20)
				dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
				want := start
				if active {
					want = mathf.NewVec2(30, 40)
					dispatchSpriteDragFrame(currentScheduler, game, want)
				}
				test.cancel(game, sprite)
				if game.inputMgr.drag != (spriteDragState{}) {
					t.Fatal("cancellation retained the drag")
				}
				other.SetLayerTo(Front)
				clock.currentTime = 1
				end := mathf.NewVec2(50, 60)
				dispatchSpriteDragFrame(currentScheduler, game, end)
				if game.inputMgr.drag != (spriteDragState{}) {
					t.Fatal("cancelled drag resumed or transferred")
				}
				dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
				assertSpriteDragPosition(t, sprite, want.X, want.Y)
				assertSpriteDragPosition(t, other, 10, 20)
				if clicks != 0 {
					t.Fatal("cancelled drag generated a click")
				}
				dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
				dispatchSpriteDragFrame(currentScheduler, game, mathf.NewVec2(30, 40))
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
			currentScheduler, game, sprite := setupSpriteDragTest(t)
			game.inputSlot.session = &inputSession{}
			camera := pkgengine.CameraMgr.(*dragCameraMgr)
			camera.world.Size = camera.viewport.Size.Divf(scale)
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
			for _, dx := range []float64{1, -2, 3} {
				dispatchSpriteDragFrame(currentScheduler, game, start.Add(mathf.NewVec2(dx/scale, 0)))
				if game.inputMgr.drag.active {
					t.Fatalf("%v viewport pixels started a drag", dx)
				}
				assertSpriteDragPosition(t, sprite, 10, 20)
			}
			dispatchSpriteDragFrame(currentScheduler, game, start.Add(mathf.NewVec2(3.1/scale, 0)))
			if !game.inputMgr.drag.active {
				t.Fatal("movement beyond 3 viewport pixels did not start a drag")
			}
		})
	}
}

func TestSpriteDragJitterStillClicks(t *testing.T) {
	currentScheduler, game, sprite := setupSpriteDragTest(t)
	game.inputSlot.session = &inputSession{}
	clicks := 0
	sprite.OnClick(func() { clicks++ })
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
	end := mathf.NewVec2(12, 22)
	dispatchSpriteDragFrame(currentScheduler, game, end)
	dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
	assertSpriteDragPosition(t, sprite, 10, 20)
	if clicks != 1 {
		t.Fatalf("jitter produced %d clicks, want 1", clicks)
	}
}

func TestSpriteDragLongPressUsesInputClock(t *testing.T) {
	currentScheduler, game, sprite := setupSpriteDragTest(t)
	clock := &inputSession{currentTime: 1}
	game.inputSlot.session = clock
	clicks := 0
	sprite.OnClick(func() { clicks++ })
	point := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(currentScheduler, game, point, &eventLeftButtonDown{Pos: point})
	for _, elapsed := range []float64{-0.1, 0.399, 0.399998, 0.4} {
		clock.currentTime = 1 + elapsed
		dispatchSpriteDragFrame(currentScheduler, game, point)
		if got, want := game.inputMgr.drag.active, elapsed >= 0.4; got != want {
			t.Fatalf("active after %v seconds = %v, want %v", elapsed, got, want)
		}
	}
	dispatchSpriteDragFrame(currentScheduler, game, point, &eventLeftButtonUp{Pos: point})
	if clicks != 0 {
		t.Fatal("stationary long press generated a click")
	}
}

func TestSpriteDragLongPressAfterExtendedRecording(t *testing.T) {
	for _, tc := range []struct{ fps, start int }{{30, 16174}, {60, 32656}, {30, 21675}, {60, 21663}} {
		for _, releaseAtBoundary := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dfps/start=%d/release=%v", tc.fps, tc.start, releaseAtBoundary), func(t *testing.T) {
				currentScheduler, game, sprite := setupSpriteDragTest(t)
				clock := &inputSession{}
				game.inputSlot.session = clock
				clicks := 0
				sprite.OnClick(func() { clicks++ })
				point := mathf.NewVec2(10, 20)
				boundary := tc.start + tc.fps*2/5
				// Reproduce timestamp rounding in v1 recordings.
				for i := 0; i <= boundary; i++ {
					if i > 0 {
						clock.currentTime += 1 / float64(tc.fps)
					}
					if i == tc.start {
						dispatchSpriteDragFrame(currentScheduler, game, point, &eventLeftButtonDown{Pos: point})
					} else if i > tc.start {
						var events []event
						if releaseAtBoundary && i == boundary {
							events = append(events, &eventLeftButtonUp{Pos: point})
						}
						dispatchSpriteDragFrame(currentScheduler, game, point, events...)
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
			currentScheduler, game, sprite := setupSpriteDragTest(t)
			game.inputSlot.session = &inputSession{}
			start := mathf.NewVec2(12, 24)
			dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
			if game.inputMgr.drag.target != sprite {
				t.Fatal("press did not capture the draggable sprite")
			}
			if change == "move" {
				sprite.SetXYpos(20, 30)
			} else {
				top := newDragTestSprite(game, "new top", 2)
				game.scene.add(top)
				if change == "new draggable foreground" {
					top.SetDragMode(Draggable)
				}
			}
			pkgengine.SpriteMgr.(*dragSpriteMgr).hit = func(_ pkgengine.Object, _ mathf.Vec2) bool {
				t.Error("activation repeated hit testing after the press")
				return false
			}
			dispatchSpriteDragFrame(currentScheduler, game, mathf.NewVec2(32, 44))
			if game.inputMgr.drag.target != sprite || !game.inputMgr.drag.active {
				t.Fatal("activation changed or lost the pressed target")
			}
			wantX, wantY := 30.0, 40.0
			if change == "move" {
				wantX, wantY = 40, 50
			}
			assertSpriteDragPosition(t, sprite, wantX, wantY)
			shapes := game.scene.getTempShapes()
			if shapes[len(shapes)-1] != sprite {
				t.Fatal("activated target is not at the front")
			}
		})
	}
}

func TestSpriteDragActivationAfterSpriteLeavesPressPoint(t *testing.T) {
	currentScheduler, game, sprite := setupSpriteDragTest(t)
	game.inputSlot.session = &inputSession{}
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
	sprite.SetXYpos(200, 100)
	pkgengine.SpriteMgr.(*dragSpriteMgr).hits[1] = false
	dispatchSpriteDragFrame(currentScheduler, game, mathf.NewVec2(30, 40))
	if !game.inputMgr.drag.active || game.inputMgr.drag.target != sprite {
		t.Fatal("activation lost the captured target after it moved")
	}
	assertSpriteDragPosition(t, sprite, 220, 120)
}

func TestSpriteDragActivationAfterLayerChange(t *testing.T) {
	currentScheduler, game, sprite := setupSpriteDragTest(t)
	game.inputSlot.session = &inputSession{}
	other := newDragTestSprite(game, "other", 2)
	other.SetDragMode(Draggable)
	game.scene.add(other)
	other.SetLayerTo(Back)
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
	if game.inputMgr.drag.target != sprite {
		t.Fatal("press did not pick the original foreground")
	}
	other.SetLayerTo(Front)
	dispatchSpriteDragFrame(currentScheduler, game, mathf.NewVec2(30, 40))
	if game.inputMgr.drag.target != sprite || !game.inputMgr.drag.active {
		t.Fatal("layer change transferred the captured gesture")
	}
	assertSpriteDragPosition(t, sprite, 30, 40)
	assertSpriteDragPosition(t, other, 10, 20)
}

func TestSpriteDragReleaseRepicksTarget(t *testing.T) {
	for _, kind := range []string{"draggable", "not draggable", "miss"} {
		t.Run(kind, func(t *testing.T) {
			currentScheduler, game, sprite := setupSpriteDragTest(t)
			game.inputSlot.session = &inputSession{}
			originalClicks, topClicks := 0, 0
			sprite.OnClick(func() { originalClicks++ })
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
			top := newDragTestSprite(game, "release target", 2)
			top.OnClick(func() { topClicks++ })
			game.scene.add(top)
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
			dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
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
			currentScheduler, game, sprite := setupSpriteDragTest(t)
			clock := &inputSession{}
			game.inputSlot.session = clock
			sprite.SetDragMode(NotDraggable)
			if origin == "stage" {
				sprite.Hide()
			}
			downClicks, releaseClicks := 0, 0
			sprite.OnClick(func() { downClicks++ })
			game.OnClick(func() { downClicks++ })
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
			if downClicks != 1 {
				t.Fatalf("mousedown clicks = %d, want 1", downClicks)
			}
			top := newDragTestSprite(game, "release target", 2)
			top.SetDragMode(Draggable)
			top.OnClick(func() { releaseClicks++ })
			game.scene.add(top)
			clock.currentTime = 0.1
			end := mathf.NewVec2(11, 20)
			dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
			clock.currentTime = 0.2
			dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
			if releaseClicks != 1 {
				t.Fatalf("cross-target release clicks = %d, want 1", releaseClicks)
			}
		})
	}
}

func TestSpriteDragEmptyPressNeverCapturesLaterTarget(t *testing.T) {
	for _, change := range []string{"appears", "enabled"} {
		t.Run(change, func(t *testing.T) {
			currentScheduler, game, sprite := setupSpriteDragTest(t)
			clock := &inputSession{}
			game.inputSlot.session = clock
			if change == "appears" {
				sprite.Hide()
			} else {
				sprite.SetDragMode(NotDraggable)
			}
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
			if game.inputMgr.drag.target != nil {
				t.Fatal("press captured an unavailable target")
			}
			clock.currentTime = .1
			dispatchSpriteDragFrame(currentScheduler, game, start.Add(mathf.NewVec2(4, 0)))
			sprite.Show()
			sprite.SetDragMode(Draggable)
			for i, point := range []mathf.Vec2{start, start.Add(mathf.NewVec2(4, 0)), start.Add(mathf.NewVec2(5, 0))} {
				clock.currentTime = float64(i+1) * .4
				dispatchSpriteDragFrame(currentScheduler, game, point)
				if game.inputMgr.drag.target != nil || game.inputMgr.drag.active {
					t.Fatalf("empty press captured a later target at step %d", i)
				}
			}
			assertSpriteDragPosition(t, sprite, 10, 20)
			dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonUp{Pos: start})
			clock.currentTime = 2
			dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
			dispatchSpriteDragFrame(currentScheduler, game, start.Add(mathf.NewVec2(10, 0)))
			if game.inputMgr.drag.target != sprite || !game.inputMgr.drag.active {
				t.Fatal("a fresh press could not capture the available target")
			}
			assertSpriteDragPosition(t, sprite, 20, 20)
		})
	}
}

func TestSpriteDragCameraMotionDoesNotActivate(t *testing.T) {
	currentScheduler, game, sprite := setupSpriteDragTest(t)
	game.inputSlot.session = &inputSession{}
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
	camera := pkgengine.CameraMgr.(*dragCameraMgr)
	camera.world.Position.X += 100
	// Keep the pointer stationary in the viewport.
	dispatchSpriteDragFrame(currentScheduler, game, start.Add(mathf.NewVec2(100, 0)))
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
				currentScheduler, game, sprite := setupSpriteDragTest(t)
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
					game.scene.add(newDragTestSprite(game, "foreground", 2))
				}
				var other *SpriteImpl
				if gesture == "layer change" {
					other = newDragTestSprite(game, "other", 2)
					other.SetDragMode(Draggable)
					game.scene.add(other)
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
					// Supply input at the engine sampling boundary.
					tick, err := consumeInputTickWithMouseEvents(state, edges, nil, 1/float64(fps))
					if err != nil {
						t.Fatal(err)
					}
					batch := &inputFrame{}
					game.inputMgr.frame.history.Process(coreruntime.InputFrame{
						Point: point, LeftButtonPressed: state.Buttons != 0,
						MouseEvents: tick.frame.MouseEvents,
					}, batch.eventHooks(0))
					dispatchSpriteDragFrame(currentScheduler, game, point, batch.events...)
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
				game.lifecycle.reset()
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
					currentScheduler.Update()
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
			currentScheduler, game, sprite := setupSpriteDragTest(t)
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
				currentScheduler.Update()
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
	currentScheduler, game, sprite := setupSpriteDragTest(t)
	live := setupInputSourceManager(t)
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
	dispatchSpriteDragFrame(currentScheduler, game, mathf.NewVec2(30, 40))
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
	currentScheduler.Update()
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
	currentScheduler.Update()
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
	currentScheduler, game, sprite := setupSpriteDragTest(t)
	sprite.PenDown()
	initialPenMoves := spy.moveCalls
	initialPenDowns, initialPenUps := spy.penDownCalls, spy.penUpCalls
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
	dispatchSpriteDragFrame(currentScheduler, game, mathf.NewVec2(30, 40))
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
	dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
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
	currentScheduler, game, sprite := setupSpriteDragTest(t)
	sprite.PenDown()
	initialDowns, initialUps := spy.penDownCalls, spy.penUpCalls
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
	end := mathf.NewVec2(50, 60)
	dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
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
	currentScheduler, game, clone := setupSpriteDragTest(t)
	original := newPenTestSprite()
	original.PenDown()
	clone.components.pen = original.pen().cloneFor(clone)
	clone.spriteState.cloned = true
	if !clone.pen().isPenDown || clone.pen().penObj != nil {
		t.Fatal("clone must inherit pen-down state without allocating a drawing resource")
	}
	initialDowns, initialCreates, initialMoves := spy.penDownCalls, spy.createCalls, spy.moveCalls
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
	end := mathf.NewVec2(50, 60)
	dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
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
	currentScheduler, game, sprite := setupSpriteDragTest(t)
	sprite.PenDown()
	initialDowns, initialUps := spy.penDownCalls, spy.penUpCalls
	initialMoves := spy.moveCalls
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
	end := mathf.NewVec2(50, 60)
	dispatchSpriteDragFrame(currentScheduler, game, end, &eventLeftButtonUp{Pos: end})
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
			currentScheduler, game, sprite := setupSpriteDragTest(t)
			clock := &inputSession{}
			game.inputSlot.session = clock
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
			camera := pkgengine.CameraMgr.(*dragCameraMgr)
			camera.world.Position.X += 100
			wantX := 10.0
			if moveSprite {
				wantX = 110
				sprite.SetXYpos(wantX, 20)
			}
			pkgengine.SpriteMgr.(*dragSpriteMgr).hits[1] = false
			clock.currentTime = .4
			dispatchSpriteDragFrame(currentScheduler, game, mathf.NewVec2(110, 20))
			if !game.inputMgr.drag.active || game.inputMgr.drag.target != sprite {
				t.Fatal("camera motion lost the pressed candidate")
			}
			assertSpriteDragPosition(t, sprite, wantX, 20)
		})
	}
}

func TestSpriteDragFloat32CameraNeverCapturesAfterEmptyPress(t *testing.T) {
	currentScheduler, g, sprite := setupSpriteDragTest(t)
	clock := &inputSession{}
	g.inputSlot.session = clock
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
	dispatchSpriteDragFrame(currentScheduler, g, start, &eventLeftButtonDown{Pos: start})
	mgr.hits[1] = true
	for i := 1; i <= 20; i++ {
		camera.world.Position = camera.world.Position.Add(mathf.NewVec2(.13, -.27))
		camera.world.Size = camera.world.Size.Mulf(1.001)
		clock.currentTime = float64(i) / 10
		dispatchSpriteDragFrame(currentScheduler, g, pointAtView(stationary))
		if g.inputMgr.drag.target != nil || g.inputMgr.drag.active {
			t.Fatalf("float32 camera captured a late sprite at frame %d", i)
		}
	}
	dispatchSpriteDragFrame(currentScheduler, g, pointAtView(stationary.Add(mathf.NewVec2(4, 0))))
	if g.inputMgr.drag.target != nil || g.inputMgr.drag.active {
		t.Fatal("later pointer movement captured a sprite after an empty press")
	}
	assertSpriteDragPosition(t, sprite, 10, 20)
}

// Replay exercises sampling and dispatch, not the platform event queue.
func TestSpriteDragSamplingAndDispatch(t *testing.T) {
	for _, gesture := range []string{"first down", "same tick click", "held", "activate"} {
		t.Run(gesture, func(t *testing.T) {
			resetInputSessionTest(t)
			currentScheduler, game, sprite := setupSpriteDragTest(t)
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
				currentScheduler.Update()
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

func TestSpriteDragActiveSamplingAndDispatch(t *testing.T) {
	for _, release := range []bool{false, true} {
		for _, change := range []string{"pan", "zoom", "viewport", "missing sample", "missing dispatch"} {
			t.Run(fmt.Sprintf("%s/release=%v", change, release), func(t *testing.T) {
				resetInputSessionTest(t)
				currentScheduler, game, sprite := setupSpriteDragTest(t)
				setupInputSourceManager(t)
				frames := []InputReplayFrame{
					{Frame: 0, Time: 0, State: InputReplayState{Mouse: InputReplayMouse{X: 12, Y: 18}, Buttons: 1}, MouseEvents: []InputReplayMouseEvent{{Button: 1, Pressed: true}}},
					{Frame: 1, Time: .1, State: InputReplayState{Mouse: InputReplayMouse{X: 32, Y: 38}, Buttons: 1}},
					{Frame: 2, Time: .2, State: InputReplayState{Mouse: InputReplayMouse{X: 32, Y: 38}, Buttons: 1}},
				}
				if release {
					frames[2].State.Buttons = 0
					frames[2].MouseEvents = []InputReplayMouseEvent{{Button: 1}}
				}
				if _, err := PrepareInputReplay(validRuntimeReplay(frames...)); err != nil {
					t.Fatal(err)
				}
				session := claimPreparedSession(t, game)
				t.Cleanup(func() { game.abortInputSession("test finished") })
				sprite.OnClick(func() { t.Error("drag generated a click") })
				camera := pkgengine.CameraMgr.(*dragCameraMgr)
				for i := range frames {
					if i == 2 && change == "missing sample" {
						camera.world.Size.X = 0
					}
					if !game.inputMgr.prepareInputFrame(.01) {
						t.Fatal("missing input frame")
					}
					if i == 2 {
						if !game.inputMgr.drag.active {
							t.Fatal("drag must already be active before the camera changes")
						}
						assertSpriteDragPosition(t, sprite, 30, 40)
						// Keep the pointer at viewport (272, 142).
						switch change {
						case "pan":
							camera.world.Position = camera.world.Position.Add(mathf.NewVec2(100, 50))
						case "zoom":
							camera.world.Size = camera.world.Size.Divf(2)
						case "viewport":
							camera.viewport.Size = camera.viewport.Size.Mulf(2)
						case "missing sample":
							camera.world.Size.X = 480
						case "missing dispatch":
							camera.world.Size.X = 0
						}
					}
					game.dispatchFrameEvents()
					currentScheduler.Update()
					session.endFrame()
				}
				// Preserve the world-space grab offset (-2, 2).
				switch change {
				case "pan":
					assertSpriteDragPosition(t, sprite, 130, -10)
				case "zoom", "viewport":
					assertSpriteDragPosition(t, sprite, -106, 111)
				default:
					assertSpriteDragPosition(t, sprite, 30, 40)
				}
				if game.inputMgr.drag.active != !release {
					t.Fatalf("active = %v after release=%v", game.inputMgr.drag.active, release)
				}
			})
		}
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
	// Bypass public cancellation to exercise the stale-target guard.
	for _, invalid := range []string{"disabled", "hidden", "destroyed"} {
		t.Run(invalid, func(t *testing.T) {
			currentScheduler, game, sprite := setupSpriteDragTest(t)
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(currentScheduler, game, start, &eventLeftButtonDown{Pos: start})
			switch invalid {
			case "disabled":
				sprite.spriteState.isDraggable = false
			case "hidden":
				sprite.spriteState.isVisible = false
			case "destroyed":
				sprite.markDestroyed()
			}
			dispatchSpriteDragFrame(currentScheduler, game, mathf.NewVec2(30, 40))
			if game.inputMgr.drag != (spriteDragState{}) {
				t.Fatal("invalid target retained drag state")
			}
			assertSpriteDragPosition(t, sprite, 10, 20)
		})
	}
}

func TestReplaySwipeExpiryAcrossFrameBoundaries(t *testing.T) {
	press := InputReplayMouseEvent{Button: 1, Pressed: true}
	release := InputReplayMouseEvent{Button: 1}
	for _, test := range []struct {
		name       string
		frames     []InputReplayFrame
		wantSwipes []int
		wantClicks int
	}{
		{
			name: "release at timeout remains valid",
			frames: []InputReplayFrame{
				{Time: 0, State: InputReplayState{Buttons: 1}, MouseEvents: []InputReplayMouseEvent{press}},
				{Time: 0.5, State: InputReplayState{Mouse: InputReplayMouse{X: 100}}, MouseEvents: []InputReplayMouseEvent{release}},
			},
			wantSwipes: []int{0, 1}, wantClicks: 1,
		},
		{
			name: "stationary frame past timeout expires gesture",
			frames: []InputReplayFrame{
				{Time: 0, State: InputReplayState{Buttons: 1}, MouseEvents: []InputReplayMouseEvent{press}},
				{Time: 0.500001, State: InputReplayState{Buttons: 1}},
				{Time: 0.6, State: InputReplayState{Mouse: InputReplayMouse{X: 100}}, MouseEvents: []InputReplayMouseEvent{release}},
			},
			wantSwipes: []int{0, 0, 0}, wantClicks: 1,
		},
		{
			name: "expired gesture permits a new press in the same frame",
			frames: []InputReplayFrame{
				{Time: 0, State: InputReplayState{Buttons: 1}, MouseEvents: []InputReplayMouseEvent{press}},
				{Time: 0.6, State: InputReplayState{Mouse: InputReplayMouse{X: 100}, Buttons: 1}, MouseEvents: []InputReplayMouseEvent{release, press}},
				{Time: 0.7, State: InputReplayState{Mouse: InputReplayMouse{X: 200}}, MouseEvents: []InputReplayMouseEvent{release}},
			},
			wantSwipes: []int{0, 0, 1}, wantClicks: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			currentScheduler, game, _ := setupInputSourceTest(t)
			for i := range test.frames {
				test.frames[i].Frame = int64(i)
			}
			if _, err := PrepareInputReplay(validRuntimeReplay(test.frames...)); err != nil {
				t.Fatal(err)
			}
			session := claimPreparedSession(t, game)
			clicks, swipes := 0, 0
			game.OnClick(func() { clicks++ })
			game.OnSwipe__0(90, func() { swipes++ })
			for i, want := range test.wantSwipes {
				// Deliberately differ from replay times: expiry follows the resolved clock.
				if !game.inputMgr.prepareInputFrame(0.01) {
					t.Fatalf("replay frame %d was not prepared", i)
				}
				game.dispatchFrameEvents()
				currentScheduler.Update()
				game.dispatchFrameEvents()
				currentScheduler.Update()
				session.endFrame()
				if swipes != want {
					t.Fatalf("frame %d swipe callbacks = %d, want %d", i, swipes, want)
				}
			}
			if clicks != test.wantClicks {
				t.Fatalf("click callbacks = %d, want %d", clicks, test.wantClicks)
			}
			if completed, err := session.completeReplayFrame(nil); err != nil || !completed {
				t.Fatalf("complete replay = %v, %v", completed, err)
			}
			before := session.status()
			if game.inputMgr.prepareInputFrame(1) {
				t.Fatal("completed replay prepared an extra frame")
			}
			game.dispatchFrameEvents()
			currentScheduler.Update()
			if session.status() != before || swipes != test.wantSwipes[len(test.wantSwipes)-1] || clicks != test.wantClicks {
				t.Fatal("completed replay advanced or repeated input callbacks")
			}
		})
	}
}

func TestInputReplayRuntimeUsesIndependentInputTicks(t *testing.T) {
	resetInputSessionTest(t)
	clock.SetFixedDeltaTime(1.0 / 60.0)

	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	if status := GetInputSessionStatus(); status.Mode != InputSessionModeRecording || status.Phase != InputSessionPhasePrepared {
		t.Fatalf("prepared recording status = %+v", status)
	}
	game := &Game{}
	claimPreparedSession(t, game)

	initial := InputReplayState{
		Mouse:    InputReplayMouse{X: 10, Y: 20},
		KeysDown: []int64{int64(KeyA)},
	}
	tick0 := InputReplayState{
		Mouse:    InputReplayMouse{X: 10, Y: 20},
		Buttons:  1,
		KeysDown: []int64{int64(KeyA), int64(KeyB)},
	}
	resolved0, err := consumeInputTickWithMouseEvents(
		tick0,
		[]InputReplayMouseEvent{{Button: 1, Pressed: true}},
		[]InputReplayKeyEvent{{Key: int64(KeyB), Pressed: true}},
		123,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved0.firstTick || resolved0.frame.Frame != 0 || resolved0.frame.Time != 0 {
		t.Fatalf("first recording tick = %+v", resolved0)
	}

	tick1 := InputReplayState{Mouse: InputReplayMouse{X: 30, Y: 40}}
	if _, err := consumeInputTickWithMouseEvents(tick1, []InputReplayMouseEvent{{Button: 1, Pressed: false}}, []InputReplayKeyEvent{
		{Key: int64(KeyA), Pressed: false},
		{Key: int64(KeyB), Pressed: false},
	}, 0.25); err != nil {
		t.Fatal(err)
	}
	replay, err := FinishInputRecording()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replay.Initial, initial) {
		t.Fatalf("recording initial = %+v, want %+v", replay.Initial, initial)
	}
	if len(replay.Frames) != 2 || replay.Frames[0].Frame != 0 || replay.Frames[1].Frame != 1 {
		t.Fatalf("recorded frames = %+v", replay.Frames)
	}
	if replay.FixedTimestep != 1.0/30.0 {
		t.Fatalf("fixed timestep = %v, want %v", replay.FixedTimestep, 1.0/30.0)
	}
	if status := GetInputSessionStatus(); !status.Completed || status.Phase != InputSessionPhaseCompleted {
		t.Fatalf("completed recording status = %+v", status)
	}

	encoded, err := FinishInputRecordingJSON()
	if err != nil {
		t.Fatal(err)
	}
	wantEncoded, err := EncodeInputReplay(replay)
	if err != nil {
		t.Fatal(err)
	}
	if encoded != wantEncoded {
		t.Fatal("cached recording JSON differs from replay encoding")
	}
	decoded, err := DecodeInputReplay(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, replay) {
		t.Fatalf("decoded replay = %+v, want %+v", decoded, replay)
	}
	for range 2 {
		replay.Initial.KeysDown[0] = 999
		replay.Frames[0].State.KeysDown[0] = 999
		replay.Frames[0].MouseEvents[0].Button = 3
		replay.Frames[0].KeyEvents[0].Key = 999
		replay.Frames[0].Time = 999
		replay, err = FinishInputRecording()
		if err != nil || !reflect.DeepEqual(replay, decoded) {
			t.Fatalf("recording result aliases its cache: %+v, %v", replay, err)
		}
		if got, err := FinishInputRecordingJSON(); err != nil || got != encoded {
			t.Fatalf("mutating recording result changed cached JSON: %s, %v", got, err)
		}
	}
}

func TestInputReplaySessionUsesDeterministicRandomByDefault(t *testing.T) {
	resetInputSessionTest(t)
	ResetRandomSeed()

	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	recorded := []float64{Rand__0(1, 100), Rand__1(0, 1), Rand__0(1, 100)}
	replay, err := FinishInputRecording()
	if err != nil {
		t.Fatal(err)
	}
	game.abortInputSession("recording game ended")
	game.lifecycle.reset()

	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	claimPreparedSession(t, game)
	replayed := []float64{Rand__0(1, 100), Rand__1(0, 1), Rand__0(1, 100)}
	if !reflect.DeepEqual(replayed, recorded) {
		t.Fatalf("replay random = %v, want recording random %v", replayed, recorded)
	}
}

func TestRecordedInputEdgesReplayThroughFrameDispatch(t *testing.T) {
	currentScheduler, game, _ := setupInputSourceTest(t)
	if _, err := PrepareInputRecording(10); err != nil {
		t.Fatal(err)
	}
	claimPreparedSession(t, game)
	point := mathf.Vec2{X: 7, Y: 8}
	samples := []struct {
		state InputReplayState
		mouse []InputReplayMouseEvent
		keys  []InputReplayKeyEvent
		want  []event
	}{
		{
			mouse: []InputReplayMouseEvent{{Button: 1, Pressed: true}, {Button: 1, Pressed: false}},
			keys:  []InputReplayKeyEvent{{Key: int64(KeyB), Pressed: true}, {Key: int64(KeyB), Pressed: false}},
			want:  []event{&eventLeftButtonDown{Pos: point}, &eventLeftButtonUp{Pos: point}, &eventKeyDown{Key: KeyB}},
		},
		{
			state: InputReplayState{Buttons: 1, KeysDown: []int64{int64(KeyA)}},
			mouse: []InputReplayMouseEvent{{Button: 1, Pressed: true}},
			keys:  []InputReplayKeyEvent{{Key: int64(KeyA), Pressed: true}},
			want:  []event{&eventLeftButtonDown{Pos: point}, &eventKeyDown{Key: KeyA}},
		},
		{
			mouse: []InputReplayMouseEvent{{Button: 1, Pressed: false}},
			keys:  []InputReplayKeyEvent{{Key: int64(KeyA), Pressed: false}},
			want:  []event{&eventLeftButtonUp{Pos: point}},
		},
	}
	// Reuse the engine's edge buffers across ticks. Stored recording frames
	// must keep their own snapshots when the next sample overwrites them.
	var mouseBuffer []engine.MouseEvent
	var keyBuffer []engine.KeyEvent
	for _, sample := range samples {
		sample.state.Mouse = InputReplayMouse{X: point.X, Y: point.Y}
		mouseBuffer = append(mouseBuffer[:0], sample.mouse...)
		keyBuffer = append(keyBuffer[:0], sample.keys...)
		if _, err := consumeInputTickWithMouseEvents(sample.state, mouseBuffer, keyBuffer, 0.1); err != nil {
			t.Fatal(err)
		}
	}
	clear(mouseBuffer)
	clear(keyBuffer)
	replay, err := FinishInputRecording()
	if err != nil {
		t.Fatal(err)
	}
	if len(replay.Frames) != len(samples) {
		t.Fatalf("recorded %d frames, want %d", len(replay.Frames), len(samples))
	}
	for i, sample := range samples {
		if !reflect.DeepEqual(replay.Frames[i].MouseEvents, sample.mouse) || !reflect.DeepEqual(replay.Frames[i].KeyEvents, sample.keys) {
			t.Fatalf("recorded frame %d did not preserve its mouse/key edges", i)
		}
	}
	game.abortInputSession("recording game ended")
	game.lifecycle.reset()
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	session := claimPreparedSession(t, game)
	clicks, swipes := 0, 0
	var keys []Key
	game.OnClick(func() { clicks++ })
	game.OnSwipe__0(90, func() { swipes++ })
	game.OnAnyKey(func(key Key) { keys = append(keys, key) })
	for i, sample := range samples {
		if !game.inputMgr.prepareInputFrame(0.1) {
			t.Fatalf("replay frame %d was not prepared", i)
		}
		if got := game.inputMgr.frame.pending.events; !reflect.DeepEqual(got, sample.want) {
			t.Fatalf("frame %d events = %+v, want %+v", i, got, sample.want)
		}
		if got := &game.inputMgr.frame; !reflect.DeepEqual(got.mouseEvents, sample.mouse) || !reflect.DeepEqual(got.keyEvents, sample.keys) {
			t.Fatalf("replay frame %d did not preserve its mouse/key edges", i)
		}
		// Dispatch owns its normalized events; reusable edge buffers must not
		// alias those events or the caller's recording.
		clear(game.inputMgr.frame.mouseEvents)
		clear(game.inputMgr.frame.keyEvents)
		if game.MousePressed() != (sample.state.Buttons != 0) || game.KeyPressed(KeyA) != (i == 1) || game.KeyPressed(KeyB) {
			t.Fatalf("frame %d held state does not match recorded input", i)
		}
		clock.Update(0.1, 10)
		game.dispatchFrameEvents()
		currentScheduler.Update()
		game.dispatchFrameEvents()
		currentScheduler.Update()
		session.endFrame()
		if !reflect.DeepEqual(replay.Frames[i].MouseEvents, sample.mouse) || !reflect.DeepEqual(replay.Frames[i].KeyEvents, sample.keys) {
			t.Fatalf("replay frame %d mutated the caller's recording", i)
		}
	}
	if swipes != 0 {
		t.Fatal("same-tick press/release or stationary hold produced a swipe")
	}
	if clicks != 2 || !reflect.DeepEqual(keys, []Key{KeyB, KeyA}) {
		t.Fatalf("replayed handlers = %d clicks, keys %v, want 2 clicks and [B A] once", clicks, keys)
	}
	if status := session.status(); status.Phase != InputSessionPhaseFinishing || status.Completed || !status.Exhausted ||
		!status.HasCurrentTick || status.CurrentTick != int64(len(samples)-1) {
		t.Fatalf("final replay tick status = %+v", status)
	}
}

func TestPrearmedInputRecordingDerivesFreshConsumerInitial(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	live := InputReplayState{
		Mouse:    InputReplayMouse{X: -240, Y: 180},
		KeysDown: []int64{int64(KeyB)},
	}
	mouseEvents := []InputReplayMouseEvent{
		{Button: 1, Pressed: true},
		{Button: 1, Pressed: false},
	}
	keyEvents := []InputReplayKeyEvent{{Key: int64(KeyB), Pressed: true}}
	tick, err := consumeInputTickWithMouseEvents(live, mouseEvents, keyEvents, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tick.frame.MouseEvents, mouseEvents) || !reflect.DeepEqual(tick.frame.KeyEvents, keyEvents) {
		t.Fatalf("tick zero edges = (%+v, %+v), want (%+v, %+v)", tick.frame.MouseEvents, tick.frame.KeyEvents, mouseEvents, keyEvents)
	}
	replay, err := FinishInputRecording()
	if err != nil {
		t.Fatal(err)
	}
	wantInitial := InputReplayState{Mouse: live.Mouse, KeysDown: []int64{}}
	if !reflect.DeepEqual(replay.Initial, wantInitial) {
		t.Fatalf("pre-armed recording initial = %+v, want event-start state %+v", replay.Initial, wantInitial)
	}
	if _, err := EncodeInputReplay(replay); err != nil {
		t.Fatalf("pre-armed recording is not self-consistent: %v", err)
	}
}

func TestRecordingTimestepLifecycle(t *testing.T) {
	for _, tt := range []struct {
		name             string
		fps              float64
		previousTimestep float64
	}{
		{name: "default", fps: clock.DefaultFPS},
		{name: "custom", fps: 24, previousTimestep: 1.0 / 60},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetInputSessionTest(t)
			clock.SetFixedDeltaTime(tt.previousTimestep)
			previousFixed := tt.previousTimestep > 0
			if _, err := PrepareInputRecording(tt.fps); err != nil {
				t.Fatal(err)
			}
			if got, fixed := clock.FixedDeltaTime(); fixed != previousFixed || got != tt.previousTimestep {
				t.Fatalf("prepared session fixed timestep = (%v, %v), want (%v, %v)", got, fixed, tt.previousTimestep, previousFixed)
			}
			game := &Game{}
			claimPreparedSession(t, game)
			want := 1 / tt.fps
			if got, ok := clock.FixedDeltaTime(); !ok || got != want {
				t.Fatalf("recording fixed timestep = (%v, %v), want (%v, true)", got, ok, want)
			}
			replay, err := FinishInputRecording()
			if err != nil {
				t.Fatal(err)
			}
			if replay.FixedTimestep != want {
				t.Fatalf("recorded fixed timestep = %v, want %v", replay.FixedTimestep, want)
			}
			if got, ok := clock.FixedDeltaTime(); !ok || got != want {
				t.Fatalf("completed session fixed timestep = (%v, %v), want (%v, true) until Game ends", got, ok, want)
			}
			game.abortInputSession("game ended")
			if got, fixed := clock.FixedDeltaTime(); fixed != previousFixed || got != tt.previousTimestep {
				t.Fatalf("restored fixed timestep = (%v, %v), want (%v, %v)", got, fixed, tt.previousTimestep, previousFixed)
			}
		})
	}
}

func TestPrepareInputRecordingRejectsInvalidValues(t *testing.T) {
	resetInputSessionTest(t)
	for _, fps := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := PrepareInputRecording(fps); err == nil {
			t.Fatalf("PrepareInputRecording(%v) succeeded", fps)
		}
	}
	if _, err := PrepareInputRecording(30, InputSessionOptions{CaptureKey: KeyAny}); err == nil {
		t.Fatal("PrepareInputRecording accepted a non-concrete capture key")
	}
	if _, err := PrepareInputRecording(30, InputSessionOptions{}, InputSessionOptions{}); err == nil {
		t.Fatal("PrepareInputRecording accepted multiple options values")
	}
}

func TestInputSessionPreparationCancelOnlyAffectsItsDescriptor(t *testing.T) {
	resetInputSessionTest(t)
	first, err := PrepareInputRecording(30)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Cancel() {
		t.Fatal("first preparation was not cancelled")
	}
	if first.Cancel() {
		t.Fatal("preparation cancellation was not idempotent")
	}

	second, err := PrepareInputReplay(validRuntimeReplay())
	if err != nil {
		t.Fatal(err)
	}
	if first.Cancel() {
		t.Fatal("stale preparation cancelled a later descriptor")
	}
	if status := GetInputSessionStatus(); status.Mode != InputSessionModeReplaying || status.Phase != InputSessionPhasePrepared {
		t.Fatalf("later preparation status = %+v", status)
	}
	if !second.Cancel() {
		t.Fatal("second preparation was not cancelled")
	}
}

func TestReplayOverridesAndRestoresTimestep(t *testing.T) {
	for _, tt := range []struct {
		name     string
		timestep float64
	}{
		{name: "fixed", timestep: 1.0 / 30},
		{name: "variable", timestep: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetInputSessionTest(t)
			clock.SetFixedDeltaTime(1.0 / 60)
			replay := validRuntimeReplay()
			replay.FixedTimestep = tt.timestep
			if _, err := PrepareInputReplay(replay); err != nil {
				t.Fatal(err)
			}
			game := &Game{}
			claimPreparedSession(t, game)
			wantFixed := tt.timestep > 0
			if got, fixed := clock.FixedDeltaTime(); fixed != wantFixed || got != tt.timestep {
				t.Fatalf("replay fixed timestep = (%v, %v), want (%v, %v)", got, fixed, tt.timestep, wantFixed)
			}
			if status := GetInputSessionStatus(); status.Mode != InputSessionModeReplaying || status.Phase != InputSessionPhaseRunning {
				t.Fatalf("replay session status = %+v", status)
			}
			game.abortInputSession("game ended")
			if got, fixed := clock.FixedDeltaTime(); !fixed || got != 1.0/60 {
				t.Fatalf("restored fixed timestep = (%v, %v), want (%v, true)", got, fixed, 1.0/60.0)
			}
		})
	}
}

func TestPreparedInputSessionIsConsumedOncePerGameLifecycle(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	first := claimPreparedSession(t, game)
	if err := game.attachPreparedInputSession(); !errors.Is(err, ErrInputSessionActive) {
		t.Fatalf("second attachment error = %v, want %v", err, ErrInputSessionActive)
	}
	if _, err := consumeInputTick(InputReplayState{}, nil, 1.0/30); err != nil {
		t.Fatal(err)
	}
	game.abortInputSession("lifecycle ended")
	game.lifecycle.reset()
	if err := game.attachPreparedInputSession(); err != nil {
		t.Fatal(err)
	}
	if game.currentInputSession() != nil {
		t.Fatal("one-shot session descriptor was consumed twice")
	}
	if status := GetInputSessionStatus(); status.Mode != InputSessionModeIdle {
		t.Fatalf("ordinary lifecycle retained terminal input status: %+v", status)
	}
	game.abortInputSession("ordinary lifecycle ended")
	game.lifecycle.reset()

	if _, err := PrepareInputReplay(validRuntimeReplay()); err != nil {
		t.Fatal(err)
	}
	second := claimPreparedSession(t, game)
	if second == first {
		t.Fatal("new Game lifecycle reused the previous input session")
	}
	if status := second.status(); status.Phase != InputSessionPhaseRunning || status.NextFrame != 0 || status.HasCurrentTick {
		t.Fatalf("new session inherited previous input state: %+v", status)
	}
}

func TestGameInputSessionCannotChangeModeDuringLifecycle(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	if _, err := PrepareInputReplay(validRuntimeReplay()); !errors.Is(err, ErrInputSessionActive) {
		t.Fatalf("PrepareInputReplay while recording error = %v", err)
	}
	if _, err := FinishInputRecording(); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareInputReplay(validRuntimeReplay()); !errors.Is(err, ErrInputSessionActive) {
		t.Fatalf("PrepareInputReplay after recording completion error = %v", err)
	}
}

func TestInputSessionCannotAttachAfterGameStarted(t *testing.T) {
	resetInputSessionTest(t)
	preparation, err := PrepareInputRecording(30)
	if err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	game.lifecycle.loaded.Store(true)
	engine.SetGame(game)
	if err := game.attachPreparedInputSession(); err == nil {
		t.Fatal("input session attached after Game started")
	}
	if status := GetInputSessionStatus(); status.Mode != InputSessionModeRecording || status.Phase != InputSessionPhasePrepared {
		t.Fatalf("rejected descriptor changed unexpectedly: %+v", status)
	}
	if !preparation.Cancel() {
		t.Fatal("rejected preparation was not cancellable")
	}
}

func TestOrdinaryGameClaimsInputLifecycleBeforeBootstrap(t *testing.T) {
	resetInputSessionTest(t)
	game := &Game{}
	engine.SetGame(game)
	if err := game.attachPreparedInputSession(); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareInputRecording(30); !errors.Is(err, ErrInputSessionActive) {
		t.Fatalf("PrepareInputRecording after lifecycle claim error = %v", err)
	}
}

func TestGameAbortInvalidatesRecordingAndRestoresEnvironment(t *testing.T) {
	resetInputSessionTest(t)
	clock.SetFixedDeltaTime(1.0 / 60)
	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	session := claimPreparedSession(t, game)
	if _, err := consumeInputTick(InputReplayState{}, nil, 1.0/30); err != nil {
		t.Fatal(err)
	}
	game.abortInputSession("game reset")
	if game.currentInputSession() != nil {
		t.Fatal("aborted session remained attached to Game")
	}
	if _, err := session.consumeSampledInputTick(1.0/30, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
		t.Fatal("aborted session sampled live input")
		return InputReplayState{}, nil, nil
	}); err == nil {
		t.Fatal("aborted session accepted another tick")
	}
	if status := session.status(); status.Phase != InputSessionPhaseAborted || status.Completed || status.Error != "game reset" {
		t.Fatalf("aborted session status = %+v", status)
	}
	if status := GetInputSessionStatus(); status.Phase != InputSessionPhaseAborted || status.Error != "game reset" || status.NextFrame != 1 || status.FrameCount != 1 {
		t.Fatalf("public aborted session status = %+v", status)
	}
	wantStatus := GetInputSessionStatus()
	game.abortInputSession("second reset")
	if got := session.close("second reset"); got != wantStatus {
		t.Fatalf("repeated close changed session status: %+v", got)
	}
	if got := GetInputSessionStatus(); got != wantStatus {
		t.Fatalf("repeated abort changed public status: %+v", got)
	}
	if _, err := session.finishRecordingResult(nil); err == nil {
		t.Fatal("aborted recording produced a commit result")
	}
	if got, ok := clock.FixedDeltaTime(); !ok || got != 1.0/60 {
		t.Fatalf("aborted session restored fixed timestep = (%v, %v)", got, ok)
	}
}

func TestGameDestroyAbortsInputSession(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputReplay(validRuntimeReplay()); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	session := claimPreparedSession(t, game)
	game.lifecycle.loaded.Store(true)
	game.OnEngineDestroy()
	if status := session.status(); status.Phase != InputSessionPhaseAborted {
		t.Fatalf("destroyed Game session status = %+v", status)
	}
	if game.currentInputSession() != nil {
		t.Fatal("destroyed Game retained input session")
	}
	if game.lifecycle.loaded.Load() {
		t.Fatal("destroyed Game remained running")
	}
	preparation, err := PrepareInputRecording(30)
	if err != nil {
		t.Fatalf("next Game preparation after destroy: %v", err)
	}
	preparation.Cancel()
}

func TestFinishRecordingRequiresAClosedEngineFrame(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	session := claimPreparedSession(t, game)
	if !session.beginFrame() {
		t.Fatal("recording frame did not open")
	}
	if _, err := FinishInputRecording(); err == nil {
		t.Fatal("recording finished inside an open engine frame")
	}
	if status := session.status(); status.Phase != InputSessionPhaseRunning || status.Completed {
		t.Fatalf("failed finish changed session status = %+v", status)
	}
	session.endFrame()
	phaseDuringFreeze := InputSessionPhase("")
	if _, err := session.finishRecordingResult(func() {
		phaseDuringFreeze = session.status().Phase
	}); err != nil {
		t.Fatal(err)
	}
	if phaseDuringFreeze != InputSessionPhaseFinishing {
		t.Fatalf("phase during freeze = %q, want %q", phaseDuringFreeze, InputSessionPhaseFinishing)
	}
	if status := session.status(); status.Phase != InputSessionPhaseCompleted || !status.Completed {
		t.Fatalf("finished recording status = %+v", status)
	}
}

func TestReplayCompletesOnlyAtFrameEnd(t *testing.T) {
	for _, tt := range []struct {
		name      string
		replay    InputReplay
		wantFrame int64
	}{
		{name: "empty", replay: validRuntimeReplay(), wantFrame: -1},
		{
			name:      "single-frame",
			replay:    validRuntimeReplay(InputReplayFrame{Frame: 0, State: InputReplayState{Mouse: InputReplayMouse{X: 2}}}),
			wantFrame: 0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetInputSessionTest(t)
			if _, err := PrepareInputReplay(tt.replay); err != nil {
				t.Fatal(err)
			}
			session := claimPreparedSession(t, &Game{})
			if status := session.status(); status.Completed || status.Exhausted {
				t.Fatalf("replay completed before tick zero: %+v", status)
			}
			first, err := consumeInputTick(InputReplayState{Mouse: InputReplayMouse{X: 99}}, nil, 1)
			if err != nil {
				t.Fatal(err)
			}
			if !first.firstTick || first.frame.Frame != tt.wantFrame {
				t.Fatalf("first replay tick = %+v, want frame %d", first, tt.wantFrame)
			}
			if status := session.status(); status.Phase != InputSessionPhaseFinishing || status.Completed || !status.Exhausted ||
				!status.HasCurrentTick || status.CurrentTick != 0 {
				t.Fatalf("status after final input tick = %+v", status)
			}
			completed, err := session.completeReplayFrame(nil)
			if err != nil {
				t.Fatal(err)
			}
			if !completed {
				t.Fatal("final frame did not complete replay")
			}
			if status := session.status(); status.Phase != InputSessionPhaseCompleted || !status.Completed {
				t.Fatalf("status after frame end = %+v", status)
			}
			completed, err = session.completeReplayFrame(nil)
			if err != nil {
				t.Fatal(err)
			}
			if completed {
				t.Fatal("replay completion was not idempotent")
			}
		})
	}
}

func TestReplayLogicalClockUsesSessionTime(t *testing.T) {
	resetInputSessionTest(t)
	replay := validRuntimeReplay(
		InputReplayFrame{Frame: 0, State: InputReplayState{}},
		InputReplayFrame{Frame: 1, Time: 0.25, State: InputReplayState{}},
	)
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	if _, err := consumeInputTick(InputReplayState{}, nil, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := consumeInputTick(InputReplayState{}, nil, 1); err != nil {
		t.Fatal(err)
	}
	if got := game.inputClock(); !got.Equal(time.Unix(0, int64(0.25*float64(time.Second)))) {
		t.Fatalf("replay clock = %v", got)
	}
}

func TestFinishRecordingWaitsForCurrentInputOperation(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	session := claimPreparedSession(t, game)
	sampling := make(chan struct{})
	release := make(chan struct{})
	tickDone := make(chan error, 1)
	go func() {
		_, err := session.consumeSampledInputTick(1.0/30, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
			close(sampling)
			<-release
			return InputReplayState{}, nil, nil
		})
		tickDone <- err
	}()
	<-sampling

	finishDone := make(chan error, 1)
	go func() {
		result, err := session.finishRecordingResult(nil)
		if err == nil && len(result.replay.Frames) != 1 {
			err = errors.New("recording finished without the current tick")
		}
		finishDone <- err
	}()
	select {
	case err := <-finishDone:
		t.Fatalf("FinishInputRecording returned inside an input operation: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-tickDone; err != nil {
		t.Fatal(err)
	}
	if err := <-finishDone; err != nil {
		t.Fatal(err)
	}
}

func TestCaptureInputSessionFixtureClearsGameAndPendingCaptures(t *testing.T) {
	setupCaptureInputSessionTest(t)

	t.Run("recording", func(t *testing.T) {
		setupCaptureInputSessionTest(t)
		if _, err := PrepareInputRecording(30); err != nil {
			t.Fatal(err)
		}
		claimPreparedSession(t, &Game{})
		capture("pending-cleanup", nil)
		if !engine.HasPendingCaptures() {
			t.Fatal("recording fixture did not queue the capture")
		}
	})

	if game := engine.GetGame(); game != nil {
		t.Errorf("Game after fixture cleanup = %T, want nil", game)
	}
	if status := GetInputSessionStatus(); status.Mode != InputSessionModeIdle {
		t.Errorf("input session after fixture cleanup = %+v, want idle", status)
	}
	if _, fixed := clock.FixedDeltaTime(); fixed {
		t.Error("fixture cleanup left the fixed timestep enabled")
	}
	if engine.HasPendingCaptures() {
		t.Error("fixture cleanup left pending captures")
	}
}

func TestCaptureUsesCurrentInputReplayTick(t *testing.T) {
	setupCaptureInputSessionTest(t)

	var got []engine.CaptureRequest
	engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
		got = append(got, req)
		return nil
	})

	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	capture("before-tick", nil)
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("captures before tick zero = %d, want 1", len(got))
	}
	if got[0].InputTick != nil {
		t.Fatalf("capture before tick zero has input tick %d", *got[0].InputTick)
	}

	resolved, err := consumeInputTick(InputReplayState{}, nil, 1.0/30.0)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.frame.Frame != 0 {
		t.Fatalf("resolved input tick = %d, want 0", resolved.frame.Frame)
	}
	capture("tick-zero", nil)
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("captures after tick zero = %d, want 2", len(got))
	}
	if got[1].InputTick == nil || *got[1].InputTick != 0 {
		t.Fatalf("capture input tick = %v, want 0", got[1].InputTick)
	}

	savedTick := captureInputTick()
	if savedTick == nil || *savedTick != 0 {
		t.Fatalf("current capture tick = %v, want 0", savedTick)
	}
	capture("after-body", func() error {
		_, err := consumeInputTick(InputReplayState{}, nil, 1.0/30.0)
		return err
	})
	if _, err := consumeInputTick(InputReplayState{}, nil, 1.0/30.0); err != nil {
		t.Fatal(err)
	}
	if *savedTick != 0 {
		t.Fatalf("saved capture tick changed to %d after input advanced", *savedTick)
	}
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[2].Name != "after-body" || got[2].InputTick == nil || *got[2].InputTick != 1 {
		t.Fatalf("captures after body = %+v, want after-body at tick 1 before the next input tick", got)
	}
}

func TestCaptureUsesSyntheticTickZeroForEmptyReplay(t *testing.T) {
	setupCaptureInputSessionTest(t)

	var got engine.CaptureRequest
	engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
		got = req
		return nil
	})
	if _, err := PrepareInputReplay(validRuntimeReplay()); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	if _, err := consumeInputTick(InputReplayState{}, nil, 0); err != nil {
		t.Fatal(err)
	}
	capture("empty-replay", nil)
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if got.InputTick == nil || *got.InputTick != 0 {
		t.Fatalf("empty replay capture input tick = %v, want 0", got.InputTick)
	}
}

func TestInputSessionCaptureKeyRequestsSnapshots(t *testing.T) {
	_, game, _ := setupInputSourceTest(t)
	setupCaptureTest(t)
	var got []engine.CaptureRequest
	engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
		got = append(got, req)
		return nil
	})
	// Real input has no key edges. Playback supplies two separate presses of
	// the capture key, its release, and unrelated key edges in the same tick.
	replay := validRuntimeReplay(InputReplayFrame{
		Frame: 0,
		State: InputReplayState{KeysDown: []int64{int64(KeyP)}},
		KeyEvents: []InputReplayKeyEvent{
			{Key: int64(KeyP), Pressed: true},
			{Key: int64(KeyP), Pressed: false},
			{Key: int64(KeyP), Pressed: true},
			{Key: int64(KeyA), Pressed: true},
			{Key: int64(KeyA), Pressed: false},
		},
	})
	if _, err := PrepareInputReplay(replay, InputSessionOptions{CaptureKey: KeyP}); err != nil {
		t.Fatal(err)
	}
	claimPreparedSession(t, game)
	frame := clock.Frame() + 1
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("capture-key replay frame was not prepared")
	}
	if engine.HasPendingCaptures() || len(got) != 0 {
		t.Fatal("capture key requested a screenshot before clock advancement")
	}
	clock.Update(1.0/30, 30)
	game.dispatchFrameEvents()
	if !engine.HasPendingCaptures() || len(got) != 0 {
		t.Fatal("capture key did not queue screenshots for the end of the frame")
	}
	game.dispatchFrameEvents()
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("configured capture requests = %+v, want one per matching press", got)
	}
	for i, request := range got {
		if request.Frame != frame || request.InputTick == nil || *request.InputTick != 0 {
			t.Fatalf("capture %d = %+v, want engine frame %d and input tick 0", i, request, frame)
		}
	}
}

func TestCaptureDoesNotInheritInputTickAcrossGameReset(t *testing.T) {
	setupCaptureInputSessionTest(t)

	var got []engine.CaptureRequest
	engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
		got = append(got, req)
		return nil
	})
	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	for i := 0; i < 2; i++ {
		if _, err := consumeInputTick(InputReplayState{}, nil, 1.0/30.0); err != nil {
			t.Fatal(err)
		}
	}
	capture("old-game", nil)
	game.abortInputSession("game reset")
	capture("aborted-game", nil)
	game.lifecycle.reset()
	if _, err := PrepareInputReplay(validRuntimeReplay()); err != nil {
		t.Fatal(err)
	}
	claimPreparedSession(t, game)
	capture("new-game-before-input", nil)
	if _, err := consumeInputTick(InputReplayState{}, nil, 1.0/30.0); err != nil {
		t.Fatal(err)
	}
	capture("new-game-first-input", nil)
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}

	if len(got) != 4 {
		t.Fatalf("capture requests = %+v, want 4 requests", got)
	}
	if got[0].InputTick == nil || *got[0].InputTick != 1 {
		t.Fatalf("old-game capture = %+v", got)
	}
	for _, request := range got[1:3] {
		if request.InputTick != nil {
			t.Fatalf("%s inherited input tick %d", request.Name, *request.InputTick)
		}
	}
	if got[3].InputTick == nil || *got[3].InputTick != 0 {
		t.Fatalf("new-game first input capture = %+v, want input tick 0", got[3])
	}
}

func TestReplayCompletionWaitsForCaptureDispatch(t *testing.T) {
	setupCaptureInputSessionTest(t)
	replay := validRuntimeReplay(InputReplayFrame{Frame: 0, State: InputReplayState{}})
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	session := claimPreparedSession(t, game)
	if _, err := consumeInputTick(InputReplayState{}, nil, 1); err != nil {
		t.Fatal(err)
	}

	finishedDuringCapture := true
	engine.SetCaptureHandler(func(engine.CaptureRequest) error {
		finishedDuringCapture = session.status().Completed
		return nil
	})
	capture("final", nil)
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if finishedDuringCapture {
		t.Fatal("replay completed before final-frame capture dispatch")
	}
	completed, err := session.completeReplayFrame(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !completed || !session.status().Completed {
		t.Fatal("replay did not complete after capture dispatch at frame end")
	}
}

func TestResetBeforeFinalFrameEndAbortsReplay(t *testing.T) {
	resetInputSessionTest(t)
	replay := validRuntimeReplay(InputReplayFrame{Frame: 0, State: InputReplayState{}})
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	session := claimPreparedSession(t, game)
	if _, err := consumeInputTick(InputReplayState{}, nil, 1); err != nil {
		t.Fatal(err)
	}
	game.abortInputSession("game reset before frame end")
	if status := session.status(); status.Phase != InputSessionPhaseAborted || status.Completed {
		t.Fatalf("replay status after reset = %+v", status)
	}
	completed, err := session.completeReplayFrame(nil)
	if err != nil {
		t.Fatal(err)
	}
	if completed {
		t.Fatal("aborted replay completed after reset")
	}
}

func TestReplayQueriesDuringTicksAndClose(t *testing.T) {
	resetInputSessionTest(t)
	state := InputReplayState{Mouse: InputReplayMouse{X: 5}, KeysDown: []int64{int64(KeyA)}}
	replay := validRuntimeReplay()
	replay.Initial = state
	for i := range 100 {
		replay.Frames = append(replay.Frames, InputReplayFrame{Frame: int64(i), Time: float64(i) / 30, State: state})
	}
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	session := claimPreparedSession(t, &Game{})
	var readers sync.WaitGroup
	defer readers.Wait()
	for range 2 {
		readers.Go(func() {
			for range 1000 {
				session.playbackInput(KeyA)
				session.playbackInput(KeyAny)
			}
		})
	}
	for range replay.Frames {
		if _, err := consumeInputTick(InputReplayState{}, nil, 1.0/30); err != nil {
			t.Fatal(err)
		}
	}
	session.close("finished")
	if _, ok := session.playbackInput(KeyA); ok {
		t.Fatal("closed replay still overrides keys")
	}
}

func TestInputSessionRejectsInvalidModesAndLifecycleStates(t *testing.T) {
	resetInputSessionTest(t)
	for _, plan := range []*inputSessionPlan{
		{mode: InputSessionModeIdle},
		{mode: InputSessionModeRecording, fixedTimestep: -1},
		{mode: InputSessionModeReplaying, replay: InputReplay{}},
	} {
		if session, err := newInputSession(plan); err == nil || session != nil {
			t.Fatalf("invalid plan %+v: session=%v error=%v", plan, session, err)
		}
	}
	if _, err := PrepareInputReplay(InputReplay{}); err == nil {
		t.Fatal("invalid replay accepted")
	}
	if _, err := PrepareInputReplay(validRuntimeReplay(), InputSessionOptions{}, InputSessionOptions{}); err == nil {
		t.Fatal("multiple options accepted")
	}
	if _, err := FinishInputRecording(); !errors.Is(err, ErrInputSessionNotRecording) {
		t.Fatalf("recording without game: %v", err)
	}
	engine.SetGame(&Game{})
	if _, err := FinishInputRecording(); !errors.Is(err, ErrInputSessionNotRecording) {
		t.Fatalf("recording without session: %v", err)
	}
	var missing *Game
	if missing.inputSessionUnavailable() || missing.currentInputSession() != nil {
		t.Fatal("nil game has an input session")
	}
	missing.abortInputSession("absent")
	for _, tt := range []struct {
		name    string
		session *inputSession
		want    string
	}{
		{"replay", &inputSession{mode: InputSessionModeReplaying}, "not recording"},
		{"aborted", &inputSession{mode: InputSessionModeRecording, phase: InputSessionPhaseAborted, abortReason: "stopped"}, "stopped"},
		{"prepared", &inputSession{mode: InputSessionModeRecording, phase: InputSessionPhasePrepared}, "phase"},
		{"uninitialized controller", &inputSession{mode: InputSessionModeRecording, phase: InputSessionPhaseRunning}, "not recording"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.session.finishRecordingResult(nil); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("finish error = %v, want %q", err, tt.want)
			}
			if tt.name == "uninitialized controller" && tt.session.phase != InputSessionPhaseRunning {
				t.Fatal("failed finish did not roll back phase")
			}
		})
	}
}

func TestInputPreparationTokenWrapAndFailedAttachment(t *testing.T) {
	resetInputSessionTest(t)
	old := preparedInputSession.nextToken
	t.Cleanup(func() { preparedInputSession.nextToken = old })
	preparedInputSession.nextToken = ^uint64(0)
	prepared, err := prepareInputSession(inputSessionPlan{mode: InputSessionModeRecording, fixedTimestep: -1})
	if err != nil || prepared.token != 1 {
		t.Fatalf("wrapped token = %+v, %v", prepared, err)
	}
	game := &Game{}
	if err := game.attachPreparedInputSession(); err == nil {
		t.Fatal("invalid prepared plan attached")
	}
	status, ok := game.terminalInputSessionStatus()
	if !ok || status.Mode != InputSessionModeRecording || status.Phase != InputSessionPhaseAborted || status.Error == "" {
		t.Fatalf("failed attachment status: %+v", status)
	}
	if _, exists := preparedInputSessionStatus(); exists {
		t.Fatal("failed attachment retained the prepared plan")
	}
}

func TestInputSessionFreezeFailures(t *testing.T) {
	resetInputSessionTest(t)
	freeze := func() { panic("host pause failed") }
	if err := freezeInputSession(nil); err != nil {
		t.Fatal(err)
	}
	if err := freezeInputSession(freeze); err == nil || !strings.Contains(err.Error(), "host pause failed") {
		t.Fatalf("freeze error = %v", err)
	}
	recording, err := newInputSession(&inputSessionPlan{mode: InputSessionModeRecording, fixedTimestep: 1.0 / 30})
	if err != nil {
		t.Fatal(err)
	}
	defer recording.close("done")
	if _, err := recording.finishRecordingResult(freeze); err == nil || recording.phase != InputSessionPhaseRunning || recording.result.json != "" || len(recording.result.replay.Frames) != 0 {
		t.Fatalf("failed finish left partial result: %+v, %v", recording.status(), err)
	}
	if _, err := recording.finishRecordingResult(nil); err != nil {
		t.Fatalf("retry finishing: %v", err)
	}
	replay, err := newInputSession(&inputSessionPlan{mode: InputSessionModeReplaying, replay: validRuntimeReplay()})
	if err != nil {
		t.Fatal(err)
	}
	defer replay.close("done")
	if _, err := replay.consumeSampledInputTick(1.0/30, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
		return InputReplayState{}, nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if complete, err := replay.completeReplayFrame(freeze); complete || err == nil || replay.phase != InputSessionPhaseAborted {
		t.Fatalf("failed replay freeze: %v, %v, %+v", complete, err, replay.status())
	}
	if _, ok := replay.playbackInput(KeyAny); ok {
		t.Fatal("aborted replay still overrides input")
	}
}

func TestInputSessionPendingReplayAndTickErrors(t *testing.T) {
	resetInputSessionTest(t)
	session, err := newInputSession(&inputSessionPlan{mode: InputSessionModeReplaying, replay: validRuntimeReplay(InputReplayFrame{Frame: 0})})
	if err != nil {
		t.Fatal(err)
	}
	defer session.close("done")
	session.phase = InputSessionPhaseFinishing
	if completed, err := session.completeReplayFrame(func() { t.Fatal("non-exhausted replay paused") }); completed || err != nil {
		t.Fatalf("premature completion = %v, %v", completed, err)
	}
	session.phase = InputSessionPhaseAborted
	game := &Game{inputSlot: inputSessionSlot{session: session}}
	engine.SetGame(game)
	if captureInputTick() != nil {
		t.Fatal("aborted session retained capture tick")
	}
	if _, err := session.consumeSampledInputTick(0, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
		t.Fatal("inactive session sampled input")
		return InputReplayState{}, nil, nil
	}); err == nil {
		t.Fatal("aborted session accepted a tick")
	}
	recording := &inputSession{mode: InputSessionModeRecording, phase: InputSessionPhaseRunning, initialPending: true}
	if _, err := recording.consumeSampledInputTick(0, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
		return InputReplayState{}, nil, nil
	}); err == nil {
		t.Fatal("uninitialized recorder synchronized initial state")
	}
	initial := inputStateBeforeEvents(InputReplayState{Buttons: 1}, []InputReplayMouseEvent{{Button: 0}, {Button: 4}}, nil)
	if initial.Buttons != 1 {
		t.Fatal("invalid mouse events changed initial state")
	}
}

func TestReplayInputQueriesUseFrozenState(t *testing.T) {
	_, game, mgr := setupInputSourceTest(t)
	mgr.point = mathf.Vec2{X: 90, Y: 80}
	game.inputMgr.mousePos = mathf.Vec2{X: 3, Y: 4}
	if got := game.inputMgr.effectiveMousePos(); got != (mathf.Vec2{X: 3, Y: 4}) {
		t.Fatalf("live position = %v", got)
	}
	pkgengine.InputMgr = &replayQueryInputMgr{mgr}
	if game.inputMgr.effectiveKeyPressed(KeyA) || !game.inputMgr.effectiveKeyPressed(KeyB) {
		t.Fatal("live key query ignored backend")
	}
	_ = game.inputMgr.effectiveMousePressed()
	game.inputSlot.session = &inputSession{mode: InputSessionModeReplaying, phase: InputSessionPhaseRunning, current: InputReplayState{KeysDown: []int64{int64(KeyA)}, Mouse: InputReplayMouse{X: 5, Y: 6}, Buttons: 2}}
	for _, phase := range []InputSessionPhase{InputSessionPhaseRunning, InputSessionPhaseFinishing, InputSessionPhaseCompleted} {
		game.inputSlot.session.phase = phase
		if !game.KeyPressed(KeyAny) || !game.KeyPressed(KeyA) || game.KeyPressed(KeyB) || !game.MousePressed() || game.MouseX() != 5 || game.MouseY() != 6 {
			t.Fatalf("queries ignored frozen replay state in phase %q", phase)
		}
	}
	snapshot, ok := game.inputSlot.session.playbackInput(KeyA)
	if !ok || !snapshot.keyPressed || snapshot.position.X != 5 || snapshot.buttons != 2 {
		t.Fatalf("unexpected scalar snapshot: %+v, %v", snapshot, ok)
	}
	game.inputSlot.session.current.Mouse.X = 99
	game.inputSlot.session.current.KeysDown = nil
	if !snapshot.keyPressed || snapshot.position.X != 5 {
		t.Fatalf("scalar snapshot changed with session state: %+v", snapshot)
	}
	if game.KeyPressed(KeyAny) || game.KeyPressed(KeyA) {
		t.Fatal("empty replay key state reported a held key")
	}
	// Keep a replay-held key that differs from the live backend.
	game.inputSlot.session.current.KeysDown = []int64{int64(KeyA)}
	for _, inactive := range []struct {
		mode  InputSessionMode
		phase InputSessionPhase
	}{
		{InputSessionModeRecording, InputSessionPhaseRunning},
		{InputSessionModeReplaying, InputSessionPhaseAborted},
	} {
		game.inputSlot.session.mode = inactive.mode
		game.inputSlot.session.phase = inactive.phase
		if game.KeyPressed(KeyA) || !game.KeyPressed(KeyB) || game.MouseX() != 3 {
			t.Fatal("inactive replay did not return to live queries")
		}
	}
	game.inputSlot.session = nil
}

func TestInputAttachmentCanceledWhilePreparingEnvironment(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	scriptRandom.mu.Lock()
	unlock := sync.OnceFunc(scriptRandom.mu.Unlock)
	done := make(chan error, 1)
	workerDone := make(chan struct{})
	t.Cleanup(func() {
		unlock()
		select {
		case <-workerDone:
		case <-time.After(time.Second):
			t.Error("attachment worker did not stop")
		}
	})
	go func() {
		defer close(workerDone)
		done <- game.attachPreparedInputSession()
	}()
	deadline := time.Now().Add(time.Second)
	for {
		if delta, enabled := clock.FixedDeltaTime(); enabled && delta == 1.0/30 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("attachment did not reach environment setup")
		}
		runtime.Gosched()
	}
	game.abortInputSession("closed during preparation")
	unlock()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "lifecycle ended") {
			t.Fatalf("attachment error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("attachment did not unwind")
	}
	if game.currentInputSession() != nil {
		t.Fatal("canceled game retained input session")
	}
	if _, enabled := clock.FixedDeltaTime(); enabled {
		t.Fatal("failed attachment did not restore environment")
	}
}

func TestRecordingTooLargeToEncodeRollsBackFinish(t *testing.T) {
	// A valid recording may exceed the codec's payload limit even though each
	// individual input sample passes validation.
	const keyCount = (16<<20)/20 + 1
	keys := make([]int64, keyCount)
	for i := range keys {
		keys[i] = math.MaxInt64 - int64(keyCount) + int64(i)
	}
	session := &inputSession{mode: InputSessionModeRecording, phase: InputSessionPhaseRunning}
	if err := session.controller.StartRecording(InputReplayState{KeysDown: keys}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := session.finishRecordingResult(nil); err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("encoding error = %v", err)
	}
	if session.phase != InputSessionPhaseRunning || session.result.json != "" {
		t.Fatal("failed serialization committed a recording")
	}
}

func TestInputPreparationRejectsUnrepresentableTimestepAndDuplicates(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputRecording(math.SmallestNonzeroFloat64); err == nil {
		t.Fatal("FPS with infinite timestep accepted")
	}
	first, err := PrepareInputRecording(30)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareInputReplay(validRuntimeReplay()); !errors.Is(err, ErrInputSessionActive) {
		t.Fatalf("duplicate preparation error = %v", err)
	}
	if !first.Cancel() {
		t.Fatal("duplicate preparation replaced the original descriptor")
	}
	(&Game{}).OnEngineFrameEnd()
}

// Exercise public finish calls after the normal input, scheduler, and frame-end
// lifecycle; engine managers remain the standard test fixtures.
func TestFinishRecordingPreservesResultsAfterEngineFrame(t *testing.T) {
	for _, name := range []string{"Replay-first", "JSON-first"} {
		t.Run(name, func(t *testing.T) {
			currentScheduler, game, _ := setupInputSourceTest(t)
			setupCaptureFlushSpriteMgr(t)
			game.camera = &cameraImpl{g: game}
			game.scene.init()
			game.syncBuffer = engine.NewSpriteSyncBuffer(1)
			oldExt := pkgengine.ExtMgr
			pkgengine.ExtMgr = &replayPauseTestExtMgr{}
			t.Cleanup(func() { pkgengine.ExtMgr = oldExt })
			if _, err := PrepareInputRecording(30); err != nil {
				t.Fatal(err)
			}
			claimPreparedSession(t, game)
			game.lifecycle.loaded.Store(true)
			game.lifecycle.startDispatched.Store(true)
			handled := 0
			game.OnCond(func() bool { return true }, func() { handled++ })
			game.OnEngineBeforeUpdate(1.0 / 30)
			clock.Update(1.0/30, 30)
			game.OnEngineUpdate(1.0 / 30)
			currentScheduler.Update()
			game.OnEngineRender(1.0 / 30)
			game.OnEngineFrameEnd()
			if handled != 1 || GetInputSessionStatus().FrameCount != 1 {
				t.Fatalf("engine frame not exercised: handlers=%d, status=%+v", handled, GetInputSessionStatus())
			}
			var first InputReplay
			var cached string
			if name == "JSON-first" {
				var err error
				cached, err = FinishInputRecordingJSON()
				if err != nil {
					t.Fatal(err)
				}
			}
			for attempt := range 2 {
				replay, err := FinishInputRecording()
				if err != nil {
					t.Fatal(err)
				}
				if attempt == 0 {
					first = replay
				} else if !reflect.DeepEqual(replay, first) {
					t.Error("consecutive public Replay results differ")
				}
				encoded, err := EncodeInputReplay(replay)
				if err != nil {
					t.Fatal(err)
				}
				got, err := FinishInputRecordingJSON()
				if err != nil {
					t.Fatal(err)
				}
				if cached == "" {
					cached = got
				}
				if got != cached || encoded != cached || !GetInputSessionStatus().Completed {
					t.Fatalf("inconsistent completed result: encoded=%s, cached=%s, returned=%s", encoded, cached, got)
				}
			}
		})
	}
}

func BenchmarkReplayInputQueries(b *testing.B) {
	game := &Game{inputSlot: inputSessionSlot{session: &inputSession{
		mode: InputSessionModeReplaying, phase: InputSessionPhaseRunning,
		current: InputReplayState{Mouse: InputReplayMouse{X: 5, Y: 6}, Buttons: 1, KeysDown: make([]int64, 128)},
	}}}
	game.inputMgr.g = game
	for i := range game.inputSlot.session.current.KeysDown {
		game.inputSlot.session.current.KeysDown[i] = int64(i + 1)
	}
	for _, query := range []struct {
		name string
		run  func()
	}{
		{"key", func() { game.inputMgr.effectiveKeyPressed(KeyA) }},
		{"mouse-position", func() { game.inputMgr.effectiveMousePos() }},
		{"mouse-buttons", func() { game.inputMgr.effectiveMousePressed() }},
	} {
		b.Run(query.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				query.run()
			}
		})
	}
}

func BenchmarkPrepareInputFrame(b *testing.B) {
	mgr := setupInputSourceManager(b)
	for _, scenario := range []struct {
		name string
		step float64
	}{{"idle", 0}, {"moving", 2}} {
		b.Run(scenario.name, func(b *testing.B) {
			game := &Game{}
			game.inputMgr.g = game
			mgr.point = mathf.Vec2{}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				mgr.point.X += scenario.step
				game.inputMgr.prepareInputFrame(1.0 / 30)
			}
		})
	}
}

func (m *inputSourceTestManager) GetGlobalMousePos() mathf.Vec2 {
	m.samples++
	return m.point
}

func (m *inputFrameFailureExtMgr) OnRuntimePanic(message string) {
	m.messages = append(m.messages, message)
}

func (m *inputFrameFailureExtMgr) RequestExit(code int64) {
	m.exits = append(m.exits, code)
}

func (inputFrameFailurePlatform) IsMainThread() bool { return true }

func (*replayQueryInputMgr) GetKey(key int64) bool { return key == int64(KeyB) }

func (m *dragSpriteMgr) CheckCollisionWithPoint(id pkgengine.Object, point mathf.Vec2, trigger bool) bool {
	if m.hit != nil {
		return m.hit(id, point)
	}
	return m.clickThroughSpriteMgr.CheckCollisionWithPoint(id, point, trigger)
}

func (*dragSpriteMgr) SetTransform(pkgengine.Object, mathf.Vec2, float64, mathf.Vec2, bool, mathf.Vec2) {
}

func (m *dragCameraMgr) GetViewportRect() mathf.Rect2 { return m.viewport }

func (m *dragCameraMgr) GetGlobalCameraRect() mathf.Rect2 { return m.world }

// consumeSampledInputTick holds the session boundary while the engine state is
// sampled and resolved into one effective input tick.
func (s *inputSession) consumeSampledInputTick(
	delta float64,
	sample func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent),
) (inputSessionTick, error) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	return s.consumeSampledInputTickLocked(delta, sample)
}

func setupInputFrameGame(t *testing.T) (*coroutine.Scheduler, *Game) {
	t.Helper()
	clock.ClearTimers()
	clock.Start()
	t.Cleanup(clock.ClearTimers)
	currentScheduler, game := setupRuntimeEventGame(t)
	game.inputMgr.init(game)
	return currentScheduler, game
}

func setupSpriteDragTest(t *testing.T) (*coroutine.Scheduler, *Game, *SpriteImpl) {
	t.Helper()
	currentScheduler, game := setupInputFrameGame(t)
	game.scene.init()
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
	game.scene.add(sprite)
	return currentScheduler, game, sprite
}

func newDragTestSprite(game *Game, name string, id pkgengine.Object) *SpriteImpl {
	sprite := newClickTestSprite(game, name, id, false)
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{X: 10, Y: 20, RotationStyle: "normal"})
	sprite.costumes = []*costume{newCostumeWithSize(10, 10)}
	sprite.runtimeState.scale = 1
	sprite.runtimeState.proxy.OnAnimationLoopedEvent = &pkgengine.Event0{}
	sprite.runtimeState.proxy.OnAnimationFinishedEvent = &pkgengine.Event0{}
	return sprite
}

func dispatchSpriteDragFrame(currentScheduler *coroutine.Scheduler, game *Game, point mathf.Vec2, events ...event) {
	game.inputMgr.frame.dragProjection = game.inputMgr.sampleDragProjection()
	game.inputMgr.mousePos = point
	game.inputMgr.frame.pending = &inputFrame{events: events}
	game.dispatchFrameEvents()
	currentScheduler.Update()
}

func assertSpriteDragPosition(t *testing.T, sprite *SpriteImpl, x, y float64) {
	t.Helper()
	if gotX, gotY := sprite.getXY(); gotX != x || gotY != y {
		t.Fatalf("sprite position = (%v, %v), want (%v, %v)", gotX, gotY, x, y)
	}
}

// resetInputSessionState isolates tests without adding a production path for
// resetting input independently of the Game lifecycle.
func resetInputSessionState() {
	preparedInputSession.Lock()
	preparedInputSession.plan = nil
	preparedInputSession.claimed = false
	preparedInputSession.Unlock()
	if game := currentGame(); game != nil {
		game.abortInputSession("input session reset")
		game.inputSlot.mu.Lock()
		game.inputSlot.terminal = InputSessionStatus{}
		game.inputSlot.mu.Unlock()
	}
}

func consumeInputTick(live InputReplayState, keyEvents []InputReplayKeyEvent, delta float64) (inputSessionTick, error) {
	return consumeInputTickWithMouseEvents(live, nil, keyEvents, delta)
}

func consumeInputTickWithMouseEvents(
	live InputReplayState,
	mouseEvents []InputReplayMouseEvent,
	keyEvents []InputReplayKeyEvent,
	delta float64,
) (inputSessionTick, error) {
	session := activeInputSession()
	if session == nil {
		return inputSessionTick{}, errors.New("no active input session")
	}
	return session.consumeSampledInputTick(delta, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
		return live, mouseEvents, keyEvents
	})
}

func resetInputSessionTest(t *testing.T) {
	t.Helper()
	resetInputSessionState()
	engine.SetGame(nil)
	clock.SetFixedDeltaTime(0)
	t.Cleanup(func() {
		resetInputSessionState()
		engine.SetGame(nil)
		clock.SetFixedDeltaTime(0)
		ResetRandomSeed()
	})
}

// setupCaptureInputSessionTest extends the replay fixture with capture cleanup.
func setupCaptureInputSessionTest(t *testing.T) {
	t.Helper()
	resetInputSessionTest(t)
	setupCaptureTest(t)
}

func claimPreparedSession(t *testing.T, game *Game) *inputSession {
	t.Helper()
	engine.SetGame(game)
	if err := game.attachPreparedInputSession(); err != nil {
		t.Fatal(err)
	}
	session := game.currentInputSession()
	if session == nil {
		t.Fatal("prepared input session was not attached")
	}
	return session
}

func validRuntimeReplay(frames ...InputReplayFrame) InputReplay {
	return InputReplay{
		Format:        InputReplayFormat,
		Version:       InputReplayVersion,
		FixedTimestep: 1.0 / 30,
		Frames:        frames,
	}
}

func setupInputSourceTest(t *testing.T) (*coroutine.Scheduler, *Game, *inputSourceTestManager) {
	t.Helper()
	resetInputSessionTest(t)
	currentScheduler, game := setupInputFrameGame(t)
	mgr := setupInputSourceManager(t)
	t.Cleanup(func() { game.abortInputSession("test finished") })
	return currentScheduler, game, mgr
}

func setupInputSourceManager(t testing.TB) *inputSourceTestManager {
	t.Helper()
	setupImmediateEngineCalls(t)
	original := pkgengine.InputMgr
	mgr := &inputSourceTestManager{}
	pkgengine.InputMgr = mgr
	t.Cleanup(func() {
		pkgengine.InputMgr = original
	})
	return mgr
}
