package coroutine

import (
	"testing"

	"github.com/goplus/spx/v2/internal/engine/platform"
)

func TestWaitMainThreadFastPathOnMainThread(t *testing.T) {
	co := New(nil)
	called := false

	platform.RunOnMainThread(func() {
		co.WaitMainThread(func() {
			called = true
		})
	})

	if !called {
		t.Fatal("WaitMainThread should execute immediately on the main thread")
	}
}
