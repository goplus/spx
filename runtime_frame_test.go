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
	"slices"
	"testing"
	"time"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
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

func TestLogicFramePreservesPhasesAndShapeSnapshot(t *testing.T) {
	setupRuntimeScheduler(t)
	itime.OnReload()
	itime.Start(nil)
	t.Cleanup(itime.OnReload)
	game := &Game{events: make(chan event, 8), sounds: make(map[string]sound)}
	backend := &animationAudioBackend{}
	game.soundMgr.Init(backend)

	newSprite := func(name string) *SpriteImpl {
		anim := newTestAnimationComponent()
		sprite := anim.sprite
		sprite.name, sprite.g = name, game
		game.sounds[name] = &coreproject.SoundConfig{Path: name + ".wav"}
		sprite.sound().pendingAudios = []string{name}
		anim.curAnimState = &animState{Name: name}
		anim.doneAnimations = []string{name}
		return sprite
	}
	first, second, late := newSprite("first"), newSprite("second"), newSprite("late")
	destroyed, unbound := newSprite("destroyed"), newSprite("unbound")
	destroyed.markDestroyed()
	unbound.runtimeState.SyncSprite = nil
	game.shapeMgr.items = []Shape{first, struct{}{}, destroyed, second, unbound}
	var steps []string
	backend.onPlay = func(id int64) {
		path := backend.plays[len(backend.plays)-1].path
		steps = append(steps, "play:"+path)
		if len(game.events) != 0 {
			t.Error("timer dispatched before audio completed")
		}
		switch id {
		case 1:
			first.animation().curAnimState.OnPlayAudioPlaybackID = id
			// Both passes must finish the original snapshot, even if playback
			// changes the live shape list.
			game.shapeMgr.items = []Shape{late}
		case 2:
			if first.animation().curAnimState == nil {
				t.Error("first animation completed before second audio played")
			}
			second.animation().curAnimState.OnPlayAudioPlaybackID = id
		case 3:
			late.animation().curAnimState.OnPlayAudioPlaybackID = id
		}
	}
	backend.onStop = func(id int64) {
		steps = append(steps, fmt.Sprintf("complete:%d", id))
		if len(game.events) != 0 {
			t.Error("timer dispatched before animation completion")
		}
	}
	itime.RegisterTimer(0.2)
	itime.RegisterTimer(0.1)
	itime.RegisterTimer(0.3)
	itime.Update(0.2, 30)
	audios, animations := make([]string, 0, 4), make([]string, 0, 4)
	audioStorage, animationStorage := &audios[:cap(audios)][0], &animations[:cap(animations)][0]
	audios, animations = game.processLogicFrame(audios, animations)
	want := []string{"play:" + engine.ToAssetPath("first.wav"), "play:" + engine.ToAssetPath("second.wav"), "complete:1", "complete:2"}
	if !slices.Equal(steps, want) {
		t.Fatalf("logic phases = %v, want %v", steps, want)
	}
	if first.animation().curAnimState != nil || second.animation().curAnimState != nil {
		t.Fatal("original snapshot did not complete both animations")
	}
	if len(late.sound().pendingAudios) != 1 || len(late.animation().doneAnimations) != 1 {
		t.Fatal("newly added sprite was processed in the old snapshot")
	}
	for _, sprite := range []*SpriteImpl{destroyed, unbound} {
		if len(sprite.sound().pendingAudios) != 0 || len(sprite.animation().doneAnimations) != 0 || sprite.animation().curAnimState == nil {
			t.Fatalf("inactive sprite %s was not drained without playback", sprite.name)
		}
	}
	if len(game.events) != 2 {
		t.Fatalf("queued timers = %d, want 2", len(game.events))
	}
	for _, timestamp := range []int64{100, 200} {
		if timer, ok := (<-game.events).(*eventTimer); !ok || timer.Timestamp != timestamp {
			t.Fatalf("timer = %+v, want timestamp %d", timer, timestamp)
		}
	}
	if _, ok := itime.NextTimer(); ok {
		t.Fatal("future timer became due early")
	}
	steps = nil
	audios, animations = game.processLogicFrame(audios, animations)
	want = []string{"play:" + engine.ToAssetPath("late.wav"), "complete:3"}
	if !slices.Equal(steps, want) || len(game.events) != 0 {
		t.Fatalf("next frame = %v, timers=%d, want %v without repeated timers", steps, len(game.events), want)
	}
	if len(audios) != 0 || cap(audios) != 4 || &audios[:cap(audios)][0] != audioStorage ||
		len(animations) != 0 || cap(animations) != 4 || &animations[:cap(animations)][0] != animationStorage {
		t.Fatal("logic frames did not preserve both reusable scratch buffers")
	}
}

func TestLogicFramePreservesEmptyScratch(t *testing.T) {
	itime.OnReload()
	t.Cleanup(itime.OnReload)
	for _, items := range [][]Shape{nil, {struct{}{}}} {
		game := &Game{}
		game.shapeMgr.items = items
		audios, animations := game.processLogicFrame([]string{}, []string{})
		if audios == nil || animations == nil || len(audios) != 0 || len(animations) != 0 {
			t.Fatalf("scratch = %#v, %#v, want non-nil empty slices", audios, animations)
		}
		audios, animations = game.processLogicFrame(nil, nil)
		if audios != nil || animations != nil {
			t.Fatalf("nil scratch = %#v, %#v, want nil slices", audios, animations)
		}
	}
}
