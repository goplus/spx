package coroutine

import (
	"sync"
	"testing"
	"time"
)

func TestJoinCanceledWaiterIsRemoved(t *testing.T) {
	for _, test := range []struct {
		name    string
		join    func(*Coroutines, Thread)
		waiters func(Thread) *waiterSet
	}{
		{
			name:    "done",
			join:    (*Coroutines).Join,
			waiters: func(th Thread) *waiterSet { return &th.joinWaiters },
		},
		{
			name:    "first-yield-or-done",
			join:    (*Coroutines).JoinYieldedOrDone,
			waiters: func(th Thread) *waiterSet { return &th.yieldWaiters },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := New(nil)
			registered := make(chan Thread, 1)
			release := make(chan struct{})
			batchDone := make(chan struct{})
			var releaseOnce sync.Once
			releaseTarget := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(func() {
				releaseTarget()
				if !co.StopAllAndWait(time.Second) {
					t.Error("coroutines did not stop during cleanup")
				}
				waitForThreadSignal(t, batchDone, "batch registration did not finish")
			})

			// Hold the target before its first run, so both join kinds must
			// register even while canceled waiters repeatedly finish.
			go func() {
				defer close(batchDone)
				co.StartBatch([]Task{{
					Owner: "target",
					Setup: func(th Thread) func() {
						registered <- th
						<-release
						return nil
					},
					Run: func(Thread) {},
				}}, BatchAsync)
			}()
			var target Thread
			select {
			case target = <-registered:
			case <-time.After(time.Second):
				t.Fatal("target registration did not start")
			}

			survivorReturned := make(chan struct{})
			survivor := co.Create("survivor", func(Thread) int {
				test.join(co, target)
				close(survivorReturned)
				return 0
			})
			waitForThreadSignal(t, survivor.yieldedOrDone, "survivor did not wait")

			for range 16 {
				waiter := co.Create("canceled-waiter", func(Thread) int {
					test.join(co, target)
					return 0
				})
				waitForThreadSignal(t, waiter.yieldedOrDone, "waiter did not wait")
				co.Stop(waiter)
				waitForThreadSignal(t, waiter.done, "canceled waiter did not finish")

				waiters := test.waiters(target)
				waiters.mu.Lock()
				count := len(waiters.threads)
				_, survivorRegistered := waiters.threads[survivor]
				waiters.mu.Unlock()
				if count != 1 || !survivorRegistered {
					t.Fatalf("waiters after cancellation = %d, survivor registered = %v; want only survivor", count, survivorRegistered)
				}
			}

			select {
			case <-survivorReturned:
				t.Fatal("survivor returned before target ran")
			default:
			}
			releaseTarget()
			waitForThreadSignal(t, survivor.done, "survivor did not resume after target ran")
			waitForThreadSignal(t, survivorReturned, "survivor did not return from join")
			waitForThreadSignal(t, target.done, "target did not finish")
		})
	}
}
