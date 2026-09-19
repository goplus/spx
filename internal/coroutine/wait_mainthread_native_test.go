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
	"sync"
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

func TestShutdownLockWaiterDoesNotPumpMainThreadJobs(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()

	started := make(chan struct{})
	release := make(chan struct{})
	blocker := co.CreateAndStart("shutdown-blocker", func(Thread) int {
		close(started)
		<-release
		return 0
	})
	<-started

	barrierLocked := make(chan struct{})
	barrierDone := make(chan bool, 1)
	go func() {
		selected, completed := co.RunAfterStopAllIf(0, func() bool {
			close(barrierLocked)
			return true
		}, nil)
		barrierDone <- selected && completed
	}()
	<-barrierLocked

	var callbackRan atomic.Bool
	callDone := make(chan struct{})
	go func() {
		co.WaitMainThread(func() { callbackRan.Store(true) })
		close(callDone)
	}()
	deadline := time.Now().Add(time.Second)
	for co.currentJobs.Count() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("WaitMainThread did not enqueue worker call")
		}
		runtime.Gosched()
	}

	if co.RunAfterStopAll(20*time.Millisecond, nil) {
		t.Fatal("second barrier completed while the first owned the lock")
	}
	if callbackRan.Load() {
		t.Fatal("shutdown lock waiter ran a main-thread job from a worker")
	}
	select {
	case <-callDone:
		t.Fatal("WaitMainThread returned before an engine update")
	default:
	}

	close(release)
	co.Join(blocker)
	select {
	case completed := <-barrierDone:
		if !completed {
			t.Fatal("first barrier did not complete")
		}
	case <-time.After(time.Second):
		t.Fatal("first barrier did not finish after drain")
	}
	co.Update()
	select {
	case <-callDone:
	case <-time.After(time.Second):
		t.Fatal("queued main-thread call did not finish after Update")
	}
	if !callbackRan.Load() {
		t.Fatal("queued main-thread call did not run on Update")
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
		if !co.StopAllAndWait(time.Second) {
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
	if !co.waitForDrain(time.Second, nil) {
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
		if !co.StopAllAndWait(time.Second) {
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

	co.StopAll()
	co.Update()
	if !co.waitForDrain(time.Second, nil) {
		t.Fatal("canceled coroutine did not stop")
	}
	if callbackRan.Load() {
		t.Fatal("main-thread callback ran after its coroutine was canceled")
	}
	if continued.Load() {
		t.Fatal("canceled coroutine continued after WaitMainThread")
	}
}

func TestWaitMainThreadCancellationWaitsForRunningCall(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	entered, release, updated := make(chan struct{}), make(chan struct{}), make(chan struct{})
	releaseCall := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		releaseCall()
		if !co.RunAfterStopAll(time.Second, nil) {
			t.Error("main-thread call did not drain")
		}
		waitForThreadSignal(t, updated, "Update did not return")
	})
	var continued atomic.Bool
	thread := co.Create("caller", func(Thread) int {
		co.WaitMainThread(func() {
			close(entered)
			<-release
		})
		continued.Store(true)
		return 0
	})
	go func() {
		co.Update()
		close(updated)
	}()
	waitForThreadSignal(t, entered, "main-thread call did not start")
	co.Stop(thread)
	reset := false
	completed := co.RunAfterStopAll(20*time.Millisecond, func() { reset = true })
	if completed || reset {
		t.Error("reset ran while the main-thread call was executing")
	}
	if co.runMu.TryLock() {
		co.runMu.Unlock()
		t.Error("cancellation released script execution before the engine call finished")
	}
	releaseCall()
	waitForThreadSignal(t, updated, "Update did not return after the call finished")
	if !co.RunAfterStopAll(time.Second, nil) {
		t.Fatal("completed main-thread call did not drain")
	}
	if continued.Load() {
		t.Fatal("canceled caller continued after its engine call")
	}
}

func TestWaitMainThreadCancellationDiscardsRunningCallResult(t *testing.T) {
	setMainThreadForTest(t, false)
	var reported atomic.Bool
	co := New(func(PanicReport) { reported.Store(true) })
	co.OnInited()
	thread := co.Create("caller", func(me Thread) int {
		co.WaitMainThread(func() {
			co.Stop(me)
			panic("canceled result")
		})
		t.Error("canceled caller continued")
		return 0
	})
	co.Update()
	waitForThreadSignal(t, thread.done, "canceled caller did not finish")
	if !co.RunAfterStopAll(time.Second, nil) {
		t.Fatal("canceled call did not drain")
	}
	if reported.Load() {
		t.Fatal("canceled call published its panic result")
	}
}

func TestWaitMainThreadRejectsSynchronousDrain(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	guarded := make(chan any, 1)
	thread := co.Create("caller", func(Thread) int {
		co.WaitMainThread(func() {
			defer func() { guarded <- recover() }()
			co.StopAllAndWait(time.Millisecond)
		})
		return 0
	})
	co.Update()
	waitForThreadSignal(t, thread.done, "caller did not finish")
	if recovered := <-guarded; recovered != ErrReentrantWait {
		t.Fatalf("synchronous drain panic = %v", recovered)
	}
	if thread.Stopped() {
		t.Fatal("rejected drain canceled its own caller")
	}
	if !co.RunAfterStopAll(time.Second, nil) {
		t.Fatal("external drain did not recover after the callback returned")
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

func TestWaitMainThreadPropagatesNilCallbackPanic(t *testing.T) {
	t.Setenv("GODEBUG", "panicnil=1")
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	returned := make(chan bool, 1)
	go func() {
		panicked := true
		defer func() {
			_ = recover()
			returned <- panicked
		}()
		co.WaitMainThread(func() { panic(nil) })
		panicked = false
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
	case panicked := <-returned:
		if !panicked {
			t.Fatal("WaitMainThread returned normally after a nil callback panic")
		}
	case <-time.After(time.Second):
		t.Fatal("WaitMainThread caller remained blocked after nil callback panic")
	}
}

func TestWaitMainThreadGoexitReleasesExternalCaller(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	callerDone := make(chan bool, 1)
	go func() {
		returned := false
		defer func() {
			_ = recover()
			callerDone <- returned
		}()
		co.WaitMainThread(runtime.Goexit)
		returned = true
	}()

	deadline := time.Now().Add(time.Second)
	for co.currentJobs.Count() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("WaitMainThread did not enqueue worker call")
		}
		runtime.Gosched()
	}

	updateDone := make(chan bool, 1)
	go func() {
		returned := false
		defer func() { updateDone <- returned }()
		co.Update()
		returned = true
	}()
	select {
	case returned := <-updateDone:
		if returned {
			t.Fatal("callback Goexit did not exit the Update goroutine")
		}
	case <-time.After(time.Second):
		t.Fatal("Update goroutine did not exit")
	}
	select {
	case returned := <-callerDone:
		if !returned {
			t.Fatal("external caller did not return normally after callback Goexit")
		}
	case <-time.After(time.Second):
		t.Fatal("external caller remained blocked after callback Goexit")
	}
}
