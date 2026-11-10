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

	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine/wrap"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
	"github.com/visualfc/gid"
)

type resetMainThreadPlatform struct {
	gdx.IPlatformMgr
	main uint64
}

type resetDirectPlatform struct {
	gdx.IPlatformMgr
}

type resetRecordingExt struct {
	gdx.IExtMgr
	calls chan int64
}

type resetBlockingExt struct {
	gdx.IExtMgr
	callback func()
	calls    atomic.Int32
	entered  chan int64
	release  chan struct{}
}

func TestResetWebRuntimeReturnsBeforeExternalDrain(t *testing.T) {
	current := coroutine.New(nil)
	recorder := setupWebResetTest(t, current)

	started := make(chan struct{})
	release := make(chan struct{})
	releaseWorker := sync.OnceFunc(func() { close(release) })
	workerDone := make(chan struct{})
	current.Create("blocked", func(coroutine.Thread) {
		defer close(workerDone)
		close(started)
		<-release
	})
	t.Cleanup(releaseWorker)
	waitForEngineSignal(t, started, time.Second, "blocking coroutine did not start")

	returned := make(chan struct{})
	go func() {
		resetWebRuntime(7)
		close(returned)
	}()
	waitForEngineSignal(t, returned, time.Second, "external reset request waited for coroutine drain")
	waitForResetAdmissionClose(t, current)
	select {
	case exitCode := <-recorder.calls:
		t.Fatalf("engine reset %d was requested before coroutine drain", exitCode)
	case <-time.After(50 * time.Millisecond):
	}

	releaseWorker()
	waitForEngineSignal(t, workerDone, time.Second, "blocking coroutine did not finish after release")
	waitForResetAdmissionOpen(t, current)
}

func TestResetWebRuntimeStopsManagedCaller(t *testing.T) {
	current := coroutine.New(nil)
	recorder := setupWebResetTest(t, current)

	entered := make(chan struct{})
	callerDone := make(chan struct{})
	var returned atomic.Bool
	caller := current.Create("caller", func(coroutine.Thread) {
		defer close(callerDone)
		close(entered)
		resetWebRuntime(9)
		returned.Store(true)
	})
	waitForEngineSignal(t, entered, time.Second, "managed caller did not start")
	waitForEngineSignal(t, callerDone, time.Second, "managed reset caller did not stop")
	if returned.Load() {
		t.Fatal("managed reset caller returned normally")
	}
	waitForEngineSignal(t, caller.Context().Done(), time.Second, "managed reset caller context was not canceled")
	select {
	case exitCode := <-recorder.calls:
		if exitCode != 9 {
			t.Fatalf("engine reset exit code = %d, want 9", exitCode)
		}
	case <-time.After(time.Second):
		t.Fatal("engine reset was not requested after managed caller drained")
	}
	waitForResetAdmissionOpen(t, current)
}

func TestResetWebRuntimeCoalescesConcurrentRequests(t *testing.T) {
	isolateGameBinding(t)
	current := coroutine.New(nil)
	setupWebResetTest(t, current)
	game := new(bindingTestGame)
	binding, err := bindGame(game, game)
	if err != nil {
		t.Fatal(err)
	}
	backend := &resetBlockingExt{
		callback: onReset,
		entered:  make(chan int64, 64),
		release:  make(chan struct{}),
	}
	gdx.ExtMgr = backend
	releaseBackend := sync.OnceFunc(func() { close(backend.release) })
	t.Cleanup(releaseBackend)
	workerStarted, workerRelease := make(chan struct{}), make(chan struct{})
	releaseWorker := sync.OnceFunc(func() { close(workerRelease) })
	t.Cleanup(releaseWorker)
	current.Create("blocked during drain", func(coroutine.Thread) {
		close(workerStarted)
		<-workerRelease
	})
	waitForEngineSignal(t, workerStarted, time.Second, "worker did not start")
	requestDuplicates := func() {
		var requests sync.WaitGroup
		for range 32 {
			requests.Go(func() { resetWebRuntime(22) })
		}
		done := make(chan struct{})
		go func() {
			requests.Wait()
			close(done)
		}()
		t.Cleanup(func() {
			releaseWorker()
			releaseBackend()
			waitForEngineSignal(t, done, time.Second, "duplicate reset requests did not finish cleanup")
		})
		waitForEngineSignal(t, done, time.Second, "duplicate reset requests did not return")
	}

	resetWebRuntime(11)
	waitForResetAdmissionClose(t, current)
	requestDuplicates()
	if backend.calls.Load() != 0 || game.reset.Load() != 0 {
		t.Fatal("reset ran before coroutine drain")
	}
	if activeGame.Load() != binding {
		t.Fatal("binding was released before coroutine drain")
	}
	releaseWorker()
	select {
	case code := <-backend.entered:
		if code != 11 {
			t.Fatalf("reset code = %d, want first request's code 11", code)
		}
	case <-time.After(time.Second):
		t.Fatal("reset did not reach backend")
	}
	requestDuplicates()
	if backend.calls.Load() != 1 || game.reset.Load() != 1 {
		t.Fatalf("reset calls: backend = %d, game = %d; want 1 each", backend.calls.Load(), game.reset.Load())
	}
	if activeGame.Load() != binding {
		t.Fatal("binding was released before backend reset completed")
	}
	if got := binding.loadPhase(); got != gameClosing {
		t.Fatalf("binding phase during reset = %d, want closing", got)
	}
	waitForResetAdmissionClose(t, current)

	releaseBackend()
	deadline := time.Now().Add(time.Second)
	for activeGame.Load() != nil && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if activeGame.Load() != nil {
		t.Fatal("completed reset retained the binding")
	}
	waitForResetAdmissionOpen(t, current)
	if backend.calls.Load() != 1 {
		t.Fatalf("backend reset calls after completion = %d, want 1", backend.calls.Load())
	}
}

func TestResetWebRuntimeCancelsStartupWithoutCallingBackend(t *testing.T) {
	isolateGameBinding(t)
	current := coroutine.New(nil)
	recorder := setupWebResetTest(t, current)

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
	current := coroutine.New(nil)
	setupWebResetTest(t, current)
	gdx.PlatformMgr = resetMainThreadPlatform{main: gid.Get()}
	game := new(bindingTestGame)
	if _, err := bindGame(game, game); err != nil {
		t.Fatal(err)
	}

	peerYielding := make(chan struct{})
	peerDone := make(chan struct{})
	peerLatch := current.NewLatch()
	current.Create("peer", func(coroutine.Thread) {
		defer close(peerDone)
		close(peerYielding)
		peerLatch.Wait()
	})

	waitForEngineSignal(t, peerYielding, time.Second, "peer coroutine did not reach yield")

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
	current.Create("caller", func(coroutine.Thread) {
		defer close(callerDone)
		resetWebRuntime(17)
	})

	deadline := time.Now().Add(time.Second)
	for activeGame.Load() != nil {
		if time.Now().After(deadline) {
			t.Fatal("web reset did not finish and release its binding")
		}
		current.Update()
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
	waitForResetAdmissionOpen(t, current)
}

func (p resetMainThreadPlatform) IsMainThread() bool { return gid.Get() == p.main }

func (resetDirectPlatform) IsMainThread() bool { return true }

func (r *resetBlockingExt) RequestReset(exitCode int64) {
	r.calls.Add(1)
	if r.callback != nil {
		r.callback()
	}
	r.entered <- exitCode
	<-r.release
}

func (r *resetRecordingExt) RequestReset(exitCode int64) {
	r.calls <- exitCode
}

func setupWebResetTest(t *testing.T, current *coroutine.Scheduler) *resetRecordingExt {
	t.Helper()

	original := scheduler
	originalPlatform := gdx.PlatformMgr
	originalExt := gdx.ExtMgr
	recorder := &resetRecordingExt{calls: make(chan int64, 1)}
	SetScheduler(current)
	gdx.PlatformMgr = resetDirectPlatform{}
	gdx.ExtMgr = recorder
	wrap.Init(WaitMainThread)
	t.Cleanup(func() {
		current.RunAfterStop(nil)
		SetScheduler(original)
		gdx.PlatformMgr = originalPlatform
		gdx.ExtMgr = originalExt
	})
	return recorder
}

func waitForResetAdmissionClose(t *testing.T, current *coroutine.Scheduler) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		probe := current.Create("reset-close-probe", func(coroutine.Thread) {})
		if probe.Stopped() {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("reset barrier did not close coroutine admission")
}

func waitForResetAdmissionOpen(t *testing.T, current *coroutine.Scheduler) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		var ran atomic.Bool
		probe := current.Create("reset-open-probe", func(coroutine.Thread) {
			ran.Store(true)
		})
		current.Join(probe)
		if ran.Load() {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("reset barrier did not reopen coroutine admission")
}

func TestDrainAndReleaseGameFinishesCleanupBeforeRelease(t *testing.T) {
	for _, mode := range []string{"return", "panic", "goexit"} {
		for _, nilScheduler := range []bool{false, true} {
			name := mode
			if nilScheduler {
				name += "/nil scheduler"
			}
			t.Run(name, func(t *testing.T) {
				isolateGameBinding(t)
				current := setupExecuteTest(t)
				binding, err := bindGame(new(bindingTestGame), "old")
				if err != nil {
					t.Fatal(err)
				}
				beginGameClose(binding)
				t.Cleanup(func() { current.RunAfterStop(func() { releaseGame(binding) }) })
				var drainScheduler *coroutine.Scheduler
				if !nilScheduler {
					drainScheduler = current
				}
				type result struct {
					panicValue any
					returned   bool
				}
				done := make(chan result, 1)
				go func() {
					returned := false
					defer func() { done <- result{recover(), returned} }()
					drainAndReleaseGame(drainScheduler, binding, func() {
						if activeGame.Load() != binding {
							t.Error("binding released before game cleanup")
						}
						if drainScheduler != nil {
							probe := current.Create("during cleanup", func(coroutine.Thread) { t.Error("cleanup admitted script") })
							if !probe.Stopped() {
								t.Error("admission open inside cleanup")
							}
						}
						switch mode {
						case "panic":
							panic("cleanup failure")
						case "goexit":
							runtime.Goexit()
						}
					})
					returned = true
				}()
				select {
				case got := <-done:
					if mode == "panic" && got.panicValue != "cleanup failure" || mode != "panic" && got.panicValue != nil {
						t.Fatalf("cleanup panic=%v", got.panicValue)
					}
					if got.returned != (mode == "return") {
						t.Fatalf("cleanup returned=%v for %s", got.returned, mode)
					}
				case <-time.After(time.Second):
					t.Fatal("cleanup did not finish")
				}
				if activeGame.Load() != nil {
					t.Fatal("completed cleanup retained the game")
				}
				ran := false
				probe := current.Create("after cleanup", func(coroutine.Thread) { ran = true })
				current.Join(probe)
				if probe.Stopped() || !ran {
					t.Fatal("completed cleanup did not reopen admission")
				}
			})
		}
	}
}
