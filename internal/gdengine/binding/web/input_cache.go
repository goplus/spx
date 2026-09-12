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

	"github.com/goplus/spbase/mathf"
)

var (
	keyMu sync.RWMutex
	// Missing keys are unknown; false means released.
	keyDown = map[int64]bool{}

	actionMu    sync.Mutex
	actionFrame uint64
	actionBool  = map[string]bool{}
	actionAxis  = map[string]float64{}
)

func RecordWebKeyState(key int64, pressed bool) {
	keyMu.Lock()
	keyDown[key] = pressed
	keyMu.Unlock()
}

func CachedInputGetGlobalMousePos(fallback func() mathf.Vec2) mathf.Vec2 {
	if inputSnap.ok {
		return inputSnap.mouse
	}
	return fallback()
}

func CachedInputGetMouseState(id int64, fallback func() bool) bool {
	if inputSnap.ok && id >= 1 && id <= 3 {
		return inputSnap.mouseBits&(1<<uint(id-1)) != 0
	}
	return fallback()
}

func CachedInputGetKey(key int64, fallback func() bool) bool {
	keyMu.RLock()
	pressed, known := keyDown[key]
	keyMu.RUnlock()
	if known {
		return pressed
	}
	return fallback()
}

func CachedInputGetAxis(neg, pos string, fallback func() float64) float64 {
	return cachedAction(actionAxis, neg+"\x00"+pos, func() float64 {
		if value, ok := webActionAxis(neg, pos); ok {
			return value
		}
		return fallback()
	})
}

func CachedInputGetKeyState(key int64, fallback func() int64) int64 {
	if CachedInputGetKey(key, func() bool { return fallback() != 0 }) {
		return 1
	}
	return 0
}

func CachedInputIsActionPressed(action string, fallback func() bool) bool {
	return cachedActionBool("pressed", action, fallback)
}

func CachedInputIsActionJustPressed(action string, fallback func() bool) bool {
	return cachedActionBool("just_pressed", action, fallback)
}

func CachedInputIsActionJustReleased(action string, fallback func() bool) bool {
	return cachedActionBool("just_released", action, fallback)
}

func cachedActionBool(kind, action string, fallback func() bool) bool {
	return cachedAction(actionBool, kind+"\x00"+action, func() bool {
		if value, ok := webActionBool(kind, action); ok {
			return value
		}
		return fallback()
	})
}

// Read unlocked to allow callbacks; discard cache writes if the frame changes.
func cachedAction[T any](values map[string]T, key string, read func() T) T {
	actionMu.Lock()
	frame := actionFrame
	value, ok := values[key]
	actionMu.Unlock()
	if ok {
		return value
	}

	value = read()

	actionMu.Lock()
	if actionFrame == frame {
		values[key] = value
	}
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
