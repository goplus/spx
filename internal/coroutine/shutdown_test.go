package coroutine

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDrainWakesMultipleWaiters(t *testing.T) {
	co := New(nil)
	workerStarted, release := make(chan struct{}), make(chan struct{})
	releaseWorker := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		releaseWorker()
		co.RunAfterStop(nil)
	})

	workerOwner := co.Create("worker-owner", func(Thread) {
		co.WaitToDo(func() {
			close(workerStarted)
			<-release
		})
	})
	waitForThreadSignal(t, workerStarted, "native worker did not start")
	latch := co.NewLatch()
	peer := co.Create("peer", func(Thread) { latch.Wait() })
	waitForThreadSignal(t, peer.firstSliceDone, "peer did not suspend")

	const count = 6
	started, completed := make(chan struct{}, count), make(chan struct{}, count)
	for range count {
		go func() {
			started <- struct{}{}
			co.drain()
			completed <- struct{}{}
		}()
	}
	for range count {
		waitForThreadSignal(t, started, "drain waiter did not start")
	}

	co.Stop(workerOwner)
	waitForThreadSignal(t, workerOwner.done, "native worker's caller did not stop")
	releaseWorker()
	select {
	case <-completed:
		t.Fatal("drain returned while its peer was still registered")
	default:
	}

	co.Stop(peer)
	for range count {
		waitForThreadSignal(t, completed, "drain did not wake after its remaining work completed")
	}
	if co.hasPendingWork() {
		t.Fatal("drain left work registered")
	}
}

func TestShutdownRejectsManagedCallerWithoutSideEffects(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
	latch := co.NewLatch()
	peer := co.Create("peer", func(Thread) { latch.Wait() })
	waitForThreadSignal(t, peer.firstSliceDone, "peer did not suspend")
	epoch := co.admissionEpoch.Load()
	var recovered any
	caller := co.Create("caller", func(me Thread) {
		recovered = catchPanic(func() {
			co.RunAfterStop(func() { t.Error("rejected shutdown ran its callback") })
		})
		if co.Current() != me || co.callerThread() != me {
			t.Error("rejected shutdown changed the managed caller")
		}
	})
	waitForThreadSignal(t, caller.done, "managed shutdown caller did not finish")
	if recovered != ErrReentrantWait {
		t.Fatalf("managed shutdown panic = %v, want %v", recovered, ErrReentrantWait)
	}
	if caller.Stopped() || peer.Stopped() || co.admissionClosed() || co.admissionEpoch.Load() != epoch {
		t.Fatal("rejected shutdown canceled a script or changed admission")
	}
}

func TestStopIfStopsActiveThread(t *testing.T) {
	co := New(nil)
	started := make(chan Thread, 1)
	done := make(chan struct{})

	co.Create("worker", func(me Thread) {
		started <- me
		for {
			select {
			case <-me.Context().Done():
				close(done)
				return
			default:
				runtime.Gosched()
			}
		}
	})

	var th Thread
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case th = <-started:
	case <-timer.C:
		t.Fatal("coroutine did not start")
	}

	th.suspendMu.Lock()
	suspendState := th.suspendState
	th.suspendMu.Unlock()
	if suspendState == suspendWaiting {
		t.Fatal("test precondition failed: spinning thread should not be suspended")
	}

	co.StopIf(func(candidate Thread) bool {
		return candidate == th
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatal("active coroutine did not stop after StopIf")
	}
}

func TestStopIsIdempotentForSuspendedThread(t *testing.T) {
	co := New(nil)
	thread := co.Create("worker", func(me Thread) {
		co.yield(me)
	})
	waitForThreadSignal(t, thread.firstSliceDone, "coroutine did not suspend")

	co.Stop(thread)
	co.Stop(thread)
	co.Stop(nil)
	waitForThreadSignal(t, thread.done, "suspended coroutine did not stop")
	co.Stop(thread)
}

func TestStopIfEvaluatesFilterWithoutHoldingMutex(t *testing.T) {
	co := New(nil)
	started := make(chan Thread, 1)
	done := make(chan struct{})

	co.Create("worker", func(me Thread) {
		started <- me
		for {
			select {
			case <-me.Context().Done():
				close(done)
				return
			default:
				runtime.Gosched()
			}
		}
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	var th Thread
	select {
	case th = <-started:
	case <-timer.C:
		t.Fatal("coroutine did not start")
	}

	var reacquired bool
	stopDone := make(chan struct{})
	go func() {
		co.StopIf(func(candidate Thread) bool {
			reacquired = canLockRegistry(co)
			return candidate == th
		})
		close(stopDone)
	}()

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-stopDone:
	case <-timer.C:
		t.Fatal("StopIf deadlocked when filter re-entered the coroutine mutex")
	}
	if !reacquired {
		t.Fatal("StopIf evaluated filter while holding the coroutine mutex")
	}

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatal("active coroutine did not stop after StopIf")
	}
}

func TestStopAllRejectsChildCreatedByCanceledOwner(t *testing.T) {
	co := New(nil)
	childResult := make(chan Thread, 1)
	var childRan atomic.Bool

	co.Create("parent", func(Thread) {
		co.StopAll()
		childResult <- co.Create("late-child", func(Thread) {
			childRan.Store(true)
		})
	})

	var child Thread
	select {
	case child = <-childResult:
	case <-time.After(time.Second):
		t.Fatal("canceled owner did not finish child registration")
	}
	if !child.Stopped() {
		t.Fatal("child created by a canceled owner was admitted")
	}
	co.drain()
	if childRan.Load() {
		t.Fatal("child created by a canceled owner ran user code")
	}
}

func TestStopAllInvalidatesPendingAdmission(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(nil) })
	admission := co.captureAdmission()
	co.StopAll()

	joined := make(chan struct{})
	fresh := co.Create("fresh", func(me Thread) {
		thread := co.createThread(admission, Task{
			Owner: "stale",
			RegisterInvocation: func(Thread) func() {
				t.Error("stale creation ran its registration callback")
				return nil
			},
			Run: func(Thread) { t.Error("stale creation ran its script") },
		})
		// This script still owns runMu: rejected work must finish without
		// running another coroutine or disturbing the current execution owner.
		for name, done := range map[string]<-chan struct{}{
			"context": thread.Context().Done(), "first slice": thread.firstSliceDone, "done": thread.done,
		} {
			select {
			case <-done:
			default:
				t.Errorf("rejected thread %s remained open while the caller held runMu", name)
			}
		}
		if !thread.Stopped() || co.Current() != me || co.callerThread() != me {
			t.Error("rejected creation was not stopped or changed the current script")
		}
		if threads := co.snapshotThreads(); len(threads) != 1 || threads[0] != me {
			t.Error("rejected thread entered the lifecycle registry")
		}
		// Managed joins consult waiter sets, not just the completed channels.
		co.JoinFirstSlice(thread)
		co.Join(thread)
		close(joined)
	})
	waitForThreadSignal(t, joined, "managed joins blocked on a rejected thread")
	waitForThreadSignal(t, fresh.done, "fresh script did not finish")
	if fresh.Stopped() {
		t.Fatal("StopAll did not reopen admission for new requests")
	}
}

func TestRunAfterStopOrdersInFlightRegistration(t *testing.T) {
	co := New(nil)
	co.admissionMu.Lock()
	registrationStarted := make(chan struct{})
	created := make(chan Thread, 1)
	go func() {
		close(registrationStarted)
		created <- co.Create("in-flight", func(Thread) {})
	}()
	select {
	case <-registrationStarted:
	case <-time.After(time.Second):
		co.admissionMu.Unlock()
		t.Fatal("registration did not start")
	}

	stopDone := make(chan struct{})
	go func() {
		co.RunAfterStop(nil)
		close(stopDone)
	}()
	select {
	case <-stopDone:
		co.admissionMu.Unlock()
		t.Fatal("shutdown barrier returned while an earlier registration was in flight")
	case <-time.After(20 * time.Millisecond):
	}
	co.admissionMu.Unlock()

	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown barrier did not finish")
	}
	var thread Thread
	select {
	case thread = <-created:
	case <-time.After(time.Second):
		t.Fatal("in-flight registration did not return")
	}
	select {
	case <-thread.done:
	case <-time.After(time.Second):
		t.Fatal("in-flight thread remained after the shutdown barrier")
	}
}

func TestRunAfterStopRejectsCreationDuringBarrier(t *testing.T) {
	co := New(nil)
	blocker := co.newThread("barrier-blocker")
	co.registerThread(blocker)

	stopDone := make(chan struct{})
	go func() {
		co.RunAfterStop(nil)
		close(stopDone)
	}()
	deadline := time.Now().Add(time.Second)
	for !blocker.Stopped() {
		if time.Now().After(deadline) {
			co.removeRunnable(blocker)
			co.unregisterThread(blocker)
			t.Fatal("shutdown barrier did not start")
		}
		runtime.Gosched()
	}

	var childRan atomic.Bool
	child := co.Create("during-barrier", func(Thread) {
		childRan.Store(true)
	})
	if !child.Stopped() {
		t.Fatal("thread created during shutdown barrier was admitted")
	}
	co.removeRunnable(blocker)
	co.unregisterThread(blocker)

	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown barrier did not finish after the blocker was removed")
	}
	select {
	case <-child.done:
	case <-time.After(time.Second):
		t.Fatal("rejected thread did not finish")
	}
	if childRan.Load() {
		t.Fatal("thread created during shutdown barrier ran user code")
	}
}

func TestRunAfterStopRejectsSynchronousPanicHandlerReentry(t *testing.T) {
	guarded := make(chan any, 1)
	var co *Coroutines
	co = New(func(PanicReport) {
		func() {
			defer func() { guarded <- recover() }()
			co.RunAfterStop(nil)
		}()
	})

	worker := co.Create("panicking-worker", func(Thread) {
		panic("worker failure")
	})
	waitForThreadSignal(t, worker.done, "panicking worker did not finish")
	select {
	case recovered := <-guarded:
		want := ErrReentrantWait
		if recovered != want {
			t.Fatalf("panic handler reentry panic = %v, want %q", recovered, want)
		}
	case <-time.After(time.Second):
		t.Fatal("panic handler did not attempt reset barrier reentry")
	}
	co.RunAfterStop(nil)
}

func TestRunAfterStopWaitsForPanicHandlerBeforeReopeningAdmission(t *testing.T) {
	handlerStarted, releaseHandler := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseHandler) })
	co := New(func(PanicReport) {
		close(handlerStarted)
		<-releaseHandler
	})
	workerStarted := make(chan struct{})
	worker := co.Create("panicking-worker", func(me Thread) {
		close(workerStarted)
		<-me.Context().Done()
		panic("worker failure")
	})
	t.Cleanup(func() {
		release()
		co.RunAfterStop(nil)
	})
	waitForThreadSignal(t, workerStarted, "panicking worker did not start")
	callback, drained := make(chan struct{}), make(chan struct{})
	go func() {
		co.RunAfterStop(func() { close(callback) })
		close(drained)
	}()
	waitForThreadSignal(t, handlerStarted, "shutdown did not reach the panic handler")
	// Script completion alone is insufficient: its panic handler still owns work.
	waitForThreadSignal(t, worker.done, "panicking script did not finish")
	assertShutdownRejectsCreation(t, co)
	select {
	case <-callback:
		t.Fatal("shutdown callback ran before the panic handler finished")
	case <-drained:
		t.Fatal("shutdown returned before the panic handler finished")
	case <-time.After(20 * time.Millisecond):
	}
	release()
	waitForThreadSignal(t, drained, "shutdown did not finish after the panic handler")
	waitForThreadSignal(t, callback, "shutdown did not run its callback")
	fresh := co.Create("after-panic-handler", func(Thread) {})
	waitForThreadSignal(t, fresh.done, "fresh script did not finish")
	if fresh.Stopped() || co.admissionClosed() {
		t.Fatal("shutdown did not reopen admission after the panic handler")
	}
}

func assertShutdownRejectsCreation(t *testing.T, co *Coroutines) {
	t.Helper()
	ran := false
	thread := co.Create("during-shutdown", func(Thread) { ran = true })
	waitForThreadSignal(t, thread.done, "rejected coroutine did not finish")
	if !co.admissionClosed() || !thread.Stopped() || ran {
		t.Fatal("shutdown admitted new work before drain completed")
	}
}

func (p *Coroutines) hasPendingWork() bool {
	p.threadsMu.Lock()
	defer p.threadsMu.Unlock()
	return p.hasPendingWorkLocked()
}

func canLockRegistry(co *Coroutines) bool {
	if !co.threadsMu.TryLock() {
		return false
	}
	co.threadsMu.Unlock()
	return true
}
