//go:build !js
// +build !js

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
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/core/clock"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type mainThreadTestPlatform struct {
	gdx.IPlatformMgr
	main           bool
	onIsMainThread func()
}

type observedDoneContext struct {
	context.Context
	onDone func()
}

func TestRunSyncRechecksAdmissionOnEngineThread(t *testing.T) {
	scheduler := New(nil)
	previous := gdx.PlatformMgr
	t.Cleanup(func() { gdx.PlatformMgr = previous })
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	checked := false
	gdx.PlatformMgr = mainThreadTestPlatform{
		main: true,
		onIsMainThread: sync.OnceFunc(func() {
			// Close admission after RunSync's first check, before its engine call.
			scheduler.admissionMu.Lock()
			scheduler.closeAdmissionLocked()
			scheduler.admissionMu.Unlock()
			checked = true
		}),
	}
	scheduler.RunSync("rejected", func(Thread) { t.Error("shutdown admitted the callback") })
	if !checked || !scheduler.admissionClosed() {
		t.Fatal("fixture did not close admission at the engine boundary")
	}
	if scheduler.LastThreadID() != 0 || scheduler.hasPendingWork() {
		t.Fatal("RunSync created a task after shutdown closed admission")
	}
}

func TestRunSyncRejectedByShutdownBarrier(t *testing.T) {
	setMainThreadForTest(t, true)
	scheduler := New(nil)

	blockerStarted := make(chan struct{})
	releaseBlocker := make(chan struct{})
	blocker := scheduler.Create("blocker", func(Thread) {
		close(blockerStarted)
		<-releaseBlocker
	})
	waitForThreadSignal(t, blockerStarted, "blocking coroutine did not start")

	callbackRan := atomic.Bool{}
	dispatchDone := make(chan struct{})
	go func() {
		scheduler.RunSync("dispatcher", func(Thread) {
			callbackRan.Store(true)
		})
		close(dispatchDone)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		scheduler.threadsMu.Lock()
		count := len(scheduler.allThreads)
		scheduler.threadsMu.Unlock()
		if count >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("managed dispatcher was not registered")
		}
		time.Sleep(time.Millisecond)
	}

	stopDone := make(chan struct{})
	go func() {
		scheduler.RunAfterStop(nil)
		close(stopDone)
	}()
	waitForThreadSignal(t, blocker.Context().Done(), "shutdown did not cancel the blocker")
	select {
	case <-stopDone:
		t.Fatal("shutdown finished while the blocker was still running")
	default:
	}

	close(releaseBlocker)
	select {
	case <-dispatchDone:
	case <-time.After(time.Second):
		t.Fatal("rejected managed dispatcher did not clean up")
	}
	if callbackRan.Load() {
		t.Fatal("managed dispatcher callback ran after shutdown admission closed")
	}
	select {
	case <-blocker.finished.done:
	case <-time.After(time.Second):
		t.Fatal("blocking coroutine did not finish")
	}
	waitForThreadSignal(t, stopDone, "shutdown did not finish after the blocker exited")
	if scheduler.hasPendingWork() {
		t.Fatal("rejected managed dispatcher leaked from thread registry")
	}
}

func TestWaitMainThreadPreservesScriptSliceAcrossFrames(t *testing.T) {
	setMainThreadForTest(t, false)
	scheduler := New(nil)
	clock.Start()
	t.Cleanup(func() {
		scheduler.RunAfterStop(nil)
	})

	var mu sync.Mutex
	var trace []string
	record := func(event string) {
		mu.Lock()
		trace = append(trace, event)
		mu.Unlock()
	}
	first := scheduler.Create("A", func(Thread) {
		for range 3 {
			scheduler.WaitNextFrame()
			record("A-before")
			scheduler.WaitMainThread(func() {
				record("main")
				if scheduler.runMu.TryLock() {
					scheduler.runMu.Unlock()
					t.Error("main-thread call released the current script slice")
				}
				runtime.Gosched()
			})
			record("A-after")
		}
	})
	second := scheduler.Create("B", func(Thread) {
		for range 3 {
			scheduler.WaitNextFrame()
			record("B")
		}
	})
	joinFirstSliceAll(scheduler, []Thread{first, second})
	scheduler.Update()

	want := []string{"A-before", "main", "A-after", "B"}
	for frame := range 3 {
		clock.Update(1.0/30, 30)
		scheduler.Update()
		mu.Lock()
		got := append([]string(nil), trace...)
		trace = nil
		mu.Unlock()
		if !slices.Equal(got, want) {
			t.Fatalf("frame %d script order = %v, want %v", frame+1, got, want)
		}
	}
	scheduler.JoinAll([]Thread{first, second})
}

func TestRunBetweenScriptsServicesPendingMainThreadCall(t *testing.T) {
	scheduler := New(nil)
	queued := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	value := 0
	go func() {
		scheduler.runMu.Lock()
		defer scheduler.runMu.Unlock()
		scheduler.queueMainThreadCallForTest(nil, unblock)
		close(queued)
		<-release
		value = 42
	}()
	<-queued
	result := make(chan int, 1)
	go scheduler.RunBetweenScripts(func() { result <- value })
	select {
	case got := <-result:
		if got != 42 {
			t.Fatalf("script state = %d, want 42", got)
		}
	case <-time.After(time.Second):
		t.Fatal("frame-boundary callback deadlocked behind an engine call")
	}
}

func TestUpdateServicesMainThreadDuringStartup(t *testing.T) {
	setMainThreadForTest(t, false)
	for _, outcome := range []string{"success", "failure", "canceled"} {
		t.Run(outcome, func(t *testing.T) {
			var completed atomic.Bool
			scheduler := New(func(PanicReport) {})
			t.Cleanup(func() {
				scheduler.RunAfterStop(nil)
			})
			called := false
			scheduler.Create("startup", func(me Thread) {
				scheduler.WaitMainThread(func() {
					called = true
					if outcome == "canceled" {
						scheduler.Stop(me)
					}
				})
				if outcome == "failure" {
					panic("startup failed")
				}
				completed.Store(true)
			})

			updated := make(chan struct{})
			go func() {
				scheduler.Update()
				close(updated)
			}()
			waitForThreadSignal(t, updated, "startup's main-thread call prevented the frame from returning")
			if !called {
				t.Fatal("startup's main-thread call was not served")
			}
			if got, want := completed.Load(), outcome == "success"; got != want {
				t.Fatalf("completed = %v, want %v", got, want)
			}
		})
	}
}

func TestWaitMainThreadFastPathOnMainThread(t *testing.T) {
	setMainThreadForTest(t, true)
	scheduler := New(nil)
	called := false

	scheduler.WaitMainThread(func() {
		called = true
	})

	if !called {
		t.Fatal("WaitMainThread should execute immediately on the main thread")
	}
}

func TestWaitMainThreadNestedFastPath(t *testing.T) {
	setMainThreadForTest(t, true)
	scheduler := New(nil)
	called := false

	scheduler.WaitMainThread(func() {
		scheduler.WaitMainThread(func() { called = true })
	})

	if !called {
		t.Fatal("nested WaitMainThread call did not execute")
	}
}

func TestWaitMainThreadQueuesFromWorker(t *testing.T) {
	setMainThreadForTest(t, false)
	scheduler := New(nil)
	returned := make(chan struct{})

	go func() {
		scheduler.WaitMainThread(func() {})
		close(returned)
	}()

	waitForQueuedJob(t, scheduler)

	select {
	case <-returned:
		t.Fatal("WaitMainThread returned before the queued call ran")
	default:
	}

	scheduler.Update()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("WaitMainThread did not return after Update")
	}
}

func TestShutdownLockWaiterDoesNotPumpMainThreadJobs(t *testing.T) {
	setMainThreadForTest(t, false)
	scheduler := New(nil)
	started, release := make(chan struct{}), make(chan struct{})
	releaseBlocker := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseBlocker)
	blocker := scheduler.Create("shutdown-blocker", func(Thread) {
		close(started)
		<-release
	})
	waitForThreadSignal(t, started, "blocker did not start")
	firstDone := make(chan struct{})
	go func() {
		scheduler.RunAfterStop(nil)
		close(firstDone)
	}()
	// Cancellation proves the first shutdown acquired the gate.
	waitForThreadSignal(t, blocker.Context().Done(), "first shutdown did not acquire the gate")
	var callbackRan atomic.Bool
	callDone := make(chan struct{})
	go func() {
		scheduler.WaitMainThread(func() { callbackRan.Store(true) })
		close(callDone)
	}()
	waitForQueuedJob(t, scheduler)
	secondStarted, secondDone := make(chan struct{}), make(chan struct{})
	go func() {
		close(secondStarted)
		scheduler.RunAfterStop(nil)
		close(secondDone)
	}()
	waitForThreadSignal(t, secondStarted, "second shutdown did not start")
	select {
	case <-secondDone:
		t.Fatal("second shutdown bypassed the held gate")
	case <-callDone:
		t.Fatal("shutdown lock waiter ran an engine job from a worker")
	case <-time.After(20 * time.Millisecond):
	}
	if callbackRan.Load() {
		t.Fatal("shutdown lock waiter ran a main-thread job from a worker")
	}
	releaseBlocker()
	waitForThreadSignal(t, firstDone, "first shutdown did not finish")
	waitForThreadSignal(t, secondDone, "second shutdown did not finish")
	if callbackRan.Load() {
		t.Fatal("shutdown ran the engine job without a main-thread caller")
	}
	scheduler.Update()
	waitForThreadSignal(t, callDone, "queued main-thread call did not finish after Update")
	if !callbackRan.Load() {
		t.Fatal("queued main-thread call did not run on Update")
	}
}

func TestWaitMainThreadWorkerDoesNotBorrowActiveCoroutine(t *testing.T) {
	setMainThreadForTest(t, false)
	scheduler := New(nil)

	active := make(chan struct{})
	releaseActive := make(chan struct{})
	scheduler.Create("active", func(Thread) {
		close(active)
		<-releaseActive
	})
	t.Cleanup(func() {
		select {
		case <-releaseActive:
		default:
			close(releaseActive)
		}
		scheduler.RunAfterStop(nil)
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
		scheduler.WaitMainThread(func() { close(executed) })
	}()

	waitForQueuedJob(t, scheduler)

	close(releaseActive)
	scheduler.drain()
	scheduler.Update()

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
	scheduler := New(nil)
	t.Cleanup(func() {
		scheduler.RunAfterStop(nil)
	})

	var callbackRan atomic.Bool
	var continued atomic.Bool
	scheduler.Create("worker", func(Thread) {
		scheduler.WaitMainThread(func() { callbackRan.Store(true) })
		continued.Store(true)
	})

	waitForQueuedJob(t, scheduler)

	scheduler.StopAll()
	scheduler.Update()
	scheduler.drain()
	if callbackRan.Load() {
		t.Fatal("main-thread callback ran after its coroutine was canceled")
	}
	if continued.Load() {
		t.Fatal("canceled coroutine continued after WaitMainThread")
	}
}

func TestCleanupMainThreadRunsAfterQueuedCallCancellation(t *testing.T) {
	setMainThreadForTest(t, false)
	scheduler := New(nil)
	var called, cleaned, continued atomic.Bool
	thread := scheduler.Create("caller", func(Thread) {
		defer scheduler.CleanupMainThread(func() {
			cleaned.Store(true)
			defer func() {
				if got := recover(); got != ErrReentrantWait {
					t.Errorf("cleanup drain panic = %v, want %v", got, ErrReentrantWait)
				}
			}()
			scheduler.RunAfterStop(nil)
		})
		scheduler.WaitMainThread(func() { called.Store(true) })
		continued.Store(true)
	})
	waitForQueuedJob(t, scheduler)
	scheduler.Stop(thread)
	deadline := time.Now().Add(time.Second)
	for {
		select {
		case <-thread.finished.done:
			if called.Load() || continued.Load() || !cleaned.Load() {
				t.Fatalf("called=%v continued=%v cleaned=%v", called.Load(), continued.Load(), cleaned.Load())
			}
			scheduler.RunAfterStop(nil)
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("canceled cleanup did not finish")
		}
		scheduler.Update()
	}
}

func TestMainThreadCallCanceledBeforeDispatch(t *testing.T) {
	scheduler := New(nil)
	caller := scheduler.newThread("caller")
	pending := &mainThreadCall{caller: caller, done: make(chan callbackResult, 1)}
	caller.Cancel()
	if recovered := catchPanic(pending.wait); recovered != ErrAbortThread {
		t.Fatalf("canceled wait panic = %v, want %v", recovered, ErrAbortThread)
	}
	pending.call = func() { t.Error("canceled call ran after its waiter returned") }
	pending.run(scheduler)
	if len(pending.done) != 0 {
		t.Fatal("canceled call published a result after its waiter returned")
	}
}

func TestMainThreadCallCanceledBeforeClaim(t *testing.T) {
	scheduler := New(nil)
	caller := scheduler.newThread("caller")
	pending := &mainThreadCall{caller: caller, done: make(chan callbackResult, 1)}
	caller.Cancel()
	pending.call = func() { t.Error("call ran after its caller was canceled") }
	pending.run(scheduler)
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
	scheduler := New(nil)
	caller := scheduler.newThread("caller")
	defer caller.Cancel()
	waiting := make(chan struct{})
	var resultReady atomic.Bool
	caller.ctx = &observedDoneContext{Context: caller.Context(), onDone: func() {
		if resultReady.Load() {
			caller.cancelFunc()
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
	scheduler := New(nil)
	entered, release, updated := make(chan struct{}), make(chan struct{}), make(chan struct{})
	releaseCall := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		releaseCall()
		scheduler.RunAfterStop(nil)
		waitForThreadSignal(t, updated, "Update did not return")
	})
	var continued atomic.Bool
	thread := scheduler.Create("caller", func(Thread) {
		scheduler.WaitMainThread(func() {
			close(entered)
			<-release
		})
		continued.Store(true)
	})
	go func() {
		scheduler.Update()
		close(updated)
	}()
	waitForThreadSignal(t, entered, "main-thread call did not start")
	scheduler.Stop(thread)
	reset, drained := make(chan struct{}), make(chan struct{})
	go func() {
		scheduler.RunAfterStop(func() { close(reset) })
		close(drained)
	}()
	select {
	case <-reset:
		t.Error("shutdown cleanup ran before the engine call finished")
	case <-drained:
		t.Error("shutdown returned before the engine call finished")
	case <-time.After(20 * time.Millisecond):
	}

	if scheduler.runMu.TryLock() {
		scheduler.runMu.Unlock()
		t.Error("cancellation released script execution before the engine call finished")
	}
	releaseCall()
	waitForThreadSignal(t, updated, "Update did not return after the call finished")
	waitForThreadSignal(t, drained, "shutdown did not finish after the engine call")
	waitForThreadSignal(t, reset, "shutdown did not run its cleanup")
	if continued.Load() {
		t.Fatal("canceled caller continued after its engine call")
	}
}

func TestWaitMainThreadCancellationDiscardsRunningCallResult(t *testing.T) {
	setMainThreadForTest(t, false)
	var reported atomic.Bool
	scheduler := New(func(PanicReport) { reported.Store(true) })
	thread := scheduler.Create("caller", func(me Thread) {
		scheduler.WaitMainThread(func() {
			scheduler.Stop(me)
			panic("canceled result")
		})
		t.Error("canceled caller continued")
	})
	scheduler.Update()
	waitForThreadSignal(t, thread.finished.done, "canceled caller did not finish")
	scheduler.RunAfterStop(nil)
	if reported.Load() {
		t.Fatal("canceled call published its panic result")
	}
}

func TestWaitMainThreadRejectsSynchronousDrain(t *testing.T) {
	setMainThreadForTest(t, false)
	scheduler := New(nil)
	guarded := make(chan any, 1)
	thread := scheduler.Create("caller", func(Thread) {
		scheduler.WaitMainThread(func() {
			defer func() { guarded <- recover() }()
			scheduler.RunAfterStop(nil)
		})
	})
	scheduler.Update()
	waitForThreadSignal(t, thread.finished.done, "caller did not finish")
	if recovered := <-guarded; recovered != ErrReentrantWait {
		t.Fatalf("synchronous drain panic = %v", recovered)
	}
	if thread.Stopped() {
		t.Fatal("rejected drain canceled its own caller")
	}
	scheduler.RunAfterStop(nil)
}

func TestWaitMainThreadPropagatesCallbackPanic(t *testing.T) {
	setMainThreadForTest(t, false)
	scheduler := New(nil)
	returned := make(chan any, 1)
	go func() {
		defer func() { returned <- recover() }()
		scheduler.WaitMainThread(func() { panic("main-thread failure") })
	}()

	waitForQueuedJob(t, scheduler)
	scheduler.Update()
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
	scheduler := New(nil)
	returned := make(chan bool, 1)
	go func() {
		panicked := true
		defer func() {
			_ = recover()
			returned <- panicked
		}()
		scheduler.WaitMainThread(func() { panicWithValue(nil) })
		panicked = false
	}()

	waitForQueuedJob(t, scheduler)
	scheduler.Update()
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
	scheduler := New(nil)
	callerDone := make(chan bool, 1)
	go func() {
		returned := false
		defer func() {
			_ = recover()
			callerDone <- returned
		}()
		scheduler.WaitMainThread(runtime.Goexit)
		returned = true
	}()

	waitForQueuedJob(t, scheduler)

	updateDone := make(chan bool, 1)
	go func() {
		returned := false
		defer func() { updateDone <- returned }()
		scheduler.Update()
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
			scheduler := New(nil)
			cleaned := false
			cleanup := func() {
				if got := catchPanic(func() { scheduler.RunAfterStop(nil) }); got != ErrReentrantWait {
					t.Errorf("pumped cleanup drain panic = %v, want %v", got, ErrReentrantWait)
				}
				cleaned = true
			}
			var first <-chan struct{}
			if !test.holdGate {
				latch := scheduler.NewLatch()
				thread := scheduler.Create("cleanup-owner", func(Thread) {
					defer scheduler.CleanupMainThread(cleanup)
					latch.Wait()
				})
				waitForThreadSignal(t, thread.firstSlice.done, "cleanup owner did not suspend")
				scheduler.Stop(thread)
			} else {
				result := make(chan struct{})
				first = result
				go func() {
					scheduler.RunAfterStop(func() {
						scheduler.CleanupMainThread(cleanup)
					})
					close(result)
				}()
			}
			waitForQueuedJob(t, scheduler)

			setMainThreadForTest(t, true)
			frame, round := clock.Frame(), scheduler.ScriptRound()
			scheduler.RunAfterStop(func() {
				if !cleaned {
					t.Error("shutdown callback ran before engine cleanup")
				}
			})
			if first != nil {
				waitForThreadSignal(t, first, "first shutdown did not finish")
			}
			if clock.Frame() != frame || scheduler.ScriptRound() != round {
				t.Fatal("shutdown advanced a frame or script round while serving engine cleanup")
			}
		})
	}
}

// A cleanup can exit the engine goroutine while shutdown is still draining.
// Gate ownership and callback scope must unwind without reopening admission.

func TestShutdownCleanupGoexitKeepsAdmissionClosedUntilRecovery(t *testing.T) {
	setMainThreadForTest(t, false)
	scheduler := New(nil)
	workerStarted, releaseWorker := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseWorker) })
	t.Cleanup(release)
	worker := scheduler.Create("pending-worker", func(Thread) {
		scheduler.WaitToDo(func() {
			close(workerStarted)
			<-releaseWorker
		})
	})
	waitForThreadSignal(t, workerStarted, "worker did not start")
	cleanupStarted := make(chan struct{})
	cleanupOwner := scheduler.Create("cleanup-goexit", func(me Thread) {
		defer scheduler.CleanupMainThread(runtime.Goexit)
		close(cleanupStarted)
		<-me.Context().Done()
	})
	waitForThreadSignal(t, cleanupStarted, "cleanup owner did not start")
	scheduler.Stop(cleanupOwner)
	waitForQueuedJob(t, scheduler)
	setMainThreadForTest(t, true)
	exited := make(chan callbackScope, 1)
	go func() {
		defer func() { exited <- scheduler.callbackScope() }()
		scheduler.RunAfterStop(func() { t.Error("shutdown ran cleanup after the engine goroutine exited") })
		t.Error("shutdown returned normally after its pumped call used Goexit")
	}()
	select {
	case scope := <-exited:
		if scope != 0 {
			t.Fatalf("interrupted shutdown left callback scope %v registered", scope)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not pump the Goexit cleanup")
	}
	waitForThreadSignal(t, cleanupOwner.finished.done, "cleanup owner did not finish")
	waitForThreadSignal(t, worker.finished.done, "shutdown did not stop the worker's caller")
	if len(scheduler.shutdownGate) != 1 {
		t.Fatal("interrupted shutdown did not release its gate")
	}
	if !scheduler.hasPendingWork() {
		t.Fatal("test precondition failed: the native worker must still be running")
	}
	assertShutdownRejectsCreation(t, scheduler)
	// Even StopAll must preserve the interrupted drain's closed admission.
	scheduler.StopAll()
	assertShutdownRejectsCreation(t, scheduler)
	release()
	scheduler.drain()
	assertShutdownRejectsCreation(t, scheduler)
	recovered := make(chan struct{})
	go func() {
		scheduler.RunAfterStop(func() {
			if !scheduler.admissionClosed() {
				t.Error("recovery cleanup ran with admission open")
			}
		})
		close(recovered)
	}()
	waitForThreadSignal(t, recovered, "later shutdown did not recover the interrupted drain")
	if scheduler.admissionClosed() || scheduler.hasPendingWork() {
		t.Fatal("successful recovery did not reopen admission and empty the registry")
	}
	fresh := scheduler.Create("after-recovery", func(Thread) {})
	waitForThreadSignal(t, fresh.finished.done, "fresh script did not finish")
	if fresh.Stopped() {
		t.Fatal("successful recovery rejected a fresh script")
	}
}

func (p mainThreadTestPlatform) IsMainThread() bool {
	if p.onIsMainThread != nil {
		p.onIsMainThread()
	}
	return p.main
}

func (c *observedDoneContext) Done() <-chan struct{} {
	c.onDone()
	return c.Context.Done()
}

func setMainThreadForTest(t *testing.T, main bool) {
	t.Helper()
	previous := gdx.PlatformMgr
	gdx.PlatformMgr = mainThreadTestPlatform{main: main}
	t.Cleanup(func() { gdx.PlatformMgr = previous })
}

func waitForQueuedJob(t *testing.T, scheduler *Scheduler) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for scheduler.currentJobCount() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("main-thread call was not queued")
		}
		runtime.Gosched()
	}
}
