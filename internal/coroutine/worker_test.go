package coroutine

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitToDoRejectsCanceledCaller(t *testing.T) {
	co := New(nil)
	called, continued := false, false
	caller := co.Create("caller", func(me Thread) {
		me.Cancel()
		co.WaitToDo(func() { called = true })
		continued = true
	})
	if err := co.drain(drainTestContext(t)); err != nil {
		t.Fatalf("canceled caller did not drain: %v", err)
	}
	if !caller.Stopped() || called || continued {
		t.Fatalf("stopped=%v, worker ran=%v, caller continued=%v", caller.Stopped(), called, continued)
	}
}

func TestWaitToDoGoexitStopsCaller(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() {
		if co.RunAfterStop(drainTestContext(t), nil) != nil {
			t.Error("worker and caller did not drain")
		}
	})
	var continued atomic.Bool
	cleaned := make(chan struct{})
	thread := co.Create("caller", func(Thread) {
		defer close(cleaned)
		co.WaitToDo(runtime.Goexit)
		continued.Store(true)
	})
	waitForThreadSignal(t, thread.done, "worker Goexit left its caller suspended")
	waitForThreadSignal(t, cleaned, "caller cleanup did not run")
	if !thread.Stopped() || continued.Load() {
		t.Fatal("caller continued after its worker exited without a result")
	}
	if co.drain(drainTestContext(t)) != nil {
		t.Fatal("exited worker remained registered")
	}
	co.callbacks.Range(func(any, any) bool {
		t.Error("worker Goexit retained callback scope")
		return false
	})
}

func TestWaitToDoRejectsSynchronousDrain(t *testing.T) {
	co := New(nil)
	guarded := make(chan any, 1)
	thread := co.Create("caller", func(Thread) {
		co.WaitToDo(func() {
			defer func() { guarded <- recover() }()
			co.RunAfterStop(context.Background(), nil)
		})
	})
	waitForThreadSignal(t, thread.done, "caller did not finish")
	if recovered := <-guarded; recovered != ErrReentrantWait {
		t.Fatalf("synchronous drain panic = %v", recovered)
	}
	if thread.Stopped() {
		t.Fatal("rejected drain canceled its own caller")
	}
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
		t.Fatal("external drain did not recover after the worker returned")
	}
}

func TestWaitToDoPropagatesWorkerPanic(t *testing.T) {
	panicReported := make(chan any, 1)
	co := New(func(report PanicReport) { panicReported <- report.Value })

	co.Create("caller", func(me Thread) {
		co.WaitToDo(func() { panic("worker failure") })
	})

	select {
	case recovered := <-panicReported:
		if recovered != "worker failure" {
			t.Fatalf("panic report = %v, want worker failure", recovered)
		}
	case <-time.After(time.Second):
		t.Fatal("worker panic did not complete the waiting coroutine")
	}
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
		t.Fatal("coroutine did not stop during cleanup")
	}
}

func TestWaitToDoPropagatesNilWorkerPanic(t *testing.T) {
	t.Setenv("GODEBUG", "panicnil=1")
	co := New(func(PanicReport) {})
	continued := make(chan struct{}, 1)
	thread := co.Create("caller", func(Thread) {
		co.WaitToDo(func() { panic(nil) })
		continued <- struct{}{}
	})

	select {
	case <-thread.done:
	case <-time.After(time.Second):
		t.Fatal("nil worker panic did not complete the waiting coroutine")
	}
	select {
	case <-continued:
		t.Fatal("coroutine continued after a nil worker panic")
	default:
	}
}

func TestRunAfterStopWaitsForWaitToDoWorker(t *testing.T) {
	co := New(nil)
	workerStarted := make(chan struct{})
	workerDone := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseWorker := func() { releaseOnce.Do(func() { close(release) }) }
	caller := co.Create("caller", func(Thread) {
		co.WaitToDo(func() {
			close(workerStarted)
			<-release
			close(workerDone)
		})
	})
	defer func() {
		releaseWorker()
		select {
		case <-workerDone:
		case <-time.After(time.Second):
			t.Error("WaitToDo worker did not finish during cleanup")
		}
		select {
		case <-caller.done:
		case <-time.After(time.Second):
			t.Error("WaitToDo caller did not finish during cleanup")
		}
	}()

	select {
	case <-workerStarted:
	case <-time.After(time.Second):
		t.Fatal("WaitToDo worker did not start")
	}

	callbackRan := make(chan struct{}, 1)
	completed := make(chan error, 1)
	go func() {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		completed <- co.RunAfterStop(timeoutCtx, func() {
			callbackRan <- struct{}{}
		})
	}()
	select {
	case err := <-completed:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("worker drain error = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not time out while WaitToDo worker was blocked")
	}
	select {
	case <-callbackRan:
		t.Fatal("shutdown callback ran before WaitToDo worker drained")
	default:
	}

	var rejectedRan atomic.Bool
	rejected := co.Create("during-timeout", func(Thread) {
		rejectedRan.Store(true)
	})
	if !rejected.Stopped() {
		t.Fatal("creation was admitted while native work was still running")
	}
	select {
	case <-rejected.done:
	case <-time.After(time.Second):
		t.Fatal("rejected creation did not finish")
	}
	if rejectedRan.Load() {
		t.Fatal("creation during native drain ran user code")
	}

	releaseWorker()
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("WaitToDo worker did not finish after release")
	}
	select {
	case <-caller.done:
	case <-time.After(time.Second):
		t.Fatal("canceled WaitToDo caller did not finish")
	}

	stillRejected := co.Create("after-drain-before-recovery", func(Thread) {
		t.Fatal("creation ran while the manager remained quarantined")
	})
	if !stillRejected.Stopped() {
		t.Fatal("admission reopened after a fatal barrier timed out")
	}
	select {
	case <-stillRejected.done:
	case <-time.After(time.Second):
		t.Fatal("quarantined creation did not finish")
	}
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
		t.Fatal("explicit recovery barrier did not complete after native drain")
	}
	next := co.Create("after-recovery", func(Thread) {})
	select {
	case <-next.done:
	case <-time.After(time.Second):
		t.Fatal("post-drain coroutine did not finish")
	}
}

func TestDrainWaitsForLastWorkerAfterCallerCancellation(t *testing.T) {
	for _, test := range []struct {
		name string
		exit func()
	}{
		{name: "return", exit: func() {}},
		{name: "goexit", exit: runtime.Goexit},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := New(nil)
			started := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
			unblock := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
			finished := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
			release := [2]func(){
				sync.OnceFunc(func() { close(unblock[0]) }),
				sync.OnceFunc(func() { close(unblock[1]) }),
			}
			t.Cleanup(func() {
				for _, releaseWorker := range release {
					releaseWorker()
				}
				if co.RunAfterStop(drainTestContext(t), nil) != nil {
					t.Error("workers did not drain during cleanup")
				}
			})

			var callers [2]Thread
			for i := range callers {
				callers[i] = co.Create("caller", func(Thread) {
					co.WaitToDo(func() {
						defer close(finished[i])
						close(started[i])
						<-unblock[i]
						if i == 1 {
							test.exit()
						}
					})
				})
			}
			for _, signal := range started {
				waitForThreadSignal(t, signal, "native worker did not start")
			}
			for _, caller := range callers {
				co.Stop(caller)
			}
			for _, caller := range callers {
				waitForThreadSignal(t, caller.done, "canceled caller did not finish")
			}

			reset := make(chan struct{})
			drained := make(chan bool, 1)
			go func() {
				workerCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				drained <- co.RunAfterStop(workerCtx, func() { close(reset) }) == nil
			}()
			release[0]()
			waitForThreadSignal(t, finished[0], "first native worker did not finish")
			select {
			case <-reset:
				t.Fatal("reset ran while the last native worker was still running")
			case completed := <-drained:
				t.Fatalf("drain returned %v before the last native worker finished", completed)
			case <-time.After(25 * time.Millisecond):
			}

			release[1]()
			waitForThreadSignal(t, finished[1], "last native worker did not finish")
			select {
			case completed := <-drained:
				if !completed {
					t.Fatal("runtime did not drain after the last native worker finished")
				}
			case <-time.After(time.Second):
				t.Fatal("drain did not wake after the last native worker finished")
			}
			waitForThreadSignal(t, reset, "drain completed without running reset")
		})
	}
}

func TestWaitForChanCommitsResultOnlyAfterResume(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() {
		if co.RunAfterStop(drainTestContext(t), nil) != nil {
			t.Error("receive did not drain")
		}
	})
	input := make(chan int)
	value := 0
	thread := co.Create("receiver", func(Thread) {
		value = WaitForChan(co, input)
	})
	waitForThreadSignal(t, thread.firstSliceDone, "receiver did not wait")

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
	co.JoinFirstSlice(thread)
	co.threadsMu.Lock()
	pending := co.workerCount
	co.threadsMu.Unlock()
	if pending != 1 {
		t.Errorf("tracked receivers = %d, want 1", pending)
	}
	if co.RunAfterStop(drainTestContext(t), func() {
		co.threadsMu.Lock()
		defer co.threadsMu.Unlock()
		if co.workerCount != 0 {
			t.Error("reset ran before the receiver drained")
		}
	}) != nil {
		t.Fatal("pending receive did not stop")
	}
	if value != 7 {
		t.Fatalf("canceled receive changed its result: %d", value)
	}
}
