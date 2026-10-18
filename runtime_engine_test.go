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
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	coreevent "github.com/goplus/spx/v3/internal/core/event"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/engine/profiler"
	"github.com/goplus/spx/v3/internal/enginewrap"
	itime "github.com/goplus/spx/v3/internal/time"
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

func TestLogicFramePreservesPhasesAndShapeSnapshot(t *testing.T) {
	setupRuntimeScheduler(t)
	itime.OnReload()
	itime.Start(nil)
	t.Cleanup(itime.OnReload)
	game := &Game{sounds: make(map[string]sound)}
	backend := &animationAudioBackend{}
	game.soundMgr.Init(backend)

	newSprite := func(name string) *SpriteImpl {
		anim := newTestAnimationComponent()
		sprite := anim.sprite
		sprite.name, sprite.g = name, game
		game.sounds[name] = &coreproject.SoundConfig{Path: name + ".wav"}
		sprite.sound().pendingAudios = []string{name}
		anim.curAnimState = &animState{Name: name}
		anim.doneAnimations = []*animState{anim.curAnimState}
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
	}
	itime.RegisterTimer(0.2)
	itime.RegisterTimer(0.1)
	itime.RegisterTimer(0.3)
	itime.Update(0.2, 30)
	audios, animations := make([]string, 0, 4), make([]*animState, 0, 4)
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
	// Timers belong to the frame event phase, not the audio/animation loop.
	for _, timestamp := range []int64{100, 200} {
		if timer, ok := itime.NextTimer(); !ok || timer != timestamp {
			t.Fatalf("pending timer = %d, %v, want %d", timer, ok, timestamp)
		}
	}
	if _, ok := itime.NextTimer(); ok {
		t.Fatal("future timer became due early")
	}
	steps = nil
	audios, animations = game.processLogicFrame(audios, animations)
	want = []string{"play:" + engine.ToAssetPath("late.wav"), "complete:3"}
	if !slices.Equal(steps, want) {
		t.Fatalf("next frame = %v, want %v", steps, want)
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
		audios, animations := game.processLogicFrame([]string{}, []*animState{})
		if audios == nil || animations == nil || len(audios) != 0 || len(animations) != 0 {
			t.Fatalf("scratch = %#v, %#v, want non-nil empty slices", audios, animations)
		}
		audios, animations = game.processLogicFrame(nil, nil)
		if audios != nil || animations != nil {
			t.Fatalf("nil scratch = %#v, %#v, want nil slices", audios, animations)
		}
	}
}

func setupInputFrameGame(t *testing.T) (*coroutine.Coroutines, *Game) {
	t.Helper()
	itime.OnReload()
	itime.Start(nil)
	t.Cleanup(itime.OnReload)
	co, game := setupRuntimeEventGame(t)
	game.inputMgr.init(game)
	return co, game
}

func TestOnEngineBeforeUpdateSamplesInputBeforeConditions(t *testing.T) {
	for _, mode := range []string{"scheduler", "direct"} {
		t.Run(mode, func(t *testing.T) {
			co, game, input := setupInputSourceTest(t)
			if mode == "direct" {
				previous := gco
				gco = nil
				t.Cleanup(func() { gco = previous })
			}
			game.lifecycleState.IsRunned.Store(true)
			game.lifecycleState.StartDispatched.Store(true)
			input.point.X, input.point.Y = 23, -17
			frame := itime.Frame()
			samples, handlers := 0, 0
			game.OnCond(func() bool {
				samples++
				if game.MouseX() != input.point.X || game.MouseY() != input.point.Y || itime.Frame() != frame {
					t.Error("condition did not observe current input before the clock advanced")
				}
				return true
			}, func() { handlers++ })
			game.OnEngineBeforeUpdate(1.0 / 30)
			if samples != 1 || handlers != 0 || game.inputMgr.frame.pending == nil {
				t.Fatalf("samples=%d, handlers=%d, pending=%v, want 1, 0, and a prepared frame", samples, handlers, game.inputMgr.frame.pending != nil)
			}
			itime.Update(1.0/30, 30)
			game.dispatchFrameEvents()
			co.Update()
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
	co, game, _ := setupInputSourceTest(t)
	engine.ResetFrameRuntime()
	t.Cleanup(engine.ResetFrameRuntime)
	game.lifecycleState.IsRunned.Store(true)
	game.lifecycleState.StartDispatched.Store(true)
	game.camera = &cameraImpl{g: game}
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	samples, conditions, timers, callbacks := 0, 0, 0, 0
	game.OnCond(func() bool { samples++; return true }, func() { conditions++ })
	game.OnTimer(0.1, func() { timers++ })
	frame := engine.CurrentFrame() + 1
	AtFrame(frame, func() { callbacks++ })

	game.OnEngineBeforeUpdate(0.1)
	if pending := game.inputMgr.frame.pending; pending == nil || len(pending.events) != 0 {
		t.Fatalf("prepared input = %+v, want a valid empty frame", pending)
	}
	if samples != 1 || conditions != 0 || timers != 0 || callbacks != 0 {
		t.Fatalf("before clock: samples/conditions/timers/callbacks = %d/%d/%d/%d, want 1/0/0/0", samples, conditions, timers, callbacks)
	}
	itime.Update(0.1, 30)
	for update := range 2 {
		game.OnEngineUpdate(0.1)
		co.Update()
		if samples != 1 || conditions != 1 || timers != 1 || callbacks != 1 {
			t.Fatalf("update %d: samples/conditions/timers/callbacks = %d/%d/%d/%d, want 1/1/1/1", update+1, samples, conditions, timers, callbacks)
		}
		if game.inputMgr.frame.pending != nil || engine.CurrentFrame() != frame {
			t.Fatal("update retained the consumed frame or advanced the clock")
		}
	}
}

func TestOnEngineUpdateStopsAfterReplayEOF(t *testing.T) {
	co, game, _ := setupInputSourceTest(t)
	engine.ResetFrameRuntime()
	t.Cleanup(engine.ResetFrameRuntime)
	if _, err := PrepareInputReplay(validRuntimeReplay(InputReplayFrame{
		Frame:     0,
		State:     InputReplayState{KeysDown: []int64{int64(KeyA)}},
		KeyEvents: []InputReplayKeyEvent{{Key: int64(KeyA), Pressed: true}},
	})); err != nil {
		t.Fatal(err)
	}
	session := claimPreparedSession(t, game)
	game.lifecycleState.IsRunned.Store(true)
	game.lifecycleState.StartDispatched.Store(true)
	samples, conditions, keys, callbacks := 0, 0, 0, 0
	game.OnCond(func() bool { samples++; return true }, func() { conditions++ })
	game.OnKey__0(KeyA, func() { keys++ })
	game.OnEngineBeforeUpdate(1.0 / 30)
	itime.Update(1.0/30, 30)
	game.dispatchFrameEvents()
	co.Update()
	session.endFrame()
	if completed, err := session.completeReplayFrame(nil); err != nil || !completed {
		t.Fatalf("replay completion = %v, %v", completed, err)
	}

	AtFrame(engine.CurrentFrame()+1, func() { callbacks++ })
	game.OnEngineBeforeUpdate(1.0 / 30)
	itime.Update(1.0/30, 30)
	game.OnEngineUpdate(1.0 / 30)
	co.Update()
	if samples != 1 || conditions != 1 || keys != 1 || callbacks != 0 {
		t.Fatalf("samples/conditions/keys/callbacks=%d/%d/%d/%d, want 1/1/1/0", samples, conditions, keys, callbacks)
	}
}

func TestDispatchFrameEventsRegistersLargeInputBatchBeforeReturn(t *testing.T) {
	co, game := setupInputFrameGame(t)
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
	co.Update()
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
	co.Update()
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
	co, game := setupInputFrameGame(t)
	started, finished := make(chan struct{}), make(chan struct{})
	game.OnKey__0(KeySpace, func() {
		close(started)
		engine.WaitNextFrame()
		close(finished)
	})
	game.inputMgr.frame.pending = &inputFrame{events: []event{&eventKeyDown{Key: KeySpace}}}
	frame := itime.Frame()
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
	if itime.Frame() != frame {
		t.Fatal("input dispatch advanced the frame clock")
	}
	co.Update()
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
	itime.Update(1.0/30, 30)
	co.Update()
	select {
	case <-finished:
	default:
		t.Fatal("handler did not resume in the following frame")
	}
}

func TestDispatchFrameEventsUsesAdvancedClockForDueTimers(t *testing.T) {
	_, game := setupInputFrameGame(t)
	itime.RegisterTimer(0.1)
	itime.RegisterTimer(0.3)
	var routed []int64
	var frames []int64
	game.scriptEvents.manager.Add(coreevent.BucketAnyKeyPressed, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, int64(value.(Key)))
		frames = append(frames, itime.Frame())
		return false
	}))
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, value.(int64))
		frames = append(frames, itime.Frame())
		return false
	}))
	game.inputMgr.frame.pending = &inputFrame{events: []event{&eventKeyDown{Key: KeyA}}}
	frame := itime.Frame() + 1
	itime.Update(0.2, 30)
	game.dispatchFrameEvents()
	if want := []int64{int64(KeyA), 100}; !slices.Equal(routed, want) {
		t.Fatalf("input and timer routing = %v, want %v", routed, want)
	}
	if !slices.Equal(frames, []int64{frame, frame}) {
		t.Fatalf("routing frames = %v, want both in frame %d", frames, frame)
	}
	if timestamp, ok := itime.NextTimer(); ok {
		t.Fatalf("future timer %d became eligible before its frame", timestamp)
	}
}

func TestDueTimerCanRunBeforePendingLogicFrame(t *testing.T) {
	co, game := setupInputFrameGame(t)
	backend := &animationAudioBackend{}
	game.soundMgr.Init(backend)
	game.sounds = map[string]sound{"step": &coreproject.SoundConfig{Path: "step.wav"}}
	anim := newTestAnimationComponent()
	anim.sprite.g = game
	anim.sprite.sound().pendingAudios = []string{"step"}
	anim.curAnimState = &animState{Name: "step"}
	anim.doneAnimations = []*animState{anim.curAnimState}
	game.shapeMgr.items = []Shape{anim.sprite}
	var phases eventOrderLog
	backend.onPlay = func(id int64) {
		phases.add("audio")
		anim.curAnimState.OnPlayAudioPlaybackID = id
	}
	backend.onStop = func(int64) { phases.add("animation-complete") }
	logic := co.Create("logicLoop", func(coroutine.Thread) {
		engine.WaitNextFrame()
		game.processLogicFrame(nil, nil)
	})
	co.JoinFirstSlice(logic)
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(any) bool {
		phases.add("timer-route")
		return false
	}))
	handled := make(chan struct{})
	game.OnTimer(0.1, func() {
		phases.add("timer-handler")
		close(handled)
	})
	itime.Update(0.1, 30)
	frame := itime.Frame()
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
	co.Update()
	co.Join(logic)
	requireEventOrder(t, &phases, []string{"timer-route", "timer-handler", "audio", "animation-complete"})
	if itime.Frame() != frame || anim.curAnimState != nil || len(anim.doneAnimations) != 0 {
		t.Fatal("pending logic did not finish its animation in the timer's frame")
	}
}

func TestZeroTimerRoutesBeforeStartRegistration(t *testing.T) {
	co, game := setupInputFrameGame(t)
	game.lifecycleState.IsRunned.Store(true)
	game.completeBootstrap(game.bootstrapGeneration())
	game.camera = &cameraImpl{g: game}
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	routed, timers, starts := 0, 0, 0
	frame := itime.Frame() + 1
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(any) bool {
		routed++
		if game.startScheduled || game.lifecycleState.StartDispatched.Load() || itime.Frame() != frame {
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
	itime.Update(1.0/30, 30)
	game.OnEngineUpdate(1.0 / 30)
	co.Update()
	// Both handlers run; their order relative to each other is not the contract.
	if routed != 1 || timers != 1 || starts != 1 || !game.lifecycleState.StartDispatched.Load() {
		t.Fatalf("timer routes/handlers/start handlers = %d/%d/%d, start dispatched = %v",
			routed, timers, starts, game.lifecycleState.StartDispatched.Load())
	}
}

func TestDispatchFrameEventsFreezesTimersBeforeRouting(t *testing.T) {
	_, game := setupInputFrameGame(t)
	itime.RegisterTimer(0)
	var routed []int64
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, value.(int64))
		if len(routed) == 1 {
			// A newly registered overdue timer and a reset are eligible for the
			// following batch, even though they happen during event routing.
			itime.RegisterTimer(-0.1)
			itime.ResetTimer()
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
	co, game := setupInputFrameGame(t)
	started, unblock := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(unblock) })
	t.Cleanup(release)
	blocker := co.Create(game, func(coroutine.Thread) {
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
		co.StopIf(func(thread coroutine.Thread) bool {
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
	previous := gco
	gco = nil
	t.Cleanup(func() { gco = previous })
	// Run callbacks inline so a condition handler deterministically mutates the
	// timer state during dispatch, rather than depending on scheduler timing.
	conditions := 0
	game.OnCond(func() bool { return true }, func() {
		conditions++
		itime.ResetTimer()
		itime.RegisterTimer(0)
	})
	itime.RegisterTimer(0.1)
	var routed []int64
	game.scriptEvents.manager.Add(coreevent.BucketTimer, coreevent.NewSink(game, nil, func(value any) bool {
		routed = append(routed, value.(int64))
		return false
	}))
	game.scriptEvents.sampleConditions()
	itime.Update(0.2, 30)
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
