package platform

import "testing"

func TestRunOnMainThreadTracksState(t *testing.T) {
	if IsMainThread() {
		t.Fatal("IsMainThread() should be false before RunOnMainThread")
	}

	inside := false
	RunOnMainThread(func() {
		inside = IsMainThread()
	})

	if !inside {
		t.Fatal("IsMainThread() should be true inside RunOnMainThread")
	}
	if IsMainThread() {
		t.Fatal("IsMainThread() should be false after RunOnMainThread")
	}
}

func TestRunOnMainThreadSupportsNestedCalls(t *testing.T) {
	steps := 0
	RunOnMainThread(func() {
		if !IsMainThread() {
			t.Fatal("outer RunOnMainThread should set the main thread marker")
		}
		steps++
		RunOnMainThread(func() {
			if !IsMainThread() {
				t.Fatal("nested RunOnMainThread should keep the main thread marker")
			}
			steps++
		})
		if !IsMainThread() {
			t.Fatal("outer RunOnMainThread should still be active after nesting")
		}
	})

	if steps != 2 {
		t.Fatalf("steps = %d, want 2", steps)
	}
	if IsMainThread() {
		t.Fatal("main thread marker leaked after nested RunOnMainThread")
	}
}

func TestRunOnMainThreadDoesNotLeakToOtherGoroutines(t *testing.T) {
	result := make(chan bool, 1)

	RunOnMainThread(func() {
		go func() {
			result <- IsMainThread()
		}()

		if <-result {
			t.Fatal("main thread marker should not leak to sibling goroutines")
		}
		if !IsMainThread() {
			t.Fatal("main goroutine should keep the main thread marker")
		}
	})
}
