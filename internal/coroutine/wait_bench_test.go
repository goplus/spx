package coroutine

import (
	"testing"
	"time"
)

func BenchmarkWaitToDo(b *testing.B) {
	co := New(nil)
	b.Cleanup(func() {
		co.RunAfterStop(nil)
	})
	work := func() {}
	b.ReportAllocs()
	b.ResetTimer()
	thread := co.Create("worker", func(Thread) {
		for range b.N {
			co.WaitToDo(work)
		}
	})
	co.Join(thread)
}

func BenchmarkCreateParallel(b *testing.B) {
	for _, registered := range []bool{false, true} {
		name := "plain"
		task := Task{Run: func(Thread) {}}
		if registered {
			name = "registered"
			task.RegisterInvocation = func(Thread) func() { return nil }
		}
		b.Run(name, func(b *testing.B) {
			co := New(nil)
			b.Cleanup(func() {
				co.RunAfterStop(nil)
			})
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					co.Join(co.createThread(co.captureAdmission(), task))
				}
			})
		})
	}
}

func BenchmarkDrainExternalTask(b *testing.B) {
	co := New(nil)
	b.ReportAllocs()
	for b.Loop() {
		if !co.admitWorker(nil) {
			b.Fatal("external task was rejected")
		}
		// Include a short external operation; timer precision depends on the host.
		time.AfterFunc(100*time.Microsecond, co.finishWorker)
		co.drain()
	}
}
