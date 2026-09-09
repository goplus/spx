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
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type eventDispatchPlatform struct {
	pkgengine.IPlatformMgr
	checks     atomic.Int32
	firstCheck chan struct{}
}

func (p *eventDispatchPlatform) IsMainThread() bool {
	if p.firstCheck != nil && p.checks.Add(1) == 1 {
		close(p.firstCheck)
		return false
	}
	return true
}

func TestExternalAsyncEventDispatchServicesPendingMainThreadCall(t *testing.T) {
	co := setupRuntimeEventScheduler(t)
	platform := &eventDispatchPlatform{firstCheck: make(chan struct{})}
	previousPlatform := pkgengine.PlatformMgr
	pkgengine.PlatformMgr = platform
	t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })

	engineCallRan := make(chan struct{})
	active := co.Create("active", func(coroutine.Thread) int {
		co.WaitMainThread(func() { close(engineCallRan) })
		return 0
	})
	select {
	case <-platform.firstCheck:
	case <-time.After(time.Second):
		t.Fatal("active coroutine did not request the engine main thread")
	}

	dispatchRan := make(chan struct{})
	dispatchDone := make(chan struct{})
	var managed atomic.Bool
	go func() {
		event := scriptEventDispatch{mode: coroutine.BatchAsync}
		event.withRegistrationBarrier(func() {
			managed.Store(co.IsInCoroutine())
			close(dispatchRan)
		})
		close(dispatchDone)
	}()

	select {
	case <-dispatchDone:
	case <-time.After(time.Second):
		// Release the synthetic engine call so cleanup can finish even if the
		// registration barrier regresses to a blocking Join.
		co.Update()
		t.Fatal("external event dispatch deadlocked with a main-thread call")
	}
	select {
	case <-engineCallRan:
	case <-time.After(time.Second):
		co.Update()
		t.Fatal("event dispatch did not service the pending main-thread call")
	}
	select {
	case <-dispatchRan:
	case <-time.After(time.Second):
		t.Fatal("event dispatch callback did not run")
	}
	if !managed.Load() {
		t.Fatal("event dispatch callback escaped its managed coroutine")
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
