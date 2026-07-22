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
	"sort"

	"github.com/goplus/spbase/mathf"
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
)

// inputSessionInput is the session-local adapter state needed to dispatch
// resolved input through the ordinary SPX event hooks.
type inputSessionInput struct {
	lastMousePos          mathf.Vec2
	lastLeftButtonPressed bool
	mouseEvents           []engine.MouseEvent
	keyEvents             []engine.KeyEvent
}

// processInputSessionTick consumes one effective input frame for the Game-owned
// session. Real input is sampled for recording and drained but ignored during
// replay.
func (p *inputManager) processInputSessionTick(session *inputSession, delta float64) {
	inputEvents, err := p.resolveInputSessionTick(session, delta)
	if err != nil {
		engine.Panic(err)
		return
	}
	p.dispatchInputSessionEvents(inputEvents)
}

func (p *inputManager) resolveInputSessionTick(session *inputSession, delta float64) ([]event, error) {
	session.operationMu.Lock()
	defer session.operationMu.Unlock()
	c := &session.input
	resolved, err := session.consumeSampledInputTickLocked(delta, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
		pointValue := p.g.engine().InputMgr.GetGlobalMousePos()
		var buttons uint8
		c.mouseEvents, buttons = engine.GetMouseInput(c.mouseEvents[:0])
		var keysDown []int64
		c.keyEvents, keysDown = engine.GetKeyInput(c.keyEvents[:0])

		return InputReplayState{
			Mouse: InputReplayMouse{
				X: float64(pointValue.X),
				Y: float64(pointValue.Y),
			},
			Buttons:  buttons,
			KeysDown: keysDown,
		}, replayMouseEventsFromEngine(c.mouseEvents), replayKeyEventsFromEngine(c.keyEvents)
	})
	if err != nil {
		return nil, err
	}
	session.captureConfiguredKeyPresses(resolved.frame.KeyEvents)

	effectivePoint := mathf.Vec2{X: resolved.frame.State.Mouse.X, Y: resolved.frame.State.Mouse.Y}
	effectiveLeftPressed := resolved.frame.State.Buttons&(1<<0) != 0
	if resolved.firstTick {
		p.resetInputSessionDerivedState(session, resolved.initial)
	}
	c.mouseEvents = engineMouseEventsFromReplay(resolved.frame.MouseEvents, c.mouseEvents)
	c.keyEvents = engineKeyEventsFromReplay(resolved.frame.KeyEvents, c.keyEvents)
	p.setMousePos(effectivePoint)
	inputEvents := make([]event, 0, len(c.mouseEvents)+len(c.keyEvents)+3)

	c.lastMousePos, c.lastLeftButtonPressed = coreruntime.ProcessInputFrame(
		coreruntime.InputFrame{
			Point:                    effectivePoint,
			LastMousePos:             c.lastMousePos,
			LastLeftButtonPressed:    c.lastLeftButtonPressed,
			CurrentLeftButtonPressed: effectiveLeftPressed,
			MouseEvents:              c.mouseEvents,
			KeyEvents:                c.keyEvents,
			MouseMovementThreshold:   mouseMovementThreshold,
		},
		coreruntime.InputFrameHooks{
			FireLeftButtonDown: func(point mathf.Vec2) {
				inputEvents = append(inputEvents, &eventLeftButtonDown{Pos: point})
			},
			FireLeftButtonUp: func(point mathf.Vec2) {
				inputEvents = append(inputEvents, &eventLeftButtonUp{Pos: point})
			},
			SetMousePos: p.setMousePos,
			OnMouseMove: func(point mathf.Vec2) {
				inputEvents = append(inputEvents, &eventMouseMove{Pos: point})
			},
			OnKeyPressed: func(keyID int64) {
				inputEvents = append(inputEvents, &eventKeyDown{Key: Key(keyID)})
			},
		},
	)
	c.clearEvents()
	return inputEvents, nil
}

func (s *inputSession) captureConfiguredKeyPresses(events []InputReplayKeyEvent) {
	if s.captureKey == 0 {
		return
	}
	for _, event := range events {
		if event.Pressed && Key(event.Key) == s.captureKey {
			Snapshot("", nil)
		}
	}
}

func (c *inputSessionInput) clearEvents() {
	c.mouseEvents = c.mouseEvents[:0]
	c.keyEvents = c.keyEvents[:0]
}

func (p *inputManager) dispatchInputSessionEvents(events []event) {
	if len(events) == 0 {
		return
	}
	dispatch := func() {
		for _, event := range events {
			p.g.handleEvent(event)
		}
	}
	if gco == nil {
		dispatch()
		return
	}
	gco.CreateAndStart(false, p.g, func(coroutine.Thread) int {
		dispatch()
		return 0
	})
}

func (p *inputManager) resetInputSessionDerivedState(session *inputSession, state InputReplayState) {
	point := mathf.Vec2{X: state.Mouse.X, Y: state.Mouse.Y}
	p.mousePos = point
	session.input.lastMousePos = point
	session.input.lastLeftButtonPressed = state.Buttons&(1<<0) != 0
	p.clickGate.InitWithClock(mouseClickInterval, p.g.inputClock)
	p.swipe.InitWithClock(p.g.inputClock)
}

func (p *inputManager) effectiveKeyPressed(key Key) bool {
	if state, replaying := p.g.currentInputPlaybackState(); replaying {
		if key == KeyAny {
			return len(state.KeysDown) != 0
		}
		keyID := int64(key)
		index := sort.Search(len(state.KeysDown), func(i int) bool {
			return state.KeysDown[i] >= keyID
		})
		return index < len(state.KeysDown) && state.KeysDown[index] == keyID
	}
	return p.g.engine().InputMgr.GetKey(int64(key))
}

func (p *inputManager) effectiveMousePos() mathf.Vec2 {
	if state, replaying := p.g.currentInputPlaybackState(); replaying {
		return mathf.Vec2{X: state.Mouse.X, Y: state.Mouse.Y}
	}
	return p.currentMousePos()
}

func (p *inputManager) effectiveMousePressed() bool {
	if state, replaying := p.g.currentInputPlaybackState(); replaying {
		return state.Buttons&0b011 != 0
	}
	return engine.AnyMouseButtonPressed()
}
