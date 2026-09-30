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

package runtime

import (
	"math"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/engine"
)

type InputFrame struct {
	Point                  mathf.Vec2
	LeftButtonPressed      bool
	MouseEvents            []engine.MouseEvent
	KeyEvents              []engine.KeyEvent
	MouseMovementThreshold float64
}

// InputFrameState is the consumer-local history used to normalize input
// snapshots and ordered edges into one stream of input events.
type InputFrameState struct {
	lastMousePos      mathf.Vec2
	leftButtonPressed bool
}

type InputFrameHooks struct {
	FireLeftButtonDown func(mathf.Vec2)
	FireLeftButtonUp   func(mathf.Vec2)
	OnMouseMove        func(mathf.Vec2)
	OnKeyPressed       func(int64)
}

type InputLoopConfig struct {
	InputFrameHooks
	BeginFrame             func() bool
	CurrentMousePos        func() mathf.Vec2
	SetMousePos            func(mathf.Vec2)
	GetMouseInput          func([]engine.MouseEvent) ([]engine.MouseEvent, uint8)
	GetKeyEvents           func([]engine.KeyEvent) []engine.KeyEvent
	MouseMovementThreshold float64
}

type inputLoopState struct {
	frameState   InputFrameState
	mouseEvents  []engine.MouseEvent
	keyEvents    []engine.KeyEvent
	wasSuspended bool
}

type LogicFrameConfig[T any] struct {
	Items                    []T
	TempAudios               []string
	TempAnimations           []string
	FlushPendingAudio        func(T, []string) []string
	FlushCompletedAnimations func(T, []string) []string
	NextTimer                func() (int64, bool)
	FireTimer                func(int64)
}

func (s *InputFrameState) Reset(point mathf.Vec2, leftButtonPressed bool) {
	*s = InputFrameState{lastMousePos: point, leftButtonPressed: leftButtonPressed}
}

func (s *InputFrameState) Process(frame InputFrame, hooks InputFrameHooks) {
	leftPressed := s.leftButtonPressed
	for _, event := range frame.MouseEvents {
		if event.Id != 1 || event.IsPressed == leftPressed {
			continue
		}
		if event.IsPressed {
			hooks.FireLeftButtonDown(frame.Point)
		} else {
			hooks.FireLeftButtonUp(frame.Point)
		}
		leftPressed = event.IsPressed
	}
	// Old replay files contain only held-state snapshots. This reconciliation
	// also guards against a platform that reports a state change without an edge.
	if frame.LeftButtonPressed != leftPressed {
		if leftPressed {
			hooks.FireLeftButtonUp(frame.Point)
		} else {
			hooks.FireLeftButtonDown(frame.Point)
		}
	}

	dx := frame.Point.X - s.lastMousePos.X
	dy := frame.Point.Y - s.lastMousePos.Y
	if math.Abs(dx) > frame.MouseMovementThreshold || math.Abs(dy) > frame.MouseMovementThreshold {
		hooks.OnMouseMove(frame.Point)
		s.lastMousePos = frame.Point
	}

	for _, ev := range frame.KeyEvents {
		if ev.IsPressed {
			hooks.OnKeyPressed(ev.Id)
		}
	}

	s.leftButtonPressed = frame.LeftButtonPressed
}

func RunInputLoop(cfg InputLoopConfig) {
	state := inputLoopState{}

	for {
		if cfg.BeginFrame != nil && !cfg.BeginFrame() {
			state.wasSuspended = true
			engine.WaitNextFrame()
			continue
		}
		runInputLoopFrame(cfg, &state)
		engine.WaitNextFrame()
	}
}

func ProcessLogicFrame[T any](cfg LogicFrameConfig[T]) ([]string, []string) {
	tempAudios := cfg.TempAudios
	for _, item := range cfg.Items {
		tempAudios = cfg.FlushPendingAudio(item, tempAudios)
	}

	tempAnimations := cfg.TempAnimations
	for _, item := range cfg.Items {
		tempAnimations = cfg.FlushCompletedAnimations(item, tempAnimations)
	}

	for {
		targetTimer, ok := cfg.NextTimer()
		if !ok {
			break
		}
		cfg.FireTimer(targetTimer)
	}
	return tempAudios, tempAnimations
}

func runInputLoopFrame(cfg InputLoopConfig, state *inputLoopState) {
	point := cfg.CurrentMousePos()
	// Keep the cached mouse position in sync with the engine every frame,
	// so callers don't need a second engine-side mouse query elsewhere.
	cfg.SetMousePos(point)
	var buttons uint8
	state.mouseEvents, buttons = cfg.GetMouseInput(state.mouseEvents[:0])
	state.keyEvents = cfg.GetKeyEvents(state.keyEvents[:0])
	leftButtonPressed := buttons&1 != 0
	if state.wasSuspended {
		// The suspended consumer owns all edges. Resume from the current held
		// state without manufacturing events at the handoff boundary.
		state.frameState.Reset(point, leftButtonPressed)
		state.wasSuspended = false
	} else {
		state.frameState.Process(
			InputFrame{
				Point:                  point,
				LeftButtonPressed:      leftButtonPressed,
				MouseEvents:            state.mouseEvents,
				KeyEvents:              state.keyEvents,
				MouseMovementThreshold: cfg.MouseMovementThreshold,
			},
			cfg.InputFrameHooks,
		)
	}
}
