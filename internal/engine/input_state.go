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
	"sort"
	"sync"
	"sync/atomic"
)

type KeyEvent struct {
	Id        int64
	IsPressed bool
}

type MouseEvent struct {
	Id        int64
	IsPressed bool
}

// IsMouseButtonPressed reports whether the button is held.
func IsMouseButtonPressed(id int64) bool {
	if id < 0 || id >= int64(len(mouseButtonStates)) {
		return false
	}
	return atomic.LoadUint32(&mouseButtonStates[id]) != 0
}

// AnyMouseButtonPressed reports whether a primary mouse button is held.
func AnyMouseButtonPressed() bool {
	return IsMouseButtonPressed(1) || IsMouseButtonPressed(2)
}

// GetKeyEvents drains the ordered key edges for the current update.
func GetKeyEvents(dst []KeyEvent) []KeyEvent {
	keyMutex.Lock()
	dst = append(dst, keyEvents...)
	keyEvents = keyEvents[:0]
	keyMutex.Unlock()
	return dst
}

// GetKeyInput drains key edges and returns a sorted held-key snapshot.
func GetKeyInput(dst []KeyEvent) ([]KeyEvent, []int64) {
	keyMutex.Lock()
	dst = append(dst, keyEvents...)
	keyEvents = keyEvents[:0]
	keysDown := append([]int64(nil), cachedKeysDown...)
	keyMutex.Unlock()
	return dst, keysDown
}

// GetMouseInput drains button edges and returns the held-button snapshot.
func GetMouseInput(dst []MouseEvent) ([]MouseEvent, uint8) {
	mouseMutex.Lock()
	dst = append(dst, mouseEvents...)
	mouseEvents = mouseEvents[:0]
	buttons := cachedMouseButtons
	mouseMutex.Unlock()
	return dst, buttons
}

// GetMouseEvents drains the ordered mouse-button edges for the current update.
func GetMouseEvents(dst []MouseEvent) []MouseEvent {
	events, _ := GetMouseInput(dst)
	return events
}

// DiscardPendingKeyEvents starts a clean input-session boundary.
func DiscardPendingKeyEvents() {
	keyMutex.Lock()
	keyEventsTemp = keyEventsTemp[:0]
	keyEvents = keyEvents[:0]
	rebuildCachedKeysDownLocked()
	keyMutex.Unlock()
}

// SetMouseEventCaptureEnabled switches ordered mouse-edge capture at a clean boundary.
func SetMouseEventCaptureEnabled(enabled bool) {
	mouseMutex.Lock()
	mouseEventsTemp = mouseEventsTemp[:0]
	mouseEvents = mouseEvents[:0]
	cachedMouseButtons = currentMouseButtons()
	mouseEventCaptureEnabled = enabled
	mouseMutex.Unlock()
}

var (
	keyEventsTemp  []KeyEvent
	keyEvents      []KeyEvent
	keyStates      = make(map[int64]bool)
	cachedKeysDown []int64
	keyMutex       sync.Mutex

	mouseButtonStates        [4]uint32
	mouseEventsTemp          []MouseEvent
	mouseEvents              []MouseEvent
	cachedMouseButtons       uint8
	mouseEventCaptureEnabled bool
	mouseMutex               sync.Mutex
)

func resetInputState() {
	resetMouseButtonStates()
	keyMutex.Lock()
	keyEventsTemp = make([]KeyEvent, 0)
	keyEvents = make([]KeyEvent, 0)
	keyStates = make(map[int64]bool)
	cachedKeysDown = nil
	keyMutex.Unlock()
	mouseMutex.Lock()
	mouseEventsTemp = make([]MouseEvent, 0)
	mouseEvents = make([]MouseEvent, 0)
	cachedMouseButtons = 0
	mouseMutex.Unlock()
}

func onKeyPressed(id int64) {
	keyMutex.Lock()
	keyStates[id] = true
	keyEventsTemp = append(keyEventsTemp, KeyEvent{Id: id, IsPressed: true})
	keyMutex.Unlock()
}

func onKeyReleased(id int64) {
	keyMutex.Lock()
	delete(keyStates, id)
	keyEventsTemp = append(keyEventsTemp, KeyEvent{Id: id, IsPressed: false})
	keyMutex.Unlock()
}

func onMousePressed(id int64) {
	queueMouseEvent(id, true)
}

func onMouseReleased(id int64) {
	queueMouseEvent(id, false)
}

func queueMouseEvent(id int64, pressed bool) {
	if id < 1 || id > 3 {
		return
	}
	mouseMutex.Lock()
	if IsMouseButtonPressed(id) == pressed {
		mouseMutex.Unlock()
		return
	}
	setMouseButtonPressed(id, pressed)
	if mouseEventCaptureEnabled {
		mouseEventsTemp = append(mouseEventsTemp, MouseEvent{Id: id, IsPressed: pressed})
	}
	mouseMutex.Unlock()
}

func cacheKeyEvents() {
	keyMutex.Lock()
	keyEvents = append(keyEvents, keyEventsTemp...)
	keyStateChanged := len(keyEventsTemp) != 0
	keyEventsTemp = keyEventsTemp[:0]
	if keyStateChanged {
		rebuildCachedKeysDownLocked()
	}
	keyMutex.Unlock()
}

func rebuildCachedKeysDownLocked() {
	cachedKeysDown = cachedKeysDown[:0]
	for key := range keyStates {
		cachedKeysDown = append(cachedKeysDown, key)
	}
	sort.Slice(cachedKeysDown, func(i, j int) bool { return cachedKeysDown[i] < cachedKeysDown[j] })
}

func cacheMouseEvents() {
	mouseMutex.Lock()
	mouseEvents = append(mouseEvents, mouseEventsTemp...)
	mouseEventsTemp = mouseEventsTemp[:0]
	cachedMouseButtons = currentMouseButtons()
	mouseMutex.Unlock()
}

func currentMouseButtons() uint8 {
	var buttons uint8
	for id := int64(1); id <= 3; id++ {
		if IsMouseButtonPressed(id) {
			buttons |= 1 << (id - 1)
		}
	}
	return buttons
}

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
