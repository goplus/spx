package coroutine

import (
	"slices"
	"testing"

	itime "github.com/goplus/spx/v4/internal/time"
)

func (p *Scheduler) queueMainThreadCallForTest(caller Thread, call func()) *mainThreadCall {
	pending := &mainThreadCall{caller: caller, call: call, done: make(chan callbackResult, 1)}
	p.enqueueMainThreadCall(pending)
	return pending
}

func TestMainThreadQueuePreservesPriorityAndStats(t *testing.T) {
	for _, pump := range []bool{false, true} {
		name := "Update"
		if pump {
			name = "pump"
		}
		t.Run(name, func(t *testing.T) {
			scheduler := New(nil)
			itime.Start()
			t.Cleanup(func() { scheduler.RunAfterStop(nil) })
			waiter := scheduler.Create("next-frame", func(Thread) { scheduler.WaitNextFrame() })
			scheduler.JoinFirstSlice(waiter)
			frame, round := itime.Frame(), scheduler.ScriptRound()
			var order []int
			scheduler.queueMainThreadCallForTest(nil, func() { order = append(order, 1) })
			scheduler.queueMainThreadCallForTest(nil, func() { order = append(order, 2) })
			canceled := scheduler.newThread("canceled")
			canceled.Cancel()
			pending := scheduler.queueMainThreadCallForTest(canceled, func() { t.Error("canceled call ran") })
			if pump {
				for range 3 {
					scheduler.pumpMainThread()
				}
				if scheduler.currentJobCount() != 1 || scheduler.waitBatch.active {
					t.Fatal("pumping engine calls consumed or scanned a script wait")
				}
			}
			scheduler.Update()
			if !slices.Equal(order, []int{2, 1}) {
				t.Fatalf("main-thread order = %v, want [2 1]", order)
			}
			if itime.Frame() != frame || scheduler.ScriptRound() != round {
				t.Fatal("servicing engine calls advanced a frame or script round")
			}
			wantJobs, wantMain := 4, 2 // One script wait plus three engine calls, one canceled.
			if pump {
				wantJobs, wantMain = 1, 0 // Pumped work is outside Update's statistics.
			}
			if stats := scheduler.UpdateStats(); stats.JobCount != wantJobs || stats.MainThreadCount != wantMain || stats.DeferredCount != 1 {
				t.Fatalf("job/main/deferred counts = %d/%d/%d, want %d/%d/1", stats.JobCount, stats.MainThreadCount, stats.DeferredCount, wantJobs, wantMain)
			}
			if got := catchPanic(pending.wait); got != ErrAbortThread {
				t.Fatalf("canceled call wait panic = %v, want %v", got, ErrAbortThread)
			}
		})
	}
}

func TestMainThreadQueuePrioritizesCallsEnqueuedDuringDispatch(t *testing.T) {
	scheduler := New(nil)
	var order []int
	scheduler.queueMainThreadCallForTest(nil, func() { order = append(order, 1) })
	scheduler.queueMainThreadCallForTest(nil, func() {
		order = append(order, 2)
		scheduler.queueMainThreadCallForTest(nil, func() { order = append(order, 3) })
	})
	scheduler.Update()
	if !slices.Equal(order, []int{2, 3, 1}) {
		t.Fatalf("main-thread order = %v, want [2 3 1]", order)
	}
	if stats := scheduler.UpdateStats(); stats.JobCount != 3 || stats.MainThreadCount != 3 {
		t.Fatalf("job/main counts = %d/%d, want 3/3", stats.JobCount, stats.MainThreadCount)
	}
}
