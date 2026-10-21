package coroutine

import (
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
	co.drain()
	if !caller.Stopped() || called || continued {
		t.Fatalf("stopped=%v, worker ran=%v, caller continued=%v", caller.Stopped(), called, continued)
	}
}

func TestWaitToDoGoexitStopsCaller(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() {
		co.RunAfterStop(nil)
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
	co.drain()
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
			co.RunAfterStop(nil)
		})
	})
	waitForThreadSignal(t, thread.done, "caller did not finish")
	if recovered := <-guarded; recovered != ErrReentrantWait {
		t.Fatalf("synchronous drain panic = %v", recovered)
	}
	if thread.Stopped() {
		t.Fatal("rejected drain canceled its own caller")
	}
	co.RunAfterStop(nil)
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
	co.RunAfterStop(nil)
}

func TestWaitToDoPropagatesNilWorkerPanic(t *testing.T) {
	t.Setenv("GODEBUG", "panicnil=1")
	co := New(func(PanicReport) {})
	continued := make(chan struct{}, 1)
	thread := co.Create("caller", func(Thread) {
		co.WaitToDo(func() { panicWithValue(nil) })
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
	started, releaseWorker := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseWorker) })
	caller := co.Create("worker-owner", func(Thread) {
		co.WaitToDo(func() {
			close(started)
			<-releaseWorker
		})
	})
	t.Cleanup(func() {
		release()
		co.RunAfterStop(nil)
	})
	waitForThreadSignal(t, started, "worker did not start")
	callback, drained := make(chan struct{}), make(chan struct{})
	go func() {
		co.RunAfterStop(func() { close(callback) })
		close(drained)
	}()
	waitForThreadSignal(t, caller.done, "shutdown did not stop the worker's caller")
	assertShutdownRejectsCreation(t, co)
	select {
	case <-callback:
		t.Fatal("shutdown callback ran before the worker exited")
	case <-drained:
		t.Fatal("shutdown returned before the worker exited")
	case <-time.After(20 * time.Millisecond):
	}
	release()
	waitForThreadSignal(t, drained, "shutdown did not finish after the worker exited")
	waitForThreadSignal(t, callback, "shutdown did not run its callback")
	fresh := co.Create("after-drain", func(Thread) {})
	waitForThreadSignal(t, fresh.done, "fresh script did not finish")
	if fresh.Stopped() || co.admissionClosed() {
		t.Fatal("shutdown did not reopen admission after the worker exited")
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
				co.RunAfterStop(nil)
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
			drained := make(chan struct{})
			go func() {
				co.RunAfterStop(func() { close(reset) })
				close(drained)
			}()
			release[0]()
			waitForThreadSignal(t, finished[0], "first native worker did not finish")
			select {
			case <-reset:
				t.Fatal("reset ran while the last native worker was still running")
			case <-drained:
				t.Fatal("drain returned before the last native worker finished")
			case <-time.After(25 * time.Millisecond):
			}

			release[1]()
			waitForThreadSignal(t, finished[1], "last native worker did not finish")
			select {
			case <-drained:
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
		co.RunAfterStop(nil)
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
			_, pending := co.pendingExecution[thread]
			co.schedulerMu.Unlock()
			if pending {
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
	co.RunAfterStop(func() {
		co.threadsMu.Lock()
		defer co.threadsMu.Unlock()
		if co.workerCount != 0 {
			t.Error("reset ran before the receiver drained")
		}
	})
	if value != 7 {
		t.Fatalf("canceled receive changed its result: %d", value)
	}
}
