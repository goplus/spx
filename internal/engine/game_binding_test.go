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
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	itime "github.com/goplus/spx/v3/internal/time"
)

type bindingTestGame struct {
	destroyed atomic.Int32
	reset     atomic.Int32
	paused    atomic.Bool
	ownerSeen atomic.Pointer[struct{}]
}

func (*bindingTestGame) OnEngineStart()               {}
func (*bindingTestGame) OnEngineUpdate(float64)       {}
func (*bindingTestGame) OnEngineBeforeUpdate(float64) {}
func (*bindingTestGame) OnEngineRender(float64)       {}
func (*bindingTestGame) OnEngineFrameEnd()            {}
func (g *bindingTestGame) OnEngineDestroy() {
	g.destroyed.Add(1)
	if owner, ok := GetGame().(*struct{}); ok {
		g.ownerSeen.Store(owner)
	}
}
func (g *bindingTestGame) OnEngineReset() {
	g.reset.Add(1)
	if owner, ok := GetGame().(*struct{}); ok {
		g.ownerSeen.Store(owner)
	}
}
func (g *bindingTestGame) OnEnginePause(paused bool) {
	g.paused.Store(paused)
}

func isolateGameBinding(t *testing.T) {
	t.Helper()
	previous := activeGame.Swap(nil)
	t.Cleanup(func() { activeGame.Store(previous) })
}

func bindGame(callbacks IGame, owner any) (*gameBinding, error) {
	return bindGameAtPhase(callbacks, owner, gameRunning)
}

func releaseGame(binding *gameBinding) {
	releaseGameAfter(binding, nil)
}

func TestBindGameRejectsSecondGameWithoutReplacingFirst(t *testing.T) {
	isolateGameBinding(t)
	firstGame, secondGame := new(bindingTestGame), new(bindingTestGame)
	firstOwner, secondOwner := new(struct{}), new(struct{})
	first, err := bindGame(firstGame, firstOwner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bindGame(secondGame, secondOwner); !errors.Is(err, ErrGameAlreadyRunning) {
		t.Fatalf("second bind error = %v, want %v", err, ErrGameAlreadyRunning)
	}
	if got := activeGame.Load(); got != first {
		t.Fatalf("active binding = %p, want first binding %p", got, first)
	}
	if got := GetGame(); got != firstOwner {
		t.Fatalf("active owner = %p, want %p", got, firstOwner)
	}
}

func TestSetGameCannotReplaceRuntimeBinding(t *testing.T) {
	isolateGameBinding(t)
	first, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("SetGame did not reject replacing a runtime binding")
		}
		if got := activeGame.Load(); got != first {
			t.Fatalf("SetGame replaced runtime binding %p with %p", first, got)
		}
	}()
	SetGame(new(struct{}))
}

func TestBindGameHasSingleConcurrentWinner(t *testing.T) {
	isolateGameBinding(t)
	const attempts = 32
	var (
		start   sync.WaitGroup
		finish  sync.WaitGroup
		winners atomic.Int32
	)
	start.Add(1)
	finish.Add(attempts)
	for range attempts {
		go func() {
			defer finish.Done()
			start.Wait()
			if _, err := bindGame(new(bindingTestGame), new(struct{})); err == nil {
				winners.Add(1)
			} else if !errors.Is(err, ErrGameAlreadyRunning) {
				t.Errorf("bind error = %v", err)
			}
		}()
	}
	start.Done()
	finish.Wait()
	if got := winners.Load(); got != 1 {
		t.Fatalf("successful binds = %d, want 1", got)
	}
}

func TestReleaseGameDoesNotClearNewerBinding(t *testing.T) {
	isolateGameBinding(t)
	first, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	releaseGame(first)
	second, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	teardownCalled := false
	if releaseGameAfter(first, func() { teardownCalled = true }) {
		t.Fatal("stale release unexpectedly succeeded")
	}
	if teardownCalled {
		t.Fatal("stale release ran teardown for the newer binding")
	}
	if got := activeGame.Load(); got != second {
		t.Fatalf("stale release cleared binding %p; want %p", got, second)
	}
}

func TestDestroyRoutesCallbackBeforeReleasingBinding(t *testing.T) {
	isolateGameBinding(t)
	game, owner := new(bindingTestGame), new(struct{})
	if _, err := bindGame(game, owner); err != nil {
		t.Fatal(err)
	}
	onPause(true)
	onDestroy()
	if got := GetGame(); got != owner {
		t.Fatalf("active owner before backend teardown = %p, want %p", got, owner)
	}
	onDestroyed()
	if !game.paused.Load() || game.destroyed.Load() != 1 {
		t.Fatalf("pause=%v destroyed=%d, want true/1", game.paused.Load(), game.destroyed.Load())
	}
	if got := game.ownerSeen.Load(); got != owner {
		t.Fatalf("destroy callback owner = %p, want %p", got, owner)
	}
	if got := GetGame(); got != nil {
		t.Fatalf("active owner after destroy = %v, want nil", got)
	}
}

func TestDestroyBeforeAwakeReleasesBinding(t *testing.T) {
	isolateGameBinding(t)
	game := new(bindingTestGame)
	binding, err := bindGame(game, new(struct{}))
	if err != nil {
		t.Fatal(err)
	}

	onDestroy()
	onDestroyed()
	if got := activeGame.Load(); got != nil {
		t.Fatalf("binding after pre-awake shutdown = %p, want nil", got)
	}
	if got := game.destroyed.Load(); got != 1 {
		t.Fatalf("destroy callbacks before awake = %d, want 1", got)
	}
	if _, err := bindGame(new(bindingTestGame), new(struct{})); err != nil {
		t.Fatalf("bind after pre-awake shutdown: %v", err)
	}
	releaseGame(activeGame.Load())
	if got := binding.loadPhase(); got != gameClosing {
		t.Fatalf("pre-awake binding phase = %d, want closing", got)
	}
}

func TestDestroyDuringStartPreventsActivation(t *testing.T) {
	isolateGameBinding(t)
	binding, err := bindGameAtPhase(new(bindingTestGame), new(struct{}), gameStarting)
	if err != nil {
		t.Fatal(err)
	}
	finishStart := sync.OnceFunc(func() { close(binding.startDone) })
	t.Cleanup(finishStart)

	done := make(chan struct{})
	go func() {
		onDestroy()
		onDestroyed()
		close(done)
	}()
	deadline := time.Now().Add(time.Second)
	for binding.loadPhase() != gameClosing && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if got := binding.loadPhase(); got != gameClosing {
		t.Fatalf("phase during pre-awake shutdown = %d, want closing", got)
	}
	if activateGame(binding, func() { t.Error("closed startup prepared a backend") }) {
		t.Fatal("closed startup activated")
	}
	if got := activeGame.Load(); got != binding {
		t.Fatalf("startup binding released before initialization finished: %p", got)
	}
	finishStart()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("destroyed callback did not finish after startup")
	}
	if got := activeGame.Load(); got != nil {
		t.Fatalf("startup binding after backend teardown = %p, want nil", got)
	}
}

func TestDestroyedWaitsForDestroyCleanup(t *testing.T) {
	isolateGameBinding(t)
	binding, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	if !beginGameClose(binding) {
		t.Fatal("destroy did not claim the running binding")
	}

	onDestroyed()
	if got := activeGame.Load(); got != binding {
		t.Fatalf("backend teardown released binding before destroy cleanup: %p", got)
	}
	binding.destroyReady.Store(true)
	if !releaseDestroyedGame(binding) {
		t.Fatal("completed destroy cleanup did not release binding")
	}
}

func TestBackendTeardownCannotReleaseBeforeCoroutinesDrain(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	binding, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	t.Cleanup(finish)
	blocker := co.CreateAndStart("destroy-drain", func(coroutine.Thread) int {
		close(started)
		<-release
		return 0
	})
	<-started

	done := make(chan struct{})
	go func() {
		onDestroy()
		close(done)
	}()
	deadline := time.Now().Add(time.Second)
	for !blocker.Stopped() && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if !blocker.Stopped() {
		t.Fatal("destroy callback did not cancel the old coroutine")
	}
	onDestroyed()
	if got := activeGame.Load(); got != binding {
		t.Fatalf("binding released before the old coroutine drained: %p", got)
	}

	finish()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("destroy callback did not finish after coroutine drain")
	}
	if got := activeGame.Load(); got != nil {
		t.Fatalf("binding after destroy and backend teardown = %p, want nil", got)
	}
}

func TestDestroyClearsProcessWideFrameWork(t *testing.T) {
	isolateGameBinding(t)
	setupExecuteTest(t)
	_, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}

	ResetFrameRuntime()
	defer ResetFrameRuntime()
	ran := false
	ScheduleFrame(CurrentFrame()+1, func() { ran = true })

	onDestroy()
	onDestroyed()
	if got := activeGame.Load(); got != nil {
		t.Fatalf("binding after destroy = %p, want nil", got)
	}
	if _, err := bindGame(new(bindingTestGame), new(struct{})); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if current := activeGame.Load(); current != nil {
			releaseGame(current)
		}
	}()

	itime.Update(0, 0)
	RunFrameCallbacks()
	if ran {
		t.Fatal("frame callback from destroyed game ran after rebinding")
	}
}

func TestBindWaitsUntilTeardownCompletes(t *testing.T) {
	isolateGameBinding(t)
	first, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}

	teardownStarted := make(chan struct{})
	finishTeardown := make(chan struct{})
	released := make(chan bool, 1)
	go func() {
		released <- releaseGameAfter(first, func() {
			close(teardownStarted)
			<-finishTeardown
		})
	}()
	<-teardownStarted

	secondBound := make(chan error, 1)
	go func() {
		_, err := bindGame(new(bindingTestGame), new(struct{}))
		secondBound <- err
	}()
	select {
	case err := <-secondBound:
		t.Fatalf("second bind completed before teardown: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	close(finishTeardown)
	if !<-released {
		t.Fatal("release after teardown failed")
	}
	if err := <-secondBound; err != nil {
		t.Fatalf("bind after teardown failed: %v", err)
	}
}

func TestResetRoutesCallbackBeforeReleasingBinding(t *testing.T) {
	isolateGameBinding(t)
	game, owner := new(bindingTestGame), new(struct{})
	if _, err := bindGame(game, owner); err != nil {
		t.Fatal(err)
	}
	onReset()
	if game.reset.Load() != 1 {
		t.Fatalf("reset callbacks = %d, want 1", game.reset.Load())
	}
	if got := game.ownerSeen.Load(); got != owner {
		t.Fatalf("reset callback owner = %p, want %p", got, owner)
	}
	if got := GetGame(); got != nil {
		t.Fatalf("active owner after reset = %v, want nil", got)
	}
	if _, err := bindGame(new(bindingTestGame), new(struct{})); err != nil {
		t.Fatalf("bind after reset failed: %v", err)
	}
}

func TestDirectResetDrainsCoroutinesBeforeReleasingBinding(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	game := new(bindingTestGame)
	binding, err := bindGame(game, new(struct{}))
	if err != nil {
		t.Fatal(err)
	}

	releaseWorker := blockExecuteScheduler(t, co)
	resetDone := make(chan struct{})
	go func() {
		onReset()
		close(resetDone)
	}()

	deadline := time.Now().Add(time.Second)
	for binding.loadPhase() != gameClosing && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if got := binding.loadPhase(); got != gameClosing {
		t.Fatalf("binding phase during reset = %d, want closing", got)
	}
	if got := game.reset.Load(); got != 0 {
		t.Fatalf("reset callback ran before coroutine drain: calls = %d", got)
	}
	if got := activeGame.Load(); got != binding {
		t.Fatalf("binding released before coroutine drain: %p", got)
	}

	releaseWorker()
	select {
	case <-resetDone:
	case <-time.After(time.Second):
		t.Fatal("reset did not finish after coroutines drained")
	}
	if got := game.reset.Load(); got != 1 {
		t.Fatalf("reset callbacks = %d, want 1", got)
	}
	if got := GetGame(); got != nil {
		t.Fatalf("active owner after reset = %v, want nil", got)
	}
}

func TestMainReleasesBindingWhenInitializePanics(t *testing.T) {
	isolateGameBinding(t)
	owner := new(struct{})
	defer func() {
		if recovered := recover(); recovered != "initialize failed" {
			t.Fatalf("recovered = %v, want initialize failed", recovered)
		}
		if got := GetGame(); got != nil {
			t.Fatalf("active owner after initialize panic = %v, want nil", got)
		}
	}()
	_ = Main(new(bindingTestGame), owner, func() {
		if got := GetGame(); got != owner {
			t.Fatalf("active owner during initialize = %p, want %p", got, owner)
		}
		panic("initialize failed")
	})
}

func TestMainRejectsSecondGameBeforeInitialize(t *testing.T) {
	isolateGameBinding(t)
	first, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { releaseGame(first) })

	initialized := false
	err = Main(new(bindingTestGame), new(struct{}), func() {
		initialized = true
	})
	if !errors.Is(err, ErrGameAlreadyRunning) {
		t.Fatalf("Main error = %v, want %v", err, ErrGameAlreadyRunning)
	}
	if initialized {
		t.Fatal("rejected game was initialized")
	}
}

func TestResetDuringStartPreventsActivationAndDefersRelease(t *testing.T) {
	isolateGameBinding(t)
	binding, err := bindGameAtPhase(new(bindingTestGame), new(struct{}), gameStarting)
	if err != nil {
		t.Fatal(err)
	}
	finishStart := sync.OnceFunc(func() { close(binding.startDone) })
	t.Cleanup(finishStart)

	resetBinding, started := beginDeferredReset()
	if !started || resetBinding != binding {
		t.Fatalf("deferred reset = (%p, %v), want (%p, true)", resetBinding, started, binding)
	}
	released := make(chan struct{})
	go func() {
		finishDeferredReset(resetBinding)
		close(released)
	}()
	select {
	case <-released:
		t.Fatal("reset released the binding before startup finished")
	case <-time.After(25 * time.Millisecond):
	}

	prepared := false
	if activateGame(binding, func() { prepared = true }) {
		t.Fatal("a closing startup was activated")
	}
	if prepared {
		t.Fatal("a closing startup prepared the engine link")
	}
	finishStart()
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("reset did not release the binding after startup finished")
	}
	if got := GetGame(); got != nil {
		t.Fatalf("active owner after canceled startup = %v, want nil", got)
	}
}

func TestDeferredResetSentinelBlocksBindUntilResetFinishes(t *testing.T) {
	isolateGameBinding(t)
	sentinel, started := beginDeferredReset()
	if !started {
		t.Fatal("deferred reset did not install a sentinel")
	}
	if _, err := bindGame(new(bindingTestGame), new(struct{})); !errors.Is(err, ErrGameAlreadyRunning) {
		t.Fatalf("bind during reset error = %v, want %v", err, ErrGameAlreadyRunning)
	}

	finishDeferredReset(sentinel)
	binding, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatalf("bind after reset failed: %v", err)
	}
	releaseGame(binding)
}

func TestQueuedRuntimeWorkRechecksPhaseBeforeRunning(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	binding, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { releaseGame(binding) })

	releaseScheduler := blockExecuteScheduler(t, co)
	var calls atomic.Int32
	Go("queued-before-close", func(context.Context) { calls.Add(1) })
	if !beginGameClose(binding) {
		t.Fatal("failed to close the active binding")
	}
	releaseScheduler()
	if !co.RunAfterStopAll(time.Second, nil) {
		t.Fatal("queued runtime work did not drain")
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("queued work ran after closing: calls = %d", got)
	}
}

func TestQueuedRuntimeWorkCannotRunInReplacementGame(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	first, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}

	releaseScheduler := blockExecuteScheduler(t, co)
	var calls atomic.Int32
	Go("queued-for-first", func(context.Context) { calls.Add(1) })
	releaseGame(first)
	second, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { releaseGame(second) })

	releaseScheduler()
	if !co.RunAfterStopAll(time.Second, nil) {
		t.Fatal("queued runtime work did not drain")
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("stale work ran in the replacement game: calls = %d", got)
	}
}

func TestReloadGatesRuntimeWorkAndRestarts(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	game := new(bindingTestGame)
	binding, err := bindGame(game, new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { releaseGame(binding) })

	var calls atomic.Int32
	checkGated := func(phase gamePhase) {
		t.Helper()
		if got := binding.loadPhase(); got != phase {
			t.Fatalf("reload phase = %d, want %d", got, phase)
		}
		threadID := co.LastThreadID()
		Go("reload-go", func(context.Context) { calls.Add(1) })
		Execute("reload-execute", func(context.Context, any) { calls.Add(1) })
		onPause(true)
		if calls.Load() != 0 || co.LastThreadID() != threadID || game.paused.Load() {
			t.Fatal("reload admitted runtime work before activation finished")
		}
	}
	var activated coroutine.Thread
	err = Reload(GetGame(), time.Second, func() error {
		checkGated(gameReloading)
		return nil
	}, func() error {
		checkGated(gameReloading)
		return nil
	}, func() {
		checkGated(gameReloading)
		activated = co.CreateAndStart("new-loop", func(coroutine.Thread) int {
			calls.Add(1)
			return 0
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	co.Join(activated)
	Execute("after-reload", func(context.Context, any) { calls.Add(1) })
	onPause(true)
	if calls.Load() != 2 || !game.paused.Load() || binding.loadPhase() != gameRunning {
		t.Fatalf("runtime after reload: calls=%d, paused=%v, phase=%d", calls.Load(), game.paused.Load(), binding.loadPhase())
	}
}

func TestReloadPrepareFailurePreservesRunningPhase(t *testing.T) {
	failure := errors.New("prepare failure")
	for _, panics := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "panic"}[panics], func(t *testing.T) {
			isolateGameBinding(t)
			binding, err := bindGame(new(bindingTestGame), new(struct{}))
			if err != nil {
				t.Fatal(err)
			}
			func() {
				defer func() {
					if got := recover(); panics && got != failure || !panics && got != nil {
						t.Fatalf("prepare panic = %v", got)
					}
				}()
				err = Reload(GetGame(), time.Second, func() error {
					if panics {
						panic(failure)
					}
					return failure
				}, func() error {
					t.Error("failed preparation ran rebuild")
					return nil
				}, nil)
			}()
			if !panics && !errors.Is(err, failure) {
				t.Fatalf("reload error = %v, want %v", err, failure)
			}
			if got := binding.loadPhase(); got != gameRunning {
				t.Fatalf("phase after failed preparation = %d, want running", got)
			}
		})
	}
}

func TestReloadWaitsForInFlightUpdate(t *testing.T) {
	isolateGameBinding(t)
	binding, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { releaseGame(binding) })

	updateMu.Lock()
	unlock := sync.OnceFunc(updateMu.Unlock)
	t.Cleanup(unlock)
	prepared := make(chan struct{})
	canceled := errors.New("cancel preparation")
	reloadDone := make(chan error, 1)
	go func() {
		reloadDone <- Reload(GetGame(), time.Second, func() error {
			close(prepared)
			return canceled
		}, nil, nil)
	}()
	select {
	case <-prepared:
		t.Fatal("reload prepared before the in-flight update finished")
	case <-time.After(25 * time.Millisecond):
	}
	unlock()
	select {
	case err := <-reloadDone:
		if !errors.Is(err, canceled) {
			t.Fatalf("reload error = %v, want %v", err, canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("reload did not proceed after the update finished")
	}
}

func TestReloadRejectsUnavailableRuntime(t *testing.T) {
	isolateGameBinding(t)
	owner := new(int)
	binding, err := bindGame(new(bindingTestGame), owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { releaseGame(binding) })
	t.Cleanup(func() { updateBusy.Store(false) })

	for _, activeFrame := range []bool{false, true} {
		updateBusy.Store(activeFrame)
		requestedOwner := owner
		if !activeFrame {
			requestedOwner = new(int)
		}
		err := Reload(requestedOwner, time.Second, func() error {
			t.Error("rejected reload ran preparation")
			return nil
		}, nil, nil)
		if !errors.Is(err, ErrReloadUnavailable) || binding.loadPhase() != gameRunning {
			t.Fatalf("rejected reload: error=%v, phase=%d", err, binding.loadPhase())
		}
	}
}

func TestReloadDefersResetUntilRebuildReturns(t *testing.T) {
	isolateGameBinding(t)
	setupExecuteTest(t)
	game := new(bindingTestGame)
	binding, err := bindGame(game, new(struct{}))
	if err != nil {
		t.Fatal(err)
	}

	rebuildEntered := make(chan struct{})
	releaseRebuild := make(chan struct{})
	finishRebuild := sync.OnceFunc(func() { close(releaseRebuild) })
	t.Cleanup(finishRebuild)
	reloadDone := make(chan error, 1)
	var activated atomic.Bool
	go func() {
		reloadDone <- Reload(GetGame(), time.Second, nil, func() error {
			close(rebuildEntered)
			<-releaseRebuild
			return nil
		}, func() { activated.Store(true) })
	}()
	<-rebuildEntered

	resetDone := make(chan struct{})
	go func() {
		onReset()
		close(resetDone)
	}()
	deadline := time.Now().Add(time.Second)
	for binding.loadPhase() != gameClosing && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if binding.loadPhase() != gameClosing || game.reset.Load() != 0 {
		t.Fatal("reset did not defer cleanup until rebuild finished")
	}
	finishRebuild()
	if err := <-reloadDone; !errors.Is(err, ErrReloadUnavailable) {
		t.Fatalf("reload error = %v, want %v", err, ErrReloadUnavailable)
	}
	select {
	case <-resetDone:
	case <-time.After(time.Second):
		t.Fatal("reset cleanup did not run after rebuild")
	}
	if activated.Load() || game.reset.Load() != 1 || activeGame.Load() != nil {
		t.Fatal("reload activated a closing game or reset did not release it")
	}
}

func TestReloadRejectsReplacementBindingWithSameOwner(t *testing.T) {
	isolateGameBinding(t)
	owner := new(struct{})
	first, err := bindGame(new(bindingTestGame), owner)
	if err != nil {
		t.Fatal(err)
	}
	var second *gameBinding
	err = Reload(owner, time.Second, func() error {
		releaseGame(first)
		var err error
		second, err = bindGame(new(bindingTestGame), owner)
		return err
	}, func() error {
		t.Error("stale reload ran rebuild")
		return nil
	}, func() { t.Error("stale reload ran activation") })
	if !errors.Is(err, ErrReloadUnavailable) {
		t.Fatalf("stale reload error = %v, want %v", err, ErrReloadUnavailable)
	}
	if second == nil || activeGame.Load() != second || second.loadPhase() != gameRunning {
		t.Fatal("stale reload changed the replacement binding")
	}
}

func TestReloadDrainTimeoutRemainsStoppedUntilReset(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	game := new(bindingTestGame)
	binding, err := bindGame(game, new(struct{}))
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	t.Cleanup(finish)
	blocker := co.CreateAndStart("reload-timeout", func(coroutine.Thread) int {
		close(started)
		<-release
		return 0
	})
	<-started
	err = Reload(GetGame(), 20*time.Millisecond, nil, func() error {
		t.Error("timed-out reload ran rebuild")
		return nil
	}, func() { t.Error("timed-out reload ran activation") })
	if !errors.Is(err, ErrReloadTimeout) || binding.loadPhase() != gameStopped {
		t.Fatalf("timed-out reload: error=%v, phase=%d", err, binding.loadPhase())
	}
	finish()
	co.Join(blocker)
	if got := binding.loadPhase(); got != gameStopped {
		t.Fatalf("phase after coroutine exit = %d, want stopped", got)
	}
	if err := Reload(GetGame(), time.Second, nil, nil, nil); !errors.Is(err, ErrReloadUnavailable) {
		t.Fatalf("reload of stopped game = %v, want unavailable", err)
	}
	onReset()
	if activeGame.Load() != nil || game.reset.Load() != 1 {
		t.Fatal("reset did not clean up the stopped reload")
	}
}

func TestReloadFailureKeepsGameStopped(t *testing.T) {
	failure := errors.New("reload failure")
	for _, stage := range []string{"rebuild error", "rebuild panic", "activation panic"} {
		t.Run(stage, func(t *testing.T) {
			isolateGameBinding(t)
			setupExecuteTest(t)
			binding, err := bindGame(new(bindingTestGame), new(struct{}))
			if err != nil {
				t.Fatal(err)
			}
			func() {
				defer func() {
					if got := recover(); stage != "rebuild error" && got != failure || stage == "rebuild error" && got != nil {
						t.Fatalf("reload panic = %v", got)
					}
				}()
				err = Reload(GetGame(), time.Second, nil, func() error {
					switch stage {
					case "rebuild error":
						return failure
					case "rebuild panic":
						panic(failure)
					}
					return nil
				}, func() {
					if stage != "activation panic" {
						t.Error("failed rebuild ran activation")
					}
					panic(failure)
				})
			}()
			if stage == "rebuild error" && !errors.Is(err, failure) {
				t.Fatalf("reload error = %v, want %v", err, failure)
			}
			if binding.loadPhase() != gameStopped {
				t.Fatalf("phase after reload failure = %d, want stopped", binding.loadPhase())
			}
			onReset()
			if activeGame.Load() != nil {
				t.Fatal("reset did not release the failed reload")
			}
		})
	}
}

func TestDestroyRejectsRuntimeWorkUntilBackendTeardown(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	game := new(bindingTestGame)
	owner := new(struct{})
	if _, err := bindGame(game, owner); err != nil {
		t.Fatal(err)
	}

	onDestroy()
	if got := GetGame(); got != owner {
		t.Fatalf("destroy released the binding before backend teardown: %p", got)
	}
	threadID := co.LastThreadID()
	var calls atomic.Int32
	Go("destroy-go", func(context.Context) { calls.Add(1) })
	Execute("destroy-execute", func(context.Context, any) { calls.Add(1) })
	if got := calls.Load(); got != 0 {
		t.Fatalf("runtime work ran during destroy: calls = %d", got)
	}
	if got := co.LastThreadID(); got != threadID {
		t.Fatalf("destroy admitted a coroutine: last thread ID = %d, want %d", got, threadID)
	}

	onDestroyed()
	if got := GetGame(); got != nil {
		t.Fatalf("binding after backend teardown = %v, want nil", got)
	}
}
