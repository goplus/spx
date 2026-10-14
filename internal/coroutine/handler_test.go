package coroutine

import "testing"

func TestRestartHandlerIgnoresRetiredCleanup(t *testing.T) {
	co := New(nil)
	state := NewHandlerState(RestartExisting)
	first, second, third := co.newThread("first"), co.newThread("second"), co.newThread("third")
	defer co.Stop(first)
	defer co.Stop(second)
	defer co.Stop(third)
	retireFirst := state.RegisterInvocation(first)
	retireSecond := state.RegisterInvocation(second)
	defer retireSecond()
	if !first.Stopped() || second.Stopped() {
		t.Fatal("restart did not replace the prior invocation")
	}
	retireFirst()
	retireThird := state.RegisterInvocation(third)
	defer retireThird()
	if !second.Stopped() || third.Stopped() {
		t.Fatal("retired cleanup released the current invocation")
	}
}

func TestIgnoreHandlerAllowsCanceledAndCompletedInvocations(t *testing.T) {
	co := New(nil)
	state := NewHandlerState(IgnoreWhileRunning)
	first, second := co.newThread("first"), co.newThread("second")
	defer co.Stop(first)
	defer co.Stop(second)
	retireFirst := state.RegisterInvocation(first)
	if cleanup := state.RegisterInvocation(second); cleanup != nil || !second.Stopped() {
		t.Fatal("overlapping invocation was admitted")
	}
	co.Stop(first)
	third := co.newThread("third")
	defer co.Stop(third)
	retireThird := state.RegisterInvocation(third)
	if retireThird == nil || third.Stopped() {
		t.Fatal("canceled invocation retained admission")
	}
	retireFirst()
	fourth := co.newThread("fourth")
	defer co.Stop(fourth)
	if cleanup := state.RegisterInvocation(fourth); cleanup != nil || !fourth.Stopped() {
		t.Fatal("retired cleanup released the current invocation")
	}
	retireThird()
	fifth := co.newThread("fifth")
	defer co.Stop(fifth)
	retireFifth := state.RegisterInvocation(fifth)
	if retireFifth == nil || fifth.Stopped() {
		t.Fatal("completed invocation retained admission")
	}
	retireFifth()
}
