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
	"github.com/goplus/spx/v4/internal/core/clock"
	inputstate "github.com/goplus/spx/v4/internal/core/input"
	inkey "github.com/goplus/spx/v4/internal/core/input/keycode"
	coreruntime "github.com/goplus/spx/v4/internal/core/runtime"
	"github.com/goplus/spx/v4/internal/engine"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

const (
	Key0            Key = pkgengine.Key0
	Key1            Key = pkgengine.Key1
	Key2            Key = pkgengine.Key2
	Key3            Key = pkgengine.Key3
	Key4            Key = pkgengine.Key4
	Key5            Key = pkgengine.Key5
	Key6            Key = pkgengine.Key6
	Key7            Key = pkgengine.Key7
	Key8            Key = pkgengine.Key8
	Key9            Key = pkgengine.Key9
	KeyA            Key = pkgengine.KeyA
	KeyB            Key = pkgengine.KeyB
	KeyC            Key = pkgengine.KeyC
	KeyD            Key = pkgengine.KeyD
	KeyE            Key = pkgengine.KeyE
	KeyF            Key = pkgengine.KeyF
	KeyG            Key = pkgengine.KeyG
	KeyH            Key = pkgengine.KeyH
	KeyI            Key = pkgengine.KeyI
	KeyJ            Key = pkgengine.KeyJ
	KeyK            Key = pkgengine.KeyK
	KeyL            Key = pkgengine.KeyL
	KeyM            Key = pkgengine.KeyM
	KeyN            Key = pkgengine.KeyN
	KeyO            Key = pkgengine.KeyO
	KeyP            Key = pkgengine.KeyP
	KeyQ            Key = pkgengine.KeyQ
	KeyR            Key = pkgengine.KeyR
	KeyS            Key = pkgengine.KeyS
	KeyT            Key = pkgengine.KeyT
	KeyU            Key = pkgengine.KeyU
	KeyV            Key = pkgengine.KeyV
	KeyW            Key = pkgengine.KeyW
	KeyX            Key = pkgengine.KeyX
	KeyY            Key = pkgengine.KeyY
	KeyZ            Key = pkgengine.KeyZ
	KeyApostrophe   Key = pkgengine.KeyApostrophe
	KeyBackslash    Key = pkgengine.KeyBackslash
	KeyBackspace    Key = pkgengine.KeyBackspace
	KeyCapsLock     Key = pkgengine.KeyCapsLock
	KeyComma        Key = pkgengine.KeyComma
	KeyDelete       Key = pkgengine.KeyDelete
	KeyDown         Key = pkgengine.KeyDown
	KeyEnd          Key = pkgengine.KeyEnd
	KeyEnter        Key = pkgengine.KeyEnter
	KeyEqual        Key = pkgengine.KeyEqual
	KeyEscape       Key = pkgengine.KeyEscape
	KeyExclam       Key = pkgengine.KeyExclam
	KeyF1           Key = pkgengine.KeyF1
	KeyF2           Key = pkgengine.KeyF2
	KeyF3           Key = pkgengine.KeyF3
	KeyF4           Key = pkgengine.KeyF4
	KeyF5           Key = pkgengine.KeyF5
	KeyF6           Key = pkgengine.KeyF6
	KeyF7           Key = pkgengine.KeyF7
	KeyF8           Key = pkgengine.KeyF8
	KeyF9           Key = pkgengine.KeyF9
	KeyF10          Key = pkgengine.KeyF10
	KeyF11          Key = pkgengine.KeyF11
	KeyF12          Key = pkgengine.KeyF12
	KeyGraveAccent  Key = pkgengine.KeyGraveAccent
	KeyHome         Key = pkgengine.KeyHome
	KeyInsert       Key = pkgengine.KeyInsert
	KeyKP0          Key = pkgengine.KeyKP0
	KeyKP1          Key = pkgengine.KeyKP1
	KeyKP2          Key = pkgengine.KeyKP2
	KeyKP3          Key = pkgengine.KeyKP3
	KeyKP4          Key = pkgengine.KeyKP4
	KeyKP5          Key = pkgengine.KeyKP5
	KeyKP6          Key = pkgengine.KeyKP6
	KeyKP7          Key = pkgengine.KeyKP7
	KeyKP8          Key = pkgengine.KeyKP8
	KeyKP9          Key = pkgengine.KeyKP9
	KeyKPDecimal    Key = pkgengine.KeyKPDecimal
	KeyKPDivide     Key = pkgengine.KeyKPDivide
	KeyKPEnter      Key = pkgengine.KeyKPEnter
	KeyKPEqual      Key = pkgengine.KeyEqual
	KeyKPMultiply   Key = pkgengine.KeyKPMultiply
	KeyKPSubtract   Key = pkgengine.KeyKPSubtract
	KeyLeft         Key = pkgengine.KeyLeft
	KeyLeftBracket  Key = pkgengine.KeyLeftBracket
	KeyMenu         Key = pkgengine.KeyMenu
	KeyMinus        Key = pkgengine.KeyMinus
	KeyNumLock      Key = pkgengine.KeyNumLock
	KeyPageDown     Key = pkgengine.KeyPageDown
	KeyPageUp       Key = pkgengine.KeyPageUp
	KeyPause        Key = pkgengine.KeyPause
	KeyPeriod       Key = pkgengine.KeyPeriod
	KeyPrintScreen  Key = pkgengine.KeyPrintScreen
	KeyRight        Key = pkgengine.KeyRight
	KeyRightBracket Key = pkgengine.KeyRightBracket
	KeyScrollLock   Key = pkgengine.KeyScrollLock
	KeySemicolon    Key = pkgengine.KeySemicolon
	KeySlash        Key = pkgengine.KeySlash
	KeySpace        Key = pkgengine.KeySpace
	KeyTab          Key = pkgengine.KeyTab
	KeyUp           Key = pkgengine.KeyUp
	KeyAlt          Key = pkgengine.KeyAlt
	KeyControl      Key = pkgengine.KeyControl
	KeyShift        Key = pkgengine.KeyShift
	KeyMax          Key = -2
	KeyAny          Key = -1
)

const (
	MOUSE_BUTTON_LEFT   int64 = inputstate.MouseButtonLeft
	MOUSE_BUTTON_RIGHT  int64 = inputstate.MouseButtonRight
	MOUSE_BUTTON_MIDDLE int64 = inputstate.MouseButtonMiddle
)

type inputManager struct {
	g        *Game
	mousePos mathf.Vec2

	clickGate inputstate.ClickGate
	drag      spriteDragState
	swipe     coreruntime.SwipeState[*SpriteImpl]
	frame     inputFrameState
}

// Sampling runs between script slices; pending is consumed on the engine thread.
type inputFrameState struct {
	history        coreruntime.InputFrameState
	mouseEvents    []engine.MouseEvent
	keyEvents      []engine.KeyEvent
	hadSession     bool
	pending        *inputFrame
	dragProjection *dragProjection
}

type inputFrame struct {
	events   []event
	captures int
}

// Frame routing is runtime work; stopping game scripts must not discard input.
type frameEventDispatcher struct{}

// KeyFromString returns KeyMax for an unrecognized key name.
func KeyFromString(key string) Key {
	if key == "Any" {
		return KeyAny
	}
	if keyCode, ok := inkey.Parse(key); ok {
		return Key(keyCode)
	}
	return KeyMax
}

// Input queries.
func (p *Game) KeyPressed(key Key) bool {
	return p.inputMgr.effectiveKeyPressed(key)
}

func (p *Game) MouseX() float64 {
	return p.inputMgr.effectiveMousePos().X
}

func (p *Game) MouseY() float64 {
	return p.inputMgr.effectiveMousePos().Y
}

func (p *Game) MousePressed() bool {
	return p.inputMgr.effectiveMousePressed()
}

func (p *Game) getMousePos() (x, y float64) {
	return p.MouseX(), p.MouseY()
}

// dispatchFrameEvents completes this frame's routing before returning to the
// engine. User handlers remain asynchronous and may wait across future frames.
func (p *Game) dispatchFrameEvents() {
	frame := p.inputMgr.frame.pending
	p.inputMgr.frame.pending = nil
	if frame == nil {
		return
	}
	// Expire the previous gesture after the frame clock advances and before
	// button edges can begin a new gesture, including frames with no movement.
	p.inputMgr.swipe.Expire()
	// Capture requests must observe the advanced frame clock.
	for range frame.captures {
		capture("", nil)
	}
	withEventRegistrationBarrier(frameEventDispatcher{}, func() {
		// Freeze due timers before dispatch: handlers may register or reset timers.
		for {
			timestamp, ok := clock.NextTimer()
			if !ok {
				break
			}
			frame.events = append(frame.events, &eventTimer{Timestamp: timestamp})
		}
		p.scriptEvents.dispatchConditions()
		for _, ev := range frame.events {
			p.handleEvent(ev)
		}
		p.inputMgr.updateDrag(p.inputMgr.mousePos)
	})
}

func (p *inputManager) init(g *Game) {
	p.mousePos = mathf.Vec2{}
	p.g = g
	// Live recordings preserve empty (rather than nil) edge slices.
	p.frame = inputFrameState{
		mouseEvents: []InputReplayMouseEvent{},
		keyEvents:   []InputReplayKeyEvent{},
	}
	p.drag = spriteDragState{}
	p.clickGate.InitWithClock(mouseClickInterval, g.inputClock)
	p.swipe.InitWithClock(g.inputClock)
}

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
	c.dragProjection = nil
	if p.drag.pressed || state.Buttons&1 != 0 || len(c.mouseEvents) != 0 {
		c.dragProjection = p.sampleDragProjection()
	}
	point := mathf.Vec2{X: state.Mouse.X, Y: state.Mouse.Y}
	if session == nil && c.hadSession {
		// The previous session owns its input edges. Resume from held state.
		c.history.Reset(state.Buttons&1 != 0)
		c.mouseEvents, c.keyEvents = c.mouseEvents[:0], c.keyEvents[:0]
		p.drag = spriteDragState{}
	}
	c.hadSession = session != nil
	p.mousePos = point
	batch := &inputFrame{}
	if edges := len(c.mouseEvents) + len(c.keyEvents); edges > 0 {
		batch.events = make([]event, 0, edges+2)
	}
	c.history.Process(coreruntime.InputFrame{
		Point:             point,
		LeftButtonPressed: state.Buttons&1 != 0,
		MouseEvents:       c.mouseEvents,
		KeyEvents:         c.keyEvents,
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
		return state, c.mouseEvents, c.keyEvents
	})
	if err != nil {
		return InputReplayState{}, err
	}
	if resolved.firstTick {
		c.history.Reset(resolved.initialLeftPressed)
		p.clickGate.InitWithClock(mouseClickInterval, p.g.inputClock)
		p.swipe.InitWithClock(p.g.inputClock)
		p.drag = spriteDragState{}
	}
	// Keep reusable frame buffers separate from the controller's snapshots.
	c.mouseEvents = append(c.mouseEvents[:0], resolved.frame.MouseEvents...)
	c.keyEvents = append(c.keyEvents[:0], resolved.frame.KeyEvents...)
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

func (p *inputFrame) eventHooks(captureKey Key) coreruntime.InputFrameHooks {
	return coreruntime.InputFrameHooks{
		FireLeftButtonDown: func(point mathf.Vec2) {
			p.events = append(p.events, &eventLeftButtonDown{Pos: point})
		},
		FireLeftButtonUp: func(point mathf.Vec2) {
			p.events = append(p.events, &eventLeftButtonUp{Pos: point})
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
