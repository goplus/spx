package coroutine

import (
	"runtime"
	sdebug "runtime/debug"
	"testing"
	"time"

	"github.com/goplus/spx/v2/internal/engine/platform"
)

func TestUpdateReadsGCStatsOnlyWhenPerfDebugEnabled(t *testing.T) {
	co := New(nil)
	co.OnInited()

	originalReadGCStats := co.readGCStats
	t.Cleanup(func() {
		co.readGCStats = originalReadGCStats
		lastDebugUpdateStats = UpdateJobsStats{}
	})

	var calls int
	co.readGCStats = func(*sdebug.GCStats) {
		calls++
	}

	co.Update()
	if calls != 0 {
		t.Fatalf("expected GC stats collection to be disabled by default, got %d calls", calls)
	}
	if co.GetLastUpdateStats().GCStatsEnabled {
		t.Fatal("expected GC stats to be marked disabled by default")
	}

	co.SetPerfDebug(true)
	co.Update()
	if calls != 2 {
		t.Fatalf("expected GC stats to be read twice when perf debug is enabled, got %d calls", calls)
	}
	if !co.GetLastUpdateStats().GCStatsEnabled {
		t.Fatal("expected GC stats to be marked enabled when perf debug is on")
	}
}

func TestWaitMainThreadFastPathOnMainThread(t *testing.T) {
	co := New(nil)
	called := false

	platform.RunOnMainThread(func() {
		co.WaitMainThread(func() {
			called = true
		})
	})

	if !called {
		t.Fatal("WaitMainThread should execute immediately on the main thread")
	}
}

func TestWaitForChanReceivesValue(t *testing.T) {
	co := New(nil)
	ch := make(chan int)
	done := make(chan struct{})

	co.CreateAndStart(false, nil, func(me Thread) int {
		defer close(done)
		var value int
		WaitForChan(co, ch, &value)
		if value != 7 {
			t.Fatalf("expected received value 7, got %d", value)
		}
		return 0
	})

	deadline := time.Now().Add(time.Second)
	for {
		co.waitMutex.Lock()
		waitingCount := len(co.waiting)
		co.waitMutex.Unlock()
		if waitingCount > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("coroutine did not enter wait state")
		}
		runtime.Gosched()
	}

	ch <- 7

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("coroutine did not resume after channel receive")
	}
}

func TestWaitForChanDoesNotKeepReceiverAfterCancel(t *testing.T) {
	co := New(nil)
	ch := make(chan int)
	done := make(chan struct{})
	var th Thread

	th = co.CreateAndStart(false, nil, func(me Thread) int {
		defer close(done)
		var value int
		WaitForChan(co, ch, &value)
		return 0
	})

	deadline := time.Now().Add(time.Second)
	for {
		co.waitMutex.Lock()
		waiting := co.waiting[th]
		co.waitMutex.Unlock()
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("coroutine did not enter wait state")
		}
		runtime.Gosched()
	}

	co.StopIf(func(candidate Thread) bool {
		return candidate == th
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("coroutine did not exit after cancel")
	}

	deadline = time.Now().Add(time.Second)
	for {
		select {
		case ch <- 1:
		case <-time.After(20 * time.Millisecond):
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("channel receiver should eventually exit after cancel")
		}
	}
}
