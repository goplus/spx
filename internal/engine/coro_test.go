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
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/goplus/spx/v4/internal/coroutine"
	itime "github.com/goplus/spx/v4/internal/time"
)

func TestResolveOwnerPreservesNilCoroutineOwner(t *testing.T) {
	co := coroutine.New(nil)
	previousCo, previousGame := gco, GetGame()
	SetCoroutines(co)
	SetGame(&struct{}{})
	t.Cleanup(func() {
		SetCoroutines(previousCo)
		SetGame(previousGame)
	})

	resolved := make(chan any, 1)
	thread := co.Create(nil, func(coroutine.Thread) {
		resolved <- resolveOwner(nil)
	})
	co.Join(thread)
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
			co := setupExecuteTest(t)
			SetGame("game")
			done := make(chan struct{})
			start := func() {
				Go(tc.owner, func(ctx context.Context, owner any) {
					defer close(done)
					thread := currentThread()
					if thread == nil || thread.Owner != tc.want || owner != tc.want {
						t.Errorf("callback owner = %v, thread = %v, want owner %v", owner, thread, tc.want)
					} else if ctx != thread.Context() || ctx.Err() != nil {
						t.Error("callback did not receive its live coroutine context")
					}
				})
			}
			if tc.fromThread {
				co.Join(co.Create(tc.parent, func(coroutine.Thread) { start() }))
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

func TestWarpRestoresStateAfterPanic(t *testing.T) {
	for _, initial := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[initial], func(t *testing.T) {
			co := setupExecuteTest(t)
			if IsRunWithoutScreenRefresh() || SetRunWithoutScreenRefresh(true) || !ShouldWaitNextFrame() {
				t.Fatal("warp mode affected a caller outside its coroutine")
			}
			thread := co.Create("warp", func(coroutine.Thread) {
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
			co.Join(thread)
		})
	}
}

func TestControlFlowWaiterFollowsWarpMode(t *testing.T) {
	co := setupExecuteTest(t)
	warpDone, done := make(chan struct{}), make(chan struct{})
	thread := co.Create("loop", func(coroutine.Thread) {
		wait := NewControlFlowWaiter()
		SetRunWithoutScreenRefresh(true)
		wait()
		close(warpDone)
		SetRunWithoutScreenRefresh(false)
		wait()
		close(done)
	})
	co.JoinFirstSlice(thread)
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
	itime.Update(0, 0)
	co.Update()
	co.Join(thread)
	select {
	case <-done:
	default:
		t.Fatal("normal loop did not resume in the next frame")
	}
}

func TestWaitYieldResumesWithoutAdvancingFrame(t *testing.T) {
	co := setupExecuteTest(t)
	frame := itime.Frame()
	done := make(chan struct{})
	thread := co.Create("yield", func(coroutine.Thread) {
		WaitYield()
		close(done)
	})
	co.JoinFirstSlice(thread)
	select {
	case <-done:
		t.Fatal("WaitYield returned before the scheduler resumed the coroutine")
	default:
	}

	co.Update()
	select {
	case <-done:
	case <-testContext(t).Done():
		t.Fatal("yielding coroutine did not resume")
	}
	co.Join(thread)
	if got := itime.Frame(); got != frame {
		t.Fatalf("frame after WaitYield = %d, want unchanged frame %d", got, frame)
	}
}

func TestWarpWaiterYieldsAfterBudget(t *testing.T) {
	defer itime.Start(nil)
	synctest.Test(t, func(t *testing.T) {
		itime.Start(nil)
		co := setupExecuteTest(t)
		done := make(chan struct{})
		thread := co.Create("warp", func(coroutine.Thread) {
			SetRunWithoutScreenRefresh(true)
			wait := NewControlFlowWaiter()
			wait()
			time.Sleep(warpBudget + time.Nanosecond)
			wait()
			close(done)
		})
		co.JoinFirstSlice(thread)
		co.Update()
		select {
		case <-done:
			t.Fatal("exhausted warp budget resumed in the same frame")
		default:
		}
		itime.Update(0, 0)
		co.Update()
		co.Join(thread)
		select {
		case <-done:
		default:
			t.Fatal("exhausted warp budget did not resume in the next frame")
		}
	})
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
	co := coroutine.New(nil)
	original := gco
	SetCoroutines(co)
	t.Cleanup(func() {
		SetCoroutines(original)
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

	th := co.Create(owner, func(me coroutine.Thread) {
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
		co.StopIf(func(candidate coroutine.Thread) bool {
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

func TestWaitAdaptersResumeAfterFrameAndPreserveElapsedTime(t *testing.T) {
	co := setupExecuteTest(t)
	itime.Start(nil)
	t.Cleanup(func() { itime.Start(nil) })
	elapsed := make(chan float64, 3)
	thread := co.Create("waits", func(coroutine.Thread) {
		elapsed <- Wait(.1)
		elapsed <- WaitNextFrame()
		SetRunWithoutScreenRefresh(true)
		elapsed <- WaitNextFrameIfNeeded()
	})
	co.JoinFirstSlice(thread)
	itime.Update(.1, 60)
	co.Update()
	co.JoinFirstSlice(thread)
	select {
	case got := <-elapsed:
		if got != .1 {
			t.Fatalf("Wait elapsed=%g", got)
		}
	default:
		t.Fatal("elapsed wait did not resume")
	}
	itime.Update(.25, 60)
	co.Update()
	co.Join(thread)
	for range 2 {
		if got := <-elapsed; got != .25 {
			t.Fatalf("frame delta=%g", got)
		}
	}
}

func TestControlFlowOptionalCallsAndReadyChannel(t *testing.T) {
	co := setupExecuteTest(t)
	RunWithoutScreenRefresh(nil)
	RunStopThisScript(nil)
	ch := make(chan int, 1)
	ch <- 42
	if got := WaitForChan(ch); got != 42 {
		t.Fatalf("channel value=%d", got)
	}
	RequestRedraw()
	SetCoroutines(nil)
	RequestRedraw()
	SetCoroutines(co)
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
