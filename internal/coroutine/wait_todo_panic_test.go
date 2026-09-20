package coroutine

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitToDoGoexitStopsCaller(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() {
		if !co.StopAllAndWait(time.Second) {
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
	if !co.waitForDrain(time.Second, nil) {
		t.Fatal("exited worker remained registered")
	}
	co.callbacks.Range(func(any, any) bool {
		t.Error("worker Goexit retained callback scope")
		return false
	})
}

func TestRunTaskTracksNilPanic(t *testing.T) {
	t.Setenv("GODEBUG", "panicnil=1")
	result := runTask(func() { panic(nil) })
	if !result.panicked {
		t.Fatal("runTask treated a nil panic as successful completion")
	}
	if result.panicValue != nil {
		t.Fatalf("panic value = %v, want nil", result.panicValue)
	}
}

func TestWaitToDoRejectsSynchronousDrain(t *testing.T) {
	co := New(nil)
	guarded := make(chan any, 1)
	thread := co.Create("caller", func(Thread) {
		co.WaitToDo(func() {
			defer func() { guarded <- recover() }()
			co.StopAllAndWait(time.Millisecond)
		})
	})
	waitForThreadSignal(t, thread.done, "caller did not finish")
	if recovered := <-guarded; recovered != ErrReentrantWait {
		t.Fatalf("synchronous drain panic = %v", recovered)
	}
	if thread.Stopped() {
		t.Fatal("rejected drain canceled its own caller")
	}
	if !co.RunAfterStopAll(time.Second, nil) {
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
	if !co.StopAllAndWait(time.Second) {
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

func TestRunAfterStopAllWaitsForWaitToDoWorker(t *testing.T) {
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
	completed := make(chan bool, 1)
	go func() {
		completed <- co.RunAfterStopAll(20*time.Millisecond, func() {
			callbackRan <- struct{}{}
		})
	}()
	select {
	case got := <-completed:
		if got {
			t.Fatal("shutdown reported success while WaitToDo worker was blocked")
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
	if !co.RunAfterStopAll(time.Second, nil) {
		t.Fatal("explicit recovery barrier did not complete after native drain")
	}
	next := co.Create("after-recovery", func(Thread) {})
	select {
	case <-next.done:
	case <-time.After(time.Second):
		t.Fatal("post-drain coroutine did not finish")
	}
}
