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
	"runtime"
	"testing"
	"time"

	"github.com/goplus/spx/v2/internal/engine/platform"
)

type namedThreadObj struct{}

func (*namedThreadObj) Name() string {
	return "named-thread"
}

type invalidNamedThreadObj struct{}

func (*invalidNamedThreadObj) Name(_ string) string {
	return "invalid"
}

func canReenterCoroutineMutex(co *Coroutines) bool {
	if !co.mutex.TryLock() {
		return false
	}
	co.mutex.Unlock()
	return true
}

func TestResolveThreadNameUsesNameMethodWithoutReflectionCall(t *testing.T) {
	if got := resolveThreadName(&namedThreadObj{}); got != "named-thread" {
		t.Fatalf("expected name from interface method, got %q", got)
	}
}

func TestResolveThreadNameFallsBackForInvalidNameSignature(t *testing.T) {
	if got := resolveThreadName(&invalidNamedThreadObj{}); got != "*invalidNamedThreadObj" {
		t.Fatalf("expected fallback type name, got %q", got)
	}
}

func TestStopIfStopsActiveThread(t *testing.T) {
	co := New(nil)
	started := make(chan Thread, 1)
	done := make(chan struct{})

	co.CreateAndStart(true, "worker", func(me Thread) int {
		started <- me
		for {
			select {
			case <-me.Context().Done():
				close(done)
				return 0
			default:
				runtime.Gosched()
			}
		}
	})

	var th Thread
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case th = <-started:
	case <-timer.C:
		t.Fatal("coroutine did not start")
	}

	co.mutex.Lock()
	_, suspended := co.suspended[th]
	co.mutex.Unlock()
	if suspended {
		t.Fatal("test precondition failed: spinning thread should not be in suspended map")
	}

	co.StopIf(func(candidate Thread) bool {
		return candidate == th
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatal("active coroutine did not stop after StopIf")
	}
}

func TestStopIfEvaluatesFilterWithoutHoldingMutex(t *testing.T) {
	co := New(nil)
	started := make(chan Thread, 1)
	done := make(chan struct{})

	co.CreateAndStart(true, "worker", func(me Thread) int {
		started <- me
		for {
			select {
			case <-me.Context().Done():
				close(done)
				return 0
			default:
				runtime.Gosched()
			}
		}
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	var th Thread
	select {
	case th = <-started:
	case <-timer.C:
		t.Fatal("coroutine did not start")
	}

	var reacquired bool
	stopDone := make(chan struct{})
	go func() {
		co.StopIf(func(candidate Thread) bool {
			reacquired = canReenterCoroutineMutex(co)
			return candidate == th
		})
		close(stopDone)
	}()

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-stopDone:
	case <-timer.C:
		t.Fatal("StopIf deadlocked when filter re-entered the coroutine mutex")
	}
	if !reacquired {
		t.Fatal("StopIf evaluated filter while holding the coroutine mutex")
	}

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatal("active coroutine did not stop after StopIf")
	}
}

func TestAbortAllAndWaitFromCoroutineWaitsForPeersWithoutWaitingForCaller(t *testing.T) {
	co := New(nil)
	result := make(chan bool, 1)
	otherYielding := make(chan struct{})
	otherDone := make(chan struct{})

	co.CreateAndStart(true, "other", func(other Thread) int {
		defer close(otherDone)
		close(otherYielding)
		co.Yield(other)
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-otherYielding:
	case <-timer.C:
		t.Fatal("other coroutine did not reach yield")
	}

	co.CreateAndStart(true, "caller", func(me Thread) int {
		result <- co.AbortAllAndWait(time.Hour)
		return 0
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case completed := <-result:
		if !completed {
			t.Fatal("AbortAllAndWait should report success after other coroutines stop")
		}
	case <-timer.C:
		t.Fatal("AbortAllAndWait did not finish after peer coroutine stopped")
	}

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-otherDone:
	case <-timer.C:
		t.Fatal("other coroutine did not exit while caller was waiting")
	}
}

func TestAbortAllAndWaitFromCoroutineDoesNotStartStoppedPeer(t *testing.T) {
	co := New(nil)
	result := make(chan bool, 1)
	peerRan := make(chan struct{}, 1)

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.CreateAndStart(false, "peer", func(peer Thread) int {
			close(peerRan)
			return 0
		})
		result <- co.AbortAllAndWait(time.Second)
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case completed := <-result:
		if !completed {
			t.Fatal("AbortAllAndWait should wait for the stopped peer to unregister")
		}
	case <-timer.C:
		t.Fatal("AbortAllAndWait did not finish after stopped peer unregistered")
	}

	select {
	case <-peerRan:
		t.Fatal("stopped peer coroutine should not run")
	default:
	}
}

func TestJoinResumesCallerAfterPeerCompletes(t *testing.T) {
	co := New(nil)
	releasePeer := make(chan struct{})
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := co.CreateAndStart(true, "peer", func(peer Thread) int {
		peerReady <- peer
		var signal struct{}
		WaitForChan(co, releasePeer, &signal)
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.Join(peer)
		close(callerDone)
		return 0
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

	peerA := co.CreateAndStart(true, "peer-a", func(peer Thread) int {
		peerAReady <- peer
		var signal struct{}
		WaitForChan(co, releaseA, &signal)
		return 0
	})
	peerB := co.CreateAndStart(true, "peer-b", func(peer Thread) int {
		peerBReady <- peer
		var signal struct{}
		WaitForChan(co, releaseB, &signal)
		return 0
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

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.JoinAll([]Thread{peerA, peerB, peerA})
		close(callerDone)
		return 0
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

func TestJoinYieldedOrDoneResumesCallerAfterPeerFirstYield(t *testing.T) {
	co := New(nil)
	releasePeer := make(chan struct{})
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := co.CreateAndStart(true, "peer", func(peer Thread) int {
		peerReady <- peer
		var signal struct{}
		WaitForChan(co, releasePeer, &signal)
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.JoinYieldedOrDone(peer)
		close(callerDone)
		return 0
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

func TestJoinYieldedOrDoneReturnsWhenPeerExitsWithoutYield(t *testing.T) {
	co := New(nil)
	peerReady := make(chan Thread, 1)
	callerDone := make(chan struct{})

	peer := co.CreateAndStart(true, "peer", func(peer Thread) int {
		peerReady <- peer
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerReady:
	case <-timer.C:
		t.Fatal("peer coroutine did not start")
	}

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.JoinYieldedOrDone(peer)
		close(callerDone)
		return 0
	})

	timer = time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-callerDone:
	case <-timer.C:
		t.Fatal("caller did not resume after peer returned without yielding")
	}
}

func TestCreateAndStartFromCoroutineYieldsCallerWhenStarted(t *testing.T) {
	co := New(nil)
	co.OnInited()

	peerStarted := make(chan struct{}, 1)
	observed := make(chan bool, 1)
	done := make(chan struct{})

	co.CreateAndStart(true, "caller", func(me Thread) int {
		co.CreateAndStart(true, "peer", func(peer Thread) int {
			close(peerStarted)
			return 0
		})

		select {
		case <-peerStarted:
			observed <- true
		default:
			observed <- false
		}
		close(done)
		return 0
	})

	deadline := time.Now().Add(time.Second)
	for {
		select {
		case <-done:
			select {
			case got := <-observed:
				if !got {
					t.Fatal("CreateAndStart(true) returned before the eagerly started peer ran")
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

		platform.RunOnMainThread(func() {
			co.Update()
		})
		time.Sleep(time.Millisecond)
	}
}
