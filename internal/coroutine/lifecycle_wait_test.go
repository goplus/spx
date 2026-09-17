package coroutine

import (
	"sync"
	"testing"
	"time"
)

func TestDrainWakesMultipleWaiters(t *testing.T) {
	co := New(nil)
	workerStarted, release := make(chan struct{}), make(chan struct{})
	releaseWorker := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		releaseWorker()
		if !co.StopAllAndWait(time.Second) {
			t.Error("coroutines did not stop during cleanup")
		}
	})

	workerOwner := co.Create("worker-owner", func(Thread) int {
		co.WaitToDo(func() {
			close(workerStarted)
			<-release
		})
		return 0
	})
	waitForThreadSignal(t, workerStarted, "native worker did not start")
	latch := co.NewLatch()
	peer := co.Create("peer", func(Thread) int {
		latch.Wait()
		return 0
	})
	waitForThreadSignal(t, peer.yieldedOrDone, "peer did not suspend")

	const count = 6
	started := make(chan struct{}, count*2)
	skippingPeer, includingPeer := make(chan bool, count), make(chan bool, count)
	for i := range count {
		// Both zero and negative timeouts wait for lifecycle completion.
		timeout := -time.Duration(i % 2)
		go func() {
			started <- struct{}{}
			skippingPeer <- co.waitForDrain(timeout, peer)
		}()
		go func() {
			started <- struct{}{}
			includingPeer <- co.waitForDrain(timeout, nil)
		}()
	}
	for range count * 2 {
		waitForThreadSignal(t, started, "drain waiter did not start")
	}

	co.Stop(workerOwner)
	waitForThreadSignal(t, workerOwner.done, "native worker's caller did not stop")
	releaseWorker()
	for range count {
		waitForDrainResult(t, skippingPeer)
	}
	if peer.Stopped() || co.hasPendingWork(peer) {
		t.Fatal("drain did not preserve the skipped peer or left other work registered")
	}
	select {
	case <-includingPeer:
		t.Fatal("drain returned while its peer was still registered")
	default:
	}

	co.Stop(peer)
	for range count {
		waitForDrainResult(t, includingPeer)
	}
}

func TestStopAllAndWaitFromCoroutineWithoutDeadline(t *testing.T) {
	for _, timeout := range []time.Duration{0, -1} {
		t.Run(timeout.String(), func(t *testing.T) {
			co := New(nil)
			workerStarted, release := make(chan struct{}), make(chan struct{})
			releaseWorker := sync.OnceFunc(func() { close(release) })
			t.Cleanup(func() {
				releaseWorker()
				if !co.StopAllAndWait(time.Second) {
					t.Error("coroutines did not stop during cleanup")
				}
			})
			peer := co.Create("peer", func(Thread) int {
				co.WaitToDo(func() {
					close(workerStarted)
					<-release
				})
				return 0
			})
			waitForThreadSignal(t, workerStarted, "native worker did not start")

			result := make(chan bool, 1)
			caller := co.Create("drainer", func(Thread) int {
				result <- co.StopAllAndWait(timeout)
				return 0
			})
			waitForThreadSignal(t, peer.done, "drain did not stop its peer")
			select {
			case <-result:
				t.Fatal("drain returned before the native worker finished")
			default:
			}
			releaseWorker()
			waitForDrainResult(t, result)
			waitForThreadSignal(t, caller.done, "drainer waited for itself")
		})
	}
}

func TestDrainDoesNotSkipUnregisteredThread(t *testing.T) {
	co := New(nil)
	latch := co.NewLatch()
	thread := co.Create("registered", func(Thread) int {
		latch.Wait()
		return 0
	})
	t.Cleanup(func() {
		if !co.StopAllAndWait(time.Second) {
			t.Error("coroutine did not stop during cleanup")
		}
	})
	waitForThreadSignal(t, thread.yieldedOrDone, "registered thread did not suspend")
	if co.waitForDrain(time.Nanosecond, co.newThread("unregistered")) {
		t.Fatal("an unregistered skip hid the registered thread")
	}
}

func waitForDrainResult(t *testing.T, result <-chan bool) {
	t.Helper()
	select {
	case completed := <-result:
		if !completed {
			t.Fatal("drain timed out after its remaining work completed")
		}
	case <-time.After(time.Second):
		t.Fatal("drain did not wake after its remaining work completed")
	}
}
