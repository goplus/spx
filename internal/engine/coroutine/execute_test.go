package coroutine

import "testing"

func TestRunSyncPreservesCallerAcrossWait(t *testing.T) {
	scheduler := New(nil)
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	completed := false
	scheduler.RunSync("outer", func(parent Thread) {
		scheduler.RunSync("nested", func(caller Thread) {
			if caller != parent || caller.Owner != "outer" {
				t.Error("nested call changed the thread or its owner")
			}
			scheduler.WaitToDo(func() {})
			if scheduler.Caller() != parent || scheduler.Current() != parent {
				t.Error("nested wait did not resume its calling thread")
			}
			completed = true
		})
	})
	if !completed || scheduler.LastThreadID() != 1 {
		t.Fatalf("completed=%v, created threads=%d", completed, scheduler.LastThreadID())
	}
}

func TestRunSyncRejectsManagedExclusiveCallback(t *testing.T) {
	scheduler := New(nil)
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	var rejected, subsequent bool
	scheduler.RunSync("outer", func(Thread) {
		result := scheduler.invokeCallback(callbackExclusive, func() {
			scheduler.RunSync("nested", func(Thread) { t.Error("exclusive callback reentered scripts") })
		})
		rejected = result.panicked && result.panicValue == ErrReentrantWait
		scheduler.RunSync("subsequent", func(Thread) { subsequent = true })
	})
	if !rejected || !subsequent {
		t.Fatalf("rejected=%v, subsequent call ran=%v", rejected, subsequent)
	}
}

func TestRunSyncRejectsBetweenScriptsCallback(t *testing.T) {
	scheduler := New(nil)
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	scheduler.RunBetweenScripts(func() {
		got := catchPanic(func() {
			scheduler.RunSync(nil, func(Thread) { t.Error("exclusive callback reentered scripts") })
		})
		if got != ErrReentrantWait || scheduler.LastThreadID() != 0 {
			t.Fatalf("panic=%v, created threads=%d", got, scheduler.LastThreadID())
		}
	})
	scheduler.RunSync(nil, func(Thread) {})
}
