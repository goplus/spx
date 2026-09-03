package coroutine

import (
	"testing"
	"time"
)

func TestWaitToDoPropagatesWorkerPanic(t *testing.T) {
	panicReported := make(chan any, 1)
	co := New(func(name, stack string) { panicReported <- "worker failure" })
	co.OnInited()
	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.WaitToDo(func() { panic("worker failure") })
		return 0
	})

	select {
	case recovered := <-panicReported:
		if recovered != "worker failure" {
			t.Fatalf("panic report = %v, want worker failure", recovered)
		}
	case <-time.After(time.Second):
		t.Fatal("worker panic did not complete the waiting coroutine")
	}
	if !co.AbortAllAndWait(time.Second) {
		t.Fatal("coroutine did not stop during cleanup")
	}
}
