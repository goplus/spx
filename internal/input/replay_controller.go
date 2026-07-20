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
	"fmt"
	"slices"
)

// InputReplayController resolves one immutable recording or replay stream. A
// Game-owned input session serializes access and resets the controller only at
// the end of that Game lifecycle.
type InputReplayController struct {
	mode      InputSessionMode
	exhausted bool
	next      int64
	elapsed   float64
	cursor    int
	record    InputReplay
	replay    InputReplay
	last      InputReplayFrame
	resolved  bool
}

func (c *InputReplayController) StartRecording(initial InputReplayState, fixedTimestep float64) error {
	if err := validateFiniteNonNegative("fixed timestep", fixedTimestep); err != nil {
		return err
	}
	if err := initial.Validate(); err != nil {
		return fmt.Errorf("initial input state: %w", err)
	}
	if c.modeValue() != InputSessionModeIdle {
		return ErrInputSessionActive
	}
	c.mode = InputSessionModeRecording
	c.record = InputReplay{
		Format:        InputReplayFormat,
		Version:       InputReplayVersion,
		FixedTimestep: fixedTimestep,
		Initial:       cloneInputReplayState(initial),
		Frames:        make([]InputReplayFrame, 0),
	}
	return nil
}

// Recording returns an isolated snapshot without ending the owning session.
func (c *InputReplayController) Recording() (InputReplay, error) {
	if c.mode != InputSessionModeRecording {
		return InputReplay{}, ErrInputSessionNotRecording
	}
	return cloneInputReplay(c.record), nil
}

func (c *InputReplayController) SynchronizeRecordingInitial(initial InputReplayState) error {
	if err := initial.Validate(); err != nil {
		return fmt.Errorf("initial input state: %w", err)
	}
	if c.mode != InputSessionModeRecording {
		return ErrInputSessionNotRecording
	}
	if c.next != 0 {
		return fmt.Errorf("cannot synchronize initial state after recording tick zero")
	}
	c.record.Initial = cloneInputReplayState(initial)
	return nil
}

func (c *InputReplayController) StartReplay(replay InputReplay) error {
	if err := replay.Validate(); err != nil {
		return err
	}
	if c.modeValue() != InputSessionModeIdle {
		return ErrInputSessionActive
	}
	c.mode = InputSessionModeReplaying
	c.replay = cloneInputReplay(replay)
	c.last = InputReplayFrame{Frame: -1, State: cloneInputReplayState(replay.Initial)}
	return nil
}

func (c *InputReplayController) Reset() {
	*c = InputReplayController{}
}

func (c *InputReplayController) Status() InputReplayControllerStatus {
	mode := c.modeValue()
	status := InputReplayControllerStatus{
		Mode:      mode,
		Exhausted: c.exhausted,
		NextFrame: c.next,
	}
	switch mode {
	case InputSessionModeRecording:
		status.FrameCount = len(c.record.Frames)
	case InputSessionModeReplaying:
		status.FrameCount = len(c.replay.Frames)
	}
	return status
}

func (c *InputReplayController) Resolve(
	live InputReplayState,
	keyEvents []InputReplayKeyEvent,
	delta float64,
) (InputReplayFrame, bool, error) {
	return c.ResolveWithMouseEvents(live, nil, keyEvents, delta)
}

func (c *InputReplayController) ResolveWithMouseEvents(
	live InputReplayState,
	mouseEvents []InputReplayMouseEvent,
	keyEvents []InputReplayKeyEvent,
	delta float64,
) (effective InputReplayFrame, firstTick bool, err error) {
	mode := c.modeValue()
	if mode == InputSessionModeReplaying {
		return c.resolveReplay()
	}
	if err := live.Validate(); err != nil {
		return InputReplayFrame{}, false, fmt.Errorf("live input state: %w", err)
	}
	if err := validateFiniteNonNegative("live delta", delta); err != nil {
		return InputReplayFrame{}, false, err
	}
	if mode == InputSessionModeIdle {
		return InputReplayFrame{
			Frame:       -1,
			State:       cloneInputReplayState(live),
			MouseEvents: slices.Clone(mouseEvents),
			KeyEvents:   slices.Clone(keyEvents),
		}, false, nil
	}

	firstTick = c.next == 0
	if !firstTick {
		nextElapsed := c.elapsed + delta
		if !isFinite(nextElapsed) || nextElapsed > maxInputReplayTime {
			return InputReplayFrame{}, false, fmt.Errorf("recorded input time overflow")
		}
		c.elapsed = nextElapsed
	}
	effective = InputReplayFrame{
		Frame:       c.next,
		Time:        c.elapsed,
		State:       cloneInputReplayState(live),
		MouseEvents: slices.Clone(mouseEvents),
		KeyEvents:   slices.Clone(keyEvents),
	}
	c.record.Frames = append(c.record.Frames, cloneInputReplayFrame(effective))
	c.next++
	return cloneInputReplayFrame(effective), firstTick, nil
}

func (c *InputReplayController) modeValue() InputSessionMode {
	if c.mode == "" {
		return InputSessionModeIdle
	}
	return c.mode
}

func (c *InputReplayController) resolveReplay() (InputReplayFrame, bool, error) {
	if c.cursor >= len(c.replay.Frames) {
		c.exhausted = true
		frozen := cloneInputReplayFrame(c.last)
		frozen.MouseEvents = nil
		frozen.KeyEvents = nil
		firstTick := !c.resolved
		c.resolved = true
		return frozen, firstTick, nil
	}

	firstTick := c.cursor == 0
	frame := cloneInputReplayFrame(c.replay.Frames[c.cursor])
	c.cursor++
	c.next = int64(c.cursor)
	c.last = cloneInputReplayFrame(frame)
	c.exhausted = c.cursor == len(c.replay.Frames)
	c.resolved = true
	return frame, firstTick, nil
}

func cloneInputReplay(replay InputReplay) InputReplay {
	cloned := replay
	cloned.Initial = cloneInputReplayState(replay.Initial)
	cloned.Frames = make([]InputReplayFrame, len(replay.Frames))
	for i, frame := range replay.Frames {
		cloned.Frames[i] = cloneInputReplayFrame(frame)
	}
	return cloned
}

func cloneInputReplayFrame(frame InputReplayFrame) InputReplayFrame {
	cloned := frame
	cloned.State = cloneInputReplayState(frame.State)
	cloned.MouseEvents = slices.Clone(frame.MouseEvents)
	cloned.KeyEvents = slices.Clone(frame.KeyEvents)
	return cloned
}

func cloneInputReplayState(state InputReplayState) InputReplayState {
	cloned := state
	cloned.KeysDown = slices.Clone(state.KeysDown)
	return cloned
}
