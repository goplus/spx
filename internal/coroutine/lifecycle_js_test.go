//go:build js && wasm

package coroutine

import (
	"syscall/js"
	"testing"
)

func TestDrainLetsJavaScriptCompleteWorker(t *testing.T) {
	co := New(nil)
	started, hostDone := make(chan struct{}), make(chan struct{})
	co.Create("host-worker", func(Thread) int {
		co.WaitToDo(func() {
			close(started)
			<-hostDone
		})
		return 0
	})
	waitForThreadSignal(t, started, "worker did not start")
	closeFromJavaScript(t, hostDone)

	// This runs in the Go test goroutine, outside a synchronous JS callback.
	// With no deadline, only the host callback can let the worker finish.
	if !co.RunAfterStopAll(0, nil) {
		t.Fatal("drain did not wait for the host callback")
	}
	select {
	case <-hostDone:
	default:
		t.Fatal("drain returned before the host callback ran")
	}
}

func TestContendedDrainLetsJavaScriptFinishShutdown(t *testing.T) {
	co := New(nil)
	started, hostDone := make(chan struct{}), make(chan struct{})
	firstDone := make(chan bool, 1)
	go func() {
		firstDone <- co.RunAfterStopAll(0, func() {
			close(started)
			<-hostDone
		})
	}()
	waitForThreadSignal(t, started, "first shutdown callback did not start")
	closeFromJavaScript(t, hostDone)

	// The first shutdown holds shutdownMu until JavaScript calls back.
	if !co.RunAfterStopAll(0, nil) {
		t.Fatal("second drain did not complete")
	}
	waitForDrainResult(t, firstDone)
}

func TestEngineDispatchLetsJavaScriptCompleteWorker(t *testing.T) {
	co := New(nil)
	hostDone := make(chan struct{})
	completed := false
	if !co.TryRunFromEngine("host-dispatch", func() {
		closeFromJavaScript(t, hostDone)
		co.WaitToDo(func() { <-hostDone })
		completed = true
	}) {
		t.Fatal("direct platform did not handle dispatch")
	}
	if !completed {
		t.Fatal("dispatch returned before its worker completed")
	}
}

func TestRunBetweenScriptsLetsJavaScriptReleaseScript(t *testing.T) {
	co := New(nil)
	started, hostDone := make(chan struct{}), make(chan struct{})
	co.Create("host-owner", func(Thread) int {
		close(started)
		<-hostDone
		return 0
	})
	waitForThreadSignal(t, started, "script did not acquire execution")
	closeFromJavaScript(t, hostDone)

	called := false
	co.RunBetweenScripts(func() { called = true })
	if !called {
		t.Fatal("callback did not run after the script released execution")
	}
}

func closeFromJavaScript(t *testing.T, done chan struct{}) {
	t.Helper()
	callback := js.FuncOf(func(js.Value, []js.Value) any {
		close(done)
		return nil
	})
	timer := js.Global().Call("setTimeout", callback, 0)
	t.Cleanup(func() {
		js.Global().Call("clearTimeout", timer)
		callback.Release()
	})
}
