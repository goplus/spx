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

package engine

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type resetWorkerPlatform struct {
	gdx.IPlatformMgr
}

func (resetWorkerPlatform) IsMainThread() bool { return false }

func TestRequestResetAfterCoroutinesStopWaitsForManagedCaller(t *testing.T) {
	co := coroutine.New(nil)
	co.OnInited()
	original := gco
	SetCoroutines(co)
	t.Cleanup(func() {
		SetCoroutines(original)
	})
	originalPlatform := gdx.PlatformMgr
	gdx.PlatformMgr = resetWorkerPlatform{}
	t.Cleanup(func() { gdx.PlatformMgr = originalPlatform })

	peerYielding := make(chan struct{})
	peerDone := make(chan struct{})
	co.CreateAndStart(true, "peer", func(peer coroutine.Thread) int {
		defer close(peerDone)
		close(peerYielding)
		co.Yield(peer)
		return 0
	})

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-peerYielding:
	case <-timer.C:
		t.Fatal("peer coroutine did not reach yield")
	}

	callerDone := make(chan struct{})
	resetCalled := make(chan struct{})
	result := make(chan bool, 1)
	var resetBeforeDrain atomic.Bool
	co.CreateAndStart(true, "caller", func(me coroutine.Thread) int {
		defer close(callerDone)
		go func() {
			result <- requestResetAfterCoroutinesStop(co, time.Second, func() {
				select {
				case <-callerDone:
				default:
					resetBeforeDrain.Store(true)
				}
				select {
				case <-peerDone:
				default:
					resetBeforeDrain.Store(true)
				}
				close(resetCalled)
			})
		}()
		co.Abort()
		return 0
	})

	deadline := time.Now().Add(time.Second)
	for {
		select {
		case completed := <-result:
			if !completed {
				t.Fatal("coroutine reset barrier timed out")
			}
			if resetBeforeDrain.Load() {
				t.Fatal("engine reset ran before managed callers drained")
			}
			select {
			case <-resetCalled:
			default:
				t.Fatal("reset barrier completed without requesting reset")
			}
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("reset barrier did not reach its main-thread callback")
		}
		co.Update()
		runtime.Gosched()
	}
}

func TestRequestResetAfterCoroutinesStopSkipsResetOnTimeout(t *testing.T) {
	co := coroutine.New(nil)
	co.OnInited()
	original := gco
	SetCoroutines(co)
	t.Cleanup(func() { SetCoroutines(original) })

	started := make(chan struct{})
	release := make(chan struct{})
	thread := co.CreateAndStart(true, "blocked", func(coroutine.Thread) int {
		close(started)
		<-release
		return 0
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("blocking coroutine did not start")
	}

	var resetCalled atomic.Bool
	if requestResetAfterCoroutinesStop(co, 20*time.Millisecond, func() {
		resetCalled.Store(true)
	}) {
		t.Fatal("reset barrier reported success with a blocked coroutine")
	}
	if resetCalled.Load() {
		t.Fatal("engine reset was requested after shutdown timed out")
	}
	close(release)
	select {
	case <-thread.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("blocked coroutine was not canceled")
	}
	if !co.AbortAllAndWait(time.Second) {
		t.Fatal("blocking coroutine did not finish after release")
	}
}
