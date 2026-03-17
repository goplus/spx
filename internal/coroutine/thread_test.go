package coroutine

import (
	"runtime"
	"testing"
	"time"
)

type namedThreadObj struct{}

func (*namedThreadObj) Name() string {
	return "named-thread"
}

type invalidNamedThreadObj struct{}

func (*invalidNamedThreadObj) Name(_ string) string {
	return "invalid"
}

func canReenterCoroutineMutex(co *Coroutines) bool {
	if !co.mutex.TryLock() {
		return false
	}
	co.mutex.Unlock()
	return true
}

func TestResolveThreadNameUsesNameMethodWithoutReflectionCall(t *testing.T) {
	if got := resolveThreadName(&namedThreadObj{}); got != "named-thread" {
		t.Fatalf("expected name from interface method, got %q", got)
	}
}

func TestResolveThreadNameFallsBackForInvalidNameSignature(t *testing.T) {
	if got := resolveThreadName(&invalidNamedThreadObj{}); got != "*invalidNamedThreadObj" {
		t.Fatalf("expected fallback type name, got %q", got)
	}
}

func TestStopIfStopsActiveThread(t *testing.T) {
	co := New(nil)
	started := make(chan Thread, 1)
	done := make(chan struct{})

	co.CreateAndStart(true, "worker", func(me Thread) int {
		started <- me
		for {
			select {
			case <-me.Context().Done():
				close(done)
				return 0
			default:
				runtime.Gosched()
			}
		}
	})

	var th Thread
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case th = <-started:
	case <-timer.C:
		t.Fatal("coroutine did not start")
	}

	co.mutex.Lock()
	_, suspended := co.suspended[th]
	co.mutex.Unlock()
	if suspended {
		t.Fatal("test precondition failed: spinning thread should not be in suspended map")
	}

	co.StopIf(func(candidate Thread) bool {
		return candidate == th
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatal("active coroutine did not stop after StopIf")
	}
}

func TestStopIfEvaluatesFilterWithoutHoldingMutex(t *testing.T) {
	co := New(nil)
	started := make(chan Thread, 1)
	done := make(chan struct{})

	co.CreateAndStart(true, "worker", func(me Thread) int {
		started <- me
		for {
			select {
			case <-me.Context().Done():
				close(done)
				return 0
			default:
				runtime.Gosched()
			}
		}
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	var th Thread
	select {
	case th = <-started:
	case <-timer.C:
		t.Fatal("coroutine did not start")
	}

	var reacquired bool
	stopDone := make(chan struct{})
	go func() {
		co.StopIf(func(candidate Thread) bool {
			reacquired = canReenterCoroutineMutex(co)
			return candidate == th
		})
		close(stopDone)
	}()

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-stopDone:
	case <-timer.C:
		t.Fatal("StopIf deadlocked when filter re-entered the coroutine mutex")
	}
	if !reacquired {
		t.Fatal("StopIf evaluated filter while holding the coroutine mutex")
	}

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatal("active coroutine did not stop after StopIf")
	}
}
