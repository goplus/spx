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

package input

import (
	"math"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
)

func TestSwipeRecognizerFinishDetectsRightSwipe(t *testing.T) {
	now := time.Unix(0, 0)
	var sr SwipeRecognizer
	sr.InitWithClock(func() time.Time { return now })

	start := mathf.NewVec2(0, 0)
	end := mathf.NewVec2(100, 0)

	sr.StartTracking(start)
	now = now.Add(100 * time.Millisecond)
	if _, ok := sr.OnMouseMove(end); ok {
		t.Fatal("expected move-only tracking not to emit swipe before finish")
	}

	now = now.Add(10 * time.Millisecond)
	result, ok := sr.Finish(end)
	if !ok {
		t.Fatal("expected swipe to be detected")
	}
	if sr.IsTracking() {
		t.Fatal("expected tracking to stop after finish")
	}
	if result.Direction != 90 {
		t.Fatalf("unexpected swipe direction: got %v", result.Direction)
	}
	if result.Distance != 100 {
		t.Fatalf("unexpected swipe distance: got %v", result.Distance)
	}
	if result.StartPos != start || result.EndPos != end {
		t.Fatalf("unexpected swipe points: got start=%v end=%v", result.StartPos, result.EndPos)
	}
	if math.Abs(result.Velocity-909.090909090909) > 0.0001 {
		t.Fatalf("unexpected swipe velocity: got %v", result.Velocity)
	}
}

func TestSwipeRecognizerTimeLimitStopsTracking(t *testing.T) {
	now := time.Unix(0, 0)
	var sr SwipeRecognizer
	sr.InitWithClock(func() time.Time { return now })

	sr.StartTracking(mathf.NewVec2(0, 0))
	now = now.Add(600 * time.Millisecond)

	if _, ok := sr.OnMouseMove(mathf.NewVec2(100, 0)); ok {
		t.Fatal("expected expired swipe not to emit a result")
	}
	if sr.IsTracking() {
		t.Fatal("expected expired swipe tracking to stop")
	}
}

func TestSwipeRecognizerInitWithClockResetsTrackingAndNilUsesDefault(t *testing.T) {
	now := time.Unix(0, 0)
	var sr SwipeRecognizer
	sr.InitWithClock(func() time.Time { return now })
	sr.StartTracking(mathf.NewVec2(1, 2))
	now = now.Add(100 * time.Millisecond)
	sr.OnMouseMove(mathf.NewVec2(3, 4))

	sr.InitWithClock(nil)
	if sr.IsTracking() {
		t.Fatal("expected reinitialization to stop tracking")
	}
	if !sr.startTime.IsZero() {
		t.Fatalf("startTime = %v, want zero", sr.startTime)
	}
	if sr.startPoint != (mathf.Vec2{}) || sr.endPoint != (mathf.Vec2{}) {
		t.Fatalf("tracking points = %v/%v, want zero values", sr.startPoint, sr.endPoint)
	}
	if sr.now == nil {
		t.Fatal("nil clock was not replaced with the default clock")
	}
}

func TestCalculateDirectionMapsVerticalSwipesToScreenCoordinates(t *testing.T) {
	if got := calculateDirection(mathf.NewVec2(0, 0), mathf.NewVec2(0, 100)); got != 180 {
		t.Fatalf("downward direction = %v, want 180", got)
	}
	if got := calculateDirection(mathf.NewVec2(0, 100), mathf.NewVec2(0, 0)); got != 0 {
		t.Fatalf("upward direction = %v, want 0", got)
	}
}
