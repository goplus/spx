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

package coroutine

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type mainThreadTestPlatform struct {
	gdx.IPlatformMgr
	main bool
}

func (p mainThreadTestPlatform) IsMainThread() bool {
	return p.main
}

func setMainThreadForTest(t *testing.T, main bool) {
	t.Helper()
	previous := gdx.PlatformMgr
	gdx.PlatformMgr = mainThreadTestPlatform{main: main}
	t.Cleanup(func() { gdx.PlatformMgr = previous })
}

func TestWaitMainThreadFastPathOnMainThread(t *testing.T) {
	setMainThreadForTest(t, true)
	co := New(nil)
	called := false

	co.WaitMainThread(func() {
		called = true
	})

	if !called {
		t.Fatal("WaitMainThread should execute immediately on the main thread")
	}
}

func TestWaitMainThreadNestedFastPath(t *testing.T) {
	setMainThreadForTest(t, true)
	co := New(nil)
	called := false

	co.WaitMainThread(func() {
		co.WaitMainThread(func() { called = true })
	})

	if !called {
		t.Fatal("nested WaitMainThread call did not execute")
	}
}

func TestWaitMainThreadQueuesFromWorker(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	returned := make(chan struct{})

	go func() {
		co.WaitMainThread(func() {})
		close(returned)
	}()

	deadline := time.Now().Add(time.Second)
	for co.currentJobs.Count() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("WaitMainThread did not enqueue worker call")
		}
		runtime.Gosched()
	}

	select {
	case <-returned:
		t.Fatal("WaitMainThread returned before the queued call ran")
	default:
	}

	co.Update()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("WaitMainThread did not return after Update")
	}
}

func TestWaitMainThreadWorkerDoesNotBorrowActiveCoroutine(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()

	active := make(chan struct{})
	releaseActive := make(chan struct{})
	co.Create("active", func(Thread) int {
		close(active)
		<-releaseActive
		return 0
	})
	t.Cleanup(func() {
		select {
		case <-releaseActive:
		default:
			close(releaseActive)
		}
		if !co.AbortAllAndWait(time.Second) {
			t.Error("coroutines did not stop during cleanup")
		}
	})
	select {
	case <-active:
	case <-time.After(time.Second):
		t.Fatal("active coroutine did not start")
	}

	executed := make(chan struct{})
	returned := make(chan any, 1)
	go func() {
		defer func() { returned <- recover() }()
		co.WaitMainThread(func() { close(executed) })
	}()

	deadline := time.Now().Add(time.Second)
	for co.currentJobs.Count() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("WaitMainThread did not enqueue worker call")
		}
		runtime.Gosched()
	}

	close(releaseActive)
	if !co.waitForThreadsToStop(time.Second, nil) {
		t.Fatal("active coroutine did not complete")
	}
	co.Update()

	select {
	case recovered := <-returned:
		if recovered != nil {
			t.Fatalf("external WaitMainThread panicked with %v", recovered)
		}
	case <-time.After(time.Second):
		t.Fatal("external WaitMainThread did not return")
	}
	select {
	case <-executed:
	default:
		t.Fatal("external main-thread callback did not run")
	}
}

func TestWaitMainThreadCanceledCoroutineDropsQueuedCall(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	t.Cleanup(func() {
		if !co.AbortAllAndWait(time.Second) {
			t.Error("coroutines did not stop during cleanup")
		}
	})

	var callbackRan atomic.Bool
	var continued atomic.Bool
	co.Create("worker", func(Thread) int {
		co.WaitMainThread(func() { callbackRan.Store(true) })
		continued.Store(true)
		return 0
	})

	deadline := time.Now().Add(time.Second)
	for co.currentJobs.Count() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("managed WaitMainThread did not enqueue its call")
		}
		runtime.Gosched()
	}

	co.AbortAll()
	co.Update()
	if !co.waitForThreadsToStop(time.Second, nil) {
		t.Fatal("canceled coroutine did not stop")
	}
	if callbackRan.Load() {
		t.Fatal("main-thread callback ran after its coroutine was canceled")
	}
	if continued.Load() {
		t.Fatal("canceled coroutine continued after WaitMainThread")
	}
}

func TestWaitMainThreadPropagatesCallbackPanic(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	returned := make(chan any, 1)
	go func() {
		defer func() { returned <- recover() }()
		co.WaitMainThread(func() { panic("main-thread failure") })
	}()

	deadline := time.Now().Add(time.Second)
	for co.currentJobs.Count() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("WaitMainThread did not enqueue worker call")
		}
		runtime.Gosched()
	}
	co.Update()
	select {
	case recovered := <-returned:
		if recovered != "main-thread failure" {
			t.Fatalf("panic = %v, want callback panic", recovered)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitMainThread caller remained blocked after callback panic")
	}
}
