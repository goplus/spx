//go:build !js && !pure_engine

package coroutine

import (
	"testing"
	"time"
)

func TestUpdateServicesMainThreadDuringStartup(t *testing.T) {
	setMainThreadForTest(t, false)
	for _, outcome := range []string{"success", "failure", "canceled"} {
		t.Run(outcome, func(t *testing.T) {
			co := New(func(PanicReport) {})
			t.Cleanup(func() {
				if !co.StopAllAndWait(time.Second) {
					t.Error("startup coroutine did not stop")
				}
			})
			called := false
			co.Create("startup", func(me Thread) int {
				co.WaitMainThread(func() {
					called = true
					if outcome == "canceled" {
						co.Stop(me)
					}
				})
				if outcome == "failure" {
					panic("startup failed")
				}
				co.OnInited()
				return 0
			})

			updated := make(chan struct{})
			go func() {
				co.Update()
				close(updated)
			}()
			waitForThreadSignal(t, updated, "startup's main-thread call prevented the frame from returning")
			if !called {
				t.Fatal("startup's main-thread call was not served")
			}
			if got, want := co.initialized.Load(), outcome == "success"; got != want {
				t.Fatalf("initialized = %v, want %v", got, want)
			}
		})
	}
}
