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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	itime "github.com/goplus/spx/v4/internal/time"
)

func TestUpdateReadsGCStatsOnlyWhenPerfDebugEnabled(t *testing.T) {
	co := New(nil)

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
	if co.UpdateStats().GCStatsEnabled {
		t.Fatal("expected GC stats to be marked disabled by default")
	}

	co.SetPerfDebug(true)
	co.Update()
	if calls != 2 {
		t.Fatalf("expected GC stats to be read twice when perf debug is enabled, got %d calls", calls)
	}
	if !co.UpdateStats().GCStatsEnabled {
		t.Fatal("expected GC stats to be marked enabled when perf debug is on")
	}
}

func TestUpdateStatsAreScopedToManager(t *testing.T) {
	first := New(nil)
	first.SetPerfDebug(true)
	first.readGCStats = func(*sdebug.GCStats) {}
	first.Update()

	second := New(nil)
	if second.UpdateStats().GCStatsEnabled {
		t.Fatal("a new manager inherited update statistics from another manager")
	}
}

func TestWaitForChanReceivesValue(t *testing.T) {
	co := New(nil)
	ch := make(chan int)
	done := make(chan struct{})

	th := co.Create(nil, func(me Thread) {
		defer close(done)
		value := WaitForChan(co, ch)
		if value != 7 {
			t.Fatalf("expected received value 7, got %d", value)
		}
	})

	deadline := time.Now().Add(time.Second)
	for !isSuspended(th) {
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

func TestExternalWaitPathsDoNotEnterScheduler(t *testing.T) {
	co := New(nil)
	co.Wait(1)

	ch := make(chan int, 1)
	ch <- 7
	if got := WaitForChan(co, ch); got != 7 {
		t.Fatalf("external WaitForChan = %d, want 7", got)
	}
	co.schedulerMu.Lock()
	queued := co.currentJobs.len() + co.deferredJobs.len() + co.roundJobs.len()
	co.schedulerMu.Unlock()
	if queued != 0 {
		t.Fatalf("external waits queued %d scheduler jobs", queued)
	}
}

func TestWaitForChanDoesNotKeepReceiverAfterCancel(t *testing.T) {
	co := New(nil)
	ch := make(chan int)
	done := make(chan struct{})

	th := co.Create(nil, func(me Thread) {
		defer close(done)
		WaitForChan(co, ch)
	})

	deadline := time.Now().Add(time.Second)
	for !isSuspended(th) {
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
	t.Cleanup(func() { co.RunAfterStop(nil) })
	started := make(chan struct{})
	done := make(chan struct{})

	th := co.Create("worker", func(me Thread) {
		co.blockThread(me)
		close(started)
		co.yield(me)
		close(done)
	})

	<-started
	deadline := time.Now().Add(time.Second)
	for !isSuspended(th) {
		if time.Now().After(deadline) {
			t.Fatal("coroutine did not suspend")
		}
		runtime.Gosched()
	}
	if current := co.Current(); current != nil {
		t.Fatalf("Current while every coroutine is suspended = %v, want nil", current)
	}

	co.wakeThread(th)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("coroutine did not finish after resume")
	}
}

func TestStopAtNextYieldStopsBeforeResume(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
	var resumed atomic.Bool
	var publishedBeforeCancellation atomic.Bool

	thread := co.createThread(co.captureAdmission(), Task{
		Owner: "worker",
		RegisterInvocation: func(th Thread) func() {
			cancel := th.cancelFunc
			// Probe the first cancellation before completion calls cancel again.
			var once sync.Once
			th.cancelFunc = func() {
				once.Do(func() {
					select {
					case <-th.firstSliceDone:
						publishedBeforeCancellation.Store(true)
					default:
					}
				})
				cancel()
			}
			return nil
		},
		Run: func(me Thread) {
			co.StopAtNextYield(me)
			co.WaitYield(me)
			resumed.Store(true)
		},
	})

	co.JoinFirstSlice(thread)
	if !thread.Stopped() {
		t.Fatal("yield boundary was published before the coroutine was stopped")
	}
	select {
	case <-thread.done:
	case <-time.After(time.Second):
		t.Fatal("coroutine did not stop at its next yield")
	}
	if publishedBeforeCancellation.Load() {
		t.Fatal("first slice was published before stop-at-yield cancellation")
	}
	co.Update()
	if resumed.Load() {
		t.Fatal("coroutine resumed after its stop-at-yield boundary")
	}

	completed := co.Create("no-yield", func(me Thread) {
		co.StopAtNextYield(me)
	})
	co.Join(completed)
	if completed.Stopped() {
		t.Fatal("coroutine was stopped without reaching a yield boundary")
	}
}

func TestResumeBeforeYieldDoesNotBlockOrLoseWakeup(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
	started := make(chan struct{})
	proceed := make(chan struct{})
	done := make(chan struct{})

	th := co.Create("worker", func(me Thread) {
		co.blockThread(me)
		close(started)
		<-proceed
		co.yield(me)
		close(done)
	})
	<-started

	resumeDone := make(chan struct{})
	go func() {
		co.wakeThread(th)
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
	t.Cleanup(func() { co.RunAfterStop(nil) })
	caller := co.Create("caller", func(Thread) {
		// Publish both peers before the caller yields to either one. A's
		// wait for B publishes A's first slice, allowing B's join to return.
		var peers []Thread
		peers = co.StartBatch([]Task{
			{Owner: "a", Run: func(Thread) { co.JoinFirstSlice(peers[1]) }},
			{Owner: "b", Run: func(Thread) { co.JoinFirstSlice(peers[0]) }},
		}, BatchAsync)
		co.JoinAll(peers)
	})
	waitForThreadSignal(t, caller.done, "mutual first-slice waiters deadlocked")
}

func TestSchedResumesWithoutWaitingForUpdate(t *testing.T) {
	co := New(nil)
	done := make(chan struct{})
	co.Create("worker", func(me Thread) {
		co.Sched(me)
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Sched waited for an Update pass")
	}
}

func TestSchedSelfSelectionLetsWorkerProgress(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
	polling := make(chan struct{})
	var workerDone atomic.Bool
	observed := make(chan bool, 1)

	co.StartBatch([]Task{
		{Run: func(Thread) {
			co.WaitToDo(func() {
				<-polling
				workerDone.Store(true)
			})
		}},
		{Run: func(me Thread) {
			close(polling)
			// Bound the poll so a missing Go scheduler handoff fails rather
			// than hanging the WASM host, which lacks async preemption.
			for range 10000 {
				if workerDone.Load() {
					break
				}
				co.Sched(me)
			}
			observed <- workerDone.Load()
		}},
	}, BatchWaitDone)

	if !<-observed {
		t.Fatal("self-selected script starved its WaitToDo worker")
	}
}

func TestUpdateDrainsRepeatedWaitYieldWithoutLosingRunnableState(t *testing.T) {
	const (
		runs   = 20
		yields = 100
	)
	for run := 0; run < runs; run++ {
		co := New(nil)
		done := make(chan struct{})
		th := co.Create("worker", func(me Thread) {
			for range yields {
				co.WaitYield(me)
			}
			close(done)
		})

		deadline := time.Now().Add(time.Second)
		for !isSuspended(th) {
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
		var ran atomic.Bool

		co.enqueueJob(&waitJob{
			kind: waitMainThread,
			call: func() {
				co.Create("spawned", func(me Thread) {
					ran.Store(true)
				})
			},
		})

		co.Update()
		if !ran.Load() {
			t.Fatalf("run %d: Update returned before the spawned coroutine ran", run)
		}
	}
}

func TestUpdateReturnsForStartupOutcomes(t *testing.T) {
	for _, outcome := range []string{"idle", "success", "failure", "canceled"} {
		t.Run(outcome, func(t *testing.T) {
			var completed atomic.Bool
			panicReported := make(chan PanicReport, 1)
			co := New(func(report PanicReport) { panicReported <- report })
			t.Cleanup(func() {
				co.RunAfterStop(nil)
			})

			if outcome != "idle" {
				co.Create("startup", func(me Thread) {
					// Finish only after Update has entered and resumed startup.
					co.WaitYield(me)
					switch outcome {
					case "success":
						completed.Store(true)
					case "failure":
						panic("startup failed")
					case "canceled":
						co.StopCurrent()
					}
				})
			}

			updated := make(chan struct{})
			go func() {
				co.Update()
				close(updated)
			}()
			waitForThreadSignal(t, updated, "Update waited for startup to succeed")
			if got, want := completed.Load(), outcome == "success"; got != want {
				t.Fatalf("completed = %v, want %v", got, want)
			}
			if outcome == "failure" {
				select {
				case report := <-panicReported:
					if report.Value != "startup failed" {
						t.Fatalf("panic = %v, want startup failed", report.Value)
					}
				case <-time.After(time.Second):
					t.Fatal("startup failure was not reported")
				}
			}
		})
	}
}

func TestUpdateReturnsWhileStartupWaitsForWorker(t *testing.T) {
	var completed atomic.Bool
	co := New(nil)
	resume := make(chan struct{})
	thread := co.Create("startup", func(me Thread) {
		WaitForChan(co, resume)
		completed.Store(true)
	})
	t.Cleanup(func() {
		co.RunAfterStop(nil)
	})

	updated := make(chan struct{})
	go func() {
		co.Update()
		close(updated)
	}()
	waitForThreadSignal(t, updated, "Update waited for startup's asynchronous work")
	if completed.Load() {
		t.Fatal("startup completed before its worker finished")
	}
	close(resume)
	waitForThreadSignal(t, thread.done, "startup did not finish after its worker returned")
	if !completed.Load() {
		t.Fatal("startup did not complete after its worker finished")
	}
}

func TestStartupCanWaitForNextFrame(t *testing.T) {
	var completed atomic.Bool
	itime.Start()
	co := New(nil)
	co.Create("startup", func(me Thread) {
		co.WaitNextFrameFor(me)
		completed.Store(true)
	})
	t.Cleanup(func() {
		co.RunAfterStop(nil)
	})
	for frame := 0; frame < 2; frame++ {
		updated := make(chan struct{})
		go func() {
			co.Update()
			close(updated)
		}()
		waitForThreadSignal(t, updated, "startup prevented the frame from returning")
		if got, want := completed.Load(), frame == 1; got != want {
			t.Fatalf("frame %d: completed = %v, want %v", frame, got, want)
		}
		itime.Update(1.0/30, 30)
	}
}

func TestUpdateWatchdogPreservesScriptsAndQueuedWork(t *testing.T) {
	co := New(nil)
	itime.Start()
	t.Cleanup(func() {
		co.RunAfterStop(nil)
	})

	deferredResumed := make(chan struct{})
	deferred := co.Create("next-frame", func(me Thread) {
		co.WaitNextFrameFor(me)
		close(deferredResumed)
	})
	co.JoinFirstSlice(deferred)

	now := time.Now()
	co.updateWatchdogNow = func() time.Time { return now }
	co.enqueueJob(&waitJob{kind: waitMainThread, call: func() {
		now = now.Add(updateWatchdogTimeout)
	}})

	resumed := make(chan struct{})
	thread := co.Create("slow-frame", func(me Thread) {
		co.WaitYield(me)
		close(resumed)
	})
	waitForThreadSignal(t, thread.firstSliceDone, "script did not reach its yield")

	queuedRan := false
	co.enqueueJob(&waitJob{kind: waitMainThread, call: func() { queuedRan = true }})
	co.Update()

	if thread.Stopped() || deferred.Stopped() {
		t.Fatal("watchdog canceled a script after a slow frame")
	}
	select {
	case <-resumed:
		t.Fatal("script resumed after the frame budget was exhausted")
	default:
	}
	select {
	case <-deferredResumed:
		t.Fatal("frame wait resumed before the next frame")
	default:
	}
	if queuedRan {
		t.Fatal("Update processed remaining work after the frame budget was exhausted")
	}

	created := co.Create("after-slow-frame", func(Thread) {})
	if created.Stopped() {
		t.Fatal("watchdog rejected a new script")
	}
	itime.Update(1.0/30, 30)
	co.Update()
	waitForThreadSignal(t, resumed, "script did not resume on the next Update")
	waitForThreadSignal(t, deferredResumed, "deferred wait was lost after a slow frame")
	if !queuedRan {
		t.Fatal("queued work was lost after a slow frame")
	}
}

func TestUpdateWatchdogReturnsOnRecursiveSpawnRetry(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	co := New(nil)
	var keepSpawning atomic.Bool
	keepSpawning.Store(true)
	clockStart := time.Now()
	var clockCalls atomic.Int64
	co.updateWatchdogNow = func() time.Time {
		if clockCalls.Add(1) == 1 {
			return clockStart
		}
		// Let Node finish the watchdog's async log write once expiry is observed.
		keepSpawning.Store(false)
		return clockStart.Add(updateWatchdogTimeout)
	}

	t.Cleanup(func() {
		keepSpawning.Store(false)
		co.RunAfterStop(nil)
	})

	var spawned atomic.Int64
	var spawnNext func(Thread)
	spawnNext = func(Thread) {
		spawned.Add(1)
		if keepSpawning.Load() {
			co.Create("recursive", spawnNext)
			runtime.Gosched()
		}
	}
	co.Create("recursive", spawnNext)

	updateDone := make(chan struct{})
	go func() {
		co.Update()
		close(updateDone)
	}()
	waitForThreadSignal(t, updateDone, "Update did not return after a retry exhausted the frame budget")
	keepSpawning.Store(false)
	co.drain()
	if clockCalls.Load() < 2 || spawned.Load() == 0 {
		t.Fatal("test did not exercise the watchdog on a recursive spawn retry")
	}
	if stats := co.UpdateStats(); stats.JobCount != 0 || stats.LoopIterations != 1 {
		t.Fatalf("processed %d wait jobs over %d iterations, want one retry", stats.JobCount, stats.LoopIterations)
	}
}

func TestResumeIgnoresCanceledThread(t *testing.T) {
	co := New(nil)
	thread := co.newThread("canceled")
	thread.Cancel()
	thread.resume()
	if thread.suspendState != suspendRunning {
		t.Fatal("Resume left a wake signal on a canceled thread")
	}
}

func TestWakeThreadDoesNotWakeCompletedThread(t *testing.T) {
	co := New(nil)

	thread := co.Create("completed", func(Thread) {})
	co.drain()

	co.wakeThread(thread)
	co.schedulerMu.Lock()
	_, exists := co.pendingExecution[thread]
	co.schedulerMu.Unlock()
	if exists {
		t.Fatal("stale resume job restored scheduler state for a completed thread")
	}
}

func TestUpdatePreservesJobsWithConcurrentMainThreadService(t *testing.T) {
	co := New(nil)
	itime.Start()
	t.Cleanup(func() {
		co.RunAfterStop(nil)
	})

	const producerCount, jobsPerProducer = 4, 100
	var resumed atomic.Int64
	for range producerCount {
		thread := co.Create("next-frame", func(me Thread) {
			co.WaitNextFrameFor(me)
			resumed.Add(1)
		})
		co.JoinFirstSlice(thread)
	}

	var calls [producerCount * jobsPerProducer]atomic.Int64
	var producers sync.WaitGroup
	for producer := range producerCount {
		producers.Go(func() {
			for index := range jobsPerProducer {
				co.enqueuePriorityJob(&waitJob{kind: waitMainThread, call: func() {
					calls[producer*jobsPerProducer+index].Add(1)
				}})
			}
		})
	}
	stopPump, pumpDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(pumpDone)
		for {
			select {
			case <-stopPump:
				return
			default:
				co.pumpMainThread()
			}
		}
	}()
	for range jobsPerProducer {
		co.Update()
	}
	producers.Wait()
	close(stopPump)
	waitForThreadSignal(t, pumpDone, "engine job service did not stop")
	co.Update()
	for index := range calls {
		if got := calls[index].Load(); got != 1 {
			t.Fatalf("engine job %d ran %d times, want once", index, got)
		}
	}
	if resumed.Load() != 0 {
		t.Fatal("deferred waits resumed in their issuing frame")
	}
	itime.Update(1.0/30, 30)
	co.Update()
	if got := resumed.Load(); got != producerCount {
		t.Fatalf("resumed %d deferred waits, want %d", got, producerCount)
	}
}

func TestCanceledWaitJobIsDiscardedBeforeItsDeadline(t *testing.T) {
	co := New(nil)
	itime.Start()

	resumed := false
	thread := co.Create("waiting", func(Thread) {
		co.Wait(math.MaxFloat64)
		resumed = true
	})
	co.JoinFirstSlice(thread)
	thread.Cancel()
	co.drain()
	co.Update()

	if resumed {
		t.Fatal("canceled wait resumed")
	}
	if current, deferred := co.currentJobCount(), co.deferredJobs.len(); current != 0 || deferred != 0 {
		t.Fatalf("canceled wait job remained queued: current=%d deferred=%d", current, deferred)
	}
}

func TestUpdateWaitsForJoinWaiterResumedByCompletingThread(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(4)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	const runs = 100
	for run := 0; run < runs; run++ {
		co := New(nil)
		var waiterRan atomic.Bool

		co.enqueueJob(&waitJob{
			kind: waitMainThread,
			call: func() {
				co.Create("controller", func(controller Thread) {
					var target Thread
					co.Create("waiter", func(waiter Thread) {
						co.Join(target)
						waiterRan.Store(true)
					})
					target = co.Create("target", func(target Thread) {
					})
				})
			},
		})

		co.Update()
		if !waiterRan.Load() {
			t.Fatalf("run %d: Update returned before the Join waiter resumed", run)
		}
	}
}

// enqueueJob injects scheduler work without publishing a blocked thread.
func (p *Coroutines) enqueueJob(job *waitJob) {
	p.schedulerMu.Lock()
	p.currentJobs.pushBack(job)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

func (p *Coroutines) currentJobCount() int {
	p.schedulerMu.Lock()
	defer p.schedulerMu.Unlock()
	return p.currentJobs.len()
}

func isSuspended(thread Thread) bool {
	if !thread.suspendMu.TryLock() {
		return false
	}
	defer thread.suspendMu.Unlock()
	return thread.suspendState == suspendWaiting
}

func joinFirstSliceAll(co *Coroutines, threads []Thread) {
	for _, thread := range threads {
		co.JoinFirstSlice(thread)
	}
}
