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

func setupExecuteTest(t *testing.T) *coroutine.Coroutines {
	t.Helper()
	co := coroutine.New(nil)
	previous, previousPlatform := gco, gdx.PlatformMgr
	SetCoroutines(co)
	gdx.PlatformMgr = executeWorkerPlatform{}
	t.Cleanup(func() {
		if !co.StopAllAndWait(time.Second) {
			t.Error("Execute test coroutines did not stop")
		}
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
	co.Create("blocker", func(coroutine.Thread) int {
		close(started)
		<-unblock
		return 0
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("scheduler blocker did not start")
	}
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
			if thread.Obj == "pending" {
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
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Execute remained blocked after its coroutine stopped before starting")
	}
	if called.Load() {
		t.Fatal("Execute ran a canceled callback")
	}
}

func TestExecuteReturnsWhenAdmissionClosed(t *testing.T) {
	co := setupExecuteTest(t)
	blockExecuteScheduler(t, co)
	if co.RunAfterStopAll(10*time.Millisecond, nil) {
		t.Fatal("shutdown completed while the blocker was running")
	}
	returned := make(chan struct{})
	var called atomic.Bool
	go func() {
		Execute("rejected", func(context.Context, any) { called.Store(true) })
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Execute remained blocked with coroutine admission closed")
	}
	if called.Load() {
		t.Fatal("Execute ran a rejected callback")
	}
}

func TestExecuteKeepsCurrentCoroutineAndSuppliedOwner(t *testing.T) {
	co := setupExecuteTest(t)
	var called bool
	thread := co.Create("original", func(thread coroutine.Thread) int {
		Execute(nil, func(ctx context.Context, owner any) {
			called = true
			if ctx != thread.Context() || co.Current() != thread || owner != nil {
				t.Error("Execute changed the current coroutine, context, or supplied owner")
			}
		})
		return 0
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
			if !IsInCoroutine() || owner != game || GetCoroutineOwner() != game {
				t.Error("Execute did not use a managed coroutine owned by the game")
			}
			close(started)
			<-unblock
		})
		close(returned)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Execute callback did not start")
	}
	select {
	case <-returned:
		t.Fatal("Execute returned while its callback was running")
	default:
	}
	release()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Execute did not return after its callback completed")
	}
	if callbackContext.Err() == nil {
		t.Fatal("Execute returned before its coroutine completed")
	}
}
