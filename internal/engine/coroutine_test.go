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
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type executeWorkerPlatform struct{ gdx.IPlatformMgr }

func TestResolveOwnerPreservesNilCoroutineOwner(t *testing.T) {
	current := coroutine.New(nil)
	previousScheduler, previousGame := scheduler, GetGame()
	SetScheduler(current)
	SetGame(&struct{}{})
	t.Cleanup(func() {
		SetScheduler(previousScheduler)
		SetGame(previousGame)
	})

	resolved := make(chan any, 1)
	thread := current.Create(nil, func(coroutine.Thread) {
		resolved <- resolveOwner(nil)
	})
	current.Join(thread)
	if owner := <-resolved; owner != nil {
		t.Fatalf("owner = %v, want nil", owner)
	}
}

func TestGoResolvesOwner(t *testing.T) {
	for _, tc := range []struct {
		name       string
		parent     any
		owner      any
		want       any
		fromThread bool
	}{
		{name: "explicit", owner: "explicit", want: "explicit"},
		{name: "game", want: "game"},
		{name: "inherited", parent: "parent", want: "parent", fromThread: true},
		{name: "override", parent: "parent", owner: "explicit", want: "explicit", fromThread: true},
		{name: "nil", fromThread: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateGameBinding(t)
			current := setupExecuteTest(t)
			SetGame("game")
			done := make(chan struct{})
			start := func() {
				Go(tc.owner, func(ctx context.Context, owner any) {
					defer close(done)
					thread := scheduler.Caller()
					if thread == nil || thread.Owner != tc.want || owner != tc.want {
						t.Errorf("callback owner = %v, thread = %v, want owner %v", owner, thread, tc.want)
					} else if ctx != thread.Context() || ctx.Err() != nil {
						t.Error("callback did not receive its live coroutine context")
					}
				})
			}
			if tc.fromThread {
				current.Join(current.Create(tc.parent, func(coroutine.Thread) { start() }))
			} else {
				start()
			}
			select {
			case <-done:
			case <-testContext(t).Done():
				t.Fatal("owned coroutine did not run")
			}
		})
	}
}

func TestExecuteKeepsCurrentCoroutineAndSuppliedOwner(t *testing.T) {
	current := setupExecuteTest(t)
	var called bool
	thread := current.Create("original", func(thread coroutine.Thread) {
		Execute(nil, func(ctx context.Context, owner any) {
			called = true
			if ctx != thread.Context() || current.Current() != thread || owner != nil {
				t.Error("Execute changed the current coroutine, context, or supplied owner")
			}
		})
	})
	current.Join(thread)
	if !called {
		t.Fatal("Execute did not call the callback")
	}
}

func TestExecuteWaitsForCompletionWithDefaultOwner(t *testing.T) {
	setupExecuteTest(t)
	previousGame := GetGame()
	game := &struct{}{}
	SetGame(game)
	t.Cleanup(func() { SetGame(previousGame) })
	started, unblock, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(unblock) })
	t.Cleanup(release)
	var callbackContext context.Context
	go func() {
		Execute(nil, func(ctx context.Context, owner any) {
			callbackContext = ctx
			if !IsInCoroutine() || owner != game || scheduler.Caller().Owner != game {
				t.Error("Execute did not use a managed coroutine owned by the game")
			}
			close(started)
			<-unblock
		})
		close(returned)
	}()
	waitForEngineSignal(t, started, time.Second, "Execute callback did not start")
	select {
	case <-returned:
		t.Fatal("Execute returned while its callback was running")
	default:
	}
	release()
	waitForEngineSignal(t, returned, time.Second, "Execute did not return after its callback completed")
	if callbackContext.Err() == nil {
		t.Fatal("Execute returned before its coroutine completed")
	}
}

func TestExecuteReturnsWhenStoppedBeforeStart(t *testing.T) {
	current := setupExecuteTest(t)
	release := blockExecuteScheduler(t, current)
	returned := make(chan struct{})
	var called atomic.Bool
	go func() {
		Execute("pending", func(context.Context, any) { called.Store(true) })
		close(returned)
	}()

	stopped := false
	deadline := time.Now().Add(time.Second)
	for !stopped {
		current.StopIf(func(thread coroutine.Thread) bool {
			if thread.Owner == "pending" {
				stopped = true
				return true
			}
			return false
		})
		if time.Now().After(deadline) {
			t.Fatal("Execute did not register its coroutine")
		}
		runtime.Gosched()
	}
	release()
	waitForEngineSignal(t, returned, time.Second, "Execute remained blocked after its coroutine stopped before starting")
	if called.Load() {
		t.Fatal("Execute ran a canceled callback")
	}
}

func TestExecuteReturnsWhenAdmissionClosed(t *testing.T) {
	current := setupExecuteTest(t)
	started, canceled, unblock := make(chan struct{}), make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(unblock) })
	t.Cleanup(release)
	current.Create("blocker", func(thread coroutine.Thread) {
		close(started)
		<-thread.Context().Done()
		close(canceled)
		<-unblock
	})
	waitForEngineSignal(t, started, time.Second, "scheduler blocker did not start")
	drained := make(chan struct{})
	go func() {
		current.RunAfterStop(nil)
		close(drained)
	}()
	waitForEngineSignal(t, canceled, time.Second, "shutdown did not cancel the blocker")
	returned := make(chan struct{})
	var called atomic.Bool
	go func() {
		Execute("rejected", func(context.Context, any) { called.Store(true) })
		close(returned)
	}()
	waitForEngineSignal(t, returned, time.Second, "Execute remained blocked with coroutine admission closed")
	if called.Load() {
		t.Fatal("Execute ran a rejected callback")
	}
	select {
	case <-drained:
		t.Fatal("shutdown completed before the blocker exited")
	default:
	}
	release()
	waitForEngineSignal(t, drained, time.Second, "shutdown did not finish after the blocker exited")
}

func TestExecuteNativeRunsInlineOutsideCoroutine(t *testing.T) {
	called := false

	ExecuteNative(func(ctx context.Context, owner any) {
		called = true
		if ctx == nil {
			t.Fatal("ExecuteNative provided nil context")
		}
		if owner != nil {
			t.Fatalf("owner = %v, want nil outside coroutine", owner)
		}
		if IsInCoroutine() {
			t.Fatal("native function should not run in an SPX coroutine outside coroutine context")
		}
	})

	if !called {
		t.Fatal("ExecuteNative did not run the function")
	}
}

func TestExecuteNativeFromCoroutineResumesWhenNativeWorkCompletes(t *testing.T) {
	current := coroutine.New(nil)
	original := scheduler
	SetScheduler(current)
	t.Cleanup(func() {
		SetScheduler(original)
	})

	owner := &struct{ name string }{name: "owner"}
	nativeStarted := make(chan struct{})
	releaseNative := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			close(releaseNative)
		})
	}

	type result struct {
		ctxNil              bool
		ctxCanceled         bool
		ownerMatches        bool
		nativeInCoroutine   bool
		returnedInCoroutine bool
	}
	done := make(chan result, 1)

	th := current.Create(owner, func(me coroutine.Thread) {
		var got result
		ExecuteNative(func(ctx context.Context, gotOwner any) {
			close(nativeStarted)
			<-releaseNative

			got.ctxNil = ctx == nil
			if ctx != nil {
				got.ctxCanceled = ctx.Err() != nil
			}
			got.ownerMatches = gotOwner == owner
			got.nativeInCoroutine = IsInCoroutine()
		})
		got.returnedInCoroutine = IsInCoroutine()
		done <- got
	})
	t.Cleanup(func() {
		release()
		current.StopIf(func(candidate coroutine.Thread) bool {
			return candidate == th
		})
	})

	waitForEngineSignal(t, nativeStarted, time.Second, "native function did not start")

	select {
	case <-done:
		t.Fatal("ExecuteNative returned before native work completed")
	default:
	}

	release()

	select {
	case got := <-done:
		if got.ctxNil {
			t.Fatal("ExecuteNative provided nil coroutine context")
		}
		if got.ctxCanceled {
			t.Fatal("coroutine context should not be canceled while native work runs")
		}
		if !got.ownerMatches {
			t.Fatal("ExecuteNative did not pass the current coroutine owner")
		}
		if got.nativeInCoroutine {
			t.Fatal("native function should run outside the SPX coroutine")
		}
		if !got.returnedInCoroutine {
			t.Fatal("ExecuteNative should resume the caller coroutine")
		}
	case <-time.After(time.Second):
		t.Fatal("ExecuteNative did not resume after native work completed")
	}
}

func TestWaitYieldResumesWithoutAdvancingFrame(t *testing.T) {
	current := setupExecuteTest(t)
	frame := clock.Frame()
	done := make(chan struct{})
	thread := current.Create("yield", func(coroutine.Thread) {
		WaitYield()
		close(done)
	})
	current.JoinFirstSlice(thread)
	select {
	case <-done:
		t.Fatal("WaitYield returned before the scheduler resumed the coroutine")
	default:
	}

	current.Update()
	select {
	case <-done:
	case <-testContext(t).Done():
		t.Fatal("yielding coroutine did not resume")
	}
	current.Join(thread)
	if got := clock.Frame(); got != frame {
		t.Fatalf("frame after WaitYield = %d, want unchanged frame %d", got, frame)
	}
}

func TestWaitAdaptersResumeAfterFrameAndPreserveElapsedTime(t *testing.T) {
	current := setupExecuteTest(t)
	clock.Start()
	t.Cleanup(func() { clock.Start() })
	elapsed := make(chan float64, 3)
	thread := current.Create("waits", func(coroutine.Thread) {
		elapsed <- Wait(.1)
		elapsed <- WaitNextFrame()
		SetRunWithoutScreenRefresh(true)
		elapsed <- WaitNextFrameIfNeeded()
	})
	current.JoinFirstSlice(thread)
	clock.Update(.1, 60)
	current.Update()
	current.JoinFirstSlice(thread)
	select {
	case got := <-elapsed:
		if got != .1 {
			t.Fatalf("Wait elapsed=%g", got)
		}
	default:
		t.Fatal("elapsed wait did not resume")
	}
	clock.Update(.25, 60)
	current.Update()
	current.Join(thread)
	for range 2 {
		if got := <-elapsed; got != .25 {
			t.Fatalf("frame delta=%g", got)
		}
	}
}

func TestWarpRestoresStateAfterPanic(t *testing.T) {
	for _, initial := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[initial], func(t *testing.T) {
			current := setupExecuteTest(t)
			if IsRunWithoutScreenRefresh() || SetRunWithoutScreenRefresh(true) || !ShouldWaitNextFrame() {
				t.Fatal("warp mode affected a caller outside its coroutine")
			}
			thread := current.Create("warp", func(coroutine.Thread) {
				if SetRunWithoutScreenRefresh(initial) {
					t.Error("new coroutine started in warp mode")
				}
				func() {
					defer func() {
						if got := recover(); got != "warp failure" {
							t.Errorf("warp panic = %v, want warp failure", got)
						}
					}()
					RunWithoutScreenRefresh(func() {
						if !IsRunWithoutScreenRefresh() || ShouldWaitNextFrame() {
							t.Error("warp did not suppress the initial frame wait")
						}
						RunWithoutScreenRefresh(func() { panic("warp failure") })
					})
				}()
				if IsRunWithoutScreenRefresh() != initial || SetRunWithoutScreenRefresh(false) != initial || !ShouldWaitNextFrame() {
					t.Error("warp did not restore the previous state after panic")
				}
			})
			current.Join(thread)
		})
	}
}

func TestControlFlowWaiterFollowsWarpMode(t *testing.T) {
	for _, origin := range []string{"managed", "external"} {
		t.Run(origin, func(t *testing.T) {
			current := setupExecuteTest(t)
			var wait func()
			if origin == "external" {
				wait = NewControlFlowWaiter()
			}
			warpDone, done := make(chan struct{}), make(chan struct{})
			thread := current.Create("loop", func(coroutine.Thread) {
				if wait == nil {
					wait = NewControlFlowWaiter()
				}
				SetRunWithoutScreenRefresh(true)
				wait()
				close(warpDone)
				SetRunWithoutScreenRefresh(false)
				wait()
				close(done)
			})
			current.JoinFirstSlice(thread)
			select {
			case <-warpDone:
			default:
				t.Fatal("warp loop yielded before its budget was exhausted")
			}
			select {
			case <-done:
				t.Fatal("normal loop did not yield")
			default:
			}
			clock.Update(0, 0)
			current.Update()
			current.Join(thread)
			select {
			case <-done:
			default:
				t.Fatal("normal loop did not resume in the next frame")
			}
		})
	}
}

func TestWarpWaiterYieldsAfterBudget(t *testing.T) {
	defer clock.Start()
	synctest.Test(t, func(t *testing.T) {
		clock.Start()
		current := setupExecuteTest(t)
		done := make(chan struct{})
		thread := current.Create("warp", func(coroutine.Thread) {
			SetRunWithoutScreenRefresh(true)
			wait := NewControlFlowWaiter()
			wait()
			time.Sleep(warpBudget + time.Nanosecond)
			wait()
			close(done)
		})
		current.JoinFirstSlice(thread)
		current.Update()
		select {
		case <-done:
			t.Fatal("exhausted warp budget resumed in the same frame")
		default:
		}
		clock.Update(0, 0)
		current.Update()
		current.Join(thread)
		select {
		case <-done:
		default:
			t.Fatal("exhausted warp budget did not resume in the next frame")
		}
	})
}

func TestControlFlowOptionalCallsAndReadyChannel(t *testing.T) {
	current := setupExecuteTest(t)
	RunWithoutScreenRefresh(nil)
	RunStopThisScript(nil)
	ch := make(chan int, 1)
	ch <- 42
	if got := WaitForChan(ch); got != 42 {
		t.Fatalf("channel value=%d", got)
	}
	RequestRedraw()
	SetScheduler(nil)
	RequestRedraw()
	SetScheduler(current)
	Lock()
	if logicMu.TryLock() {
		logicMu.Unlock()
		t.Fatal("Lock did not acquire the engine mutex")
	}
	Unlock()
	if !logicMu.TryLock() {
		t.Fatal("Unlock retained the engine mutex")
	}
	logicMu.Unlock()
}

func TestStopThisScriptPreservesOtherPanics(t *testing.T) {
	for _, tc := range []struct {
		name   string
		signal any
		want   any
	}{
		{name: "normal"},
		{name: "stop", signal: coroutine.ErrStopThisScript},
		{name: "abort", signal: coroutine.ErrAbortThread, want: coroutine.ErrAbortThread},
		{name: "panic", signal: "failure", want: "failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got any
			returned := false
			func() {
				defer func() { got = recover() }()
				RunStopThisScript(func() {
					if tc.signal != nil {
						panic(tc.signal)
					}
				})
				returned = true
			}()
			if got != tc.want || returned != (tc.want == nil) {
				t.Fatalf("procedure boundary: panic=%v, returned=%v, want panic %v", got, returned, tc.want)
			}
			if IsAbortThreadError(got) != (tc.want == coroutine.ErrAbortThread) {
				t.Fatalf("abort classification changed for %v", got)
			}
		})
	}
}

func (executeWorkerPlatform) IsMainThread() bool { return false }

func setupExecuteTest(t *testing.T) *coroutine.Scheduler {
	t.Helper()
	current := coroutine.New(nil)
	previous, previousPlatform := scheduler, gdx.PlatformMgr
	SetScheduler(current)
	gdx.PlatformMgr = executeWorkerPlatform{}
	t.Cleanup(func() {
		current.RunAfterStop(nil)
		SetScheduler(previous)
		gdx.PlatformMgr = previousPlatform
	})
	return current
}

func blockExecuteScheduler(t *testing.T, current *coroutine.Scheduler) func() {
	t.Helper()
	started, unblock := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(unblock) })
	t.Cleanup(release)
	current.Create("blocker", func(coroutine.Thread) {
		close(started)
		<-unblock
	})
	waitForEngineSignal(t, started, time.Second, "scheduler blocker did not start")
	return release
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	return ctx
}

func waitForEngineSignal(t *testing.T, done <-chan struct{}, timeout time.Duration, failure string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal(failure)
	}
}
