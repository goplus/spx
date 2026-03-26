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
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	itime "github.com/goplus/spx/v2/internal/time"
)

func (p *Game) initEventLoop() {
	coreruntime.InitLoops(gco.Create, p.eventLoop, p.inputEventLoop, p.logicLoop)
}

func (p *Game) eventLoop(me coroutine.Thread) int {
	return coreruntime.RunEventLoop(me, p.events, p.handleEvent)
}

func (p *Game) inputEventLoop(me coroutine.Thread) int {
	return coreruntime.RunInputLoop(me, coreruntime.InputLoopConfig{
		CurrentMousePos: func() mathf.Vec2 {
			curMousePos := p.engine().InputMgr.GetGlobalMousePos()
			return mathf.Vec2{X: float64(curMousePos.X), Y: float64(curMousePos.Y)}
		},
		IsLeftButtonPressed: func() bool {
			return engine.IsMouseButtonPressed(MOUSE_BUTTON_LEFT)
		},
		FireLeftButtonDown: func(point mathf.Vec2) {
			p.fireEvent(&eventLeftButtonDown{Pos: point})
		},
		FireLeftButtonUp: func(point mathf.Vec2) {
			p.fireEvent(&eventLeftButtonUp{Pos: point})
		},
		SetMousePos:  p.inputMgr.setMousePos,
		OnMouseMove:  p.inputMgr.onMouseMove,
		GetKeyEvents: engine.GetKeyEvents,
		OnKeyPressed: func(keyID int64) {
			p.fireEvent(&eventKeyDown{Key: Key(keyID)})
		},
		MouseMovementThreshold: mouseMovementThreshold,
	})
}

func (p *Game) logicLoop(me coroutine.Thread) int {
	return coreruntime.RunLogicLoop(me, coreruntime.LogicLoopConfig[Shape]{
		Items: p.getTempShapes,
		FlushPendingAudio: func(item Shape, tempAudios []string) []string {
			sprite, ok := item.(*SpriteImpl)
			if !ok {
				return tempAudios
			}
			return sprite.flushPendingAudios(tempAudios)
		},
		FlushCompletedAnimations: func(item Shape, tempAnimations []string) []string {
			sprite, ok := item.(*SpriteImpl)
			if !ok {
				return tempAnimations
			}
			return sprite.flushCompletedAnimations(tempAnimations)
		},
		NextTimer: itime.NextTimer,
		FireTimer: func(targetTimer float64) {
			p.fireEvent(&eventTimer{Time: targetTimer})
		},
		ShowDebugPanel: p.showDebugPanel,
	})
}
