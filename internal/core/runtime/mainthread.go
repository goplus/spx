package runtime

import "github.com/goplus/spx/v2/internal/engine/platform"

// These wrappers keep the main-thread marker API available in the runtime
// package while delegating to the shared platform-level implementation used by
// coroutine/enginewrap/gdengine.
// Prefer RunOnMainThread unless you need to span the marker across multiple
// calls, and always pair EnterMainThread with defer ExitMainThread in the same
// stack frame.
func EnterMainThread() {
	platform.EnterMainThread()
}

// ExitMainThread clears the manual main-thread marker set by EnterMainThread.
func ExitMainThread() {
	platform.ExitMainThread()
}

// IsMainThread reports whether the current goroutine is marked as executing
// engine main-thread work.
func IsMainThread() bool {
	return platform.IsMainThread()
}

// RunOnMainThread marks the current goroutine as main-thread work for the
// duration of call.
func RunOnMainThread(call func()) {
	platform.RunOnMainThread(call)
}
