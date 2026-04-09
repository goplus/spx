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
