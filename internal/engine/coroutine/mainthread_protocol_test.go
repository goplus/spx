package coroutine

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type directMainThreadPlatform struct{ gdx.IPlatformMgr }

func (directMainThreadPlatform) IsMainThread() bool { return true }

// Native uses the direct path too, so these regressions also run under -race.
func useDirectMainThread(t *testing.T) {
	t.Helper()
	previous := gdx.PlatformMgr
	gdx.PlatformMgr = directMainThreadPlatform{}
	t.Cleanup(func() { gdx.PlatformMgr = previous })
}

func TestWaitMainThreadDirectPreservesFaultStack(t *testing.T) {
	useDirectMainThread(t)
	errorValue := errors.New("callback failure")
	for _, cleanup := range []bool{false, true} {
		name := "normal"
		if cleanup {
			name = "cleanup"
		}
		t.Run(name, func(t *testing.T) {
			for _, value := range []any{"sentinel", errorValue, []string{"uncomparable", "value"}} {
				t.Run(reflect.TypeOf(value).String(), func(t *testing.T) {
					reports := make(chan PanicReport, 1)
					scheduler := New(func(report PanicReport) { reports <- report })
					worker := scheduler.Create("callback-worker", func(Thread) {
						call := scheduler.WaitMainThread
						if cleanup {
							call = scheduler.CleanupMainThread
						}
						call(func() { panicWithValue(value) })
					})
					waitForThreadSignal(t, worker.done, "panicking callback did not finish")
					scheduler.RunAfterStop(nil)
					select {
					case report := <-reports:
						if !reflect.DeepEqual(report.Value, value) || value == errorValue && report.Value != errorValue {
							t.Errorf("reported panic = %#v, want original %#v", report.Value, value)
						}
						if report.Name != "callback-worker" {
							t.Errorf("reported name = %q, want callback-worker", report.Name)
						}
						if !strings.Contains(report.Stack, "panicWithValue") {
							t.Errorf("report lost the callback fault site:\n%s", report.Stack)
						}
					default:
						t.Fatal("callback panic was not reported")
					}
					scheduler.callbacks.Range(func(_, _ any) bool {
						t.Error("panicking callback retained its scope")
						return false
					})
				})
			}
		})
	}
}

func TestWaitMainThreadDirectPreservesCanceledAbnormalExit(t *testing.T) {
	useDirectMainThread(t)
	t.Setenv("GODEBUG", "panicnil=1")
	for _, test := range []struct {
		name string
		call func()
		want any
	}{
		{"nil panic", func() { panicWithValue(nil) }, ErrAbortThread},
		{"Goexit", runtime.Goexit, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			scheduler := New(nil)
			checked := false
			worker := scheduler.Create("canceled-callback", func(me Thread) {
				defer func() {
					if got := recover(); got != test.want {
						t.Errorf("exit panic = %v, want %v", got, test.want)
					}
					if scheduler.callbackScope() != 0 {
						t.Error("abnormal callback retained its scope")
					}
					checked = true
				}()
				scheduler.WaitMainThread(func() {
					me.Cancel()
					test.call()
				})
				t.Error("canceled abnormal callback returned normally")
			})
			waitForThreadSignal(t, worker.done, "canceled callback did not finish")
			scheduler.RunAfterStop(nil)
			if !checked {
				t.Fatal("canceled callback never reached its caller's defer")
			}
		})
	}
}

func TestWaitMainThreadDirectCancellation(t *testing.T) {
	useDirectMainThread(t)
	for _, when := range []string{"before", "during", "during panic"} {
		t.Run(when, func(t *testing.T) {
			var called, continued, cleaned, reported atomic.Bool
			scheduler := New(func(PanicReport) { reported.Store(true) })
			thread := scheduler.Create("caller", func(me Thread) {
				defer scheduler.CleanupMainThread(func() { cleaned.Store(true) })
				if when == "before" {
					me.Cancel()
				}
				scheduler.WaitMainThread(func() {
					called.Store(true)
					me.Cancel()
					if when == "during panic" {
						panic("discarded result")
					}
				})
				continued.Store(true)
			})
			waitForThreadSignal(t, thread.done, "canceled caller did not finish")
			scheduler.RunAfterStop(nil)
			if called.Load() != (when != "before") || continued.Load() || !cleaned.Load() || reported.Load() {
				t.Fatalf("called=%v continued=%v cleaned=%v reported=%v", called.Load(), continued.Load(), cleaned.Load(), reported.Load())
			}
		})
	}
}

func TestWaitMainThreadDirectRejectsDrain(t *testing.T) {
	useDirectMainThread(t)
	scheduler := New(nil)
	got := catchPanic(func() {
		scheduler.WaitMainThread(func() { scheduler.RunAfterStop(nil) })
	})
	if got != ErrReentrantWait {
		t.Fatalf("callback drain panic = %v, want %v", got, ErrReentrantWait)
	}
	if scheduler.admissionClosed() || scheduler.callbackScope() != 0 {
		t.Fatal("rejected drain left admission closed or callback scope installed")
	}
}

func TestWaitMainThreadDirectPreservesAbnormalExit(t *testing.T) {
	useDirectMainThread(t)
	t.Setenv("GODEBUG", "panicnil=1")
	for _, test := range []struct {
		name string
		call func()
		want any
	}{
		{"panic", func() { panic("callback failure") }, "callback failure"},
		{"nil panic", func() { panicWithValue(nil) }, nil},
		{"Goexit", runtime.Goexit, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			scheduler := New(nil)
			exited := make(chan struct{})
			go func() {
				defer close(exited)
				defer func() {
					if got := recover(); got != test.want {
						t.Errorf("exit panic = %v, want %v", got, test.want)
					}
					if scheduler.callbackScope() != 0 {
						t.Error("abnormal exit retained callback scope")
					}
				}()
				scheduler.WaitMainThread(test.call)
				t.Error("abnormal callback returned normally")
			}()
			waitForThreadSignal(t, exited, "abnormal callback did not release the caller")
		})
	}
}

func TestManagedMainThreadCallbackRejectsYieldBeforeSideEffects(t *testing.T) {
	useDirectMainThread(t)
	for _, cleanup := range []bool{false, true} {
		name := "normal"
		if cleanup {
			name = "cleanup"
		}
		t.Run(name, func(t *testing.T) {
			for _, operation := range []string{"schedule", "time", "frame", "worker", "channel", "join", "first slice", "latch", "batch done", "batch first slice", "execute"} {
				t.Run(operation, func(t *testing.T) {
					scheduler := New(nil)
					latch := scheduler.NewLatch()
					var workRan atomic.Bool
					thread := scheduler.Create("caller", func(me Thread) {
						peer := scheduler.Create("peer", func(Thread) {})
						lastID := scheduler.LastThreadID()
						call := scheduler.WaitMainThread
						if cleanup {
							call = scheduler.CleanupMainThread
						}
						call(func() {
							got := catchPanic(func() {
								switch operation {
								case "schedule":
									scheduler.Sched(me)
								case "time":
									scheduler.Wait(3600)
								case "frame":
									scheduler.WaitNextFrame()
								case "worker":
									scheduler.WaitToDo(func() { workRan.Store(true) })
								case "channel":
									WaitForChan(scheduler, latch.Done())
								case "execute":
									scheduler.RunSync(nil, func(Thread) { workRan.Store(true) })
								case "join":
									scheduler.Join(peer)
								case "first slice":
									scheduler.JoinFirstSlice(peer)
								case "latch":
									latch.Wait()
								case "batch done", "batch first slice":
									mode := BatchWaitDone
									if operation == "batch first slice" {
										mode = BatchWaitFirstSlice
									}
									scheduler.StartBatch([]Task{{Run: func(Thread) { workRan.Store(true) }}}, mode)
								}
							})
							if got != ErrReentrantWait {
								t.Errorf("yield panic = %v, want %v", got, ErrReentrantWait)
							}
							select {
							case <-me.firstSliceDone:
								t.Error("rejected operation released the first script slice")
							default:
							}
							if scheduler.LastThreadID() != lastID || scheduler.currentJobCount() != 0 {
								t.Error("rejected operation registered a task or wait job")
							}
						})
					})
					// A regression that yields must still be able to unwind without a frame.
					waitForThreadSignal(t, thread.firstSliceDone, "caller neither rejected the wait nor yielded")
					scheduler.Stop(thread)
					waitForThreadSignal(t, thread.done, "caller did not finish")
					scheduler.RunAfterStop(nil)
					if workRan.Load() {
						t.Error("rejected wait launched its worker or batch")
					}
				})
			}
		})
	}
}

func TestCleanupMainThreadDirectAllowsNestedEngineCallsAfterCancellation(t *testing.T) {
	useDirectMainThread(t)
	scheduler := New(nil)
	cleaned := 0
	thread := scheduler.Create("caller", func(me Thread) {
		me.Cancel()
		scheduler.CleanupMainThread(func() {
			scheduler.WaitMainThread(func() { cleaned++ })
			scheduler.CleanupMainThread(func() {
				scheduler.WaitMainThread(func() { cleaned++ })
			})
		})
		got := catchPanic(func() {
			scheduler.WaitMainThread(func() { t.Error("cleanup cancellation exemption escaped its scope") })
		})
		if got != ErrAbortThread {
			t.Errorf("ordinary call after cleanup panic = %v, want %v", got, ErrAbortThread)
		}
	})
	waitForThreadSignal(t, thread.done, "nested engine cleanup did not finish")
	scheduler.RunAfterStop(nil)
	if cleaned != 2 {
		t.Fatalf("nested engine cleanup calls = %d, want 2", cleaned)
	}
}

func TestExclusiveCallbackChannelWait(t *testing.T) {
	scheduler := New(nil)
	ready := make(chan int, 1)
	ready <- 7
	closed := make(chan int)
	close(closed)
	scheduler.RunBetweenScripts(func() {
		if got := WaitForChan(scheduler, ready); got != 7 {
			t.Errorf("ready channel value = %d, want 7", got)
		}
		if got := WaitForChan(scheduler, closed); got != 0 {
			t.Errorf("closed channel value = %d, want 0", got)
		}
	})

	blocked := make(chan int, 1)
	result := make(chan any, 1)
	go func() {
		result <- catchPanic(func() {
			scheduler.RunBetweenScripts(func() { WaitForChan(scheduler, blocked) })
		})
	}()
	select {
	case got := <-result:
		if got != ErrReentrantWait {
			t.Fatalf("blocked channel panic = %v, want %v", got, ErrReentrantWait)
		}
	case <-time.After(time.Second):
		// Release a broken implementation before reporting the failure.
		blocked <- 9
		<-result
		t.Fatal("exclusive callback blocked on a channel without a ready value")
	}
	blocked <- 9
	if got := <-blocked; got != 9 {
		t.Fatalf("rejected channel wait consumed value %d", got)
	}
}

func TestManagedMainThreadCallbackAllowsCompletedWaits(t *testing.T) {
	useDirectMainThread(t)
	scheduler := New(nil)
	finished := scheduler.Create("finished", func(Thread) {})
	scheduler.Join(finished)
	latch := scheduler.NewLatch()
	latch.Open()
	thread := scheduler.Create("caller", func(me Thread) {
		for _, call := range []func(func()){scheduler.WaitMainThread, scheduler.CleanupMainThread} {
			call(func() {
				scheduler.Join(nil)
				scheduler.Join(me)
				scheduler.Join(finished)
				scheduler.JoinFirstSlice(finished)
				latch.Wait()
				scheduler.StartBatch(nil, BatchWaitDone)
				scheduler.StartBatch(nil, BatchWaitFirstSlice)
				scheduler.WaitMainThread(func() {})
			})
		}
	})
	waitForThreadSignal(t, thread.done, "completed waits blocked the callback")
	scheduler.RunAfterStop(nil)
}
