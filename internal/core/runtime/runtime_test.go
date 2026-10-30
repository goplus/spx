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

package runtime

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/input"
)

func TestInputFrameStateProcess(t *testing.T) {
	var (
		downs []mathf.Vec2
		keys  []int64
	)
	var state InputFrameState

	state.Process(
		InputFrame{
			Point:             mathf.Vec2{X: 10, Y: 20},
			LeftButtonPressed: true,
			KeyEvents: []input.KeyEvent{
				{Id: 1, IsPressed: true},
				{Id: 2, IsPressed: false},
			},
		},
		InputFrameHooks{
			FireLeftButtonDown: func(pos mathf.Vec2) { downs = append(downs, pos) },
			FireLeftButtonUp:   func(mathf.Vec2) {},
			OnKeyPressed:       func(key int64) { keys = append(keys, key) },
		},
	)

	if len(downs) != 1 || downs[0].X != 10 || downs[0].Y != 20 {
		t.Fatalf("unexpected button downs: %+v", downs)
	}
	if len(keys) != 1 || keys[0] != 1 {
		t.Fatalf("unexpected key presses: %+v", keys)
	}
	if !state.leftButtonPressed {
		t.Fatal("expected left button state to update")
	}
}

func TestInputFrameStateProcessPreservesPressAndReleaseWithinOneTick(t *testing.T) {
	var edges []string
	var state InputFrameState
	state.Process(
		InputFrame{
			Point:             mathf.Vec2{X: 7, Y: 8},
			LeftButtonPressed: false,
			MouseEvents: []input.MouseEvent{
				{Id: 1, IsPressed: true},
				{Id: 1, IsPressed: false},
			},
		},
		InputFrameHooks{
			FireLeftButtonDown: func(mathf.Vec2) { edges = append(edges, "down") },
			FireLeftButtonUp:   func(mathf.Vec2) { edges = append(edges, "up") },
			OnKeyPressed:       func(int64) {},
		},
	)
	if !reflect.DeepEqual(edges, []string{"down", "up"}) {
		t.Fatalf("mouse edges = %v, want [down up]", edges)
	}
	if state.leftButtonPressed {
		t.Fatal("short click left button remained pressed")
	}
}

func TestInputFrameStateProcessIgnoresUnrelatedEdgesAndReconcilesRelease(t *testing.T) {
	var state InputFrameState
	state.Reset(true)
	var ups int
	state.Process(
		InputFrame{
			Point:             mathf.Vec2{X: 4, Y: 5},
			LeftButtonPressed: false,
			MouseEvents: []input.MouseEvent{
				{Id: 2, IsPressed: false},
				{Id: 1, IsPressed: true},
			},
		},
		InputFrameHooks{
			FireLeftButtonDown: func(mathf.Vec2) { t.Error("unchanged left-button edge fired a down callback") },
			FireLeftButtonUp:   func(mathf.Vec2) { ups++ },
			OnKeyPressed:       func(int64) {},
		},
	)
	if ups != 1 || state.leftButtonPressed {
		t.Fatalf("reconciled release count/state = %d/%v, want 1/false", ups, state.leftButtonPressed)
	}
}

func TestInputFrameStateResetReplacesSessionState(t *testing.T) {
	var state InputFrameState
	state.Reset(true)
	if !state.leftButtonPressed {
		t.Fatalf("reset state = %+v, want pressed", state)
	}
}

func TestMainExecutionTimedOut(t *testing.T) {
	started := time.Unix(10, 0)
	for _, tt := range []struct {
		name    string
		started time.Time
		elapsed time.Duration
		want    bool
	}{
		{"outside Main", time.Time{}, 10 * time.Second, false},
		{"before deadline", started, 5*time.Second - time.Nanosecond, false},
		{"at deadline", started, 5 * time.Second, true},
		{"after deadline", started, 10 * time.Second, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := MainExecutionTimedOut(tt.started, started.Add(tt.elapsed), 5*time.Second)
			if got != tt.want {
				t.Fatalf("MainExecutionTimedOut = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepeatAndWaitUntil(t *testing.T) {
	var (
		repeatCalls int
		waitCalls   int
	)
	Repeat(3, func() {
		repeatCalls++
	}, func() {
		waitCalls++
	})
	if repeatCalls != 3 || waitCalls != 3 {
		t.Fatalf("repeatCalls=%d waitCalls=%d, want 3/3", repeatCalls, waitCalls)
	}

	condCalls := 0
	bodyCalls := 0
	waitCalls = 0
	RepeatUntil(
		func() bool {
			condCalls++
			return condCalls > 2
		},
		func() {
			bodyCalls++
		},
		func() {
			waitCalls++
		},
	)
	if bodyCalls != 2 || waitCalls != 2 {
		t.Fatalf("RepeatUntil body=%d wait=%d, want 2/2", bodyCalls, waitCalls)
	}

	condCalls = 0
	waitCalls = 0
	WaitUntil(
		func() bool {
			condCalls++
			return condCalls > 2
		},
		func() {
			waitCalls++
		},
	)
	if waitCalls != 2 {
		t.Fatalf("WaitUntil waitCalls=%d, want 2", waitCalls)
	}
}

func TestScheduleHelpersIgnoreNilAndEmptyCallbacks(t *testing.T) {
	called := 0
	yielded := 0
	Forever(nil, func() { yielded++ })
	Repeat(3, nil, func() { yielded++ })
	Repeat(-1, func() { called++ }, func() { yielded++ })
	RepeatUntil(nil, func() { called++ }, func() { yielded++ })
	RepeatUntil(func() bool { return false }, nil, func() { yielded++ })
	WaitUntil(nil, func() { yielded++ })
	if called != 0 || yielded != 0 {
		t.Fatalf("nil/empty callbacks ran: called=%d yielded=%d", called, yielded)
	}
}

func TestForeverRunsUntilYieldStops(t *testing.T) {
	type stopForever struct{}
	calls, yields := 0, 0
	completed := make(chan any, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { completed <- recover() }()
		Forever(func() { calls++ }, func() {
			yields++
			panic(stopForever{})
		})
	}()
	select {
	case recovered := <-completed:
		if recovered != (stopForever{}) {
			t.Fatalf("Forever panic = %v, want stop sentinel", recovered)
		}
	case <-time.After(time.Second):
		t.Fatal("Forever did not return after yield")
	}
	<-done
	if calls != 1 || yields != 1 {
		t.Fatalf("Forever iterations = calls %d, yields %d; want 1/1", calls, yields)
	}
}

func TestSwipeStateFinishReturnsGestureAndTarget(t *testing.T) {
	for _, target := range []string{"sprite", ""} {
		t.Run(target, func(t *testing.T) {
			now := time.Unix(0, 0)
			var state SwipeState[string]
			state.InitWithClock(func() time.Time { return now })
			start, end := mathf.Vec2{X: 10, Y: 20}, mathf.Vec2{X: 110, Y: 20}
			state.Begin(start, target)
			now = now.Add(100 * time.Millisecond)
			state.Expire()
			result, gotTarget, ok := state.Finish(end)
			if !ok || gotTarget != target || result.Direction != 90 || result.Distance != 100 ||
				math.Abs(result.Velocity-1000) > 1e-9 || result.StartPos != start || result.EndPos != end {
				t.Fatalf("Finish = (%+v, %q, %v), want complete gesture for %q", result, gotTarget, ok, target)
			}
			if state.target != "" || state.recognizer.IsTracking() {
				t.Fatal("successful finish retained target or tracking")
			}
		})
	}
}

func TestSwipeStateFinishClearsTargetWhenSwipeFails(t *testing.T) {
	for _, tt := range []struct {
		name    string
		elapsed time.Duration
		point   mathf.Vec2
	}{
		{"short", 100 * time.Millisecond, mathf.Vec2{X: 10}},
		{"zero elapsed", 0, mathf.Vec2{X: 100}},
		{"negative elapsed", -100 * time.Millisecond, mathf.Vec2{X: 100}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Unix(0, 0)
			var state SwipeState[string]
			state.InitWithClock(func() time.Time { return now })
			state.Begin(mathf.Vec2{}, "sprite")
			now = now.Add(tt.elapsed)
			if _, _, ok := state.Finish(tt.point); ok {
				t.Fatal("invalid gesture succeeded")
			}
			if state.target != "" || state.recognizer.IsTracking() {
				t.Fatal("failed finish retained target or tracking")
			}
		})
	}
}

func TestSwipeStateUntrackedFinishDoesNotReadClock(t *testing.T) {
	var state SwipeState[string]
	state.InitWithClock(func() time.Time { panic("untracked gesture read the clock") })
	state.Expire()
	if _, _, ok := state.Finish(mathf.Vec2{X: 100}); ok {
		t.Fatal("untracked gesture succeeded")
	}
}

func TestSwipeStateInitWithClockResetsState(t *testing.T) {
	var state SwipeState[string]
	state.InitWithClock(func() time.Time { return time.Unix(0, 0) })
	state.Begin(mathf.Vec2{}, "stale")
	state.InitWithClock(nil)
	if state.target != "" || state.recognizer.IsTracking() {
		t.Fatal("reinitialization retained target or tracking")
	}
}

func TestSwipeStateExpiresTargetBeforeFinish(t *testing.T) {
	now := time.Unix(0, 0)
	var state SwipeState[string]
	state.InitWithClock(func() time.Time { return now })
	state.Begin(mathf.Vec2{}, "sprite")
	now = now.Add(600 * time.Millisecond)
	state.Expire()
	if state.target != "" || state.recognizer.IsTracking() {
		t.Fatal("expired gesture did not clear the target and tracking")
	}

	// A later clock correction cannot revive an expired gesture.
	now = now.Add(-500 * time.Millisecond)
	if _, _, ok := state.Finish(mathf.Vec2{X: 100}); ok {
		t.Fatal("expired gesture revived after clock correction")
	}
}
