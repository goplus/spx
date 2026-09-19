//go:build !pure_engine

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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
	"github.com/visualfc/gid"
)

type resetMainThreadPlatform struct {
	gdx.IPlatformMgr
	main uint64
}

func (p resetMainThreadPlatform) IsMainThread() bool { return gid.Get() == p.main }

type resetDirectPlatform struct {
	gdx.IPlatformMgr
}

func (resetDirectPlatform) IsMainThread() bool { return true }

type resetRecordingExt struct {
	gdx.IExtMgr
	calls chan int64
}

type resetBlockingExt struct {
	gdx.IExtMgr
	callback func()
	entered  chan int64
	release  chan struct{}
}

func (r *resetBlockingExt) RequestReset(exitCode int64) {
	if r.callback != nil {
		r.callback()
	}
	r.entered <- exitCode
	<-r.release
}

func (r *resetRecordingExt) RequestReset(exitCode int64) {
	r.calls <- exitCode
}

func setupWebResetTest(t *testing.T, co *coroutine.Coroutines) *resetRecordingExt {
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
		if !co.StopAllAndWait(time.Second) {
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
		probe := co.CreateAndStart("reset-open-probe", func(coroutine.Thread) int {
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

func TestResetWebRuntimeReturnsBeforeExternalDrain(t *testing.T) {
	co := coroutine.New(nil)
	co.OnInited()
	recorder := setupWebResetTest(t, co)

	started := make(chan struct{})
	release := make(chan struct{})
	releaseWorker := sync.OnceFunc(func() { close(release) })
	workerDone := make(chan struct{})
	co.CreateAndStart("blocked", func(coroutine.Thread) int {
		defer close(workerDone)
		close(started)
		<-release
		return 0
	})
	t.Cleanup(releaseWorker)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("blocking coroutine did not start")
	}

	returned := make(chan struct{})
	go func() {
		resetWebRuntime(7)
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

	releaseWorker()
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("blocking coroutine did not finish after release")
	}
	waitForResetAdmissionOpen(t, co)
}

func TestResetWebRuntimeStopsManagedCaller(t *testing.T) {
	co := coroutine.New(nil)
	co.OnInited()
	recorder := setupWebResetTest(t, co)

	entered := make(chan struct{})
	callerDone := make(chan struct{})
	var returned atomic.Bool
	caller := co.CreateAndStart("caller", func(coroutine.Thread) int {
		defer close(callerDone)
		close(entered)
		resetWebRuntime(9)
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
		t.Fatal("managed reset caller did not stop")
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

func TestResetWebRuntimeReleasesBindingAfterDrainReopens(t *testing.T) {
	isolateGameBinding(t)
	co := coroutine.New(nil)
	co.OnInited()
	setupWebResetTest(t, co)

	game, owner := new(bindingTestGame), new(struct{})
	binding, err := bindGame(game, owner)
	if err != nil {
		t.Fatal(err)
	}
	backend := &resetBlockingExt{
		callback: onReset,
		entered:  make(chan int64, 1),
		release:  make(chan struct{}),
	}
	gdx.ExtMgr = backend
	releaseBackend := sync.OnceFunc(func() { close(backend.release) })
	t.Cleanup(releaseBackend)

	resetWebRuntime(11)
	select {
	case code := <-backend.entered:
		if code != 11 {
			t.Fatalf("reset code = %d, want 11", code)
		}
	case <-time.After(time.Second):
		t.Fatal("backend reset was not requested")
	}
	if game.reset.Load() != 1 {
		t.Fatalf("reset callbacks = %d, want 1", game.reset.Load())
	}
	if got := activeGame.Load(); got != binding {
		t.Fatalf("binding was released inside the reset barrier: %p", got)
	}
	if got := binding.loadPhase(); got != gameClosing {
		t.Fatalf("binding phase during reset = %d, want closing", got)
	}
	waitForResetAdmissionClose(t, co)

	releaseBackend()
	deadline := time.Now().Add(time.Second)
	for GetGame() != nil && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if got := GetGame(); got != nil {
		t.Fatalf("binding after reset completion = %v, want nil", got)
	}
	waitForResetAdmissionOpen(t, co)
}

func TestResetWebRuntimeCancelsStartupWithoutCallingBackend(t *testing.T) {
	isolateGameBinding(t)
	co := coroutine.New(nil)
	co.OnInited()
	recorder := setupWebResetTest(t, co)

	binding, err := bindGameAtPhase(new(bindingTestGame), new(struct{}), gameStarting)
	if err != nil {
		t.Fatal(err)
	}
	finishStart := sync.OnceFunc(func() { close(binding.startDone) })
	t.Cleanup(finishStart)

	resetWebRuntime(13)
	select {
	case code := <-recorder.calls:
		t.Fatalf("backend reset %d was requested before startup established a backend", code)
	case <-time.After(25 * time.Millisecond):
	}
	finishStart()
	deadline := time.Now().Add(time.Second)
	for GetGame() != nil && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if got := GetGame(); got != nil {
		t.Fatalf("startup binding after reset = %v, want nil", got)
	}
	select {
	case code := <-recorder.calls:
		t.Fatalf("backend reset %d was requested for a canceled startup", code)
	default:
	}
}

func TestResetWebRuntimeWaitsForManagedCallersOnMainThread(t *testing.T) {
	isolateGameBinding(t)
	co := coroutine.New(nil)
	co.OnInited()
	setupWebResetTest(t, co)
	gdx.PlatformMgr = resetMainThreadPlatform{main: gid.Get()}
	game := new(bindingTestGame)
	if _, err := bindGame(game, game); err != nil {
		t.Fatal(err)
	}

	peerYielding := make(chan struct{})
	peerDone := make(chan struct{})
	co.CreateAndStart("peer", func(peer coroutine.Thread) int {
		defer close(peerDone)
		close(peerYielding)
		co.Yield(peer)
		return 0
	})

	select {
	case <-peerYielding:
	case <-time.After(time.Second):
		t.Fatal("peer coroutine did not reach yield")
	}

	callerDone := make(chan struct{})
	var resetBeforeDrain atomic.Bool
	var resetOffMainThread atomic.Bool
	backend := &resetBlockingExt{
		callback: func() {
			for _, done := range []chan struct{}{callerDone, peerDone} {
				select {
				case <-done:
				default:
					resetBeforeDrain.Store(true)
				}
			}
			if runtime.GOOS != "js" && !gdx.PlatformMgr.IsMainThread() {
				resetOffMainThread.Store(true)
			}
			onReset()
		},
		entered: make(chan int64, 1),
		release: make(chan struct{}),
	}
	close(backend.release)
	gdx.ExtMgr = backend
	co.CreateAndStart("caller", func(coroutine.Thread) int {
		defer close(callerDone)
		resetWebRuntime(17)
		return 0
	})

	deadline := time.Now().Add(time.Second)
	for activeGame.Load() != nil {
		if time.Now().After(deadline) {
			t.Fatal("web reset did not finish and release its binding")
		}
		co.Update()
		runtime.Gosched()
	}
	select {
	case code := <-backend.entered:
		if code != 17 {
			t.Fatalf("reset code = %d, want 17", code)
		}
	default:
		t.Fatal("web reset released its binding without requesting backend reset")
	}
	if resetBeforeDrain.Load() {
		t.Fatal("engine reset ran before managed callers drained")
	}
	if resetOffMainThread.Load() {
		t.Fatal("engine reset ran outside the main thread")
	}
	if got := game.reset.Load(); got != 1 {
		t.Fatalf("reset callbacks = %d, want 1", got)
	}
	waitForResetAdmissionOpen(t, co)
}
