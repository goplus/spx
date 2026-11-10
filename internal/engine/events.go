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

	"github.com/goplus/spx/v4/internal/base/sliceutil"
	"github.com/goplus/spx/v4/internal/input"
)

type KeyEvent = input.KeyEvent

type MouseEvent = input.MouseEvent

type TriggerEvent struct {
	Src *Sprite
	Dst *Sprite
}

// frameBuffer keeps arriving events separate from events published at a frame
// boundary. Its owner provides synchronization.
type frameBuffer[T any] struct {
	pending []T
	ready   []T
}

type keyInputState struct {
	mu       sync.Mutex
	events   frameBuffer[KeyEvent]
	pressed  map[int64]bool
	keysDown []int64
}

type mouseInputState struct {
	mu            sync.Mutex
	buttons       atomic.Uint32
	events        frameBuffer[MouseEvent]
	cachedButtons uint8
}

type triggerEventQueue struct {
	mu     sync.Mutex
	events frameBuffer[TriggerEvent]
}

var (
	keyInput   = keyInputState{pressed: make(map[int64]bool)}
	mouseInput mouseInputState
)

var triggerEvents triggerEventQueue

// GetKeyInput drains key edges and returns a sorted held-key snapshot.
func GetKeyInput(dst []KeyEvent) ([]KeyEvent, []int64) {
	return keyInput.drainInput(dst)
}

// DiscardPendingKeyEvents starts a clean input-session boundary.
func DiscardPendingKeyEvents() {
	keyInput.discard()
}

// GetMouseInput drains button edges and returns the held-button snapshot.
func GetMouseInput(dst []MouseEvent) ([]MouseEvent, uint8) {
	return mouseInput.drainInput(dst)
}

// DiscardPendingMouseEvents starts a clean input-consumer boundary while
// preserving the current held-button snapshot.
func DiscardPendingMouseEvents() {
	mouseInput.discard()
}

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

func GetTriggerEvents(dst []TriggerEvent) []TriggerEvent {
	return triggerEvents.drain(dst)
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
	keyInput.events.enqueue(KeyEvent{Key: id, Pressed: pressed})
	keyInput.mu.Unlock()
}

func (s *keyInputState) cache() {
	s.mu.Lock()
	if s.events.publish() {
		s.rebuildKeysDownLocked()
	}
	s.mu.Unlock()
}

func (s *keyInputState) drainInput(dst []KeyEvent) ([]KeyEvent, []int64) {
	s.mu.Lock()
	dst = s.events.drain(dst)
	keysDown := append([]int64(nil), s.keysDown...)
	s.mu.Unlock()
	return dst, keysDown
}

func (s *keyInputState) discard() {
	s.mu.Lock()
	s.events.discard()
	s.rebuildKeysDownLocked()
	s.mu.Unlock()
}

func (s *keyInputState) reset() {
	s.mu.Lock()
	s.events.reset()
	s.pressed = make(map[int64]bool)
	s.keysDown = nil
	s.mu.Unlock()
}

func (s *keyInputState) rebuildKeysDownLocked() {
	s.keysDown = s.keysDown[:0]
	for key := range s.pressed {
		s.keysDown = append(s.keysDown, key)
	}
	slices.Sort(s.keysDown)
}

func onMousePressed(id int64) {
	queueMouseEvent(id, true)
}

func onMouseReleased(id int64) {
	queueMouseEvent(id, false)
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
	mouseInput.events.enqueue(MouseEvent{Button: id, Pressed: pressed})
	mouseInput.mu.Unlock()
}

func (s *mouseInputState) cache() {
	s.mu.Lock()
	s.events.publish()
	s.cachedButtons = uint8(s.buttons.Load())
	s.mu.Unlock()
}

func (s *mouseInputState) drainInput(dst []MouseEvent) ([]MouseEvent, uint8) {
	s.mu.Lock()
	dst = s.events.drain(dst)
	buttons := s.cachedButtons
	s.mu.Unlock()
	return dst, buttons
}

func (s *mouseInputState) discard() {
	s.mu.Lock()
	s.events.discard()
	s.cachedButtons = uint8(s.buttons.Load())
	s.mu.Unlock()
}

func (s *mouseInputState) reset() {
	s.mu.Lock()
	s.buttons.Store(0)
	s.events.reset()
	s.cachedButtons = 0
	s.mu.Unlock()
}

func enqueueTriggerEvent(src, dst *Sprite) {
	triggerEvents.mu.Lock()
	triggerEvents.events.enqueue(TriggerEvent{Src: src, Dst: dst})
	triggerEvents.mu.Unlock()
}

func cacheTriggerEvents() {
	triggerEvents.cache()
}

func resetTriggerEvents() {
	triggerEvents.reset()
}

func (q *triggerEventQueue) cache() {
	q.mu.Lock()
	q.events.publish()
	q.mu.Unlock()
}

func (q *triggerEventQueue) drain(dst []TriggerEvent) []TriggerEvent {
	q.mu.Lock()
	dst = q.events.drain(dst)
	q.mu.Unlock()
	return dst
}

func (q *triggerEventQueue) reset() {
	q.mu.Lock()
	q.events.reset()
	q.mu.Unlock()
}

func (b *frameBuffer[T]) enqueue(event T) {
	b.pending = append(b.pending, event)
}

// publish appends to undrained events and reports whether new events arrived.
func (b *frameBuffer[T]) publish() bool {
	changed := len(b.pending) != 0
	b.ready = append(b.ready, b.pending...)
	b.pending = sliceutil.Reset(b.pending, 0)
	return changed
}

func (b *frameBuffer[T]) drain(dst []T) []T {
	dst = append(dst, b.ready...)
	b.ready = sliceutil.Reset(b.ready, 0)
	return dst
}

func (b *frameBuffer[T]) discard() {
	b.pending = sliceutil.Reset(b.pending, 0)
	b.ready = sliceutil.Reset(b.ready, 0)
}

func (b *frameBuffer[T]) reset() {
	b.discard()
	b.pending = nil
	b.ready = nil
}
