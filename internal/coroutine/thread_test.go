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

func TestStopIfStopsActiveThread(t *testing.T) {
	co := New(nil)
	started := make(chan Thread, 1)
	done := make(chan struct{})

	co.CreateAndStart(true, "worker", func(me Thread) int {
		started <- me
		for {
			select {
			case <-me.Context().Done():
				close(done)
				return 0
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
	thread := co.Create("worker", func(me Thread) int {
		co.Yield(me)
		return 0
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

	co.CreateAndStart(true, "worker", func(me Thread) int {
		started <- me
		for {
			select {
			case <-me.Context().Done():
				close(done)
				return 0
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

func TestAbortAllAndWaitFromCoroutineWaitsForPeersWithoutWaitingForCaller(t *testing.T) {
	co := New(nil)
	result := make(chan bool, 1)
	otherYielding := make(chan struct{})
	otherDone := make(chan struct{})

	co.CreateAndStart(true, "other", func(other Thread) int {
		defer close(otherDone)
		close(otherYielding)
		co.Yield(other)
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-otherYielding:
	case <-timer.C:
		t.Fatal("other coroutine did not reach yield")
	}

	co.CreateAndStart(true, "caller", func(me Thread) int {
		result <- co.AbortAllAndWait(time.Hour)
		return 0
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case completed := <-result:
		if !completed {
			t.Fatal("AbortAllAndWait should report success after other coroutines stop")
		}
	case <-timer.C:
		t.Fatal("AbortAllAndWait did not finish after peer coroutine stopped")
	}

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-otherDone:
	case <-timer.C:
		t.Fatal("other coroutine did not exit while caller was waiting")
	}
}

func TestAbortAllAndWaitFromCoroutineDoesNotStartStoppedPeer(t *testing.T) {
	co := New(nil)
	result := make(chan bool, 1)
	peerRan := make(chan struct{}, 1)

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.CreateAndStart(false, "peer", func(peer Thread) int {
			close(peerRan)
			return 0
		})
		result <- co.AbortAllAndWait(time.Second)
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case completed := <-result:
		if !completed {
			t.Fatal("AbortAllAndWait should wait for the stopped peer to unregister")
		}
	case <-timer.C:
		t.Fatal("AbortAllAndWait did not finish after stopped peer unregistered")
	}

	select {
	case <-peerRan:
		t.Fatal("stopped peer coroutine should not run")
	default:
	}
}

func TestAbortAllRejectsChildCreatedByCanceledOwner(t *testing.T) {
	co := New(nil)
	co.OnInited()
	childResult := make(chan Thread, 1)
	var childRan atomic.Bool

	co.CreateAndStart(true, "parent", func(Thread) int {
		co.AbortAll()
		childResult <- co.Create("late-child", func(Thread) int {
			childRan.Store(true)
			return 0
		})
		return 0
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
	if !co.waitForThreadsToStop(time.Second, nil) {
		t.Fatal("canceled owner and rejected child did not stop")
	}
	if childRan.Load() {
		t.Fatal("child created by a canceled owner ran user code")
	}
}

func TestAbortAllAndWaitOrdersInFlightRegistration(t *testing.T) {
	co := New(nil)
	co.OnInited()
	co.creationMu.Lock()
	registrationStarted := make(chan struct{})
	created := make(chan Thread, 1)
	go func() {
		close(registrationStarted)
		created <- co.Create("in-flight", func(Thread) int { return 0 })
	}()
	select {
	case <-registrationStarted:
	case <-time.After(time.Second):
		co.creationMu.Unlock()
		t.Fatal("registration did not start")
	}

	abortDone := make(chan bool, 1)
	go func() {
		abortDone <- co.AbortAllAndWait(time.Second)
	}()
	select {
	case <-abortDone:
		co.creationMu.Unlock()
		t.Fatal("abort barrier returned while an earlier registration was in flight")
	case <-time.After(20 * time.Millisecond):
	}
	co.creationMu.Unlock()

	select {
	case completed := <-abortDone:
		if !completed {
			t.Fatal("abort barrier timed out after registration was released")
		}
	case <-time.After(time.Second):
		t.Fatal("abort barrier did not finish")
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
		t.Fatal("in-flight thread remained after the abort barrier")
	}
}

func TestAbortAllAndWaitRejectsCreationDuringBarrier(t *testing.T) {
	co := New(nil)
	co.OnInited()
	blocker := co.newThread("barrier-blocker")
	co.registerThread(blocker)

	abortDone := make(chan bool, 1)
	go func() {
		abortDone <- co.AbortAllAndWait(time.Second)
	}()
	deadline := time.Now().Add(time.Second)
	for !blocker.Stopped() {
		if time.Now().After(deadline) {
			co.removeThreadState(blocker)
			co.unregisterThread(blocker)
			t.Fatal("abort barrier did not start")
		}
		runtime.Gosched()
	}

	var childRan atomic.Bool
	child := co.Create("during-barrier", func(Thread) int {
		childRan.Store(true)
		return 0
	})
	if !child.Stopped() {
		t.Fatal("thread created during abort barrier was admitted")
	}
	co.removeThreadState(blocker)
	co.unregisterThread(blocker)

	select {
	case completed := <-abortDone:
		if !completed {
			t.Fatal("abort barrier timed out")
		}
	case <-time.After(time.Second):
		t.Fatal("abort barrier did not finish after the blocker was removed")
	}
	select {
	case <-child.done:
	case <-time.After(time.Second):
		t.Fatal("rejected thread did not finish")
	}
	if childRan.Load() {
		t.Fatal("thread created during abort barrier ran user code")
	}
}

func TestRunAfterAbortAllTimeoutKeepsAdmissionClosedUntilDrain(t *testing.T) {
	co := New(nil)
	co.OnInited()
	blocker := co.newThread("barrier-blocker")
	co.registerThread(blocker)

	var callbackRan atomic.Bool
	if co.RunAfterAbortAll(20*time.Millisecond, func() {
		callbackRan.Store(true)
	}) {
		t.Fatal("abort barrier reported success with a registered blocker")
	}
	if callbackRan.Load() {
		t.Fatal("abort barrier ran callback after timing out")
	}

	var rejectedRan atomic.Bool
	rejected := co.CreateAndStart(true, "during-timeout", func(Thread) int {
		rejectedRan.Store(true)
		return 0
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
	var nextRan atomic.Bool
	next := co.CreateAndStart(true, "after-timeout", func(Thread) int {
		nextRan.Store(true)
		return 0
	})
	select {
	case <-next.done:
	case <-time.After(time.Second):
		t.Fatal("creation remained blocked after abort barrier timed out")
	}
	if !nextRan.Load() {
		t.Fatal("creation was rejected after abort barrier timed out")
	}
}

func TestJoinResumesCallerAfterPeerCompletes(t *testing.T) {
	co := New(nil)
	releasePeer := make(chan struct{})
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := co.CreateAndStart(true, "peer", func(peer Thread) int {
		peerReady <- peer
		var signal struct{}
		WaitForChan(co, releasePeer, &signal)
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.Join(peer)
		close(callerDone)
		return 0
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

	peerA := co.CreateAndStart(true, "peer-a", func(peer Thread) int {
		peerAReady <- peer
		var signal struct{}
		WaitForChan(co, releaseA, &signal)
		return 0
	})
	peerB := co.CreateAndStart(true, "peer-b", func(peer Thread) int {
		peerBReady <- peer
		var signal struct{}
		WaitForChan(co, releaseB, &signal)
		return 0
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

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.JoinAll([]Thread{peerA, peerB, peerA})
		close(callerDone)
		return 0
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

	peer := co.CreateAndStart(true, "peer", func(peer Thread) int {
		peerReady <- peer
		var signal struct{}
		WaitForChan(co, releasePeer, &signal)
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.JoinYieldedOrDone(peer)
		close(callerDone)
		return 0
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

	peer := co.CreateAndStart(true, "peer", func(peer Thread) int {
		peerReady <- peer
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.JoinYieldedOrDone(peer)
		close(callerDone)
		return 0
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

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.CreateAndStart(true, "peer", func(peer Thread) int {
			close(peerStarted)
			return 0
		})

		select {
		case <-peerStarted:
			observed <- true
		default:
			observed <- false
		}
		close(done)
		return 0
	})

	deadline := time.Now().Add(time.Second)
	for {
		select {
		case <-done:
			select {
			case got := <-observed:
				if !got {
					t.Fatal("CreateAndStart(true) returned before the eagerly started peer ran")
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
