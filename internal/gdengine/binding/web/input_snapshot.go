//go:build js && wasm

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

package webffi

import (
	"sync"
	"syscall/js"

	"github.com/goplus/spbase/mathf"
)

type inputSnapshot struct {
	mouse     mathf.Vec2
	mouseBits uint32
	ok        bool
	frame     uint64
}

var (
	inputSnap inputSnapshot

	actionIDMu  sync.RWMutex
	actionIDs   = map[string]int{}
	actionEpoch int
)

func SyncWebInputSnapshot() {
	inputSnap.frame++
	clearActionCache(inputSnap.frame)
	syncActionIDCache()

	bindings := js.Global().Get("GdspxFuncs")
	if bindings.Type() != js.TypeFunction {
		inputSnap.ok = false
		return
	}
	outputs := bindings.Get("arrayOutputs")
	if outputs.Type() != js.TypeObject || outputs.IsNull() {
		inputSnap.ok = false
		return
	}
	fn := outputs.Get("gdspx_input_write_snapshot")
	if fn.Type() != js.TypeFunction {
		inputSnap.ok = false
		return
	}

	data, ok := JsToGdArray(fn.Invoke()).([]float32)
	if !ok || len(data) < 3 {
		inputSnap.ok = false
		return
	}

	inputSnap.mouse = mathf.Vec2{X: float64(data[0]), Y: float64(data[1])}
	inputSnap.mouseBits = uint32(data[2])
	inputSnap.ok = true
}

func webActionBool(kind, action string) (bool, bool) {
	id, ok := webActionID(action)
	if !ok {
		return false, false
	}

	var fn js.Value
	switch kind {
	case "pressed":
		fn = API.SpxInputIsActionPressedId
	case "just_pressed":
		fn = API.SpxInputIsActionJustPressedId
	case "just_released":
		fn = API.SpxInputIsActionJustReleasedId
	}
	if fn.Type() != js.TypeFunction {
		return false, false
	}

	value := fn.Invoke(id, 0)
	if value.IsUndefined() || value.IsNull() {
		return false, false
	}
	return value.Bool(), true
}

func webActionAxis(neg, pos string) (float64, bool) {
	negID, ok := webActionID(neg)
	if !ok {
		return 0, false
	}
	posID, ok := webActionID(pos)
	if !ok {
		return 0, false
	}

	fn := API.SpxInputGetAxisId
	if fn.Type() != js.TypeFunction {
		return 0, false
	}

	value := fn.Invoke(negID, 0, posID, 0)
	if value.IsUndefined() || value.IsNull() {
		return 0, false
	}
	return value.Float(), true
}

func webActionID(action string) (int, bool) {
	actionIDMu.RLock()
	id, ok := actionIDs[action]
	actionIDMu.RUnlock()
	if ok {
		return id, true
	}

	syncActionIDCache()

	actionIDMu.RLock()
	id, ok = actionIDs[action]
	actionIDMu.RUnlock()
	if ok {
		return id, true
	}

	fn := js.Global().Get("GdspxGetInputActionID")
	if fn.Type() != js.TypeFunction {
		return 0, false
	}

	id = fn.Invoke(action).Int()
	if id < 0 {
		return 0, false
	}

	actionIDMu.Lock()
	actionIDs[action] = id
	actionIDMu.Unlock()
	return id, true
}

func syncActionIDCache() {
	fn := js.Global().Get("GdspxGetInputActionEpoch")
	if fn.Type() != js.TypeFunction {
		return
	}

	epoch := fn.Invoke().Int()
	actionIDMu.Lock()
	if epoch != actionEpoch {
		actionEpoch = epoch
		clear(actionIDs)
	}
	actionIDMu.Unlock()
}
