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

	. "github.com/goplus/spbase/mathf"
)

type inputSnapshot struct {
	mouse     Vec2
	mouseBits uint32
	ok        bool
	frame     uint64
}

var (
	inputSnap inputSnapshot

	keyMu    sync.RWMutex
	keyDown  = map[int64]bool{}
	keyKnown = map[int64]bool{}

	actionIDMu  sync.RWMutex
	actionIDs   = map[string]int{}
	actionEpoch int

	actionMu    sync.Mutex
	actionFrame uint64
	actionBool  = map[string]bool{}
	actionAxis  = map[string]float64{}
)

func SyncWebInputSnapshot() {
	inputSnap.frame++
	clearActionCache(inputSnap.frame)

	fn := js.Global().Get("GdspxInputSnapshot")
	if fn.Type() != js.TypeFunction {
		inputSnap.ok = false
		return
	}

	data, ok := JsToGdArray(fn.Invoke()).([]float32)
	if !ok || len(data) < 3 {
		inputSnap.ok = false
		return
	}

	inputSnap.mouse = Vec2{X: float64(data[0]), Y: float64(data[1])}
	inputSnap.mouseBits = uint32(data[2])
	inputSnap.ok = true
}

func WebInputMousePos(fallback func() Vec2) Vec2 {
	if inputSnap.ok {
		return inputSnap.mouse
	}
	return fallback()
}

func WebInputMouseState(id int64, fallback func() bool) bool {
	if inputSnap.ok && id >= 0 && id < 32 {
		return inputSnap.mouseBits&(1<<uint(id)) != 0
	}
	return fallback()
}

func WebInputKeyState(key int64, fallback func() bool) bool {
	keyMu.RLock()
	known := keyKnown[key]
	pressed := keyDown[key]
	keyMu.RUnlock()
	if known {
		return pressed
	}
	return fallback()
}

func RecordWebKeyState(key int64, pressed bool) {
	keyMu.Lock()
	keyKnown[key] = true
	keyDown[key] = pressed
	keyMu.Unlock()
}

func CachedActionBool(kind, action string, fallback func() bool) bool {
	key := kind + "\x00" + action

	actionMu.Lock()
	if value, ok := actionBool[key]; ok {
		actionMu.Unlock()
		return value
	}
	actionMu.Unlock()

	value, ok := webActionBool(kind, action)
	if !ok {
		value = fallback()
	}

	actionMu.Lock()
	actionBool[key] = value
	actionMu.Unlock()
	return value
}

func CachedActionAxis(neg, pos string, fallback func() float64) float64 {
	key := neg + "\x00" + pos

	actionMu.Lock()
	if value, ok := actionAxis[key]; ok {
		actionMu.Unlock()
		return value
	}
	actionMu.Unlock()

	value, ok := webActionAxis(neg, pos)
	if !ok {
		value = fallback()
	}

	actionMu.Lock()
	actionAxis[key] = value
	actionMu.Unlock()
	return value
}

func clearActionCache(frame uint64) {
	actionMu.Lock()
	defer actionMu.Unlock()
	if actionFrame == frame {
		return
	}
	actionFrame = frame
	clear(actionBool)
	clear(actionAxis)
}

func webActionBool(kind, action string) (bool, bool) {
	id, ok := webActionID(action)
	if !ok {
		return false, false
	}

	fn := js.Global().Get("GdspxInputActionBool")
	if fn.Type() != js.TypeFunction {
		return false, false
	}

	value := fn.Invoke(actionKindID(kind), id)
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

	fn := js.Global().Get("GdspxInputAxisByID")
	if fn.Type() != js.TypeFunction {
		return 0, false
	}

	value := fn.Invoke(negID, posID)
	if value.IsUndefined() || value.IsNull() {
		return 0, false
	}
	return value.Float(), true
}

func webActionID(action string) (int, bool) {
	syncActionIDCache()

	actionIDMu.RLock()
	id, ok := actionIDs[action]
	actionIDMu.RUnlock()
	if ok {
		return id, true
	}

	fn := js.Global().Get("GdspxInputActionID")
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
	fn := js.Global().Get("GdspxInputActionEpoch")
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

func actionKindID(kind string) int {
	switch kind {
	case "pressed":
		return 1
	case "just_pressed":
		return 2
	case "just_released":
		return 3
	default:
		return 0
	}
}
