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
	"github.com/goplus/spx/v3/internal/engine"
)

func (p *Game) queuePenMove(obj engine.Object, position mathf.Vec2) {
	if p.penSyncBuffer == nil {
		p.engine().PenMgr.MovePenTo(obj, position)
		return
	}
	if p.penSyncBuffer.AddMove(obj, position.X, position.Y) {
		p.flushPenCommands()
	}
}

func (p *Game) queuePenDown(obj engine.Object, moveByMouse bool) {
	if moveByMouse {
		p.penCommandBarrier(func() {
			p.engine().PenMgr.PenDown(obj, true)
		})
		return
	}
	if p.penSyncBuffer == nil {
		p.engine().PenMgr.PenDown(obj, false)
		return
	}
	if p.penSyncBuffer.AddDown(obj, false) {
		p.flushPenCommands()
	}
}

func (p *Game) queuePenUp(obj engine.Object) {
	if p.penSyncBuffer == nil {
		p.engine().PenMgr.PenUp(obj)
		return
	}
	if p.penSyncBuffer.AddUp(obj) {
		p.flushPenCommands()
	}
}

func (p *Game) queuePenColor(obj engine.Object, color mathf.Color) {
	if p.penSyncBuffer == nil {
		p.engine().PenMgr.SetPenColorTo(obj, color)
		return
	}
	if p.penSyncBuffer.AddColor(obj, color.R, color.G, color.B, color.A) {
		p.flushPenCommands()
	}
}

func (p *Game) queuePenSize(obj engine.Object, size float64) {
	if p.penSyncBuffer == nil {
		p.engine().PenMgr.SetPenSizeTo(obj, size)
		return
	}
	if p.penSyncBuffer.AddSetSize(obj, size) {
		p.flushPenCommands()
	}
}

func (p *Game) flushPenCommands() {
	if p.penSyncBuffer == nil {
		return
	}
	p.penSyncBuffer.Flush(p.engine().PenMgr.BatchUpdateCommands)
}

func (p *Game) discardPenCommands() {
	if p.penSyncBuffer != nil {
		p.penSyncBuffer.Discard()
	}
}

func (p *Game) penCommandBarrier(operation func()) {
	if p.penSyncBuffer == nil {
		operation()
		return
	}
	p.penSyncBuffer.Barrier(p.engine().PenMgr.BatchUpdateCommands, operation)
}
