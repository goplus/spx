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
)

func (p *Game) currentMousePos() mathf.Vec2 {
	curMousePos := p.engine().InputMgr.GetGlobalMousePos()
	return mathf.Vec2{X: float64(curMousePos.X), Y: float64(curMousePos.Y)}
}

func (p *Game) handleLeftButtonChange(point mathf.Vec2, lastPressed bool) bool {
	curPressed := p.engine().InputMgr.GetMouseState(MOUSE_BUTTON_LEFT)
	if curPressed == lastPressed {
		return lastPressed
	}
	if lastPressed {
		p.fireEvent(&eventLeftButtonUp{Pos: point})
	} else {
		p.fireEvent(&eventLeftButtonDown{Pos: point})
	}
	return curPressed
}

func (p *Game) handleMouseMove(point, lastMousePos mathf.Vec2) mathf.Vec2 {
	dx := point.X - lastMousePos.X
	dy := point.Y - lastMousePos.Y
	if math.Abs(dx) <= mouseMovementThreshold && math.Abs(dy) <= mouseMovementThreshold {
		return lastMousePos
	}
	// Keep mouse move handling on the input loop path to avoid per-frame event allocation.
	p.inputMgr.setMousePos(point)
	p.inputMgr.onMouseMove(point)
	return point
}

func (p *Game) handleKeyEvents(keyEvents []engine.KeyEvent) {
	for _, ev := range keyEvents {
		if ev.IsPressed {
			p.fireEvent(&eventKeyDown{Key: Key(ev.Id)})
		}
		// Key release events are currently handled via polling (KeyPressed),
		// not via event callbacks. Keep this explicit to avoid silent assumptions.
	}
}

func (p *Game) inputEventLoop(me coroutine.Thread) int {
	lastLbtnPressed := false
	lastMousePos := mathf.Vec2{}
	keyEvents := make([]engine.KeyEvent, 0)

	for {
		point := p.currentMousePos()
		lastLbtnPressed = p.handleLeftButtonChange(point, lastLbtnPressed)
		lastMousePos = p.handleMouseMove(point, lastMousePos)
		keyEvents = engine.GetKeyEvents(keyEvents)
		p.handleKeyEvents(keyEvents)
		keyEvents = keyEvents[:0]
		engine.WaitNextFrame()
	}
}
