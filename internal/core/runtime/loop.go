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
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
)

func RunEventLoop[T any](me coroutine.Thread, events chan T, handle func(T)) int {
	for {
		var ev T
		engine.WaitForChan(events, &ev)
		handle(ev)
	}
}

type InputFrame struct {
	Point                    mathf.Vec2
	LastMousePos             mathf.Vec2
	LastLeftButtonPressed    bool
	CurrentLeftButtonPressed bool
	MouseEvents              []engine.MouseEvent
	KeyEvents                []engine.KeyEvent
	MouseMovementThreshold   float64
}

type InputFrameHooks struct {
	FireLeftButtonDown func(mathf.Vec2)
	FireLeftButtonUp   func(mathf.Vec2)
	SetMousePos        func(mathf.Vec2)
	OnMouseMove        func(mathf.Vec2)
	OnKeyPressed       func(int64)
}

func ProcessInputFrame(frame InputFrame, hooks InputFrameHooks) (mathf.Vec2, bool) {
	leftPressed := frame.LastLeftButtonPressed
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
	if frame.CurrentLeftButtonPressed != leftPressed {
		if leftPressed {
			hooks.FireLeftButtonUp(frame.Point)
		} else {
			hooks.FireLeftButtonDown(frame.Point)
		}
	}

	lastMousePos := frame.LastMousePos
	dx := frame.Point.X - frame.LastMousePos.X
	dy := frame.Point.Y - frame.LastMousePos.Y
	if math.Abs(dx) > frame.MouseMovementThreshold || math.Abs(dy) > frame.MouseMovementThreshold {
		hooks.SetMousePos(frame.Point)
		hooks.OnMouseMove(frame.Point)
		lastMousePos = frame.Point
	}

	for _, ev := range frame.KeyEvents {
		if ev.IsPressed {
			hooks.OnKeyPressed(ev.Id)
		}
	}

	return lastMousePos, frame.CurrentLeftButtonPressed
}

type InputLoopConfig struct {
	BeginFrame             func() bool
	EndFrame               func()
	CurrentMousePos        func() mathf.Vec2
	IsLeftButtonPressed    func() bool
	FireLeftButtonDown     func(mathf.Vec2)
	FireLeftButtonUp       func(mathf.Vec2)
	SetMousePos            func(mathf.Vec2)
	OnMouseMove            func(mathf.Vec2)
	GetKeyEvents           func([]engine.KeyEvent) []engine.KeyEvent
	OnKeyPressed           func(int64)
	MouseMovementThreshold float64
}

type inputLoopState struct {
	lastLeftButtonPressed bool
	lastMousePos          mathf.Vec2
	keyEvents             []engine.KeyEvent
	wasSuspended          bool
}

func RunInputLoop(me coroutine.Thread, cfg InputLoopConfig) int {
	state := inputLoopState{keyEvents: make([]engine.KeyEvent, 0)}

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

func runInputLoopFrame(cfg InputLoopConfig, state *inputLoopState) {
	if cfg.EndFrame != nil {
		defer cfg.EndFrame()
	}
	point := cfg.CurrentMousePos()
	// Keep the cached mouse position in sync with the engine every frame,
	// so callers don't need a second engine-side mouse query elsewhere.
	cfg.SetMousePos(point)
	state.keyEvents = cfg.GetKeyEvents(state.keyEvents)
	currentLeftButtonPressed := cfg.IsLeftButtonPressed()
	if state.wasSuspended {
		// The suspended consumer owns all edges. Resume from the current held
		// state without manufacturing events at the handoff boundary.
		state.lastMousePos = point
		state.lastLeftButtonPressed = currentLeftButtonPressed
		state.wasSuspended = false
	} else {
		state.lastMousePos, state.lastLeftButtonPressed = ProcessInputFrame(
			InputFrame{
				Point:                    point,
				LastMousePos:             state.lastMousePos,
				LastLeftButtonPressed:    state.lastLeftButtonPressed,
				CurrentLeftButtonPressed: currentLeftButtonPressed,
				KeyEvents:                state.keyEvents,
				MouseMovementThreshold:   cfg.MouseMovementThreshold,
			},
			InputFrameHooks{
				FireLeftButtonDown: cfg.FireLeftButtonDown,
				FireLeftButtonUp:   cfg.FireLeftButtonUp,
				SetMousePos:        cfg.SetMousePos,
				OnMouseMove:        cfg.OnMouseMove,
				OnKeyPressed:       cfg.OnKeyPressed,
			},
		)
	}
	state.keyEvents = state.keyEvents[:0]
}

type LogicFrameConfig[T any] struct {
	Items                    []T
	TempAudios               []string
	TempAnimations           []string
	FlushPendingAudio        func(T, []string) []string
	FlushCompletedAnimations func(T, []string) []string
	NextTimer                func() (float64, bool)
	FireTimer                func(float64)
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

type LogicLoopConfig[T any] struct {
	Items                    func() []T
	FlushPendingAudio        func(T, []string) []string
	FlushCompletedAnimations func(T, []string) []string
	NextTimer                func() (float64, bool)
	FireTimer                func(float64)
	ShowDebugPanel           func()
}

func RunLogicLoop[T any](me coroutine.Thread, cfg LogicLoopConfig[T]) int {
	tempAudios := []string{}
	tempAnimations := []string{}

	for {
		tempAudios, tempAnimations = ProcessLogicFrame(LogicFrameConfig[T]{
			Items:                    cfg.Items(),
			TempAudios:               tempAudios,
			TempAnimations:           tempAnimations,
			FlushPendingAudio:        cfg.FlushPendingAudio,
			FlushCompletedAnimations: cfg.FlushCompletedAnimations,
			NextTimer:                cfg.NextTimer,
			FireTimer:                cfg.FireTimer,
		})
		engine.WaitNextFrame()
		cfg.ShowDebugPanel()
	}
}

func InitLoops(
	create func(coroutine.ThreadObj, func(coroutine.Thread) int) coroutine.Thread,
	eventLoop func(coroutine.Thread) int,
	inputLoop func(coroutine.Thread) int,
	logicLoop func(coroutine.Thread) int,
) {
	if eventLoop != nil {
		create("eventLoop", eventLoop)
	}
	if inputLoop != nil {
		create("inputEventLoop", inputLoop)
	}
	if logicLoop != nil {
		create("logicLoop", logicLoop)
	}
}
