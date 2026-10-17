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
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	itime "github.com/goplus/spx/v3/internal/time"
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

func waitForQueuedJob(t *testing.T, co *Coroutines) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for co.currentJobCount() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("main-thread call was not queued")
		}
		runtime.Gosched()
	}
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
	returned := make(chan struct{})

	go func() {
		co.WaitMainThread(func() {})
		close(returned)
	}()

	waitForQueuedJob(t, co)

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

	started := make(chan struct{})
	release := make(chan struct{})
	blocker := co.Create("shutdown-blocker", func(Thread) {
		close(started)
		<-release
	})
	<-started

	barrierLocked := make(chan struct{})
	barrierDone := make(chan bool, 1)
	go func() {
		selected, err := co.RunAfterStopIf(context.Background(), func() bool {
			close(barrierLocked)
			return true
		}, nil)
		barrierDone <- selected && err == nil
	}()
	<-barrierLocked

	var callbackRan atomic.Bool
	callDone := make(chan struct{})
	go func() {
		co.WaitMainThread(func() { callbackRan.Store(true) })
		close(callDone)
	}()
	waitForQueuedJob(t, co)

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := co.RunAfterStop(timeoutCtx, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("barrier wait error = %v, want context.DeadlineExceeded", err)
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
	waitForDrainResult(t, barrierDone)
	co.Update()
	waitForThreadSignal(t, callDone, "queued main-thread call did not finish after Update")
	if !callbackRan.Load() {
		t.Fatal("queued main-thread call did not run on Update")
	}
}

func TestWaitMainThreadWorkerDoesNotBorrowActiveCoroutine(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)

	active := make(chan struct{})
	releaseActive := make(chan struct{})
	co.Create("active", func(Thread) {
		close(active)
		<-releaseActive
	})
	t.Cleanup(func() {
		select {
		case <-releaseActive:
		default:
			close(releaseActive)
		}
		if co.RunAfterStop(drainTestContext(t), nil) != nil {
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

	waitForQueuedJob(t, co)

	close(releaseActive)
	if co.drain(drainTestContext(t)) != nil {
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
	t.Cleanup(func() {
		if co.RunAfterStop(drainTestContext(t), nil) != nil {
			t.Error("coroutines did not stop during cleanup")
		}
	})

	var callbackRan atomic.Bool
	var continued atomic.Bool
	co.Create("worker", func(Thread) {
		co.WaitMainThread(func() { callbackRan.Store(true) })
		continued.Store(true)
	})

	waitForQueuedJob(t, co)

	co.StopAll()
	co.Update()
	if co.drain(drainTestContext(t)) != nil {
		t.Fatal("canceled coroutine did not stop")
	}
	if callbackRan.Load() {
		t.Fatal("main-thread callback ran after its coroutine was canceled")
	}
	if continued.Load() {
		t.Fatal("canceled coroutine continued after WaitMainThread")
	}
}

func TestCleanupMainThreadRunsAfterQueuedCallCancellation(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	var called, cleaned, continued atomic.Bool
	thread := co.Create("caller", func(Thread) {
		defer co.CleanupMainThread(func() {
			cleaned.Store(true)
			defer func() {
				if got := recover(); got != ErrReentrantWait {
					t.Errorf("cleanup drain panic = %v, want %v", got, ErrReentrantWait)
				}
			}()
			co.RunAfterStop(context.Background(), nil)
		})
		co.WaitMainThread(func() { called.Store(true) })
		continued.Store(true)
	})
	waitForQueuedJob(t, co)
	co.Stop(thread)
	deadline := time.Now().Add(time.Second)
	for {
		select {
		case <-thread.done:
			if called.Load() || continued.Load() || !cleaned.Load() {
				t.Fatalf("called=%v continued=%v cleaned=%v", called.Load(), continued.Load(), cleaned.Load())
			}
			if co.RunAfterStop(drainTestContext(t), nil) != nil {
				t.Fatal("cleanup did not drain")
			}
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("canceled cleanup did not finish")
		}
		co.Update()
	}
}

func TestMainThreadCallCanceledBeforeDispatch(t *testing.T) {
	co := New(nil)
	caller := co.newThread("caller")
	pending := &mainThreadCall{caller: caller, done: make(chan callbackResult, 1)}
	caller.Cancel()
	if recovered := catchPanic(pending.wait); recovered != ErrAbortThread {
		t.Fatalf("canceled wait panic = %v, want %v", recovered, ErrAbortThread)
	}
	pending.run(co, func() { t.Error("canceled call ran after its waiter returned") })
	if len(pending.done) != 0 {
		t.Fatal("canceled call published a result after its waiter returned")
	}
}

func TestMainThreadCallCanceledBeforeClaim(t *testing.T) {
	co := New(nil)
	caller := co.newThread("caller")
	pending := &mainThreadCall{caller: caller, done: make(chan callbackResult, 1)}
	caller.Cancel()
	pending.run(co, func() { t.Error("call ran after its caller was canceled") })
	select {
	case result := <-pending.done:
		if result.panicked {
			t.Fatal("skipped call published a panic")
		}
	default:
		t.Fatal("skipped call did not release its waiter")
	}
}

func TestMainThreadCallRechecksCancellationAfterResult(t *testing.T) {
	co := New(nil)
	caller := co.newThread("caller")
	defer caller.Cancel()
	waiting := make(chan struct{})
	var resultReady atomic.Bool
	caller.ctx = &observedDoneContext{Context: caller.Context(), onDone: func() {
		if resultReady.Load() {
			caller.cancelContext()
		} else {
			close(waiting)
		}
	}}
	pending := &mainThreadCall{caller: caller, done: make(chan callbackResult, 1)}
	returned := make(chan any, 1)
	go func() { returned <- catchPanic(pending.wait) }()
	waitForThreadSignal(t, waiting, "main-thread waiter did not start")
	// Deliver a result before cancellation, then cancel when the waiter rechecks.
	resultReady.Store(true)
	pending.done <- callbackResult{panicked: true, panicValue: "stale result"}
	select {
	case recovered := <-returned:
		if recovered != ErrAbortThread {
			t.Fatalf("completed canceled call panic = %v, want %v", recovered, ErrAbortThread)
		}
	case <-time.After(time.Second):
		t.Fatal("completed main-thread call did not return")
	}
}

func TestWaitMainThreadCancellationWaitsForRunningCall(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	entered, release, updated := make(chan struct{}), make(chan struct{}), make(chan struct{})
	releaseCall := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		releaseCall()
		if co.RunAfterStop(drainTestContext(t), nil) != nil {
			t.Error("main-thread call did not drain")
		}
		waitForThreadSignal(t, updated, "Update did not return")
	})
	var continued atomic.Bool
	thread := co.Create("caller", func(Thread) {
		co.WaitMainThread(func() {
			close(entered)
			<-release
		})
		continued.Store(true)
	})
	go func() {
		co.Update()
		close(updated)
	}()
	waitForThreadSignal(t, entered, "main-thread call did not start")
	co.Stop(thread)
	reset := false
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := co.RunAfterStop(timeoutCtx, func() { reset = true })
	if !errors.Is(err, context.DeadlineExceeded) || reset {
		t.Errorf("running engine call drain error = %v, reset = %v", err, reset)
	}
	if co.runMu.TryLock() {
		co.runMu.Unlock()
		t.Error("cancellation released script execution before the engine call finished")
	}
	releaseCall()
	waitForThreadSignal(t, updated, "Update did not return after the call finished")
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
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
	thread := co.Create("caller", func(me Thread) {
		co.WaitMainThread(func() {
			co.Stop(me)
			panic("canceled result")
		})
		t.Error("canceled caller continued")
	})
	co.Update()
	waitForThreadSignal(t, thread.done, "canceled caller did not finish")
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
		t.Fatal("canceled call did not drain")
	}
	if reported.Load() {
		t.Fatal("canceled call published its panic result")
	}
}

func TestWaitMainThreadRejectsSynchronousDrain(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	guarded := make(chan any, 1)
	thread := co.Create("caller", func(Thread) {
		co.WaitMainThread(func() {
			defer func() { guarded <- recover() }()
			co.RunAfterStop(context.Background(), nil)
		})
	})
	co.Update()
	waitForThreadSignal(t, thread.done, "caller did not finish")
	if recovered := <-guarded; recovered != ErrReentrantWait {
		t.Fatalf("synchronous drain panic = %v", recovered)
	}
	if thread.Stopped() {
		t.Fatal("rejected drain canceled its own caller")
	}
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
		t.Fatal("external drain did not recover after the callback returned")
	}
}

func TestWaitMainThreadPropagatesCallbackPanic(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	returned := make(chan any, 1)
	go func() {
		defer func() { returned <- recover() }()
		co.WaitMainThread(func() { panic("main-thread failure") })
	}()

	waitForQueuedJob(t, co)
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

	waitForQueuedJob(t, co)
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

	waitForQueuedJob(t, co)

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

func TestShutdownPumpsCleanupWithoutAdvancingFrames(t *testing.T) {
	for _, test := range []struct {
		name     string
		holdGate bool
	}{
		{"drain", false},
		{"gate", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			setMainThreadForTest(t, false)
			co := New(nil)
			cleaned := false
			cleanup := func() { cleaned = true }
			var first <-chan error
			if !test.holdGate {
				latch := co.NewLatch()
				thread := co.Create("cleanup-owner", func(Thread) {
					defer co.CleanupMainThread(cleanup)
					latch.Wait()
				})
				waitForThreadSignal(t, thread.firstSliceDone, "cleanup owner did not suspend")
				co.Stop(thread)
			} else {
				result := make(chan error, 1)
				first = result
				go func() {
					result <- co.RunAfterStop(context.Background(), func() {
						co.CleanupMainThread(cleanup)
					})
				}()
			}
			waitForQueuedJob(t, co)

			setMainThreadForTest(t, true)
			frame, round := itime.Frame(), co.ScriptRound()
			if err := co.RunAfterStop(drainTestContext(t), func() {
				if !cleaned {
					t.Error("shutdown callback ran before engine cleanup")
				}
			}); err != nil {
				t.Fatalf("main-thread shutdown failed to serve cleanup: %v", err)
			}
			if first != nil {
				if err := waitForShutdown(t, first); err != nil {
					t.Fatalf("first barrier failed: %v", err)
				}
			}
			if itime.Frame() != frame || co.ScriptRound() != round {
				t.Fatal("shutdown advanced a frame or script round while serving engine cleanup")
			}
		})
	}
}
