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

package input

import (
	"errors"
	"fmt"
	"math"
)

const (
	InputReplayFormat  = "spx-input-replay"
	InputReplayVersion = 1
	// Keep conversion to Unix nanoseconds below MaxInt64 even after float64
	// rounding. The final fractional second is intentionally excluded.
	maxInputReplayTime = float64(math.MaxInt64 / 1_000_000_000)

	// MaxInputReplayJSONSize bounds untrusted replay payloads before decoding.
	MaxInputReplayJSONSize = 16 << 20
)

var (
	ErrInputSessionActive       = errors.New("input session is already prepared or active")
	ErrInputSessionNotRecording = errors.New("input session is not recording")
)

type InputReplayMouse struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// InputReplayState is the complete user-input state observed on one input tick.
// Buttons uses bits 0, 1, and 2 for left, right, and middle mouse buttons.
type InputReplayState struct {
	Mouse    InputReplayMouse `json:"mouse"`
	Buttons  uint8            `json:"buttons"`
	KeysDown []int64          `json:"keysDown"`
}

type InputReplayKeyEvent struct {
	Key     int64 `json:"key"`
	Pressed bool  `json:"pressed"`
}

type InputReplayMouseEvent struct {
	Button  int64 `json:"button"`
	Pressed bool  `json:"pressed"`
}

type InputReplayFrame struct {
	Frame       int64                   `json:"frame"`
	Time        float64                 `json:"time"`
	State       InputReplayState        `json:"state"`
	MouseEvents []InputReplayMouseEvent `json:"mouseEvents,omitempty"`
	KeyEvents   []InputReplayKeyEvent   `json:"keyEvents,omitempty"`
}

type InputReplay struct {
	Format        string             `json:"format"`
	Version       int                `json:"version"`
	FixedTimestep float64            `json:"fixedTimestep"`
	Initial       InputReplayState   `json:"initial"`
	Frames        []InputReplayFrame `json:"frames"`
}

type InputSessionMode string

const (
	InputSessionModeIdle      InputSessionMode = "idle"
	InputSessionModeRecording InputSessionMode = "recording"
	InputSessionModeReplaying InputSessionMode = "replaying"
)

type InputSessionPhase string

const (
	InputSessionPhasePrepared  InputSessionPhase = "prepared"
	InputSessionPhaseRunning   InputSessionPhase = "running"
	InputSessionPhaseFinishing InputSessionPhase = "finishing"
	InputSessionPhaseCompleted InputSessionPhase = "completed"
	InputSessionPhaseAborted   InputSessionPhase = "aborted"
)

type InputSessionStatus struct {
	Mode      InputSessionMode
	Phase     InputSessionPhase
	Completed bool
	Exhausted bool
	// CurrentTick is the most recently consumed effective input tick.
	CurrentTick int64
	// HasCurrentTick is false until the session consumes its first tick.
	HasCurrentTick bool
	NextFrame      int64
	FrameCount     int
	Error          string
}

type InputReplayControllerStatus struct {
	Mode       InputSessionMode
	Exhausted  bool
	NextFrame  int64
	FrameCount int
}

func (s InputReplayState) Validate() error {
	if !isFinite(s.Mouse.X) || !isFinite(s.Mouse.Y) {
		return fmt.Errorf("mouse coordinates must be finite")
	}
	if s.Buttons&^uint8(0b111) != 0 {
		return fmt.Errorf("mouse buttons contain unsupported bits: %#x", s.Buttons)
	}
	for i, key := range s.KeysDown {
		if key <= 0 {
			return fmt.Errorf("keysDown contains invalid key %d at index %d", key, i)
		}
		if i > 0 && key <= s.KeysDown[i-1] {
			return fmt.Errorf("keysDown must be strictly increasing at index %d", i)
		}
	}
	return nil
}

func (r InputReplay) Validate() error {
	if r.Format != InputReplayFormat {
		return fmt.Errorf("input replay format %q, want %q", r.Format, InputReplayFormat)
	}
	if r.Version != InputReplayVersion {
		return fmt.Errorf("input replay version %d, want %d", r.Version, InputReplayVersion)
	}
	if err := validateFiniteNonNegative("fixed timestep", r.FixedTimestep); err != nil {
		return err
	}
	if err := r.Initial.Validate(); err != nil {
		return fmt.Errorf("initial input state: %w", err)
	}

	previousTime := float64(0)
	buttons := r.Initial.Buttons
	keysDown := make(map[int64]bool, len(r.Initial.KeysDown))
	for _, key := range r.Initial.KeysDown {
		keysDown[key] = true
	}
	for i, frame := range r.Frames {
		if frame.Frame != int64(i) {
			return fmt.Errorf("input replay frame index %d has frame %d", i, frame.Frame)
		}
		if !isFinite(frame.Time) || frame.Time < 0 || frame.Time > maxInputReplayTime {
			return fmt.Errorf("input replay frame %d time must be finite, non-negative, and no greater than %v", i, maxInputReplayTime)
		}
		if i == 0 && frame.Time != 0 {
			return fmt.Errorf("input replay frame 0 time is %v, want 0", frame.Time)
		}
		if i > 0 && frame.Time < previousTime {
			return fmt.Errorf("input replay frame %d time %v precedes %v", i, frame.Time, previousTime)
		}
		if err := frame.State.Validate(); err != nil {
			return fmt.Errorf("input replay frame %d state: %w", i, err)
		}
		if len(frame.MouseEvents) == 0 {
			// Recordings created before ordered mouse edges were added use only
			// per-tick button snapshots. Preserve compatibility with those files.
			buttons = frame.State.Buttons
		} else {
			for eventIndex, event := range frame.MouseEvents {
				if event.Button < 1 || event.Button > 3 {
					return fmt.Errorf("input replay frame %d mouse event %d has invalid button %d", i, eventIndex, event.Button)
				}
				mask := uint8(1 << (event.Button - 1))
				if ((buttons & mask) != 0) == event.Pressed {
					return fmt.Errorf("input replay frame %d mouse event %d does not change button %d", i, eventIndex, event.Button)
				}
				if event.Pressed {
					buttons |= mask
				} else {
					buttons &^= mask
				}
			}
			if buttons != frame.State.Buttons {
				return fmt.Errorf("input replay frame %d mouse events do not match buttons", i)
			}
		}
		for eventIndex, event := range frame.KeyEvents {
			if event.Key <= 0 {
				return fmt.Errorf("input replay frame %d key event %d has invalid key %d", i, eventIndex, event.Key)
			}
			if event.Pressed {
				keysDown[event.Key] = true
			} else {
				delete(keysDown, event.Key)
			}
		}
		if !matchesKeysDown(keysDown, frame.State.KeysDown) {
			return fmt.Errorf("input replay frame %d key events do not match keysDown", i)
		}
		previousTime = frame.Time
	}
	return nil
}

func matchesKeysDown(current map[int64]bool, sorted []int64) bool {
	if len(current) != len(sorted) {
		return false
	}
	for _, key := range sorted {
		if !current[key] {
			return false
		}
	}
	return true
}

func validateFiniteNonNegative(name string, value float64) error {
	if !isFinite(value) || value < 0 {
		return fmt.Errorf("%s must be finite and non-negative", name)
	}
	return nil
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
