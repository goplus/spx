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

package time

import (
	"math"
	"sync/atomic"
	stdtime "time"
)

const DefaultFPS = 30

var (
	timeSinceLevelLoad      float64
	logicalTimeCompensation float64
	deltaTime               float64
	fixedDeltaTimeBits      atomic.Uint64
	curFrame                atomic.Int64
	startTimestamp          stdtime.Time
	fps                     float64
)

func RealTimeSinceStart() float64 {
	return stdtime.Since(startTimestamp).Seconds()
}

func FPS() float64 {
	return fps
}

func Frame() int64 {
	return curFrame.Load()
}

func DeltaTime() float64 {
	return deltaTime
}

func FixedDeltaTime() (float64, bool) {
	fixedDeltaTime := math.Float64frombits(fixedDeltaTimeBits.Load())
	return fixedDeltaTime, fixedDeltaTime > 0
}

// EffectiveLogicalDeltaTime returns the delta used by SPX logical update paths.
// It intentionally does not apply to Godot fixed-physics callbacks, which keep
// their engine-provided delta until deterministic physics is supported.
func EffectiveLogicalDeltaTime(rawDelta float64) float64 {
	fixedDeltaTime := math.Float64frombits(fixedDeltaTimeBits.Load())
	if fixedDeltaTime > 0 {
		return fixedDeltaTime
	}
	return rawDelta
}

func SetFixedDeltaTime(delta float64) {
	if delta <= 0 {
		fixedDeltaTimeBits.Store(0)
		return
	}
	fixedDeltaTimeBits.Store(math.Float64bits(delta))
}

func TimeSinceLevelLoad() float64 {
	return timeSinceLevelLoad
}

func Start() {
	timeSinceLevelLoad = 0
	logicalTimeCompensation = 0
	deltaTime = 0
	curFrame.Store(0)
	fps = DefaultFPS

	startTimestamp = stdtime.Now()
	ResetTimer()
}

// Update records frame timing. The delta argument must already be the
// resolved SPX logical delta for the current update path.
func Update(delta float64, pfps float64) {
	deltaTime = delta
	advanceLogicalTime(delta)
	curFrame.Add(1)
	fps = pfps
}

// advanceLogicalTime keeps repeated fixed steps numerically stable.
func advanceLogicalTime(delta float64) {
	adjusted := delta - logicalTimeCompensation
	next := timeSinceLevelLoad + adjusted
	if math.IsInf(next, 0) || math.IsNaN(next) {
		logicalTimeCompensation = 0
	} else {
		logicalTimeCompensation = (next - timeSinceLevelLoad) - adjusted
	}
	timeSinceLevelLoad = next
}
