package coroutine

import (
	"runtime"
	"testing"
)

func TestSetupRejectsSynchronousDrain(t *testing.T) {
	for _, managed := range []bool{false, true} {
		name := "external"
		if managed {
			name = "managed"
		}
		t.Run(name, func(t *testing.T) {
			checkDrainReentry(t, func() {
				co := New(nil)
				var recovered any
				var threads []Thread
				ran := false
				start := func() {
					threads = co.StartBatch([]Task{{
						Setup: func(Thread) func() {
							recovered = catchDrainPanic(func() { co.StopAllAndWait(0) })
							return nil
						},
						Run: func(Thread) { ran = true },
					}}, BatchAsync)
				}
				var parent Thread
				if managed {
					parent = co.Create("parent", func(Thread) int { start(); return 0 })
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
				if co.admissionClosed() {
					t.Error("rejected setup drain closed admission")
				}
			})
		})
	}
}

func TestCleanupRejectsSynchronousDrain(t *testing.T) {
	checkDrainReentry(t, func() {
		co := New(nil)
		var recovered any
		threads := co.StartBatch([]Task{{
			Setup: func(Thread) func() {
				return func() {
					recovered = catchDrainPanic(func() { co.StopAllAndWait(0) })
				}
			},
			Run: func(Thread) {},
		}}, BatchAsync)
		<-threads[0].done
		if recovered != ErrReentrantWait {
			t.Errorf("cleanup drain panic = %v, want %v", recovered, ErrReentrantWait)
		}
		if threads[0].Stopped() || co.admissionClosed() {
			t.Error("rejected cleanup drain canceled the thread or closed admission")
		}
		if !co.RunAfterStopAll(0, nil) {
			t.Error("thread did not drain after cleanup")
		}
		co.callbacks.Range(func(_, _ any) bool {
			t.Error("cleanup left a callback scope registered")
			return false
		})
	})
}

func TestShutdownCallbackCanStopAndRejectCreation(t *testing.T) {
	checkDrainReentry(t, func() {
		co := New(nil)
		var threads []Thread
		setup, ran := false, false
		completed := co.RunAfterStopAll(0, func() {
			co.StopAll()
			threads = append(threads, co.Create("rejected", func(Thread) int {
				ran = true
				return 0
			}))
			threads = append(threads, co.StartBatch([]Task{{
				Setup: func(Thread) func() { setup = true; return nil },
				Run:   func(Thread) { ran = true },
			}}, BatchAsync)...)
		})
		for _, thread := range threads {
			<-thread.done
			if !thread.Stopped() {
				t.Error("shutdown callback admitted a thread")
			}
		}
		if !completed || setup || ran {
			t.Errorf("shutdown completed=%v, setup=%v, ran=%v", completed, setup, ran)
		}
		if co.admissionClosed() || !co.RunAfterStopAll(0, nil) {
			t.Error("shutdown callback did not restore admission and caller scope")
		}
	})
}

func TestShutdownCallbackRejectsNestedDrain(t *testing.T) {
	for _, test := range []struct {
		name string
		call func(*Coroutines)
	}{
		{"StopAllAndWait", func(co *Coroutines) { co.StopAllAndWait(0) }},
		{"RunAfterStopAll", func(co *Coroutines) { co.RunAfterStopAll(0, nil) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			checkDrainReentry(t, func() {
				co := New(nil)
				var recovered any
				if !co.RunAfterStopAll(0, func() {
					recovered = catchDrainPanic(func() { test.call(co) })
					if !co.admissionClosed() {
						t.Error("nested drain reopened the outer shutdown barrier")
					}
				}) {
					t.Error("outer shutdown did not finish")
				}
				if recovered != ErrReentrantWait {
					t.Errorf("nested drain panic = %v, want %v", recovered, ErrReentrantWait)
				}
				if !co.RunAfterStopAll(0, nil) {
					t.Error("shutdown scope remained after the callback returned")
				}
			})
		})
	}
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
				co := New(nil)
				// This runs on the same goroutine after the callback's defers,
				// including when Goexit prevents the shutdown call from returning.
				defer func() {
					if recovered := recover(); recovered != test.want {
						t.Errorf("shutdown panic = %v, want %v", recovered, test.want)
					}
					if co.admissionClosed() || !co.RunAfterStopAll(0, nil) {
						t.Error("unwinding shutdown did not restore admission and caller scope")
					}
					thread := co.Create("after-unwind", func(Thread) int { return 0 })
					<-thread.done
					if thread.Stopped() {
						t.Error("unwinding shutdown rejected a fresh thread")
					}
				}()
				co.RunAfterStopAll(0, test.exit)
				t.Error("abnormal shutdown callback returned normally")
			})
		})
	}
}

func catchDrainPanic(call func()) (recovered any) {
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
