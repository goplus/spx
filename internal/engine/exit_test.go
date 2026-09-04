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
	"github.com/goplus/spx/v3/internal/enginewrap"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type resetWorkerPlatform struct {
	gdx.IPlatformMgr
}

func (resetWorkerPlatform) IsMainThread() bool { return false }

type resetDirectPlatform struct {
	gdx.IPlatformMgr
}

func (resetDirectPlatform) IsMainThread() bool { return true }

type resetRecordingExt struct {
	gdx.IExtMgr
	calls chan int64
}

func (r *resetRecordingExt) RequestReset(exitCode int64) {
	r.calls <- exitCode
}

func setupAbortCoroutinesAndResetTest(t *testing.T, co *coroutine.Coroutines) *resetRecordingExt {
	t.Helper()

	original := gco
	originalPlatform := gdx.PlatformMgr
	originalExt := gdx.ExtMgr
	recorder := &resetRecordingExt{calls: make(chan int64, 1)}
	SetCoroutines(co)
	gdx.PlatformMgr = resetDirectPlatform{}
	gdx.ExtMgr = recorder
	enginewrap.Init(WaitMainThread)
	t.Cleanup(func() {
		if !co.AbortAllAndWait(time.Second) {
			t.Error("test coroutines did not stop")
		}
		SetCoroutines(original)
		gdx.PlatformMgr = originalPlatform
		gdx.ExtMgr = originalExt
	})
	return recorder
}

func waitForResetAdmissionClose(t *testing.T, co *coroutine.Coroutines) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		probe := co.Create("reset-close-probe", func(coroutine.Thread) int { return 0 })
		if probe.Stopped() {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("reset barrier did not close coroutine admission")
}

func waitForResetAdmissionOpen(t *testing.T, co *coroutine.Coroutines) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		var ran atomic.Bool
		probe := co.CreateAndStart(true, "reset-open-probe", func(coroutine.Thread) int {
			ran.Store(true)
			return 0
		})
		co.Join(probe)
		if ran.Load() {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("reset barrier did not reopen coroutine admission")
}

func TestAbortCoroutinesAndResetReturnsBeforeExternalDrain(t *testing.T) {
	co := coroutine.New(nil)
	co.OnInited()
	recorder := setupAbortCoroutinesAndResetTest(t, co)

	started := make(chan struct{})
	release := make(chan struct{})
	var released atomic.Bool
	workerDone := make(chan struct{})
	co.CreateAndStart(true, "blocked", func(coroutine.Thread) int {
		defer close(workerDone)
		close(started)
		<-release
		return 0
	})
	t.Cleanup(func() {
		if released.CompareAndSwap(false, true) {
			close(release)
		}
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("blocking coroutine did not start")
	}

	returned := make(chan struct{})
	go func() {
		abortCoroutinesAndReset(7)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("external reset request waited for coroutine drain")
	}
	waitForResetAdmissionClose(t, co)
	select {
	case exitCode := <-recorder.calls:
		t.Fatalf("engine reset %d was requested before coroutine drain", exitCode)
	case <-time.After(50 * time.Millisecond):
	}

	if released.CompareAndSwap(false, true) {
		close(release)
	}
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("blocking coroutine did not finish after release")
	}
	waitForResetAdmissionOpen(t, co)
}

func TestAbortCoroutinesAndResetAbortsManagedCaller(t *testing.T) {
	co := coroutine.New(nil)
	co.OnInited()
	recorder := setupAbortCoroutinesAndResetTest(t, co)

	entered := make(chan struct{})
	callerDone := make(chan struct{})
	var returned atomic.Bool
	caller := co.CreateAndStart(true, "caller", func(coroutine.Thread) int {
		defer close(callerDone)
		close(entered)
		abortCoroutinesAndReset(9)
		returned.Store(true)
		return 0
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("managed caller did not start")
	}
	select {
	case <-callerDone:
	case <-time.After(time.Second):
		t.Fatal("managed reset caller was not aborted")
	}
	if returned.Load() {
		t.Fatal("managed reset caller returned normally")
	}
	select {
	case <-caller.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("managed reset caller context was not canceled")
	}
	select {
	case exitCode := <-recorder.calls:
		if exitCode != 9 {
			t.Fatalf("engine reset exit code = %d, want 9", exitCode)
		}
	case <-time.After(time.Second):
		t.Fatal("engine reset was not requested after managed caller drained")
	}
	waitForResetAdmissionOpen(t, co)
}

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
