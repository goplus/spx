//go:build !js && !pure_engine

package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	itime "github.com/goplus/spx/v3/internal/time"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
	"github.com/visualfc/gid"
)

func TestExecuteRejectsMainThreadCallbackReentry(t *testing.T) {
	setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	done := make(chan struct{})
	var recovered any
	var nestedRan, subsequentRan bool
	go func() {
		defer close(done)
		platform.mainGID.Store(gid.Get())
		Execute("outer", func(context.Context, any) {
			WaitMainThread(func() {
				defer func() { recovered = recover() }()
				Execute("nested", func(context.Context, any) { nestedRan = true })
			})
		})
		Execute("subsequent", func(context.Context, any) { subsequentRan = true })
	}()
	waitForEngineSignal(t, done, "main-thread callback deadlocked reentering Execute")
	if recovered != coroutine.ErrReentrantWait || nestedRan || !subsequentRan {
		t.Fatalf("reentry panic = %v, nested ran = %v, subsequent ran = %v", recovered, nestedRan, subsequentRan)
	}
}

func TestExecuteRejectsBetweenScriptsReentry(t *testing.T) {
	co := setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	done := make(chan struct{})
	var recovered any
	var nestedRan, subsequentRan bool
	go func() {
		defer close(done)
		platform.mainGID.Store(gid.Get())
		co.RunBetweenScripts(func() {
			defer func() { recovered = recover() }()
			Execute("nested", func(context.Context, any) { nestedRan = true })
		})
		co.RunBetweenScripts(func() { subsequentRan = true })
	}()
	waitForEngineSignal(t, done, "between-scripts callback deadlocked reentering Execute")
	if recovered != coroutine.ErrReentrantWait || nestedRan || !subsequentRan {
		t.Fatalf("reentry panic = %v, nested ran = %v, subsequent ran = %v", recovered, nestedRan, subsequentRan)
	}
}

func TestExecuteFromNativeWorkerCanCallMainThread(t *testing.T) {
	setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	done := make(chan struct{})
	called := false
	go func() {
		defer close(done)
		platform.mainGID.Store(gid.Get())
		Execute("outer", func(context.Context, any) {
			ExecuteNative(func(context.Context, any) {
				Execute("nested", func(context.Context, any) {
					WaitMainThread(func() { called = platform.IsMainThread() })
				})
			})
		})
	}()
	waitForEngineSignal(t, done, "worker could not reenter Execute and call the engine")
	if !called {
		t.Fatal("worker's managed callback did not run its engine call on the main thread")
	}
}

func TestEngineDrainServicesWorkerMainThreadCleanup(t *testing.T) {
	co := setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	started := make(chan struct{})
	called := false
	co.Create("caller", func(thread coroutine.Thread) {
		co.WaitToDo(func() {
			close(started)
			<-thread.Context().Done()
			WaitMainThread(func() { called = platform.IsMainThread() })
		})
	})
	waitForEngineSignal(t, started, "worker did not start")
	drained := make(chan struct{})
	go func() {
		platform.mainGID.Store(gid.Get())
		co.RunAfterStop(nil)
		close(drained)
	}()
	waitForEngineSignal(t, drained, "engine drain did not return")
	if !called {
		t.Fatal("drain did not service worker cleanup on the main thread")
	}
}

func TestEngineDrainStopsWaitsWithoutUpdate(t *testing.T) {
	for _, test := range []struct {
		name string
		wait func(*coroutine.Coroutines, coroutine.Thread)
	}{
		{"Wait", func(co *coroutine.Coroutines, _ coroutine.Thread) { co.Wait(3600) }},
		{"WaitNextFrame", func(co *coroutine.Coroutines, _ coroutine.Thread) { co.WaitNextFrame() }},
		{"WaitYield", (*coroutine.Coroutines).WaitYield},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := setupExecuteTest(t)
			platform := &executeMainThreadPlatform{}
			platform.mainGID.Store(gid.Get())
			gdx.PlatformMgr = platform
			cleaned, afterCalled := false, false
			thread := co.Create("suspended-script", func(me coroutine.Thread) {
				defer co.CleanupMainThread(func() {
					if !platform.IsMainThread() {
						t.Error("cleanup did not run on the engine thread")
					}
					cleaned = true
				})
				test.wait(co, me)
				t.Error("canceled script resumed after its wait")
			})
			co.JoinFirstSlice(thread)
			frame, round := itime.Frame(), co.ScriptRound()
			co.RunAfterStop(func() {
				afterCalled = true
				if !cleaned {
					t.Error("shutdown callback ran before script cleanup")
				}
			})
			if !thread.Stopped() || !afterCalled {
				t.Fatalf("shutdown stopped=%v after=%v", thread.Stopped(), afterCalled)
			}
			if itime.Frame() != frame || co.ScriptRound() != round {
				t.Fatal("shutdown advanced a frame or script round")
			}
		})
	}
}

func TestEngineDrainServicesJobsWhileAnotherDrainOwnsLock(t *testing.T) {
	co := setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)
	called := false
	co.Create("caller", func(thread coroutine.Thread) {
		co.WaitToDo(func() {
			close(started)
			<-thread.Context().Done()
			close(canceled)
			<-release
			WaitMainThread(func() { called = platform.IsMainThread() })
		})
	})
	waitForEngineSignal(t, started, "worker did not start")
	first := make(chan struct{})
	go func() {
		co.RunAfterStop(nil)
		close(first)
	}()
	// Cancellation proves the first drain owns the shutdown gate before the engine waits.
	waitForEngineSignal(t, canceled, "first drain did not cancel the caller")
	engineStarted := make(chan struct{})
	second := make(chan struct{})
	go func() {
		platform.mainGID.Store(gid.Get())
		close(engineStarted)
		co.RunAfterStop(nil)
		close(second)
	}()
	waitForEngineSignal(t, engineStarted, "engine drain did not start")
	unblock()
	waitForEngineSignal(t, first, "first engine drain did not return")
	waitForEngineSignal(t, second, "second engine drain did not return")
	if !called {
		t.Fatal("waiting for the shutdown gate prevented worker cleanup on the main thread")
	}
}

func waitForEngineSignal(t *testing.T, done <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal(failure)
	}
}
