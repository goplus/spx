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
	"slices"
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

type keyInputState struct {
	mu       sync.Mutex
	pending  []KeyEvent
	ready    []KeyEvent
	pressed  map[int64]bool
	keysDown []int64
}

type mouseInputState struct {
	mu             sync.Mutex
	buttons        [4]atomic.Bool
	pending        []MouseEvent
	ready          []MouseEvent
	cachedButtons  uint8
	captureEnabled bool
}

var (
	keyInput   = keyInputState{pressed: make(map[int64]bool)}
	mouseInput mouseInputState
)

// IsMouseButtonPressed reports whether the button is held.
func IsMouseButtonPressed(id int64) bool {
	if id < 0 || id >= int64(len(mouseInput.buttons)) {
		return false
	}
	return mouseInput.buttons[id].Load()
}

// AnyMouseButtonPressed reports whether a primary mouse button is held.
func AnyMouseButtonPressed() bool {
	return IsMouseButtonPressed(1) || IsMouseButtonPressed(2)
}

// GetKeyEvents drains the ordered key edges for the current update.
func GetKeyEvents(dst []KeyEvent) []KeyEvent {
	return keyInput.drain(dst)
}

// GetKeyInput drains key edges and returns a sorted held-key snapshot.
func GetKeyInput(dst []KeyEvent) ([]KeyEvent, []int64) {
	return keyInput.drainInput(dst)
}

// GetMouseInput drains button edges and returns the held-button snapshot.
func GetMouseInput(dst []MouseEvent) ([]MouseEvent, uint8) {
	return mouseInput.drainInput(dst)
}

// GetMouseEvents drains the ordered mouse-button edges for the current update.
func GetMouseEvents(dst []MouseEvent) []MouseEvent {
	events, _ := GetMouseInput(dst)
	return events
}

// DiscardPendingKeyEvents starts a clean input-session boundary.
func DiscardPendingKeyEvents() {
	keyInput.discard()
}

// SetMouseEventCaptureEnabled switches ordered mouse-edge capture at a clean boundary.
func SetMouseEventCaptureEnabled(enabled bool) {
	mouseInput.setCaptureEnabled(enabled)
}

// ResetInputState clears process-wide input state at a game lifecycle boundary.
func ResetInputState() {
	keyInput.reset()
	mouseInput.reset()
}

func onKeyPressed(id int64) {
	queueKeyEvent(id, true)
}

func onKeyReleased(id int64) {
	queueKeyEvent(id, false)
}

func queueKeyEvent(id int64, pressed bool) {
	if !acceptsRuntimeWork() {
		return
	}
	keyInput.mu.Lock()
	if pressed {
		keyInput.pressed[id] = true
	} else {
		delete(keyInput.pressed, id)
	}
	keyInput.pending = append(keyInput.pending, KeyEvent{Id: id, IsPressed: pressed})
	keyInput.mu.Unlock()
}

func onMousePressed(id int64) {
	queueMouseEvent(id, true)
}

func onMouseReleased(id int64) {
	queueMouseEvent(id, false)
}

func queueMouseEvent(id int64, pressed bool) {
	if !acceptsRuntimeWork() || id < 1 || id >= int64(len(mouseInput.buttons)) {
		return
	}
	mouseInput.mu.Lock()
	if mouseInput.buttons[id].Load() == pressed {
		mouseInput.mu.Unlock()
		return
	}
	mouseInput.buttons[id].Store(pressed)
	if mouseInput.captureEnabled {
		mouseInput.pending = append(mouseInput.pending, MouseEvent{Id: id, IsPressed: pressed})
	}
	mouseInput.mu.Unlock()
}

func cacheKeyEvents() {
	keyInput.cache()
}

func cacheMouseEvents() {
	mouseInput.cache()
}

func resetMouseButtonStates() {
	mouseInput.mu.Lock()
	mouseInput.resetButtonsLocked()
	mouseInput.mu.Unlock()
}

func (s *keyInputState) drain(dst []KeyEvent) []KeyEvent {
	s.mu.Lock()
	dst = append(dst, s.ready...)
	s.ready = s.ready[:0]
	s.mu.Unlock()
	return dst
}

func (s *keyInputState) drainInput(dst []KeyEvent) ([]KeyEvent, []int64) {
	s.mu.Lock()
	dst = append(dst, s.ready...)
	s.ready = s.ready[:0]
	keysDown := append([]int64(nil), s.keysDown...)
	s.mu.Unlock()
	return dst, keysDown
}

func (s *keyInputState) discard() {
	s.mu.Lock()
	s.pending = s.pending[:0]
	s.ready = s.ready[:0]
	s.rebuildKeysDownLocked()
	s.mu.Unlock()
}

func (s *keyInputState) reset() {
	s.mu.Lock()
	s.pending = nil
	s.ready = nil
	s.pressed = make(map[int64]bool)
	s.keysDown = nil
	s.mu.Unlock()
}

func (s *keyInputState) cache() {
	s.mu.Lock()
	s.ready = append(s.ready, s.pending...)
	changed := len(s.pending) != 0
	s.pending = s.pending[:0]
	if changed {
		s.rebuildKeysDownLocked()
	}
	s.mu.Unlock()
}

func (s *keyInputState) rebuildKeysDownLocked() {
	s.keysDown = s.keysDown[:0]
	for key := range s.pressed {
		s.keysDown = append(s.keysDown, key)
	}
	slices.Sort(s.keysDown)
}

func (s *mouseInputState) drainInput(dst []MouseEvent) ([]MouseEvent, uint8) {
	s.mu.Lock()
	dst = append(dst, s.ready...)
	s.ready = s.ready[:0]
	buttons := s.cachedButtons
	s.mu.Unlock()
	return dst, buttons
}

func (s *mouseInputState) setCaptureEnabled(enabled bool) {
	s.mu.Lock()
	s.pending = s.pending[:0]
	s.ready = s.ready[:0]
	s.cachedButtons = s.buttonMask()
	s.captureEnabled = enabled
	s.mu.Unlock()
}

func (s *mouseInputState) reset() {
	s.mu.Lock()
	s.resetButtonsLocked()
	s.pending = nil
	s.ready = nil
	s.cachedButtons = 0
	s.mu.Unlock()
}

func (s *mouseInputState) cache() {
	s.mu.Lock()
	s.ready = append(s.ready, s.pending...)
	s.pending = s.pending[:0]
	s.cachedButtons = s.buttonMask()
	s.mu.Unlock()
}

func (s *mouseInputState) buttonMask() uint8 {
	var buttons uint8
	for id := 1; id < len(s.buttons); id++ {
		if s.buttons[id].Load() {
			buttons |= 1 << (id - 1)
		}
	}
	return buttons
}

func (s *mouseInputState) resetButtonsLocked() {
	for i := range s.buttons {
		s.buttons[i].Store(false)
	}
}
