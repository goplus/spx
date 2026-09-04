package coroutine

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

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

func TestWaitToDoPropagatesWorkerPanic(t *testing.T) {
	panicReported := make(chan any, 1)
	co := New(func(name, stack string) { panicReported <- "worker failure" })
	co.OnInited()
	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.WaitToDo(func() { panic("worker failure") })
		return 0
	})

	select {
	case recovered := <-panicReported:
		if recovered != "worker failure" {
			t.Fatalf("panic report = %v, want worker failure", recovered)
		}
	case <-time.After(time.Second):
		t.Fatal("worker panic did not complete the waiting coroutine")
	}
	if !co.AbortAllAndWait(time.Second) {
		t.Fatal("coroutine did not stop during cleanup")
	}
}

func TestWaitToDoPropagatesNilWorkerPanic(t *testing.T) {
	t.Setenv("GODEBUG", "panicnil=1")
	co := New(func(string, string) {})
	co.OnInited()
	continued := make(chan struct{}, 1)
	thread := co.CreateAndStart(true, "caller", func(Thread) int {
		co.WaitToDo(func() { panic(nil) })
		continued <- struct{}{}
		return 0
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

func TestRunAfterAbortAllWaitsForWaitToDoWorker(t *testing.T) {
	co := New(nil)
	co.OnInited()
	workerStarted := make(chan struct{})
	workerDone := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseWorker := func() { releaseOnce.Do(func() { close(release) }) }
	caller := co.CreateAndStart(true, "caller", func(Thread) int {
		co.WaitToDo(func() {
			close(workerStarted)
			<-release
			close(workerDone)
		})
		return 0
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
		completed <- co.RunAfterAbortAll(20*time.Millisecond, func() {
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
	rejected := co.CreateAndStart(true, "during-timeout", func(Thread) int {
		rejectedRan.Store(true)
		return 0
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

	stillRejected := co.Create("after-drain-before-recovery", func(Thread) int {
		t.Fatal("creation ran while the manager remained quarantined")
		return 0
	})
	if !stillRejected.Stopped() {
		t.Fatal("admission reopened after a fatal barrier timed out")
	}
	select {
	case <-stillRejected.done:
	case <-time.After(time.Second):
		t.Fatal("quarantined creation did not finish")
	}
	if !co.RunAfterAbortAll(time.Second, nil) {
		t.Fatal("explicit recovery barrier did not complete after native drain")
	}
	next := co.Create("after-recovery", func(Thread) int { return 0 })
	select {
	case <-next.done:
	case <-time.After(time.Second):
		t.Fatal("post-drain coroutine did not finish")
	}
}

func TestRunawayShutdownDoesNotBlockOnNativeWorker(t *testing.T) {
	co := New(nil)
	co.OnInited()
	workerStarted := make(chan struct{})
	release := make(chan struct{})
	caller := co.CreateAndStart(true, "caller", func(Thread) int {
		co.WaitToDo(func() {
			close(workerStarted)
			<-release
		})
		return 0
	})

	cleanup := func() {
		select {
		case <-release:
		default:
			close(release)
		}
		if !co.AbortAllAndWait(time.Second) {
			t.Error("coroutines did not stop during cleanup")
		}
	}
	defer cleanup()

	select {
	case <-workerStarted:
	case <-time.After(time.Second):
		t.Fatal("native worker did not start")
	}

	shutdownDone := make(chan struct{})
	go func() {
		co.stopRunawayThreads()
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
	case <-time.After(2 * runawayShutdownTimeout):
		t.Fatal("runaway shutdown blocked on an uncooperative native worker")
	}

	var rejectedRan atomic.Bool
	rejected := co.Create("during-native-drain", func(Thread) int {
		rejectedRan.Store(true)
		return 0
	})
	if !rejected.Stopped() {
		t.Fatal("runaway shutdown reopened admission before native worker drained")
	}
	if rejectedRan.Load() {
		t.Fatal("coroutine created during native drain ran user code")
	}
	select {
	case <-rejected.done:
	case <-time.After(time.Second):
		t.Fatal("rejected coroutine did not finish")
	}

	close(release)
	select {
	case <-caller.done:
	case <-time.After(time.Second):
		t.Fatal("canceled WaitToDo caller did not finish after native worker release")
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		next := co.Create("after-native-drain", func(Thread) int { return 0 })
		if !next.Stopped() {
			select {
			case <-next.done:
			case <-time.After(time.Second):
				t.Fatal("post-drain coroutine did not finish")
			}
			return
		}
		select {
		case <-next.done:
		case <-time.After(time.Second):
			t.Fatal("rejected post-drain coroutine did not finish")
		}
		runtime.Gosched()
	}
	t.Fatal("admission did not reopen after native worker drained")
}
