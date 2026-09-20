package coroutine

import (
	"fmt"
	"testing"

	itime "github.com/goplus/spx/v3/internal/time"
)

func BenchmarkUpdateSleepingThreads(b *testing.B) {
	for _, count := range []int{1, 100, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			co := New(nil)
			itime.Start(nil)
			// Measure scheduler bookkeeping without goroutine startup or script work.
			for range count {
				thread := co.newThread("sleeper")
				co.setThreadState(thread, threadBlocked)
				co.currentJobs.PushBack(&WaitJob{Th: thread, Type: waitTypeTime, Time: 1e12})
			}
			co.Update()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				co.Update()
			}
		})
	}
}
