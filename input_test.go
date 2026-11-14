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

func setupInputFrameGame(t *testing.T) (*coroutine.Scheduler, *Game) {
	t.Helper()
	clock.ClearTimers()
	clock.Start()
	t.Cleanup(clock.ClearTimers)
	currentScheduler, game := setupRuntimeEventGame(t)
	game.inputMgr.init(game)
	return currentScheduler, game
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
