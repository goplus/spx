package enginewrap

import (
	"testing"

	"github.com/goplus/spx/v2/internal/engine/platform"
)

func TestCallInMainThreadFastPath(t *testing.T) {
	originalCallback := mainCallback
	t.Cleanup(func() {
		mainCallback = originalCallback
	})

	mainCallback = func(func()) {
		t.Fatal("mainCallback should not be used on the main thread fast path")
	}

	called := false
	platform.RunOnMainThread(func() {
		callInMainThread(func() {
			called = true
		})
	})

	if !called {
		t.Fatal("callInMainThread should execute immediately on the main thread")
	}
}

func TestCallInMainThreadRequiresInitOffMainThread(t *testing.T) {
	originalCallback := mainCallback
	t.Cleanup(func() {
		mainCallback = originalCallback
	})

	mainCallback = nil

	defer func() {
		if r := recover(); r != "enginewrap: Init must be called before using manager methods off the main thread" {
			t.Fatalf("panic = %v, want explicit Init requirement", r)
		}
	}()

	callInMainThread(func() {})
}
