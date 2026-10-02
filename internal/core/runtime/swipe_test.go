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
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
)

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
