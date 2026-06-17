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

func TestMouseButtonPressedStaysVisibleForNextFrame(t *testing.T) {
	resetMouseButtonStates()

	setMouseButtonPressed(1, true)
	setMouseButtonPressed(1, false)
	if !IsMouseButtonPressed(1) {
		t.Fatal("expected quick click to remain visible before first frame")
	}

	beginMouseInputFrame()
	if !IsMouseButtonPressed(1) {
		t.Fatal("expected quick click to remain visible for the next frame")
	}

	beginMouseInputFrame()
	if IsMouseButtonPressed(1) {
		t.Fatal("expected quick click visibility to expire after the next frame")
	}
}

func TestMouseButtonPressedWhileHeld(t *testing.T) {
	resetMouseButtonStates()

	beginMouseInputFrame()
	setMouseButtonPressed(1, true)
	if !IsMouseButtonPressed(1) {
		t.Fatal("expected held mouse button to be pressed")
	}

	beginMouseInputFrame()
	if !IsMouseButtonPressed(1) {
		t.Fatal("expected held mouse button to remain pressed across frames")
	}

	setMouseButtonPressed(1, false)
	beginMouseInputFrame()
	if IsMouseButtonPressed(1) {
		t.Fatal("expected held mouse button to clear immediately after release")
	}
}

func TestMouseButtonPressedForPollingStaysPendingUntilObserved(t *testing.T) {
	resetMouseButtonStates()

	setMouseButtonPressed(1, true)
	setMouseButtonPressed(1, false)

	for range 4 {
		beginMouseInputFrame()
	}
	if !IsMouseButtonPressedForPolling(1) {
		t.Fatal("expected quick click to remain pending until script polling observes it")
	}
	if !IsMouseButtonPressedForPolling(1) {
		t.Fatal("expected observed pending click to stay visible for the rest of the frame")
	}

	beginMouseInputFrame()
	if IsMouseButtonPressedForPolling(1) {
		t.Fatal("expected pending click to clear on the frame after it was observed")
	}
}

func TestMouseButtonPressedForPollingDoesNotReplayAfterHeldRead(t *testing.T) {
	resetMouseButtonStates()

	beginMouseInputFrame()
	setMouseButtonPressed(1, true)
	if !IsMouseButtonPressedForPolling(1) {
		t.Fatal("expected held mouse button to be reported to polling scripts")
	}

	setMouseButtonPressed(1, false)
	beginMouseInputFrame()
	if IsMouseButtonPressedForPolling(1) {
		t.Fatal("expected polling helper not to replay a click already observed while held")
	}
}

func TestGetPollingMousePosUsesPressSnapshotForPendingClick(t *testing.T) {
	resetMouseButtonStates()
	beginMouseInputFrame()

	prevProvider := mouseButtonSnapshotProvider
	mouseButtonSnapshotProvider = func() (float64, float64, bool) {
		return 12, -34, true
	}
	defer func() {
		mouseButtonSnapshotProvider = prevProvider
	}()

	setMouseButtonPressed(1, true)
	setMouseButtonPressed(1, false)

	x, y, ok := GetPollingMousePos()
	if !ok || x != 12 || y != -34 {
		t.Fatalf("GetPollingMousePos() = (%v, %v, %v), want (12, -34, true)", x, y, ok)
	}

	if !IsMouseButtonPressedForPolling(1) {
		t.Fatal("expected pending click to be visible to polling")
	}

	if _, _, ok := GetPollingMousePos(); !ok {
		t.Fatal("expected snapshot to remain visible while the observed frame is still active")
	}

	beginMouseInputFrame()
	if IsMouseButtonPressedForPolling(1) {
		t.Fatal("expected pending click to clear after the observed frame advances")
	}
	if _, _, ok := GetPollingMousePos(); ok {
		t.Fatal("expected snapshot to clear after the observed frame advances")
	}
}

func TestGetMouseButtonPressSnapshotUsesStickyWindow(t *testing.T) {
	resetMouseButtonStates()
	beginMouseInputFrame()

	prevProvider := mouseButtonSnapshotProvider
	mouseButtonSnapshotProvider = func() (float64, float64, bool) {
		return 12, -34, true
	}
	defer func() {
		mouseButtonSnapshotProvider = prevProvider
	}()

	setMouseButtonPressed(1, true)
	setMouseButtonPressed(1, false)

	beginMouseInputFrame()
	x, y, ok := GetMouseButtonPressSnapshot(1)
	if !ok || x != 12 || y != -34 {
		t.Fatalf("GetMouseButtonPressSnapshot(1) = (%v, %v, %v), want (12, -34, true)", x, y, ok)
	}

	beginMouseInputFrame()
	if _, _, ok := GetMouseButtonPressSnapshot(1); ok {
		t.Fatal("expected sticky click snapshot to expire with the sticky pressed state")
	}
}

func TestGetPollingMousePosDoesNotOverrideHeldMousePosition(t *testing.T) {
	resetMouseButtonStates()
	beginMouseInputFrame()

	prevProvider := mouseButtonSnapshotProvider
	mouseButtonSnapshotProvider = func() (float64, float64, bool) {
		return 12, -34, true
	}
	defer func() {
		mouseButtonSnapshotProvider = prevProvider
	}()

	setMouseButtonPressed(1, true)
	if _, _, ok := GetPollingMousePos(); ok {
		t.Fatal("expected held mouse press to keep using live cursor position")
	}
}
