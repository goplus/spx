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
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/coroutine"
	itime "github.com/goplus/spx/v3/internal/time"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type bindingTestGame struct {
	destroyed atomic.Int32
	reset     atomic.Int32
	paused    atomic.Bool
	ownerSeen atomic.Pointer[struct{}]
	onReset   func()
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
	if g.onReset != nil {
		g.onReset()
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

func bindGameAtPhase(callbacks IGame, owner any, phase gamePhase) (*gameBinding, error) {
	binding, err := beginGame(callbacks, owner)
	if err != nil {
		return nil, err
	}
	binding.storePhase(phase)
	if phase != gameStarting {
		binding.startDone = nil
	}
	return binding, nil
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

func TestInactiveGameRejectsRuntimeWork(t *testing.T) {
	for _, test := range []struct {
		name  string
		phase gamePhase
	}{
		{"starting", gameStarting},
		{"stopped", gameStopped},
		{"closing", gameClosing},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateGameBinding(t)
			co := setupExecuteTest(t)
			game := new(bindingTestGame)
			binding, err := bindGameAtPhase(game, game, test.phase)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { releaseGame(binding) })

			var calls atomic.Int32
			threadID := co.LastThreadID()
			Go(game, func(context.Context, any) { calls.Add(1) })
			Execute(game, func(context.Context, any) { calls.Add(1) })
			onPause(true)
			if calls.Load() != 0 || co.LastThreadID() != threadID || game.paused.Load() {
				t.Fatal("inactive game admitted runtime work or forwarded pause")
			}
		})
	}
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
	if releaseGame(first) {
		t.Fatal("stale release unexpectedly succeeded")
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
	// Teardown notifications may repeat while the backend is still closing.
	onDestroy()
	onReset()
	if game.destroyed.Load() != 1 || game.reset.Load() != 0 {
		t.Fatalf("closing game received duplicate callbacks: destroy=%d reset=%d", game.destroyed.Load(), game.reset.Load())
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
	// Late callbacks must remain harmless after ownership is released.
	onStart()
	onUpdate(0.25)
	onPause(false)
	onReset()
	onDestroy()
	onDestroyed()
	if activeGame.Load() != nil || !game.paused.Load() || game.destroyed.Load() != 1 || game.reset.Load() != 0 {
		t.Fatal("late callbacks changed a released game")
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
	if activateGame(binding, gdx.CoreCallbackInfo{}) {
		t.Fatal("closed startup activated")
	}
	if binding.link != nil {
		t.Fatal("closed startup prepared a backend")
	}
	if got := activeGame.Load(); got != binding {
		t.Fatalf("startup binding released before initialization finished: %p", got)
	}
	finishStart()
	waitForEngineSignal(t, done, time.Second, "destroyed callback did not finish after startup")
	if got := activeGame.Load(); got != nil {
		t.Fatalf("startup binding after backend teardown = %p, want nil", got)
	}
}

func TestResetClearsPendingCapturesBeforeRebinding(t *testing.T) {
	for _, test := range []struct {
		name    string
		cleanup func(*testing.T, *gameBinding)
	}{
		{"reset", func(*testing.T, *gameBinding) { onReset() }},
		{"deferred_reset", func(t *testing.T, binding *gameBinding) {
			resetBinding, started := beginDeferredReset()
			if !started || resetBinding != binding {
				t.Fatal("deferred reset did not retain the current binding")
			}
			onReset()
			releaseGame(resetBinding)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateGameBinding(t)
			isolateCaptureState(t)
			setupExecuteTest(t)
			binding, err := bindGame(new(bindingTestGame), new(struct{}))
			if err != nil {
				t.Fatal(err)
			}

			var got []CaptureRequest
			SetCaptureHandler(func(request CaptureRequest) error {
				got = append(got, request)
				return nil
			})
			if err := EnqueueCapture("old.png", nil); err != nil {
				t.Fatal(err)
			}
			if !HasPendingCaptures() || len(got) != 0 {
				t.Fatal("old game's capture was not queued")
			}

			test.cleanup(t, binding)
			if activeGame.Load() != nil || HasPendingCaptures() {
				t.Fatal("reset retained the game binding or a capture")
			}
			next, err := bindGame(new(bindingTestGame), new(struct{}))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { releaseGame(next) })

			if err := FlushCaptures(); err != nil {
				t.Fatal(err)
			}
			if len(got) != 0 {
				t.Fatalf("capture from previous game dispatched after rebinding: %v", got)
			}
			if err := EnqueueCapture("new.png", nil); err != nil {
				t.Fatal(err)
			}
			if err := FlushCaptures(); err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Name != "new.png" || got[0].Sequence != 1 {
				t.Fatalf("captures after rebinding = %v, want preserved handler receiving new.png with sequence 1", got)
			}
		})
	}
}

func TestBindRejectsNewGameUntilResetCallbackCompletes(t *testing.T) {
	isolateGameBinding(t)
	setupExecuteTest(t)
	resetStarted, finishReset := make(chan struct{}), make(chan struct{})
	finish := sync.OnceFunc(func() { close(finishReset) })
	t.Cleanup(finish)
	game := &bindingTestGame{onReset: func() {
		close(resetStarted)
		<-finishReset
	}}
	first, err := bindGame(game, new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	resetDone := make(chan struct{})
	go func() {
		onReset()
		close(resetDone)
	}()
	waitForEngineSignal(t, resetStarted, time.Second, "reset callback did not start")
	if activeGame.Load() != first {
		t.Fatal("reset released the binding before its callback completed")
	}
	if _, err := bindGame(new(bindingTestGame), new(struct{})); !errors.Is(err, ErrGameAlreadyRunning) {
		t.Fatalf("bind during reset = %v, want %v", err, ErrGameAlreadyRunning)
	}
	finish()
	waitForEngineSignal(t, resetDone, time.Second, "reset did not finish after its callback returned")
	second, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatalf("bind after reset failed: %v", err)
	}
	t.Cleanup(func() { releaseGame(second) })
}

func TestResetClearsInputAndTriggerEvents(t *testing.T) {
	for _, test := range []struct {
		name    string
		cleanup func(*testing.T, *gameBinding)
	}{
		{"reset", func(*testing.T, *gameBinding) { onReset() }},
		{"deferred_reset", func(t *testing.T, binding *gameBinding) {
			resetBinding, started := beginDeferredReset()
			if !started || resetBinding != binding {
				t.Fatal("deferred reset did not retain the current binding")
			}
			onReset()
			releaseGame(resetBinding)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateGameBinding(t)
			setupExecuteTest(t)
			resetInputState()
			resetTriggerEvents()
			t.Cleanup(func() {
				resetInputState()
				resetTriggerEvents()
			})
			binding, err := bindGame(new(bindingTestGame), new(struct{}))
			if err != nil {
				t.Fatal(err)
			}

			// Retain both current-frame events and events awaiting the next frame.
			onKeyPressed(7)
			onMousePressed(1)
			enqueueTriggerEvent(&Sprite{}, &Sprite{})
			cacheKeyEvents()
			cacheMouseEvents()
			cacheTriggerEvents()
			onKeyPressed(8)
			onMousePressed(2)
			enqueueTriggerEvent(&Sprite{}, &Sprite{})

			test.cleanup(t, binding)
			if activeGame.Load() != nil {
				t.Fatal("reset retained the game binding")
			}
			for frame := range 2 {
				keys, held := GetKeyInput(nil)
				mouse, buttons := GetMouseInput(nil)
				triggers := GetTriggerEvents(nil)
				if len(keys) != 0 || len(held) != 0 || len(mouse) != 0 || buttons != 0 || len(triggers) != 0 {
					t.Fatalf("frame %d retained input or triggers: keys=%v held=%v mouse=%v buttons=%d triggers=%v",
						frame, keys, held, mouse, buttons, triggers)
				}
				if IsMouseButtonPressed(1) || IsMouseButtonPressed(2) || AnyMouseButtonPressed() {
					t.Fatal("reset retained held mouse buttons")
				}
				cacheKeyEvents()
				cacheMouseEvents()
				cacheTriggerEvents()
			}

			// A new binding must receive fresh events without an onStart reset.
			next, err := bindGame(new(bindingTestGame), new(struct{}))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { releaseGame(next) })
			source, target := &Sprite{}, &Sprite{}
			onKeyPressed(7)
			onMousePressed(1)
			enqueueTriggerEvent(source, target)
			cacheKeyEvents()
			cacheMouseEvents()
			cacheTriggerEvents()
			keys, held := GetKeyInput(nil)
			mouse, buttons := GetMouseInput(nil)
			triggers := GetTriggerEvents(nil)
			if !slices.Equal(keys, []KeyEvent{{Id: 7, IsPressed: true}}) || !slices.Equal(held, []int64{7}) ||
				!slices.Equal(mouse, []MouseEvent{{Id: 1, IsPressed: true}}) || buttons != 1 ||
				!slices.Equal(triggers, []TriggerEvent{{Src: source, Dst: target}}) {
				t.Fatalf("new game input or triggers: keys=%v held=%v mouse=%v buttons=%d triggers=%v",
					keys, held, mouse, buttons, triggers)
			}
		})
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
	waitForEngineSignal(t, resetDone, time.Second, "reset did not finish after coroutines drained")
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

func TestMainStartsAndReleasesBinding(t *testing.T) {
	isolateGameBinding(t)
	owner := new(struct{})
	game := new(bindingTestGame)
	if err := Main(game, owner, nil); err != nil {
		t.Fatal(err)
	}
	binding := activeGame.Load()
	if binding == nil || binding.callbacks != game || binding.loadPhase() != gameRunning || binding.link == nil {
		t.Fatalf("main binding = %+v, want running binding with a link", binding)
	}
	if got := GetGame(); got != owner {
		t.Fatalf("active owner = %p, want %p", got, owner)
	}
	if !releaseGame(binding) {
		t.Fatal("main binding was not released")
	}
}

func TestResetDuringStartPreventsActivationAndDefersRelease(t *testing.T) {
	isolateGameBinding(t)
	var binding *gameBinding
	released := make(chan struct{})
	if err := Main(new(bindingTestGame), new(struct{}), func() {
		binding = activeGame.Load()
		resetBinding, started := beginDeferredReset()
		if !started || resetBinding != binding {
			t.Fatalf("deferred reset = (%p, %v), want (%p, true)", resetBinding, started, binding)
		}
		go func() {
			finishWebReset(nil, resetBinding, 0)
			close(released)
		}()
		select {
		case <-released:
			t.Fatal("reset released the binding before startup finished")
		case <-time.After(25 * time.Millisecond):
		}
	}); err != nil {
		t.Fatal(err)
	}
	if binding.link != nil {
		t.Fatal("a closing startup prepared the engine link")
	}
	waitForEngineSignal(t, released, time.Second, "reset did not release the binding after startup finished")
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

	releaseGame(sentinel)
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
	Go("queued-before-close", func(context.Context, any) { calls.Add(1) })
	if !beginGameClose(binding) {
		t.Fatal("failed to close the active binding")
	}
	releaseScheduler()
	co.RunAfterStop(nil)
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
	Go("queued-for-first", func(context.Context, any) { calls.Add(1) })
	releaseGame(first)
	second, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { releaseGame(second) })

	releaseScheduler()
	co.RunAfterStop(nil)
	if got := calls.Load(); got != 0 {
		t.Fatalf("stale work ran in the replacement game: calls = %d", got)
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
	Go("destroy-go", func(context.Context, any) { calls.Add(1) })
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

func TestDestroyWaitsPastFormerDrainTimeout(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	game := new(bindingTestGame)
	binding, err := bindGame(game, new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	t.Cleanup(finish)
	co.Create("slow-destroy", func(coroutine.Thread) {
		close(started)
		<-release
	})
	<-started
	done := make(chan struct{})
	go func() {
		// Native shutdown tears down the backend as soon as onDestroy returns.
		onDestroy()
		if game.destroyed.Load() != 1 {
			t.Error("backend teardown preceded the destroy hook")
		}
		onDestroyed()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("destroy returned before the blocked script drained")
	case <-time.After(2200 * time.Millisecond):
	}
	if activeGame.Load() != binding || game.destroyed.Load() != 0 {
		t.Fatal("destroy released the binding or ran its callback before scripts drained")
	}
	finish()
	waitForEngineSignal(t, done, time.Second, "destroy did not finish after the script drained")
	if activeGame.Load() != nil {
		t.Fatal("destroy did not release the binding")
	}
}

type frameRecordingGame struct {
	bindingTestGame
	step   func(string)
	deltas []float64
}

func (g *frameRecordingGame) record(name string, delta float64) {
	g.deltas = append(g.deltas, delta)
	g.step(name)
}
func (g *frameRecordingGame) OnEngineBeforeUpdate(delta float64) { g.record("before", delta) }
func (g *frameRecordingGame) OnEngineUpdate(delta float64)       { g.record("update", delta) }
func (g *frameRecordingGame) OnEngineRender(delta float64)       { g.record("render", delta) }
func (g *frameRecordingGame) OnEngineFrameEnd()                  { g.step("end") }

func TestFrameRuntimeUpdatesPrecedeScriptsAndCapture(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "running", true: "stopped"}[stop], func(t *testing.T) {
			isolateGameBinding(t)
			co := setupExecuteTest(t)
			isolateCaptureState(t)
			resetRuntimeDeltaTestState()
			t.Cleanup(func() {
				co.RunAfterStop(nil)
				resetRuntimeDeltaTestState()
			})
			var steps []string
			var scriptValue atomic.Int32
			game := &frameRecordingGame{step: func(phase string) {
				steps = append(steps, phase)
				if phase == "before" && itime.Frame() != 0 {
					t.Error("frame clock advanced before readback")
				}
				if phase == "render" && scriptValue.Load() != 42 {
					t.Error("committed before runtime-created script completed its slice")
				}
			}}
			binding, err := bindGame(game, game)
			if err != nil {
				t.Fatal(err)
			}
			SetCaptureHandler(func(CaptureRequest) error {
				steps = append(steps, "capture")
				return nil
			})
			sprite := &frameCallbackSprite{}
			sprite.Id = 1
			state.sprites[sprite.Id] = sprite
			sprite.onPosition = func(mathf.Vec2) {
				steps = append(steps, "runtime")
				if itime.Frame() != 1 {
					t.Error("runtime update did not see the advanced clock")
				}
				if stop {
					binding.storePhase(gameStopped)
					return
				}
				Go(nil, func(context.Context, any) {
					scriptValue.Store(42)
					if err := EnqueueCapture("runtime", nil); err != nil {
						t.Error(err)
					}
					WaitNextFrame()
					scriptValue.Store(84)
				})
			}
			tweenPos(sprite, mathf.NewVec2(20, 30), 0.25, func() {})
			onUpdate(0.25)
			want := []string{"before", "runtime", "update", "render", "capture", "end"}
			if stop {
				want = want[:2]
			}
			if !slices.Equal(steps, want) {
				t.Fatalf("frame phases=%v, want %v", steps, want)
			}
		})
	}
}

func TestFrameStopBoundaries(t *testing.T) {
	for _, stopAfter := range []string{"", "before", "update", "render", "capture"} {
		t.Run("stop_after_"+stopAfter, func(t *testing.T) {
			isolateGameBinding(t)
			setupExecuteTest(t)
			resetInputState()
			resetTriggerEvents()
			itime.Start(nil)
			t.Cleanup(func() { resetInputState(); resetTriggerEvents(); itime.Start(nil) })
			isolateCaptureState(t)
			var steps []string
			var binding *gameBinding
			step := func(name string) {
				steps = append(steps, name)
				if name == stopAfter {
					binding.storePhase(gameStopped)
				}
			}
			game := &frameRecordingGame{step: step}
			var err error
			binding, err = bindGame(game, game)
			if err != nil {
				t.Fatal(err)
			}
			SetCaptureHandler(func(request CaptureRequest) error {
				if request.Name != "frame.png" || request.Sequence != 1 {
					t.Errorf("capture=%+v", request)
				}
				step("capture")
				return nil
			})
			if err := EnqueueCapture("frame.png", nil); err != nil {
				t.Fatal(err)
			}
			onUpdate(0.25)
			want := []string{"before", "update", "render", "capture", "end"}
			if stopAfter == "before" || stopAfter == "update" {
				want = want[:slices.Index(want, stopAfter)+1]
			}
			if !slices.Equal(steps, want) {
				t.Fatalf("frame steps=%v, want %v", steps, want)
			}
			if wantDeltas := ([]float64{0.25, 0.25, 0.25})[:min(3, len(want))]; !slices.Equal(game.deltas, wantDeltas) {
				t.Errorf("frame deltas=%v, want %v", game.deltas, wantDeltas)
			}
			wantPending := stopAfter == "before" || stopAfter == "update"
			if HasPendingCaptures() != wantPending {
				t.Errorf("pending capture=%v, want %v", HasPendingCaptures(), wantPending)
			}
			if got := itime.Frame(); got != 1 {
				t.Errorf("frame=%d, want 1", got)
			}
			if stopAfter != "" {
				onUpdate(0.25)
				if !slices.Equal(steps, want) {
					t.Fatal("stopped game received another frame")
				}
			}
		})
	}
}
