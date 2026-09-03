package coroutine

import (
	"testing"
	"time"
)

func TestExternalCallerCannotYieldCurrentCoroutine(t *testing.T) {
	co := New(nil)
	co.OnInited()

	started := make(chan Thread, 1)
	release := make(chan struct{})
	th := co.Create("owner", func(me Thread) int {
		started <- me
		<-release
		return 0
	})
	t.Cleanup(func() {
		close(release)
		if !co.AbortAllAndWait(time.Second) {
			t.Fatal("coroutine did not stop during cleanup")
		}
	})

	var current Thread
	select {
	case current = <-started:
	case <-time.After(time.Second):
		t.Fatal("coroutine did not start")
	}
	if co.Current() != current {
		t.Fatalf("Current() = %v, want running coroutine %v", co.Current(), current)
	}

	panicValue := func() (recovered any) {
		defer func() { recovered = recover() }()
		co.WaitToDo(func() {})
		return nil
	}()
	if panicValue != nil {
		t.Fatalf("external WaitToDo unexpectedly panicked: %v", panicValue)
	}
	if co.Current() != current {
		t.Fatal("external WaitToDo changed scheduler ownership")
	}
	panicValue = func() (recovered any) {
		defer func() { recovered = recover() }()
		co.Yield(current)
		return nil
	}()
	if panicValue != ErrCannotYieldANonrunningThread {
		t.Fatalf("external Yield panic = %v, want %v", panicValue, ErrCannotYieldANonrunningThread)
	}

	_ = th
}
