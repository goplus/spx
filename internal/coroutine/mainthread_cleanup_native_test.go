//go:build !js

package coroutine

import "testing"

func TestCleanupMainThreadQueuedAllowsNestedEngineCallsAfterCancellation(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	cleaned := false
	thread := co.Create("caller", func(me Thread) {
		me.Cancel()
		co.CleanupMainThread(func() {
			co.WaitMainThread(func() { cleaned = true })
		})
	})
	waitForQueuedJob(t, co)
	// The cleanup was queued from a worker; nested wrappers run directly on the engine.
	setMainThreadForTest(t, true)
	co.Update()
	waitForThreadSignal(t, thread.done, "queued engine cleanup did not finish")
	co.RunAfterStop(nil)
	if !cleaned {
		t.Fatal("cancellation skipped the nested engine cleanup")
	}
}
