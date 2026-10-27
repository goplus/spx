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

	"github.com/goplus/spx/v4/internal/input"
)

type KeyEvent = input.KeyEvent
type MouseEvent = input.MouseEvent

type keyInputState struct {
	mu       sync.Mutex
	pending  []KeyEvent
	ready    []KeyEvent
	pressed  map[int64]bool
	keysDown []int64
}

type mouseInputState struct {
	mu            sync.Mutex
	buttons       atomic.Uint32
	pending       []MouseEvent
	ready         []MouseEvent
	cachedButtons uint8
}

var (
	keyInput   = keyInputState{pressed: make(map[int64]bool)}
	mouseInput mouseInputState
)

// IsMouseButtonPressed reports whether the button is held.
func IsMouseButtonPressed(id int64) bool {
	if id < input.MouseButtonLeft || id > input.MouseButtonMiddle {
		return false
	}
	return mouseInput.buttons.Load()&(1<<(id-1)) != 0
}

// AnyMouseButtonPressed reports whether a primary mouse button is held.
func AnyMouseButtonPressed() bool {
	return mouseInput.buttons.Load()&0b011 != 0
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

// DiscardPendingKeyEvents starts a clean input-session boundary.
func DiscardPendingKeyEvents() {
	keyInput.discard()
}

// DiscardPendingMouseEvents starts a clean input-consumer boundary while
// preserving the current held-button snapshot.
func DiscardPendingMouseEvents() {
	mouseInput.discard()
}

func (s *keyInputState) reset() {
	s.mu.Lock()
	s.pending = nil
	s.ready = nil
	s.pressed = make(map[int64]bool)
	s.keysDown = nil
	s.mu.Unlock()
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

func (s *mouseInputState) reset() {
	s.mu.Lock()
	s.buttons.Store(0)
	s.pending = nil
	s.ready = nil
	s.cachedButtons = 0
	s.mu.Unlock()
}

func (s *mouseInputState) drainInput(dst []MouseEvent) ([]MouseEvent, uint8) {
	s.mu.Lock()
	dst = append(dst, s.ready...)
	s.ready = s.ready[:0]
	buttons := s.cachedButtons
	s.mu.Unlock()
	return dst, buttons
}

func (s *mouseInputState) discard() {
	s.mu.Lock()
	s.pending = s.pending[:0]
	s.ready = s.ready[:0]
	s.cachedButtons = uint8(s.buttons.Load())
	s.mu.Unlock()
}

func (s *mouseInputState) cache() {
	s.mu.Lock()
	s.ready = append(s.ready, s.pending...)
	s.pending = s.pending[:0]
	s.cachedButtons = uint8(s.buttons.Load())
	s.mu.Unlock()
}

// resetInputState clears process-wide input state at a game lifecycle boundary.
func resetInputState() {
	keyInput.reset()
	mouseInput.reset()
}

func cacheInputState() {
	keyInput.cache()
	mouseInput.cache()
}

func onKeyPressed(id int64) {
	queueKeyEvent(id, true)
}

func onKeyReleased(id int64) {
	queueKeyEvent(id, false)
}

func onMousePressed(id int64) {
	queueMouseEvent(id, true)
}

func onMouseReleased(id int64) {
	queueMouseEvent(id, false)
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

func queueMouseEvent(id int64, pressed bool) {
	if !acceptsRuntimeWork() || id < input.MouseButtonLeft || id > input.MouseButtonMiddle {
		return
	}
	mouseInput.mu.Lock()
	buttons := mouseInput.buttons.Load()
	mask := uint32(1) << (id - 1)
	if (buttons&mask != 0) == pressed {
		mouseInput.mu.Unlock()
		return
	}
	if pressed {
		buttons |= mask
	} else {
		buttons &^= mask
	}
	mouseInput.buttons.Store(buttons)
	mouseInput.pending = append(mouseInput.pending, MouseEvent{Id: id, IsPressed: pressed})
	mouseInput.mu.Unlock()
}
