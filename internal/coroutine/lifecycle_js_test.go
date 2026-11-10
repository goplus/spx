//go:build js && wasm

package coroutine

import (
	"syscall/js"
	"testing"
)

func TestDrainLetsJavaScriptCompleteWorker(t *testing.T) {
	scheduler := New(nil)
	started, hostDone := make(chan struct{}), make(chan struct{})
	scheduler.Create("host-worker", func(Thread) {
		scheduler.WaitToDo(func() {
			close(started)
			<-hostDone
		})
	})
	waitForThreadSignal(t, started, "worker did not start")
	closeFromJavaScript(t, hostDone)

	// This runs in the Go test goroutine, outside a synchronous JS callback.
	// With no deadline, only the host callback can let the worker finish.
	scheduler.RunAfterStop(nil)
	select {
	case <-hostDone:
	default:
		t.Fatal("drain returned before the host callback ran")
	}
}

func TestContendedDrainLetsJavaScriptFinishShutdown(t *testing.T) {
	scheduler := New(nil)
	started, hostDone := make(chan struct{}), make(chan struct{})
	firstDone := make(chan struct{})
	go func() {
		scheduler.RunAfterStop(func() {
			close(started)
			<-hostDone
		})
		close(firstDone)
	}()
	waitForThreadSignal(t, started, "first shutdown callback did not start")
	closeFromJavaScript(t, hostDone)
	// The first shutdown holds the gate until JavaScript calls back.
	scheduler.RunAfterStop(nil)
	waitForThreadSignal(t, firstDone, "first shutdown did not finish")
}

func TestEngineDispatchLetsJavaScriptCompleteWorker(t *testing.T) {
	scheduler := New(nil)
	hostDone := make(chan struct{})
	completed := false
	scheduler.RunSync("host-dispatch", func(Thread) {
		closeFromJavaScript(t, hostDone)
		scheduler.WaitToDo(func() { <-hostDone })
		completed = true
	})
	if !completed {
		t.Fatal("dispatch returned before its worker completed")
	}
}

func TestRunBetweenScriptsLetsJavaScriptReleaseScript(t *testing.T) {
	scheduler := New(nil)
	started, hostDone := make(chan struct{}), make(chan struct{})
	scheduler.Create("host-owner", func(Thread) {
		close(started)
		<-hostDone
	})
	waitForThreadSignal(t, started, "script did not acquire execution")
	closeFromJavaScript(t, hostDone)

	called := false
	scheduler.RunBetweenScripts(func() { called = true })
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
