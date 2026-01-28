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

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	gtime "github.com/goplus/spx/v2/internal/time"
	"github.com/goplus/spx/v2/internal/timer"
)

// -----------------------------------------------------------------------------
// Event Loop System

type clicker interface {
	threadObj
	doWhenClick(this threadObj)
	getProxy() *engine.Sprite
	Visible() bool
}

func (p *Game) doWhenLeftButtonUp(ev *eventLeftButtonUp) {
	point := ev.Pos
	p.inputs.checkTracking(point)
}

func (p *Game) doWhenLeftButtonDown(ev *eventLeftButtonDown) {
	point := ev.Pos

	// Detect target sprite for both swipe and click events
	tempItems := p.getTempShapes()
	count := len(tempItems)

	var target clicker = nil
	var targetSprite *SpriteImpl = nil
	for i := range count {
		item := tempItems[count-i-1]
		if o, ok := item.(clicker); ok {
			syncSprite := o.getProxy()
			if syncSprite != nil && o.Visible() {
				isClicked := spriteMgr.CheckCollisionWithPoint(syncSprite.GetId(), point, true)
				if isClicked {
					target = o
					// Try to get the SpriteImpl from the clicker
					if sprite, ok := o.(*SpriteImpl); ok {
						targetSprite = sprite
					}
					break
				}
			}
		}
	}

	// Start swipe tracking with detected target sprite (can be nil for stage swipes)
	p.inputs.startTracking(point, targetSprite)

	// add a global click cooldown
	if !p.inputs.canTriggerClickEvent(inputGlobalClickTimerId) {
		return
	}

	if target != nil {
		syncSprite := target.getProxy()
		if p.inputs.canTriggerClickEvent(syncSprite.GetId()) {
			target.doWhenClick(target)
		}
	} else {
		if p.inputs.canTriggerClickEvent(inputStageClickTimerId) {
			p.sinkMgr.doWhenClick(p)
		}
	}
}

func (p *Game) doWhenMouseMove(ev *eventMouseMove) {
	// Update current mouse position
	p.mousePos = ev.Pos
	// If swipe tracking is active, record the movement
	p.inputs.onMouseMove(ev.Pos)
}

// handleEvent dispatches events to their respective handlers
func (p *Game) handleEvent(ev event) {
	switch e := ev.(type) {
	case *eventLeftButtonUp:
		p.doWhenLeftButtonUp(e)
	case *eventLeftButtonDown:
		p.doWhenLeftButtonDown(e)
	case *eventMouseMove:
		p.doWhenMouseMove(e)
	case *eventKeyDown:
		p.sinkMgr.doWhenKeyPressed(e.Key)
	case *eventStart:
		p.sinkMgr.doWhenAwake(nil)
		p.sinkMgr.doWhenStart()
	case *eventTimer:
		p.sinkMgr.doWhenTimer(e.Time)
	}
}

func (p *Game) fireEvent(ev event) {
	select {
	case p.events <- ev:
	default:
		if debugInstr {
			spxlog.Warn("Event buffer is full. Skip event: %v", ev)
		}
	}
}

func (p *Game) eventLoop(me coroutine.Thread) int {
	for {
		var ev event
		engine.WaitForChan(p.events, &ev)
		p.handleEvent(ev)
	}
}

// processPendingAudios plays any pending audio for sprites
func (p *Game) processPendingAudios(items []Shape, tempAudios []string) []string {
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			engine.Lock()
			tempAudios = sprite.sound().takePendingAudios(tempAudios)
			engine.Unlock()

			for _, audio := range tempAudios {
				sprite.playAudio(audio, false)
			}
			tempAudios = tempAudios[:0]
		}
	}
	return tempAudios
}

// processAnimationEvents handles completed animation events for sprites
func (p *Game) processAnimationEvents(items []Shape, tempAnimations []string) []string {
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			engine.Lock()
			tempAnimations = sprite.animation().takeDonedAnimations(tempAnimations)
			engine.Unlock()

			for _, animName := range tempAnimations {
				sprite.onAnimationDone(animName)
			}
			tempAnimations = tempAnimations[:0]
		}
	}
	return tempAnimations
}

func (p *Game) logicLoop(me coroutine.Thread) int {
	tempAudios := []string{}
	tempAnimations := []string{}
	for {
		p.camera.onUpdate(gtime.DeltaTime())
		tempItems := p.getTempShapes()

		tempAudios = p.processPendingAudios(tempItems, tempAudios)
		tempAnimations = p.processAnimationEvents(tempItems, tempAnimations)

		if targetTimer := timer.CheckTimerEvent(); targetTimer >= 0 {
			p.fireEvent(&eventTimer{Time: targetTimer})
		}
		engine.WaitNextFrame()
		p.showDebugPanel()
	}
}

func (p *Game) inputEventLoop(me coroutine.Thread) int {
	lastLbtnPressed := false
	lastMousePos := mathf.Vec2{} // Track last mouse position
	keyEvents := make([]engine.KeyEvent, 0)

	for {
		// Check mouse button state
		curLbtnPressed := inputMgr.GetMouseState(MOUSE_BUTTON_LEFT)
		if curLbtnPressed != lastLbtnPressed {
			if lastLbtnPressed {
				p.fireEvent(&eventLeftButtonUp{Pos: p.mousePos})
			} else {
				p.fireEvent(&eventLeftButtonDown{Pos: p.mousePos})
			}
		}
		lastLbtnPressed = curLbtnPressed

		// Check mouse movement
		// Note: We need to get the actual current mouse position from the engine
		// For now, we'll use the stored mousePos which should be updated elsewhere
		curMousePos := inputMgr.GetGlobalMousePos()
		mathfMousePos := mathf.Vec2{X: float64(curMousePos.X), Y: float64(curMousePos.Y)}

		// Check if mouse moved significantly
		dx := mathfMousePos.X - lastMousePos.X
		dy := mathfMousePos.Y - lastMousePos.Y
		if math.Abs(dx) > mouseMovementThreshold || math.Abs(dy) > mouseMovementThreshold {
			p.inputs.onMouseMove(mathfMousePos)
			lastMousePos = mathfMousePos
		}

		// Handle keyboard events
		keyEvents = engine.GetKeyEvents(keyEvents)
		for _, ev := range keyEvents {
			if ev.IsPressed {
				p.fireEvent(&eventKeyDown{Key: Key(ev.Id)})
			} else {
				p.fireEvent(&eventKeyUp{Key: Key(ev.Id)})
			}
		}
		keyEvents = keyEvents[:0]
		engine.WaitNextFrame()
	}
}

func (p *Game) initEventLoop() {
	gco.Create(nil, p.eventLoop)
	gco.Create(nil, p.inputEventLoop)
	gco.Create(nil, p.logicLoop)
}
