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
	"sort"
	"time"

	"github.com/goplus/spx/v3/internal/engine"
)

type inputSessionTick struct {
	frame     InputReplayFrame
	initial   InputReplayState
	firstTick bool
}

func consumeInputTick(live InputReplayState, keyEvents []InputReplayKeyEvent, delta float64) (inputSessionTick, error) {
	return consumeInputTickWithMouseEvents(live, nil, keyEvents, delta)
}

func consumeInputTickWithMouseEvents(
	live InputReplayState,
	mouseEvents []InputReplayMouseEvent,
	keyEvents []InputReplayKeyEvent,
	delta float64,
) (inputSessionTick, error) {
	session := activeInputSession()
	if session == nil {
		return inputSessionTick{}, fmt.Errorf("no active input session")
	}
	return session.consumeSampledInputTick(delta, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
		return live, mouseEvents, keyEvents
	})
}

// consumeSampledInputTick holds the session boundary while the engine state is
// sampled and resolved into one effective input tick.
func (s *inputSession) consumeSampledInputTick(
	delta float64,
	sample func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent),
) (inputSessionTick, error) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	return s.consumeSampledInputTickLocked(delta, sample)
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
		s.initial = cloneInputReplayState(initial)
		s.initialPending = false
	}

	frame, firstTick, err := s.controller.ResolveWithMouseEvents(live, mouseEvents, keyEvents, delta)
	if err != nil {
		return inputSessionTick{}, err
	}
	s.current = cloneInputReplayState(frame.State)
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
		initial:   cloneInputReplayState(s.initial),
		firstTick: firstTick,
	}, nil
}

func (s *inputSession) playbackState() (InputReplayState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mode != InputSessionModeReplaying || s.phase == InputSessionPhaseAborted {
		return InputReplayState{}, false
	}
	return cloneInputReplayState(s.current), true
}

func (p *Game) currentInputPlaybackState() (InputReplayState, bool) {
	session := p.currentInputSession()
	if session == nil {
		return InputReplayState{}, false
	}
	return session.playbackState()
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

// currentInputSessionTickForCapture returns the latest tick owned by the active
// Game lifecycle. A new Game can never inherit the previous run's tick.
func currentInputSessionTickForCapture() (int64, bool) {
	session := activeInputSession()
	if session == nil {
		return 0, false
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.hasCurrentTick || session.phase == InputSessionPhaseAborted {
		return 0, false
	}
	return session.currentTick, true
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
	sort.Slice(pressed, func(i, j int) bool { return pressed[i] < pressed[j] })
	return pressed
}

// inputStateBeforeEvents reconstructs the state immediately before tick zero's
// ordered edges, preserving a complete press/release pair in that first tick.
func inputStateBeforeEvents(
	live InputReplayState,
	mouseEvents []InputReplayMouseEvent,
	keyEvents []InputReplayKeyEvent,
) InputReplayState {
	initial := cloneInputReplayState(live)
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

func cloneInputReplayState(state InputReplayState) InputReplayState {
	cloned := state
	cloned.KeysDown = append([]int64(nil), state.KeysDown...)
	return cloned
}
