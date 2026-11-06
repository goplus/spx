/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	itime "github.com/goplus/spx/v4/internal/time"
)

func TestWaitBatchPreservesOrderAcrossWatchdog(t *testing.T) {
	for _, wait := range []struct {
		name    string
		yield   func(*Coroutines, Thread)
		advance bool
	}{
		{"frame", (*Coroutines).WaitNextFrameFor, true},
		{"loop", (*Coroutines).YieldLoopFor, false},
	} {
		for _, count := range []int{2, 256} {
			t.Run(fmt.Sprintf("%s/%d", wait.name, count), func(t *testing.T) {
				co := New(nil)
				itime.Start()
				t.Cleanup(func() { co.RunAfterStop(nil) })

				var mu sync.Mutex
				var order []string
				record := func(event string) {
					mu.Lock()
					order = append(order, event)
					mu.Unlock()
				}
				for i := range count {
					thread := co.Create(nil, func(me Thread) {
						wait.yield(co, me)
						switch i {
						case 0:
							record("A")
							co.Create(nil, func(Thread) { record("C") })
						case count - 1:
							record("B")
						}
					})
					co.JoinFirstSlice(thread)
				}
				co.RunBetweenScripts(func() {})
				if wait.advance {
					itime.Update(1.0/30, 30)
				}

				// Expire at the first watchdog checkpoint, including when the
				// entire small batch has been scanned but not yet published.
				expireUpdateAtFirstCheckpoint(co)
				co.Update()
				if got := co.UpdateStats().JobCount; got == 0 || count == 256 && got >= count {
					t.Fatalf("scanned %d of %d waits before watchdog, want bounded progress", got, count)
				}

				// The engine synchronously runs dispatchers before its next Update.
				// A suspended wait batch must allow these scripts to finish.
				dispatcher := co.Create(nil, func(Thread) { record("dispatcher") })
				waitForThreadSignal(t, dispatcher.done, "incomplete wait batch blocked the engine dispatcher")
				want := []string{"dispatcher"}
				// A priority engine call arriving during the pause must neither
				// publish the partial batch nor consume its remaining wait count.
				co.queueMainThreadCallForTest(nil, func() { record("main") })
				expireUpdateAtFirstCheckpoint(co)
				co.Update()
				if stats := co.UpdateStats(); stats.JobCount != 1 || stats.MainThreadCount != 1 {
					t.Fatalf("engine call consumed waiting scripts: %+v", stats)
				}
				want = append(want, "main")
				mu.Lock()
				got := slices.Clone(order)
				mu.Unlock()
				if !slices.Equal(got, want) {
					t.Fatalf("scripts ran while the wait batch was unpublished: %v", got)
				}

				co.updateWatchdogNow = time.Now
				itime.Update(1.0/30, 30)
				co.Update()
				mu.Lock()
				got = slices.Clone(order)
				mu.Unlock()
				want = append(want, "A", "B", "C")
				if !slices.Equal(got, want) {
					t.Fatalf("resumed order = %v, want %v", got, want)
				}
			})
		}
	}
}

func TestInterruptedWaitBatchKeepsClockSnapshot(t *testing.T) {
	co := New(nil)
	itime.Start()
	t.Cleanup(func() { co.RunAfterStop(nil) })
	var resumed atomic.Int32
	const count = 256
	for range count {
		thread := co.Create(nil, func(Thread) {
			co.Wait(1)
			resumed.Add(1)
		})
		co.JoinFirstSlice(thread)
	}
	co.Update()
	itime.Update(0.5, 30)
	expireUpdateAtFirstCheckpoint(co)
	co.Update()
	if got := co.UpdateStats().JobCount; got == 0 || got >= count {
		t.Fatalf("processed %d waits, want a partially scanned batch", got)
	}

	// All waits were unexpired at the original batch clock. Crossing their
	// deadline while paused must not recheck only the already-scanned prefix.
	itime.Update(0.5, 30)
	co.updateWatchdogNow = time.Now
	co.Update()
	if got := resumed.Load(); got != 0 {
		t.Fatalf("%d waits resumed using a different clock from the rest of their batch", got)
	}
	itime.Update(1.0/30, 30)
	co.Update()
	if got := resumed.Load(); got != count {
		t.Fatalf("resumed %d waits after the next frame, want %d", got, count)
	}
}

func TestInterruptedWaitBatchAllowsCancelAndRestart(t *testing.T) {
	co := New(nil)
	itime.Start()
	t.Cleanup(co.StopAll)
	var cleaned, resumed atomic.Int32
	threads := make([]Thread, 256)
	for i := range threads {
		threads[i] = co.Create(nil, func(Thread) {
			defer cleaned.Add(1)
			co.WaitNextFrame()
			resumed.Add(1)
		})
		co.JoinFirstSlice(threads[i])
	}
	co.Update()
	itime.Update(1.0/30, 30)

	expireUpdateAtFirstCheckpoint(co)
	co.Update()
	if got := co.UpdateStats().JobCount; got == 0 || got >= len(threads) {
		t.Fatalf("watchdog processed %d of %d waits, want an incomplete batch", got, len(threads))
	}
	if got := resumed.Load(); got != 0 {
		t.Fatalf("%d scripts ran before the wait batch was published", got)
	}

	// Cancellation must wake both an unpublished eligible script and a script
	// whose wait job has not yet been scanned, without another Update.
	co.Stop(threads[0])
	co.Stop(threads[len(threads)-1])
	waitForThreadSignal(t, threads[0].done, "staged script did not finish cancellation")
	waitForThreadSignal(t, threads[len(threads)-1].done, "unscanned script did not finish cancellation")
	co.schedulerMu.Lock()
	for _, thread := range []Thread{threads[0], threads[len(threads)-1]} {
		_, published := co.pendingExecution[thread]
		_, staged := co.waitBatch.runnable.pendingExecution[thread]
		if thread.readyHeap != nil || published || staged {
			t.Error("finished script remains in runnable state")
		}
	}
	co.schedulerMu.Unlock()
	drained := make(chan struct{})
	go func() {
		co.RunAfterStop(nil)
		close(drained)
	}()
	waitForThreadSignal(t, drained, "interrupted wait batch prevented shutdown from draining")
	if got := cleaned.Load(); got != int32(len(threads)) {
		t.Fatalf("cleanup count = %d, want %d", got, len(threads))
	}
	if got := resumed.Load(); got != 0 {
		t.Fatalf("%d canceled scripts resumed normally", got)
	}

	co.updateWatchdogNow = time.Now
	updated := make(chan struct{})
	go func() {
		co.Update()
		close(updated)
	}()
	waitForThreadSignal(t, updated, "canceled wait batch left permanently pending execution")

	restarted := co.Create(nil, func(Thread) { co.WaitNextFrame() })
	waitForThreadSignal(t, restarted.firstSliceDone, "scheduler did not accept new work after shutdown")
	itime.Update(1.0/30, 30)
	co.Update()
	waitForThreadSignal(t, restarted.done, "new script did not resume after shutdown")
}

func expireUpdateAtFirstCheckpoint(co *Coroutines) {
	now := time.Now()
	first := true
	co.updateWatchdogNow = func() time.Time {
		if first {
			first = false
			return now
		}
		return now.Add(updateWatchdogTimeout)
	}
}
