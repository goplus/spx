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
	"github.com/goplus/spx/v3/internal/engine"
)

func TestInputFrameStateProcess(t *testing.T) {
	var (
		downs []mathf.Vec2
		moves []mathf.Vec2
		keys  []int64
	)
	var state InputFrameState

	state.Process(
		InputFrame{
			Point:             mathf.Vec2{X: 10, Y: 20},
			LeftButtonPressed: true,
			KeyEvents: []engine.KeyEvent{
				{Id: 1, IsPressed: true},
				{Id: 2, IsPressed: false},
			},
			MouseMovementThreshold: 1,
		},
		InputFrameHooks{
			FireLeftButtonDown: func(pos mathf.Vec2) { downs = append(downs, pos) },
			FireLeftButtonUp:   func(mathf.Vec2) {},
			OnMouseMove:        func(pos mathf.Vec2) { moves = append(moves, pos) },
			OnKeyPressed:       func(key int64) { keys = append(keys, key) },
		},
	)

	if len(downs) != 1 || downs[0].X != 10 || downs[0].Y != 20 {
		t.Fatalf("unexpected button downs: %+v", downs)
	}
	if len(moves) != 1 || moves[0].X != 10 || moves[0].Y != 20 {
		t.Fatalf("unexpected mouse moves: %+v", moves)
	}
	if len(keys) != 1 || keys[0] != 1 {
		t.Fatalf("unexpected key presses: %+v", keys)
	}
	if state.lastMousePos.X != 10 || state.lastMousePos.Y != 20 {
		t.Fatalf("lastMousePos = %+v, want {10 20}", state.lastMousePos)
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
			MouseEvents: []engine.MouseEvent{
				{Id: 1, IsPressed: true},
				{Id: 1, IsPressed: false},
			},
		},
		InputFrameHooks{
			FireLeftButtonDown: func(mathf.Vec2) { edges = append(edges, "down") },
			FireLeftButtonUp:   func(mathf.Vec2) { edges = append(edges, "up") },
			OnMouseMove:        func(mathf.Vec2) {},
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

func TestLiveAndSessionInputSamplesAreEquivalent(t *testing.T) {
	type observedEvent struct {
		kind  string
		point mathf.Vec2
		key   int64
	}
	recordingHooks := func(events *[]observedEvent) InputFrameHooks {
		return InputFrameHooks{
			FireLeftButtonDown: func(point mathf.Vec2) {
				*events = append(*events, observedEvent{kind: "down", point: point})
			},
			FireLeftButtonUp: func(point mathf.Vec2) {
				*events = append(*events, observedEvent{kind: "up", point: point})
			},
			OnMouseMove: func(point mathf.Vec2) {
				*events = append(*events, observedEvent{kind: "move", point: point})
			},
			OnKeyPressed: func(key int64) {
				*events = append(*events, observedEvent{kind: "key", key: key})
			},
		}
	}

	frames := []InputFrame{
		{
			Point:                  mathf.Vec2{X: 2, Y: 3},
			LeftButtonPressed:      true,
			MouseEvents:            []engine.MouseEvent{{Id: 1, IsPressed: true}},
			KeyEvents:              []engine.KeyEvent{{Id: 10, IsPressed: true}, {Id: 11, IsPressed: false}},
			MouseMovementThreshold: 1,
		},
		{
			Point:                  mathf.Vec2{X: 2.5, Y: 3},
			LeftButtonPressed:      true,
			MouseMovementThreshold: 1,
		},
		{
			Point:             mathf.Vec2{X: 6, Y: 8},
			LeftButtonPressed: false,
			MouseEvents: []engine.MouseEvent{
				{Id: 1, IsPressed: false},
				{Id: 1, IsPressed: true},
				{Id: 1, IsPressed: false},
			},
			KeyEvents:              []engine.KeyEvent{{Id: 12, IsPressed: true}},
			MouseMovementThreshold: 1,
		},
		{
			Point:                  mathf.Vec2{X: 6, Y: 8},
			LeftButtonPressed:      true, // Legacy replay snapshot without mouse edges.
			MouseMovementThreshold: 1,
		},
	}

	var liveState inputLoopState
	var sessionState InputFrameState
	var liveEvents, sessionEvents []observedEvent
	for _, frame := range frames {
		runInputLoopFrame(InputLoopConfig{
			InputFrameHooks: recordingHooks(&liveEvents),
			CurrentMousePos: func() mathf.Vec2 { return frame.Point },
			SetMousePos:     func(mathf.Vec2) {},
			GetMouseInput: func(dst []engine.MouseEvent) ([]engine.MouseEvent, uint8) {
				dst = append(dst, frame.MouseEvents...)
				var buttons uint8
				if frame.LeftButtonPressed {
					buttons = 1
				}
				return dst, buttons
			},
			GetKeyEvents: func(dst []engine.KeyEvent) []engine.KeyEvent {
				return append(dst, frame.KeyEvents...)
			},
			MouseMovementThreshold: frame.MouseMovementThreshold,
		}, &liveState)
		sessionState.Process(frame, recordingHooks(&sessionEvents))
	}

	if !reflect.DeepEqual(liveEvents, sessionEvents) {
		t.Fatalf("live events = %+v, session events = %+v", liveEvents, sessionEvents)
	}
	if liveState.frameState != sessionState {
		t.Fatalf("live state = %+v, session state = %+v", liveState.frameState, sessionState)
	}
}

func TestRunInputLoopFrameDropsEdgesAtConsumerHandoff(t *testing.T) {
	state := inputLoopState{wasSuspended: true}
	var edges []string
	runInputLoopFrame(InputLoopConfig{
		InputFrameHooks: InputFrameHooks{
			FireLeftButtonDown: func(mathf.Vec2) { edges = append(edges, "down") },
			FireLeftButtonUp:   func(mathf.Vec2) { edges = append(edges, "up") },
			OnMouseMove:        func(mathf.Vec2) { edges = append(edges, "move") },
			OnKeyPressed:       func(int64) { edges = append(edges, "key") },
		},
		CurrentMousePos: func() mathf.Vec2 { return mathf.Vec2{X: 5, Y: 6} },
		SetMousePos:     func(mathf.Vec2) {},
		GetMouseInput: func(dst []engine.MouseEvent) ([]engine.MouseEvent, uint8) {
			return append(dst,
				engine.MouseEvent{Id: 1, IsPressed: true},
				engine.MouseEvent{Id: 1, IsPressed: false},
			), 0
		},
		GetKeyEvents: func(dst []engine.KeyEvent) []engine.KeyEvent {
			return append(dst, engine.KeyEvent{Id: 10, IsPressed: true})
		},
		MouseMovementThreshold: 1,
	}, &state)

	if len(edges) != 0 {
		t.Fatalf("handoff replayed consumed edges: %v", edges)
	}
	if state.frameState.lastMousePos != (mathf.Vec2{X: 5, Y: 6}) || state.frameState.leftButtonPressed {
		t.Fatalf("handoff state = %+v", state.frameState)
	}
}

func TestMainExecutionTimedOut(t *testing.T) {
	now := time.Unix(20, 0)
	if !MainExecutionTimedOut(ScheduleState{
		MainStartedAt:   time.Unix(10, 0),
		Now:             now,
		MainExecTimeout: 5 * time.Second,
	}) {
		t.Fatal("expected Main execution timeout")
	}
	if MainExecutionTimedOut(ScheduleState{
		Now:             now,
		MainExecTimeout: 5 * time.Second,
	}) {
		t.Fatal("unexpected timeout outside Main")
	}
}

func TestSchedNow(t *testing.T) {
	scheduled := false
	err := SchedNow(
		ScheduleState{
			Now:             time.Unix(1, 0),
			MainExecTimeout: time.Second,
		},
		SchedulerHooks{
			SchedCurrent: func() { scheduled = true },
		},
	)
	if err != nil {
		t.Fatalf("SchedNow error: %v", err)
	}
	if !scheduled {
		t.Fatal("expected current thread to be scheduled")
	}

	err = SchedNow(
		ScheduleState{
			MainStartedAt:   time.Unix(1, 0),
			Now:             time.Unix(5, 0),
			MainExecTimeout: 2 * time.Second,
		},
		SchedulerHooks{},
	)
	if err != ErrMainExecutionTimedOut {
		t.Fatalf("SchedNow timeout error = %v, want %v", err, ErrMainExecutionTimedOut)
	}
}

func TestSched(t *testing.T) {
	called := false
	err := Sched(
		ScheduleState{
			Now:             time.Unix(1, 0),
			MainExecTimeout: time.Second,
		},
		3000,
		SchedulerHooks{
			IsSchedTimeout: func(ms float64) bool {
				return ms == 3000
			},
			OnSchedTimeout: func() {
				called = true
			},
		},
	)
	if err != ErrLoopExecutionTimedOut {
		t.Fatalf("Sched error = %v, want %v", err, ErrLoopExecutionTimedOut)
	}
	if !called {
		t.Fatal("expected sched timeout hook to run")
	}

	err = Sched(
		ScheduleState{
			Now:             time.Unix(1, 0),
			MainExecTimeout: time.Second,
		},
		3000,
		SchedulerHooks{
			IsSchedTimeout: func(float64) bool { return true },
		},
	)
	if err != ErrLoopExecutionTimedOut {
		t.Fatalf("Sched timeout error = %v, want %v", err, ErrLoopExecutionTimedOut)
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

func TestSwipeStateMovementExpiresTargetBeforeFinish(t *testing.T) {
	now := time.Unix(0, 0)
	var state SwipeState[string]
	state.InitWithClock(func() time.Time { return now })
	state.Begin(mathf.Vec2{}, "sprite")
	now = now.Add(600 * time.Millisecond)
	state.Expire()
	if state.target != "" || state.recognizer.IsTracking() {
		t.Fatal("expired movement did not clear the target and tracking")
	}

	// A later clock correction cannot revive an expired gesture.
	now = now.Add(-500 * time.Millisecond)
	if _, _, ok := state.Finish(mathf.Vec2{X: 100}); ok {
		t.Fatal("expired gesture revived after clock correction")
	}
}
