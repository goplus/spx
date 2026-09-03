package coroutine

import (
	"testing"
	"time"
)

func TestRunTaskTracksNilPanic(t *testing.T) {
	t.Setenv("GODEBUG", "panicnil=1")
	result := runTask(func() { panic(nil) })
	if !result.panicked {
		t.Fatal("runTask treated a nil panic as successful completion")
	}
	if result.panicValue != nil {
		t.Fatalf("panic value = %v, want nil", result.panicValue)
	}
}

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

func TestWaitToDoPropagatesNilWorkerPanic(t *testing.T) {
	t.Setenv("GODEBUG", "panicnil=1")
	co := New(func(string, string) {})
	co.OnInited()
	continued := make(chan struct{}, 1)
	thread := co.CreateAndStart(true, "caller", func(Thread) int {
		co.WaitToDo(func() { panic(nil) })
		continued <- struct{}{}
		return 0
	})

	select {
	case <-thread.done:
	case <-time.After(time.Second):
		t.Fatal("nil worker panic did not complete the waiting coroutine")
	}
	select {
	case <-continued:
		t.Fatal("coroutine continued after a nil worker panic")
	default:
	}
}
