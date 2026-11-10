package coroutine

import (
	"testing"
	"time"
)

func BenchmarkWaitToDo(b *testing.B) {
	scheduler := New(nil)
	b.Cleanup(func() {
		scheduler.RunAfterStop(nil)
	})
	work := func() {}
	b.ReportAllocs()
	b.ResetTimer()
	thread := scheduler.Create("worker", func(Thread) {
		for range b.N {
			scheduler.WaitToDo(work)
		}
	})
	scheduler.Join(thread)
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
			scheduler := New(nil)
			b.Cleanup(func() {
				scheduler.RunAfterStop(nil)
			})
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					scheduler.Join(scheduler.createThread(scheduler.captureAdmission(), task))
				}
			})
		})
	}
}

func BenchmarkDrainExternalTask(b *testing.B) {
	scheduler := New(nil)
	b.ReportAllocs()
	for b.Loop() {
		if !scheduler.admitWorker(nil) {
			b.Fatal("external task was rejected")
		}
		// Include a short external operation; timer precision depends on the host.
		time.AfterFunc(100*time.Microsecond, scheduler.finishWorker)
		scheduler.drain()
	}
}
