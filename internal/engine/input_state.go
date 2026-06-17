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

package engine

import (
	"math"
	"sync/atomic"

	gdx "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

var mouseButtonStates [4]uint32
var mouseButtonStickyUntil [4]uint64

// Pending presses let slow script polling observe a click once even if the
// physical button was already released before the next script iteration.
var mouseButtonPending [4]uint32
var mouseButtonPendingObservedEpoch [4]uint64
var mouseButtonPendingXBits [4]uint64
var mouseButtonPendingYBits [4]uint64
var mouseInputEpoch uint64
var mouseButtonSnapshotProvider = func() (float64, float64, bool) {
	if gdx.InputMgr == nil {
		return 0, 0, false
	}
	pos := gdx.InputMgr.GetGlobalMousePos()
	return pos.X, pos.Y, true
}

func resetMouseButtonStates() {
	for i := range mouseButtonStates {
		atomic.StoreUint32(&mouseButtonStates[i], 0)
		atomic.StoreUint64(&mouseButtonStickyUntil[i], 0)
		atomic.StoreUint32(&mouseButtonPending[i], 0)
		atomic.StoreUint64(&mouseButtonPendingObservedEpoch[i], 0)
		atomic.StoreUint64(&mouseButtonPendingXBits[i], 0)
		atomic.StoreUint64(&mouseButtonPendingYBits[i], 0)
	}
	atomic.StoreUint64(&mouseInputEpoch, 0)
}

func beginMouseInputFrame() {
	atomic.AddUint64(&mouseInputEpoch, 1)
}

func setMouseButtonPressed(id int64, pressed bool) {
	if id < 0 || id >= int64(len(mouseButtonStates)) {
		return
	}
	var state uint32
	if pressed {
		state = 1
		// Keep short clicks visible through the current and next script frame,
		// so polling code running behind a heavy warp loop can still observe them.
		epoch := atomic.LoadUint64(&mouseInputEpoch)
		atomic.StoreUint64(&mouseButtonStickyUntil[id], epoch+1)
		atomic.StoreUint32(&mouseButtonPending[id], 1)
		atomic.StoreUint64(&mouseButtonPendingObservedEpoch[id], 0)
		storeMouseButtonPendingPos(id)
	}
	atomic.StoreUint32(&mouseButtonStates[id], state)
}

func IsMouseButtonPressed(id int64) bool {
	if id < 0 || id >= int64(len(mouseButtonStates)) {
		return false
	}
	if atomic.LoadUint32(&mouseButtonStates[id]) != 0 {
		return true
	}
	stickyUntil := atomic.LoadUint64(&mouseButtonStickyUntil[id])
	if stickyUntil == 0 {
		return false
	}
	return atomic.LoadUint64(&mouseInputEpoch) <= stickyUntil
}

func AnyMouseButtonPressed() bool {
	return IsMouseButtonPressed(1) || IsMouseButtonPressed(2)
}

func GetMouseButtonPressSnapshot(id int64) (float64, float64, bool) {
	if id < 0 || id >= int64(len(mouseButtonStates)) {
		return 0, 0, false
	}
	if atomic.LoadUint32(&mouseButtonStates[id]) != 0 {
		return 0, 0, false
	}
	if atomic.LoadUint32(&mouseButtonPending[id]) == 0 {
		return 0, 0, false
	}

	stickyUntil := atomic.LoadUint64(&mouseButtonStickyUntil[id])
	if stickyUntil == 0 || atomic.LoadUint64(&mouseInputEpoch) > stickyUntil {
		return 0, 0, false
	}

	x, y := loadMouseButtonPendingPos(id)
	return x, y, true
}

func IsMouseButtonPressedForPolling(id int64) bool {
	if id < 0 || id >= int64(len(mouseButtonStates)) {
		return false
	}

	currentEpoch := atomic.LoadUint64(&mouseInputEpoch)
	if atomic.LoadUint32(&mouseButtonStates[id]) != 0 {
		atomic.StoreUint64(&mouseButtonPendingObservedEpoch[id], currentEpoch)
		return true
	}

	if atomic.LoadUint32(&mouseButtonPending[id]) == 0 {
		atomic.StoreUint64(&mouseButtonPendingObservedEpoch[id], 0)
		return false
	}

	observedEpoch := atomic.LoadUint64(&mouseButtonPendingObservedEpoch[id])
	if observedEpoch == 0 {
		// Keep the pending press visible for the whole current input frame once a
		// script has observed it, then clear it on the next frame.
		atomic.StoreUint64(&mouseButtonPendingObservedEpoch[id], currentEpoch)
		return true
	}
	if observedEpoch == currentEpoch {
		return true
	}

	atomic.StoreUint32(&mouseButtonPending[id], 0)
	atomic.StoreUint64(&mouseButtonPendingObservedEpoch[id], 0)
	return false
}

func AnyMouseButtonPressedForPolling() bool {
	return IsMouseButtonPressedForPolling(1) || IsMouseButtonPressedForPolling(2)
}

func GetPollingMousePos() (float64, float64, bool) {
	const leftButtonID = 1
	if atomic.LoadUint32(&mouseButtonStates[leftButtonID]) != 0 {
		return 0, 0, false
	}
	if atomic.LoadUint32(&mouseButtonPending[leftButtonID]) == 0 {
		return 0, 0, false
	}

	currentEpoch := atomic.LoadUint64(&mouseInputEpoch)
	observedEpoch := atomic.LoadUint64(&mouseButtonPendingObservedEpoch[leftButtonID])
	if observedEpoch != 0 && observedEpoch != currentEpoch {
		return 0, 0, false
	}

	x, y := loadMouseButtonPendingPos(leftButtonID)
	return x, y, true
}

func storeMouseButtonPendingPos(id int64) {
	x, y, ok := mouseButtonSnapshotProvider()
	if !ok {
		return
	}
	atomic.StoreUint64(&mouseButtonPendingXBits[id], math.Float64bits(x))
	atomic.StoreUint64(&mouseButtonPendingYBits[id], math.Float64bits(y))
}

func loadMouseButtonPendingPos(id int64) (float64, float64) {
	x := math.Float64frombits(atomic.LoadUint64(&mouseButtonPendingXBits[id]))
	y := math.Float64frombits(atomic.LoadUint64(&mouseButtonPendingYBits[id]))
	return x, y
}
