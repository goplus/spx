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

	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

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
	game.resetBootstrapState()

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
	game.resetBootstrapState()

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

func TestInputSessionEnvironmentLivesUntilGameEnds(t *testing.T) {
	resetInputSessionTest(t)
	itime.SetFixedDeltaTime(1.0 / 60)
	if _, err := PrepareInputRecording(24); err != nil {
		t.Fatal(err)
	}
	if got, _ := itime.FixedDeltaTime(); got != 1.0/60 {
		t.Fatalf("prepared session changed fixed timestep to %v", got)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	if got, ok := itime.FixedDeltaTime(); !ok || got != 1.0/24 {
		t.Fatalf("recording fixed timestep = (%v, %v), want (%v, true)", got, ok, 1.0/24.0)
	}
	replay, err := FinishInputRecording()
	if err != nil {
		t.Fatal(err)
	}
	if replay.FixedTimestep != 1.0/24 {
		t.Fatalf("recorded fixed timestep = %v, want %v", replay.FixedTimestep, 1.0/24.0)
	}
	if got, _ := itime.FixedDeltaTime(); got != 1.0/24 {
		t.Fatalf("completed session released timing before Game ended: %v", got)
	}
	game.abortInputSession("game ended")
	if got, ok := itime.FixedDeltaTime(); !ok || got != 1.0/60 {
		t.Fatalf("restored fixed timestep = (%v, %v), want (%v, true)", got, ok, 1.0/60.0)
	}
}

func TestInputRecordingUsesDefaultFixedTimestep(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	if got, ok := itime.FixedDeltaTime(); !ok || got != 1.0/itime.DefaultFPS {
		t.Fatalf("recording fixed timestep = (%v, %v), want (%v, true)", got, ok, 1.0/itime.DefaultFPS)
	}
	replay, err := FinishInputRecording()
	if err != nil {
		t.Fatal(err)
	}
	if replay.FixedTimestep != 1.0/itime.DefaultFPS {
		t.Fatalf("recorded fixed timestep = %v", replay.FixedTimestep)
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

func TestInputReplayUsesRecordedFixedTimestepForGame(t *testing.T) {
	resetInputSessionTest(t)
	itime.SetFixedDeltaTime(1.0 / 60)
	replay := validRuntimeReplay()
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	if got, ok := itime.FixedDeltaTime(); !ok || got != 1.0/30 {
		t.Fatalf("replay fixed timestep = (%v, %v), want (%v, true)", got, ok, 1.0/30.0)
	}
	if status := GetInputSessionStatus(); status.Mode != InputSessionModeReplaying || status.Phase != InputSessionPhaseRunning {
		t.Fatalf("replay session status = %+v", status)
	}
	game.abortInputSession("game ended")
	if got, ok := itime.FixedDeltaTime(); !ok || got != 1.0/60 {
		t.Fatalf("restored fixed timestep = (%v, %v)", got, ok)
	}
}

func TestVariableTimestepReplayOverridesAndRestoresFixedTime(t *testing.T) {
	resetInputSessionTest(t)
	itime.SetFixedDeltaTime(1.0 / 60)
	replay := validRuntimeReplay()
	replay.FixedTimestep = 0
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	claimPreparedSession(t, game)
	if _, fixed := itime.FixedDeltaTime(); fixed {
		t.Fatal("variable-timestep replay inherited the previous fixed timestep")
	}
	game.abortInputSession("game ended")
	if got, fixed := itime.FixedDeltaTime(); !fixed || got != 1.0/60 {
		t.Fatalf("restored fixed timestep = (%v, %v), want (%v, true)", got, fixed, 1.0/60.0)
	}
}

func TestPreparedInputSessionIsConsumedOncePerGameGeneration(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputRecording(30); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	first := claimPreparedSession(t, game)
	if first.generation != game.currentBootstrapGeneration() {
		t.Fatalf("session generation = %d, want %d", first.generation, game.currentBootstrapGeneration())
	}
	game.abortInputSession("generation ended")
	game.resetBootstrapState()
	if err := game.attachPreparedInputSession(); err != nil {
		t.Fatal(err)
	}
	if game.currentInputSession() != nil {
		t.Fatal("one-shot session descriptor was consumed twice")
	}
	game.abortInputSession("ordinary generation ended")
	game.resetBootstrapState()

	if _, err := PrepareInputReplay(validRuntimeReplay()); err != nil {
		t.Fatal(err)
	}
	second := claimPreparedSession(t, game)
	if second == first {
		t.Fatal("new Game generation reused the previous input session")
	}
	if second.generation != game.currentBootstrapGeneration() {
		t.Fatalf("second generation = %d, want current generation %d", second.generation, game.currentBootstrapGeneration())
	}
	if status := second.status(); status.NextFrame != 0 {
		t.Fatalf("new session did not start at tick zero: %+v", status)
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
	if status := session.status(); status.Phase != InputSessionPhaseAborted || status.Completed || status.Error != "game reset" {
		t.Fatalf("aborted session status = %+v", status)
	}
	if status := GetInputSessionStatus(); status.Phase != InputSessionPhaseAborted || status.Error != "game reset" || status.NextFrame != 1 || status.FrameCount != 1 {
		t.Fatalf("public aborted session status = %+v", status)
	}
	if _, err := session.finishRecording(nil); err == nil {
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
	if _, err := session.finishRecording(func() {
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
	resetInputSessionTest(t)
	replay := validRuntimeReplay(InputReplayFrame{Frame: 0, State: InputReplayState{Mouse: InputReplayMouse{X: 2}}})
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	session := claimPreparedSession(t, game)
	if _, err := consumeInputTick(InputReplayState{}, nil, 1); err != nil {
		t.Fatal(err)
	}
	if status := session.status(); status.Phase != InputSessionPhaseFinishing || status.Completed || !status.Exhausted {
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
}

func TestEmptyReplayCompletesAfterFirstFrameEnd(t *testing.T) {
	resetInputSessionTest(t)
	if _, err := PrepareInputReplay(validRuntimeReplay()); err != nil {
		t.Fatal(err)
	}
	game := &Game{}
	session := claimPreparedSession(t, game)
	if status := session.status(); status.Completed || status.Exhausted {
		t.Fatalf("empty replay completed before tick zero: %+v", status)
	}
	first, err := consumeInputTick(InputReplayState{Mouse: InputReplayMouse{X: 99}}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !first.firstTick || first.frame.Frame != -1 {
		t.Fatalf("empty replay first tick = %+v", first)
	}
	if status := session.status(); status.Completed || !status.Exhausted || status.Phase != InputSessionPhaseFinishing ||
		!status.HasCurrentTick || status.CurrentTick != 0 {
		t.Fatalf("empty replay before frame end = %+v", status)
	}
	completed, err := session.completeReplayFrame(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !completed || !session.status().Completed {
		t.Fatal("empty replay did not complete at its first frame end")
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
		replay, err := session.finishRecording(nil)
		if err == nil && len(replay.Frames) != 1 {
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

func TestReplayCompletionWaitsForCaptureDispatch(t *testing.T) {
	resetInputSessionTest(t)
	engine.ResetFrameRuntime()
	t.Cleanup(func() {
		engine.ResetFrameRuntime()
		engine.SetCaptureHandler(nil)
	})
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
