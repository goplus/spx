//go:build !js && !pure_engine
// +build !js,!pure_engine

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
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	spxfs "github.com/goplus/spx/v4/fs"
	"github.com/goplus/spx/v4/fs/asset"
	coreevent "github.com/goplus/spx/v4/internal/core/event"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	itime "github.com/goplus/spx/v4/internal/time"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
	"github.com/visualfc/gid"
)

type eventDispatchPlatform struct {
	pkgengine.IPlatformMgr
	mainGID    atomic.Uint64
	workerSeen chan struct{}
	workerOnce sync.Once
}

type eventLifecycleFunc func(coroutine.Thread) func()

type inputFrameSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	platform    *eventDispatchPlatform
	queries     atomic.Int32
	wrongThread atomic.Bool
}

type failingPauseExtMgr struct{ inputFrameFailureExtMgr }

type resetDuringStartupPlatform struct {
	loadGamePlatformMgr
	onTitle func()
}

type queuedQuerySpriteMgr struct {
	*frameSyncSpriteMgr
	queriedPosition mathf.Vec2
}

func TestConditionSamplingServicesPendingMainThreadCall(t *testing.T) {
	co, game, input := setupInputSourceTest(t)
	platform := &eventDispatchPlatform{workerSeen: make(chan struct{})}
	previousPlatform := pkgengine.PlatformMgr
	pkgengine.PlatformMgr = platform
	t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })
	var returned chan struct{}
	t.Cleanup(func() {
		co.StopAll()
		if returned != nil {
			<-returned
		}
	})
	game.lifecycleState.IsRunned.Store(true)
	game.lifecycleState.StartDispatched.Store(true)
	value, sampled, handled := 0, 0, 0
	game.OnCond(func() bool {
		sampled = value
		return game.MouseX() == 23
	}, func() { handled++ })
	co.Create(game, func(coroutine.Thread) {
		co.WaitMainThread(func() { input.point.X = 23 })
		value = 42
	})
	select {
	case <-platform.workerSeen:
	case <-time.After(time.Second):
		t.Fatal("script did not request the main thread")
	}
	returned = make(chan struct{})
	go func() {
		platform.useCurrentAsMainThread()
		game.OnEngineBeforeUpdate(1.0 / 30)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("input and condition sampling deadlocked behind an engine call")
	}
	platform.useCurrentAsMainThread()
	if sampled != 42 || handled != 0 {
		t.Fatalf("sampled value/handlers = %d/%d, want 42/0", sampled, handled)
	}
	itime.Update(1.0/30, 30)
	game.dispatchFrameEvents()
	co.Update()
	if handled != 1 {
		t.Fatalf("condition handlers = %d, want 1", handled)
	}
}

func TestExternalAsyncEventDispatchRunsMainThreadRoundTrips(t *testing.T) {
	co := setupRuntimeScheduler(t)
	platform := &eventDispatchPlatform{workerSeen: make(chan struct{})}
	previousPlatform := pkgengine.PlatformMgr
	pkgengine.PlatformMgr = platform
	t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })

	var engineCalls atomic.Int32
	var wrongThread atomic.Bool
	engineCall := func() {
		engineCalls.Add(1)
		if gid.Get() != platform.mainGID.Load() {
			wrongThread.Store(true)
		}
	}
	active := co.Create("active", func(coroutine.Thread) {
		co.WaitMainThread(engineCall)
	})
	select {
	case <-platform.workerSeen:
	case <-time.After(time.Second):
		t.Fatal("active coroutine did not request the engine main thread")
	}

	producerReturned := make(chan struct{})
	resumeEngine := make(chan struct{})
	handlerDone := make(chan struct{})
	cleanupDone := make(chan struct{})
	engineDone := make(chan struct{})
	var registered, cleaned atomic.Int32
	handler := eventLifecycleFunc(func(coroutine.Thread) func() {
		registered.Add(1)
		return func() {
			cleaned.Add(1)
			close(cleanupDone)
		}
	})
	event := scriptEventDispatch{
		mode: coroutine.BatchAsync,
		run: func(coroutine.Thread, *eventSink) {
			co.WaitMainThread(engineCall)
			co.WaitMainThread(engineCall)
			close(handlerDone)
		},
	}
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		platform.useCurrentAsMainThread()
		withEventRegistrationBarrier(nil, func() {
			dispatchMatchedScriptEventBatch([]eventSink{{Owner: "handler", Handler: handler}}, event)
		})
		close(producerReturned)
		<-resumeEngine
		co.Update()
		close(engineDone)
	}()

	select {
	case <-producerReturned:
	case <-time.After(time.Second):
		t.Fatal("external event dispatch deadlocked with a main-thread call")
	}
	select {
	case <-handlerDone:
		t.Fatal("async producer waited for the handler to finish")
	default:
	}
	close(resumeEngine)
	select {
	case <-engineDone:
	case <-time.After(time.Second):
		t.Fatal("engine thread did not finish handler round trips")
	}
	select {
	case <-handlerDone:
	default:
		t.Fatal("event handler did not finish")
	}
	select {
	case <-cleanupDone:
	default:
		t.Fatal("event handler cleanup did not finish")
	}
	if got := engineCalls.Load(); got != 3 {
		t.Fatalf("engine calls = %d, want 3", got)
	}
	if wrongThread.Load() {
		t.Fatal("engine call ran outside the designated engine thread")
	}
	if got := registered.Load(); got != 1 {
		t.Fatalf("lifecycle registrations = %d, want 1", got)
	}
	if got := cleaned.Load(); got != 1 {
		t.Fatalf("lifecycle cleanups = %d, want 1", got)
	}
	co.Join(active)
}

func TestWorkerAsyncEventDispatchRegistersBeforeReturning(t *testing.T) {
	previousPlatform := pkgengine.PlatformMgr
	pkgengine.PlatformMgr = &eventDispatchPlatform{}
	t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })
	co := setupRuntimeScheduler(t)
	gate := co.NewLatch()
	var registered int
	var dispatcherOwner any
	var finished atomic.Bool
	handler := eventLifecycleFunc(func(coroutine.Thread) func() {
		registered++
		if co.IsInCoroutine() {
			dispatcherOwner = co.Current().Owner
		}
		return nil
	})
	returned := make(chan struct{})
	go func() {
		withEventRegistrationBarrier("dispatcher", func() {
			dispatchMatchedScriptEventBatch([]eventSink{{Owner: "handler", Handler: handler}}, scriptEventDispatch{
				mode: coroutine.BatchAsync,
				run: func(coroutine.Thread, *eventSink) {
					gate.Wait()
					finished.Store(true)
				},
			})
		})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("worker event dispatch waited for the async handler to finish")
	}
	if registered != 1 || dispatcherOwner != "dispatcher" {
		t.Fatalf("registration count/dispatcher owner = %d/%v, want 1/dispatcher", registered, dispatcherOwner)
	}
	if finished.Load() {
		t.Fatal("handler finished before its gate opened")
	}
	gate.Open()
	co.Update()
	if !finished.Load() {
		t.Fatal("registered handler did not finish after its gate opened")
	}
}

func TestExternalAsyncEventDispatchSkipsShutdownBarrier(t *testing.T) {
	co := setupRuntimeScheduler(t)
	previousPlatform := pkgengine.PlatformMgr
	pkgengine.PlatformMgr = &eventDispatchPlatform{}
	t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })

	called := false
	co.RunAfterStop(func() {
		withEventRegistrationBarrier(nil, func() { called = true })
	})
	if called {
		t.Fatal("event dispatch ran while coroutine admission was closed")
	}
}

func TestAsyncDispatchDuringShutdownDoesNotRunRejectedLifecycle(t *testing.T) {
	co := setupRuntimeScheduler(t)
	platform := &eventDispatchPlatform{}
	previousPlatform := pkgengine.PlatformMgr
	pkgengine.PlatformMgr = platform
	t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })

	callbackStarted := make(chan coroutine.Thread, 1)
	releaseCallback := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseCallback) }) }
	defer release()
	registrationDone := make(chan struct{})
	var registered, ran atomic.Bool
	handler := &scriptEventHandler{}
	lifecycle := eventLifecycleFunc(func(thread coroutine.Thread) func() {
		registered.Store(true)
		return handler.RegisterInvocation(thread)
	})
	event := scriptEventDispatch{
		mode: coroutine.BatchAsync,
		run:  func(coroutine.Thread, *eventSink) { ran.Store(true) },
	}
	go func() {
		platform.useCurrentAsMainThread()
		co.TryRunFromEngine("outer", func() {
			callbackStarted <- co.Current()
			<-releaseCallback
			dispatchMatchedScriptEventBatch([]eventSink{{Owner: handler, Handler: lifecycle}}, event)
			close(registrationDone)
		})
	}()
	var outer coroutine.Thread
	select {
	case outer = <-callbackStarted:
	case <-time.After(time.Second):
		t.Fatal("outer dispatcher callback did not start")
	}

	stopDone := make(chan struct{})
	go func() {
		co.RunAfterStop(nil)
		close(stopDone)
	}()
	// Cancellation is the handshake that admission has closed. The outer callback
	// remains blocked so shutdown cannot finish before the nested registration.
	select {
	case <-outer.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel the outer dispatcher")
	}
	select {
	case <-stopDone:
		t.Fatal("shutdown returned before the outer dispatcher exited")
	default:
	}
	release()
	select {
	case <-registrationDone:
	case <-time.After(time.Second):
		t.Fatal("nested batch registration did not finish")
	}
	if registered.Load() {
		t.Fatal("rejected nested handler ran lifecycle registration during shutdown")
	}
	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish after the outer dispatcher exited")
	}
	if ran.Load() {
		t.Fatal("rejected nested handler ran during shutdown")
	}
	co.StartBatch([]coroutine.Task{{
		Owner:              handler,
		RegisterInvocation: handler.RegisterInvocation,
		Run:                func(coroutine.Thread) { ran.Store(true) },
	}}, coroutine.BatchAsync)
	co.Update()
	if !ran.Load() {
		t.Fatal("handler could not run after shutdown reopened admission")
	}
}

func TestDispatchFrameEventsServicesClickQueriesOnEngineThread(t *testing.T) {
	_, game := setupInputFrameGame(t)
	platform := &eventDispatchPlatform{}
	mgr := &inputFrameSpriteMgr{platform: platform}
	previousPlatform, previousSprites := pkgengine.PlatformMgr, pkgengine.SpriteMgr
	pkgengine.PlatformMgr, pkgengine.SpriteMgr = platform, mgr
	enginewrap.Init(engine.WaitMainThread)
	t.Cleanup(func() {
		pkgengine.PlatformMgr, pkgengine.SpriteMgr = previousPlatform, previousSprites
		enginewrap.Init(func(call func()) { call() })
	})
	sprite := newClickTestSprite(game, "button", 1, false)
	game.shapeMgr.items = []Shape{sprite}
	var routed atomic.Bool
	game.scriptEvents.manager.Add(coreevent.BucketClick, coreevent.NewSink(sprite, nil, func(any) bool {
		routed.Store(true)
		return false
	}))
	game.inputMgr.frame.pending = &inputFrame{events: []event{&eventLeftButtonDown{}}}
	returned := make(chan struct{})
	go func() {
		platform.useCurrentAsMainThread()
		game.dispatchFrameEvents()
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("frame input dispatch deadlocked on an engine-thread click query")
	}
	if mgr.queries.Load() != 1 || mgr.wrongThread.Load() || !routed.Load() {
		t.Fatalf("dispatch returned with queries=%d, wrongThread=%v, routed=%v", mgr.queries.Load(), mgr.wrongThread.Load(), routed.Load())
	}
}

func TestSnapshotReportsBodyAndCaptureFailures(t *testing.T) {
	for _, tt := range []struct {
		name       string
		activeGame bool
		bodyError  bool
	}{
		{name: "body without game", bodyError: true},
		{name: "body with game", activeGame: true, bodyError: true},
		{name: "capture backend"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupSnapshotInputSessionTest(t)
			setupImmediateEngineCalls(t)
			original := pkgengine.ExtMgr
			recorder := &inputFrameFailureExtMgr{}
			pkgengine.ExtMgr = recorder
			t.Cleanup(func() { pkgengine.ExtMgr = original })
			setupRuntimeScheduler(t)
			if tt.activeGame {
				engine.SetGame(struct{}{})
			}
			failure := errors.New("snapshot " + tt.name + " failed")
			bodyCalls, captures := 0, 0
			engine.SetCaptureHandler(func(req engine.CaptureRequest) error {
				captures++
				if req.Name != "failed.png" {
					t.Errorf("capture name = %q, want failed.png", req.Name)
				}
				return failure
			})
			Snapshot("failed.png", func() error {
				bodyCalls++
				if tt.bodyError {
					return failure
				}
				return nil
			})
			if !slices.Equal(recorder.messages, []string{failure.Error()}) || !slices.Equal(recorder.exits, []int64{1}) {
				t.Fatalf("runtime errors = %v, exits = %v; want %q and exit 1", recorder.messages, recorder.exits, failure)
			}
			wantCaptures := 1
			if tt.bodyError {
				wantCaptures = 0
			}
			if bodyCalls != 1 || captures != wantCaptures || engine.HasPendingCaptures() {
				t.Fatalf("body calls = %d, captures = %d, pending = %v; want 1, %d, false", bodyCalls, captures, engine.HasPendingCaptures(), wantCaptures)
			}
		})
	}
}

func TestRuntimeInvalidConfigurationAndIdentityReportErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		call func()
	}{
		{"invalid environment value", func() {
			(&Game{}).applyRuntimeConfig(&Config{ScreenshotKey: "bad\x00key"}, &coreproject.ProjectConfig{})
		}},
		{"invalid script owner", func() {
			if got := nameOf(struct{}{}); got != "" {
				t.Errorf("invalid owner name = %q", got)
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := setupRuntimeFailureRecorder(t)
			tt.call()
			if len(recorder.messages) != 1 || recorder.messages[0] == "" || !slices.Equal(recorder.exits, []int64{1}) {
				t.Fatalf("errors=%v exits=%v", recorder.messages, recorder.exits)
			}
		})
	}
}

func TestExitOverloadsForwardExitCodes(t *testing.T) {
	for _, code := range []int{0, 7} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			recorder := setupRuntimeFailureRecorder(t)
			if code == 0 {
				Exit__0()
			} else {
				Exit__1(code)
			}
			if !slices.Equal(recorder.exits, []int64{int64(code)}) || len(recorder.messages) != 0 {
				t.Fatalf("exit forwarded: %v / %v", recorder.exits, recorder.messages)
			}
		})
	}
}

func TestFinishingInputSessionReportsPauseFailure(t *testing.T) {
	resetInputSessionTest(t)
	setupRuntimeScheduler(t)
	setupImmediateEngineCalls(t)
	previous := pkgengine.ExtMgr
	recorder := &failingPauseExtMgr{}
	pkgengine.ExtMgr = recorder
	t.Cleanup(func() { pkgengine.ExtMgr = previous })
	session, err := newInputSession(&inputSessionPlan{mode: InputSessionModeReplaying, replay: validRuntimeReplay()})
	if err != nil {
		t.Fatal(err)
	}
	game := &Game{inputSession: session}
	engine.SetGame(game)
	if _, err := session.consumeSampledInputTick(0, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
		return InputReplayState{}, nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	game.OnEngineFrameEnd()
	if session.phase != InputSessionPhaseAborted || len(recorder.messages) != 1 || !slices.Equal(recorder.exits, []int64{1}) {
		t.Fatalf("failed completion: %+v, %v, %v", session.status(), recorder.messages, recorder.exits)
	}
	game.inputSession = &inputSession{mode: InputSessionModeRecording, phase: InputSessionPhaseAborted, abortReason: "recording failed"}
	game.lifecycleState.IsRunned.Store(true)
	if _, err := FinishInputRecording(); err == nil || !strings.Contains(err.Error(), "recording failed") {
		t.Fatalf("finish error = %v", err)
	}
}

func TestOnEngineStartReportsResourceLoadFailure(t *testing.T) {
	if isolatedGameTest(t) {
		return
	}
	co, game := setupRuntimeEventGame(t)
	setupImmediateEngineCalls(t)
	previous := pkgengine.ExtMgr
	recorder := &runtimePanicRecorder{}
	pkgengine.ExtMgr = recorder
	t.Cleanup(func() { pkgengine.ExtMgr = previous })
	failure := errors.New("startup resources unavailable")
	spxfs.RegisterSchema("", func(path string) (spxfs.Dir, error) {
		if path != "assets" {
			t.Errorf("startup resource path=%q,want assets", path)
		}
		return nil, failure
	})
	t.Cleanup(func() { spxfs.RegisterSchema("", asset.Open) })
	game.OnEngineStart()
	co.Update()
	if len(recorder.messages) != 1 || !strings.Contains(recorder.messages[0], failure.Error()) || !slices.Equal(recorder.exits, []int64{1}) {
		t.Fatalf("startup failure messages=%v exits=%v", recorder.messages, recorder.exits)
	}
	if game.lifecycleState.IsRunned.Load() || game.lifecycleState.BootstrapDone.Load() {
		t.Fatal("failed resource load completed startup")
	}
}

func TestOnEngineStartRejectsResetDuringResourceLoad(t *testing.T) {
	if isolatedGameTest(t) {
		return
	}
	setupLoadGameFlags(t)
	files := gameTestFS{"index.json": `{"run":{"title":"stale startup"}}`}
	gamer := &engineStartTestGame{Game: &Game{}}
	game := setupGameTestRuntime(t, files, gamer, nil, false)
	game.lifecycleState.IsRunned.Store(false)
	generation := game.bootstrapGeneration()
	platform := &resetDuringStartupPlatform{onTitle: game.resetBootstrap}
	pkgengine.PlatformMgr = platform
	pkgengine.ResMgr = &loadGameAssetResMgr{}
	previousAudio := pkgengine.AudioMgr
	pkgengine.AudioMgr = &loadGameAudioMgr{}
	t.Cleanup(func() { pkgengine.AudioMgr = previousAudio })
	resource := &loadGameFS{gameTestFS: files}
	spxfs.RegisterSchema("", func(path string) (spxfs.Dir, error) { return resource, nil })
	t.Cleanup(func() { spxfs.RegisterSchema("", asset.Open) })
	game.OnEngineStart()
	gco.Update()
	if platform.title != "stale startup" || game.fs != resource || resource.closeCalls != 0 {
		t.Fatalf("resource loading did not finish: title=%q fs=%v closes=%d", platform.title, game.fs, resource.closeCalls)
	}
	if game.bootstrapGeneration() != generation+1 || game.lifecycleState.IsRunned.Load() || game.lifecycleState.BootstrapDone.Load() || gamer.mainCalls != 0 {
		t.Fatalf("stale startup survived reset: generation=%d running=%v bootstrap=%v main=%d", game.bootstrapGeneration(), game.lifecycleState.IsRunned.Load(), game.lifecycleState.BootstrapDone.Load(), gamer.mainCalls)
	}
}

func TestFrameSyncQueuedQueryMergesPosition(t *testing.T) {
	for _, tt := range []struct {
		name   string
		writeX bool
		x      float64
		want   mathf.Vec2
	}{
		{name: "visual only", want: mathf.NewVec2(20, 30)},
		{name: "set X", writeX: true, x: 42, want: mathf.NewVec2(42, 30)},
		{name: "set unchanged X", writeX: true, x: 10, want: mathf.NewVec2(10, 30)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			co, game, backend := setupFrameSyncGame(t)
			sprite := addFrameSyncSprite(game, 1, "moving")
			sprite.SetXYpos(10, 11)
			game.OnEngineRender(0)
			mgr := &queuedQuerySpriteMgr{frameSyncSpriteMgr: backend}
			pkgengine.SpriteMgr = mgr
			platform := &eventDispatchPlatform{workerSeen: make(chan struct{})}
			platform.useCurrentAsMainThread()
			previousPlatform := pkgengine.PlatformMgr
			pkgengine.PlatformMgr = platform
			t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })
			enginewrap.Init(engine.WaitMainThread)

			release := make(chan struct{})
			thread := co.Create(sprite, func(coroutine.Thread) {
				engine.WaitForChan(release)
				if tt.writeX {
					sprite.SetXpos(tt.x)
				}
				sprite.Hide()
				sprite.TouchingColor__0(Color{})
			})
			co.JoinFirstSlice(thread)
			// Physics advances while the script waits for external work.
			mgr.positions[1] = mathf.NewVec2(20, 30)
			close(release)
			select {
			case <-platform.workerSeen:
			case <-time.After(time.Second):
				t.Fatal("resumed script did not request the engine main thread")
			}
			// The entry boundary must service the query before it can acquire
			// the script lock and perform its normal physics snapshot.
			game.OnEngineBeforeUpdate(1.0 / 60)
			co.Join(thread)
			if mgr.queriedPosition != tt.want || mgr.positions[1] != tt.want {
				t.Fatalf("query position=%v backend=%v, want %v", mgr.queriedPosition, mgr.positions[1], tt.want)
			}
			requirePosition(t, sprite, tt.want.X, tt.want.Y)
		})
	}
}

func (p *eventDispatchPlatform) IsMainThread() bool {
	if p.mainGID.Load() == gid.Get() {
		return true
	}
	if p.workerSeen != nil {
		p.workerOnce.Do(func() { close(p.workerSeen) })
	}
	return false
}

func (p *eventDispatchPlatform) useCurrentAsMainThread() {
	p.mainGID.Store(gid.Get())
}

func (f eventLifecycleFunc) RegisterInvocation(thread coroutine.Thread) func() {
	return f(thread)
}

func (p *inputFrameSpriteMgr) CheckCollisionWithPoint(pkgengine.Object, mathf.Vec2, bool) bool {
	p.queries.Add(1)
	if !p.platform.IsMainThread() {
		p.wrongThread.Store(true)
	}
	return true
}

func (*failingPauseExtMgr) Pause() { panic("pause unavailable") }

func (p *resetDuringStartupPlatform) SetWindowTitle(title string) {
	p.loadGamePlatformMgr.SetWindowTitle(title)
	p.onTitle()
}

func (m *queuedQuerySpriteMgr) GetPosition(id pkgengine.Object) mathf.Vec2 {
	return m.positions[int64(id)]
}

func (m *queuedQuerySpriteMgr) SetTransform(id pkgengine.Object, pos mathf.Vec2, _ float64, _ mathf.Vec2, _ bool, _ mathf.Vec2) {
	m.positions[int64(id)] = pos
}

func (m *queuedQuerySpriteMgr) CheckCollisionByColor(id pkgengine.Object, _ mathf.Color, _, _ float64) bool {
	m.queriedPosition = m.positions[int64(id)]
	return false
}

func setupRuntimeFailureRecorder(t *testing.T) *inputFrameFailureExtMgr {
	t.Helper()
	previousGame := engine.GetGame()
	engine.SetGame(nil)
	t.Cleanup(func() { engine.SetGame(previousGame) })
	setupRuntimeScheduler(t)
	setupImmediateEngineCalls(t)
	old := pkgengine.ExtMgr
	recorder := &inputFrameFailureExtMgr{}
	pkgengine.ExtMgr = recorder
	t.Cleanup(func() { pkgengine.ExtMgr = old })
	return recorder
}
