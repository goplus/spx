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
	scheduler := New(nil)
	scheduler.JoinFirstSlice(nil)
}

func TestJoinRejectsForeignThreads(t *testing.T) {
	for _, test := range []struct {
		name string
		join func(*Scheduler, Thread)
	}{
		{"done", (*Scheduler).Join},
		{"first-slice", (*Scheduler).JoinFirstSlice},
		{"all", func(scheduler *Scheduler, target Thread) { scheduler.JoinAll([]Thread{nil, target}) }},
	} {
		for _, completed := range []bool{false, true} {
			name := test.name + "/live"
			if completed {
				name = test.name + "/completed"
			}
			t.Run(name, func(t *testing.T) {
				scheduler, other := New(nil), New(nil)
				release := make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				t.Cleanup(func() {
					unblock()
					scheduler.RunAfterStop(nil)
					other.RunAfterStop(nil)
				})
				target := other.Create("foreign", func(Thread) { <-release })
				if completed {
					unblock()
					other.Join(target)
				}
				for _, managed := range []bool{false, true} {
					result := make(chan any, 1)
					attempt := func() { result <- catchPanic(func() { test.join(scheduler, target) }) }
					if managed {
						scheduler.Create("caller", func(Thread) { attempt() })
					} else {
						go attempt()
					}
					select {
					case got := <-result:
						if got != ErrForeignThread {
							t.Fatalf("managed=%v: panic = %v, want ErrForeignThread", managed, got)
						}
					case <-time.After(time.Second):
						t.Fatalf("managed=%v: foreign Join waited instead of rejecting", managed)
					}
				}
				unblock()
				scheduler.RunAfterStop(nil)
				other.RunAfterStop(nil)
				// A rejected waiter must never enter the target manager's scheduler.
				for _, manager := range []*Scheduler{scheduler, other} {
					manager.schedulerMu.Lock()
					pending := manager.hasPendingExecutionLocked()
					manager.schedulerMu.Unlock()
					if pending {
						t.Fatal("foreign Join left executable work in a manager")
					}
					updated := make(chan struct{})
					go func() { manager.Update(); close(updated) }()
					waitForThreadSignal(t, updated, "Update blocked after rejected Join")
				}
			})
		}
	}
}

func TestJoinAllValidatesBeforeWaiting(t *testing.T) {
	scheduler, other := New(nil), New(nil)
	t.Cleanup(func() { scheduler.RunAfterStop(nil); other.RunAfterStop(nil) })
	local := scheduler.Create("local", func(Thread) { scheduler.Wait(1e9) })
	scheduler.JoinFirstSlice(local)
	foreign := other.Create("foreign", func(Thread) {})
	other.Join(foreign)
	result := make(chan any, 1)
	caller := scheduler.Create("caller", func(Thread) {
		result <- catchPanic(func() { scheduler.JoinAll([]Thread{nil, local, foreign}) })
	})
	select {
	case got := <-result:
		if got != ErrForeignThread {
			t.Fatalf("panic = %v, want ErrForeignThread", got)
		}
	case <-time.After(time.Second):
		t.Fatal("JoinAll waited for an earlier target before validating the batch")
	}
	waitForThreadSignal(t, caller.done, "rejected caller did not finish")
	local.joinWaiters.mu.Lock()
	waiters := len(local.joinWaiters.threads)
	local.joinWaiters.mu.Unlock()
	if waiters != 0 {
		t.Fatal("JoinAll registered a waiter before validating all targets")
	}
}

func TestJoinCanceledWaiterIsRemoved(t *testing.T) {
	for _, test := range []struct {
		name    string
		join    func(*Scheduler, Thread)
		waiters func(Thread) *waiterSet
	}{
		{
			name:    "done",
			join:    (*Scheduler).Join,
			waiters: func(th Thread) *waiterSet { return &th.joinWaiters },
		},
		{
			name:    "first-yield-or-done",
			join:    (*Scheduler).JoinFirstSlice,
			waiters: func(th Thread) *waiterSet { return &th.firstSliceWaiters },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			scheduler := New(nil)
			registered := make(chan Thread, 1)
			release := make(chan struct{})
			batchDone := make(chan struct{})
			var releaseOnce sync.Once
			releaseTarget := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(func() {
				releaseTarget()
				scheduler.RunAfterStop(nil)
				waitForThreadSignal(t, batchDone, "batch registration did not finish")
			})

			// Hold the target before its first run, so both join kinds must
			// register even while canceled waiters repeatedly finish.
			go func() {
				defer close(batchDone)
				scheduler.StartBatch([]Task{{
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
			survivor := scheduler.Create("survivor", func(Thread) {
				test.join(scheduler, target)
				close(survivorReturned)
			})
			waitForThreadSignal(t, survivor.firstSliceDone, "survivor did not wait")

			for range 16 {
				waiter := scheduler.Create("canceled-waiter", func(Thread) {
					test.join(scheduler, target)
				})
				waitForThreadSignal(t, waiter.firstSliceDone, "waiter did not wait")
				scheduler.Stop(waiter)
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

func TestCompletionResumesWaitersInScriptOrder(t *testing.T) {
	for _, test := range []struct {
		name           string
		earlier, later func(*Scheduler, Thread)
	}{
		{"join before first slice", (*Scheduler).Join, (*Scheduler).JoinFirstSlice},
		{"first slice before join", (*Scheduler).JoinFirstSlice, (*Scheduler).Join},
	} {
		t.Run(test.name, func(t *testing.T) {
			scheduler := New(nil)
			t.Cleanup(func() { scheduler.RunAfterStop(nil) })
			// An unpublished target can finish cancellation without a scheduler
			// permit, making both kinds of waiters eligible at once.
			target := scheduler.registerTask(scheduler.captureAdmission(), Task{
				Run: func(Thread) { t.Error("unpublished canceled target ran") },
			})
			order := make(chan string, 2)
			earlier := scheduler.Create("earlier", func(Thread) {
				test.earlier(scheduler, target)
				order <- "earlier"
			})
			waitForThreadSignal(t, earlier.firstSliceDone, "earlier waiter did not block")
			later := scheduler.Create("later", func(Thread) {
				test.later(scheduler, target)
				order <- "later"
			})
			waitForThreadSignal(t, later.firstSliceDone, "later waiter did not block")
			// Wait for the last yield to release its scheduler permit.
			scheduler.runMu.Lock()
			scheduler.runMu.Unlock()
			target.Cancel()
			waitForThreadSignal(t, earlier.done, "earlier waiter did not finish")
			waitForThreadSignal(t, later.done, "later waiter did not finish")
			if first, second := <-order, <-order; first != "earlier" || second != "later" {
				t.Fatalf("resumption order = %s, %s; want earlier, later", first, second)
			}
		})
	}
}

func TestJoinResumesCallerAfterPeerCompletes(t *testing.T) {
	scheduler := New(nil)
	releasePeer := make(chan struct{})
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := scheduler.Create("peer", func(peer Thread) {
		peerReady <- peer
		WaitForChan(scheduler, releasePeer)
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	scheduler.Create("caller", func(me Thread) {
		scheduler.Join(peer)
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
	scheduler := New(nil)
	releaseA := make(chan struct{})
	releaseB := make(chan struct{})
	peerAReady := make(chan Thread, 1)
	peerBReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peerA := scheduler.Create("peer-a", func(peer Thread) {
		peerAReady <- peer
		WaitForChan(scheduler, releaseA)
	})
	peerB := scheduler.Create("peer-b", func(peer Thread) {
		peerBReady <- peer
		WaitForChan(scheduler, releaseB)
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

	scheduler.Create("caller", func(me Thread) {
		scheduler.JoinAll([]Thread{nil, me, peerA, peerB, peerA, nil, me})
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
	scheduler := New(nil)
	releasePeer := make(chan struct{})
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := scheduler.Create("peer", func(peer Thread) {
		peerReady <- peer
		WaitForChan(scheduler, releasePeer)
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	scheduler.Create("caller", func(me Thread) {
		scheduler.JoinFirstSlice(peer)
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
	scheduler := New(nil)
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := scheduler.Create("peer", func(peer Thread) {
		peerReady <- peer
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	scheduler.Create("caller", func(me Thread) {
		scheduler.JoinFirstSlice(peer)
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
	scheduler := New(nil)

	peerStarted := make(chan struct{}, 1)
	observed := make(chan bool, 1)
	done := make(chan struct{})

	scheduler.Create("caller", func(me Thread) {
		peer := scheduler.Create("peer", func(peer Thread) {
			close(peerStarted)
		})
		scheduler.JoinFirstSlice(peer)

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

		scheduler.Update()
		time.Sleep(time.Millisecond)
	}
}
