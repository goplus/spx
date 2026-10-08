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
	"github.com/goplus/spbase/mathf"
	coreruntime "github.com/goplus/spx/v3/internal/core/runtime"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

func (p *Game) initEventLoop() {
	gco.Create("eventLoop", p.eventLoop)
	gco.Create("inputEventLoop", p.inputEventLoop)
	gco.Create("logicLoop", p.logicLoop)
}

func (p *Game) eventLoop(coroutine.Thread) {
	events := p.events
	for {
		p.handleEvent(engine.WaitForChan(events))
	}
}

func (p *Game) inputEventLoop(coroutine.Thread) {
	hooks := inputFrameEventHooks(p.fireEvent)
	// Mouse movement updates swipe state synchronously; other live events stay queued.
	hooks.OnMouseMove = p.inputMgr.onMouseMove
	coreruntime.RunInputLoop(coreruntime.InputLoopConfig{
		BeginFrame: func() bool {
			return p.currentInputSession() == nil
		},
		CurrentMousePos:        engine.Managers().InputMgr.GetGlobalMousePos,
		InputFrameHooks:        hooks,
		SetMousePos:            p.inputMgr.setMousePos,
		GetMouseInput:          engine.GetMouseInput,
		GetKeyEvents:           engine.GetKeyEvents,
		MouseMovementThreshold: mouseMovementThreshold,
	})
}

func inputFrameEventHooks(emit func(event)) coreruntime.InputFrameHooks {
	return coreruntime.InputFrameHooks{
		FireLeftButtonDown: func(point mathf.Vec2) {
			emit(&eventLeftButtonDown{Pos: point})
		},
		FireLeftButtonUp: func(point mathf.Vec2) {
			emit(&eventLeftButtonUp{Pos: point})
		},
		OnMouseMove: func(point mathf.Vec2) {
			emit(&eventMouseMove{Pos: point})
		},
		OnKeyPressed: func(keyID int64) {
			emit(&eventKeyDown{Key: Key(keyID)})
		},
	}
}

func (p *Game) logicLoop(coroutine.Thread) {
	tempAudios, tempAnimations := []string{}, []string{}
	for {
		tempAudios, tempAnimations = p.processLogicFrame(tempAudios, tempAnimations)
		engine.WaitNextFrame()
		p.showDebugPanel()
	}
}

func (p *Game) processLogicFrame(tempAudios, tempAnimations []string) ([]string, []string) {
	items := p.shapeMgr.getTempShapes()
	// Flush all pending audio before completing any animations.
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			tempAudios = sprite.flushPendingAudios(tempAudios)
		}
	}
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			tempAnimations = sprite.flushCompletedAnimations(tempAnimations)
		}
	}
	for {
		timestamp, ok := itime.NextTimer()
		if !ok {
			break
		}
		p.fireEvent(&eventTimer{Timestamp: timestamp})
	}
	return tempAudios, tempAnimations
}
