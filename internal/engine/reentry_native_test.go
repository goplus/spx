//go:build !js && !pure_engine

package engine

import (
	"context"
	"runtime"
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
	drained := make(chan error, 1)
	go func() {
		platform.mainGID.Store(gid.Get())
		drained <- co.RunAfterStop(testContext(t), nil)
	}()
	waitForEngineDrain(t, drained)
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
			waitCtx, cancelWait := context.WithTimeout(context.Background(), time.Second)
			defer cancelWait()
			if err := co.RunAfterStop(waitCtx, func() {
				afterCalled = true
				if !cleaned {
					t.Error("shutdown callback ran before script cleanup")
				}
			}); err != nil {
				t.Fatalf("shutdown did not finish without Update: %v", err)
			}
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
	first := make(chan error, 1)
	go func() { first <- co.RunAfterStop(testContext(t), nil) }()
	// Cancellation proves the first drain owns the shutdown gate before the engine waits.
	waitForEngineSignal(t, canceled, "first drain did not cancel the caller")
	engineStarted := make(chan struct{})
	second := make(chan error, 1)
	go func() {
		platform.mainGID.Store(gid.Get())
		close(engineStarted)
		second <- co.RunAfterStop(testContext(t), nil)
	}()
	waitForEngineSignal(t, engineStarted, "engine drain did not start")
	unblock()
	waitForEngineDrain(t, first)
	waitForEngineDrain(t, second)
	if !called {
		t.Fatal("waiting for the shutdown gate prevented worker cleanup on the main thread")
	}
}

func TestReloadCleanupGoexit(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	game := new(bindingTestGame)
	binding, err := bindGame(game, new(struct{}))
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	thread := co.Create("reload-cleanup-goexit", func(thread coroutine.Thread) {
		defer co.CleanupMainThread(runtime.Goexit)
		close(started)
		<-thread.Context().Done()
	})
	waitForEngineSignal(t, started, "script did not start")
	done := make(chan struct{})
	go func() {
		defer close(done)
		platform.mainGID.Store(gid.Get())
		waitCtx, cancelWait := context.WithTimeout(context.Background(), time.Second)
		defer cancelWait()
		Reload(waitCtx, GetGame(), nil, func() error {
			t.Error("reload rebuilt after cleanup exited the engine goroutine")
			return nil
		}, func() { t.Error("reload activated after cleanup exited the engine goroutine") })
		t.Error("reload returned after cleanup called Goexit")
	}()
	waitForEngineSignal(t, done, "reload did not service cleanup on the engine thread")
	co.Join(thread)
	if binding.loadPhase() != gameStopped {
		t.Fatalf("phase after cleanup exited reload = %d, want stopped", binding.loadPhase())
	}
	rejected := co.Create("rejected-after-cleanup-goexit", func(coroutine.Thread) {
		t.Error("incomplete reload reopened coroutine admission")
	})
	if !rejected.Stopped() {
		t.Fatal("incomplete reload admitted a new coroutine")
	}
	co.Join(rejected)
	onReset()
	if activeGame.Load() != nil || game.reset.Load() != 1 {
		t.Fatal("reset did not recover the interrupted reload")
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

func waitForEngineDrain(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("engine drain failed before worker cleanup: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("engine drain did not return")
	}
}
