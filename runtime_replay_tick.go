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

package spx

import (
	"fmt"
	"slices"
	"time"

	"github.com/goplus/spx/v4/internal/engine"
	inputstate "github.com/goplus/spx/v4/internal/input"
)

type inputMouseState struct {
	position InputReplayMouse
	buttons  uint8
}

type inputSessionTick struct {
	frame     InputReplayFrame
	initial   inputMouseState
	firstTick bool
}

func (s *inputSession) consumeSampledInputTickLocked(
	delta float64,
	sample func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent),
) (inputSessionTick, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase != InputSessionPhaseRunning {
		return inputSessionTick{}, fmt.Errorf("input session cannot consume a tick in phase %q", s.phase)
	}

	live, mouseEvents, keyEvents := sample()
	status := s.controller.Status()
	if s.mode == InputSessionModeRecording && s.initialPending && status.NextFrame == 0 {
		initial := inputStateBeforeEvents(live, mouseEvents, keyEvents)
		if err := s.controller.SynchronizeRecordingInitial(initial); err != nil {
			return inputSessionTick{}, err
		}
		s.initial = inputMouseState{position: initial.Mouse, buttons: initial.Buttons}
		s.initialPending = false
	}

	frame, firstTick, err := s.controller.Resolve(live, mouseEvents, keyEvents, delta)
	if err != nil {
		return inputSessionTick{}, err
	}
	s.current = inputstate.CloneInputReplayState(frame.State)
	s.currentTime = frame.Time
	s.currentTick = frame.Frame
	if s.mode == InputSessionModeReplaying && firstTick && frame.Frame < 0 {
		// An empty replay still owns one synthetic effective tick so its initial
		// state can be observed for a complete rendered frame.
		s.currentTick = 0
	}
	s.hasCurrentTick = true
	if s.mode == InputSessionModeReplaying && s.controller.Status().Exhausted {
		s.phase = InputSessionPhaseFinishing
	}
	return inputSessionTick{
		frame:     frame,
		initial:   s.initial,
		firstTick: firstTick,
	}, nil
}

// inputPlaybackState contains only scalar values, so queries cannot expose the
// session's held-key slice or require an owned copy of it.
type inputPlaybackState struct {
	inputMouseState
	keyPressed bool
}

func (s *inputSession) playbackInput(key Key) (inputPlaybackState, bool) {
	if s == nil {
		return inputPlaybackState{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mode != InputSessionModeReplaying || s.phase == InputSessionPhaseAborted {
		return inputPlaybackState{}, false
	}
	pressed := len(s.current.KeysDown) != 0
	if key != KeyAny {
		_, pressed = slices.BinarySearch(s.current.KeysDown, int64(key))
	}
	return inputPlaybackState{
		inputMouseState: inputMouseState{position: s.current.Mouse, buttons: s.current.Buttons},
		keyPressed:      pressed,
	}, true
}

func (s *inputSession) logicalClock() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Unix(0, int64(s.currentTime*float64(time.Second)))
}

func (p *Game) inputClock() time.Time {
	if session := p.currentInputSession(); session != nil {
		return session.logicalClock()
	}
	return time.Now()
}

// captureInputTick returns a copy of the active game's latest input tick, or nil.
func captureInputTick() *int64 {
	session := activeInputSession()
	if session == nil {
		return nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.hasCurrentTick || session.phase == InputSessionPhaseAborted {
		return nil
	}
	tick := session.currentTick
	return &tick
}

func replayKeyEventsFromEngine(events []engine.KeyEvent) []InputReplayKeyEvent {
	converted := make([]InputReplayKeyEvent, len(events))
	for i, event := range events {
		converted[i] = InputReplayKeyEvent{Key: event.Id, Pressed: event.IsPressed}
	}
	return converted
}

func replayMouseEventsFromEngine(events []engine.MouseEvent) []InputReplayMouseEvent {
	converted := make([]InputReplayMouseEvent, len(events))
	for i, event := range events {
		converted[i] = InputReplayMouseEvent{Button: event.Id, Pressed: event.IsPressed}
	}
	return converted
}

func engineMouseEventsFromReplay(events []InputReplayMouseEvent, reuse []engine.MouseEvent) []engine.MouseEvent {
	reuse = reuse[:0]
	for _, event := range events {
		reuse = append(reuse, engine.MouseEvent{Id: event.Button, IsPressed: event.Pressed})
	}
	return reuse
}

func engineKeyEventsFromReplay(events []InputReplayKeyEvent, reuse []engine.KeyEvent) []engine.KeyEvent {
	reuse = reuse[:0]
	for _, event := range events {
		reuse = append(reuse, engine.KeyEvent{Id: event.Key, IsPressed: event.Pressed})
	}
	return reuse
}

func sortedPressedKeys(keys map[int64]bool) []int64 {
	pressed := make([]int64, 0, len(keys))
	for key, down := range keys {
		if down {
			pressed = append(pressed, key)
		}
	}
	slices.Sort(pressed)
	return pressed
}

// inputStateBeforeEvents reconstructs the state immediately before tick zero's
// ordered edges, preserving a complete press/release pair in that first tick.
func inputStateBeforeEvents(
	live InputReplayState,
	mouseEvents []InputReplayMouseEvent,
	keyEvents []InputReplayKeyEvent,
) InputReplayState {
	initial := live
	buttons := live.Buttons
	for i := len(mouseEvents) - 1; i >= 0; i-- {
		event := mouseEvents[i]
		if event.Button < 1 || event.Button > 3 {
			continue
		}
		mask := uint8(1 << (event.Button - 1))
		if event.Pressed {
			buttons &^= mask
		} else {
			buttons |= mask
		}
	}
	initial.Buttons = buttons

	keys := make(map[int64]bool, len(live.KeysDown))
	for _, key := range live.KeysDown {
		keys[key] = true
	}
	for i := len(keyEvents) - 1; i >= 0; i-- {
		event := keyEvents[i]
		if event.Pressed {
			delete(keys, event.Key)
		} else {
			keys[event.Key] = true
		}
	}
	initial.KeysDown = sortedPressedKeys(keys)
	return initial
}
