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
