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
)

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
	err = Reload(testContext(t), GetGame(), func() error {
		checkGated(gameReloading)
		return nil
	}, func() error {
		checkGated(gameReloading)
		return nil
	}, func() {
		checkGated(gameReloading)
		activated = co.Create("new-loop", func(coroutine.Thread) {
			calls.Add(1)
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
				err = Reload(testContext(t), GetGame(), func() error {
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
		reloadDone <- Reload(testContext(t), GetGame(), func() error {
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
		err := Reload(testContext(t), requestedOwner, func() error {
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
		reloadDone <- Reload(testContext(t), GetGame(), nil, func() error {
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
	err = Reload(testContext(t), owner, func() error {
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
	blocker := co.Create("reload-timeout", func(coroutine.Thread) {
		close(started)
		<-release
	})
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err = Reload(ctx, GetGame(), nil, func() error {
		t.Error("timed-out reload ran rebuild")
		return nil
	}, func() { t.Error("timed-out reload ran activation") })
	if !errors.Is(err, ErrReloadIncomplete) || !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrReloadUnavailable) || binding.loadPhase() != gameStopped {
		t.Fatalf("timed-out reload: error=%v, phase=%d", err, binding.loadPhase())
	}
	finish()
	co.Join(blocker)
	if got := binding.loadPhase(); got != gameStopped {
		t.Fatalf("phase after coroutine exit = %d, want stopped", got)
	}
	if err := Reload(testContext(t), GetGame(), nil, nil, nil); !errors.Is(err, ErrReloadUnavailable) {
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
				err = Reload(testContext(t), GetGame(), nil, func() error {
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

func TestReloadDeadlineDuringPreparation(t *testing.T) {
	isolateGameBinding(t)
	setupExecuteTest(t)
	binding, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err = Reload(ctx, GetGame(), func() error {
		<-ctx.Done()
		return nil
	}, func() error {
		t.Error("reload rebuilt after preparation exhausted its deadline")
		return nil
	}, func() { t.Error("reload activated after preparation exhausted its deadline") })
	if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrReloadIncomplete) || errors.Is(err, ErrReloadUnavailable) || binding.loadPhase() != gameRunning {
		t.Fatalf("reload after slow preparation: error=%v, phase=%d", err, binding.loadPhase())
	}
}

func TestReloadCanceledBeforeStop(t *testing.T) {
	for _, stage := range []string{"entry", "preparation"} {
		t.Run(stage, func(t *testing.T) {
			isolateGameBinding(t)
			co := setupExecuteTest(t)
			binding, err := bindGame(new(bindingTestGame), new(struct{}))
			if err != nil {
				t.Fatal(err)
			}

			started, release := make(chan struct{}), make(chan struct{})
			finish := sync.OnceFunc(func() { close(release) })
			t.Cleanup(finish)
			thread := co.Create("unchanged", func(coroutine.Thread) {
				close(started)
				<-release
			})
			<-started
			waitCtx, cancelWait := context.WithCancel(context.Background())
			defer cancelWait()
			if stage == "entry" {
				cancelWait()
			}
			err = Reload(waitCtx, GetGame(), func() error {
				if stage == "entry" {
					t.Error("pre-canceled reload ran preparation")
				}
				cancelWait()
				return nil
			}, func() error {
				t.Error("canceled reload rebuilt the game")
				return nil
			}, func() { t.Error("canceled reload activated the game") })
			if !errors.Is(err, context.Canceled) || errors.Is(err, ErrReloadIncomplete) || errors.Is(err, ErrReloadUnavailable) {
				t.Fatalf("reload error = %v, want cancellation before stopping", err)
			}
			if binding.loadPhase() != gameRunning || thread.Stopped() {
				t.Fatalf("canceled reload changed running game: phase=%d, stopped=%v", binding.loadPhase(), thread.Stopped())
			}
			finish()
			co.Join(thread)
		})
	}
}

func TestReloadChecksCancellationAfterLifecycleLock(t *testing.T) {
	for _, stage := range []string{"binding", "update"} {
		t.Run(stage, func(t *testing.T) {
			isolateGameBinding(t)
			co := setupExecuteTest(t)
			owner := new(struct{})
			binding, err := bindGame(new(bindingTestGame), owner)
			if err != nil {
				t.Fatal(err)
			}

			started, release := make(chan struct{}), make(chan struct{})
			finish := sync.OnceFunc(func() { close(release) })
			t.Cleanup(finish)
			thread := co.Create("unchanged", func(coroutine.Thread) {
				close(started)
				<-release
			})
			<-started

			barrier := &bindingMu
			if stage == "update" {
				barrier = &updateMu
			} else {
				// Binding cancellation must not wait for the frame barrier.
				updateMu.Lock()
				t.Cleanup(updateMu.Unlock)
			}
			barrier.Lock()
			unlock := sync.OnceFunc(barrier.Unlock)
			t.Cleanup(unlock)
			deadline := testContext(t)
			waitCtx, cancelWait := context.WithCancel(deadline)
			defer cancelWait()
			ctx := &observedErrContext{Context: waitCtx, checked: make(chan struct{})}
			var calls int
			done := make(chan error, 1)
			go func() {
				done <- Reload(ctx, owner, func() error {
					calls++
					return nil
				}, func() error {
					calls++
					return nil
				}, func() { calls++ })
			}()
			select {
			case <-ctx.checked:
			case <-deadline.Done():
				t.Fatal("reload did not start")
			}
			for stage == "update" && binding.loadPhase() != gameReloading {
				select {
				case <-deadline.Done():
					t.Fatal("reload did not reach the update barrier")
				default:
					runtime.Gosched()
				}
			}
			cancelWait()
			// Cancellation is checked after acquiring the lock; it cannot interrupt Lock.
			unlock()
			select {
			case err = <-done:
			case <-deadline.Done():
				t.Fatal("canceled reload did not return after the lifecycle lock was released")
			}
			if !errors.Is(err, context.Canceled) || errors.Is(err, ErrReloadIncomplete) || errors.Is(err, ErrReloadUnavailable) {
				t.Fatalf("reload error = %v, want cancellation before stopping", err)
			}
			if calls != 0 || binding.loadPhase() != gameRunning || thread.Stopped() {
				t.Fatalf("canceled reload changed running game: calls=%d, phase=%d, stopped=%v", calls, binding.loadPhase(), thread.Stopped())
			}
			finish()
			co.Join(thread)
		})
	}
}

func TestReloadCanceledAtBarrier(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	binding, err := bindGame(new(bindingTestGame), new(struct{}))
	if err != nil {
		t.Fatal(err)
	}

	entered, release := make(chan struct{}), make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	t.Cleanup(finish)
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- co.RunAfterStop(context.Background(), func() {
			close(entered)
			<-release
		})
	}()
	<-entered

	waitCtx, cancelWait := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelWait()
	err = Reload(waitCtx, GetGame(), nil, func() error {
		t.Error("reload rebuilt before acquiring the stop barrier")
		return nil
	}, func() { t.Error("reload activated before acquiring the stop barrier") })
	if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrReloadUnavailable) || errors.Is(err, ErrReloadIncomplete) {
		t.Fatalf("reload error = %v, want deadline exceeded before stopping", err)
	}
	if binding.loadPhase() != gameRunning {
		t.Fatalf("phase after unselected reload = %d, want running", binding.loadPhase())
	}
	finish()
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := Reload(context.Background(), GetGame(), nil, nil, nil); err != nil {
		t.Fatalf("retry after unselected reload: %v", err)
	}
}

func TestReloadCanceledAfterStop(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	game := new(bindingTestGame)
	binding, err := bindGame(game, new(struct{}))
	if err != nil {
		t.Fatal(err)
	}

	started, stopped, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	t.Cleanup(finish)
	thread := co.Create("reload-context-blocker", func(thread coroutine.Thread) {
		close(started)
		<-thread.Context().Done()
		close(stopped)
		<-release
	})
	<-started
	waitCtx, cancelWait := context.WithCancel(context.Background())
	defer cancelWait()
	done := make(chan error, 1)
	go func() {
		done <- Reload(waitCtx, GetGame(), nil, func() error {
			t.Error("canceled drain rebuilt the game")
			return nil
		}, func() { t.Error("canceled drain activated the game") })
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("reload did not cancel its existing script")
	}
	cancelWait()
	select {
	case err = <-done:
	case <-time.After(time.Second):
		t.Fatal("reload did not observe wait cancellation")
	}
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrReloadIncomplete) || errors.Is(err, ErrReloadUnavailable) {
		t.Fatalf("reload error = %v, want incomplete cleanup and cancellation", err)
	}
	if binding.loadPhase() != gameStopped {
		t.Fatalf("phase after canceled drain = %d, want stopped", binding.loadPhase())
	}
	rejected := co.Create("rejected-after-reload-cancel", func(coroutine.Thread) {
		t.Error("canceled drain reopened coroutine admission")
	})
	if !rejected.Stopped() {
		t.Fatal("canceled drain admitted a new coroutine")
	}
	finish()
	co.Join(thread)
	co.Join(rejected)
	if err := Reload(context.Background(), GetGame(), nil, nil, nil); !errors.Is(err, ErrReloadUnavailable) {
		t.Fatalf("reload of stopped game = %v, want unavailable", err)
	}
	onReset()
	if activeGame.Load() != nil || game.reset.Load() != 1 {
		t.Fatal("reset did not clean up the canceled reload")
	}
}

func TestReloadCancellationDuringRebuild(t *testing.T) {
	for _, scheduler := range []bool{false, true} {
		t.Run(map[bool]string{false: "without_scheduler", true: "with_scheduler"}[scheduler], func(t *testing.T) {
			isolateGameBinding(t)
			if scheduler {
				setupExecuteTest(t)
			} else {
				previous := gco
				SetCoroutines(nil)
				t.Cleanup(func() { SetCoroutines(previous) })
			}
			binding, err := bindGame(new(bindingTestGame), new(struct{}))
			if err != nil {
				t.Fatal(err)
			}
			waitCtx, cancelWait := context.WithCancel(context.Background())
			defer cancelWait()
			activated := false
			err = Reload(waitCtx, GetGame(), nil, func() error {
				cancelWait()
				return nil
			}, func() { activated = true })
			if err != nil || !activated || binding.loadPhase() != gameRunning {
				t.Fatalf("reload after rebuild canceled wait: error=%v, activated=%v, phase=%d", err, activated, binding.loadPhase())
			}
		})
	}
}

type observedErrContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *observedErrContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() { close(c.checked) })
	return err
}
