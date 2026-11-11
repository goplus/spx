//go:build !js

package coroutine

import "testing"

func TestCleanupMainThreadQueuedAllowsNestedEngineCallsAfterCancellation(t *testing.T) {
	setMainThreadForTest(t, false)
	scheduler := New(nil)
	cleaned := false
	thread := scheduler.Create("caller", func(me Thread) {
		me.Cancel()
		scheduler.CleanupMainThread(func() {
			scheduler.WaitMainThread(func() { cleaned = true })
		})
	})
	waitForQueuedJob(t, scheduler)
	// The cleanup was queued from a worker; nested wrappers run directly on the engine.
	setMainThreadForTest(t, true)
	scheduler.Update()
	waitForThreadSignal(t, thread.done, "queued engine cleanup did not finish")
	scheduler.RunAfterStop(nil)
	if !cleaned {
		t.Fatal("cancellation skipped the nested engine cleanup")
	}
}
