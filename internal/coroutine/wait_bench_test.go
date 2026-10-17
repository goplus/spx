package coroutine

import (
	"context"
	"testing"
	"time"
)

func BenchmarkWaitToDo(b *testing.B) {
	co := New(nil)
	b.Cleanup(func() {
		if co.RunAfterStop(drainTestContext(b), nil) != nil {
			b.Error("external tasks did not drain")
		}
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
				if err := co.RunAfterStop(drainTestContext(b), nil); err != nil {
					b.Fatal(err)
				}
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
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := co.drain(ctx)
		cancel()
		if err != nil {
			b.Fatal("external task did not drain")
		}
	}
}
