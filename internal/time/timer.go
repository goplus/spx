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

import "slices"

const timePrecision = 1000

var (
	timerBaseTime    float64
	registeredTimers []int64
	pendingTimers    []int64
)

func Timer() float64 {
	return float64(timerMillis()) / timePrecision
}

func ResetTimer() {
	timerBaseTime = timeSinceLevelLoad
	pendingTimers = append(pendingTimers[:0], registeredTimers...)
}

func OnReload() {
	registeredTimers = registeredTimers[:0]
	ResetTimer()
}

// RegisterTimer returns a millisecond key that fires once per reset.
// New overdue keys fire on the next poll.
func RegisterTimer(timer float64) int64 {
	timestamp := int64(timer * timePrecision)
	i, exists := slices.BinarySearch(registeredTimers, timestamp)
	if !exists {
		registeredTimers = slices.Insert(registeredTimers, i, timestamp)
		i, _ = slices.BinarySearch(pendingTimers, timestamp)
		pendingTimers = slices.Insert(pendingTimers, i, timestamp)
	}
	return timestamp
}

func NextTimer() (int64, bool) {
	if len(pendingTimers) == 0 || pendingTimers[0] > timerMillis() {
		return 0, false
	}
	timestamp := pendingTimers[0]
	pendingTimers = pendingTimers[1:]
	return timestamp, true
}

func timerMillis() int64 {
	return int64(max(timeSinceLevelLoad-timerBaseTime, 0) * timePrecision)
}
