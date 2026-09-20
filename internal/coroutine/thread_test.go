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

	"github.com/visualfc/gid"
)

type namedThreadObj struct{}

func (*namedThreadObj) Name() string {
	return "named-thread"
}

type invalidNamedThreadObj struct{}

func (*invalidNamedThreadObj) Name(_ string) string {
	return "invalid"
}

func canReenterCoroutineMutex(co *Coroutines) bool {
	if !co.threadsMu.TryLock() {
		return false
	}
	co.threadsMu.Unlock()
	return true
}

func TestResolveThreadNameUsesNameMethodWithoutReflectionCall(t *testing.T) {
	if got := resolveThreadName(&namedThreadObj{}); got != "named-thread" {
		t.Fatalf("expected name from interface method, got %q", got)
	}
}

func TestResolveThreadNameFallsBackForInvalidNameSignature(t *testing.T) {
	if got := resolveThreadName(&invalidNamedThreadObj{}); got != "*invalidNamedThreadObj" {
		t.Fatalf("expected fallback type name, got %q", got)
	}
}

func TestMainExecutionScopeRestoresEnclosingExecution(t *testing.T) {
	thread := New(nil).newThread("main")
	outerStartedAt := time.Unix(10, 0)
	innerStartedAt := time.Unix(20, 0)

	endOuter := thread.BeginMain(outerStartedAt)
	endInner := thread.BeginMain(innerStartedAt)
	if got := thread.MainStartedAt(); !got.Equal(innerStartedAt) {
		t.Fatalf("inner Main started at %v, want %v", got, innerStartedAt)
	}
	thread.DisableMainTimeout()
	if got := thread.MainStartedAt(); !got.IsZero() {
		t.Fatalf("disabled Main timeout started at %v, want zero", got)
	}

	endInner()
	if got := thread.MainStartedAt(); !got.Equal(outerStartedAt) {
		t.Fatalf("restored outer Main started at %v, want %v", got, outerStartedAt)
	}
	endOuter()
	if got := thread.MainStartedAt(); !got.IsZero() {
		t.Fatalf("finished Main started at %v, want zero", got)
	}
}

func TestStopIfStopsActiveThread(t *testing.T) {
	co := New(nil)
	started := make(chan Thread, 1)
	done := make(chan struct{})

	co.CreateAndStart("worker", func(me Thread) {
		started <- me
		for {
			select {
			case <-me.Context().Done():
				close(done)
				return
			default:
				runtime.Gosched()
			}
		}
	})

	var th Thread
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case th = <-started:
	case <-timer.C:
		t.Fatal("coroutine did not start")
	}

	th.suspendMu.Lock()
	suspendState := th.suspendState
	th.suspendMu.Unlock()
	if suspendState == suspendStateSuspended {
		t.Fatal("test precondition failed: spinning thread should not be suspended")
	}

	co.StopIf(func(candidate Thread) bool {
		return candidate == th
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatal("active coroutine did not stop after StopIf")
	}
}

func waitForThreadSignal(t *testing.T, signal <-chan struct{}, failure string) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-signal:
	case <-timer.C:
		t.Fatal(failure)
	}
}

func TestStopIsIdempotentForSuspendedThread(t *testing.T) {
	co := New(nil)
	thread := co.Create("worker", func(me Thread) {
		co.Yield(me)
	})
	waitForThreadSignal(t, thread.yieldedOrDone, "coroutine did not suspend")

	co.Stop(thread)
	co.Stop(thread)
	co.Stop(nil)
	waitForThreadSignal(t, thread.done, "suspended coroutine did not stop")
	co.Stop(thread)
}

func TestStopIfEvaluatesFilterWithoutHoldingMutex(t *testing.T) {
	co := New(nil)
	started := make(chan Thread, 1)
	done := make(chan struct{})

	co.CreateAndStart("worker", func(me Thread) {
		started <- me
		for {
			select {
			case <-me.Context().Done():
				close(done)
				return
			default:
				runtime.Gosched()
			}
		}
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	var th Thread
	select {
	case th = <-started:
	case <-timer.C:
		t.Fatal("coroutine did not start")
	}

	var reacquired bool
	stopDone := make(chan struct{})
	go func() {
		co.StopIf(func(candidate Thread) bool {
			reacquired = canReenterCoroutineMutex(co)
			return candidate == th
		})
		close(stopDone)
	}()

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-stopDone:
	case <-timer.C:
		t.Fatal("StopIf deadlocked when filter re-entered the coroutine mutex")
	}
	if !reacquired {
		t.Fatal("StopIf evaluated filter while holding the coroutine mutex")
	}

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatal("active coroutine did not stop after StopIf")
	}
}

func TestStopAllAndWaitFromCoroutineWaitsForPeersWithoutWaitingForCaller(t *testing.T) {
	co := New(nil)
	result := make(chan bool, 1)
	otherYielding := make(chan struct{})
	otherDone := make(chan struct{})

	co.CreateAndStart("other", func(other Thread) {
		defer close(otherDone)
		close(otherYielding)
		co.Yield(other)
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-otherYielding:
	case <-timer.C:
		t.Fatal("other coroutine did not reach yield")
	}

	co.CreateAndStart("caller", func(me Thread) {
		result <- co.StopAllAndWait(time.Hour)
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case completed := <-result:
		if !completed {
			t.Fatal("StopAllAndWait should report success after other coroutines stop")
		}
	case <-timer.C:
		t.Fatal("StopAllAndWait did not finish after peer coroutine stopped")
	}

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-otherDone:
	case <-timer.C:
		t.Fatal("other coroutine did not exit while caller was waiting")
	}
}

func TestStopAllAndWaitFromCoroutineDoesNotStartStoppedPeer(t *testing.T) {
	co := New(nil)
	result := make(chan bool, 1)
	peerRan := make(chan struct{}, 1)

	co.CreateAndStart("caller", func(me Thread) {
		co.Create("peer", func(peer Thread) {
			close(peerRan)
		})
		result <- co.StopAllAndWait(time.Second)
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case completed := <-result:
		if !completed {
			t.Fatal("StopAllAndWait should wait for the stopped peer to unregister")
		}
	case <-timer.C:
		t.Fatal("StopAllAndWait did not finish after stopped peer unregistered")
	}

	select {
	case <-peerRan:
		t.Fatal("stopped peer coroutine should not run")
	default:
	}
}

func TestStopAllRejectsChildCreatedByCanceledOwner(t *testing.T) {
	co := New(nil)
	co.OnInited()
	childResult := make(chan Thread, 1)
	var childRan atomic.Bool

	co.CreateAndStart("parent", func(Thread) {
		co.StopAll()
		childResult <- co.Create("late-child", func(Thread) {
			childRan.Store(true)
		})
	})

	var child Thread
	select {
	case child = <-childResult:
	case <-time.After(time.Second):
		t.Fatal("canceled owner did not finish child registration")
	}
	if !child.Stopped() {
		t.Fatal("child created by a canceled owner was admitted")
	}
	if !co.waitForDrain(time.Second, nil) {
		t.Fatal("canceled owner and rejected child did not stop")
	}
	if childRan.Load() {
		t.Fatal("child created by a canceled owner ran user code")
	}
}

func TestStopAllInvalidatesPendingAdmission(t *testing.T) {
	co := New(nil)
	admission := co.captureAdmission()
	co.StopAll()

	var registered, ran bool
	thread := co.createThread(admission, Task{
		Owner: "stale",
		Setup: func(Thread) func() {
			registered = true
			return nil
		},
		Run: func(Thread) {
			ran = true
		},
	})
	waitForThreadSignal(t, thread.done, "rejected thread did not finish")
	if !thread.Stopped() || registered || ran {
		t.Fatal("an old creation request survived StopAll")
	}

	next := co.Create("fresh", func(Thread) {
		ran = true
	})
	waitForThreadSignal(t, next.done, "new thread did not finish")
	if next.Stopped() || !ran {
		t.Fatal("StopAll did not reopen admission for new requests")
	}
}

func TestStopAllAndWaitOrdersInFlightRegistration(t *testing.T) {
	co := New(nil)
	co.OnInited()
	co.admissionMu.Lock()
	registrationStarted := make(chan struct{})
	created := make(chan Thread, 1)
	go func() {
		close(registrationStarted)
		created <- co.Create("in-flight", func(Thread) {})
	}()
	select {
	case <-registrationStarted:
	case <-time.After(time.Second):
		co.admissionMu.Unlock()
		t.Fatal("registration did not start")
	}

	stopDone := make(chan bool, 1)
	go func() {
		stopDone <- co.StopAllAndWait(time.Second)
	}()
	select {
	case <-stopDone:
		co.admissionMu.Unlock()
		t.Fatal("shutdown barrier returned while an earlier registration was in flight")
	case <-time.After(20 * time.Millisecond):
	}
	co.admissionMu.Unlock()

	select {
	case completed := <-stopDone:
		if !completed {
			t.Fatal("shutdown barrier timed out after registration was released")
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown barrier did not finish")
	}
	var thread Thread
	select {
	case thread = <-created:
	case <-time.After(time.Second):
		t.Fatal("in-flight registration did not return")
	}
	select {
	case <-thread.done:
	case <-time.After(time.Second):
		t.Fatal("in-flight thread remained after the shutdown barrier")
	}
}

func TestStopAllAndWaitRejectsCreationDuringBarrier(t *testing.T) {
	co := New(nil)
	co.OnInited()
	blocker := co.newThread("barrier-blocker")
	co.registerThread(blocker)

	stopDone := make(chan bool, 1)
	go func() {
		stopDone <- co.StopAllAndWait(time.Second)
	}()
	deadline := time.Now().Add(time.Second)
	for !blocker.Stopped() {
		if time.Now().After(deadline) {
			co.removeThreadState(blocker)
			co.unregisterThread(blocker)
			t.Fatal("shutdown barrier did not start")
		}
		runtime.Gosched()
	}

	var childRan atomic.Bool
	child := co.Create("during-barrier", func(Thread) {
		childRan.Store(true)
	})
	if !child.Stopped() {
		t.Fatal("thread created during shutdown barrier was admitted")
	}
	co.removeThreadState(blocker)
	co.unregisterThread(blocker)

	select {
	case completed := <-stopDone:
		if !completed {
			t.Fatal("shutdown barrier timed out")
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown barrier did not finish after the blocker was removed")
	}
	select {
	case <-child.done:
	case <-time.After(time.Second):
		t.Fatal("rejected thread did not finish")
	}
	if childRan.Load() {
		t.Fatal("thread created during shutdown barrier ran user code")
	}
}

func TestRunAfterStopAllTimeoutRequiresExplicitRecovery(t *testing.T) {
	co := New(nil)
	co.OnInited()
	blocker := co.newThread("barrier-blocker")
	co.registerThread(blocker)

	var callbackRan atomic.Bool
	if co.RunAfterStopAll(20*time.Millisecond, func() {
		callbackRan.Store(true)
	}) {
		t.Fatal("shutdown barrier reported success with a registered blocker")
	}
	if callbackRan.Load() {
		t.Fatal("shutdown barrier ran callback after timing out")
	}

	var rejectedRan atomic.Bool
	rejected := co.CreateAndStart("during-timeout", func(Thread) {
		rejectedRan.Store(true)
	})
	if !rejected.Stopped() {
		t.Fatal("creation was admitted while a timed-out barrier still had work")
	}
	select {
	case <-rejected.done:
	case <-time.After(time.Second):
		t.Fatal("rejected creation did not finish")
	}
	if rejectedRan.Load() {
		t.Fatal("creation during a timed-out barrier ran user code")
	}

	co.removeThreadState(blocker)
	co.unregisterThread(blocker)

	stillRejected := co.CreateAndStart("after-drain-before-recovery", func(Thread) {
		t.Fatal("creation ran while the manager remained quarantined")
	})
	if !stillRejected.Stopped() {
		t.Fatal("creation was admitted after a timed-out barrier drained without recovery")
	}
	waitForThreadSignal(t, stillRejected.done, "quarantined creation did not finish")

	// Explicit recovery reopens admission after the drain.
	if !co.RunAfterStopAll(time.Second, nil) {
		t.Fatal("explicit recovery barrier did not complete after drain")
	}
	var nextRan atomic.Bool
	next := co.CreateAndStart("after-timeout", func(Thread) {
		nextRan.Store(true)
	})
	select {
	case <-next.done:
	case <-time.After(time.Second):
		t.Fatal("creation remained blocked after shutdown barrier timed out")
	}
	if !nextRan.Load() {
		t.Fatal("creation was rejected after shutdown barrier timed out")
	}
}

func TestRunAfterStopAllIfRejectsWithoutChangingState(t *testing.T) {
	co := New(nil)
	co.OnInited()
	started := make(chan struct{})
	release := make(chan struct{})
	thread := co.CreateAndStart("conditional-blocker", func(Thread) {
		close(started)
		<-release
	})
	<-started
	epoch := co.admissionEpoch.Load()
	called := false
	selected, completed := co.RunAfterStopAllIf(time.Second, func() bool {
		return false
	}, func() {
		called = true
	})
	if selected || completed {
		t.Fatalf("conditional barrier = (%v, %v), want (false, false)", selected, completed)
	}
	if called || thread.Stopped() || co.admissionClosed() {
		t.Fatalf("rejected barrier changed state: called=%v stopped=%v admissionClosed=%v", called, thread.Stopped(), co.admissionClosed())
	}
	if got := co.admissionEpoch.Load(); got != epoch {
		t.Fatalf("admission epoch = %d, want %d", got, epoch)
	}
	close(release)
	waitForThreadSignal(t, thread.done, "conditional blocker did not finish")
}

func TestRunAfterStopAllIfPredicatePanicReleasesBarrier(t *testing.T) {
	co := New(nil)
	co.OnInited()
	func() {
		defer func() {
			if recovered := recover(); recovered != "predicate failure" {
				t.Fatalf("predicate panic = %v, want predicate failure", recovered)
			}
		}()
		co.RunAfterStopAllIf(time.Second, func() bool {
			panic("predicate failure")
		}, nil)
	}()
	if !co.RunAfterStopAll(time.Second, nil) {
		t.Fatal("shutdown barrier remained locked after predicate panic")
	}
}

func TestRunAfterStopAllTimeoutIncludesBarrierWait(t *testing.T) {
	co := New(nil)
	co.OnInited()
	started := make(chan struct{})
	release := make(chan struct{})
	blocker := co.CreateAndStart("barrier-owner", func(Thread) {
		close(started)
		<-release
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

	startedAt := time.Now()
	if co.RunAfterStopAll(20*time.Millisecond, nil) {
		t.Fatal("second barrier completed while the first still owned the lock")
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("barrier lock wait took %v, want a bounded timeout", elapsed)
	}

	close(release)
	co.Join(blocker)
	waitForDrainResult(t, barrierDone)
}

func TestRunAfterStopAllRejectsSynchronousPanicHandlerReentry(t *testing.T) {
	guarded := make(chan any, 1)
	var co *Coroutines
	co = New(func(PanicReport) {
		func() {
			defer func() { guarded <- recover() }()
			co.RunAfterStopAll(time.Second, nil)
		}()
	})
	co.OnInited()

	worker := co.CreateAndStart("panicking-worker", func(Thread) {
		panic("worker failure")
	})
	waitForThreadSignal(t, worker.done, "panicking worker did not finish")
	select {
	case recovered := <-guarded:
		want := ErrReentrantWait
		if recovered != want {
			t.Fatalf("panic handler reentry panic = %v, want %q", recovered, want)
		}
	case <-time.After(time.Second):
		t.Fatal("panic handler did not attempt reset barrier reentry")
	}
	if !co.RunAfterStopAll(time.Second, nil) {
		t.Fatal("manager remained blocked after panic handler returned")
	}
}

func TestFinishThreadCleansLifecycleStateWhenPanicHandlerPanics(t *testing.T) {
	co := New(func(PanicReport) {
		panic("panic handler failure")
	})
	worker := co.newThread("panicking-worker")
	co.registerThread(worker)

	recovered := make(chan any, 1)
	go func() {
		goroutineID := gid.Get()
		co.goroutineThreads.Store(goroutineID, worker)
		co.runMu.Lock()
		co.setCurrent(worker)
		defer func() { recovered <- recover() }()
		co.finishThread(worker, goroutineID, "worker failure")
	}()

	select {
	case got := <-recovered:
		if got != "panic handler failure" {
			t.Fatalf("panic handler panic = %v, want %q", got, "panic handler failure")
		}
	case <-time.After(time.Second):
		t.Fatal("panicking handler did not return control")
	}
	if co.hasPendingWork(nil) {
		t.Fatal("thread remained registered after panic handler panicked")
	}
	finalizing := 0
	co.callbacks.Range(func(any, any) bool {
		finalizing++
		return true
	})
	if finalizing != 0 {
		t.Fatalf("finalizing goroutine markers = %d, want 0", finalizing)
	}
	if !co.RunAfterStopAll(time.Second, nil) {
		t.Fatal("manager remained blocked after panic handler panicked")
	}
}

func TestRunAfterStopAllWaitsForPanicHandlerBeforeReopeningAdmission(t *testing.T) {
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	var releaseHandlerOnce sync.Once
	co := New(func(PanicReport) {
		close(handlerStarted)
		<-releaseHandler
	})
	co.OnInited()

	workerStarted := make(chan struct{})
	releaseWorker := make(chan struct{})
	var releaseWorkerOnce sync.Once
	worker := co.CreateAndStart("panicking-worker", func(Thread) {
		close(workerStarted)
		<-releaseWorker
		panic("worker failure")
	})
	t.Cleanup(func() {
		releaseWorkerOnce.Do(func() { close(releaseWorker) })
		releaseHandlerOnce.Do(func() { close(releaseHandler) })
		if !co.StopAllAndWait(time.Second) {
			t.Error("panicking worker did not drain during cleanup")
		}
	})
	waitForThreadSignal(t, workerStarted, "panicking worker did not start")

	if co.RunAfterStopAll(20*time.Millisecond, nil) {
		t.Fatal("shutdown barrier reported success while the worker ignored cancellation")
	}
	releaseWorkerOnce.Do(func() { close(releaseWorker) })
	waitForThreadSignal(t, handlerStarted, "panic handler did not start")

	var rejectedRan atomic.Bool
	rejected := co.CreateAndStart("during-panic-handler", func(Thread) {
		rejectedRan.Store(true)
	})
	if !rejected.Stopped() {
		t.Fatal("creation was admitted while the panic handler was running")
	}
	waitForThreadSignal(t, rejected.done, "rejected creation did not finish")
	if rejectedRan.Load() {
		t.Fatal("creation ran while the panic handler was running")
	}

	barrierStarted := make(chan struct{})
	barrierCallback := make(chan struct{})
	barrierResult := make(chan bool, 1)
	go func() {
		close(barrierStarted)
		barrierResult <- co.RunAfterStopAll(time.Second, func() {
			close(barrierCallback)
		})
	}()
	waitForThreadSignal(t, barrierStarted, "second shutdown barrier did not start")
	select {
	case <-barrierCallback:
		t.Fatal("second shutdown barrier ran its callback while the panic handler was running")
	case result := <-barrierResult:
		t.Fatalf("second shutdown barrier returned %v while the panic handler was running", result)
	case <-time.After(50 * time.Millisecond):
	}

	releaseHandlerOnce.Do(func() { close(releaseHandler) })
	waitForThreadSignal(t, barrierCallback, "second shutdown barrier did not run its callback after the panic handler returned")
	select {
	case result := <-barrierResult:
		if !result {
			t.Fatal("second shutdown barrier timed out after the panic handler returned")
		}
	case <-time.After(time.Second):
		t.Fatal("second shutdown barrier did not return after the panic handler returned")
	}
	waitForThreadSignal(t, worker.done, "panicking worker did not finish")

	var admittedRan atomic.Bool
	admitted := co.CreateAndStart("after-panic-handler", func(Thread) {
		admittedRan.Store(true)
	})
	waitForThreadSignal(t, admitted.done, "creation remained blocked after the panic handler returned")
	if !admittedRan.Load() {
		t.Fatal("creation was rejected after the panic handler returned")
	}
}

func TestJoinResumesCallerAfterPeerCompletes(t *testing.T) {
	co := New(nil)
	releasePeer := make(chan struct{})
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := co.CreateAndStart("peer", func(peer Thread) {
		peerReady <- peer
		WaitForChan(co, releasePeer)
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.CreateAndStart("caller", func(me Thread) {
		co.Join(peer)
		close(callerDone)
	})

	select {
	case <-callerDone:
		t.Fatal("caller returned before peer completed")
	default:
	}

	close(releasePeer)

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-callerDone:
	case <-timer.C:
		t.Fatal("caller did not resume after peer completed")
	}
}

func TestJoinAllWaitsForEveryPeer(t *testing.T) {
	co := New(nil)
	releaseA := make(chan struct{})
	releaseB := make(chan struct{})
	peerAReady := make(chan Thread, 1)
	peerBReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peerA := co.CreateAndStart("peer-a", func(peer Thread) {
		peerAReady <- peer
		WaitForChan(co, releaseA)
	})
	peerB := co.CreateAndStart("peer-b", func(peer Thread) {
		peerBReady <- peer
		WaitForChan(co, releaseB)
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerAReady:
	case <-timer.C:
		t.Fatal("peer A coroutine did not start")
	}

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerBReady:
	case <-timer.C:
		t.Fatal("peer B coroutine did not start")
	}

	co.CreateAndStart("caller", func(me Thread) {
		co.JoinAll([]Thread{peerA, peerB, peerA})
		close(callerDone)
	})

	close(releaseA)

	select {
	case <-callerDone:
		t.Fatal("caller returned before every peer completed")
	default:
	}

	close(releaseB)

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-callerDone:
	case <-timer.C:
		t.Fatal("caller did not resume after every peer completed")
	}
}

func TestJoinYieldedOrDoneResumesCallerAfterPeerFirstYield(t *testing.T) {
	co := New(nil)
	releasePeer := make(chan struct{})
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := co.CreateAndStart("peer", func(peer Thread) {
		peerReady <- peer
		WaitForChan(co, releasePeer)
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.CreateAndStart("caller", func(me Thread) {
		co.JoinYieldedOrDone(peer)
		close(callerDone)
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-callerDone:
	case <-timer.C:
		t.Fatal("caller did not resume after peer reached its first yield")
	}

	select {
	case <-releasePeer:
		t.Fatal("releasePeer should remain controlled by the test")
	default:
	}
	close(releasePeer)
}

func TestJoinYieldedOrDoneReturnsWhenPeerExitsWithoutYield(t *testing.T) {
	co := New(nil)
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := co.CreateAndStart("peer", func(peer Thread) {
		peerReady <- peer
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.CreateAndStart("caller", func(me Thread) {
		co.JoinYieldedOrDone(peer)
		close(callerDone)
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-callerDone:
	case <-timer.C:
		t.Fatal("caller did not resume after peer returned without yielding")
	}
}

func TestCreateAndStartFromCoroutineYieldsCallerWhenStarted(t *testing.T) {
	co := New(nil)
	co.OnInited()

	peerStarted := make(chan struct{}, 1)
	observed := make(chan bool, 1)
	done := make(chan struct{})

	co.CreateAndStart("caller", func(me Thread) {
		co.CreateAndStart("peer", func(peer Thread) {
			close(peerStarted)
		})

		select {
		case <-peerStarted:
			observed <- true
		default:
			observed <- false
		}
		close(done)
	})

	deadline := time.Now().Add(time.Second)
	for {
		select {
		case <-done:
			select {
			case got := <-observed:
				if !got {
					t.Fatal("CreateAndStart returned before the eagerly started peer ran")
				}
			default:
				t.Fatal("caller completed without recording eager-start observation")
			}
			return
		default:
		}

		if time.Now().After(deadline) {
			t.Fatal("caller did not finish while pumping the scheduler")
		}

		co.Update()
		time.Sleep(time.Millisecond)
	}
}
