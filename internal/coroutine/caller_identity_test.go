package coroutine

import (
	"testing"
	"time"
)

func TestExternalCallerCannotYieldCurrentCoroutine(t *testing.T) {
	co := New(nil)

	started := make(chan Thread, 1)
	release := make(chan struct{})
	th := co.Create("owner", func(me Thread) {
		started <- me
		<-release
	})
	t.Cleanup(func() {
		close(release)
		if !co.StopAllAndWait(time.Second) {
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

func TestExternalWaitDoesNotChangeSchedulerState(t *testing.T) {
	co := New(nil)
	started, release := make(chan struct{}), make(chan struct{})
	thread := co.Create("active", func(Thread) {
		close(started)
		<-release
	})
	<-started
	t.Cleanup(func() {
		close(release)
		if !co.StopAllAndWait(time.Second) {
			t.Error("coroutine did not stop")
		}
	})
	for _, test := range []struct {
		name string
		wait func(Thread)
	}{
		{"yield", co.WaitYield},
		{"frame", co.WaitNextFrameFor},
		{"loop", co.YieldLoopFor},
		{"round", co.YieldToNextRoundFor},
	} {
		t.Run(test.name, func(t *testing.T) {
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				test.wait(thread)
			}()
			if recovered != ErrCannotYieldANonrunningThread {
				t.Fatalf("panic = %v, want invalid caller", recovered)
			}
			co.schedulerMu.Lock()
			defer co.schedulerMu.Unlock()
			_, runnable := co.runnableThreads[thread]
			if !runnable || co.currentJobs.Count() != 0 {
				t.Fatal("rejected wait changed the running script or queued work")
			}
		})
	}
}
