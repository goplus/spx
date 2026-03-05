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
)

// Key represents a keyboard key code.
type Key = engine.KeyCode

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
	id2Timer  map[engine.Object]int64

	swipeRecognizer inputSwipeRecognizer
}

func (p *inputManager) init(g *Game) {
	p.tempItems = make([]Shape, 50)
	p.id2Timer = make(map[engine.Object]int64)
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

func (p *inputManager) canTriggerClickEvent(id engine.Object) bool {
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
