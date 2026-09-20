package coroutine

import (
	"runtime"
	"testing"
	"time"
)

func TestWaitForChanCommitsResultOnlyAfterResume(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() {
		if !co.RunAfterStopAll(time.Second, nil) {
			t.Error("receive did not drain")
		}
	})
	input := make(chan int)
	value := 0
	thread := co.Create("receiver", func(Thread) {
		value = WaitForChan(co, input)
	})
	waitForThreadSignal(t, thread.yieldedOrDone, "receiver did not wait")

	// Another script slice owns execution while the receive completes.
	co.runMu.Lock()
	func() {
		defer co.runMu.Unlock()
		select {
		case input <- 7:
		case <-time.After(time.Second):
			t.Fatal("receiver did not accept a value")
		}
		deadline := time.Now().Add(time.Second)
		for {
			co.schedulerMu.Lock()
			_, ready := co.runnableThreads[thread]
			co.schedulerMu.Unlock()
			if ready {
				break
			}
			if time.Now().After(deadline) {
				co.Stop(thread)
				t.Fatal("receive did not wake its caller")
			}
			runtime.Gosched()
		}
		if value != 0 {
			t.Errorf("result committed without script execution: %d", value)
		}
		co.Stop(thread)
	}()
	waitForThreadSignal(t, thread.done, "canceled receiver did not finish")
	if value != 0 {
		t.Fatalf("canceled receive committed its result: %d", value)
	}
}

func TestWaitForChanTracksPendingReceive(t *testing.T) {
	co := New(nil)
	value := 7
	thread := co.Create("receiver", func(Thread) {
		value = WaitForChan[int](co, nil)
	})
	co.JoinYieldedOrDone(thread)
	co.threadsMu.Lock()
	pending := co.workerCount
	co.threadsMu.Unlock()
	if pending != 1 {
		t.Errorf("tracked receivers = %d, want 1", pending)
	}
	if !co.RunAfterStopAll(time.Second, func() {
		co.threadsMu.Lock()
		defer co.threadsMu.Unlock()
		if co.workerCount != 0 {
			t.Error("reset ran before the receiver drained")
		}
	}) {
		t.Fatal("pending receive did not stop")
	}
	if value != 7 {
		t.Fatalf("canceled receive changed its result: %d", value)
	}
}

func TestCancelStopsSuspendedThread(t *testing.T) {
	co := New(nil)
	thread := co.Create("suspended", func(me Thread) {
		co.WaitYield(me)
		t.Error("canceled script continued")
	})
	t.Cleanup(func() { co.StopAllAndWait(time.Second) })
	co.JoinYieldedOrDone(thread)
	thread.Cancel()
	waitForThreadSignal(t, thread.done, "Cancel did not wake the suspended script")
	if !thread.Stopped() || thread.Context().Err() == nil {
		t.Fatal("Cancel did not publish a stop request")
	}
}
