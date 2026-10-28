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
	"time"

	"github.com/goplus/spbase/mathf"
)

type SwipeResult struct {
	Direction float64
	Velocity  float64
	Distance  float64
	StartPos  mathf.Vec2
	EndPos    mathf.Vec2
}

const (
	swipeTimeLimit       = 0.5
	swipeMinimumDistance = 50.0
	swipeMaximumDistance = 500.0
)

type SwipeRecognizer struct {
	isTracking bool
	startTime  time.Time
	startPoint mathf.Vec2
	now        func() time.Time
}

// A nil clock falls back to the system clock.
func (sr *SwipeRecognizer) InitWithClock(now func() time.Time) {
	if now == nil {
		now = time.Now
	}
	*sr = SwipeRecognizer{now: now}
}

func (sr *SwipeRecognizer) StartTracking(startPos mathf.Vec2) {
	sr.isTracking = true
	sr.startTime = sr.nowTime()
	sr.startPoint = startPos
}

func (sr *SwipeRecognizer) StopTracking() {
	sr.isTracking = false
}

func (sr *SwipeRecognizer) IsTracking() bool {
	return sr.isTracking
}

// Expire stops tracking if the gesture has exceeded its time limit.
func (sr *SwipeRecognizer) Expire() {
	if !sr.isTracking {
		return
	}
	if sr.elapsedSeconds() > swipeTimeLimit {
		sr.StopTracking()
	}
}

func (sr *SwipeRecognizer) Finish(point mathf.Vec2) (SwipeResult, bool) {
	if !sr.isTracking {
		return SwipeResult{}, false
	}
	elapsed := sr.elapsedSeconds()
	sr.StopTracking()
	if elapsed <= 0 || elapsed > swipeTimeLimit {
		return SwipeResult{}, false
	}

	dx := point.X - sr.startPoint.X
	dy := point.Y - sr.startPoint.Y
	distance := math.Sqrt(dx*dx + dy*dy)
	if distance < swipeMinimumDistance || distance > swipeMaximumDistance {
		return SwipeResult{}, false
	}

	return SwipeResult{
		Direction: calculateDirection(sr.startPoint, point),
		Velocity:  distance / elapsed,
		Distance:  distance,
		StartPos:  sr.startPoint,
		EndPos:    point,
	}, true
}

func (sr *SwipeRecognizer) nowTime() time.Time {
	now := sr.now
	if now == nil {
		now = time.Now
	}
	return now()
}

func (sr *SwipeRecognizer) elapsedSeconds() float64 {
	return sr.nowTime().Sub(sr.startTime).Seconds()
}

func calculateDirection(startPoint, endPoint mathf.Vec2) float64 {
	delta := endPoint.Sub(startPoint)
	angle := math.Atan2(delta.Y, delta.X) * (180 / math.Pi)
	if angle < 0 {
		angle += 360
	}
	switch {
	case angle >= 315 || angle < 45:
		return 90
	case angle >= 45 && angle < 135:
		return 180
	case angle >= 135 && angle < 225:
		return -90
	case angle >= 225 && angle < 315:
		return 0
	default:
		return -1
	}
}
