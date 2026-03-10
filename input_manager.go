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
	"time"

	"github.com/goplus/spbase/mathf"
	coreevent "github.com/goplus/spx/v2/internal/core/event"
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	inputstate "github.com/goplus/spx/v2/internal/input"
	inkey "github.com/goplus/spx/v2/internal/input/keycode"
	spxlog "github.com/goplus/spx/v2/internal/log"
	engine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

// Key represents a keyboard key code.
type Key = coreevent.Key

// Keyboard key constants
const (
	Key0            Key = engine.Key0
	Key1            Key = engine.Key1
	Key2            Key = engine.Key2
	Key3            Key = engine.Key3
	Key4            Key = engine.Key4
	Key5            Key = engine.Key5
	Key6            Key = engine.Key6
	Key7            Key = engine.Key7
	Key8            Key = engine.Key8
	Key9            Key = engine.Key9
	KeyA            Key = engine.KeyA
	KeyB            Key = engine.KeyB
	KeyC            Key = engine.KeyC
	KeyD            Key = engine.KeyD
	KeyE            Key = engine.KeyE
	KeyF            Key = engine.KeyF
	KeyG            Key = engine.KeyG
	KeyH            Key = engine.KeyH
	KeyI            Key = engine.KeyI
	KeyJ            Key = engine.KeyJ
	KeyK            Key = engine.KeyK
	KeyL            Key = engine.KeyL
	KeyM            Key = engine.KeyM
	KeyN            Key = engine.KeyN
	KeyO            Key = engine.KeyO
	KeyP            Key = engine.KeyP
	KeyQ            Key = engine.KeyQ
	KeyR            Key = engine.KeyR
	KeyS            Key = engine.KeyS
	KeyT            Key = engine.KeyT
	KeyU            Key = engine.KeyU
	KeyV            Key = engine.KeyV
	KeyW            Key = engine.KeyW
	KeyX            Key = engine.KeyX
	KeyY            Key = engine.KeyY
	KeyZ            Key = engine.KeyZ
	KeyApostrophe   Key = engine.KeyApostrophe
	KeyBackslash    Key = engine.KeyBackslash
	KeyBackspace    Key = engine.KeyBackspace
	KeyCapsLock     Key = engine.KeyCapsLock
	KeyComma        Key = engine.KeyComma
	KeyDelete       Key = engine.KeyDelete
	KeyDown         Key = engine.KeyDown
	KeyEnd          Key = engine.KeyEnd
	KeyEnter        Key = engine.KeyEnter
	KeyEqual        Key = engine.KeyEqual
	KeyEscape       Key = engine.KeyEscape
	KeyF1           Key = engine.KeyF1
	KeyF2           Key = engine.KeyF2
	KeyF3           Key = engine.KeyF3
	KeyF4           Key = engine.KeyF4
	KeyF5           Key = engine.KeyF5
	KeyF6           Key = engine.KeyF6
	KeyF7           Key = engine.KeyF7
	KeyF8           Key = engine.KeyF8
	KeyF9           Key = engine.KeyF9
	KeyF10          Key = engine.KeyF10
	KeyF11          Key = engine.KeyF11
	KeyF12          Key = engine.KeyF12
	KeyGraveAccent  Key = engine.KeyGraveAccent
	KeyHome         Key = engine.KeyHome
	KeyInsert       Key = engine.KeyInsert
	KeyKP0          Key = engine.KeyKP0
	KeyKP1          Key = engine.KeyKP1
	KeyKP2          Key = engine.KeyKP2
	KeyKP3          Key = engine.KeyKP3
	KeyKP4          Key = engine.KeyKP4
	KeyKP5          Key = engine.KeyKP5
	KeyKP6          Key = engine.KeyKP6
	KeyKP7          Key = engine.KeyKP7
	KeyKP8          Key = engine.KeyKP8
	KeyKP9          Key = engine.KeyKP9
	KeyKPDecimal    Key = engine.KeyKPDecimal
	KeyKPDivide     Key = engine.KeyKPDivide
	KeyKPEnter      Key = engine.KeyKPEnter
	KeyKPEqual      Key = engine.KeyEqual
	KeyKPMultiply   Key = engine.KeyKPMultiply
	KeyKPSubtract   Key = engine.KeyKPSubtract
	KeyLeft         Key = engine.KeyLeft
	KeyLeftBracket  Key = engine.KeyLeftBracket
	KeyMenu         Key = engine.KeyMenu
	KeyMinus        Key = engine.KeyMinus
	KeyNumLock      Key = engine.KeyNumLock
	KeyPageDown     Key = engine.KeyPageDown
	KeyPageUp       Key = engine.KeyPageUp
	KeyPause        Key = engine.KeyPause
	KeyPeriod       Key = engine.KeyPeriod
	KeyPrintScreen  Key = engine.KeyPrintScreen
	KeyRight        Key = engine.KeyRight
	KeyRightBracket Key = engine.KeyRightBracket
	KeyScrollLock   Key = engine.KeyScrollLock
	KeySemicolon    Key = engine.KeySemicolon
	KeySlash        Key = engine.KeySlash
	KeySpace        Key = engine.KeySpace
	KeyTab          Key = engine.KeyTab
	KeyUp           Key = engine.KeyUp
	KeyAlt          Key = engine.KeyAlt
	KeyControl      Key = engine.KeyControl
	KeyShift        Key = engine.KeyShift
	KeyMax          Key = -2
	KeyAny          Key = -1
)

// KeyFromString converts a string to its corresponding Key code.
// It supports key names like "A", "Space", "Enter", "Left", etc.
// Returns KeyMax if the key name is not recognized.
func KeyFromString(key string) Key {
	if key == "Any" {
		return KeyAny
	}
	if keyCode, ok := inkey.Parse(key); ok {
		return Key(keyCode)
	}
	return KeyMax
}

const (
	// Minimum interval between two mouse click events.
	mouseClickInterval = 50 * time.Millisecond
)

// inputManager handles runtime input state and delegates specialized logic to helpers.
type inputManager struct {
	g        *Game
	mousePos mathf.Vec2

	clickGate inputstate.ClickGate
	swipe     coreruntime.SwipeState[*SpriteImpl]
}

func (p *inputManager) init(g *Game) {
	p.mousePos = mathf.Vec2{}
	p.g = g
	p.clickGate.Init(mouseClickInterval)
	p.swipe.Init()
}

func (p *inputManager) currentMousePos() mathf.Vec2 {
	return p.mousePos
}

func (p *inputManager) setMousePos(pos mathf.Vec2) {
	p.mousePos = pos
}

func (p *inputManager) canTriggerClickEvent(id engine.Object) bool {
	return p.clickGate.Allow(id)
}

func (p *inputManager) removeClickTarget(id engine.Object) {
	p.clickGate.Remove(id)
}

func (p *inputManager) beginSwipeTracking(startPos mathf.Vec2, targetSprite *SpriteImpl) {
	p.swipe.Begin(startPos, targetSprite)
}

func (p *inputManager) finishSwipeTracking(point mathf.Vec2) {
	p.swipe.Finish(point, p.swipeHooks())
}

func (p *inputManager) onMouseMove(pos mathf.Vec2) {
	p.swipe.OnMouseMove(pos, p.swipeHooks())
}

func (p *inputManager) swipeHooks() coreruntime.SwipeHooks[*SpriteImpl] {
	return coreruntime.SwipeHooks[*SpriteImpl]{
		Debug: coreevent.If1(isDebugEventEnabled, func(ev coreruntime.SwipeEvent[*SpriteImpl]) {
			targetName := "stage"
			if ev.Target != nil {
				targetName = ev.Target.name
			}
			spxlog.Debug("Swipe detected: direction=%v, velocity=%.2f, distance=%.2f, target=%s",
				Direction(ev.Direction), ev.Velocity, ev.Distance, targetName)
		}),
		DispatchTarget: func(direction float64, targetSprite *SpriteImpl) {
			targetSprite.doWhenSwipe(Direction(direction), targetSprite)
		},
		DispatchStage: func(direction float64) {
			p.g.scriptEvents.doWhenSwipe(Direction(direction), p.g)
		},
	}
}
