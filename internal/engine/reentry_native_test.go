//go:build !js && !pure_engine

package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
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
	drained := make(chan bool, 1)
	go func() {
		platform.mainGID.Store(gid.Get())
		drained <- co.RunAfterStopAll(time.Second, nil)
	}()
	waitForEngineDrain(t, drained)
	if !called {
		t.Fatal("drain did not service worker cleanup on the main thread")
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
	first := make(chan bool, 1)
	go func() { first <- co.RunAfterStopAll(time.Second, nil) }()
	// Cancellation proves the first drain owns shutdownMu before the engine waits.
	waitForEngineSignal(t, canceled, "first drain did not cancel the caller")
	engineStarted := make(chan struct{})
	second := make(chan bool, 1)
	go func() {
		platform.mainGID.Store(gid.Get())
		close(engineStarted)
		second <- co.RunAfterStopAll(time.Second, nil)
	}()
	waitForEngineSignal(t, engineStarted, "engine drain did not start")
	unblock()
	waitForEngineDrain(t, first)
	waitForEngineDrain(t, second)
	if !called {
		t.Fatal("waiting for shutdownMu prevented worker cleanup on the main thread")
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

func waitForEngineDrain(t *testing.T, done <-chan bool) {
	t.Helper()
	select {
	case completed := <-done:
		if !completed {
			t.Fatal("engine drain timed out before worker cleanup")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("engine drain did not return")
	}
}
