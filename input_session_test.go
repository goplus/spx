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
	"math"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/engine"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type replayQueryInputMgr struct{ *inputSourceTestManager }

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
		capture("pending-cleanup")
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

func TestCaptureUsesCurrentInputTick(t *testing.T) {
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
	capture("before-tick")
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
	capture("tick-zero")
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
	if _, err := consumeInputTick(InputReplayState{}, nil, 1.0/30.0); err != nil {
		t.Fatal(err)
	}
	capture("tick-one")
	if _, err := consumeInputTick(InputReplayState{}, nil, 1.0/30.0); err != nil {
		t.Fatal(err)
	}
	if *savedTick != 0 {
		t.Fatalf("saved capture tick changed to %d after input advanced", *savedTick)
	}
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[2].Name != "tick-one" || got[2].InputTick == nil || *got[2].InputTick != 1 {
		t.Fatalf("captures = %+v, want tick-one at tick 1 before the next input tick", got)
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
	capture("empty-replay")
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if got.InputTick == nil || *got.InputTick != 0 {
		t.Fatalf("empty replay capture input tick = %v, want 0", got.InputTick)
	}
}

func TestInputSessionCaptureKeyQueuesCaptures(t *testing.T) {
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
	capture("old-game")
	game.abortInputSession("game reset")
	capture("aborted-game")
	game.lifecycle.reset()
	if _, err := PrepareInputReplay(validRuntimeReplay()); err != nil {
		t.Fatal(err)
	}
	claimPreparedSession(t, game)
	capture("new-game-before-input")
	if _, err := consumeInputTick(InputReplayState{}, nil, 1.0/30.0); err != nil {
		t.Fatal(err)
	}
	capture("new-game-first-input")
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
	capture("final")
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

func (*replayQueryInputMgr) GetKey(key int64) bool { return key == int64(KeyB) }

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

// setupCaptureInputSessionTest extends the session fixture with capture cleanup.
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
