/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"github.com/goplus/spbase/mathf"
	coreruntime "github.com/goplus/spx/v4/internal/core/runtime"
	"github.com/goplus/spx/v4/internal/engine"
	itime "github.com/goplus/spx/v4/internal/time"
)

// Sampling runs between script slices; pending is consumed on the engine thread.
type inputFrameState struct {
	history     coreruntime.InputFrameState
	mouseEvents []engine.MouseEvent
	keyEvents   []engine.KeyEvent
	hadSession  bool
	pending     *inputFrame
}

type inputFrame struct {
	events   []event
	captures int
}

// Frame routing is runtime work; stopping game scripts must not discard input.
type frameEventDispatcher struct{}

// prepareInputFrame freezes input before condition sampling and clock advancement.
func (p *inputManager) prepareInputFrame(delta float64) bool {
	c := &p.frame
	c.pending = nil
	session := p.g.currentInputSession()
	var state InputReplayState
	var captureKey Key
	if session == nil {
		state = p.sampleInputState()
	} else {
		if !session.beginFrame() {
			return false
		}
		var err error
		state, err = p.resolveInputSessionState(session, delta)
		if err != nil {
			session.endFrame()
			engine.Panic(err)
			return false
		}
		captureKey = session.captureKey
	}
	point := mathf.Vec2{X: state.Mouse.X, Y: state.Mouse.Y}
	if session == nil && c.hadSession {
		// The previous session owns its input edges. Resume from held state.
		c.history.Reset(point, state.Buttons&1 != 0)
		c.mouseEvents, c.keyEvents = c.mouseEvents[:0], c.keyEvents[:0]
	}
	c.hadSession = session != nil
	p.mousePos = point
	batch := &inputFrame{}
	if edges := len(c.mouseEvents) + len(c.keyEvents); edges > 0 {
		batch.events = make([]event, 0, edges+3)
	}
	c.history.Process(coreruntime.InputFrame{
		Point:                  point,
		LeftButtonPressed:      state.Buttons&1 != 0,
		MouseEvents:            c.mouseEvents,
		KeyEvents:              c.keyEvents,
		MouseMovementThreshold: mouseMovementThreshold,
	}, batch.eventHooks(captureKey))
	c.pending = batch
	return true
}

// sampleInputState drains live input into the shared edge buffers.
func (p *inputManager) sampleInputState() InputReplayState {
	c := &p.frame
	point := engine.Managers().InputMgr.GetGlobalMousePos()
	var buttons uint8
	c.mouseEvents, buttons = engine.GetMouseInput(c.mouseEvents[:0])
	var keysDown []int64
	c.keyEvents, keysDown = engine.GetKeyInput(c.keyEvents[:0])
	return InputReplayState{
		Mouse:    InputReplayMouse{X: point.X, Y: point.Y},
		Buttons:  buttons,
		KeysDown: keysDown,
	}
}

func (p *inputManager) resolveInputSessionState(session *inputSession, delta float64) (InputReplayState, error) {
	session.operationMu.Lock()
	defer session.operationMu.Unlock()
	c := &p.frame
	resolved, err := session.consumeSampledInputTickLocked(delta, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
		state := p.sampleInputState()
		return state, replayMouseEventsFromEngine(c.mouseEvents), replayKeyEventsFromEngine(c.keyEvents)
	})
	if err != nil {
		return InputReplayState{}, err
	}
	if resolved.firstTick {
		point := mathf.Vec2{X: resolved.initial.position.X, Y: resolved.initial.position.Y}
		c.history.Reset(point, resolved.initial.buttons&1 != 0)
		p.clickGate.InitWithClock(mouseClickInterval, p.g.inputClock)
		p.swipe.InitWithClock(p.g.inputClock)
	}
	c.mouseEvents = engineMouseEventsFromReplay(resolved.frame.MouseEvents, c.mouseEvents)
	c.keyEvents = engineKeyEventsFromReplay(resolved.frame.KeyEvents, c.keyEvents)
	return resolved.frame.State, nil
}

func (p *inputManager) effectiveKeyPressed(key Key) bool {
	if state, replaying := p.g.currentInputSession().playbackInput(key); replaying {
		return state.keyPressed
	}
	return engine.Managers().InputMgr.GetKey(int64(key))
}

func (p *inputManager) effectiveMousePos() mathf.Vec2 {
	if mouse, replaying := p.g.currentInputSession().playbackInput(KeyAny); replaying {
		return mathf.Vec2{X: mouse.position.X, Y: mouse.position.Y}
	}
	return p.mousePos
}

func (p *inputManager) effectiveMousePressed() bool {
	if mouse, replaying := p.g.currentInputSession().playbackInput(KeyAny); replaying {
		return mouse.buttons&0b011 != 0
	}
	return engine.AnyMouseButtonPressed()
}

// dispatchFrameEvents completes this frame's routing before returning to the
// engine. User handlers remain asynchronous and may wait across future frames.
func (p *Game) dispatchFrameEvents() {
	frame := p.inputMgr.frame.pending
	p.inputMgr.frame.pending = nil
	if frame == nil {
		return
	}
	// Capture requests must observe the advanced frame clock.
	for range frame.captures {
		Snapshot("", nil)
	}
	withEventRegistrationBarrier(frameEventDispatcher{}, func() {
		// Freeze due timers before dispatch: handlers may register or reset timers.
		for {
			timestamp, ok := itime.NextTimer()
			if !ok {
				break
			}
			frame.events = append(frame.events, &eventTimer{Timestamp: timestamp})
		}
		p.scriptEvents.dispatchConditions()
		for _, ev := range frame.events {
			p.handleEvent(ev)
		}
	})
}

func (p *inputFrame) eventHooks(captureKey Key) coreruntime.InputFrameHooks {
	return coreruntime.InputFrameHooks{
		FireLeftButtonDown: func(point mathf.Vec2) {
			p.events = append(p.events, &eventLeftButtonDown{Pos: point})
		},
		FireLeftButtonUp: func(point mathf.Vec2) {
			p.events = append(p.events, &eventLeftButtonUp{Pos: point})
		},
		OnMouseMove: func(point mathf.Vec2) {
			p.events = append(p.events, &eventMouseMove{Pos: point})
		},
		OnKeyPressed: func(keyID int64) {
			key := Key(keyID)
			p.events = append(p.events, &eventKeyDown{Key: key})
			if captureKey != 0 && key == captureKey {
				p.captures++
			}
		},
	}
}
