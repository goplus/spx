package engine

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type executeWorkerPlatform struct{ gdx.IPlatformMgr }

func (executeWorkerPlatform) IsMainThread() bool { return false }

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	return ctx
}

func waitForEngineSignal(t *testing.T, done <-chan struct{}, timeout time.Duration, failure string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal(failure)
	}
}

func setupExecuteTest(t *testing.T) *coroutine.Coroutines {
	t.Helper()
	co := coroutine.New(nil)
	previous, previousPlatform := gco, gdx.PlatformMgr
	SetCoroutines(co)
	gdx.PlatformMgr = executeWorkerPlatform{}
	t.Cleanup(func() {
		co.RunAfterStop(nil)
		SetCoroutines(previous)
		gdx.PlatformMgr = previousPlatform
	})
	return co
}

func blockExecuteScheduler(t *testing.T, co *coroutine.Coroutines) func() {
	t.Helper()
	started, unblock := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(unblock) })
	t.Cleanup(release)
	co.Create("blocker", func(coroutine.Thread) {
		close(started)
		<-unblock
	})
	waitForEngineSignal(t, started, time.Second, "scheduler blocker did not start")
	return release
}

func TestExecuteReturnsWhenStoppedBeforeStart(t *testing.T) {
	co := setupExecuteTest(t)
	release := blockExecuteScheduler(t, co)
	returned := make(chan struct{})
	var called atomic.Bool
	go func() {
		Execute("pending", func(context.Context, any) { called.Store(true) })
		close(returned)
	}()

	stopped := false
	deadline := time.Now().Add(time.Second)
	for !stopped {
		co.StopIf(func(thread coroutine.Thread) bool {
			if thread.Owner == "pending" {
				stopped = true
				return true
			}
			return false
		})
		if time.Now().After(deadline) {
			t.Fatal("Execute did not register its coroutine")
		}
		runtime.Gosched()
	}
	release()
	waitForEngineSignal(t, returned, time.Second, "Execute remained blocked after its coroutine stopped before starting")
	if called.Load() {
		t.Fatal("Execute ran a canceled callback")
	}
}

func TestExecuteReturnsWhenAdmissionClosed(t *testing.T) {
	co := setupExecuteTest(t)
	started, canceled, unblock := make(chan struct{}), make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(unblock) })
	t.Cleanup(release)
	co.Create("blocker", func(thread coroutine.Thread) {
		close(started)
		<-thread.Context().Done()
		close(canceled)
		<-unblock
	})
	waitForEngineSignal(t, started, time.Second, "scheduler blocker did not start")
	drained := make(chan struct{})
	go func() {
		co.RunAfterStop(nil)
		close(drained)
	}()
	waitForEngineSignal(t, canceled, time.Second, "shutdown did not cancel the blocker")
	returned := make(chan struct{})
	var called atomic.Bool
	go func() {
		Execute("rejected", func(context.Context, any) { called.Store(true) })
		close(returned)
	}()
	waitForEngineSignal(t, returned, time.Second, "Execute remained blocked with coroutine admission closed")
	if called.Load() {
		t.Fatal("Execute ran a rejected callback")
	}
	select {
	case <-drained:
		t.Fatal("shutdown completed before the blocker exited")
	default:
	}
	release()
	waitForEngineSignal(t, drained, time.Second, "shutdown did not finish after the blocker exited")
}

func TestExecuteKeepsCurrentCoroutineAndSuppliedOwner(t *testing.T) {
	co := setupExecuteTest(t)
	var called bool
	thread := co.Create("original", func(thread coroutine.Thread) {
		Execute(nil, func(ctx context.Context, owner any) {
			called = true
			if ctx != thread.Context() || co.Current() != thread || owner != nil {
				t.Error("Execute changed the current coroutine, context, or supplied owner")
			}
		})
	})
	co.Join(thread)
	if !called {
		t.Fatal("Execute did not call the callback")
	}
}

func TestExecuteWaitsForCompletionWithDefaultOwner(t *testing.T) {
	setupExecuteTest(t)
	previousGame := GetGame()
	game := &struct{}{}
	SetGame(game)
	t.Cleanup(func() { SetGame(previousGame) })
	started, unblock, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(unblock) })
	t.Cleanup(release)
	var callbackContext context.Context
	go func() {
		Execute(nil, func(ctx context.Context, owner any) {
			callbackContext = ctx
			if !IsInCoroutine() || owner != game || currentThread().Owner != game {
				t.Error("Execute did not use a managed coroutine owned by the game")
			}
			close(started)
			<-unblock
		})
		close(returned)
	}()
	waitForEngineSignal(t, started, time.Second, "Execute callback did not start")
	select {
	case <-returned:
		t.Fatal("Execute returned while its callback was running")
	default:
	}
	release()
	waitForEngineSignal(t, returned, time.Second, "Execute did not return after its callback completed")
	if callbackContext.Err() == nil {
		t.Fatal("Execute returned before its coroutine completed")
	}
}
