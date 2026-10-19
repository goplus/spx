/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package coroutine

import (
	"sync"
	"testing"
	"time"
)

func TestJoinFirstSliceNilIsNoOp(t *testing.T) {
	co := New(nil)
	co.JoinFirstSlice(nil)
}

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
			join:    (*Coroutines).JoinFirstSlice,
			waiters: func(th Thread) *waiterSet { return &th.firstSliceWaiters },
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
				co.RunAfterStop(nil)
				waitForThreadSignal(t, batchDone, "batch registration did not finish")
			})

			// Hold the target before its first run, so both join kinds must
			// register even while canceled waiters repeatedly finish.
			go func() {
				defer close(batchDone)
				co.StartBatch([]Task{{
					Owner: "target",
					RegisterInvocation: func(th Thread) func() {
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
			survivor := co.Create("survivor", func(Thread) {
				test.join(co, target)
				close(survivorReturned)
			})
			waitForThreadSignal(t, survivor.firstSliceDone, "survivor did not wait")

			for range 16 {
				waiter := co.Create("canceled-waiter", func(Thread) {
					test.join(co, target)
				})
				waitForThreadSignal(t, waiter.firstSliceDone, "waiter did not wait")
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

func TestJoinResumesCallerAfterPeerCompletes(t *testing.T) {
	co := New(nil)
	releasePeer := make(chan struct{})
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := co.Create("peer", func(peer Thread) {
		peerReady <- peer
		WaitForChan(co, releasePeer)
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.Create("caller", func(me Thread) {
		co.Join(peer)
		close(callerDone)
	})

	select {
	case <-callerDone:
		t.Fatal("caller returned before peer completed")
	default:
	}

	close(releasePeer)

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-callerDone:
	case <-timer.C:
		t.Fatal("caller did not resume after peer completed")
	}
}

func TestJoinAllWaitsForEveryPeer(t *testing.T) {
	co := New(nil)
	releaseA := make(chan struct{})
	releaseB := make(chan struct{})
	peerAReady := make(chan Thread, 1)
	peerBReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peerA := co.Create("peer-a", func(peer Thread) {
		peerAReady <- peer
		WaitForChan(co, releaseA)
	})
	peerB := co.Create("peer-b", func(peer Thread) {
		peerBReady <- peer
		WaitForChan(co, releaseB)
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerAReady:
	case <-timer.C:
		t.Fatal("peer A coroutine did not start")
	}

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerBReady:
	case <-timer.C:
		t.Fatal("peer B coroutine did not start")
	}

	co.Create("caller", func(me Thread) {
		co.JoinAll([]Thread{nil, me, peerA, peerB, peerA, nil, me})
		close(callerDone)
	})

	close(releaseA)

	select {
	case <-callerDone:
		t.Fatal("caller returned before every peer completed")
	default:
	}

	close(releaseB)

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-callerDone:
	case <-timer.C:
		t.Fatal("caller did not resume after every peer completed")
	}
}

func TestJoinFirstSliceResumesCallerAfterPeerFirstYield(t *testing.T) {
	co := New(nil)
	releasePeer := make(chan struct{})
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := co.Create("peer", func(peer Thread) {
		peerReady <- peer
		WaitForChan(co, releasePeer)
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.Create("caller", func(me Thread) {
		co.JoinFirstSlice(peer)
		close(callerDone)
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-callerDone:
	case <-timer.C:
		t.Fatal("caller did not resume after peer reached its first yield")
	}

	select {
	case <-releasePeer:
		t.Fatal("releasePeer should remain controlled by the test")
	default:
	}
	close(releasePeer)
}

func TestJoinFirstSliceReturnsWhenPeerExitsWithoutYield(t *testing.T) {
	co := New(nil)
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := co.Create("peer", func(peer Thread) {
		peerReady <- peer
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.Create("caller", func(me Thread) {
		co.JoinFirstSlice(peer)
		close(callerDone)
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-callerDone:
	case <-timer.C:
		t.Fatal("caller did not resume after peer returned without yielding")
	}
}

func TestJoinFirstSliceWaitsForChildFromCoroutine(t *testing.T) {
	co := New(nil)

	peerStarted := make(chan struct{}, 1)
	observed := make(chan bool, 1)
	done := make(chan struct{})

	co.Create("caller", func(me Thread) {
		peer := co.Create("peer", func(peer Thread) {
			close(peerStarted)
		})
		co.JoinFirstSlice(peer)

		select {
		case <-peerStarted:
			observed <- true
		default:
			observed <- false
		}
		close(done)
	})

	deadline := time.Now().Add(time.Second)
	for {
		select {
		case <-done:
			select {
			case got := <-observed:
				if !got {
					t.Fatal("JoinFirstSlice returned before the peer ran")
				}
			default:
				t.Fatal("caller completed without recording eager-start observation")
			}
			return
		default:
		}

		if time.Now().After(deadline) {
			t.Fatal("caller did not finish while pumping the scheduler")
		}

		co.Update()
		time.Sleep(time.Millisecond)
	}
}
