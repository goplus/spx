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

func TestFindClickTarget(t *testing.T) {
	selection, ok := FindClickTarget([]int{1, 2, 3}, func(item int) (ClickSelection[int, int], bool) {
		if item >= 2 {
			return ClickSelection[int, int]{Target: item, SwipeTarget: item * 10}, true
		}
		return ClickSelection[int, int]{}, false
	})
	if !ok {
		t.Fatal("expected click target")
	}
	if selection.Target != 3 || selection.SwipeTarget != 30 {
		t.Fatalf("selection = %+v, want {Target:3 SwipeTarget:30}", selection)
	}
}

func TestHandleLeftButtonDownTarget(t *testing.T) {
	var (
		begins []int
		gates  []int
		hits   []int
		stages int
	)
	HandleLeftButtonDown(mathf.Vec2{X: 1, Y: 2}, ClickDownHooks[int, int, int]{
		FindTarget: func(point mathf.Vec2) (ClickSelection[int, int], bool) {
			if point.X != 1 || point.Y != 2 {
				t.Fatalf("point = %+v, want {1 2}", point)
			}
			return ClickSelection[int, int]{Target: 7, SwipeTarget: 70}, true
		},
		BeginSwipe: func(point mathf.Vec2, target int) {
			if point.X != 1 || point.Y != 2 {
				t.Fatalf("point = %+v, want {1 2}", point)
			}
			begins = append(begins, target)
		},
		CanTrigger: func(id int) bool {
			gates = append(gates, id)
			return true
		},
		GlobalID: -1,
		StageID:  0,
		TargetID: func(target int) (int, bool) { return target, true },
		DispatchTarget: func(target int) {
			hits = append(hits, target)
		},
		DispatchStage: func() {
			stages++
		},
	})

	if len(begins) != 1 || begins[0] != 70 {
		t.Fatalf("begins = %+v, want [70]", begins)
	}
	if len(gates) != 2 || gates[0] != -1 || gates[1] != 7 {
		t.Fatalf("gates = %+v, want [-1 7]", gates)
	}
	if len(hits) != 1 || hits[0] != 7 {
		t.Fatalf("hits = %+v, want [7]", hits)
	}
	if stages != 0 {
		t.Fatalf("stages = %d, want 0", stages)
	}
}

func TestHandleLeftButtonDownBlockedByGlobalGate(t *testing.T) {
	var (
		begins []int
		gates  []int
		hits   int
		stages int
	)
	HandleLeftButtonDown(mathf.Vec2{}, ClickDownHooks[int, int, int]{
		FindTarget: func(mathf.Vec2) (ClickSelection[int, int], bool) {
			return ClickSelection[int, int]{Target: 7, SwipeTarget: 70}, true
		},
		BeginSwipe: func(_ mathf.Vec2, target int) {
			begins = append(begins, target)
		},
		CanTrigger: func(id int) bool {
			gates = append(gates, id)
			return id != -1
		},
		GlobalID: -1,
		StageID:  0,
		TargetID: func(target int) (int, bool) { return target, true },
		DispatchTarget: func(int) {
			hits++
		},
		DispatchStage: func() {
			stages++
		},
	})

	if len(begins) != 1 || begins[0] != 70 {
		t.Fatalf("begins = %+v, want [70]", begins)
	}
	if len(gates) != 1 || gates[0] != -1 {
		t.Fatalf("gates = %+v, want [-1]", gates)
	}
	if hits != 0 || stages != 0 {
		t.Fatalf("hits=%d stages=%d, want 0/0", hits, stages)
	}
}

func TestHandleLeftButtonDownStage(t *testing.T) {
	var (
		begins []int
		gates  []int
		stages int
	)
	HandleLeftButtonDown(mathf.Vec2{}, ClickDownHooks[int, int, int]{
		FindTarget: func(mathf.Vec2) (ClickSelection[int, int], bool) {
			return ClickSelection[int, int]{}, false
		},
		BeginSwipe: func(_ mathf.Vec2, target int) {
			begins = append(begins, target)
		},
		CanTrigger: func(id int) bool {
			gates = append(gates, id)
			return true
		},
		GlobalID: -1,
		StageID:  0,
		DispatchStage: func() {
			stages++
		},
	})

	if len(begins) != 1 || begins[0] != 0 {
		t.Fatalf("begins = %+v, want [0]", begins)
	}
	if len(gates) != 2 || gates[0] != -1 || gates[1] != 0 {
		t.Fatalf("gates = %+v, want [-1 0]", gates)
	}
	if stages != 1 {
		t.Fatalf("stages = %d, want 1", stages)
	}
}

func TestHandleLeftButtonDownBlockedTargetGate(t *testing.T) {
	var (
		gates  []int
		hits   int
		stages int
	)
	HandleLeftButtonDown(mathf.Vec2{}, ClickDownHooks[int, int, int]{
		FindTarget: func(mathf.Vec2) (ClickSelection[int, int], bool) {
			return ClickSelection[int, int]{Target: 7, SwipeTarget: 70}, true
		},
		BeginSwipe: func(mathf.Vec2, int) {},
		CanTrigger: func(id int) bool {
			gates = append(gates, id)
			return id != 7
		},
		GlobalID: -1,
		StageID:  0,
		TargetID: func(target int) (int, bool) { return target, true },
		DispatchTarget: func(int) {
			hits++
		},
		DispatchStage: func() {
			stages++
		},
	})

	if len(gates) != 2 || gates[0] != -1 || gates[1] != 7 {
		t.Fatalf("gates = %+v, want [-1 7]", gates)
	}
	if hits != 0 || stages != 0 {
		t.Fatalf("hits=%d stages=%d, want 0/0", hits, stages)
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

func TestProcessTriggerPairs(t *testing.T) {
	pairs := []engine.TriggerEvent{
		{
			Src: &engine.Sprite{Target: "src"},
			Dst: &engine.Sprite{Target: "dst"},
		},
		{
			Src: &engine.Sprite{Target: "bad"},
			Dst: &engine.Sprite{Target: "dst"},
		},
	}

	var (
		touches  [][2]string
		invalids int
	)
	ProcessTriggerPairs(
		pairs,
		func(target any) (string, bool) {
			v, ok := target.(string)
			return v, ok && v != "bad"
		},
		func(v string) bool { return v != "dst-blocked" },
		func(src, dst string) { touches = append(touches, [2]string{src, dst}) },
		func() { invalids++ },
	)

	if len(touches) != 1 || touches[0] != [2]string{"src", "dst"} {
		t.Fatalf("unexpected touches: %+v", touches)
	}
	if invalids != 1 {
		t.Fatalf("invalids = %d, want 1", invalids)
	}
}
