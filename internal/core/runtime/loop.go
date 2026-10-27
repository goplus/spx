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
	"github.com/goplus/spx/v4/internal/input"
)

type InputFrame struct {
	Point                  mathf.Vec2
	LeftButtonPressed      bool
	MouseEvents            []input.MouseEvent
	KeyEvents              []input.KeyEvent
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

func (s *InputFrameState) Reset(point mathf.Vec2, leftButtonPressed bool) {
	*s = InputFrameState{lastMousePos: point, leftButtonPressed: leftButtonPressed}
}

func (s *InputFrameState) Process(frame InputFrame, hooks InputFrameHooks) {
	leftPressed := s.leftButtonPressed
	for _, event := range frame.MouseEvents {
		if event.Id != input.MouseButtonLeft || event.IsPressed == leftPressed {
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
