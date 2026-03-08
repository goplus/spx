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
)

func (p *Game) inputEventLoop(me coroutine.Thread) int {
	return coreruntime.RunInputLoop(me, coreruntime.InputLoopConfig{
		CurrentMousePos: func() mathf.Vec2 {
			curMousePos := p.engine().InputMgr.GetGlobalMousePos()
			return mathf.Vec2{X: float64(curMousePos.X), Y: float64(curMousePos.Y)}
		},
		IsLeftButtonPressed: func() bool {
			return p.engine().InputMgr.GetMouseState(MOUSE_BUTTON_LEFT)
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
