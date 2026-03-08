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
	engine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

type SwipeResult struct {
	Direction float64
	Velocity  float64
	Distance  float64
	StartPos  mathf.Vec2
	EndPos    mathf.Vec2
}

// SwipeRecognizer handles swipe gesture detection and recognition.
type SwipeRecognizer struct {
	timeToSwipe     float64
	enableTimeLimit bool
	minimumDistance float64
	maximumDistance float64

	isTracking bool
	startTime  time.Time
	startPoint mathf.Vec2
	endPoint   mathf.Vec2
	points     []mathf.Vec2
	now        func() time.Time
}

func (sr *SwipeRecognizer) Init() {
	sr.timeToSwipe = 0.5
	sr.enableTimeLimit = true
	sr.minimumDistance = 50.0
	sr.maximumDistance = 500.0
	sr.points = make([]mathf.Vec2, 0, 50)
	if sr.now == nil {
		sr.now = time.Now
	}
}

func (sr *SwipeRecognizer) StartTracking(startPos mathf.Vec2) {
	sr.isTracking = true
	sr.startTime = sr.nowTime()
	sr.startPoint = startPos
	sr.endPoint = startPos
	sr.points = sr.points[:0]
	sr.points = append(sr.points, startPos)
}

func (sr *SwipeRecognizer) StopTracking() {
	sr.isTracking = false
}

func (sr *SwipeRecognizer) IsTracking() bool {
	return sr.isTracking
}

func (sr *SwipeRecognizer) OnMouseMove(pos mathf.Vec2) (SwipeResult, bool) {
	if !sr.isTracking {
		return SwipeResult{}, false
	}
	if sr.enableTimeLimit && sr.timeToSwipe > 0 {
		if sr.elapsedSeconds() > sr.timeToSwipe {
			sr.StopTracking()
			return SwipeResult{}, false
		}
	}
	sr.points = append(sr.points, pos)
	sr.endPoint = pos
	return SwipeResult{}, false
}

func (sr *SwipeRecognizer) Finish(point mathf.Vec2) (SwipeResult, bool) {
	if !sr.isTracking {
		return SwipeResult{}, false
	}
	sr.endPoint = point
	result, ok := sr.checkForSwipeCompletion()
	sr.StopTracking()
	return result, ok
}

func (sr *SwipeRecognizer) checkForSwipeCompletion() (SwipeResult, bool) {
	if len(sr.points) < 2 {
		return SwipeResult{}, false
	}

	elapsed := sr.elapsedSeconds()
	if sr.enableTimeLimit && sr.timeToSwipe > 0 {
		if elapsed > sr.timeToSwipe {
			return SwipeResult{}, false
		}
	}

	dx := sr.endPoint.X - sr.startPoint.X
	dy := sr.endPoint.Y - sr.startPoint.Y
	distance := math.Sqrt(dx*dx + dy*dy)
	if distance < sr.minimumDistance || distance > sr.maximumDistance {
		return SwipeResult{}, false
	}

	return SwipeResult{
		Direction: calculateDirection(sr.startPoint, sr.endPoint),
		Velocity:  distance / elapsed,
		Distance:  distance,
		StartPos:  sr.startPoint,
		EndPos:    sr.endPoint,
	}, true
}

func calculateDirection(startPoint, endPoint mathf.Vec2) float64 {
	delta := endPoint.Sub(startPoint)
	angle := engine.RadToDeg(math.Atan2(delta.Y, delta.X))
	if angle < 0 {
		angle += 360
	}
	switch {
	case angle >= 315 || angle < 45:
		return 90
	case angle >= 45 && angle < 135:
		return 0
	case angle >= 135 && angle < 225:
		return -90
	case angle >= 225 && angle < 315:
		return 180
	default:
		return -1
	}
}

func (sr *SwipeRecognizer) nowTime() time.Time {
	if sr.now == nil {
		sr.now = time.Now
	}
	return sr.now()
}

func (sr *SwipeRecognizer) elapsedSeconds() float64 {
	return sr.nowTime().Sub(sr.startTime).Seconds()
}
