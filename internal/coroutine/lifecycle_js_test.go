//go:build js && wasm

package coroutine

import (
	"context"
	"errors"
	"syscall/js"
	"testing"
)

func TestDrainLetsJavaScriptCompleteWorker(t *testing.T) {
	co := New(nil)
	started, hostDone := make(chan struct{}), make(chan struct{})
	co.Create("host-worker", func(Thread) {
		co.WaitToDo(func() {
			close(started)
			<-hostDone
		})
	})
	waitForThreadSignal(t, started, "worker did not start")
	closeFromJavaScript(t, hostDone)

	// This runs in the Go test goroutine, outside a synchronous JS callback.
	// With no deadline, only the host callback can let the worker finish.
	if co.RunAfterStop(context.Background(), nil) != nil {
		t.Fatal("drain did not wait for the host callback")
	}
	select {
	case <-hostDone:
	default:
		t.Fatal("drain returned before the host callback ran")
	}
}

func TestContendedDrainLetsJavaScriptFinishShutdown(t *testing.T) {
	for _, test := range []struct {
		name     string
		deadline bool
	}{
		{"background", false},
		{"deadline", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := New(nil)
			started, hostDone := make(chan struct{}), make(chan struct{})
			firstDone := make(chan bool, 1)
			go func() {
				firstDone <- co.RunAfterStop(context.Background(), func() {
					close(started)
					<-hostDone
				}) == nil
			}()
			waitForThreadSignal(t, started, "first shutdown callback did not start")
			closeFromJavaScript(t, hostDone)

			ctx := context.Background()
			if test.deadline {
				ctx = drainTestContext(t)
			}
			// The first shutdown holds the gate until JavaScript calls back.
			if co.RunAfterStop(ctx, nil) != nil {
				t.Fatal("second drain did not complete")
			}
			waitForDrainResult(t, firstDone)
		})
	}
}

func TestShutdownAcceptsJavaScriptCancellation(t *testing.T) {
	for _, test := range []struct {
		name     string
		holdGate bool
	}{
		{"drain", false},
		{"gate", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := New(nil)
			if test.holdGate {
				held, release := make(chan struct{}), make(chan struct{})
				finished := make(chan error, 1)
				go func() {
					finished <- co.RunAfterStop(context.Background(), func() {
						close(held)
						<-release
					})
				}()
				t.Cleanup(func() {
					close(release)
					if err := waitForShutdown(t, finished); err != nil {
						t.Fatal(err)
					}
				})
				waitForThreadSignal(t, held, "first barrier was not acquired")
			} else {
				if !co.admitWorker(nil) {
					t.Fatal("worker was rejected")
				}
				t.Cleanup(co.finishWorker)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			hostDone := make(chan struct{})
			go func() {
				<-hostDone
				cancel()
			}()
			closeFromJavaScript(t, hostDone)
			// Run outside a synchronous JS callback so the host can deliver cancellation.
			if err := co.RunAfterStop(ctx, func() { t.Error("canceled wait ran its callback") }); !errors.Is(err, context.Canceled) {
				t.Fatalf("host cancellation = %v, want context.Canceled", err)
			}
		})
	}
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
	co.Create("host-owner", func(Thread) {
		close(started)
		<-hostDone
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
