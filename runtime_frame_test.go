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
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

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

func TestAtFrameSchedulesCallbackForActiveGame(t *testing.T) {
	co := setupRuntimeScheduler(t)

	ran := false
	engine.SetGame(struct{}{})
	defer engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	base := engine.CurrentFrame()

	AtFrame(base+1, func() {
		ran = true
	})
	if ran {
		t.Fatal("AtFrame ran callback before target frame")
	}

	itime.Update(0, 0)
	engine.RunFrameCallbacks()
	co.Update()

	if !ran {
		t.Fatal("AtFrame did not run callback at target frame")
	}
}

func TestSnapshotUsesConfiguredHandlerAfterBody(t *testing.T) {
	var got []string
	engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
		got = append(got, "capture:"+req.Name)
		return nil
	})
	defer engine.SetCaptureHandler(nil)

	Snapshot("step_001.png", func() error {
		got = append(got, "body")
		return nil
	})

	want := []string{"body", "capture:step_001.png"}
	if len(got) != len(want) {
		t.Fatalf("Snapshot order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Snapshot order = %v, want %v", got, want)
		}
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
	if _, err := consumeInputTick(InputReplayState{}, nil, 1.0/30.0); err != nil {
		t.Fatal(err)
	}
	Snapshot("old-game", nil)
	game.abortInputSession("game reset")
	Snapshot("new-game", nil)
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 || got[0].InputTick == nil || *got[0].InputTick != 0 {
		t.Fatalf("old-game capture = %+v", got)
	}
	if got[1].InputTick != nil {
		t.Fatalf("new Game inherited input tick %d", *got[1].InputTick)
	}
}

func TestSnapshotQueuesRequestForActiveGame(t *testing.T) {
	var got []string
	engine.SetGame(struct{}{})
	defer engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
		got = append(got, req.Name)
		return nil
	})
	defer engine.SetCaptureHandler(nil)

	Snapshot("step_001.png", nil)
	if len(got) != 0 {
		t.Fatalf("Snapshot ran immediately: %v", got)
	}

	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "step_001.png" {
		t.Fatalf("FlushCaptures = %v, want [step_001.png]", got)
	}
}

func TestSnapshotBodyMayYieldInsideFrameCallback(t *testing.T) {
	co := setupRuntimeScheduler(t)

	engine.SetGame(struct{}{})
	defer engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	base := itime.Frame()

	bodyStarted := false
	bodyCompleted := false
	var captured []engine.CaptureRequest
	engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
		captured = append(captured, req)
		return nil
	})
	defer engine.SetCaptureHandler(nil)

	AtFrame(base+1, func() {
		Snapshot("yielded.png", func() error {
			bodyStarted = true
			engine.WaitYield()
			bodyCompleted = true
			return nil
		})
	})
	itime.Update(0, 0)
	engine.RunFrameCallbacks()

	co.Update()
	if !bodyStarted {
		t.Fatal("capture body did not start on its target frame")
	}
	if !bodyCompleted {
		t.Fatal("capture body did not resume from yield during scheduler update")
	}
	if !engine.HasPendingCaptures() {
		t.Fatal("capture was not queued after its body completed")
	}
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(captured) != 1 || captured[0].Name != "yielded.png" || captured[0].Frame != base+1 {
		t.Fatalf("captured requests = %+v, want yielded.png at frame %d", captured, base+1)
	}
}

func TestAtFrameCallbackCanWaitForMainThread(t *testing.T) {
	co := setupRuntimeScheduler(t)

	engine.SetGame(struct{}{})
	defer engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	base := itime.Frame()

	mainThreadCallRan := false
	AtFrame(base+1, func() {
		engine.WaitMainThread(func() {
			mainThreadCallRan = true
		})
	})
	itime.Update(0, 0)
	engine.RunFrameCallbacks()

	updateDone := make(chan struct{})
	go func() {
		co.Update()
		close(updateDone)
	}()
	select {
	case <-updateDone:
	case <-time.After(time.Second):
		t.Fatal("AtFrame callback deadlocked while waiting for the main thread")
	}
	if !mainThreadCallRan {
		t.Fatal("AtFrame callback did not complete its main-thread call")
	}
}
