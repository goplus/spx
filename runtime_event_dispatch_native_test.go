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
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
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

func TestExternalAsyncEventDispatchRunsMainThreadRoundTrips(t *testing.T) {
	co := setupRuntimeEventScheduler(t)
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
	active := co.Create("active", func(coroutine.Thread) int {
		co.WaitMainThread(engineCall)
		return 0
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
	handler := &messageEventHandler{}
	event := scriptEventDispatch{
		mode: coroutine.BatchAsync,
		lifecycle: func(coroutine.Thread, *eventSink) func() {
			registered.Add(1)
			return func() {
				cleaned.Add(1)
				close(cleanupDone)
			}
		},
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
		event.withRegistrationBarrier(func() {
			dispatchMatchedScriptEventBatch([]eventSink{{Owner: handler, Handler: handler}}, event)
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
	co := setupRuntimeEventScheduler(t)
	previousPlatform := pkgengine.PlatformMgr
	pkgengine.PlatformMgr = &eventDispatchPlatform{}
	t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })

	called := false
	event := scriptEventDispatch{mode: coroutine.BatchAsync}
	if !co.RunAfterAbortAll(time.Second, func() {
		event.withRegistrationBarrier(func() { called = true })
	}) {
		t.Fatal("shutdown barrier did not complete")
	}
	if called {
		t.Fatal("event dispatch ran while coroutine admission was closed")
	}
}
