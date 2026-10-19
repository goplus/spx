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
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/engine/profiler"
	"github.com/goplus/spx/v3/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type engineStartTestGame struct {
	*Game
	mainCalls int
}

func (g *engineStartTestGame) MainEntry() { g.mainCalls++ }

func TestOnEngineStartSchedulesBootstrapOnce(t *testing.T) {
	for _, reset := range []bool{false, true} {
		t.Run(fmt.Sprintf("reset=%v", reset), func(t *testing.T) {
			previousBounds := cachedBounds
			t.Cleanup(func() { cachedBounds = previousBounds })
			co, game := setupRuntimeEventGame(t)
			gamer := &engineStartTestGame{Game: game}
			game.gamer = gamer
			game.lifecycleState.IsRunned.Store(true)
			bootstrapCalls := 0
			game.queueBootstrap(game.bootstrapGeneration(), func() { bootstrapCalls++ })

			// Hold execution until both host callbacks and any reset finish.
			co.RunBetweenScripts(func() {
				game.OnEngineStart()
				game.OnEngineStart()
				if reset {
					game.resetBootstrap()
				}
			})
			co.Update()

			wantCalls := 1
			if reset {
				wantCalls = 0
			}
			if gamer.mainCalls != wantCalls || bootstrapCalls != wantCalls {
				t.Fatalf("MainEntry calls = %d, bootstrap calls = %d; want %d each", gamer.mainCalls, bootstrapCalls, wantCalls)
			}
			if game.lifecycleState.IsRunned.Load() != !reset || game.lifecycleState.BootstrapDone.Load() != !reset {
				t.Fatalf("running = %v, bootstrap done = %v; want %v", game.lifecycleState.IsRunned.Load(), game.lifecycleState.BootstrapDone.Load(), !reset)
			}
			if game.lifecycleState.StartDispatched.Load() {
				t.Fatal("bootstrap dispatched OnStart before the first frame")
			}
			threads := co.LastThreadID()
			game.OnEngineStart()
			co.Update()
			if co.LastThreadID() != threads {
				t.Fatal("repeated OnEngineStart scheduled more work")
			}
		})
	}
}

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
	setupImmediateEngineCalls(t)
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

type debugPanelUiMgr struct {
	pkgengine.IUiMgr
	text      string
	destroyed []pkgengine.Object
}

func (m *debugPanelUiMgr) SetText(_ pkgengine.Object, text string) { m.text = text }
func (*debugPanelUiMgr) SetScale(pkgengine.Object, mathf.Vec2)     {}
func (m *debugPanelUiMgr) DestroyNode(id pkgengine.Object) bool {
	m.destroyed = append(m.destroyed, id)
	return m.IUiMgr.DestroyNode(id)
}

func TestDebugPanelShowsLatestCoroutineStats(t *testing.T) {
	installBubbleTestUiMgr(t)
	mgr := &debugPanelUiMgr{IUiMgr: pkgengine.UiMgr}
	pkgengine.UiMgr = mgr
	previousDebug := profiler.Debug
	t.Cleanup(func() { profiler.Debug = previousDebug })
	co := setupRuntimeScheduler(t)
	thread := co.Create("debug", func(me coroutine.Thread) {
		co.WaitYield(me)
		co.WaitNextFrameFor(me)
	})
	co.JoinFirstSlice(thread)
	var game Game
	game.debugState.Debug = true
	for range 2 {
		co.Update()
		game.showDebugPanel()
		panel := game.debugState.DebugPanel
		game.showDebugPanel()
		if panel == nil || game.debugState.DebugPanel != panel || !profiler.Debug {
			t.Fatal("debug panel was not created and reused")
		}
		stats := co.UpdateStats()
		for _, want := range []string{
			fmt.Sprintf("coro: MoveTime: %.2f\n", stats.MoveTime),
			fmt.Sprintf("coro: WaitTime: %.2f\n", stats.WaitTime),
			fmt.Sprintf("coro: TaskProcessing: %.2f\n", stats.JobTime),
			fmt.Sprintf("coro: TaskCounts: %d\n", stats.JobCount),
			fmt.Sprintf("coro: WaitFrameCount: %d\n", stats.FrameWaitCount),
			fmt.Sprintf("coro: WaitMainCount: %d\n", stats.MainThreadCount),
			fmt.Sprintf("coro: NextCount: %d\n", stats.DeferredCount),
		} {
			if !strings.Contains(mgr.text, want) {
				t.Errorf("debug panel missing %q: %s", want, mgr.text)
			}
		}
	}
	id := game.debugState.DebugPanel.GetId()
	game.debugState.Debug = false
	game.showDebugPanel()
	game.showDebugPanel()
	if profiler.Debug || game.debugState.DebugPanel != nil || !slices.Equal(mgr.destroyed, []pkgengine.Object{id}) {
		t.Fatal("disabling debug did not destroy the panel exactly once")
	}
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
	engine.ResetCaptures()
	defer engine.ResetCaptures()
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
	if err := engine.EnqueueCapture("after-coroutine", nil); err != nil {
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

func TestEngineResetClearsBootstrapAndAllowsStartupAgain(t *testing.T) {
	installBubbleTestUiMgr(t)
	game := setupStageLoadTestGame(t, gameTestFS{}).baseGame()
	// Reset owns both registries, even when prior tests left native proxies.
	pkgengine.CreateEmptySprite[engine.Sprite](mathf.Vec2{})
	pkgengine.CreateUI[pkgengine.UiNode]("reset")
	game.shapeMgr.add(&struct{}{})
	game.lifecycleState.IsRunned.Store(true)
	game.lifecycleState.BootstrapDone.Store(true)
	game.lifecycleState.StartDispatched.Store(true)
	generation := game.bootstrapGeneration()
	calls := 0
	game.lifecycleState.RunOnce.Do(func() { calls++ })
	game.OnEnginePause(true)
	game.OnEnginePause(false)
	if !game.lifecycleState.IsRunned.Load() || game.bootstrapGeneration() != generation {
		t.Fatal("pause callback changed the managed lifecycle")
	}
	game.OnEngineReset()
	if game.bootstrapGeneration() != generation+1 || game.lifecycleState.IsRunned.Load() || game.lifecycleState.BootstrapDone.Load() || game.lifecycleState.StartDispatched.Load() || game.shapeMgr.count() != 0 {
		t.Fatal("engine reset left startup state or shapes from the prior generation")
	}
	if len(pkgengine.Sprites()) != 0 || len(pkgengine.UiNodes()) != 0 {
		t.Fatal("engine reset left registered sprite or UI proxies")
	}
	game.lifecycleState.RunOnce.Do(func() { calls++ })
	if calls != 2 {
		t.Fatalf("startup calls after reset=%d,want2", calls)
	}
}

func TestScheduleStartEventRejectsUnreadyAndDuplicateDispatch(t *testing.T) {
	for _, tt := range []struct {
		name                        string
		done, dispatched, scheduled bool
	}{
		{"bootstrap incomplete", false, false, false},
		{"already dispatched", true, true, false},
		{"already scheduled", true, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game := &Game{}
			game.startScheduled = tt.scheduled
			game.lifecycleState.BootstrapDone.Store(tt.done)
			game.lifecycleState.StartDispatched.Store(tt.dispatched)
			if event := game.scheduleStartEvent(); event != nil || game.startScheduled != tt.scheduled {
				t.Fatalf("invalid start scheduling returned %v and changed flag to %v", event, game.startScheduled)
			}
		})
	}
	game := &Game{}
	game.bootstrapGen = 7
	game.lifecycleState.BootstrapDone.Store(true)
	if event := game.scheduleStartEvent(); event == nil || event.generation != 7 {
		t.Fatalf("ready start event=%v,want generation7", event)
	}
	if game.scheduleStartEvent() != nil {
		t.Fatal("same bootstrap scheduled OnStart twice")
	}
}
