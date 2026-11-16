package coroutine

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStopThisScriptPanicsWithSentinel(t *testing.T) {
	scheduler := New(nil)
	defer func() {
		if recovered := recover(); recovered != ErrStopThisScript {
			t.Fatalf("StopThisScript panic = %v, want %v", recovered, ErrStopThisScript)
		}
	}()
	scheduler.StopThisScript()
}

func TestDrainWakesMultipleWaiters(t *testing.T) {
	scheduler := New(nil)
	workerStarted, release := make(chan struct{}), make(chan struct{})
	releaseWorker := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		releaseWorker()
		scheduler.RunAfterStop(nil)
	})

	workerOwner := scheduler.Create("worker-owner", func(Thread) {
		scheduler.WaitToDo(func() {
			close(workerStarted)
			<-release
		})
	})
	waitForThreadSignal(t, workerStarted, "native worker did not start")
	latch := scheduler.NewLatch()
	peer := scheduler.Create("peer", func(Thread) { latch.Wait() })
	waitForThreadSignal(t, peer.firstSlice.done, "peer did not suspend")

	const count = 6
	started, completed := make(chan struct{}, count), make(chan struct{}, count)
	for range count {
		go func() {
			started <- struct{}{}
			scheduler.drain()
			completed <- struct{}{}
		}()
	}
	for range count {
		waitForThreadSignal(t, started, "drain waiter did not start")
	}

	scheduler.Stop(workerOwner)
	waitForThreadSignal(t, workerOwner.finished.done, "native worker's caller did not stop")
	releaseWorker()
	select {
	case <-completed:
		t.Fatal("drain returned while its peer was still registered")
	default:
	}

	scheduler.Stop(peer)
	for range count {
		waitForThreadSignal(t, completed, "drain did not wake after its remaining work completed")
	}
	if scheduler.hasPendingWork() {
		t.Fatal("drain left work registered")
	}
}

func TestShutdownRejectsManagedCallerWithoutSideEffects(t *testing.T) {
	scheduler := New(nil)
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	latch := scheduler.NewLatch()
	peer := scheduler.Create("peer", func(Thread) { latch.Wait() })
	waitForThreadSignal(t, peer.firstSlice.done, "peer did not suspend")
	epoch := scheduler.admissionEpoch.Load()
	var recovered any
	caller := scheduler.Create("caller", func(me Thread) {
		recovered = catchPanic(func() {
			scheduler.RunAfterStop(func() { t.Error("rejected shutdown ran its callback") })
		})
		if scheduler.Current() != me || scheduler.Caller() != me {
			t.Error("rejected shutdown changed the managed caller")
		}
	})
	waitForThreadSignal(t, caller.finished.done, "managed shutdown caller did not finish")
	if recovered != ErrReentrantWait {
		t.Fatalf("managed shutdown panic = %v, want %v", recovered, ErrReentrantWait)
	}
	if caller.Stopped() || peer.Stopped() || scheduler.admissionClosed() || scheduler.admissionEpoch.Load() != epoch {
		t.Fatal("rejected shutdown canceled a script or changed admission")
	}
}

func TestStopIfStopsActiveThread(t *testing.T) {
	scheduler := New(nil)
	started := make(chan Thread, 1)
	done := make(chan struct{})

	scheduler.Create("worker", func(me Thread) {
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

	scheduler.StopIf(func(candidate Thread) bool {
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
	scheduler := New(nil)
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	blocked := scheduler.NewLatch()
	thread := scheduler.Create("worker", func(Thread) {
		blocked.Wait()
	})
	waitForThreadSignal(t, thread.firstSlice.done, "coroutine did not suspend")

	scheduler.Stop(thread)
	scheduler.Stop(thread)
	scheduler.Stop(nil)
	waitForThreadSignal(t, thread.finished.done, "suspended coroutine did not stop")
	scheduler.Stop(thread)
}

func TestStopIfEvaluatesFilterWithoutHoldingMutex(t *testing.T) {
	scheduler := New(nil)
	started := make(chan Thread, 1)
	done := make(chan struct{})

	scheduler.Create("worker", func(me Thread) {
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
		scheduler.StopIf(func(candidate Thread) bool {
			reacquired = canLockRegistry(scheduler)
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
	scheduler := New(nil)
	childResult := make(chan Thread, 1)
	var childRan atomic.Bool

	scheduler.Create("parent", func(Thread) {
		scheduler.StopAll()
		childResult <- scheduler.Create("late-child", func(Thread) {
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
	scheduler.drain()
	if childRan.Load() {
		t.Fatal("child created by a canceled owner ran user code")
	}
}

func TestStopAllInvalidatesPendingAdmission(t *testing.T) {
	scheduler := New(nil)
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	admission := scheduler.captureAdmission()
	scheduler.StopAll()

	joined := make(chan struct{})
	fresh := scheduler.Create("fresh", func(me Thread) {
		thread := scheduler.createThread(admission, Task{
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
			"context": thread.Context().Done(), "first slice": thread.firstSlice.done, "done": thread.finished.done,
		} {
			select {
			case <-done:
			default:
				t.Errorf("rejected thread %s remained open while the caller held runMu", name)
			}
		}
		if !thread.Stopped() || scheduler.Current() != me || scheduler.Caller() != me {
			t.Error("rejected creation was not stopped or changed the current script")
		}
		if threads := scheduler.snapshotThreads(); len(threads) != 1 || threads[0] != me {
			t.Error("rejected thread entered the lifecycle registry")
		}
		// Managed joins consult waiter sets, not just the completed channels.
		scheduler.JoinFirstSlice(thread)
		scheduler.Join(thread)
		close(joined)
	})
	waitForThreadSignal(t, joined, "managed joins blocked on a rejected thread")
	waitForThreadSignal(t, fresh.finished.done, "fresh script did not finish")
	if fresh.Stopped() {
		t.Fatal("StopAll did not reopen admission for new requests")
	}
}

func TestRunAfterStopOrdersInFlightRegistration(t *testing.T) {
	scheduler := New(nil)
	scheduler.admissionMu.Lock()
	registrationStarted := make(chan struct{})
	created := make(chan Thread, 1)
	go func() {
		close(registrationStarted)
		created <- scheduler.Create("in-flight", func(Thread) {})
	}()
	select {
	case <-registrationStarted:
	case <-time.After(time.Second):
		scheduler.admissionMu.Unlock()
		t.Fatal("registration did not start")
	}

	stopDone := make(chan struct{})
	go func() {
		scheduler.RunAfterStop(nil)
		close(stopDone)
	}()
	select {
	case <-stopDone:
		scheduler.admissionMu.Unlock()
		t.Fatal("shutdown barrier returned while an earlier registration was in flight")
	case <-time.After(20 * time.Millisecond):
	}
	scheduler.admissionMu.Unlock()

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
	case <-thread.finished.done:
	case <-time.After(time.Second):
		t.Fatal("in-flight thread remained after the shutdown barrier")
	}
}

func TestRunAfterStopRejectsCreationDuringBarrier(t *testing.T) {
	scheduler := New(nil)
	blocker := scheduler.newThread("barrier-blocker")
	scheduler.registerThread(blocker)

	stopDone := make(chan struct{})
	go func() {
		scheduler.RunAfterStop(nil)
		close(stopDone)
	}()
	deadline := time.Now().Add(time.Second)
	for !blocker.Stopped() {
		if time.Now().After(deadline) {
			scheduler.blockThread(blocker)
			scheduler.unregisterThread(blocker)
			t.Fatal("shutdown barrier did not start")
		}
		runtime.Gosched()
	}

	var childRan atomic.Bool
	child := scheduler.Create("during-barrier", func(Thread) {
		childRan.Store(true)
	})
	if !child.Stopped() {
		t.Fatal("thread created during shutdown barrier was admitted")
	}
	scheduler.blockThread(blocker)
	scheduler.unregisterThread(blocker)

	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown barrier did not finish after the blocker was removed")
	}
	select {
	case <-child.finished.done:
	case <-time.After(time.Second):
		t.Fatal("rejected thread did not finish")
	}
	if childRan.Load() {
		t.Fatal("thread created during shutdown barrier ran user code")
	}
}

func TestRunAfterStopRejectsSynchronousPanicHandlerReentry(t *testing.T) {
	guarded := make(chan any, 1)
	var scheduler *Scheduler
	scheduler = New(func(PanicReport) {
		func() {
			defer func() { guarded <- recover() }()
			scheduler.RunAfterStop(nil)
		}()
	})

	worker := scheduler.Create("panicking-worker", func(Thread) {
		panic("worker failure")
	})
	waitForThreadSignal(t, worker.finished.done, "panicking worker did not finish")
	select {
	case recovered := <-guarded:
		want := ErrReentrantWait
		if recovered != want {
			t.Fatalf("panic handler reentry panic = %v, want %q", recovered, want)
		}
	case <-time.After(time.Second):
		t.Fatal("panic handler did not attempt reset barrier reentry")
	}
	scheduler.RunAfterStop(nil)
}

func TestRunAfterStopWaitsForPanicHandlerBeforeReopeningAdmission(t *testing.T) {
	handlerStarted, releaseHandler := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseHandler) })
	scheduler := New(func(PanicReport) {
		close(handlerStarted)
		<-releaseHandler
	})
	workerStarted := make(chan struct{})
	worker := scheduler.Create("panicking-worker", func(me Thread) {
		close(workerStarted)
		<-me.Context().Done()
		panic("worker failure")
	})
	t.Cleanup(func() {
		release()
		scheduler.RunAfterStop(nil)
	})
	waitForThreadSignal(t, workerStarted, "panicking worker did not start")
	callback, drained := make(chan struct{}), make(chan struct{})
	go func() {
		scheduler.RunAfterStop(func() { close(callback) })
		close(drained)
	}()
	waitForThreadSignal(t, handlerStarted, "shutdown did not reach the panic handler")
	// Script completion alone is insufficient: its panic handler still owns work.
	waitForThreadSignal(t, worker.finished.done, "panicking script did not finish")
	assertShutdownRejectsCreation(t, scheduler)
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
	fresh := scheduler.Create("after-panic-handler", func(Thread) {})
	waitForThreadSignal(t, fresh.finished.done, "fresh script did not finish")
	if fresh.Stopped() || scheduler.admissionClosed() {
		t.Fatal("shutdown did not reopen admission after the panic handler")
	}
}

func (p *Scheduler) hasPendingWork() bool {
	p.threadsMu.Lock()
	defer p.threadsMu.Unlock()
	return p.hasPendingWorkLocked()
}

func assertShutdownRejectsCreation(t *testing.T, scheduler *Scheduler) {
	t.Helper()
	ran := false
	thread := scheduler.Create("during-shutdown", func(Thread) { ran = true })
	waitForThreadSignal(t, thread.finished.done, "rejected coroutine did not finish")
	if !scheduler.admissionClosed() || !thread.Stopped() || ran {
		t.Fatal("shutdown admitted new work before drain completed")
	}
}

func canLockRegistry(scheduler *Scheduler) bool {
	if !scheduler.threadsMu.TryLock() {
		return false
	}
	scheduler.threadsMu.Unlock()
	return true
}
