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
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/timer"
)

// ============================================================================
// Keyboard Input
// ============================================================================

func (p *Game) KeyPressed(key Key) bool {
	return p.rt().inputMgr.GetKey(int64(key))
}

// ============================================================================
// Mouse Input
// ============================================================================

func (p *Game) MouseX() float64 {
	return p.mousePos.X
}

func (p *Game) MouseY() float64 {
	return p.mousePos.Y
}

func (p *Game) MousePressed() bool {
	return p.rt().inputMgr.MousePressed()
}

func (p *Game) getMousePos() (x, y float64) {
	return p.MouseX(), p.MouseY()
}

// ============================================================================
// User Information
// ============================================================================

func (p *Game) Username() string {
	panic("todo")
}

// ============================================================================
// Timing and Frame Control
// ============================================================================

func (p *Game) WaitNextFrame() float64 {
	return engine.WaitNextFrame()
}

func (p *Game) Wait(secs float64) {
	engine.Wait(secs)
}

func (p *Game) Timer() float64 {
	return timer.Timer()
}

func (p *Game) ResetTimer() {
	timer.ResetTimer()
}
