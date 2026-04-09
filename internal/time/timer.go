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
	"sort"
)

const timePrecision = 1000

var (
	timerBaseTime  float64
	timestamps     []int64
	nextTimerIndex int
)

func Timer() float64 {
	return timestampToTimerValue(currentTimerTimestamp())
}

func ResetTimer() {
	timerBaseTime = timeSinceLevelLoad
	nextTimerIndex = 0
}

func OnReload() {
	ResetTimer()
	timestamps = timestamps[:0]
}

func RegisterTimer(timer float64) {
	timestamp := timerValueToTimestamp(timer)

	insertIndex := sort.Search(len(timestamps), func(i int) bool {
		return timestamps[i] >= timestamp
	})

	if insertIndex < len(timestamps) && timestamps[insertIndex] == timestamp {
		return
	}

	timestamps = append(timestamps, 0)
	copy(timestamps[insertIndex+1:], timestamps[insertIndex:])
	timestamps[insertIndex] = timestamp
}

func NextTimer() (float64, bool) {
	if len(timestamps) == 0 {
		return 0, false
	}

	if nextTimerIndex >= len(timestamps) {
		return 0, false
	}

	targetTimer := timestamps[nextTimerIndex]
	currentTime := currentTimerTimestamp()
	if targetTimer > currentTime {
		return 0, false
	}

	nextTimerIndex++
	return timestampToTimerValue(targetTimer), true
}

func currentTimerTimestamp() int64 {
	currentTimer := timeSinceLevelLoad - timerBaseTime
	if currentTimer <= 0 {
		return 0
	}
	return timerValueToTimestamp(currentTimer)
}

func timerValueToTimestamp(timer float64) int64 {
	return int64(timer * timePrecision)
}

func timestampToTimerValue(timestamp int64) float64 {
	return float64(timestamp) / timePrecision
}
