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
	"log"
	"math"
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

type eventMouseMove struct {
	Pos mathf.Vec2
}

type eventFirer interface {
	fireEvent(ev event)
}

// -------------------------------------------------------------------------------------

type inputManager struct {
	tempItems []Shape
	g         *Game
	id2Timer  map[gdx.Object]int64

	swipeRecognizer inputSwipeRecognizer
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

	p.swipeRecognizer.init()
}

func (p *inputManager) startTracking(startPos mathf.Vec2, targetSprite *SpriteImpl) {
	p.swipeRecognizer.startTracking(startPos, targetSprite)
}

func (p *inputManager) checkTracking(point mathf.Vec2) {
	// check swipe gesture
	p.checkSwipe(point)
}

func (p *inputManager) checkSwipe(point mathf.Vec2) {
	// check swipe gesture
	swiper := p.swipeRecognizer
	// Check for swipe completion
	if swiper.isTracking {
		swiper.isTracking = false
		swiper.endPoint = point
		if swiper.checkForSwipeCompletion() {
			var targetName string
			if swiper.targetSprite != nil {
				targetName = swiper.targetSprite.name
			} else {
				targetName = "stage"
			}

			if debugEvent {
				log.Printf("Swipe detected: direction=%v, velocity=%.2f, distance=%.2f, target=%s",
					swiper.detectedDirection, swiper.swipeVelocity, swiper.swipeDistance, targetName)
			}

			// Trigger sprite or stage swipe events through sinkMgr
			if swiper.targetSprite != nil {
				// Trigger swipe event on the specific sprite only
				swiper.targetSprite.doWhenSwipe(swiper.detectedDirection, swiper.targetSprite)
			} else {
				// Trigger swipe event on the stage (game) only
				p.g.sinkMgr.doWhenSwipe(swiper.detectedDirection, p.g)
			}
		}
		p.swipeRecognizer.stopTracking()
	}
}

func (p *inputManager) onMouseMove(pos mathf.Vec2) {
	if p.swipeRecognizer.isTracking {
		p.swipeRecognizer.onMouseMove(pos)
	}
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

// -----------------------------------------------------------------------------
// inputSwipeRecognizer methods

// inputSwipeRecognizer handles swipe gesture detection
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

	// Callback for swipe detection
	onSwipeCallback func(direction Direction, velocity float64, distance float64, startPos, endPos mathf.Vec2, targetSprite *SpriteImpl)
}

// initinputSwipeRecognizer initializes the swipe recognizer with default settings
func (sr *inputSwipeRecognizer) init() {
	sr.timeToSwipe = 0.5 // 500ms default
	sr.enableTimeLimit = true
	sr.minimumDistance = 50.0             // 50 pixels minimum
	sr.maximumDistance = 500.0            // 500 pixels maximum
	sr.triggerWhenCriteriaMet = false     // trigger on mouse up only
	sr.points = make([]mathf.Vec2, 0, 50) // pre-allocate for better performance
}

// setSwipeConfig configures the swipe recognizer parameters
func (p *inputSwipeRecognizer) setSwipeConfig(timeToSwipe, minDistance, maxDistance float64) {
	p.timeToSwipe = timeToSwipe
	p.minimumDistance = minDistance
	p.maximumDistance = maxDistance
}

// startTracking begins swipe tracking
func (sr *inputSwipeRecognizer) startTracking(startPos mathf.Vec2, targetSprite *SpriteImpl) {
	sr.isTracking = true
	sr.startTime = time.Now()
	sr.startPoint = startPos
	sr.endPoint = startPos
	sr.points = sr.points[:0] // clear previous points
	sr.points = append(sr.points, startPos)
	sr.targetSprite = targetSprite
	sr.detectedDirection = -1
	sr.swipeVelocity = 0
	sr.swipeDistance = 0
}

// stopTracking ends swipe tracking
func (sr *inputSwipeRecognizer) stopTracking() {
	sr.isTracking = false
	sr.targetSprite = nil // Clear target sprite reference
}

// onMouseMove handles mouse movement during tracking
func (sr *inputSwipeRecognizer) onMouseMove(pos mathf.Vec2) {
	if !sr.isTracking {
		return
	}

	// Check if time limit exceeded
	if sr.enableTimeLimit && sr.timeToSwipe > 0 {
		elapsed := time.Since(sr.startTime).Seconds()
		if elapsed > sr.timeToSwipe {
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

// checkForSwipeCompletion checks if current gesture qualifies as a swipe
func (sr *inputSwipeRecognizer) checkForSwipeCompletion() bool {
	if len(sr.points) < 2 {
		return false
	}

	// 1. Time validation
	if sr.enableTimeLimit && sr.timeToSwipe > 0 {
		elapsed := time.Since(sr.startTime).Seconds()
		if elapsed > sr.timeToSwipe {
			return false
		}
	}

	// 2. Distance calculation
	dx := sr.endPoint.X - sr.startPoint.X
	dy := sr.endPoint.Y - sr.startPoint.Y
	idealDistance := math.Sqrt(dx*dx + dy*dy)
	if idealDistance < sr.minimumDistance || idealDistance > sr.maximumDistance {
		return false
	}
	// 4. Direction calculation
	direction := sr.calculateDirection(sr.startPoint, sr.endPoint)

	// 5. Calculate velocity and distance
	elapsed := time.Since(sr.startTime).Seconds()
	sr.swipeVelocity = idealDistance / elapsed
	sr.swipeDistance = idealDistance
	sr.detectedDirection = direction

	return true
}

// calculateDirection determines swipe direction based on start and end points
func (sr *inputSwipeRecognizer) calculateDirection(startPoint, endPoint mathf.Vec2) Direction {
	delta := endPoint.Sub(startPoint)

	// In screen coordinates: Y increases downward, X increases rightward
	// When finger moves down: delta.Y > 0, should return SwipeDown
	// When finger moves up: delta.Y < 0, should return SwipeUp

	angle := math.Atan2(delta.Y, delta.X) * 180 / math.Pi

	// Normalize angle to 0-360 degrees
	if angle < 0 {
		angle += 360
	}

	// Map angles to 4 basic directions (each direction covers 90°)
	switch {
	case angle >= 315 || angle < 45:
		return Right // 315° - 45° - finger moves right
	case angle >= 45 && angle < 135:
		return Up // 45° - 135° - finger moves down
	case angle >= 135 && angle < 225:
		return Left // 135° - 225° - finger moves left
	case angle >= 225 && angle < 315:
		return Down // 225° - 315° - finger moves up
	default:
		return -1
	}
}

// onSwipeDetected triggers the swipe callback
func (sr *inputSwipeRecognizer) onSwipeDetected() {
	if sr.onSwipeCallback != nil {
		sr.onSwipeCallback(sr.detectedDirection, sr.swipeVelocity, sr.swipeDistance, sr.startPoint, sr.endPoint, sr.targetSprite)
	}
}
