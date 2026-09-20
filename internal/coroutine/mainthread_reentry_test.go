package coroutine

import (
	"testing"
	"time"
)

func TestExclusiveCallbackRejectsBlockedWaits(t *testing.T) {
	for _, test := range []struct {
		name string
		wait func(*Coroutines, Thread, *Latch)
	}{
		{"join", func(co *Coroutines, th Thread, _ *Latch) { co.Join(th) }},
		{"first yield", func(co *Coroutines, th Thread, _ *Latch) { co.JoinYieldedOrDone(th) }},
		{"latch", func(_ *Coroutines, _ Thread, latch *Latch) { latch.Wait() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := New(nil)
			latch := co.NewLatch()
			finished := make(chan struct{})
			guarded := make(chan any, 1)
			go func() {
				defer close(finished)
				var thread Thread
				co.RunBetweenScripts(func() {
					thread = co.Create("child", func(Thread) {})
					func() {
						defer func() { guarded <- recover() }()
						test.wait(co, thread, latch)
					}()
				})
				co.Join(thread)
				latch.Open()
				// Completed waits are safe even when the callback owns runMu.
				co.RunBetweenScripts(func() { test.wait(co, thread, latch) })
			}()
			select {
			case recovered := <-guarded:
				if recovered != ErrReentrantWait {
					t.Fatalf("blocked wait panic = %v", recovered)
				}
			case <-time.After(time.Second):
				t.Fatal("callback waited for a script while holding its execution lock")
			}
			waitForThreadSignal(t, finished, "callback did not release its execution lock")
		})
	}
}
