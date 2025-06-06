/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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

	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
	"github.com/realdream-ai/mathf"
)

type Key = gdx.KeyCode

const (
	Key0            Key = gdx.Key0
	Key1            Key = gdx.Key1
	Key2            Key = gdx.Key2
	Key3            Key = gdx.Key3
	Key4            Key = gdx.Key4
	Key5            Key = gdx.Key5
	Key6            Key = gdx.Key6
	Key7            Key = gdx.Key7
	Key8            Key = gdx.Key8
	Key9            Key = gdx.Key9
	KeyA            Key = gdx.KeyA
	KeyB            Key = gdx.KeyB
	KeyC            Key = gdx.KeyC
	KeyD            Key = gdx.KeyD
	KeyE            Key = gdx.KeyE
	KeyF            Key = gdx.KeyF
	KeyG            Key = gdx.KeyG
	KeyH            Key = gdx.KeyH
	KeyI            Key = gdx.KeyI
	KeyJ            Key = gdx.KeyJ
	KeyK            Key = gdx.KeyK
	KeyL            Key = gdx.KeyL
	KeyM            Key = gdx.KeyM
	KeyN            Key = gdx.KeyN
	KeyO            Key = gdx.KeyO
	KeyP            Key = gdx.KeyP
	KeyQ            Key = gdx.KeyQ
	KeyR            Key = gdx.KeyR
	KeyS            Key = gdx.KeyS
	KeyT            Key = gdx.KeyT
	KeyU            Key = gdx.KeyU
	KeyV            Key = gdx.KeyV
	KeyW            Key = gdx.KeyW
	KeyX            Key = gdx.KeyX
	KeyY            Key = gdx.KeyY
	KeyZ            Key = gdx.KeyZ
	KeyApostrophe   Key = gdx.KeyApostrophe
	KeyBackslash    Key = gdx.KeyBackslash
	KeyBackspace    Key = gdx.KeyBackspace
	KeyCapsLock     Key = gdx.KeyCapsLock
	KeyComma        Key = gdx.KeyComma
	KeyDelete       Key = gdx.KeyDelete
	KeyDown         Key = gdx.KeyDown
	KeyEnd          Key = gdx.KeyEnd
	KeyEnter        Key = gdx.KeyEnter
	KeyEqual        Key = gdx.KeyEqual
	KeyEscape       Key = gdx.KeyEscape
	KeyF1           Key = gdx.KeyF1
	KeyF2           Key = gdx.KeyF2
	KeyF3           Key = gdx.KeyF3
	KeyF4           Key = gdx.KeyF4
	KeyF5           Key = gdx.KeyF5
	KeyF6           Key = gdx.KeyF6
	KeyF7           Key = gdx.KeyF7
	KeyF8           Key = gdx.KeyF8
	KeyF9           Key = gdx.KeyF9
	KeyF10          Key = gdx.KeyF10
	KeyF11          Key = gdx.KeyF11
	KeyF12          Key = gdx.KeyF12
	KeyGraveAccent  Key = gdx.KeyQuoteLeft
	KeyHome         Key = gdx.KeyHome
	KeyInsert       Key = gdx.KeyInsert
	KeyKP0          Key = gdx.KeyKP0
	KeyKP1          Key = gdx.KeyKP1
	KeyKP2          Key = gdx.KeyKP2
	KeyKP3          Key = gdx.KeyKP3
	KeyKP4          Key = gdx.KeyKP4
	KeyKP5          Key = gdx.KeyKP5
	KeyKP6          Key = gdx.KeyKP6
	KeyKP7          Key = gdx.KeyKP7
	KeyKP8          Key = gdx.KeyKP8
	KeyKP9          Key = gdx.KeyKP9
	KeyKPDecimal    Key = gdx.KeyKPPeriod
	KeyKPDivide     Key = gdx.KeyKPDivide
	KeyKPEnter      Key = gdx.KeyKPEnter
	KeyKPEqual      Key = gdx.KeyEqual
	KeyKPMultiply   Key = gdx.KeyKPMultiply
	KeyKPSubtract   Key = gdx.KeyKPSubtract
	KeyLeft         Key = gdx.KeyLeft
	KeyLeftBracket  Key = gdx.KeyBracketLeft
	KeyMenu         Key = gdx.KeyMenu
	KeyMinus        Key = gdx.KeyMinus
	KeyNumLock      Key = gdx.KeyNumLock
	KeyPageDown     Key = gdx.KeyPageDown
	KeyPageUp       Key = gdx.KeyPageUp
	KeyPause        Key = gdx.KeyPause
	KeyPeriod       Key = gdx.KeyPeriod
	KeyPrintScreen  Key = gdx.KeyPrint
	KeyRight        Key = gdx.KeyRight
	KeyRightBracket Key = gdx.KeyBracketRight
	KeyScrollLock   Key = gdx.KeyScrollLock
	KeySemicolon    Key = gdx.KeySemicolon
	KeySlash        Key = gdx.KeySlash
	KeySpace        Key = gdx.KeySpace
	KeyTab          Key = gdx.KeyTab
	KeyUp           Key = gdx.KeyUp
	KeyAlt          Key = gdx.KeyAlt
	KeyControl      Key = gdx.KeyCmdOrCtrl
	KeyShift        Key = gdx.KeyShift
	KeyMax          Key = -2
	KeyAny          Key = -1
)

func initInput() {

}

const (
	mouseStateNone     = 0x00
	mouseStatePressing = 0x01
	mouseFlagStates    = 0x7f
	mouseFlagTouching  = 0x80
)

// -------------------------------------------------------------------------------------

type event any

type eventStart struct{}

type eventKeyDown struct {
	Key Key
}

type eventKeyUp struct {
	Key Key
}

type eventLeftButtonDown struct {
	Pos mathf.Vec2
}

type eventLeftButtonUp struct {
	Pos mathf.Vec2
}

type eventTimer struct {
	Time float64
}

type eventFirer interface {
	fireEvent(ev event)
}

// -------------------------------------------------------------------------------------

type inputManager struct {
	tempItems []Shape
	g         *Game
	id2Timer  map[gdx.Object]int64
}

const (
	// minimum interval between two mouse click events
	inputMouseClickIntervalMs = 50
	inputGlobalClickTimerId   = -1 // global click cooldown
	inputStageClickTimerId    = 0  // stage click cooldown
)

func (p *inputManager) init(g *Game) {
	p.tempItems = make([]Shape, 50)
	p.id2Timer = make(map[gdx.Object]int64)
	p.g = g
}

func (p *inputManager) canTriggerClickEvent(id gdx.Object) bool {
	currentTime := time.Now()
	milliseconds := currentTime.UnixNano() / int64(time.Millisecond)
	if lastTime, ok := p.id2Timer[id]; ok {
		if milliseconds-lastTime < inputMouseClickIntervalMs {
			return false
		}
	}
	p.id2Timer[id] = milliseconds
	return true
}
