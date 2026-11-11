package coroutine

import (
	"runtime"
	"testing"
	"time"
)

func TestInvokeCallbackTracksNilPanic(t *testing.T) {
	t.Setenv("GODEBUG", "panicnil=1")
	scheduler := New(nil)
	result := scheduler.invokeCallback(callbackNoDrain, func() { panicWithValue(nil) })
	if !result.panicked {
		t.Fatal("invokeCallback treated a nil panic as successful completion")
	}
	if result.panicValue != nil {
		t.Fatalf("panic value = %v, want nil", result.panicValue)
	}
}

func TestRegisterInvocationRejectsSynchronousDrain(t *testing.T) {
	for _, managed := range []bool{false, true} {
		name := "external"
		if managed {
			name = "managed"
		}
		t.Run(name, func(t *testing.T) {
			checkDrainReentry(t, func() {
				scheduler := New(nil)
				var recovered any
				var threads []Thread
				ran := false
				start := func() {
					threads = scheduler.StartBatch([]Task{{
						RegisterInvocation: func(Thread) func() {
							recovered = catchPanic(func() { scheduler.RunAfterStop(nil) })
							return nil
						},
						Run: func(Thread) { ran = true },
					}}, BatchAsync)
				}
				var parent Thread
				if managed {
					parent = scheduler.Create("parent", func(Thread) { start() })
					<-parent.done
				} else {
					start()
				}
				<-threads[0].done
				if recovered != ErrReentrantWait {
					t.Errorf("setup drain panic = %v, want %v", recovered, ErrReentrantWait)
				}
				if threads[0].Stopped() || parent != nil && parent.Stopped() || !ran {
					t.Error("rejected setup drain canceled a thread or prevented its run")
				}
				if scheduler.admissionClosed() {
					t.Error("rejected setup drain closed admission")
				}
			})
		})
	}
}

func TestCleanupRejectsSynchronousDrain(t *testing.T) {
	checkDrainReentry(t, func() {
		scheduler := New(nil)
		var recovered any
		threads := scheduler.StartBatch([]Task{{
			RegisterInvocation: func(Thread) func() {
				return func() {
					recovered = catchPanic(func() { scheduler.RunAfterStop(nil) })
				}
			},
			Run: func(Thread) {},
		}}, BatchAsync)
		<-threads[0].done
		if recovered != ErrReentrantWait {
			t.Errorf("cleanup drain panic = %v, want %v", recovered, ErrReentrantWait)
		}
		if threads[0].Stopped() || scheduler.admissionClosed() {
			t.Error("rejected cleanup drain canceled the thread or closed admission")
		}
		scheduler.RunAfterStop(nil)
		scheduler.callbacks.Range(func(_, _ any) bool {
			t.Error("cleanup left a callback scope registered")
			return false
		})
	})
}

func TestShutdownCallbackCanStopAndRejectCreation(t *testing.T) {
	checkDrainReentry(t, func() {
		scheduler := New(nil)
		var threads []Thread
		setup, ran := false, false
		scheduler.RunAfterStop(func() {
			scheduler.StopAll()
			threads = append(threads, scheduler.Create("rejected", func(Thread) {
				ran = true
			}))
			threads = append(threads, scheduler.StartBatch([]Task{{
				RegisterInvocation: func(Thread) func() { setup = true; return nil },
				Run:                func(Thread) { ran = true },
			}}, BatchAsync)...)
		})
		for _, thread := range threads {
			<-thread.done
			if !thread.Stopped() {
				t.Error("shutdown callback admitted a thread")
			}
		}
		if setup || ran {
			t.Errorf("shutdown admitted setup=%v, ran=%v", setup, ran)
		}
		if scheduler.admissionClosed() {
			t.Error("shutdown callback did not restore admission and caller scope")
		}
		scheduler.RunAfterStop(nil)
	})
}

func TestRunSyncSkipsWhenAdmissionIsClosed(t *testing.T) {
	scheduler := New(nil)
	called := false
	scheduler.RunAfterStop(func() {
		if !scheduler.admissionClosed() {
			t.Fatal("shutdown callback ran with admission open")
		}
		scheduler.RunSync("rejected", func(Thread) { called = true })
	})
	if called {
		t.Fatal("RunSync ran a callback after admission closed")
	}
}

func TestShutdownCallbackRejectsNestedDrain(t *testing.T) {
	checkDrainReentry(t, func() {
		scheduler := New(nil)
		var recovered any
		scheduler.RunAfterStop(func() {
			recovered = catchPanic(func() { scheduler.RunAfterStop(nil) })
			if !scheduler.admissionClosed() {
				t.Error("nested drain reopened the outer shutdown barrier")
			}
		})
		if recovered != ErrReentrantWait {
			t.Errorf("nested drain panic = %v, want %v", recovered, ErrReentrantWait)
		}
		if scheduler.admissionClosed() {
			t.Error("shutdown did not restore admission and caller scope after the callback returned")
		}
		scheduler.RunAfterStop(nil)
	})
}

func TestShutdownCallbackUnwindRestoresAdmission(t *testing.T) {
	for _, test := range []struct {
		name string
		exit func()
		want any
	}{
		{"panic", func() { panic("shutdown callback failure") }, "shutdown callback failure"},
		{"Goexit", runtime.Goexit, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			checkDrainReentry(t, func() {
				scheduler := New(nil)
				// This runs on the same goroutine after the callback's defers,
				// including when Goexit prevents the shutdown call from returning.
				defer func() {
					if recovered := recover(); recovered != test.want {
						t.Errorf("shutdown panic = %v, want %v", recovered, test.want)
					}
					if scheduler.admissionClosed() {
						t.Error("unwinding shutdown did not restore admission and caller scope")
					}
					scheduler.RunAfterStop(nil)
					thread := scheduler.Create("after-unwind", func(Thread) {})
					<-thread.done
					if thread.Stopped() {
						t.Error("unwinding shutdown rejected a fresh thread")
					}
				}()
				scheduler.RunAfterStop(test.exit)
				t.Error("abnormal shutdown callback returned normally")
			})
		})
	}
}

func TestExclusiveCallbackRejectsBlockedWaits(t *testing.T) {
	for _, test := range []struct {
		name string
		wait func(*Scheduler, Thread, *Latch)
	}{
		{"join", func(scheduler *Scheduler, th Thread, _ *Latch) { scheduler.Join(th) }},
		{"first yield", func(scheduler *Scheduler, th Thread, _ *Latch) { scheduler.JoinFirstSlice(th) }},
		{"latch", func(_ *Scheduler, _ Thread, latch *Latch) { latch.Wait() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			scheduler := New(nil)
			latch := scheduler.NewLatch()
			finished := make(chan struct{})
			guarded := make(chan any, 1)
			go func() {
				defer close(finished)
				var thread Thread
				scheduler.RunBetweenScripts(func() {
					thread = scheduler.Create("child", func(Thread) {})
					func() {
						defer func() { guarded <- recover() }()
						test.wait(scheduler, thread, latch)
					}()
				})
				scheduler.Join(thread)
				latch.Open()
				// Completed waits are safe even when the callback owns runMu.
				scheduler.RunBetweenScripts(func() { test.wait(scheduler, thread, latch) })
			}()
			select {
			case recovered := <-guarded:
				if recovered != ErrReentrantWait {
					t.Fatalf("blocked wait panic = %v", recovered)
				}
			case <-time.After(time.Second):
				t.Fatal("callback waited for a script while holding its execution lock")
			}
			waitForThreadSignal(t, finished, "callback did not release its execution lock")
		})
	}
}

func TestRunBetweenScriptsRejectsReentry(t *testing.T) {
	for _, managed := range []bool{true, false} {
		scheduler := New(nil)
		called := false
		var recovered any
		attempt := func() {
			recovered = catchPanic(func() { scheduler.RunBetweenScripts(func() { called = true }) })
		}
		if managed {
			caller := scheduler.Create("caller", func(Thread) { attempt() })
			waitForThreadSignal(t, caller.done, "rejected engine entry blocked its caller")
		} else {
			scheduler.RunBetweenScripts(attempt)
		}
		if recovered != ErrReentrantWait || called {
			t.Fatalf("managed=%v: reentry panic=%v, callback ran=%v", managed, recovered, called)
		}
		scheduler.RunBetweenScripts(func() { called = true })
		if !called {
			t.Fatal("rejected reentry prevented a subsequent engine call")
		}
	}
}

func catchPanic(call func()) (recovered any) {
	defer func() { recovered = recover() }()
	call()
	return
}

// Bound the whole check: a lock reentry must not hang the test goroutine.
func checkDrainReentry(t *testing.T, check func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("unexpected drain panic: %v", recovered)
			}
		}()
		check()
	}()
	waitForThreadSignal(t, done, "synchronous drain did not return")
}
