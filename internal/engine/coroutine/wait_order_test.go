package coroutine

import (
	"slices"
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/core/clock"
)

func TestInterruptedMixedWaitsPreserveClockAndExecutionOrder(t *testing.T) {
	scheduler := New(nil)
	clock.Start()
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	const count = waitScanChunkSize * 2
	resumed := make(chan int, count)
	for i := range count {
		th := scheduler.Create(nil, func(me Thread) {
			if i%2 == 0 {
				scheduler.Wait(10)
			} else {
				scheduler.YieldToNextRoundFor(me)
			}
			resumed <- i
		})
		scheduler.JoinFirstSlice(th)
	}
	// Deferred and next-round waits may be scanned in a different order from
	// registration. Execution order must still be determined by the ready queue.
	scheduler.Update()
	clock.Update(1, 30)
	expireUpdateAtFirstCheckpoint(scheduler)
	scheduler.Update()
	if got := len(resumed); got != 0 {
		t.Fatalf("partial batch resumed %d scripts", got)
	}
	if batch := &scheduler.waitBatch; !batch.active || batch.remaining != count-waitScanChunkSize {
		t.Fatalf("batch active/remaining = %v/%d, want true/%d", batch.active, batch.remaining, count-waitScanChunkSize)
	}

	// Crossing the time deadline cannot change the interrupted batch's clock.
	clock.Update(20, 30)
	scheduler.updateWatchdogNow = time.Now
	scheduler.Update()
	checkOrder := func(first int) {
		t.Helper()
		var got, want []int
		for len(resumed) != 0 {
			got = append(got, <-resumed)
		}
		for i := first; i < count; i += 2 {
			want = append(want, i)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("resumed order = %v, want %v", got, want)
		}
	}
	checkOrder(1)
	clock.Update(1, 30)
	scheduler.Update()
	checkOrder(0)
}
