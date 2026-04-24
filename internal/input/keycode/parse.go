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

package keycode

var stringMap = map[string]int64{
	"0": Key0, "1": Key1, "2": Key2, "3": Key3, "4": Key4,
	"5": Key5, "6": Key6, "7": Key7, "8": Key8, "9": Key9,
	"A": KeyA, "B": KeyB, "C": KeyC, "D": KeyD, "E": KeyE,
	"F": KeyF, "G": KeyG, "H": KeyH, "I": KeyI, "J": KeyJ,
	"K": KeyK, "L": KeyL, "M": KeyM, "N": KeyN, "O": KeyO,
	"P": KeyP, "Q": KeyQ, "R": KeyR, "S": KeyS, "T": KeyT,
	"U": KeyU, "V": KeyV, "W": KeyW, "X": KeyX, "Y": KeyY, "Z": KeyZ,
	"a": KeyA, "b": KeyB, "c": KeyC, "d": KeyD, "e": KeyE,
	"f": KeyF, "g": KeyG, "h": KeyH, "i": KeyI, "j": KeyJ,
	"k": KeyK, "l": KeyL, "m": KeyM, "n": KeyN, "o": KeyO,
	"p": KeyP, "q": KeyQ, "r": KeyR, "s": KeyS, "t": KeyT,
	"u": KeyU, "v": KeyV, "w": KeyW, "x": KeyX, "y": KeyY, "z": KeyZ,
	"Apostrophe": KeyApostrophe, "'": KeyApostrophe,
	"Backslash": KeyBackslash, "\\": KeyBackslash,
	"Backspace": KeyBackspace,
	"CapsLock":  KeyCapsLock,
	"Comma":     KeyComma, ",": KeyComma,
	"Delete": KeyDelete, "Del": KeyDelete,
	"Down":  KeyDown,
	"End":   KeyEnd,
	"Enter": KeyEnter, "Return": KeyEnter,
	"Equal": KeyEqual, "=": KeyEqual,
	"Escape": KeyEscape, "Esc": KeyEscape,
	"Exclam": KeyExclam, "!": KeyExclam,
	"F1": KeyF1, "F2": KeyF2, "F3": KeyF3, "F4": KeyF4,
	"F5": KeyF5, "F6": KeyF6, "F7": KeyF7, "F8": KeyF8,
	"F9": KeyF9, "F10": KeyF10, "F11": KeyF11, "F12": KeyF12,
	"GraveAccent": KeyGraveAccent, "`": KeyGraveAccent,
	"Home":   KeyHome,
	"Insert": KeyInsert, "Ins": KeyInsert,
	"KP0": KeyKP0, "KP1": KeyKP1, "KP2": KeyKP2, "KP3": KeyKP3, "KP4": KeyKP4,
	"KP5": KeyKP5, "KP6": KeyKP6, "KP7": KeyKP7, "KP8": KeyKP8, "KP9": KeyKP9,
	"KPDecimal": KeyKPDecimal, "KPPeriod": KeyKPDecimal,
	"KPDivide": KeyKPDivide, "KP/": KeyKPDivide,
	"KPEnter": KeyKPEnter,
	"KPEqual": KeyKPEqual, "KP=": KeyKPEqual,
	"KPMultiply": KeyKPMultiply, "KP*": KeyKPMultiply,
	"KPSubtract": KeyKPSubtract, "KP-": KeyKPSubtract,
	"Left":        KeyLeft,
	"LeftBracket": KeyLeftBracket, "[": KeyLeftBracket,
	"Menu":  KeyMenu,
	"Minus": KeyMinus, "-": KeyMinus,
	"NumLock":  KeyNumLock,
	"PageDown": KeyPageDown, "PgDn": KeyPageDown,
	"PageUp": KeyPageUp, "PgUp": KeyPageUp,
	"Pause":  KeyPause,
	"Period": KeyPeriod, ".": KeyPeriod,
	"PrintScreen": KeyPrintScreen, "Print": KeyPrintScreen,
	"Right":        KeyRight,
	"RightBracket": KeyRightBracket, "]": KeyRightBracket,
	"ScrollLock": KeyScrollLock,
	"Semicolon":  KeySemicolon, ";": KeySemicolon,
	"Slash": KeySlash, "/": KeySlash,
	"Space": KeySpace, " ": KeySpace,
	"Tab":     KeyTab,
	"Up":      KeyUp,
	"Alt":     KeyAlt,
	"Control": KeyControl, "Ctrl": KeyControl,
	"Shift": KeyShift,
}

// Parse converts a string key name into a low-level key code.
func Parse(key string) (int64, bool) {
	code, ok := stringMap[key]
	return code, ok
}
