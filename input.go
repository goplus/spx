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

	gdx "github.com/goplus/spx/pkg/gdspx/pkg/engine"
	"github.com/realdream-ai/mathf"
)

type Key = int64

var (
	Key0            Key
	Key1            Key
	Key2            Key
	Key3            Key
	Key4            Key
	Key5            Key
	Key6            Key
	Key7            Key
	Key8            Key
	Key9            Key
	KeyA            Key
	KeyB            Key
	KeyC            Key
	KeyD            Key
	KeyE            Key
	KeyF            Key
	KeyG            Key
	KeyH            Key
	KeyI            Key
	KeyJ            Key
	KeyK            Key
	KeyL            Key
	KeyM            Key
	KeyN            Key
	KeyO            Key
	KeyP            Key
	KeyQ            Key
	KeyR            Key
	KeyS            Key
	KeyT            Key
	KeyU            Key
	KeyV            Key
	KeyW            Key
	KeyX            Key
	KeyY            Key
	KeyZ            Key
	KeyApostrophe   Key
	KeyBackslash    Key
	KeyBackspace    Key
	KeyCapsLock     Key
	KeyComma        Key
	KeyDelete       Key
	KeyDown         Key
	KeyEnd          Key
	KeyEnter        Key
	KeyEqual        Key
	KeyEscape       Key
	KeyF1           Key
	KeyF2           Key
	KeyF3           Key
	KeyF4           Key
	KeyF5           Key
	KeyF6           Key
	KeyF7           Key
	KeyF8           Key
	KeyF9           Key
	KeyF10          Key
	KeyF11          Key
	KeyF12          Key
	KeyGraveAccent  Key
	KeyHome         Key
	KeyInsert       Key
	KeyKP0          Key
	KeyKP1          Key
	KeyKP2          Key
	KeyKP3          Key
	KeyKP4          Key
	KeyKP5          Key
	KeyKP6          Key
	KeyKP7          Key
	KeyKP8          Key
	KeyKP9          Key
	KeyKPDecimal    Key
	KeyKPDivide     Key
	KeyKPEnter      Key
	KeyKPEqual      Key
	KeyKPMultiply   Key
	KeyKPSubtract   Key
	KeyLeft         Key
	KeyLeftBracket  Key
	KeyMenu         Key
	KeyMinus        Key
	KeyNumLock      Key
	KeyPageDown     Key
	KeyPageUp       Key
	KeyPause        Key
	KeyPeriod       Key
	KeyPrintScreen  Key
	KeyRight        Key
	KeyRightBracket Key
	KeyScrollLock   Key
	KeySemicolon    Key
	KeySlash        Key
	KeySpace        Key
	KeyTab          Key
	KeyUp           Key
	KeyAlt          Key
	KeyControl      Key
	KeyShift        Key
	KeyMax          Key = -2
	KeyAny          Key = -1
)

func initInput() {
	Key0 = gdx.KeyCode.Key0
	Key1 = gdx.KeyCode.Key1
	Key2 = gdx.KeyCode.Key2
	Key3 = gdx.KeyCode.Key3
	Key4 = gdx.KeyCode.Key4
	Key5 = gdx.KeyCode.Key5
	Key6 = gdx.KeyCode.Key6
	Key7 = gdx.KeyCode.Key7
	Key8 = gdx.KeyCode.Key8
	Key9 = gdx.KeyCode.Key9
	KeyA = gdx.KeyCode.A
	KeyB = gdx.KeyCode.B
	KeyC = gdx.KeyCode.C
	KeyD = gdx.KeyCode.D
	KeyE = gdx.KeyCode.E
	KeyF = gdx.KeyCode.F
	KeyG = gdx.KeyCode.G
	KeyH = gdx.KeyCode.H
	KeyI = gdx.KeyCode.I
	KeyJ = gdx.KeyCode.J
	KeyK = gdx.KeyCode.K
	KeyL = gdx.KeyCode.L
	KeyM = gdx.KeyCode.M
	KeyN = gdx.KeyCode.N
	KeyO = gdx.KeyCode.O
	KeyP = gdx.KeyCode.P
	KeyQ = gdx.KeyCode.Q
	KeyR = gdx.KeyCode.R
	KeyS = gdx.KeyCode.S
	KeyT = gdx.KeyCode.T
	KeyU = gdx.KeyCode.U
	KeyV = gdx.KeyCode.V
	KeyW = gdx.KeyCode.W
	KeyX = gdx.KeyCode.X
	KeyY = gdx.KeyCode.Y
	KeyZ = gdx.KeyCode.Z
	KeyApostrophe = gdx.KeyCode.Apostrophe
	KeyBackslash = gdx.KeyCode.Backslash
	KeyBackspace = gdx.KeyCode.Backspace
	KeyCapsLock = gdx.KeyCode.CapsLock
	KeyComma = gdx.KeyCode.Comma
	KeyDelete = gdx.KeyCode.Delete
	KeyDown = gdx.KeyCode.Down
	KeyEnd = gdx.KeyCode.End
	KeyEnter = gdx.KeyCode.Enter
	KeyEqual = gdx.KeyCode.Equal
	KeyEscape = gdx.KeyCode.Escape
	KeyF1 = gdx.KeyCode.F1
	KeyF2 = gdx.KeyCode.F2
	KeyF3 = gdx.KeyCode.F3
	KeyF4 = gdx.KeyCode.F4
	KeyF5 = gdx.KeyCode.F5
	KeyF6 = gdx.KeyCode.F6
	KeyF7 = gdx.KeyCode.F7
	KeyF8 = gdx.KeyCode.F8
	KeyF9 = gdx.KeyCode.F9
	KeyF10 = gdx.KeyCode.F10
	KeyF11 = gdx.KeyCode.F11
	KeyF12 = gdx.KeyCode.F12
	KeyGraveAccent = gdx.KeyCode.QuoteLeft
	KeyHome = gdx.KeyCode.Home
	KeyInsert = gdx.KeyCode.Insert
	KeyKP0 = gdx.KeyCode.KP0
	KeyKP1 = gdx.KeyCode.KP1
	KeyKP2 = gdx.KeyCode.KP2
	KeyKP3 = gdx.KeyCode.KP3
	KeyKP4 = gdx.KeyCode.KP4
	KeyKP5 = gdx.KeyCode.KP5
	KeyKP6 = gdx.KeyCode.KP6
	KeyKP7 = gdx.KeyCode.KP7
	KeyKP8 = gdx.KeyCode.KP8
	KeyKP9 = gdx.KeyCode.KP9
	KeyKPDecimal = gdx.KeyCode.KPPeriod
	KeyKPDivide = gdx.KeyCode.KPDivide
	KeyKPEnter = gdx.KeyCode.KPEnter
	KeyKPEqual = gdx.KeyCode.Equal
	KeyKPMultiply = gdx.KeyCode.KPMultiply
	KeyKPSubtract = gdx.KeyCode.KPSubtract
	KeyLeft = gdx.KeyCode.Left
	KeyLeftBracket = gdx.KeyCode.BracketLeft
	KeyMenu = gdx.KeyCode.Menu
	KeyMinus = gdx.KeyCode.Minus
	KeyNumLock = gdx.KeyCode.NumLock
	KeyPageDown = gdx.KeyCode.PageDown
	KeyPageUp = gdx.KeyCode.PageUp
	KeyPause = gdx.KeyCode.Pause
	KeyPeriod = gdx.KeyCode.Period
	KeyPrintScreen = gdx.KeyCode.Print
	KeyRight = gdx.KeyCode.Right
	KeyRightBracket = gdx.KeyCode.BracketRight
	KeyScrollLock = gdx.KeyCode.ScrollLock
	KeySemicolon = gdx.KeyCode.Semicolon
	KeySlash = gdx.KeyCode.Slash
	KeySpace = gdx.KeyCode.Space
	KeyTab = gdx.KeyCode.Tab
	KeyUp = gdx.KeyCode.Up
	KeyAlt = gdx.KeyCode.Alt
	KeyControl = gdx.KeyCode.CmdOrCtrl
	KeyShift = gdx.KeyCode.Shift
	KeyMax = -2
	KeyAny = -1
}

const (
	mouseStateNone     = 0x00
	mouseStatePressing = 0x01
	mouseFlagStates    = 0x7f
	mouseFlagTouching  = 0x80
)

// -------------------------------------------------------------------------------------

type event interface{}

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
