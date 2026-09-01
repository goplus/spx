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
	"github.com/visualfc/gid"
)

type mainThreadTestPlatform struct {
	gdx.IPlatformMgr
	main bool
}

func (p mainThreadTestPlatform) IsMainThread() bool {
	return p.main
}

type goroutineMainThreadTestPlatform struct {
	gdx.IPlatformMgr
	mainID atomic.Uint64
}

type blockingMainThreadTestPlatform struct {
	gdx.IPlatformMgr
	block   atomic.Bool
	entered chan struct{}
	release chan struct{}
}

func (p *blockingMainThreadTestPlatform) IsMainThread() bool {
	if p.block.CompareAndSwap(true, false) {
		close(p.entered)
		<-p.release
	}
	return true
}

func (p *goroutineMainThreadTestPlatform) IsMainThread() bool {
	return p.mainID.Load() == gid.Get()
}

func setMainThreadForTest(t *testing.T, main bool) {
	t.Helper()
	previous := gdx.PlatformMgr
	gdx.PlatformMgr = mainThreadTestPlatform{main: main}
	t.Cleanup(func() { gdx.PlatformMgr = previous })
}

func setGoroutineMainThreadForTest(t *testing.T) *goroutineMainThreadTestPlatform {
	t.Helper()
	previous := gdx.PlatformMgr
	manager := new(goroutineMainThreadTestPlatform)
	gdx.PlatformMgr = manager
	t.Cleanup(func() { gdx.PlatformMgr = previous })
	return manager
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

func TestWaitMainThreadFastPathCancellationBeforeAdmissionDropsCall(t *testing.T) {
	previous := gdx.PlatformMgr
	platform := &blockingMainThreadTestPlatform{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	platform.block.Store(true)
	gdx.PlatformMgr = platform
	t.Cleanup(func() { gdx.PlatformMgr = previous })

	co := New(nil)
	co.OnInited()
	var callbackRan atomic.Bool
	var continued atomic.Bool
	thread := co.Create("worker", func(Thread) int {
		co.WaitMainThread(func() { callbackRan.Store(true) })
		continued.Store(true)
		return 0
	})

	waitForThreadSignal(t, platform.entered, "fast path did not reach main-thread admission")
	co.Stop(thread)
	close(platform.release)
	if !co.waitForThreadsToStop(time.Second, nil) {
		t.Fatal("canceled fast-path coroutine did not stop")
	}
	if callbackRan.Load() {
		t.Fatal("fast-path callback ran after cancellation won admission")
	}
	if continued.Load() {
		t.Fatal("canceled fast-path coroutine continued after WaitMainThread")
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

func TestWaitMainThreadQueuedCallWakesWhenAdmissionObservesCancellation(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	var callbackRan atomic.Bool
	var continued atomic.Bool
	thread := co.Create("worker", func(Thread) int {
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

	// Model cancellation after processWaitJob's precheck but before Call. Marking
	// the atomic stop bit does not wake the waiter, so admittedCall deterministically
	// owns Pending -> Canceled and therefore must close done itself.
	co.schedulerMu.Lock()
	job := co.currentJobs.PopFront()
	co.schedulerMu.Unlock()
	thread.stopped.Store(true)
	job.Call()

	if !co.waitForThreadsToStop(time.Second, nil) {
		t.Fatal("admission-side cancellation left WaitMainThread blocked")
	}
	if callbackRan.Load() {
		t.Fatal("canceled main-thread callback ran")
	}
	if continued.Load() {
		t.Fatal("canceled coroutine continued after WaitMainThread")
	}
}

func TestWaitMainThreadCancellationWaitsForAdmittedCall(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	t.Cleanup(func() {
		if !co.AbortAllAndWait(time.Second) {
			t.Error("coroutines did not stop during cleanup")
		}
	})

	callbackStarted := make(chan struct{})
	releaseCallback := make(chan struct{})
	callbackDone := make(chan struct{})
	var continued atomic.Bool
	thread := co.Create("worker", func(Thread) int {
		co.WaitMainThread(func() {
			close(callbackStarted)
			<-releaseCallback
			close(callbackDone)
		})
		continued.Store(true)
		return 0
	})

	updateDone := make(chan struct{})
	go func() {
		co.Update()
		close(updateDone)
	}()
	select {
	case <-callbackStarted:
	case <-time.After(time.Second):
		t.Fatal("main-thread callback was not admitted")
	}
	co.Stop(thread)
	select {
	case <-thread.done:
		t.Fatal("canceled coroutine finalized while its admitted callback was running")
	default:
	}

	close(releaseCallback)
	if !co.waitForThreadsToStop(time.Second, nil) {
		t.Fatal("canceled coroutine did not stop after its callback completed")
	}
	select {
	case <-updateDone:
	case <-time.After(time.Second):
		t.Fatal("Update did not return after callback completion")
	}
	select {
	case <-callbackDone:
	default:
		t.Fatal("admitted callback did not finish before coroutine teardown")
	}
	if continued.Load() {
		t.Fatal("canceled coroutine continued after its admitted callback completed")
	}
}

func TestCanceledThreadFinalizerCanWaitMainThread(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	bodyReady := make(chan struct{})
	cleanupRan := make(chan struct{})
	thread := co.CreateWithFinalizer(
		"worker",
		func(me Thread) int {
			close(bodyReady)
			co.WaitNextFrameFor(me)
			return 0
		},
		func() {
			co.WaitMainThread(func() { close(cleanupRan) })
		},
	)
	waitForThreadSignal(t, bodyReady, "coroutine did not reach its wait")
	co.Stop(thread)

	deadline := time.Now().Add(time.Second)
	for {
		co.Update()
		select {
		case <-cleanupRan:
			if !co.waitForThreadsToStop(time.Second, nil) {
				t.Fatal("coroutine did not unregister after finalizer")
			}
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("canceled finalizer inherited cancellation in WaitMainThread")
		}
		runtime.Gosched()
	}
}

func TestStopRunawayThreadsServicesFinalizerMainThreadCall(t *testing.T) {
	platform := setGoroutineMainThreadForTest(t)
	co := New(nil)
	co.OnInited()
	bodyReady := make(chan struct{})
	cleanupRan := make(chan struct{})
	co.CreateWithFinalizer(
		"worker",
		func(me Thread) int {
			close(bodyReady)
			co.WaitNextFrameFor(me)
			return 0
		},
		func() {
			co.WaitMainThread(func() { close(cleanupRan) })
		},
	)
	waitForThreadSignal(t, bodyReady, "coroutine did not reach its wait")

	shutdownDone := make(chan struct{})
	go func() {
		platform.mainID.Store(gid.Get())
		co.stopRunawayThreads()
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("runaway shutdown deadlocked with a main-thread finalizer")
	}
	select {
	case <-cleanupRan:
	default:
		t.Fatal("runaway shutdown returned before main-thread finalizer cleanup")
	}
}

func TestRunSynchronizedCleanupServicesBlockedMainThreadCall(t *testing.T) {
	platform := setGoroutineMainThreadForTest(t)
	co := New(nil)
	co.OnInited()
	callbackRan := make(chan struct{})
	cleanupRan := make(chan struct{})
	co.Create("worker", func(Thread) int {
		co.WaitMainThread(func() { close(callbackRan) })
		return 0
	})

	deadline := time.Now().Add(time.Second)
	for co.currentJobs.Count() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("managed WaitMainThread did not enqueue its call")
		}
		runtime.Gosched()
	}

	cleanupDone := make(chan struct{})
	go func() {
		platform.mainID.Store(gid.Get())
		co.RunSynchronizedCleanup(func() { close(cleanupRan) })
		close(cleanupDone)
	}()
	select {
	case <-cleanupDone:
	case <-time.After(time.Second):
		t.Fatal("main-thread cleanup deadlocked behind a queued main-thread call")
	}
	select {
	case <-callbackRan:
	default:
		t.Fatal("cleanup acquired runMu without servicing the queued main-thread call")
	}
	select {
	case <-cleanupRan:
	default:
		t.Fatal("synchronized cleanup did not run")
	}
}
