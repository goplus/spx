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
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/profiler"
	"github.com/goplus/spx/v4/internal/enginewrap"
	itime "github.com/goplus/spx/v4/internal/time"
	"github.com/goplus/spx/v4/internal/ui"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
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
	game.shapeMgr.add(destroyed)
	game.shapeMgr.removeShape(destroyed)

	engine.SetGame(&game)
	defer engine.SetGame(nil)
	setupCaptureTest(t)
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
	game.shapeMgr.add(destroyedWithCapture)
	game.shapeMgr.removeShape(destroyedWithCapture)
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
	game.shapeMgr.add(destroyed)
	game.shapeMgr.removeShape(destroyed)
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

// The backend advances independently of the Go sprite, as Godot physics does.
type frameSyncSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	positions map[int64]mathf.Vec2
	batches   [][]float32
}

func (m *frameSyncSpriteMgr) BatchRetrievePositions(ids []int64, out []float32) bool {
	for i, id := range ids {
		pos := m.positions[id]
		out[i*2], out[i*2+1] = float32(pos.X), float32(pos.Y)
	}
	return true
}

func (m *frameSyncSpriteMgr) SetPosition(id pkgengine.Object, pos mathf.Vec2) {
	m.positions[int64(id)] = pos
}

func (*frameSyncSpriteMgr) SetZIndex(pkgengine.Object, int64) {}

func (m *frameSyncSpriteMgr) BatchUpdateTransforms(data []float32) {
	m.batches = append(m.batches, append([]float32(nil), data...))
	for i := 0; i < int(data[0]); i++ {
		at := 2 + i*engine.SyncFieldsPerSprite
		m.positions[int64(data[at])] = mathf.NewVec2(float64(data[at+1]), float64(data[at+2]))
	}
}

type frameUpdateFunc func(float64)

func (update frameUpdateFunc) onUpdate(delta float64) { update(delta) }

func TestFrameSyncUpdatesCustomShapesAndMonitors(t *testing.T) {
	_, game := setupRuntimeEventGame(t)
	calls, delta := 0, 0.0
	updater := frameUpdateFunc(func(dt float64) { calls++; delta = dt })
	panel := &monitorPanelSpy{}
	monitor := &Monitor{visible: true, isDirty: true, panel: panel, eval: func() ui.MonitorValue { return ui.MonitorValue{Text: "42"} }}
	game.shapeMgr.items = []Shape{&SpriteImpl{}, &textBubble{}, &quoterBubble{}, monitor, updater, &struct{}{}}
	withEventRegistrationBarrier(game, game.dispatchShapeUpdates)
	if calls != 1 || delta != itime.DeltaTime() || panel.value.Text != "42" || monitor.isDirty {
		t.Fatalf("shape update calls=%d delta=%v monitor=%+v", calls, delta, panel)
	}
}

func setupFrameSyncGame(t *testing.T) (*coroutine.Coroutines, *Game, *frameSyncSpriteMgr) {
	t.Helper()
	co, game, _ := setupInputSourceTest(t)
	original := pkgengine.SpriteMgr
	mgr := &frameSyncSpriteMgr{positions: make(map[int64]mathf.Vec2)}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() { pkgengine.SpriteMgr = original })
	game.initShapeMgr()
	game.camera = &cameraImpl{g: game}
	game.syncBuffer = engine.NewSpriteSyncBuffer(2)
	game.lifecycleState.IsRunned.Store(true)
	game.lifecycleState.StartDispatched.Store(true)
	return co, game, mgr
}

func addFrameSyncSprite(game *Game, id int64, name string) *SpriteImpl {
	sprite := &newCloneAwakeOrderSprite(game, name).SpriteImpl
	sprite.runtimeState.SyncSprite = &engine.Sprite{Target: sprite, MirrorPosition: sprite.mirrorPosition}
	sprite.runtimeState.SyncSprite.SetId(id)
	sprite.runtimeState.IsCostumeDirty = false
	sprite.spriteState.IsVisible = true
	sprite.physics().physicsMode = DynamicPhysics
	sprite.markProxyDirty()
	game.shapeMgr.add(sprite)
	return sprite
}

func runFrameSyncGame(co *coroutine.Coroutines, game *Game) {
	const delta = 1.0 / 60
	game.OnEngineBeforeUpdate(delta)
	itime.Update(delta, 60)
	game.OnEngineUpdate(delta)
	co.Update()
	game.OnEngineRender(delta)
}

func TestFrameSyncReadsPhysicsBeforeConditionsAndPreservesVisualEdits(t *testing.T) {
	co, game, mgr := setupFrameSyncGame(t)
	sprite := addFrameSyncSprite(game, 1, "moving")
	sprite.transform().setPositionRaw(10, 0)
	game.OnEngineRender(0)
	mgr.batches = nil
	mgr.positions[1] = mathf.NewVec2(20, 30)
	sprite.Hide()

	var sampled mathf.Vec2
	game.OnCond(func() bool {
		sampled = mathf.NewVec2(sprite.Xpos(), sprite.Ypos())
		return false
	}, func() {})
	runFrameSyncGame(co, game)

	if want := mathf.NewVec2(20, 30); sampled != want || mgr.positions[1] != want {
		t.Fatalf("condition=%v backend=%v, want physical position %v", sampled, mgr.positions[1], want)
	}
	if len(mgr.batches) != 1 || mgr.batches[0][10] != 0 {
		t.Fatalf("frame submissions=%v, want one hidden sprite", mgr.batches)
	}
}

func TestFrameSyncCommitsCollisionSliceAndKeepsFutureWait(t *testing.T) {
	co, game, mgr := setupFrameSyncGame(t)
	source := addFrameSyncSprite(game, 1, "source")
	target := addFrameSyncSprite(game, 2, "target")
	game.OnEngineRender(0)
	mgr.batches = nil
	source.addTouchStartHandler(func(Sprite) {
		source.SetXpos(42)
		engine.WaitNextFrame()
		source.SetXpos(84)
	})
	game.triggerEvents = []engine.TriggerEvent{{Src: source.runtimeState.SyncSprite, Dst: target.runtimeState.SyncSprite}}

	runFrameSyncGame(co, game)
	if source.Xpos() != 42 || mgr.positions[1].X != 42 || len(mgr.batches) != 1 {
		t.Fatalf("collision frame: logical=%v backend=%v batches=%d", source.Xpos(), mgr.positions[1], len(mgr.batches))
	}
	runFrameSyncGame(co, game)
	if source.Xpos() != 84 || mgr.positions[1].X != 84 || len(mgr.batches) != 2 {
		t.Fatalf("following frame: logical=%v backend=%v batches=%d", source.Xpos(), mgr.positions[1], len(mgr.batches))
	}
}

func TestFrameSyncRetainsBootstrapContactsUntilFirstActiveFrame(t *testing.T) {
	co, game, mgr := setupFrameSyncGame(t)
	source := addFrameSyncSprite(game, 1, "source")
	target := addFrameSyncSprite(game, 2, "target")
	game.lifecycleState.IsRunned.Store(false)
	game.lifecycleState.StartDispatched.Store(false)
	touches, starts := 0, 0
	source.addTouchStartHandler(func(Sprite) {
		touches++
		source.SetXpos(42)
	})
	game.OnStart(func() {
		starts++
		if len(game.triggerEvents) != 0 {
			t.Error("OnStart ran before bootstrap contacts were registered")
		}
	})
	game.triggerEvents = []engine.TriggerEvent{{Src: source.runtimeState.SyncSprite, Dst: target.runtimeState.SyncSprite}}
	game.OnEngineUpdate(0)
	co.Update()
	game.OnEngineRender(0)
	if touches != 0 || starts != 0 || len(game.triggerEvents) != 1 {
		t.Fatal("incomplete bootstrap consumed contacts or started handlers")
	}
	game.lifecycleState.IsRunned.Store(true)
	game.lifecycleState.BootstrapDone.Store(true)
	runFrameSyncGame(co, game)
	if touches != 1 || starts != 1 || mgr.positions[1].X != 42 {
		t.Fatalf("first active frame: touches=%d starts=%d position=%v", touches, starts, mgr.positions[1])
	}
	runFrameSyncGame(co, game)
	if touches != 1 || starts != 1 {
		t.Fatal("startup events were repeated")
	}
}

func TestFrameSyncShapeCallbacksCanBroadcastBeforeCommit(t *testing.T) {
	co, game, mgr := setupFrameSyncGame(t)
	sprite := addFrameSyncSprite(game, 1, "moving")
	game.OnMsg__1("move", func() { sprite.SetXpos(42) })
	game.shapeMgr.add(frameUpdateFunc(func(float64) {
		game.Broadcast__0("move")
	}))

	runFrameSyncGame(co, game)
	if sprite.Xpos() != 42 || mgr.positions[1].X != 42 || len(mgr.batches) != 1 {
		t.Fatalf("shape callback: logical=%v backend=%v batches=%d", sprite.Xpos(), mgr.positions[1], len(mgr.batches))
	}
}

// A failed frame must release the host before fixture cleanup restores globals.
func runFrameSyncCall(t *testing.T, co *coroutine.Coroutines, call func()) {
	t.Helper()
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		call()
	}()
	select {
	case recovered := <-done:
		if recovered != nil {
			t.Fatalf("frame panicked: %v", recovered)
		}
	case <-time.After(time.Second):
		co.StopAll()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("frame did not exit after stopping its scripts")
		}
		t.Fatal("frame waited for a future-frame continuation")
	}
}

func TestFrameSyncShapeWaitPreservesRegistrationAndCommit(t *testing.T) {
	for _, test := range []struct {
		name   string
		wait   func()
		frames int
	}{
		{"zero", func() { engine.Wait(0) }, 1},
		{"next_frame", func() { engine.WaitNextFrame() }, 1},
		{"duration", func() { engine.Wait(2.0 / 60) }, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			co, game, mgr := setupFrameSyncGame(t)
			source := addFrameSyncSprite(game, 1, "source")
			target := addFrameSyncSprite(game, 2, "target")
			game.lifecycleState.StartDispatched.Store(false)
			game.lifecycleState.BootstrapDone.Store(true)
			started, completed, active, later, keys, touches, starts := 0, 0, 0, 0, 0, 0, 0
			var threads []coroutine.Thread
			game.shapeMgr.add(frameUpdateFunc(func(float64) {
				threads = append(threads, currentScriptThread())
				started++
				active++
				defer func() { active-- }()
				test.wait()
				completed++
			}))
			game.shapeMgr.add(frameUpdateFunc(func(float64) {
				later++
				source.SetYpos(7)
			}))
			game.OnKey__0(KeyA, func() { keys++ })
			source.addTouchStartHandler(func(Sprite) { touches++ })
			game.OnStart(func() { starts++ })
			game.triggerEvents = []engine.TriggerEvent{{Src: source.runtimeState.SyncSprite, Dst: target.runtimeState.SyncSprite}}
			game.OnEngineBeforeUpdate(1.0 / 60)
			game.inputMgr.frame.pending.events = append(game.inputMgr.frame.pending.events, &eventKeyDown{Key: KeyA})
			itime.Update(1.0/60, 60)
			runFrameSyncCall(t, co, func() {
				game.OnEngineUpdate(1.0 / 60)
				co.Update()
				game.OnEngineRender(1.0 / 60)
			})
			if started != 1 || completed != 0 || active != 1 || later != 1 || keys != 1 || touches != 1 || starts != 1 {
				t.Fatalf("first frame: started/completed/active=%d/%d/%d, later/key/touch/start=%d/%d/%d/%d", started, completed, active, later, keys, touches, starts)
			}
			if len(mgr.batches) != 1 || mgr.positions[1].Y != 7 || game.inputMgr.frame.pending != nil || len(game.triggerEvents) != 0 {
				t.Fatal("waiting shape interrupted event registration or frame commit")
			}
			for frame := 1; frame <= test.frames; frame++ {
				runFrameSyncCall(t, co, func() { runFrameSyncGame(co, game) })
				wantCompleted := 0
				if frame == test.frames {
					wantCompleted = 1
				}
				if started != frame+1 || completed != wantCompleted || active != started-completed || later != frame+1 {
					t.Fatalf("frame %d: started/completed/active/later=%d/%d/%d/%d", frame+1, started, completed, active, later)
				}
			}
			runFrameSyncCall(t, co, func() {
				game.Stop(AllOtherScripts)
				co.JoinAll(threads)
				co.Update()
			})
			if active != 0 {
				t.Fatalf("stopped shape updates still active: %d", active)
			}
		})
	}
}

type frameMonitorWaitGame struct {
	*Game
	wait                       func()
	thread                     coroutine.Thread
	started, completed, active int
}

func (g *frameMonitorWaitGame) Value() float64 {
	g.thread = currentScriptThread()
	g.started++
	g.active++
	defer func() { g.active-- }()
	g.wait()
	g.completed++
	return 42
}

func addFrameSyncMonitor(t *testing.T, getter *frameMonitorWaitGame) (*Monitor, *monitorPanelSpy) {
	t.Helper()
	panel := &monitorPanelSpy{}
	monitor := &Monitor{
		visible: true, isDirty: true, panel: panel,
		eval: buildMonitorEval(reflect.ValueOf(getter).Elem(), 1, "value", ui.MonitorAppearanceDefault),
	}
	if monitor.eval == nil {
		t.Fatal("lowercase monitor property did not bind the Value getter")
	}
	getter.shapeMgr.add(monitor)
	return monitor, panel
}

func TestFrameSyncMonitorWaitSkipsBusyAndCleansStoppedGetter(t *testing.T) {
	for _, test := range []struct {
		name string
		wait func(*Game)
	}{
		{"zero", func(g *Game) { g.Wait(0) }},
		{"next_frame", func(*Game) { engine.WaitNextFrame() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			co, game, _ := setupFrameSyncGame(t)
			getter := &frameMonitorWaitGame{Game: game, wait: func() { test.wait(game) }}
			monitor, panel := addFrameSyncMonitor(t, getter)
			for frame := range 2 {
				runFrameSyncCall(t, co, func() { runFrameSyncGame(co, game) })
				if getter.started != 1 || getter.completed != frame || getter.active != 1-frame {
					t.Fatalf("frame %d: getter started/completed/active=%d/%d/%d", frame+1, getter.started, getter.completed, getter.active)
				}
			}
			if panel.value.Text != "42.00" {
				t.Fatalf("resumed getter rendered %q", panel.value.Text)
			}
			monitor.isDirty = true
			getter.wait = func() { engine.Wait(10) }
			runFrameSyncCall(t, co, func() { runFrameSyncGame(co, game) })
			monitor.SetSize(2)
			runFrameSyncCall(t, co, func() { runFrameSyncGame(co, game) })
			if monitor.isDirty || panel.scale != 2 {
				t.Fatal("busy getter blocked the pending UI refresh")
			}
			runFrameSyncCall(t, co, func() {
				game.Stop(AllOtherScripts)
				co.Join(getter.thread)
				co.Update()
			})
			if getter.started != 2 || getter.completed != 1 || getter.active != 0 {
				t.Fatalf("stop: getter started/completed/active=%d/%d/%d", getter.started, getter.completed, getter.active)
			}
			runFrameSyncCall(t, co, func() { runFrameSyncGame(co, game) })
			if getter.started != 3 || getter.active != 1 {
				t.Fatal("canceled getter retained its admission state")
			}
			co.RunAfterStop(nil)
		})
	}
}

func TestFrameSyncMonitorWaitAppliesVisibilityAndLayout(t *testing.T) {
	for _, test := range []struct {
		name   string
		wait   func()
		frames int
	}{
		{"zero", func() { engine.Wait(0) }, 1},
		{"next_frame", func() { engine.WaitNextFrame() }, 1},
		{"duration", func() { engine.Wait(2.5 / 60) }, 3},
	} {
		for _, hide := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/hide=%v", test.name, hide), func(t *testing.T) {
				co, game, _ := setupFrameSyncGame(t)
				defer co.RunAfterStop(nil)
				getter := &frameMonitorWaitGame{Game: game}
				getter.wait = func() {
					if getter.started == 2 {
						test.wait()
					}
				}
				monitor, panel := addFrameSyncMonitor(t, getter)
				lateEdit := false
				game.shapeMgr.add(frameUpdateFunc(func(float64) {
					if lateEdit {
						monitor.SetXpos(99)
						lateEdit = false
					}
				}))
				frame := func() { runFrameSyncCall(t, co, func() { runFrameSyncGame(co, game) }) }
				frame()
				if !panel.visible || getter.started != 1 || getter.completed != 1 {
					t.Fatal("initial getter did not show the monitor")
				}
				monitor.isDirty = true
				frame()
				if getter.started != 2 || getter.active != 1 {
					t.Fatal("second getter did not wait")
				}
				panel.events = nil
				if hide {
					monitor.Hide()
				} else {
					monitor.SetXYpos(12, -34)
					monitor.SetSize(2)
				}
				for elapsed := 1; elapsed <= test.frames; elapsed++ {
					// This edit occurs after the UI refresh but before getter resumption.
					lateEdit = !hide && elapsed == test.frames
					frame()
					if panel.visible == hide || monitor.Visible() == hide {
						t.Fatalf("frame %d: panel visible=%v, logical visible=%v, events=%v", elapsed, panel.visible, monitor.Visible(), panel.events)
					}
					if hide && slices.Contains(panel.events, "show") {
						t.Fatalf("resumed getter restored stale visibility: %v", panel.events)
					}
					if !hide && (panel.pos != mathf.NewVec2(12, -34) || panel.scale != 2) {
						t.Fatalf("frame %d: waiting getter blocked layout: position=%v, scale=%v", elapsed, panel.pos, panel.scale)
					}
					wantCompleted := 1
					if elapsed == test.frames {
						wantCompleted = 2
					}
					if getter.started != 2 || getter.completed != wantCompleted || getter.active != 2-wantCompleted {
						t.Fatalf("frame %d: getter started/completed/active=%d/%d/%d", elapsed, getter.started, getter.completed, getter.active)
					}
				}
				if monitor.isDirty == hide {
					t.Fatalf("getter lost pending UI state: dirty=%v, hide=%v", monitor.isDirty, hide)
				}
				if !hide {
					frame()
					if monitor.isDirty || panel.pos != mathf.NewVec2(99, -34) {
						t.Fatal("late layout edit did not reach the next refresh")
					}
				}
			})
		}
	}
}

func TestFrameSyncMonitorWaitStopsBeforeGameDestroy(t *testing.T) {
	co, game, _ := setupFrameSyncGame(t)
	getter := &frameMonitorWaitGame{Game: game, wait: func() { engine.WaitNextFrame() }}
	monitor, panel := addFrameSyncMonitor(t, getter)
	runFrameSyncCall(t, co, func() { runFrameSyncGame(co, game) })
	if getter.started != 1 || getter.active != 1 {
		t.Fatal("getter did not wait before destroy")
	}
	stoppedBeforeDestroy := false
	// The engine drains script work before destroying the game.
	co.RunAfterStop(func() {
		stoppedBeforeDestroy = getter.active == 0 && !monitor.evaluating
		game.OnEngineDestroy()
	})
	runFrameSyncCall(t, co, func() {
		itime.Update(1.0/60, 60)
		co.Update()
	})
	if !stoppedBeforeDestroy || getter.completed != 0 || getter.active != 0 || len(panel.events) != 0 {
		t.Fatalf("destroy: stopped=%v, getter completed/active=%d/%d, UI events=%v", stoppedBeforeDestroy, getter.completed, getter.active, panel.events)
	}
}

func TestFrameSyncShapeBroadcastWaitResumesAfterReceiver(t *testing.T) {
	co, game, mgr := setupFrameSyncGame(t)
	sprite := addFrameSyncSprite(game, 1, "moving")
	var order []string
	game.OnMsg__1("move", func() {
		order = append(order, "receiver started")
		engine.WaitNextFrame()
		sprite.SetXpos(42)
		order = append(order, "receiver finished")
	})
	game.shapeMgr.add(frameUpdateFunc(func(float64) {
		if len(order) != 0 {
			return
		}
		order = append(order, "sender started")
		game.BroadcastAndWait__0("move")
		order = append(order, "sender finished")
	}))
	runFrameSyncCall(t, co, func() { runFrameSyncGame(co, game) })
	if !slices.Equal(order, []string{"sender started", "receiver started"}) || sprite.Xpos() != 0 {
		t.Fatalf("first frame: order=%v position=%v", order, sprite.Xpos())
	}
	runFrameSyncCall(t, co, func() { runFrameSyncGame(co, game) })
	if !slices.Equal(order, []string{"sender started", "receiver started", "receiver finished", "sender finished"}) || mgr.positions[1].X != 42 {
		t.Fatalf("second frame: order=%v backend=%v", order, mgr.positions[1])
	}
}

func TestFrameSyncShapeBatchRejectsStaleGame(t *testing.T) {
	for _, reset := range []bool{false, true} {
		t.Run(fmt.Sprintf("reset=%v", reset), func(t *testing.T) {
			co, game, _ := setupFrameSyncGame(t)
			later := 0
			game.shapeMgr.add(frameUpdateFunc(func(float64) {
				if reset {
					game.resetBootstrap()
				} else {
					engine.SetGame(new(Game))
				}
			}))
			game.shapeMgr.add(frameUpdateFunc(func(float64) { later++ }))
			runFrameSyncCall(t, co, func() { runFrameSyncGame(co, game) })
			if later != 0 {
				t.Fatal("queued shape started after its game changed")
			}
		})
	}
}

func TestFrameSyncDestroyCancelsOnlyItsSpriteContinuations(t *testing.T) {
	co, game, _ := setupFrameSyncGame(t)
	destroyed := addFrameSyncSprite(game, 1, "destroyed")
	survivor := addFrameSyncSprite(game, 2, "survivor")
	pkgengine.InitSpriteInstance(1, destroyed.runtimeState.SyncSprite, nil)
	owners := []any{destroyed, survivor, game}
	threads := make([]coroutine.Thread, len(owners))
	continued := make([]bool, len(owners))
	cleaned := make([]bool, len(owners))
	for i, owner := range owners {
		threads[i] = co.Create(owner, func(coroutine.Thread) {
			defer func() { cleaned[i] = true }()
			engine.WaitNextFrame()
			continued[i] = true
		})
		co.JoinFirstSlice(threads[i])
	}
	runFrameSyncCall(t, co, func() {
		destroyed.Destroy()
		co.Join(threads[0])
	})
	if !cleaned[0] || continued[0] || !destroyed.isDestroyed() || destroyed.runtimeState.SyncSprite == nil {
		t.Fatal("public Destroy did not cancel its continuation before proxy deletion committed")
	}
	runFrameSyncCall(t, co, func() {
		runFrameSyncGame(co, game)
		co.JoinAll(threads)
	})
	if !slices.Equal(continued, []bool{false, true, true}) || !slices.Equal(cleaned, []bool{true, true, true}) {
		t.Fatalf("continued=%v cleaned=%v; want only the surviving sprite and global callbacks to continue", continued, cleaned)
	}
}

func TestFrameSyncPreservesDirectEnginePositionWrite(t *testing.T) {
	co, game, mgr := setupFrameSyncGame(t)
	sprite := addFrameSyncSprite(game, 1, "moving")
	sprite.SetXpos(10)
	game.OnEngineBeforeUpdate(1.0 / 60)
	itime.Update(1.0/60, 60)
	// Low-level runtime tweens write the proxy after the physical snapshot.
	withEventRegistrationBarrier(frameEventDispatcher{}, func() {
		engine.WaitMainThread(func() {
			sprite.runtimeState.SyncSprite.SetPosition(mathf.NewVec2(42, 43))
		})
		sprite.Hide()
	})
	game.OnEngineUpdate(1.0 / 60)
	co.Update()
	game.OnEngineRender(1.0 / 60)
	if want := mathf.NewVec2(42, 43); mgr.positions[1] != want {
		t.Fatalf("visual batch replaced direct engine write: got %v, want %v", mgr.positions[1], want)
	}
	requirePosition(t, sprite, 42, 43)
	if sprite.transform().pendingPosition != (positionVersions{}) {
		t.Fatal("direct position write left superseded position intent pending")
	}
	mgr.positions[1] = mathf.NewVec2(50, 60)
	runFrameSyncGame(co, game)
	requirePosition(t, sprite, 50, 60)
}
