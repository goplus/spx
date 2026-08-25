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
	"math"
	"runtime"
	sdebug "runtime/debug"
	"sync/atomic"
	"testing"
	"time"
)

func TestUpdateReadsGCStatsOnlyWhenPerfDebugEnabled(t *testing.T) {
	co := New(nil)
	co.OnInited()

	originalReadGCStats := co.readGCStats
	t.Cleanup(func() {
		co.readGCStats = originalReadGCStats
	})

	var calls int
	co.readGCStats = func(*sdebug.GCStats) {
		calls++
	}

	co.Update()
	if calls != 0 {
		t.Fatalf("expected GC stats collection to be disabled by default, got %d calls", calls)
	}
	if co.GetLastUpdateStats().GCStatsEnabled {
		t.Fatal("expected GC stats to be marked disabled by default")
	}

	co.SetPerfDebug(true)
	co.Update()
	if calls != 2 {
		t.Fatalf("expected GC stats to be read twice when perf debug is enabled, got %d calls", calls)
	}
	if !co.GetLastUpdateStats().GCStatsEnabled {
		t.Fatal("expected GC stats to be marked enabled when perf debug is on")
	}
}

func TestLastUpdateStatsAreScopedToManager(t *testing.T) {
	first := New(nil)
	first.OnInited()
	first.SetPerfDebug(true)
	first.readGCStats = func(*sdebug.GCStats) {}
	first.Update()

	second := New(nil)
	if second.GetLastUpdateStats().GCStatsEnabled {
		t.Fatal("a new manager inherited update statistics from another manager")
	}
}

func TestWaitForChanReceivesValue(t *testing.T) {
	co := New(nil)
	ch := make(chan int)
	done := make(chan struct{})

	co.CreateAndStart(false, nil, func(me Thread) int {
		defer close(done)
		var value int
		WaitForChan(co, ch, &value)
		if value != 7 {
			t.Fatalf("expected received value 7, got %d", value)
		}
		return 0
	})

	deadline := time.Now().Add(time.Second)
	for {
		co.schedulerMu.Lock()
		waitingCount := len(co.threadStates)
		co.schedulerMu.Unlock()
		if waitingCount > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("coroutine did not enter wait state")
		}
		runtime.Gosched()
	}

	ch <- 7

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("coroutine did not resume after channel receive")
	}
}

func TestWaitForChanDoesNotKeepReceiverAfterCancel(t *testing.T) {
	co := New(nil)
	ch := make(chan int)
	done := make(chan struct{})

	th := co.CreateAndStart(false, nil, func(me Thread) int {
		defer close(done)
		var value int
		WaitForChan(co, ch, &value)
		return 0
	})

	deadline := time.Now().Add(time.Second)
	for {
		co.schedulerMu.Lock()
		state := co.threadStates[th]
		co.schedulerMu.Unlock()
		if state == threadBlocked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("coroutine did not enter wait state")
		}
		runtime.Gosched()
	}

	co.StopIf(func(candidate Thread) bool {
		return candidate == th
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("coroutine did not exit after cancel")
	}

	deadline = time.Now().Add(time.Second)
	for {
		select {
		case ch <- 1:
		case <-time.After(20 * time.Millisecond):
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("channel receiver should eventually exit after cancel")
		}
	}
}

func TestYieldClearsCurrentWhileCoroutineIsSuspended(t *testing.T) {
	co := New(nil)
	started := make(chan struct{})
	done := make(chan struct{})

	th := co.Create("worker", func(me Thread) int {
		close(started)
		co.Yield(me)
		close(done)
		return 0
	})

	<-started
	deadline := time.Now().Add(time.Second)
	for !th.suspended.Load() {
		if time.Now().After(deadline) {
			t.Fatal("coroutine did not suspend")
		}
		runtime.Gosched()
	}
	if current := co.Current(); current != nil {
		t.Fatalf("Current while every coroutine is suspended = %v, want nil", current)
	}

	co.Resume(th)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("coroutine did not finish after resume")
	}
}

func TestResumeBeforeYieldDoesNotBlockOrLoseWakeup(t *testing.T) {
	co := New(nil)
	started := make(chan struct{})
	proceed := make(chan struct{})
	done := make(chan struct{})

	th := co.Create("worker", func(me Thread) int {
		close(started)
		<-proceed
		co.Yield(me)
		close(done)
		return 0
	})
	<-started

	resumeDone := make(chan struct{})
	go func() {
		co.Resume(th)
		close(resumeDone)
	}()
	select {
	case <-resumeDone:
	case <-time.After(time.Second):
		close(proceed)
		t.Fatal("Resume blocked while waiting for Yield to publish suspension")
	}
	close(proceed)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Yield did not consume the earlier Resume signal")
	}
}

func TestMutualYieldWaitersDoNotDeadlock(t *testing.T) {
	co := New(nil)
	a := co.newThread("a")
	b := co.newThread("b")
	co.registerThread(a)
	co.registerThread(b)
	a.yieldWaiters = map[Thread]struct{}{b: {}}
	b.yieldWaiters = map[Thread]struct{}{a: {}}

	// Keep A between releasing runMu and publishing its suspension. B can then
	// publish its own suspension and try to wake A. Yield must not hold B's
	// suspendMu during that wake-up, or A and B can deadlock on each other's
	// suspendMu.
	a.suspendMu.Lock()
	aStarted := make(chan struct{})
	bStarted := make(chan struct{})
	done := make(chan struct{}, 2)
	go func() {
		co.runThread(a, func(me Thread) int {
			close(aStarted)
			co.Yield(me)
			return 0
		})
		done <- struct{}{}
	}()
	<-aStarted
	co.runMu.Lock()
	co.runMu.Unlock()

	go func() {
		co.runThread(b, func(me Thread) int {
			close(bStarted)
			co.Yield(me)
			return 0
		})
		done <- struct{}{}
	}()
	<-bStarted

	deadline := time.Now().Add(time.Second)
	for !b.suspended.Load() {
		if time.Now().After(deadline) {
			a.suspendMu.Unlock()
			t.Fatal("B did not publish its suspended state")
		}
		runtime.Gosched()
	}
	a.suspendMu.Unlock()

	for range 2 {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("mutual yield waiters deadlocked")
		}
	}
}

func TestSchedResumesWithoutWaitingForUpdate(t *testing.T) {
	co := New(nil)
	done := make(chan struct{})
	co.Create("worker", func(me Thread) int {
		co.Sched(me)
		close(done)
		return 0
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Sched waited for an Update pass")
	}
}

func TestUpdateDrainsRepeatedWaitYieldWithoutLosingRunnableState(t *testing.T) {
	const (
		runs   = 20
		yields = 100
	)
	for run := 0; run < runs; run++ {
		co := New(nil)
		co.OnInited()
		done := make(chan struct{})
		th := co.Create("worker", func(me Thread) int {
			for range yields {
				co.WaitYield(me)
			}
			close(done)
			return 0
		})

		deadline := time.Now().Add(time.Second)
		for !th.suspended.Load() {
			if time.Now().After(deadline) {
				t.Fatalf("run %d: coroutine did not reach its first yield", run)
			}
			runtime.Gosched()
		}
		co.Update()
		select {
		case <-done:
		default:
			t.Fatalf("run %d: Update returned before all %d cooperative yields completed", run, yields)
		}
	}
}

func TestUpdateWaitsForCoroutineSpawnedByCurrentTask(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	const runs = 100
	for run := 0; run < runs; run++ {
		co := New(nil)
		co.OnInited()
		var ran atomic.Bool

		co.enqueueJob(&WaitJob{
			Type: waitTypeMainThread,
			Call: func() {
				co.Create("spawned", func(me Thread) int {
					ran.Store(true)
					return 0
				})
			},
		})

		co.Update()
		if !ran.Load() {
			t.Fatalf("run %d: Update returned before the spawned coroutine ran", run)
		}
	}
}

func TestUpdateWatchdogDeadlineResetsWhileAwaitingInitialization(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() {
		co.OnInited()
		if !co.AbortAllAndWait(time.Second) {
			t.Error("coroutines did not stop during cleanup")
		}
	})

	clockStart := time.Now()
	waitingForInitialization := make(chan struct{})
	var clockCalls atomic.Int64
	co.updateWatchdogNow = func() time.Time {
		call := clockCalls.Add(1)
		if call == 1 {
			return clockStart
		}
		if call == 2 {
			close(waitingForInitialization)
		}
		return clockStart.Add(2 * updateWatchdogTimeout)
	}

	updateDone := make(chan struct{})
	go func() {
		co.Update()
		close(updateDone)
	}()

	select {
	case <-waitingForInitialization:
	case <-time.After(time.Second):
		co.OnInited()
		select {
		case <-updateDone:
		case <-time.After(time.Second):
			t.Fatal("Update remained blocked after initialization cleanup")
		}
		t.Fatal("Update did not refresh its watchdog while awaiting initialization")
	}

	var resumed atomic.Bool
	co.enqueueJob(&WaitJob{
		Type: waitTypeMainThread,
		Call: func() {
			co.Create("after-initialization", func(me Thread) int {
				co.WaitYield(me)
				resumed.Store(true)
				return 0
			})
		},
	})
	co.OnInited()

	select {
	case <-updateDone:
	case <-time.After(time.Second):
		t.Fatal("Update did not complete after initialization")
	}
	if !resumed.Load() {
		t.Fatal("watchdog stopped normal work immediately after initialization")
	}
}

func TestUpdateWatchdogStopsRecursiveSpawnChainOnRetry(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	co := New(nil)
	co.OnInited()

	clockStart := time.Now()
	var clockCalls atomic.Int64
	co.updateWatchdogNow = func() time.Time {
		if clockCalls.Add(1) == 1 {
			return clockStart
		}
		return clockStart.Add(updateWatchdogTimeout + time.Nanosecond)
	}

	var keepSpawning atomic.Bool
	keepSpawning.Store(true)
	t.Cleanup(func() {
		keepSpawning.Store(false)
		if !co.AbortAllAndWait(time.Second) {
			t.Error("recursive spawn chain did not stop during cleanup")
		}
	})

	var spawned atomic.Int64
	var spawnNext func(Thread) int
	spawnNext = func(Thread) int {
		spawned.Add(1)
		if keepSpawning.Load() {
			co.Create("recursive", spawnNext)
			runtime.Gosched()
		}
		return 0
	}
	co.Create("recursive", spawnNext)

	updateDone := make(chan struct{})
	go func() {
		co.Update()
		close(updateDone)
	}()

	select {
	case <-updateDone:
	case <-time.After(time.Second):
		keepSpawning.Store(false)
		if !co.AbortAllAndWait(time.Second) {
			t.Fatal("recursive spawn chain did not stop after the watchdog test timed out")
		}
		select {
		case <-updateDone:
		case <-time.After(time.Second):
			t.Fatal("Update remained blocked after recursive spawn chain cleanup")
		}
		t.Fatal("Update did not stop a recursive spawn chain after the watchdog expired")
	}

	if got := clockCalls.Load(); got < 2 {
		t.Fatalf("watchdog clock was called %d times, want a deadline check after a wait retry", got)
	}
	if got := spawned.Load(); got == 0 {
		t.Fatal("recursive spawn chain never started")
	}
	stats := co.GetLastUpdateStats()
	if stats.TaskCounts != 0 {
		t.Fatalf("processed %d wait jobs, want a retry-only spawn chain", stats.TaskCounts)
	}
	if remaining := len(co.snapshotThreads()); remaining != 0 {
		t.Fatalf("%d managed threads remained after watchdog shutdown", remaining)
	}
	keepSpawning.Store(false)

	co.updateWatchdogNow = time.Now
	resumed := make(chan struct{})
	thread := co.Create("after-watchdog", func(me Thread) int {
		co.WaitYield(me)
		close(resumed)
		return 0
	})
	deadline := time.Now().Add(time.Second)
	for !thread.suspended.Load() {
		if time.Now().After(deadline) {
			t.Fatal("post-watchdog coroutine did not reach its yield")
		}
		runtime.Gosched()
	}

	recoveryDone := make(chan struct{})
	go func() {
		co.Update()
		close(recoveryDone)
	}()
	select {
	case <-recoveryDone:
	case <-time.After(time.Second):
		if !co.AbortAllAndWait(time.Second) {
			t.Fatal("post-watchdog coroutine did not stop during recovery cleanup")
		}
		select {
		case <-recoveryDone:
		case <-time.After(time.Second):
			t.Fatal("recovery Update remained blocked after cleanup")
		}
		t.Fatal("scheduler did not complete an Update after watchdog recovery")
	}
	select {
	case <-resumed:
	default:
		t.Fatal("scheduler did not resume a coroutine after watchdog recovery")
	}
}

func TestRunawayShutdownRejectsThreadCreation(t *testing.T) {
	co := New(nil)
	blocker := co.newThread("shutdown-blocker")
	co.registerThread(blocker)
	removeBlocker := func() {
		co.removeThreadState(blocker)
		co.unregisterThread(blocker)
	}
	t.Cleanup(func() {
		removeBlocker()
		if !co.AbortAllAndWait(time.Second) {
			t.Error("coroutines did not stop during cleanup")
		}
	})

	shutdownDone := make(chan struct{})
	go func() {
		co.stopRunawayThreads()
		close(shutdownDone)
	}()

	deadline := time.Now().Add(time.Second)
	for !blocker.Stopped() {
		if time.Now().After(deadline) {
			t.Fatal("runaway shutdown did not reach its abort snapshot")
		}
		runtime.Gosched()
	}

	var ran atomic.Bool
	created := make(chan Thread, 1)
	go func() {
		created <- co.Create("after-shutdown", func(Thread) int {
			ran.Store(true)
			return 0
		})
	}()

	var rejected Thread
	select {
	case rejected = <-created:
	case <-time.After(time.Second):
		t.Fatal("thread creation remained blocked during runaway shutdown")
	}
	if !rejected.Stopped() {
		t.Fatal("thread created during runaway shutdown was not canceled")
	}

	removeBlocker()
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("runaway shutdown did not finish")
	}
	if ran.Load() {
		t.Fatal("thread created during runaway shutdown ran user code")
	}

	afterShutdown := make(chan struct{})
	co.Create("after-shutdown", func(Thread) int {
		close(afterShutdown)
		return 0
	})
	select {
	case <-afterShutdown:
	case <-time.After(time.Second):
		t.Fatal("thread created after runaway shutdown did not run")
	}
	if !co.waitForThreadsToStop(time.Second, nil) {
		t.Fatal("post-shutdown thread did not complete")
	}
}

func TestRunawayShutdownAllowsCanceledCleanupToCreate(t *testing.T) {
	co := New(nil)
	co.OnInited()
	t.Cleanup(func() {
		if !co.AbortAllAndWait(time.Second) {
			t.Error("coroutines did not stop during cleanup")
		}
	})

	var cleanupChildRan atomic.Bool
	cleanupCreated := make(chan struct{})
	thread := co.Create("worker", func(me Thread) int {
		defer func() {
			co.Create("cleanup-child", func(Thread) int {
				cleanupChildRan.Store(true)
				return 0
			})
			close(cleanupCreated)
		}()
		co.WaitYield(me)
		return 0
	})

	deadline := time.Now().Add(time.Second)
	for !thread.suspended.Load() {
		if time.Now().After(deadline) {
			t.Fatal("worker did not reach its yield")
		}
		runtime.Gosched()
	}

	shutdownDone := make(chan struct{})
	go func() {
		co.stopRunawayThreads()
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("runaway shutdown deadlocked with coroutine cleanup")
	}
	select {
	case <-cleanupCreated:
	default:
		t.Fatal("canceled coroutine cleanup did not finish creating its child")
	}
	if cleanupChildRan.Load() {
		t.Fatal("child created by canceled cleanup ran during shutdown")
	}
}

func TestResumeJobDoesNotRestoreCompletedThreadState(t *testing.T) {
	co := New(nil)

	thread := co.Create("completed", func(Thread) int { return 0 })
	resume := co.newResumeWaitJob(thread, waitTypeYield)
	if !co.waitForThreadsToStop(time.Second, nil) {
		t.Fatal("coroutine did not complete")
	}

	resume.Call()
	co.schedulerMu.Lock()
	_, exists := co.threadStates[thread]
	co.schedulerMu.Unlock()
	if exists {
		t.Fatal("stale resume job restored scheduler state for a completed thread")
	}
}

func TestCanceledWaitJobIsDiscardedBeforeItsDeadline(t *testing.T) {
	co := New(nil)
	co.OnInited()

	thread := co.Create("completed", func(Thread) int { return 0 })
	if !co.waitForThreadsToStop(time.Second, nil) {
		t.Fatal("coroutine did not complete")
	}

	called := false
	co.enqueueJob(&WaitJob{
		Th:   thread,
		Type: waitTypeTime,
		Time: math.MaxFloat64,
		Call: func() { called = true },
	})
	co.Update()

	if called {
		t.Fatal("canceled wait job callback ran")
	}
	if current, deferred := co.currentJobs.Count(), co.deferredJobs.Count(); current != 0 || deferred != 0 {
		t.Fatalf("canceled wait job remained queued: current=%d deferred=%d", current, deferred)
	}
}

func TestUpdateWaitsForJoinWaiterResumedByCompletingThread(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(4)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	const runs = 100
	for run := 0; run < runs; run++ {
		co := New(nil)
		co.OnInited()
		var waiterRan atomic.Bool

		co.enqueueJob(&WaitJob{
			Type: waitTypeMainThread,
			Call: func() {
				co.Create("controller", func(controller Thread) int {
					var target Thread
					co.Create("waiter", func(waiter Thread) int {
						co.Join(target)
						waiterRan.Store(true)
						return 0
					})
					target = co.Create("target", func(target Thread) int {
						return 0
					})
					return 0
				})
			},
		})

		co.Update()
		if !waiterRan.Load() {
			t.Fatalf("run %d: Update returned before the Join waiter resumed", run)
		}
	}
}
