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
	realTimeSinceLevelLoad float64
	timeSinceLevelLoad     float64
	deltaTime              float64
	realDeltaTime          float64
	fixedDeltaTimeBits     atomic.Uint64
	timeScale              float64
	curFrame               atomic.Int64
	setTimeScaleCallback   func(float64)
	startTimestamp         stdtime.Time
	lastTimestamp          stdtime.Time
	fps                    float64
)

func Sleep(ms float64) {
	stdtime.Sleep(stdtime.Microsecond * stdtime.Duration(ms*1000))
}

func RealTimeSinceStart() float64 {
	return stdtime.Since(startTimestamp).Seconds()
}

func RealTimeSinceCurFrame() float64 {
	return stdtime.Since(lastTimestamp).Seconds()
}

func RealTimeSinceCurFrameMs() float64 {
	return RealTimeSinceCurFrame() * 1000
}

func FPS() float64 {
	return fps
}

func Frame() int64 {
	return curFrame.Load()
}

func TimeScale() float64 {
	return timeScale
}

func SetTimeScale(value float64) {
	if setTimeScaleCallback != nil {
		setTimeScaleCallback(value)
	}
	timeScale = value
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

func UnscaledDeltaTime() float64 {
	return realDeltaTime
}

func UnscaledTimeSinceLevelLoad() float64 {
	return realTimeSinceLevelLoad
}

func TimeSinceLevelLoad() float64 {
	return timeSinceLevelLoad
}

func Start(setTimeScaleCB func(float64)) {
	now := stdtime.Now()

	realTimeSinceLevelLoad, timeSinceLevelLoad = 0, 0
	deltaTime, realDeltaTime = 0, 0
	timeScale = 1
	curFrame.Store(0)
	fps = DefaultFPS

	setTimeScaleCallback = setTimeScaleCB
	startTimestamp, lastTimestamp = now, now
	ResetTimer()
}

// Update records frame timing. The delta argument must already be the
// resolved SPX logical delta for the current update path.
func Update(delta float64, pfps float64) {
	curTime := stdtime.Now()
	realTimeSinceLevelLoad = curTime.Sub(startTimestamp).Seconds()
	realDeltaTime = curTime.Sub(lastTimestamp).Seconds()
	lastTimestamp = curTime

	deltaTime = delta
	timeSinceLevelLoad += deltaTime
	curFrame.Add(1)
	fps = pfps
}
