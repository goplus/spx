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
	"testing"

	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestDiscardPendingKeyEventsDefinesSessionBoundary(t *testing.T) {
	keyInput.reset()
	DiscardPendingKeyEvents()
	onKeyPressed(1)
	keyInput.cache()
	onKeyReleased(1)

	DiscardPendingKeyEvents()
	onKeyPressed(2)
	keyInput.cache()

	events, _ := GetKeyInput(nil)
	if len(events) != 1 || events[0].Key != 2 || !events[0].Pressed {
		t.Fatalf("events after boundary = %+v, want key 2 pressed", events)
	}
}

func TestKeyInputIncludesHeldStateAtUpdateBoundary(t *testing.T) {
	keyInput.reset()
	DiscardPendingKeyEvents()
	t.Cleanup(DiscardPendingKeyEvents)

	onKeyPressed(2)
	onKeyPressed(1)
	keyInput.cache()
	onKeyReleased(1)

	events, keysDown := GetKeyInput(nil)
	if len(events) != 2 {
		t.Fatalf("key events = %+v, want two presses", events)
	}
	if len(keysDown) != 2 || keysDown[0] != 1 || keysDown[1] != 2 {
		t.Fatalf("keys down = %+v, want [1 2]", keysDown)
	}

	keyInput.cache()
	_, keysDown = GetKeyInput(nil)
	if len(keysDown) != 1 || keysDown[0] != 2 {
		t.Fatalf("keys down after release = %+v, want [2]", keysDown)
	}
}

func TestInputDrainOwnsSnapshotsAndReusesCallerBuffers(t *testing.T) {
	resetInputState()
	t.Cleanup(resetInputState)
	onKeyPressed(2)
	onKeyPressed(1)
	onMousePressed(1)
	cacheInputState()
	onKeyReleased(2)
	onMouseReleased(1)
	onMousePressed(2)
	cacheInputState()
	// Releases arriving after the cache boundary belong to the next frame.
	onKeyReleased(1)
	onMouseReleased(2)

	keyPrefix := KeyEvent{Key: 9, Pressed: true}
	mousePrefix := MouseEvent{Button: 3, Pressed: true}
	keyEvents, keysDown := GetKeyInput(append(make([]KeyEvent, 0, 8), keyPrefix))
	mouseEvents, buttons := GetMouseInput(append(make([]MouseEvent, 0, 8), mousePrefix))
	wantKeys := []KeyEvent{keyPrefix, {Key: 2, Pressed: true}, {Key: 1, Pressed: true}, {Key: 2}}
	wantMouse := []MouseEvent{mousePrefix, {Button: 1, Pressed: true}, {Button: 1}, {Button: 2, Pressed: true}}
	if !slices.Equal(keyEvents, wantKeys) || !slices.Equal(keysDown, []int64{1}) {
		t.Fatalf("key input = (%v, %v), want (%v, [1])", keyEvents, keysDown, wantKeys)
	}
	if !slices.Equal(mouseEvents, wantMouse) || buttons != 0b010 {
		t.Fatalf("mouse input = (%v, %#x), want (%v, 0b010)", mouseEvents, buttons, wantMouse)
	}

	// A consumer may edit its copies without changing held input or later edges.
	keyEvents[1] = KeyEvent{Key: 8}
	mouseEvents[1] = MouseEvent{Button: 3}
	keysDown[0] = 8
	if events, held := GetKeyInput(nil); len(events) != 0 || !slices.Equal(held, []int64{1}) {
		t.Fatalf("second key drain = (%v, %v), want no edges and [1]", events, held)
	}
	if events, held := GetMouseInput(nil); len(events) != 0 || held != 0b010 {
		t.Fatalf("second mouse drain = (%v, %#x), want no edges and 0b010", events, held)
	}
	previousKeys, previousMouse := slices.Clone(keyEvents), slices.Clone(mouseEvents)
	cacheInputState()
	if !slices.Equal(keyEvents, previousKeys) || !slices.Equal(mouseEvents, previousMouse) || keysDown[0] != 8 {
		t.Fatal("caching the next frame changed previously returned input")
	}

	keyEvents, keysDown = GetKeyInput(keyEvents[:1])
	mouseEvents, buttons = GetMouseInput(mouseEvents[:1])
	wantKeys = []KeyEvent{keyPrefix, {Key: 1}}
	wantMouse = []MouseEvent{mousePrefix, {Button: 2}}
	if !slices.Equal(keyEvents, wantKeys) || len(keysDown) != 0 {
		t.Fatalf("reused key buffer = (%v, %v), want (%v, [])", keyEvents, keysDown, wantKeys)
	}
	if !slices.Equal(mouseEvents, wantMouse) || buttons != 0 {
		t.Fatalf("reused mouse buffer = (%v, %#x), want (%v, 0)", mouseEvents, buttons, wantMouse)
	}
}

func TestDiscardPendingKeyEventsPreservesHeldState(t *testing.T) {
	keyInput.reset()
	DiscardPendingKeyEvents()
	t.Cleanup(func() {
		keyInput.reset()
		DiscardPendingKeyEvents()
	})

	onKeyPressed(7)
	DiscardPendingKeyEvents()
	keyInput.cache()
	events, keysDown := GetKeyInput(nil)
	if len(events) != 0 {
		t.Fatalf("key events after press boundary = %+v, want none", events)
	}
	if len(keysDown) != 1 || keysDown[0] != 7 {
		t.Fatalf("keys down after press boundary = %+v, want [7]", keysDown)
	}

	onKeyReleased(7)
	DiscardPendingKeyEvents()
	keyInput.cache()
	events, keysDown = GetKeyInput(nil)
	if len(events) != 0 {
		t.Fatalf("key events after release boundary = %+v, want none", events)
	}
	if len(keysDown) != 0 {
		t.Fatalf("keys down after release boundary = %+v, want none", keysDown)
	}
}

func TestMouseInputPreservesShortClickForOrdinaryConsumer(t *testing.T) {
	mouseInput.reset()
	t.Cleanup(mouseInput.reset)
	onMousePressed(1)
	onMouseReleased(1)
	mouseInput.cache()

	events, buttons := GetMouseInput(nil)
	if len(events) != 2 || events[0] != (MouseEvent{Button: 1, Pressed: true}) ||
		events[1] != (MouseEvent{Button: 1, Pressed: false}) {
		t.Fatalf("mouse events = %+v, want ordered press/release", events)
	}
	if buttons != 0 {
		t.Fatalf("cached mouse buttons = %#x, want released", buttons)
	}
	if IsMouseButtonPressed(1) {
		t.Fatal("short click left mouse button held after release")
	}
}

func TestMouseInputKeepsLiveAndFrameStatesSeparate(t *testing.T) {
	mouseInput.reset()
	t.Cleanup(mouseInput.reset)
	for _, id := range []int64{-1, 0, 4, 32, 1 << 40} {
		onMousePressed(id)
		if IsMouseButtonPressed(id) {
			t.Errorf("invalid mouse button %d reported held", id)
		}
		onMouseReleased(id)
	}
	onMousePressed(3)
	onMousePressed(3)
	if !IsMouseButtonPressed(3) || AnyMouseButtonPressed() {
		t.Fatal("middle button must be held without a primary button")
	}
	if events, buttons := GetMouseInput(nil); len(events) != 0 || buttons != 0 {
		t.Fatalf("input before frame boundary = (%v, %#x), want empty", events, buttons)
	}

	mouseInput.cache()
	onMouseReleased(3)
	onMouseReleased(3)
	onMousePressed(1)
	onMousePressed(1)
	onMousePressed(2)
	if IsMouseButtonPressed(3) || !IsMouseButtonPressed(1) || !IsMouseButtonPressed(2) || !AnyMouseButtonPressed() {
		t.Fatal("live input must reflect released middle and held left/right buttons")
	}
	events, buttons := GetMouseInput(nil)
	want := []MouseEvent{{Button: 3, Pressed: true}}
	if !slices.Equal(events, want) || buttons != 0b100 {
		t.Fatalf("cached input = (%v, %#x), want (%v, 0b100)", events, buttons, want)
	}

	mouseInput.cache()
	events, buttons = GetMouseInput(nil)
	want = []MouseEvent{{Button: 3}, {Button: 1, Pressed: true}, {Button: 2, Pressed: true}}
	if !slices.Equal(events, want) || buttons != 0b011 {
		t.Fatalf("next frame input = (%v, %#x), want (%v, 0b011)", events, buttons, want)
	}
	onMouseReleased(1)
	if !AnyMouseButtonPressed() {
		t.Fatal("right button must remain held after left release")
	}
	onMouseReleased(2)
	if AnyMouseButtonPressed() {
		t.Fatal("no primary button should remain held")
	}
	mouseInput.cache()
	events, buttons = GetMouseInput(nil)
	want = []MouseEvent{{Button: 1}, {Button: 2}}
	if !slices.Equal(events, want) || buttons != 0 {
		t.Fatalf("released input = (%v, %#x), want (%v, 0)", events, buttons, want)
	}
}

func TestResetInputStateClearsMouseStateAndEvents(t *testing.T) {
	resetInputState()
	t.Cleanup(resetInputState)
	onMousePressed(1)
	mouseInput.cache()
	onMousePressed(2)
	onMousePressed(3)

	resetInputState()
	if IsMouseButtonPressed(1) || IsMouseButtonPressed(2) || IsMouseButtonPressed(3) || AnyMouseButtonPressed() {
		t.Fatal("reset left a mouse button held")
	}
	if events, buttons := GetMouseInput(nil); len(events) != 0 || buttons != 0 {
		t.Fatalf("input after reset = (%v, %#x), want empty", events, buttons)
	}

	mouseInput.cache()
	if events, buttons := GetMouseInput(nil); len(events) != 0 || buttons != 0 {
		t.Fatalf("next frame after reset = (%v, %#x), want empty", events, buttons)
	}
}

func TestMouseInputSnapshotExcludesReleaseAfterCacheBoundary(t *testing.T) {
	mouseInput.reset()
	t.Cleanup(mouseInput.reset)
	onMousePressed(1)
	mouseInput.cache()
	onMouseReleased(1)

	events, buttons := GetMouseInput(nil)
	if len(events) != 1 || events[0] != (MouseEvent{Button: 1, Pressed: true}) || buttons != 1 {
		t.Fatalf("cached input = %v, buttons %#x, want press and left held", events, buttons)
	}
	if IsMouseButtonPressed(1) || AnyMouseButtonPressed() {
		t.Fatal("instantaneous mouse queries did not observe the release")
	}
	if events, buttons := GetMouseInput(nil); len(events) != 0 || buttons != 1 {
		t.Fatalf("second drain = %v, buttons %#x, want consumed edges and unchanged snapshot", events, buttons)
	}
	mouseInput.cache()
	events, buttons = GetMouseInput(nil)
	if len(events) != 1 || events[0] != (MouseEvent{Button: 1}) || buttons != 0 {
		t.Fatalf("next cached input = %v, buttons %#x, want release and no held buttons", events, buttons)
	}
}

func TestDiscardPendingMouseEventsDefinesConsumerBoundary(t *testing.T) {
	mouseInput.reset()
	t.Cleanup(mouseInput.reset)
	onMousePressed(1)
	mouseInput.cache()
	onMouseReleased(1)

	DiscardPendingMouseEvents()
	onMousePressed(2)
	mouseInput.cache()

	events, _ := GetMouseInput(nil)
	if len(events) != 1 || events[0] != (MouseEvent{Button: 2, Pressed: true}) {
		t.Fatalf("mouse events after boundary = %+v, want button 2 pressed", events)
	}
}

func TestDiscardPendingMouseEventsPreservesHeldState(t *testing.T) {
	mouseInput.reset()
	t.Cleanup(mouseInput.reset)

	onMousePressed(1)
	mouseInput.cache()
	DiscardPendingMouseEvents()
	events, buttons := GetMouseInput(nil)
	if len(events) != 0 {
		t.Fatalf("mouse events after boundary = %+v, want none", events)
	}
	if buttons != 1 {
		t.Fatalf("mouse buttons after boundary = %#x, want left held", buttons)
	}
}

func TestInputCallbacksIgnoreLifecycleTransitions(t *testing.T) {
	for _, test := range []struct {
		name  string
		phase gamePhase
	}{
		{"starting", gameStarting},
		{"stopped", gameStopped},
		{"closing", gameClosing},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateGameBinding(t)
			resetInputState()
			t.Cleanup(resetInputState)

			binding, err := bindGameAtPhase(new(bindingTestGame), new(struct{}), test.phase)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { releaseGame(binding) })
			onKeyPressed(7)
			onMousePressed(1)

			cacheInputState()
			keyEvents, keysDown := GetKeyInput(nil)
			mouseEvents, buttons := GetMouseInput(nil)
			if len(keyEvents) != 0 || len(keysDown) != 0 {
				t.Fatalf("transition key state = events %v, held %v; want empty", keyEvents, keysDown)
			}
			if len(mouseEvents) != 0 || buttons != 0 || IsMouseButtonPressed(1) {
				t.Fatalf("transition mouse state = events %v, buttons %#x; want empty", mouseEvents, buttons)
			}
		})
	}
}

func TestTriggerEventsObserveFrameBoundary(t *testing.T) {
	resetTriggerEvents()
	t.Cleanup(resetTriggerEvents)

	firstSrc, firstDst := &Sprite{}, &Sprite{}
	secondSrc, secondDst := &Sprite{}, &Sprite{}
	enqueueTriggerEvent(firstSrc, firstDst)
	if events := GetTriggerEvents(nil); len(events) != 0 {
		t.Fatalf("events before frame boundary = %+v, want none", events)
	}

	cacheTriggerEvents()
	enqueueTriggerEvent(secondSrc, secondDst)
	events := GetTriggerEvents(nil)
	if len(events) != 1 || events[0].Src != firstSrc || events[0].Dst != firstDst {
		t.Fatalf("current-frame events = %+v, want first event", events)
	}
	if events := GetTriggerEvents(nil); len(events) != 0 {
		t.Fatalf("events after drain = %+v, want none", events)
	}

	cacheTriggerEvents()
	events = GetTriggerEvents(nil)
	if len(events) != 1 || events[0].Src != secondSrc || events[0].Dst != secondDst {
		t.Fatalf("next-frame events = %+v, want second event", events)
	}
}

func TestSpriteTriggerIgnoresOtherImplementations(t *testing.T) {
	resetTriggerEvents()
	t.Cleanup(resetTriggerEvents)
	source, target := &Sprite{}, &Sprite{}
	source.OnTriggerEnter(&gdx.Sprite{})
	source.OnTriggerEnter(target)
	cacheTriggerEvents()
	events := GetTriggerEvents(nil)
	if len(events) != 1 || events[0].Src != source || events[0].Dst != target {
		t.Fatalf("trigger events=%v", events)
	}
}
