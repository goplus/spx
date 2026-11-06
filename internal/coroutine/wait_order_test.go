package coroutine

import (
	"slices"
	"testing"
	"time"

	itime "github.com/goplus/spx/v4/internal/time"
)

func TestInterruptedMixedWaitsPreserveClockAndExecutionOrder(t *testing.T) {
	co := New(nil)
	itime.Start()
	t.Cleanup(func() { co.RunAfterStop(nil) })
	const count = waitScanChunkSize * 2
	resumed := make(chan int, count)
	for i := range count {
		th := co.Create(nil, func(me Thread) {
			if i%2 == 0 {
				co.Wait(10)
			} else {
				co.YieldToNextRoundFor(me)
			}
			resumed <- i
		})
		co.JoinFirstSlice(th)
	}
	// Deferred and next-round waits may be scanned in a different order from
	// registration. Execution order must still be determined by the ready queue.
	co.Update()
	itime.Update(1, 30)
	expireUpdateAtFirstCheckpoint(co)
	co.Update()
	if got := len(resumed); got != 0 {
		t.Fatalf("partial batch resumed %d scripts", got)
	}
	if batch := &co.waitBatch; !batch.active || batch.remaining != count-waitScanChunkSize {
		t.Fatalf("batch active/remaining = %v/%d, want true/%d", batch.active, batch.remaining, count-waitScanChunkSize)
	}

	// Crossing the time deadline cannot change the interrupted batch's clock.
	itime.Update(20, 30)
	co.updateWatchdogNow = time.Now
	co.Update()
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
	itime.Update(1, 30)
	co.Update()
	checkOrder(0)
}
