package coroutine

import "testing"

func TestRunSyncPreservesCallerAcrossWait(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
	completed := false
	co.RunSync("outer", func(parent Thread) {
		co.RunSync("nested", func(caller Thread) {
			if caller != parent || caller.Owner != "outer" {
				t.Error("nested call changed the thread or its owner")
			}
			co.WaitToDo(func() {})
			if co.Caller() != parent || co.Current() != parent {
				t.Error("nested wait did not resume its calling thread")
			}
			completed = true
		})
	})
	if !completed || co.LastThreadID() != 1 {
		t.Fatalf("completed=%v, created threads=%d", completed, co.LastThreadID())
	}
}

func TestRunSyncRejectsManagedExclusiveCallback(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
	var rejected, subsequent bool
	co.RunSync("outer", func(Thread) {
		result := co.invokeCallback(callbackExclusive, func() {
			co.RunSync("nested", func(Thread) { t.Error("exclusive callback reentered scripts") })
		})
		rejected = result.panicked && result.panicValue == ErrReentrantWait
		co.RunSync("subsequent", func(Thread) { subsequent = true })
	})
	if !rejected || !subsequent {
		t.Fatalf("rejected=%v, subsequent call ran=%v", rejected, subsequent)
	}
}

func TestRunSyncRejectsBetweenScriptsCallback(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
	co.RunBetweenScripts(func() {
		got := catchPanic(func() {
			co.RunSync(nil, func(Thread) { t.Error("exclusive callback reentered scripts") })
		})
		if got != ErrReentrantWait || co.LastThreadID() != 0 {
			t.Fatalf("panic=%v, created threads=%d", got, co.LastThreadID())
		}
	})
	co.RunSync(nil, func(Thread) {})
}
