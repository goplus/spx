package coroutine

import (
	"testing"
	"time"
)

func BenchmarkWaitToDo(b *testing.B) {
	co := New(nil)
	b.Cleanup(func() {
		if !co.RunAfterStopAll(time.Second, nil) {
			b.Error("external tasks did not drain")
		}
	})
	work := func() {}
	b.ReportAllocs()
	b.ResetTimer()
	thread := co.Create("worker", func(Thread) int {
		for range b.N {
			co.WaitToDo(work)
		}
		return 0
	})
	co.Join(thread)
}
