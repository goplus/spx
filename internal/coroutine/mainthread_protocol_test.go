package coroutine

import (
	"runtime"
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

func TestWaitMainThreadDirectCancellation(t *testing.T) {
	useDirectMainThread(t)
	for _, when := range []string{"before", "during", "during panic"} {
		t.Run(when, func(t *testing.T) {
			var called, continued, cleaned, reported atomic.Bool
			co := New(func(PanicReport) { reported.Store(true) })
			thread := co.Create("caller", func(me Thread) {
				defer co.CleanupMainThread(func() { cleaned.Store(true) })
				if when == "before" {
					me.Cancel()
				}
				co.WaitMainThread(func() {
					called.Store(true)
					me.Cancel()
					if when == "during panic" {
						panic("discarded result")
					}
				})
				continued.Store(true)
			})
			waitForThreadSignal(t, thread.done, "canceled caller did not finish")
			co.RunAfterStop(nil)
			if called.Load() != (when != "before") || continued.Load() || !cleaned.Load() || reported.Load() {
				t.Fatalf("called=%v continued=%v cleaned=%v reported=%v", called.Load(), continued.Load(), cleaned.Load(), reported.Load())
			}
		})
	}
}

func TestWaitMainThreadDirectRejectsDrain(t *testing.T) {
	useDirectMainThread(t)
	co := New(nil)
	got := catchPanic(func() {
		co.WaitMainThread(func() { co.RunAfterStop(nil) })
	})
	if got != ErrReentrantWait {
		t.Fatalf("callback drain panic = %v, want %v", got, ErrReentrantWait)
	}
	if co.admissionClosed() || co.callbackScope() != 0 {
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
			co := New(nil)
			exited := make(chan struct{})
			go func() {
				defer close(exited)
				defer func() {
					if got := recover(); got != test.want {
						t.Errorf("exit panic = %v, want %v", got, test.want)
					}
					if co.callbackScope() != 0 {
						t.Error("abnormal exit retained callback scope")
					}
				}()
				co.WaitMainThread(test.call)
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
					co := New(nil)
					latch := co.NewLatch()
					var workRan atomic.Bool
					thread := co.Create("caller", func(me Thread) {
						peer := co.Create("peer", func(Thread) {})
						lastID := co.LastThreadID()
						call := co.WaitMainThread
						if cleanup {
							call = co.CleanupMainThread
						}
						call(func() {
							got := catchPanic(func() {
								switch operation {
								case "schedule":
									co.Sched(me)
								case "time":
									co.Wait(3600)
								case "frame":
									co.WaitNextFrame()
								case "worker":
									co.WaitToDo(func() { workRan.Store(true) })
								case "channel":
									WaitForChan(co, latch.Done())
								case "execute":
									co.RunSync(nil, func(Thread) { workRan.Store(true) })
								case "join":
									co.Join(peer)
								case "first slice":
									co.JoinFirstSlice(peer)
								case "latch":
									latch.Wait()
								case "batch done", "batch first slice":
									mode := BatchWaitDone
									if operation == "batch first slice" {
										mode = BatchWaitFirstSlice
									}
									co.StartBatch([]Task{{Run: func(Thread) { workRan.Store(true) }}}, mode)
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
							if co.LastThreadID() != lastID || co.currentJobCount() != 0 {
								t.Error("rejected operation registered a task or wait job")
							}
						})
					})
					// A regression that yields must still be able to unwind without a frame.
					waitForThreadSignal(t, thread.firstSliceDone, "caller neither rejected the wait nor yielded")
					co.Stop(thread)
					waitForThreadSignal(t, thread.done, "caller did not finish")
					co.RunAfterStop(nil)
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
	co := New(nil)
	cleaned := 0
	thread := co.Create("caller", func(me Thread) {
		me.Cancel()
		co.CleanupMainThread(func() {
			co.WaitMainThread(func() { cleaned++ })
			co.CleanupMainThread(func() {
				co.WaitMainThread(func() { cleaned++ })
			})
		})
		got := catchPanic(func() {
			co.WaitMainThread(func() { t.Error("cleanup cancellation exemption escaped its scope") })
		})
		if got != ErrAbortThread {
			t.Errorf("ordinary call after cleanup panic = %v, want %v", got, ErrAbortThread)
		}
	})
	waitForThreadSignal(t, thread.done, "nested engine cleanup did not finish")
	co.RunAfterStop(nil)
	if cleaned != 2 {
		t.Fatalf("nested engine cleanup calls = %d, want 2", cleaned)
	}
}

func TestExclusiveCallbackChannelWait(t *testing.T) {
	co := New(nil)
	ready := make(chan int, 1)
	ready <- 7
	closed := make(chan int)
	close(closed)
	co.RunBetweenScripts(func() {
		if got := WaitForChan(co, ready); got != 7 {
			t.Errorf("ready channel value = %d, want 7", got)
		}
		if got := WaitForChan(co, closed); got != 0 {
			t.Errorf("closed channel value = %d, want 0", got)
		}
	})

	blocked := make(chan int, 1)
	result := make(chan any, 1)
	go func() {
		result <- catchPanic(func() {
			co.RunBetweenScripts(func() { WaitForChan(co, blocked) })
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
	co := New(nil)
	finished := co.Create("finished", func(Thread) {})
	co.Join(finished)
	latch := co.NewLatch()
	latch.Open()
	thread := co.Create("caller", func(me Thread) {
		for _, call := range []func(func()){co.WaitMainThread, co.CleanupMainThread} {
			call(func() {
				co.Join(nil)
				co.Join(me)
				co.Join(finished)
				co.JoinFirstSlice(finished)
				latch.Wait()
				co.StartBatch(nil, BatchWaitDone)
				co.StartBatch(nil, BatchWaitFirstSlice)
				co.WaitMainThread(func() {})
			})
		}
	})
	waitForThreadSignal(t, thread.done, "completed waits blocked the callback")
	co.RunAfterStop(nil)
}
