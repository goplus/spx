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
	"github.com/realdream-ai/gdspx/pkg/engine"
)

type Key = int64

var (
	Key0            Key = engine.KeyCode.Key0
	Key1            Key = engine.KeyCode.Key1
	Key2            Key = engine.KeyCode.Key2
	Key3            Key = engine.KeyCode.Key3
	Key4            Key = engine.KeyCode.Key4
	Key5            Key = engine.KeyCode.Key5
	Key6            Key = engine.KeyCode.Key6
	Key7            Key = engine.KeyCode.Key7
	Key8            Key = engine.KeyCode.Key8
	Key9            Key = engine.KeyCode.Key9
	KeyA            Key = engine.KeyCode.A
	KeyB            Key = engine.KeyCode.B
	KeyC            Key = engine.KeyCode.C
	KeyD            Key = engine.KeyCode.D
	KeyE            Key = engine.KeyCode.E
	KeyF            Key = engine.KeyCode.F
	KeyG            Key = engine.KeyCode.G
	KeyH            Key = engine.KeyCode.H
	KeyI            Key = engine.KeyCode.I
	KeyJ            Key = engine.KeyCode.J
	KeyK            Key = engine.KeyCode.K
	KeyL            Key = engine.KeyCode.L
	KeyM            Key = engine.KeyCode.M
	KeyN            Key = engine.KeyCode.N
	KeyO            Key = engine.KeyCode.O
	KeyP            Key = engine.KeyCode.P
	KeyQ            Key = engine.KeyCode.Q
	KeyR            Key = engine.KeyCode.R
	KeyS            Key = engine.KeyCode.S
	KeyT            Key = engine.KeyCode.T
	KeyU            Key = engine.KeyCode.U
	KeyV            Key = engine.KeyCode.V
	KeyW            Key = engine.KeyCode.W
	KeyX            Key = engine.KeyCode.X
	KeyY            Key = engine.KeyCode.Y
	KeyZ            Key = engine.KeyCode.Z
	KeyApostrophe   Key = engine.KeyCode.Apostrophe
	KeyBackslash    Key = engine.KeyCode.Backslash
	KeyBackspace    Key = engine.KeyCode.Backspace
	KeyCapsLock     Key = engine.KeyCode.CapsLock
	KeyComma        Key = engine.KeyCode.Comma
	KeyDelete       Key = engine.KeyCode.Delete
	KeyDown         Key = engine.KeyCode.Down
	KeyEnd          Key = engine.KeyCode.End
	KeyEnter        Key = engine.KeyCode.Enter
	KeyEqual        Key = engine.KeyCode.Equal
	KeyEscape       Key = engine.KeyCode.Escape
	KeyF1           Key = engine.KeyCode.F1
	KeyF2           Key = engine.KeyCode.F2
	KeyF3           Key = engine.KeyCode.F3
	KeyF4           Key = engine.KeyCode.F4
	KeyF5           Key = engine.KeyCode.F5
	KeyF6           Key = engine.KeyCode.F6
	KeyF7           Key = engine.KeyCode.F7
	KeyF8           Key = engine.KeyCode.F8
	KeyF9           Key = engine.KeyCode.F9
	KeyF10          Key = engine.KeyCode.F10
	KeyF11          Key = engine.KeyCode.F11
	KeyF12          Key = engine.KeyCode.F12
	KeyGraveAccent  Key = engine.KeyCode.QuoteLeft
	KeyHome         Key = engine.KeyCode.Home
	KeyInsert       Key = engine.KeyCode.Insert
	KeyKP0          Key = engine.KeyCode.KP0
	KeyKP1          Key = engine.KeyCode.KP1
	KeyKP2          Key = engine.KeyCode.KP2
	KeyKP3          Key = engine.KeyCode.KP3
	KeyKP4          Key = engine.KeyCode.KP4
	KeyKP5          Key = engine.KeyCode.KP5
	KeyKP6          Key = engine.KeyCode.KP6
	KeyKP7          Key = engine.KeyCode.KP7
	KeyKP8          Key = engine.KeyCode.KP8
	KeyKP9          Key = engine.KeyCode.KP9
	KeyKPDecimal    Key = engine.KeyCode.KPPeriod
	KeyKPDivide     Key = engine.KeyCode.KPDivide
	KeyKPEnter      Key = engine.KeyCode.KPEnter
	KeyKPEqual      Key = engine.KeyCode.Equal
	KeyKPMultiply   Key = engine.KeyCode.KPMultiply
	KeyKPSubtract   Key = engine.KeyCode.KPSubtract
	KeyLeft         Key = engine.KeyCode.Left
	KeyLeftBracket  Key = engine.KeyCode.BracketLeft
	KeyMenu         Key = engine.KeyCode.Menu
	KeyMinus        Key = engine.KeyCode.Minus
	KeyNumLock      Key = engine.KeyCode.NumLock
	KeyPageDown     Key = engine.KeyCode.PageDown
	KeyPageUp       Key = engine.KeyCode.PageUp
	KeyPause        Key = engine.KeyCode.Pause
	KeyPeriod       Key = engine.KeyCode.Period
	KeyPrintScreen  Key = engine.KeyCode.Print
	KeyRight        Key = engine.KeyCode.Right
	KeyRightBracket Key = engine.KeyCode.BracketRight
	KeyScrollLock   Key = engine.KeyCode.ScrollLock
	KeySemicolon    Key = engine.KeyCode.Semicolon
	KeySlash        Key = engine.KeyCode.Slash
	KeySpace        Key = engine.KeyCode.Space
	KeyTab          Key = engine.KeyCode.Tab
	KeyUp           Key = engine.KeyCode.Up
	KeyAlt          Key = engine.KeyCode.Alt
	KeyControl      Key = engine.KeyCode.CmdOrCtrl
	KeyShift        Key = engine.KeyCode.Shift
	KeyMax          Key = -2
	KeyAny          Key = -1
)

func initInput() {
	Key0 = engine.KeyCode.Key0
	Key1 = engine.KeyCode.Key1
	Key2 = engine.KeyCode.Key2
	Key3 = engine.KeyCode.Key3
	Key4 = engine.KeyCode.Key4
	Key5 = engine.KeyCode.Key5
	Key6 = engine.KeyCode.Key6
	Key7 = engine.KeyCode.Key7
	Key8 = engine.KeyCode.Key8
	Key9 = engine.KeyCode.Key9
	KeyA = engine.KeyCode.A
	KeyB = engine.KeyCode.B
	KeyC = engine.KeyCode.C
	KeyD = engine.KeyCode.D
	KeyE = engine.KeyCode.E
	KeyF = engine.KeyCode.F
	KeyG = engine.KeyCode.G
	KeyH = engine.KeyCode.H
	KeyI = engine.KeyCode.I
	KeyJ = engine.KeyCode.J
	KeyK = engine.KeyCode.K
	KeyL = engine.KeyCode.L
	KeyM = engine.KeyCode.M
	KeyN = engine.KeyCode.N
	KeyO = engine.KeyCode.O
	KeyP = engine.KeyCode.P
	KeyQ = engine.KeyCode.Q
	KeyR = engine.KeyCode.R
	KeyS = engine.KeyCode.S
	KeyT = engine.KeyCode.T
	KeyU = engine.KeyCode.U
	KeyV = engine.KeyCode.V
	KeyW = engine.KeyCode.W
	KeyX = engine.KeyCode.X
	KeyY = engine.KeyCode.Y
	KeyZ = engine.KeyCode.Z
	KeyApostrophe = engine.KeyCode.Apostrophe
	KeyBackslash = engine.KeyCode.Backslash
	KeyBackspace = engine.KeyCode.Backspace
	KeyCapsLock = engine.KeyCode.CapsLock
	KeyComma = engine.KeyCode.Comma
	KeyDelete = engine.KeyCode.Delete
	KeyDown = engine.KeyCode.Down
	KeyEnd = engine.KeyCode.End
	KeyEnter = engine.KeyCode.Enter
	KeyEqual = engine.KeyCode.Equal
	KeyEscape = engine.KeyCode.Escape
	KeyF1 = engine.KeyCode.F1
	KeyF2 = engine.KeyCode.F2
	KeyF3 = engine.KeyCode.F3
	KeyF4 = engine.KeyCode.F4
	KeyF5 = engine.KeyCode.F5
	KeyF6 = engine.KeyCode.F6
	KeyF7 = engine.KeyCode.F7
	KeyF8 = engine.KeyCode.F8
	KeyF9 = engine.KeyCode.F9
	KeyF10 = engine.KeyCode.F10
	KeyF11 = engine.KeyCode.F11
	KeyF12 = engine.KeyCode.F12
	KeyGraveAccent = engine.KeyCode.QuoteLeft
	KeyHome = engine.KeyCode.Home
	KeyInsert = engine.KeyCode.Insert
	KeyKP0 = engine.KeyCode.KP0
	KeyKP1 = engine.KeyCode.KP1
	KeyKP2 = engine.KeyCode.KP2
	KeyKP3 = engine.KeyCode.KP3
	KeyKP4 = engine.KeyCode.KP4
	KeyKP5 = engine.KeyCode.KP5
	KeyKP6 = engine.KeyCode.KP6
	KeyKP7 = engine.KeyCode.KP7
	KeyKP8 = engine.KeyCode.KP8
	KeyKP9 = engine.KeyCode.KP9
	KeyKPDecimal = engine.KeyCode.KPPeriod
	KeyKPDivide = engine.KeyCode.KPDivide
	KeyKPEnter = engine.KeyCode.KPEnter
	KeyKPEqual = engine.KeyCode.Equal
	KeyKPMultiply = engine.KeyCode.KPMultiply
	KeyKPSubtract = engine.KeyCode.KPSubtract
	KeyLeft = engine.KeyCode.Left
	KeyLeftBracket = engine.KeyCode.BracketLeft
	KeyMenu = engine.KeyCode.Menu
	KeyMinus = engine.KeyCode.Minus
	KeyNumLock = engine.KeyCode.NumLock
	KeyPageDown = engine.KeyCode.PageDown
	KeyPageUp = engine.KeyCode.PageUp
	KeyPause = engine.KeyCode.Pause
	KeyPeriod = engine.KeyCode.Period
	KeyPrintScreen = engine.KeyCode.Print
	KeyRight = engine.KeyCode.Right
	KeyRightBracket = engine.KeyCode.BracketRight
	KeyScrollLock = engine.KeyCode.ScrollLock
	KeySemicolon = engine.KeyCode.Semicolon
	KeySlash = engine.KeyCode.Slash
	KeySpace = engine.KeyCode.Space
	KeyTab = engine.KeyCode.Tab
	KeyUp = engine.KeyCode.Up
	KeyAlt = engine.KeyCode.Alt
	KeyControl = engine.KeyCode.CmdOrCtrl
	KeyShift = engine.KeyCode.Shift
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
	X, Y int
}

type eventLeftButtonUp struct {
	X, Y int
}

type eventFirer interface {
	fireEvent(ev event)
}

// -------------------------------------------------------------------------------------
