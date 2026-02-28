package timer

import "sort"

// timePrecision defines the precision multiplier for timer values to avoid floating-point errors.
// Timer values are stored as integers (milliseconds) by multiplying by this constant.
const timePrecision = 1000

var (
	// gameTimer tracks the current game time in seconds
	gameTimer float64

	// timestamps stores registered timer events in ascending order (as milliseconds)
	timestamps []int64

	// nextTimerIndex points to the next timer event to be triggered
	nextTimerIndex int
)

// Timer returns the current game timer value rounded to 3 decimal places.
// This prevents floating-point precision issues.
func Timer() float64 {
	return float64(int64(gameTimer*timePrecision)) / timePrecision
}

// ResetTimer resets the game timer and timer index to their initial state.
// This is typically called when starting or restarting a game.
func ResetTimer() {
	gameTimer = 0
	nextTimerIndex = 0
}

// OnReload clears all timer state including registered timestamps.
// This is called when the game is reloaded.
func OnReload() {
	ResetTimer()
	timestamps = timestamps[:0]
}

// RegisterTimer registers a timer event at the specified time.
// The timestamp is inserted in sorted order. Duplicate timestamps are ignored.
func RegisterTimer(timer float64) {
	timestamp := int64(timer * timePrecision)

	// Use binary search to find insertion point
	insertIndex := sort.Search(len(timestamps), func(i int) bool {
		return timestamps[i] >= timestamp
	})

	// Avoid duplicate timestamps
	if insertIndex < len(timestamps) && timestamps[insertIndex] == timestamp {
		return
	}

	// Insert timestamp at the correct position
	timestamps = append(timestamps, 0)
	copy(timestamps[insertIndex+1:], timestamps[insertIndex:])
	timestamps[insertIndex] = timestamp
}

// NextTimer returns the next timer timestamp that has been reached.
// Returns (timerValue, true) if a timestamp is ready, or (0, false) if none are ready.
// This function consumes the timestamp by advancing the internal index.
func NextTimer() (float64, bool) {
	// No timestamps registered
	if len(timestamps) == 0 {
		return 0, false
	}

	// All timestamps have been processed
	if nextTimerIndex >= len(timestamps) {
		return 0, false
	}

	// Check if the next timer timestamp should trigger
	targetTimer := timestamps[nextTimerIndex]
	currentTime := int64(gameTimer * timePrecision)
	if targetTimer > currentTime {
		return 0, false
	}

	// Timestamp is ready, advance the index and return the timer value
	nextTimerIndex++
	return float64(targetTimer) / timePrecision, true
}

// OnUpdate advances the game timer by the specified delta time.
func OnUpdate(deltaTime float64) {
	gameTimer += deltaTime
}
