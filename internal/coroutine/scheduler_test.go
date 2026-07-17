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
	sdebug "runtime/debug"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v2/internal/engine/platform"
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

func TestWaitMainThreadFastPathOnMainThread(t *testing.T) {
	co := New(nil)
	called := false

	platform.RunOnMainThread(func() {
		co.WaitMainThread(func() {
			called = true
		})
	})

	if !called {
		t.Fatal("WaitMainThread should execute immediately on the main thread")
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
