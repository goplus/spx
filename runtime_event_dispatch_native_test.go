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
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	coreevent "github.com/goplus/spx/v3/internal/core/event"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	itime "github.com/goplus/spx/v3/internal/time"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
	"github.com/visualfc/gid"
)

type eventDispatchPlatform struct {
	pkgengine.IPlatformMgr
	mainGID    atomic.Uint64
	workerSeen chan struct{}
	workerOnce sync.Once
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

type eventLifecycleFunc func(coroutine.Thread) func()

func (f eventLifecycleFunc) RegisterInvocation(thread coroutine.Thread) func() {
	return f(thread)
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

func TestExternalAsyncEventDispatchSkipsShutdownBarrier(t *testing.T) {
	co := setupRuntimeScheduler(t)
	previousPlatform := pkgengine.PlatformMgr
	pkgengine.PlatformMgr = &eventDispatchPlatform{}
	t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })

	called := false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := co.RunAfterStop(ctx, func() {
		withEventRegistrationBarrier(nil, func() { called = true })
	}); err != nil {
		t.Fatalf("shutdown barrier did not complete: %v", err)
	}
	if called {
		t.Fatal("event dispatch ran while coroutine admission was closed")
	}
}

type inputFrameSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	platform    *eventDispatchPlatform
	queries     atomic.Int32
	wrongThread atomic.Bool
}

func (p *inputFrameSpriteMgr) CheckCollisionWithPoint(pkgengine.Object, mathf.Vec2, bool) bool {
	p.queries.Add(1)
	if !p.platform.IsMainThread() {
		p.wrongThread.Store(true)
	}
	return true
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
