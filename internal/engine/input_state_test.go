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

import "testing"

func TestDiscardPendingKeyEventsDefinesSessionBoundary(t *testing.T) {
	keyMutex.Lock()
	keyStates = make(map[int64]bool)
	cachedKeysDown = nil
	keyMutex.Unlock()
	DiscardPendingKeyEvents()
	onKeyPressed(1)
	cacheKeyEvents()
	onKeyReleased(1)

	DiscardPendingKeyEvents()
	onKeyPressed(2)
	cacheKeyEvents()

	events := GetKeyEvents(nil)
	if len(events) != 1 || events[0].Id != 2 || !events[0].IsPressed {
		t.Fatalf("events after boundary = %+v, want key 2 pressed", events)
	}
}

func TestKeyInputIncludesHeldStateAtUpdateBoundary(t *testing.T) {
	keyMutex.Lock()
	keyStates = make(map[int64]bool)
	cachedKeysDown = nil
	keyMutex.Unlock()
	DiscardPendingKeyEvents()
	t.Cleanup(DiscardPendingKeyEvents)

	onKeyPressed(2)
	onKeyPressed(1)
	cacheKeyEvents()
	onKeyReleased(1)

	events, keysDown := GetKeyInput(nil)
	if len(events) != 2 {
		t.Fatalf("key events = %+v, want two presses", events)
	}
	if len(keysDown) != 2 || keysDown[0] != 1 || keysDown[1] != 2 {
		t.Fatalf("keys down = %+v, want [1 2]", keysDown)
	}

	cacheKeyEvents()
	_, keysDown = GetKeyInput(nil)
	if len(keysDown) != 1 || keysDown[0] != 2 {
		t.Fatalf("keys down after release = %+v, want [2]", keysDown)
	}
}

func TestDiscardPendingKeyEventsPreservesHeldState(t *testing.T) {
	keyMutex.Lock()
	keyStates = make(map[int64]bool)
	cachedKeysDown = nil
	keyMutex.Unlock()
	DiscardPendingKeyEvents()
	t.Cleanup(func() {
		keyMutex.Lock()
		keyStates = make(map[int64]bool)
		cachedKeysDown = nil
		keyMutex.Unlock()
		DiscardPendingKeyEvents()
	})

	onKeyPressed(7)
	DiscardPendingKeyEvents()
	cacheKeyEvents()
	events, keysDown := GetKeyInput(nil)
	if len(events) != 0 {
		t.Fatalf("key events after press boundary = %+v, want none", events)
	}
	if len(keysDown) != 1 || keysDown[0] != 7 {
		t.Fatalf("keys down after press boundary = %+v, want [7]", keysDown)
	}

	onKeyReleased(7)
	DiscardPendingKeyEvents()
	cacheKeyEvents()
	events, keysDown = GetKeyInput(nil)
	if len(events) != 0 {
		t.Fatalf("key events after release boundary = %+v, want none", events)
	}
	if len(keysDown) != 0 {
		t.Fatalf("keys down after release boundary = %+v, want none", keysDown)
	}
}

func TestMouseEventsPreserveShortClickWithinOneInputTick(t *testing.T) {
	resetMouseButtonStates()
	SetMouseEventCaptureEnabled(true)
	t.Cleanup(func() {
		resetMouseButtonStates()
		SetMouseEventCaptureEnabled(false)
	})
	onMousePressed(1)
	onMouseReleased(1)
	cacheMouseEvents()

	events, buttons := GetMouseInput(nil)
	if len(events) != 2 || events[0] != (MouseEvent{Id: 1, IsPressed: true}) ||
		events[1] != (MouseEvent{Id: 1, IsPressed: false}) {
		t.Fatalf("mouse events = %+v, want ordered press/release", events)
	}
	if buttons != 0 {
		t.Fatalf("cached mouse buttons = %#x, want released", buttons)
	}
	if IsMouseButtonPressed(1) {
		t.Fatal("short click left mouse button held after release")
	}
}

func TestMouseEventCaptureSwitchDefinesSessionBoundary(t *testing.T) {
	resetMouseButtonStates()
	SetMouseEventCaptureEnabled(true)
	t.Cleanup(func() {
		resetMouseButtonStates()
		SetMouseEventCaptureEnabled(false)
	})
	onMousePressed(1)
	cacheMouseEvents()
	onMouseReleased(1)

	SetMouseEventCaptureEnabled(false)
	SetMouseEventCaptureEnabled(true)
	onMousePressed(2)
	cacheMouseEvents()

	events := GetMouseEvents(nil)
	if len(events) != 1 || events[0] != (MouseEvent{Id: 2, IsPressed: true}) {
		t.Fatalf("mouse events after boundary = %+v, want button 2 pressed", events)
	}
}

func TestMouseEventsAreNotQueuedOutsideCaptureSession(t *testing.T) {
	resetMouseButtonStates()
	SetMouseEventCaptureEnabled(false)
	t.Cleanup(func() {
		resetMouseButtonStates()
		SetMouseEventCaptureEnabled(false)
	})

	onMousePressed(1)
	onMouseReleased(1)
	cacheMouseEvents()
	events, buttons := GetMouseInput(nil)
	if len(events) != 0 {
		t.Fatalf("mouse events outside capture session = %+v, want none", events)
	}
	if buttons != 0 {
		t.Fatalf("mouse buttons outside capture session = %#x, want released", buttons)
	}
}
