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
	"github.com/goplus/spx/v3/internal/enginewrap"
	itime "github.com/goplus/spx/v3/internal/time"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type captureFlushTestShape struct {
	updates int
}

func (s *captureFlushTestShape) onUpdate(float64) {
	s.updates++
}

type captureFlushSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	batches [][]float32
}

func setupCaptureFlushSpriteMgr(t *testing.T) *captureFlushSpriteMgr {
	t.Helper()
	enginewrap.Init(func(call func()) { call() })
	original := pkgengine.SpriteMgr
	mgr := &captureFlushSpriteMgr{}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() { pkgengine.SpriteMgr = original })
	return mgr
}

type replayPauseTestExtMgr struct {
	pauses int
}

func (*replayPauseTestExtMgr) RequestExit(int64)        {}
func (*replayPauseTestExtMgr) RequestReset(int64)       {}
func (*replayPauseTestExtMgr) RequestRestart()          {}
func (*replayPauseTestExtMgr) OnRuntimePanic(string)    {}
func (m *replayPauseTestExtMgr) Pause()                 { m.pauses++ }
func (*replayPauseTestExtMgr) Resume()                  {}
func (*replayPauseTestExtMgr) IsPaused() bool           { return false }
func (*replayPauseTestExtMgr) NextFrame()               {}
func (*replayPauseTestExtMgr) SetLayerSorterMode(int64) {}

func (m *captureFlushSpriteMgr) BatchUpdateTransforms(buffer []float32) {
	m.batches = append(m.batches, append([]float32(nil), buffer...))
}

func TestOnEngineRenderFlushesWithoutClonePublicationSignal(t *testing.T) {
	spriteMgr := setupCaptureFlushSpriteMgr(t)

	var game Game
	game.lifecycleState.IsRunned.Store(true)
	game.camera = &cameraImpl{g: &game}
	game.initShapeMgr()
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	destroyed := &SpriteImpl{}
	destroyed.runtimeState.SyncSprite = &engine.Sprite{}
	game.shapeMgr.remove(destroyed)

	game.OnEngineRender(0)

	if destroyed.runtimeState.SyncSprite != nil {
		t.Fatal("post-coroutine proxy flush required a clone publication signal")
	}
	if len(spriteMgr.batches) != 1 {
		t.Fatalf("proxy batches without a clone publication signal = %d, want 1", len(spriteMgr.batches))
	}
}

func TestOnEngineRenderFlushesSpriteProxiesEveryFrame(t *testing.T) {
	spriteMgr := setupCaptureFlushSpriteMgr(t)

	var game Game
	game.lifecycleState.IsRunned.Store(true)
	game.camera = &cameraImpl{g: &game}
	game.initShapeMgr()
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)

	shape := &captureFlushTestShape{}
	game.shapeMgr.add(shape)
	destroyed := &SpriteImpl{}
	destroyed.runtimeState.SyncSprite = &engine.Sprite{}
	game.shapeMgr.remove(destroyed)

	engine.SetGame(&game)
	defer engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()
	engine.SetCaptureHandler(func(engine.CaptureRequest) error { return nil })
	defer engine.SetCaptureHandler(nil)

	game.OnEngineRender(0)
	if shape.updates != 0 {
		t.Fatalf("post-coroutine proxy flush advanced shape logic %d times, want 0", shape.updates)
	}
	if destroyed.runtimeState.SyncSprite != nil {
		t.Fatal("pending sprite destroy was not flushed on an ordinary frame")
	}
	if len(spriteMgr.batches) != 1 {
		t.Fatalf("ordinary-frame proxy batches = %d, want 1", len(spriteMgr.batches))
	}

	destroyedWithCapture := &SpriteImpl{}
	destroyedWithCapture.runtimeState.SyncSprite = &engine.Sprite{}
	game.shapeMgr.remove(destroyedWithCapture)
	if err := engine.EnqueueCapture("after-coroutine"); err != nil {
		t.Fatal(err)
	}
	game.OnEngineRender(0)
	if shape.updates != 0 {
		t.Fatalf("capture-frame proxy flush advanced shape logic %d times, want 0", shape.updates)
	}
	if destroyedWithCapture.runtimeState.SyncSprite != nil {
		t.Fatal("pending sprite destroy was not included in the capture-frame proxy flush")
	}
	if len(spriteMgr.batches) != 2 {
		t.Fatalf("proxy batches after capture frame = %d, want 2", len(spriteMgr.batches))
	}
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
}

func TestOnEngineRenderFlushesSpriteProxiesBeforeReplayEOFPause(t *testing.T) {
	resetInputSessionTest(t)
	spriteMgr := setupCaptureFlushSpriteMgr(t)

	var game Game
	game.camera = &cameraImpl{g: &game}
	game.initShapeMgr()
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	shape := &captureFlushTestShape{}
	game.shapeMgr.add(shape)
	destroyed := &SpriteImpl{}
	destroyed.runtimeState.SyncSprite = &engine.Sprite{}
	game.shapeMgr.remove(destroyed)
	if _, err := PrepareInputReplay(validRuntimeReplay(InputReplayFrame{Frame: 0, State: InputReplayState{}})); err != nil {
		t.Fatal(err)
	}

	session := claimPreparedSession(t, &game)
	if _, err := consumeInputTick(InputReplayState{}, nil, 1); err != nil {
		t.Fatal(err)
	}
	game.lifecycleState.IsRunned.Store(true)
	game.OnEngineRender(0)

	if shape.updates != 0 {
		t.Fatalf("EOF-only proxy flush advanced shape logic %d times, want 0", shape.updates)
	}
	if destroyed.runtimeState.SyncSprite != nil {
		t.Fatal("pending sprite destroy was not flushed before EOF pause")
	}
	if len(spriteMgr.batches) != 1 {
		t.Fatalf("EOF-only proxy batches = %d, want 1", len(spriteMgr.batches))
	}

	originalExtMgr := pkgengine.ExtMgr
	extMgr := &replayPauseTestExtMgr{}
	pkgengine.ExtMgr = extMgr
	t.Cleanup(func() { pkgengine.ExtMgr = originalExtMgr })
	game.OnEngineFrameEnd()
	game.OnEngineFrameEnd()
	if extMgr.pauses != 1 {
		t.Fatalf("EOF frame-end pauses = %d, want one", extMgr.pauses)
	}
	if status := session.status(); status.Phase != InputSessionPhaseCompleted || !status.Completed {
		t.Fatalf("EOF session status = %+v", status)
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
