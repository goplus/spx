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
	"math"

	inputstate "github.com/goplus/spx/v3/internal/input"
)

const (
	InputReplayFormat  = inputstate.InputReplayFormat
	InputReplayVersion = inputstate.InputReplayVersion
)

type (
	InputReplay           = inputstate.InputReplay
	InputReplayMouse      = inputstate.InputReplayMouse
	InputReplayState      = inputstate.InputReplayState
	InputReplayMouseEvent = inputstate.InputReplayMouseEvent
	InputReplayKeyEvent   = inputstate.InputReplayKeyEvent
	InputReplayFrame      = inputstate.InputReplayFrame
	InputSessionMode      = inputstate.InputSessionMode
	InputSessionPhase     = inputstate.InputSessionPhase
	InputSessionStatus    = inputstate.InputSessionStatus
)

// InputSessionPreparation owns an unclaimed descriptor for the next Game.
// Cancel is safe after the descriptor has been claimed and never affects a
// later preparation.
type InputSessionPreparation struct {
	token uint64
}

// InputSessionOptions configures optional behavior owned by one Game session.
// CaptureKey is disabled when zero.
type InputSessionOptions struct {
	CaptureKey Key
}

// Cancel discards this preparation if no Game has claimed it yet.
func (p InputSessionPreparation) Cancel() bool {
	return cancelPreparedInputSession(p.token)
}

const (
	InputSessionModeIdle      = inputstate.InputSessionModeIdle
	InputSessionModeRecording = inputstate.InputSessionModeRecording
	InputSessionModeReplaying = inputstate.InputSessionModeReplaying
)

const (
	InputSessionPhasePrepared  = inputstate.InputSessionPhasePrepared
	InputSessionPhaseRunning   = inputstate.InputSessionPhaseRunning
	InputSessionPhaseFinishing = inputstate.InputSessionPhaseFinishing
	InputSessionPhaseCompleted = inputstate.InputSessionPhaseCompleted
	InputSessionPhaseAborted   = inputstate.InputSessionPhaseAborted
)

var (
	ErrInputSessionActive       = inputstate.ErrInputSessionActive
	ErrInputSessionNotRecording = inputstate.ErrInputSessionNotRecording
)

// PrepareInputRecording configures recording for the next Game lifecycle.
func PrepareInputRecording(fps float64, options ...InputSessionOptions) (InputSessionPreparation, error) {
	if fps <= 0 || math.IsNaN(fps) || math.IsInf(fps, 0) {
		return InputSessionPreparation{}, fmt.Errorf("input recording FPS must be finite and positive")
	}
	fixedTimestep := 1 / fps
	if fixedTimestep <= 0 || math.IsNaN(fixedTimestep) || math.IsInf(fixedTimestep, 0) {
		return InputSessionPreparation{}, fmt.Errorf("input recording FPS %v produces an invalid fixed timestep", fps)
	}
	option, err := normalizeInputSessionOptions(options)
	if err != nil {
		return InputSessionPreparation{}, err
	}
	return prepareInputRecordingSession(fixedTimestep, option)
}

// FinishInputRecording completes the active Game's recording.
func FinishInputRecording() (InputReplay, error) {
	return finishInputRecordingSession()
}

// FinishInputRecordingJSON completes the recording and returns its cached JSON.
func FinishInputRecordingJSON() (string, error) {
	return finishInputRecordingJSONSession()
}

// PrepareInputReplay configures playback for the next Game lifecycle.
func PrepareInputReplay(replay InputReplay, options ...InputSessionOptions) (InputSessionPreparation, error) {
	option, err := normalizeInputSessionOptions(options)
	if err != nil {
		return InputSessionPreparation{}, err
	}
	return prepareInputReplaySession(replay, option)
}

// GetInputSessionStatus returns the prepared or active input session status.
func GetInputSessionStatus() InputSessionStatus {
	return inputSessionStatus()
}

// EncodeInputReplay validates and encodes replay as JSON.
func EncodeInputReplay(replay InputReplay) (string, error) {
	data, err := inputstate.EncodeInputReplay(replay)
	return string(data), err
}

// DecodeInputReplay decodes and validates replay JSON.
func DecodeInputReplay(data string) (InputReplay, error) {
	return inputstate.DecodeInputReplay([]byte(data))
}

func normalizeInputSessionOptions(options []InputSessionOptions) (InputSessionOptions, error) {
	if len(options) > 1 {
		return InputSessionOptions{}, fmt.Errorf("input session accepts at most one options value")
	}
	if len(options) == 0 {
		return InputSessionOptions{}, nil
	}
	option := options[0]
	if option.CaptureKey < 0 {
		return InputSessionOptions{}, fmt.Errorf("input session capture key must name one concrete key")
	}
	return option, nil
}
