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
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	coreruntime "github.com/goplus/spx/v3/internal/core/runtime"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

// resetInputSessionState isolates tests without adding a production path for
// resetting input independently of the Game lifecycle.
func resetInputSessionState() {
	preparedInputSession.Lock()
	preparedInputSession.plan = nil
	preparedInputSession.claimed = false
	preparedInputSession.Unlock()
	if game := currentGame(); game != nil {
		game.abortInputSession("input session reset")
		game.inputSessionMu.Lock()
		game.inputTerminal = InputSessionStatus{}
		game.inputSessionMu.Unlock()
	}
}

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
	itime.SetFixedDeltaTime(0)
	t.Cleanup(func() {
		resetInputSessionState()
		engine.SetGame(nil)
		itime.SetFixedDeltaTime(0)
		ResetRandomSeed()
	})
}

// setupSnapshotInputSessionTest extends the replay fixture with capture cleanup.
func setupSnapshotInputSessionTest(t *testing.T) {
	t.Helper()
	resetInputSessionTest(t)
	reset := func() {
		engine.ResetFrameRuntime()
		engine.SetCaptureHandler(nil)
	}
	reset()
	t.Cleanup(reset)
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

func TestInputReplayRuntimeUsesIndependentInputTicks(t *testing.T) {
	resetInputSessionTest(t)
	itime.SetFixedDeltaTime(1.0 / 60.0)

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
	game.resetBootstrap()

	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	claimPreparedSession(t, game)
	replayed := []float64{Rand__0(1, 100), Rand__1(0, 1), Rand__0(1, 100)}
	if !reflect.DeepEqual(replayed, recorded) {
		t.Fatalf("replay random = %v, want recording random %v", replayed, recorded)
	}
}

func TestInputReplayRuntimePreservesShortClickWithinOneTick(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	wantEdges := []InputReplayMouseEvent{
		{Button: 1, Pressed: true},
		{Button: 1, Pressed: false},
	}
	recordedTick, err := consumeInputTickWithMouseEvents(InputReplayState{}, wantEdges, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !recordedTick.firstTick || !reflect.DeepEqual(recordedTick.frame.MouseEvents, wantEdges) {
		t.Fatalf("recorded short click tick = %+v, want edges %+v", recordedTick, wantEdges)
	}
	replay, err := FinishInputRecording()
	if err != nil {
		t.Fatal(err)
	}
	game.abortInputSession("recording game ended")
	game.resetBootstrap()

	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	session := claimPreparedSession(t, game)
	replayedTick, err := consumeInputTickWithMouseEvents(
		InputReplayState{Buttons: 1},
		[]InputReplayMouseEvent{{Button: 1, Pressed: true}},
		nil,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayedTick.frame.MouseEvents, wantEdges) || replayedTick.frame.State.Buttons != 0 {
		t.Fatalf("replayed short click tick = %+v, want released state and edges %+v", replayedTick, wantEdges)
	}
	if status := session.status(); status.Phase != InputSessionPhaseFinishing || status.Completed || !status.Exhausted ||
		!status.HasCurrentTick || status.CurrentTick != replayedTick.frame.Frame {
		t.Fatalf("final replay tick status = %+v", status)
	}
}

func TestLiveAndReplayInputSamplesMapToEquivalentEvents(t *testing.T) {
	point := mathf.Vec2{X: 4, Y: 5}
	mouseEvents := []engine.MouseEvent{
		{Id: 1, IsPressed: true},
		{Id: 1, IsPressed: false},
	}
	keyEvents := []engine.KeyEvent{
		{Id: int64(KeyA), IsPressed: true},
		{Id: int64(KeyB), IsPressed: false},
	}
	liveFrame := coreruntime.InputFrame{
		Point:                  point,
		MouseEvents:            mouseEvents,
		KeyEvents:              keyEvents,
		MouseMovementThreshold: mouseMovementThreshold,
	}
	replayFrame := coreruntime.InputFrame{
		Point:                  point,
		MouseEvents:            engineMouseEventsFromReplay(replayMouseEventsFromEngine(mouseEvents), nil),
		KeyEvents:              engineKeyEventsFromReplay(replayKeyEventsFromEngine(keyEvents), nil),
		MouseMovementThreshold: mouseMovementThreshold,
	}

	var liveState, replayState coreruntime.InputFrameState
	var liveEvents, replayEvents []event
	liveState.Process(liveFrame, inputFrameEventHooks(func(ev event) {
		liveEvents = append(liveEvents, ev)
	}))
	replayState.Process(replayFrame, inputFrameEventHooks(func(ev event) {
		replayEvents = append(replayEvents, ev)
	}))

	want := []event{
		&eventLeftButtonDown{Pos: point},
		&eventLeftButtonUp{Pos: point},
		&eventMouseMove{Pos: point},
		&eventKeyDown{Key: KeyA},
	}
	if !reflect.DeepEqual(liveEvents, want) {
		t.Fatalf("live events = %#v, want %#v", liveEvents, want)
	}
	if !reflect.DeepEqual(replayEvents, liveEvents) {
		t.Fatalf("replay events = %#v, live events = %#v", replayEvents, liveEvents)
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
		{name: "default", fps: itime.DefaultFPS},
		{name: "custom", fps: 24, previousTimestep: 1.0 / 60},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetInputSessionTest(t)
			itime.SetFixedDeltaTime(tt.previousTimestep)
			previousFixed := tt.previousTimestep > 0
			if _, err := PrepareInputRecording(tt.fps); err != nil {
				t.Fatal(err)
			}
			if got, fixed := itime.FixedDeltaTime(); fixed != previousFixed || got != tt.previousTimestep {
				t.Fatalf("prepared session fixed timestep = (%v, %v), want (%v, %v)", got, fixed, tt.previousTimestep, previousFixed)
			}
			game := &Game{}
			claimPreparedSession(t, game)
			want := 1 / tt.fps
			if got, ok := itime.FixedDeltaTime(); !ok || got != want {
				t.Fatalf("recording fixed timestep = (%v, %v), want (%v, true)", got, ok, want)
			}
			replay, err := FinishInputRecording()
			if err != nil {
				t.Fatal(err)
			}
			if replay.FixedTimestep != want {
				t.Fatalf("recorded fixed timestep = %v, want %v", replay.FixedTimestep, want)
			}
			if got, ok := itime.FixedDeltaTime(); !ok || got != want {
				t.Fatalf("completed session fixed timestep = (%v, %v), want (%v, true) until Game ends", got, ok, want)
			}
			game.abortInputSession("game ended")
			if got, fixed := itime.FixedDeltaTime(); fixed != previousFixed || got != tt.previousTimestep {
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
			itime.SetFixedDeltaTime(1.0 / 60)
			replay := validRuntimeReplay()
			replay.FixedTimestep = tt.timestep
			if _, err := PrepareInputReplay(replay); err != nil {
				t.Fatal(err)
			}
			game := &Game{}
			claimPreparedSession(t, game)
			wantFixed := tt.timestep > 0
			if got, fixed := itime.FixedDeltaTime(); fixed != wantFixed || got != tt.timestep {
				t.Fatalf("replay fixed timestep = (%v, %v), want (%v, %v)", got, fixed, tt.timestep, wantFixed)
			}
			if status := GetInputSessionStatus(); status.Mode != InputSessionModeReplaying || status.Phase != InputSessionPhaseRunning {
				t.Fatalf("replay session status = %+v", status)
			}
			game.abortInputSession("game ended")
			if got, fixed := itime.FixedDeltaTime(); !fixed || got != 1.0/60 {
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
	game.resetBootstrap()
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
	game.resetBootstrap()

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
	game.lifecycleState.IsRunned.Store(true)
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
	itime.SetFixedDeltaTime(1.0 / 60)
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
	if got, ok := itime.FixedDeltaTime(); !ok || got != 1.0/60 {
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
	game.lifecycleState.IsRunned.Store(true)
	game.OnEngineDestroy()
	if status := session.status(); status.Phase != InputSessionPhaseAborted {
		t.Fatalf("destroyed Game session status = %+v", status)
	}
	if game.currentInputSession() != nil {
		t.Fatal("destroyed Game retained input session")
	}
	if game.lifecycleState.IsRunned.Load() {
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

func TestSnapshotInputSessionFixtureClearsGameAndPendingCaptures(t *testing.T) {
	setupSnapshotInputSessionTest(t)

	t.Run("recording", func(t *testing.T) {
		setupSnapshotInputSessionTest(t)
		if _, err := PrepareInputRecording(30); err != nil {
			t.Fatal(err)
		}
		claimPreparedSession(t, &Game{})
		Snapshot("pending-cleanup", nil)
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
	if _, fixed := itime.FixedDeltaTime(); fixed {
		t.Error("fixture cleanup left the fixed timestep enabled")
	}
	if engine.HasPendingCaptures() {
		t.Error("fixture cleanup left pending captures")
	}
}

func TestSnapshotUsesCurrentInputReplayTick(t *testing.T) {
	setupSnapshotInputSessionTest(t)

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
	Snapshot("before-tick", nil)
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
	Snapshot("tick-zero", nil)
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("captures after tick zero = %d, want 2", len(got))
	}
	if got[1].InputTick == nil || *got[1].InputTick != 0 {
		t.Fatalf("capture input tick = %v, want 0", got[1].InputTick)
	}
}

func TestSnapshotUsesSyntheticTickZeroForEmptyReplay(t *testing.T) {
	setupSnapshotInputSessionTest(t)

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
	Snapshot("empty-replay", nil)
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if got.InputTick == nil || *got.InputTick != 0 {
		t.Fatalf("empty replay capture input tick = %v, want 0", got.InputTick)
	}
}

func TestInputSessionCaptureKeyRequestsSnapshots(t *testing.T) {
	setupSnapshotInputSessionTest(t)

	var got []engine.CaptureRequest
	engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
		got = append(got, req)
		return nil
	})
	if _, err := PrepareInputRecording(30, InputSessionOptions{CaptureKey: KeyP}); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	session := claimPreparedSession(t, game)
	recordedTick, err := consumeInputTick(
		InputReplayState{KeysDown: []int64{int64(KeyP)}},
		[]InputReplayKeyEvent{{Key: int64(KeyP), Pressed: true}},
		1.0/30.0,
	)
	if err != nil {
		t.Fatal(err)
	}
	session.captureConfiguredKeyPresses(recordedTick.frame.KeyEvents)
	replay, err := FinishInputRecording()
	if err != nil {
		t.Fatal(err)
	}

	game.abortInputSession("recording ended")
	game.resetBootstrap()
	if _, err := PrepareInputReplay(replay, InputSessionOptions{CaptureKey: KeyP}); err != nil {
		t.Fatal(err)
	}
	session = claimPreparedSession(t, game)
	replayedTick, err := consumeInputTick(InputReplayState{}, nil, 1.0/30.0)
	if err != nil {
		t.Fatal(err)
	}
	session.captureConfiguredKeyPresses(replayedTick.frame.KeyEvents)
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("configured capture requests = %+v", got)
	}
	for i, request := range got {
		if request.InputTick == nil || *request.InputTick != 0 {
			t.Fatalf("configured capture %d input tick = %v, want 0", i, request.InputTick)
		}
	}
}

func TestSnapshotDoesNotInheritInputTickAcrossGameReset(t *testing.T) {
	setupSnapshotInputSessionTest(t)

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
	Snapshot("old-game", nil)
	game.abortInputSession("game reset")
	Snapshot("aborted-game", nil)
	game.resetBootstrap()
	if _, err := PrepareInputReplay(validRuntimeReplay()); err != nil {
		t.Fatal(err)
	}
	claimPreparedSession(t, game)
	Snapshot("new-game-before-input", nil)
	if _, err := consumeInputTick(InputReplayState{}, nil, 1.0/30.0); err != nil {
		t.Fatal(err)
	}
	Snapshot("new-game-first-input", nil)
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
	setupSnapshotInputSessionTest(t)
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
	Snapshot("final", nil)
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
