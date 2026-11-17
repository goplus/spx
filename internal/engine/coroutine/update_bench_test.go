package coroutine

import (
	"fmt"
	"testing"

	"github.com/goplus/spx/v4/internal/core/clock"
)

func BenchmarkUpdateSleepingThreads(b *testing.B) {
	for _, count := range []int{1, 100, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			scheduler := New(nil)
			clock.Start()
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

func BenchmarkWaitJobEnqueue(b *testing.B) {
	for _, count := range []int{1, 100, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			scheduler := New(nil)
			var last *waitJob
			b.ReportAllocs()
			// Include fresh wait jobs and queue bookkeeping, without scheduling scripts.
			for b.Loop() {
				for range count {
					scheduler.currentJobs.pushBack(&waitJob{kind: waitTime, deadline: 1e12})
				}
				for range count {
					last = scheduler.currentJobs.popFront()
				}
			}
			if last == nil || last.deadline != 1e12 || scheduler.currentJobs.len() != 0 {
				b.Fatal("queue did not preserve and drain the wait jobs")
			}
		})
	}
}
