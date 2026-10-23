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
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	itime "github.com/goplus/spx/v4/internal/time"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
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

type observedDoneContext struct {
	context.Context
	onDone func()
}

func (c *observedDoneContext) Done() <-chan struct{} {
	c.onDone()
	return c.Context.Done()
}

func TestTryRunFromEngineRejectedByShutdownBarrier(t *testing.T) {
	setMainThreadForTest(t, true)
	co := New(nil)

	blockerStarted := make(chan struct{})
	releaseBlocker := make(chan struct{})
	blocker := co.Create("blocker", func(Thread) {
		close(blockerStarted)
		<-releaseBlocker
	})
	waitForThreadSignal(t, blockerStarted, "blocking coroutine did not start")

	callbackRan := atomic.Bool{}
	dispatchDone := make(chan struct{})
	go func() {
		co.TryRunFromEngine("dispatcher", func() {
			callbackRan.Store(true)
		})
		close(dispatchDone)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		co.threadsMu.Lock()
		count := len(co.allThreads)
		co.threadsMu.Unlock()
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
		co.RunAfterStop(nil)
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
	case <-blocker.done:
	case <-time.After(time.Second):
		t.Fatal("blocking coroutine did not finish")
	}
	waitForThreadSignal(t, stopDone, "shutdown did not finish after the blocker exited")
	if co.hasPendingWork() {
		t.Fatal("rejected managed dispatcher leaked from thread registry")
	}
}

func TestWaitMainThreadPreservesScriptSliceAcrossFrames(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	itime.Start(nil)
	t.Cleanup(func() {
		co.RunAfterStop(nil)
	})

	var mu sync.Mutex
	var trace []string
	record := func(event string) {
		mu.Lock()
		trace = append(trace, event)
		mu.Unlock()
	}
	first := co.Create("A", func(Thread) {
		for range 3 {
			co.WaitNextFrame()
			record("A-before")
			co.WaitMainThread(func() {
				record("main")
				if co.runMu.TryLock() {
					co.runMu.Unlock()
					t.Error("main-thread call released the current script slice")
				}
				runtime.Gosched()
			})
			record("A-after")
		}
	})
	second := co.Create("B", func(Thread) {
		for range 3 {
			co.WaitNextFrame()
			record("B")
		}
	})
	joinFirstSliceAll(co, []Thread{first, second})
	co.Update()

	want := []string{"A-before", "main", "A-after", "B"}
	for frame := range 3 {
		itime.Update(1.0/30, 30)
		co.Update()
		mu.Lock()
		got := append([]string(nil), trace...)
		trace = nil
		mu.Unlock()
		if !slices.Equal(got, want) {
			t.Fatalf("frame %d script order = %v, want %v", frame+1, got, want)
		}
	}
	co.JoinAll([]Thread{first, second})
}

func TestRunBetweenScriptsServicesPendingMainThreadCall(t *testing.T) {
	co := New(nil)
	queued := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	value := 0
	go func() {
		co.runMu.Lock()
		defer co.runMu.Unlock()
		co.enqueuePriorityJob(&waitJob{kind: waitMainThread, call: unblock})
		close(queued)
		<-release
		value = 42
	}()
	<-queued
	result := make(chan int, 1)
	go co.RunBetweenScripts(func() { result <- value })
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
			co := New(func(PanicReport) {})
			t.Cleanup(func() {
				co.RunAfterStop(nil)
			})
			called := false
			co.Create("startup", func(me Thread) {
				co.WaitMainThread(func() {
					called = true
					if outcome == "canceled" {
						co.Stop(me)
					}
				})
				if outcome == "failure" {
					panic("startup failed")
				}
				completed.Store(true)
			})

			updated := make(chan struct{})
			go func() {
				co.Update()
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
	started, release := make(chan struct{}), make(chan struct{})
	releaseBlocker := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseBlocker)
	blocker := co.Create("shutdown-blocker", func(Thread) {
		close(started)
		<-release
	})
	waitForThreadSignal(t, started, "blocker did not start")
	firstDone := make(chan struct{})
	go func() {
		co.RunAfterStop(nil)
		close(firstDone)
	}()
	// Cancellation proves the first shutdown acquired the gate.
	waitForThreadSignal(t, blocker.Context().Done(), "first shutdown did not acquire the gate")
	var callbackRan atomic.Bool
	callDone := make(chan struct{})
	go func() {
		co.WaitMainThread(func() { callbackRan.Store(true) })
		close(callDone)
	}()
	waitForQueuedJob(t, co)
	secondStarted, secondDone := make(chan struct{}), make(chan struct{})
	go func() {
		close(secondStarted)
		co.RunAfterStop(nil)
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
		co.RunAfterStop(nil)
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
	co.drain()
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
		co.RunAfterStop(nil)
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
	co.drain()
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
			co.RunAfterStop(nil)
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
			co.RunAfterStop(nil)
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
	co := New(nil)
	entered, release, updated := make(chan struct{}), make(chan struct{}), make(chan struct{})
	releaseCall := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		releaseCall()
		co.RunAfterStop(nil)
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
	reset, drained := make(chan struct{}), make(chan struct{})
	go func() {
		co.RunAfterStop(func() { close(reset) })
		close(drained)
	}()
	select {
	case <-reset:
		t.Error("shutdown cleanup ran before the engine call finished")
	case <-drained:
		t.Error("shutdown returned before the engine call finished")
	case <-time.After(20 * time.Millisecond):
	}

	if co.runMu.TryLock() {
		co.runMu.Unlock()
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
	co.RunAfterStop(nil)
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
			co.RunAfterStop(nil)
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
	co.RunAfterStop(nil)
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
		co.WaitMainThread(func() { panicWithValue(nil) })
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
			cleanup := func() {
				if got := catchPanic(func() { co.RunAfterStop(nil) }); got != ErrReentrantWait {
					t.Errorf("pumped cleanup drain panic = %v, want %v", got, ErrReentrantWait)
				}
				cleaned = true
			}
			var first <-chan struct{}
			if !test.holdGate {
				latch := co.NewLatch()
				thread := co.Create("cleanup-owner", func(Thread) {
					defer co.CleanupMainThread(cleanup)
					latch.Wait()
				})
				waitForThreadSignal(t, thread.firstSliceDone, "cleanup owner did not suspend")
				co.Stop(thread)
			} else {
				result := make(chan struct{})
				first = result
				go func() {
					co.RunAfterStop(func() {
						co.CleanupMainThread(cleanup)
					})
					close(result)
				}()
			}
			waitForQueuedJob(t, co)

			setMainThreadForTest(t, true)
			frame, round := itime.Frame(), co.ScriptRound()
			co.RunAfterStop(func() {
				if !cleaned {
					t.Error("shutdown callback ran before engine cleanup")
				}
			})
			if first != nil {
				waitForThreadSignal(t, first, "first shutdown did not finish")
			}
			if itime.Frame() != frame || co.ScriptRound() != round {
				t.Fatal("shutdown advanced a frame or script round while serving engine cleanup")
			}
		})
	}
}

// A cleanup can exit the engine goroutine while shutdown is still draining.
// Gate ownership and callback scope must unwind without reopening admission.

func TestShutdownCleanupGoexitKeepsAdmissionClosedUntilRecovery(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	workerStarted, releaseWorker := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseWorker) })
	t.Cleanup(release)
	worker := co.Create("pending-worker", func(Thread) {
		co.WaitToDo(func() {
			close(workerStarted)
			<-releaseWorker
		})
	})
	waitForThreadSignal(t, workerStarted, "worker did not start")
	cleanupStarted := make(chan struct{})
	cleanupOwner := co.Create("cleanup-goexit", func(me Thread) {
		defer co.CleanupMainThread(runtime.Goexit)
		close(cleanupStarted)
		<-me.Context().Done()
	})
	waitForThreadSignal(t, cleanupStarted, "cleanup owner did not start")
	co.Stop(cleanupOwner)
	waitForQueuedJob(t, co)
	setMainThreadForTest(t, true)
	exited := make(chan callbackScope, 1)
	go func() {
		defer func() { exited <- co.callbackScope() }()
		co.RunAfterStop(func() { t.Error("shutdown ran cleanup after the engine goroutine exited") })
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
	waitForThreadSignal(t, cleanupOwner.done, "cleanup owner did not finish")
	waitForThreadSignal(t, worker.done, "shutdown did not stop the worker's caller")
	if len(co.shutdownGate) != 1 {
		t.Fatal("interrupted shutdown did not release its gate")
	}
	if !co.hasPendingWork() {
		t.Fatal("test precondition failed: the native worker must still be running")
	}
	assertShutdownRejectsCreation(t, co)
	// Even StopAll must preserve the interrupted drain's closed admission.
	co.StopAll()
	assertShutdownRejectsCreation(t, co)
	release()
	co.drain()
	assertShutdownRejectsCreation(t, co)
	recovered := make(chan struct{})
	go func() {
		co.RunAfterStop(func() {
			if !co.admissionClosed() {
				t.Error("recovery cleanup ran with admission open")
			}
		})
		close(recovered)
	}()
	waitForThreadSignal(t, recovered, "later shutdown did not recover the interrupted drain")
	if co.admissionClosed() || co.hasPendingWork() {
		t.Fatal("successful recovery did not reopen admission and empty the registry")
	}
	fresh := co.Create("after-recovery", func(Thread) {})
	waitForThreadSignal(t, fresh.done, "fresh script did not finish")
	if fresh.Stopped() {
		t.Fatal("successful recovery rejected a fresh script")
	}
}
