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
	"math"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

// Key represents a keyboard key code.
type Key = gdx.KeyCode

// Keyboard key constants
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

// keyStringMap maps string representations to Key codes for parsing.
var keyStringMap = map[string]Key{
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
	"Any":   KeyAny,
}

// KeyFromString converts a string to its corresponding Key code.
// It supports key names like "A", "Space", "Enter", "Left", etc.
// Returns KeyMax if the key name is not recognized.
func KeyFromString(key string) Key {
	if keyCode, ok := keyStringMap[key]; ok {
		return keyCode
	}
	return KeyMax
}

// -------------------------------------------------------------------------------------
// Event types

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

type eventMouseMove struct {
	Pos mathf.Vec2
}

// -------------------------------------------------------------------------------------
// Input Manager

const (
	// Minimum interval between two mouse click events (in milliseconds)
	mouseClickIntervalMs = 50
	// Special timer IDs for click event management
	clickTimerGlobal = -1 // Global click cooldown
	clickTimerStage  = 0  // Stage click cooldown
)

// inputManager handles all input-related operations including mouse, keyboard,
// and gesture recognition.
type inputManager struct {
	tempItems []Shape
	g         *Game
	id2Timer  map[gdx.Object]int64

	swipeRecognizer inputSwipeRecognizer
}

func (p *inputManager) init(g *Game) {
	p.tempItems = make([]Shape, 50)
	p.id2Timer = make(map[gdx.Object]int64)
	p.g = g
	p.swipeRecognizer.init()
}

func (p *inputManager) startTracking(startPos mathf.Vec2, targetSprite *SpriteImpl) {
	p.swipeRecognizer.startTracking(startPos, targetSprite)
}

func (p *inputManager) checkTracking(point mathf.Vec2) {
	p.checkSwipe(point)
}

func (p *inputManager) checkSwipe(point mathf.Vec2) {
	swiper := &p.swipeRecognizer
	if !swiper.isTracking {
		return
	}

	swiper.isTracking = false
	swiper.endPoint = point

	if !swiper.checkForSwipeCompletion() {
		swiper.stopTracking()
		return
	}

	// Determine target name for logging
	targetName := "stage"
	if swiper.targetSprite != nil {
		targetName = swiper.targetSprite.name
	}

	if isDebugEventEnabled() {
		spxlog.Debug("Swipe detected: direction=%v, velocity=%.2f, distance=%.2f, target=%s",
			swiper.detectedDirection, swiper.swipeVelocity, swiper.swipeDistance, targetName)
	}

	// Trigger swipe event on sprite or stage
	if swiper.targetSprite != nil {
		swiper.targetSprite.doWhenSwipe(swiper.detectedDirection, swiper.targetSprite)
	} else {
		p.g.sinkMgr.doWhenSwipe(swiper.detectedDirection, p.g)
	}

	swiper.stopTracking()
}

func (p *inputManager) onMouseMove(pos mathf.Vec2) {
	if p.swipeRecognizer.isTracking {
		p.swipeRecognizer.onMouseMove(pos)
	}
}

func (p *inputManager) canTriggerClickEvent(id gdx.Object) bool {
	currentTimeMs := time.Now().UnixNano() / int64(time.Millisecond)

	if lastTimeMs, ok := p.id2Timer[id]; ok {
		if currentTimeMs-lastTimeMs < mouseClickIntervalMs {
			return false
		}
	}

	p.id2Timer[id] = currentTimeMs
	return true
}

// -------------------------------------------------------------------------------------
// Swipe Gesture Recognizer

// inputSwipeRecognizer handles swipe gesture detection and recognition.
type inputSwipeRecognizer struct {
	// Configuration parameters
	timeToSwipe            float64 // Maximum swipe time in seconds
	enableTimeLimit        bool    // Whether to enable time limit
	minimumDistance        float64 // Minimum swipe distance in pixels
	maximumDistance        float64 // Maximum swipe distance in pixels
	triggerWhenCriteriaMet bool    // Whether to trigger immediately when criteria are met

	// State data
	isTracking   bool
	startTime    time.Time
	startPoint   mathf.Vec2
	endPoint     mathf.Vec2
	points       []mathf.Vec2 // Trajectory points
	targetSprite *SpriteImpl  // The sprite that the swipe is targeting (nil for stage swipes)

	// Output results
	detectedDirection Direction
	swipeVelocity     float64
	swipeDistance     float64

	// Callback for swipe detection (optional)
	onSwipeCallback func(direction Direction, velocity float64, distance float64, startPos, endPos mathf.Vec2, targetSprite *SpriteImpl)
}

// init initializes the swipe recognizer with default settings.
func (sr *inputSwipeRecognizer) init() {
	sr.timeToSwipe = 0.5 // 500ms default
	sr.enableTimeLimit = true
	sr.minimumDistance = 50.0         // 50 pixels minimum
	sr.maximumDistance = 500.0        // 500 pixels maximum
	sr.triggerWhenCriteriaMet = false // Trigger on mouse up only
	sr.points = make([]mathf.Vec2, 0, 50)
}

// setSwipeConfig configures the swipe recognizer parameters.
func (sr *inputSwipeRecognizer) setSwipeConfig(timeToSwipe, minDistance, maxDistance float64) {
	sr.timeToSwipe = timeToSwipe
	sr.minimumDistance = minDistance
	sr.maximumDistance = maxDistance
}

// startTracking begins swipe tracking at the given start position.
func (sr *inputSwipeRecognizer) startTracking(startPos mathf.Vec2, targetSprite *SpriteImpl) {
	sr.isTracking = true
	sr.startTime = time.Now()
	sr.startPoint = startPos
	sr.endPoint = startPos
	sr.points = sr.points[:0] // Clear previous points
	sr.points = append(sr.points, startPos)
	sr.targetSprite = targetSprite
	sr.detectedDirection = -1
	sr.swipeVelocity = 0
	sr.swipeDistance = 0
}

// stopTracking ends swipe tracking and clears target sprite reference.
func (sr *inputSwipeRecognizer) stopTracking() {
	sr.isTracking = false
	sr.targetSprite = nil
}

// onMouseMove handles mouse movement during tracking.
func (sr *inputSwipeRecognizer) onMouseMove(pos mathf.Vec2) {
	if !sr.isTracking {
		return
	}
	// Check if time limit exceeded
	if sr.enableTimeLimit && sr.timeToSwipe > 0 {
		if time.Since(sr.startTime).Seconds() > sr.timeToSwipe {
			sr.stopTracking()
			return
		}
	}
	// Record trajectory point
	sr.points = append(sr.points, pos)
	sr.endPoint = pos
	// Optional: real-time detection
	if sr.triggerWhenCriteriaMet {
		if sr.checkForSwipeCompletion() {
			sr.onSwipeDetected()
			sr.stopTracking()
		}
	}
}

// checkForSwipeCompletion checks if the current gesture qualifies as a swipe.
func (sr *inputSwipeRecognizer) checkForSwipeCompletion() bool {
	if len(sr.points) < 2 {
		return false
	}

	// Calculate elapsed time once for efficiency
	elapsed := time.Since(sr.startTime).Seconds()

	// Time validation
	if sr.enableTimeLimit && sr.timeToSwipe > 0 {
		if elapsed > sr.timeToSwipe {
			return false
		}
	}

	// Distance calculation
	dx := sr.endPoint.X - sr.startPoint.X
	dy := sr.endPoint.Y - sr.startPoint.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	if distance < sr.minimumDistance || distance > sr.maximumDistance {
		return false
	}

	// Direction calculation
	direction := sr.calculateDirection(sr.startPoint, sr.endPoint)

	// Store results (elapsed should always be > 0 since time has passed)
	sr.swipeVelocity = distance / elapsed
	sr.swipeDistance = distance
	sr.detectedDirection = direction

	return true
}

// calculateDirection determines swipe direction based on start and end points.
// The direction is mapped to 4 basic directions (Up, Down, Left, Right).
func (sr *inputSwipeRecognizer) calculateDirection(startPoint, endPoint mathf.Vec2) Direction {
	delta := endPoint.Sub(startPoint)
	// Calculate angle in degrees (0-360)
	angle := engine.RadToDeg(math.Atan2(delta.Y, delta.X))
	if angle < 0 {
		angle += 360
	}
	// Map angles to 4 basic directions (each covers 90°)
	switch {
	case angle >= 315 || angle < 45:
		return Right // Finger moves right
	case angle >= 45 && angle < 135:
		return Up // Finger moves down
	case angle >= 135 && angle < 225:
		return Left // Finger moves left
	case angle >= 225 && angle < 315:
		return Down // Finger moves up
	default:
		return -1
	}
}

// onSwipeDetected triggers the swipe callback if one is registered.
func (sr *inputSwipeRecognizer) onSwipeDetected() {
	if sr.onSwipeCallback != nil {
		sr.onSwipeCallback(sr.detectedDirection, sr.swipeVelocity, sr.swipeDistance,
			sr.startPoint, sr.endPoint, sr.targetSprite)
	}
}
