package coroutine

import (
	"context"
	"errors"
	"runtime"
	"strings"
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
		if co.RunAfterStop(drainTestContext(t), nil) != nil {
			t.Error("coroutines did not stop during cleanup")
		}
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
	started, completed := make(chan struct{}, count), make(chan bool, count)
	for range count {
		go func() {
			started <- struct{}{}
			completed <- co.drain(context.Background()) == nil
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
		waitForDrainResult(t, completed)
	}
	if co.hasPendingWork() {
		t.Fatal("drain left work registered")
	}
}

func TestShutdownPreCanceledHasNoEffects(t *testing.T) {
	for _, test := range []struct {
		name string
		stop func(*testing.T, *Coroutines, context.Context) error
	}{
		{"RunAfterStop", func(t *testing.T, co *Coroutines, ctx context.Context) error {
			return co.RunAfterStop(ctx, func() { t.Error("pre-canceled shutdown ran callback") })
		}},
		{"RunAfterStopIf", func(t *testing.T, co *Coroutines, ctx context.Context) error {
			stopStarted, err := co.RunAfterStopIf(ctx, func() bool {
				t.Error("pre-canceled shutdown checked condition")
				return true
			}, func() { t.Error("pre-canceled shutdown ran callback") })
			if stopStarted {
				t.Error("pre-canceled shutdown started stopping")
			}
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := New(nil)
			t.Cleanup(func() { co.RunAfterStop(drainTestContext(t), nil) })
			latch := co.NewLatch()
			peer := co.Create("peer", func(Thread) { latch.Wait() })
			waitForThreadSignal(t, peer.firstSliceDone, "peer did not suspend")
			epoch := co.admissionEpoch.Load()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			if err := test.stop(t, co, ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("shutdown error = %v, want context.Canceled", err)
			}
			if peer.Stopped() || co.admissionEpoch.Load() != epoch {
				t.Fatal("pre-canceled shutdown canceled a peer or changed admission")
			}
		})
	}
}

func TestShutdownCanceledConditionHasNoEffects(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(drainTestContext(t), nil) })
	latch := co.NewLatch()
	peer := co.Create("peer", func(Thread) { latch.Wait() })
	waitForThreadSignal(t, peer.firstSliceDone, "peer did not suspend")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	epoch := co.admissionEpoch.Load()
	stopStarted, err := co.RunAfterStopIf(ctx, func() bool {
		cancel()
		return true
	}, func() { t.Error("canceled condition ran shutdown callback") })
	if stopStarted || !errors.Is(err, context.Canceled) {
		t.Fatalf("condition-canceled shutdown = (%v, %v), want (false, context.Canceled)", stopStarted, err)
	}
	if peer.Stopped() || co.admissionEpoch.Load() != epoch {
		t.Fatal("condition-canceled shutdown canceled a peer or changed admission")
	}
}

func TestShutdownCanceledDrainRequiresRecovery(t *testing.T) {
	co := New(nil)
	started, workCanceled, release, exited := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	releaseWorker := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		releaseWorker()
		if co.RunAfterStop(drainTestContext(t), nil) != nil {
			t.Error("worker did not drain during cleanup")
		}
	})
	owner := co.Create("worker-owner", func(me Thread) {
		co.WaitToDo(func() {
			defer close(exited)
			close(started)
			<-me.Context().Done()
			close(workCanceled)
			<-release
		})
	})
	waitForThreadSignal(t, started, "worker did not start")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	afterCalled := false
	result := make(chan error, 1)
	go func() {
		stopStarted, err := co.RunAfterStopIf(ctx, nil, func() { afterCalled = true })
		if !stopStarted {
			t.Error("shutdown did not start stopping before drain cancellation")
		}
		result <- err
	}()
	waitForThreadSignal(t, workCanceled, "shutdown did not cancel the worker context")
	waitForThreadSignal(t, owner.done, "canceled worker owner did not finish")
	select {
	case err := <-result:
		t.Fatalf("drain finished before the worker exited: %v", err)
	default:
	}
	cancel()
	if err := waitForShutdown(t, result); !errors.Is(err, context.Canceled) || afterCalled {
		t.Fatalf("canceled drain error = %v, after=%v", err, afterCalled)
	}
	assertShutdownRejectsCreation(t, co)

	releaseWorker()
	waitForThreadSignal(t, exited, "worker did not exit")
	if err := co.drain(drainTestContext(t)); err != nil {
		t.Fatalf("worker exit did not finish drain: %v", err)
	}
	assertShutdownRejectsCreation(t, co)
	if err := co.RunAfterStop(context.Background(), func() {
		if !co.admissionClosed() {
			t.Error("recovery callback ran with admission open")
		}
		afterCalled = true
	}); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if !afterCalled || co.admissionClosed() {
		t.Fatal("successful recovery did not run the callback and reopen admission")
	}
	fresh := co.Create("after-recovery", func(Thread) {})
	waitForThreadSignal(t, fresh.done, "fresh coroutine did not finish")
	if fresh.Stopped() {
		t.Fatal("successful recovery rejected fresh work")
	}
}

func TestShutdownCanceledAfterGateAcquired(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(drainTestContext(t), nil) })
	latch := co.NewLatch()
	peer := co.Create("peer", func(Thread) { latch.Wait() })
	co.JoinFirstSlice(peer)
	epoch := co.admissionEpoch.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := &observedErrContext{Context: ctx, onErr: func() {
		// Cancel once the gate is acquired, before shutdown starts.
		if len(co.shutdownGate) == 0 {
			cancel()
		}
	}}
	stopStarted, err := co.RunAfterStopIf(observed, func() bool {
		t.Error("canceled shutdown checked its condition")
		return true
	}, func() { t.Error("canceled shutdown ran its callback") })
	if stopStarted || !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown stopStarted=%v, error=%v, want false and context.Canceled", stopStarted, err)
	}
	if peer.Stopped() || co.admissionClosed() || co.admissionEpoch.Load() != epoch {
		t.Fatal("canceled shutdown stopped a peer or changed admission")
	}
	called := false
	if err := co.RunAfterStop(drainTestContext(t), func() { called = true }); err != nil || !called {
		t.Fatalf("canceled shutdown left the gate unusable: callback=%v, error=%v", called, err)
	}
}

func TestShutdownCancelsBarrierWait(t *testing.T) {
	co := New(nil)
	held, release := make(chan struct{}), make(chan struct{})
	releaseBarrier := sync.OnceFunc(func() { close(release) })
	defer releaseBarrier()
	first := make(chan error, 1)
	go func() {
		stopStarted, err := co.RunAfterStopIf(context.Background(), func() bool {
			close(held)
			<-release
			return false
		}, nil)
		if stopStarted {
			t.Error("false condition started the first shutdown")
		}
		first <- err
	}()
	waitForThreadSignal(t, held, "first barrier was not acquired")
	epoch := co.admissionEpoch.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waiting := make(chan struct{})
	observed := &observedDoneContext{Context: ctx, onDone: sync.OnceFunc(func() { close(waiting) })}
	conditionCalled, afterCalled := false, false
	second := make(chan error, 1)
	go func() {
		stopStarted, err := co.RunAfterStopIf(observed, func() bool {
			conditionCalled = true
			return true
		}, func() { afterCalled = true })
		if stopStarted {
			t.Error("canceled waiter started the second shutdown")
		}
		second <- err
	}()
	waitForThreadSignal(t, waiting, "second barrier did not wait for context cancellation")
	cancel()
	if err := waitForShutdown(t, second); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting barrier error = %v, want context.Canceled", err)
	}
	if conditionCalled || afterCalled || co.admissionEpoch.Load() != epoch {
		t.Fatal("canceled barrier waiter ran callbacks or changed admission")
	}
	releaseBarrier()
	if err := waitForShutdown(t, first); err != nil {
		t.Fatalf("first barrier failed: %v", err)
	}
	if err := co.RunAfterStop(drainTestContext(t), nil); err != nil {
		t.Fatalf("canceled waiter left the barrier unusable: %v", err)
	}
}

func TestShutdownDeadlineExceeded(t *testing.T) {
	co := New(nil)
	blocked := co.newThread("deadline-blocker")
	co.registerThread(blocked)
	defer func() {
		co.removeRunnable(blocked)
		co.unregisterThread(blocked)
	}()
	if !co.admitWorker(nil) {
		t.Fatal("worker was rejected")
	}
	defer co.finishWorker()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	stopStarted, err := co.RunAfterStopIf(ctx, nil, func() { t.Error("callback ran before worker exit") })
	if !stopStarted || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline drain = (%v, %v), want (true, context.DeadlineExceeded)", stopStarted, err)
	}
	if !strings.Contains(err.Error(), blocked.Name()) || !strings.Contains(err.Error(), "1 workers") {
		t.Fatalf("deadline error does not describe pending work: %v", err)
	}
}

func TestDrainCompletionWinsCancellation(t *testing.T) {
	for range 100 {
		co := New(nil)
		if !co.admitWorker(nil) {
			t.Fatal("worker was rejected")
		}
		ctx, cancel := context.WithCancel(context.Background())
		// Make both lifecycle completion and cancellation ready before the wait
		// selects either event. The registry must resolve the race as success.
		observed := &observedDoneContext{Context: ctx, onDone: sync.OnceFunc(func() {
			co.finishWorker()
			cancel()
		})}
		if err := co.drain(observed); err != nil {
			t.Fatalf("completed registry lost to context cancellation: %v", err)
		}
		cancel()
	}
}

func TestShutdownRejectsManagedCallerWithoutSideEffects(t *testing.T) {
	for _, test := range []struct {
		name string
		stop func(*testing.T, *Coroutines, context.Context)
	}{
		{"RunAfterStop", func(t *testing.T, co *Coroutines, ctx context.Context) {
			co.RunAfterStop(ctx, func() { t.Error("rejected shutdown ran its callback") })
		}},
		{"RunAfterStopIf", func(t *testing.T, co *Coroutines, ctx context.Context) {
			co.RunAfterStopIf(ctx, func() bool {
				t.Error("rejected shutdown checked its condition")
				return true
			}, func() { t.Error("rejected shutdown ran its callback") })
		}},
	} {
		for _, canceled := range []bool{false, true} {
			name := test.name + "/background"
			if canceled {
				name = test.name + "/pre-canceled"
			}
			t.Run(name, func(t *testing.T) {
				co := New(nil)
				t.Cleanup(func() { co.RunAfterStop(drainTestContext(t), nil) })
				latch := co.NewLatch()
				peer := co.Create("peer", func(Thread) { latch.Wait() })
				waitForThreadSignal(t, peer.firstSliceDone, "peer did not suspend")
				epoch := co.admissionEpoch.Load()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if canceled {
					cancel()
				}
				var recovered any
				caller := co.Create("caller", func(me Thread) {
					recovered = catchPanic(func() { test.stop(t, co, ctx) })
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
			})
		}
	}
}

func TestShutdownConditionRejectsDrainReentry(t *testing.T) {
	checkDrainReentry(t, func() {
		co := New(nil)
		stopStarted, err := co.RunAfterStopIf(context.Background(), func() bool {
			got := catchPanic(func() { co.RunAfterStop(context.Background(), nil) })
			if got != ErrReentrantWait {
				t.Errorf("condition drain panic = %v, want %v", got, ErrReentrantWait)
			}
			return false
		}, nil)
		if stopStarted || err != nil || co.admissionClosed() {
			t.Errorf("condition reentry changed shutdown state: stopStarted=%v, err=%v", stopStarted, err)
		}
	})
}

func TestShutdownIgnoresCancellationDuringCallback(t *testing.T) {
	co := New(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := co.RunAfterStop(ctx, func() {
		cancel()
		if !co.admissionClosed() {
			t.Error("callback ran with admission open")
		}
	})
	if err != nil || co.admissionClosed() {
		t.Fatalf("completed callback failed to reopen admission: %v", err)
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
	if co.drain(drainTestContext(t)) != nil {
		t.Fatal("canceled owner and rejected child did not stop")
	}
	if childRan.Load() {
		t.Fatal("child created by a canceled owner ran user code")
	}
}

func TestStopAllInvalidatesPendingAdmission(t *testing.T) {
	co := New(nil)
	t.Cleanup(func() { co.RunAfterStop(drainTestContext(t), nil) })
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

	stopDone := make(chan bool, 1)
	go func() {
		stopDone <- co.RunAfterStop(drainTestContext(t), nil) == nil
	}()
	select {
	case <-stopDone:
		co.admissionMu.Unlock()
		t.Fatal("shutdown barrier returned while an earlier registration was in flight")
	case <-time.After(20 * time.Millisecond):
	}
	co.admissionMu.Unlock()

	select {
	case completed := <-stopDone:
		if !completed {
			t.Fatal("shutdown barrier timed out after registration was released")
		}
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

	stopDone := make(chan bool, 1)
	go func() {
		stopDone <- co.RunAfterStop(drainTestContext(t), nil) == nil
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
	case completed := <-stopDone:
		if !completed {
			t.Fatal("shutdown barrier timed out")
		}
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

func TestRunAfterStopTimeoutRequiresExplicitRecovery(t *testing.T) {
	co := New(nil)
	blocker := co.newThread("barrier-blocker")
	co.registerThread(blocker)

	var callbackRan atomic.Bool
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := co.RunAfterStop(timeoutCtx, func() {
		callbackRan.Store(true)
	}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked drain error = %v, want context.DeadlineExceeded", err)
	}
	if callbackRan.Load() {
		t.Fatal("shutdown barrier ran callback after timing out")
	}

	var rejectedRan atomic.Bool
	rejected := co.Create("during-timeout", func(Thread) {
		rejectedRan.Store(true)
	})
	if !rejected.Stopped() {
		t.Fatal("creation was admitted while a timed-out barrier still had work")
	}
	select {
	case <-rejected.done:
	case <-time.After(time.Second):
		t.Fatal("rejected creation did not finish")
	}
	if rejectedRan.Load() {
		t.Fatal("creation during a timed-out barrier ran user code")
	}

	co.removeRunnable(blocker)
	co.unregisterThread(blocker)

	stillRejected := co.Create("after-drain-before-recovery", func(Thread) {
		t.Fatal("creation ran while the manager remained quarantined")
	})
	if !stillRejected.Stopped() {
		t.Fatal("creation was admitted after a timed-out barrier drained without recovery")
	}
	waitForThreadSignal(t, stillRejected.done, "quarantined creation did not finish")

	// Explicit recovery reopens admission after the drain.
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
		t.Fatal("explicit recovery barrier did not complete after drain")
	}
	var nextRan atomic.Bool
	next := co.Create("after-timeout", func(Thread) {
		nextRan.Store(true)
	})
	select {
	case <-next.done:
	case <-time.After(time.Second):
		t.Fatal("creation remained blocked after shutdown barrier timed out")
	}
	if !nextRan.Load() {
		t.Fatal("creation was rejected after shutdown barrier timed out")
	}
}

func TestRunAfterStopIfFalseHasNoEffects(t *testing.T) {
	co := New(nil)
	started := make(chan struct{})
	release := make(chan struct{})
	releaseThread := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		releaseThread()
		co.RunAfterStop(drainTestContext(t), nil)
	})
	thread := co.Create("conditional-blocker", func(Thread) {
		close(started)
		<-release
	})
	waitForThreadSignal(t, started, "conditional blocker did not start")
	epoch := co.admissionEpoch.Load()
	called := false
	stopStarted, err := co.RunAfterStopIf(drainTestContext(t), func() bool {
		return false
	}, func() {
		called = true
	})
	if stopStarted || err != nil {
		t.Fatalf("conditional barrier = (%v, %v), want (false, nil)", stopStarted, err)
	}
	if called || thread.Stopped() || co.admissionClosed() || co.Current() != thread {
		t.Fatal("false condition ran its callback, stopped a script, changed admission, or released script execution")
	}
	if got := co.admissionEpoch.Load(); got != epoch {
		t.Fatalf("admission epoch = %d, want %d", got, epoch)
	}
	releaseThread()
	waitForThreadSignal(t, thread.done, "conditional blocker did not finish")
}

func TestRunAfterStopIfPredicatePanicReleasesBarrier(t *testing.T) {
	co := New(nil)
	func() {
		defer func() {
			if recovered := recover(); recovered != "predicate failure" {
				t.Fatalf("predicate panic = %v, want predicate failure", recovered)
			}
		}()
		co.RunAfterStopIf(drainTestContext(t), func() bool {
			panic("predicate failure")
		}, nil)
	}()
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
		t.Fatal("shutdown barrier remained locked after predicate panic")
	}
}

func TestRunAfterStopTimeoutIncludesBarrierWait(t *testing.T) {
	co := New(nil)
	started := make(chan struct{})
	release := make(chan struct{})
	blocker := co.Create("barrier-owner", func(Thread) {
		close(started)
		<-release
	})
	<-started

	barrierLocked := make(chan struct{})
	barrierDone := make(chan bool, 1)
	go func() {
		stopStarted, err := co.RunAfterStopIf(context.Background(), func() bool {
			close(barrierLocked)
			return true
		}, nil)
		barrierDone <- stopStarted && err == nil
	}()
	<-barrierLocked

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	startedAt := time.Now()
	if err := co.RunAfterStop(timeoutCtx, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("barrier wait error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("barrier lock wait took %v, want a bounded timeout", elapsed)
	}

	close(release)
	co.Join(blocker)
	waitForDrainResult(t, barrierDone)
}

func TestRunAfterStopRejectsSynchronousPanicHandlerReentry(t *testing.T) {
	guarded := make(chan any, 1)
	var co *Coroutines
	co = New(func(PanicReport) {
		func() {
			defer func() { guarded <- recover() }()
			co.RunAfterStop(drainTestContext(t), nil)
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
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
		t.Fatal("manager remained blocked after panic handler returned")
	}
}

func TestRunAfterStopWaitsForPanicHandlerBeforeReopeningAdmission(t *testing.T) {
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	var releaseHandlerOnce sync.Once
	co := New(func(PanicReport) {
		close(handlerStarted)
		<-releaseHandler
	})

	workerStarted := make(chan struct{})
	releaseWorker := make(chan struct{})
	var releaseWorkerOnce sync.Once
	worker := co.Create("panicking-worker", func(Thread) {
		close(workerStarted)
		<-releaseWorker
		panic("worker failure")
	})
	t.Cleanup(func() {
		releaseWorkerOnce.Do(func() { close(releaseWorker) })
		releaseHandlerOnce.Do(func() { close(releaseHandler) })
		if co.RunAfterStop(drainTestContext(t), nil) != nil {
			t.Error("panicking worker did not drain during cleanup")
		}
	})
	waitForThreadSignal(t, workerStarted, "panicking worker did not start")

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := co.RunAfterStop(timeoutCtx, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("worker drain error = %v, want context.DeadlineExceeded", err)
	}
	releaseWorkerOnce.Do(func() { close(releaseWorker) })
	waitForThreadSignal(t, handlerStarted, "panic handler did not start")

	var rejectedRan atomic.Bool
	rejected := co.Create("during-panic-handler", func(Thread) {
		rejectedRan.Store(true)
	})
	if !rejected.Stopped() {
		t.Fatal("creation was admitted while the panic handler was running")
	}
	waitForThreadSignal(t, rejected.done, "rejected creation did not finish")
	if rejectedRan.Load() {
		t.Fatal("creation ran while the panic handler was running")
	}

	barrierStarted := make(chan struct{})
	barrierCallback := make(chan struct{})
	barrierResult := make(chan bool, 1)
	go func() {
		close(barrierStarted)
		barrierResult <- co.RunAfterStop(drainTestContext(t), func() {
			close(barrierCallback)
		}) == nil
	}()
	waitForThreadSignal(t, barrierStarted, "second shutdown barrier did not start")
	select {
	case <-barrierCallback:
		t.Fatal("second shutdown barrier ran its callback while the panic handler was running")
	case result := <-barrierResult:
		t.Fatalf("second shutdown barrier returned %v while the panic handler was running", result)
	case <-time.After(50 * time.Millisecond):
	}

	releaseHandlerOnce.Do(func() { close(releaseHandler) })
	waitForThreadSignal(t, barrierCallback, "second shutdown barrier did not run its callback after the panic handler returned")
	select {
	case result := <-barrierResult:
		if !result {
			t.Fatal("second shutdown barrier timed out after the panic handler returned")
		}
	case <-time.After(time.Second):
		t.Fatal("second shutdown barrier did not return after the panic handler returned")
	}
	waitForThreadSignal(t, worker.done, "panicking worker did not finish")

	var admittedRan atomic.Bool
	admitted := co.Create("after-panic-handler", func(Thread) {
		admittedRan.Store(true)
	})
	waitForThreadSignal(t, admitted.done, "creation remained blocked after the panic handler returned")
	if !admittedRan.Load() {
		t.Fatal("creation was rejected after the panic handler returned")
	}
}

type observedErrContext struct {
	context.Context
	onErr func()
}

func (c *observedErrContext) Err() error {
	c.onErr()
	return c.Context.Err()
}

type observedDoneContext struct {
	context.Context
	onDone func()
}

func (c *observedDoneContext) Done() <-chan struct{} {
	c.onDone()
	return c.Context.Done()
}

func waitForShutdown(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(time.Second):
		t.Fatal("context shutdown did not return")
		return nil
	}
}

func assertShutdownRejectsCreation(t *testing.T, co *Coroutines) {
	t.Helper()
	ran := false
	thread := co.Create("during-canceled-shutdown", func(Thread) { ran = true })
	waitForThreadSignal(t, thread.done, "rejected coroutine did not finish")
	if !co.admissionClosed() || !thread.Stopped() || ran {
		t.Fatal("canceled shutdown admitted new work before explicit recovery")
	}
}

func waitForDrainResult(t *testing.T, result <-chan bool) {
	t.Helper()
	select {
	case completed := <-result:
		if !completed {
			t.Fatal("drain timed out after its remaining work completed")
		}
	case <-time.After(time.Second):
		t.Fatal("drain did not wake after its remaining work completed")
	}
}

func drainTestContext(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	return ctx
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
