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

	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
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

func TestOnEngineRenderFlushesReadyClonePublications(t *testing.T) {
	enginewrap.Init(func(call func()) { call() })
	originalSpriteMgr := pkgengine.SpriteMgr
	spriteMgr := &captureFlushSpriteMgr{}
	pkgengine.SpriteMgr = spriteMgr
	t.Cleanup(func() { pkgengine.SpriteMgr = originalSpriteMgr })

	var game Game
	game.lifecycleState.IsRunned.Store(true)
	game.camera = &cameraImpl{g: &game}
	game.initShapeMgr()
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	destroyed := &SpriteImpl{}
	destroyed.runtimeState.SyncSprite = &engine.Sprite{}
	game.shapeMgr.remove(destroyed)
	game.shapeMgr.markCloneProxyPublicationReady()

	game.OnEngineRender(0)

	if destroyed.runtimeState.SyncSprite != nil {
		t.Fatal("ready clone publication did not trigger the post-coroutine proxy flush")
	}
	if len(spriteMgr.batches) != 1 {
		t.Fatalf("ready clone publication batches = %d, want 1", len(spriteMgr.batches))
	}
}

func TestOnEngineRenderFlushesSpriteProxiesWhenCaptureIsPending(t *testing.T) {
	enginewrap.Init(func(call func()) { call() })
	originalSpriteMgr := pkgengine.SpriteMgr
	spriteMgr := &captureFlushSpriteMgr{}
	pkgengine.SpriteMgr = spriteMgr
	t.Cleanup(func() { pkgengine.SpriteMgr = originalSpriteMgr })

	var game Game
	game.lifecycleState.IsRunned.Store(true)
	game.camera = &cameraImpl{g: &game}
	game.initShapeMgr()
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)

	shape := &captureFlushTestShape{}
	game.addShape(shape)
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
		t.Fatalf("shape updates without capture = %d, want 0", shape.updates)
	}
	if destroyed.runtimeState.SyncSprite == nil {
		t.Fatal("pending sprite destroy flushed without capture")
	}

	if err := engine.EnqueueCapture("after-coroutine"); err != nil {
		t.Fatal(err)
	}
	game.OnEngineRender(0)
	if shape.updates != 0 {
		t.Fatalf("capture-only proxy flush advanced shape logic %d times, want 0", shape.updates)
	}
	if destroyed.runtimeState.SyncSprite != nil {
		t.Fatal("pending sprite destroy was not included in capture-only proxy flush")
	}
	if len(spriteMgr.batches) != 1 {
		t.Fatalf("capture-only proxy batches = %d, want 1", len(spriteMgr.batches))
	}
	if err := engine.FlushCaptures(); err != nil {
		t.Fatal(err)
	}
}

func TestOnEngineRenderFlushesSpriteProxiesBeforeReplayEOFPause(t *testing.T) {
	resetInputSessionState()
	engine.SetGame(nil)
	t.Cleanup(resetInputSessionState)
	enginewrap.Init(func(call func()) { call() })
	originalSpriteMgr := pkgengine.SpriteMgr
	spriteMgr := &captureFlushSpriteMgr{}
	pkgengine.SpriteMgr = spriteMgr
	t.Cleanup(func() { pkgengine.SpriteMgr = originalSpriteMgr })

	var game Game
	game.camera = &cameraImpl{g: &game}
	game.initShapeMgr()
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	shape := &captureFlushTestShape{}
	game.addShape(shape)
	destroyed := &SpriteImpl{}
	destroyed.runtimeState.SyncSprite = &engine.Sprite{}
	game.shapeMgr.remove(destroyed)
	if _, err := PrepareInputReplay(validRuntimeReplay(InputReplayFrame{Frame: 0, State: InputReplayState{}})); err != nil {
		t.Fatal(err)
	}

	engine.SetGame(&game)
	defer engine.SetGame(nil)
	if err := game.attachPreparedInputSession(); err != nil {
		t.Fatal(err)
	}
	session := game.currentInputSession()
	if _, err := session.consumeSampledInputTick(1, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
		return InputReplayState{}, nil, nil
	}); err != nil {
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
