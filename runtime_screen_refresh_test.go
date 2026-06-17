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

package spx

import (
	"testing"
	"time"

	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
)

func TestRunWithoutScreenRefreshSkipsControlFlowFrameWaits(t *testing.T) {
	co := coroutine.New(nil)
	original := gco
	gco = co
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		gco = original
		engine.SetCoroutines(original)
	})

	type result struct {
		repeatCalls int
		condCalls   int
		flagDuring  bool
		flagAfter   bool
	}

	done := make(chan result, 1)
	th := co.CreateAndStart(true, "run-without-screen-refresh", func(me coroutine.Thread) int {
		flagDuring := false
		repeatCalls := 0
		condCalls := 0
		RunWithoutScreenRefresh(func() {
			flagDuring = IsRunWithoutScreenRefresh()
			Repeat(3, func() {
				repeatCalls++
			})
			WaitUntil(func() bool {
				condCalls++
				return condCalls > 2
			})
		})

		done <- result{
			repeatCalls: repeatCalls,
			condCalls:   condCalls,
			flagDuring:  flagDuring,
			flagAfter:   IsRunWithoutScreenRefresh(),
		}
		return 0
	})
	t.Cleanup(func() {
		co.StopIf(func(candidate coroutine.Thread) bool {
			return candidate == th
		})
	})

	select {
	case got := <-done:
		if !got.flagDuring || got.flagAfter {
			t.Fatalf("flag state = during:%v after:%v, want true/false", got.flagDuring, got.flagAfter)
		}
		if got.repeatCalls != 3 {
			t.Fatalf("repeatCalls = %d, want 3", got.repeatCalls)
		}
		if got.condCalls != 3 {
			t.Fatalf("condCalls = %d, want 3", got.condCalls)
		}
	case <-time.After(time.Second):
		t.Fatal("control-flow wait points should not block when run without screen refresh is enabled")
	}
}

func TestRunWithoutScreenRefreshRestoresPreviousState(t *testing.T) {
	co := coroutine.New(nil)
	original := gco
	gco = co
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		gco = original
		engine.SetCoroutines(original)
	})

	type result struct {
		prev        bool
		outerDuring bool
		callDuring  bool
		callAfter   bool
		restored    bool
	}

	done := make(chan result, 1)
	th := co.CreateAndStart(true, "run-without-screen-refresh-state", func(me coroutine.Thread) int {
		prev := SetRunWithoutScreenRefresh(true)
		outerDuring := IsRunWithoutScreenRefresh()

		callDuring := false
		RunWithoutScreenRefresh(func() {
			callDuring = IsRunWithoutScreenRefresh()
		})

		callAfter := IsRunWithoutScreenRefresh()
		SetRunWithoutScreenRefresh(prev)
		done <- result{
			prev:        prev,
			outerDuring: outerDuring,
			callDuring:  callDuring,
			callAfter:   callAfter,
			restored:    IsRunWithoutScreenRefresh(),
		}
		return 0
	})
	t.Cleanup(func() {
		co.StopIf(func(candidate coroutine.Thread) bool {
			return candidate == th
		})
	})

	select {
	case got := <-done:
		if got.prev {
			t.Fatal("previous run-without-screen-refresh flag should default to false")
		}
		if !got.outerDuring || !got.callDuring || !got.callAfter {
			t.Fatalf("flag state = outer:%v call:%v after:%v, want true/true/true", got.outerDuring, got.callDuring, got.callAfter)
		}
		if got.restored {
			t.Fatal("run-without-screen-refresh flag should restore to false after outer reset")
		}
	case <-time.After(time.Second):
		t.Fatal("run-without-screen-refresh state should restore without blocking")
	}
}

func TestWarpCallsFunctionWithArgs(t *testing.T) {
	co := coroutine.New(nil)
	original := gco
	gco = co
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		gco = original
		engine.SetCoroutines(original)
	})

	type result struct {
		noArgDuring bool
		argDuring   bool
		convDuring  bool
		gotInt      int
		gotString   string
		gotFloat    float64
		flagAfter   bool
	}

	done := make(chan result, 1)
	th := co.CreateAndStart(true, "warp", func(me coroutine.Thread) int {
		var got result
		Warp(func() {
			got.noArgDuring = IsRunWithoutScreenRefresh()
		})
		Warp(func(n int, text string) {
			got.argDuring = IsRunWithoutScreenRefresh()
			got.gotInt = n
			got.gotString = text
		}, 7, "ok")
		Warp(func(n float64) {
			got.convDuring = IsRunWithoutScreenRefresh()
			got.gotFloat = n
		}, 3)
		got.flagAfter = IsRunWithoutScreenRefresh()
		done <- got
		return 0
	})
	t.Cleanup(func() {
		co.StopIf(func(candidate coroutine.Thread) bool {
			return candidate == th
		})
	})

	select {
	case got := <-done:
		if !got.noArgDuring || !got.argDuring || !got.convDuring || got.flagAfter {
			t.Fatalf("flag state = noArg:%v arg:%v conv:%v after:%v, want true/true/true/false", got.noArgDuring, got.argDuring, got.convDuring, got.flagAfter)
		}
		if got.gotInt != 7 || got.gotString != "ok" || got.gotFloat != 3 {
			t.Fatalf("warp args = (%d, %q, %v), want (7, %q, 3)", got.gotInt, got.gotString, got.gotFloat, "ok")
		}
	case <-time.After(time.Second):
		t.Fatal("warp call should finish without blocking")
	}
}
