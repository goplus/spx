package coroutine

import (
	"fmt"
	"testing"

	itime "github.com/goplus/spx/v4/internal/time"
)

func BenchmarkUpdateSleepingThreads(b *testing.B) {
	for _, count := range []int{1, 100, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			scheduler := New(nil)
			itime.Start()
			// Measure scheduler bookkeeping without goroutine startup or script work.
			for range count {
				thread := scheduler.newThread("sleeper")
				scheduler.blockThread(thread)
				scheduler.enqueueJob(&waitJob{thread: thread, kind: waitTime, deadline: 1e12})
			}
			scheduler.Update()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				scheduler.Update()
			}
		})
	}
}
