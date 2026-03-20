package engine

import "sync/atomic"

var mouseButtonStates [4]uint32

func resetMouseButtonStates() {
	for i := range mouseButtonStates {
		atomic.StoreUint32(&mouseButtonStates[i], 0)
	}
}

func setMouseButtonPressed(id int64, pressed bool) {
	if id < 0 || id >= int64(len(mouseButtonStates)) {
		return
	}
	var state uint32
	if pressed {
		state = 1
	}
	atomic.StoreUint32(&mouseButtonStates[id], state)
}

func IsMouseButtonPressed(id int64) bool {
	if id < 0 || id >= int64(len(mouseButtonStates)) {
		return false
	}
	return atomic.LoadUint32(&mouseButtonStates[id]) != 0
}

func AnyMouseButtonPressed() bool {
	return IsMouseButtonPressed(1) || IsMouseButtonPressed(2)
}
