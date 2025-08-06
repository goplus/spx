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
	"sync/atomic"
	"unsafe"

	"github.com/goplus/spx/internal/coroutine"
	"github.com/hajimehoshi/ebiten/v2"
)

// -------------------------------------------------------------------------------------

type tickHandlerBase struct {
	prev, next *tickHandlerBase
}

func (p *tickHandlerBase) initList() {
	p.prev, p.next = p, p
}

func (p *tickHandlerBase) removeFromList() {
	prev, next := p.prev, p.next
	prev.next, next.prev = next, prev
	p.prev, p.next = p, p
}

func (p *tickHandlerBase) insertNext(this *tickHandler) *tickHandler {
	next := p.next
	h := (*tickHandlerBase)(unsafe.Pointer(this))
	h.prev, h.next = p, next
	p.next, next.prev = h, h
	return this
}

type tickHandler struct {
	tickHandlerBase
	base      int64
	totalTick int64
	onTick    func(tick int64) // tick = 1..totalTick
	self      coroutine.Thread
}

// Stop stops listening `onTick` event.
func (p *tickHandler) Stop() {
	p.removeFromList()
}

// -------------------------------------------------------------------------------------

type tickMgr struct {
	tick       int64
	currentTPS float64
	list       tickHandlerBase
}

// currentTPS is the current TPS (ticks per second),
// that represents how many update function is called in a second.
func getCurrentTPS() float64 {
	if tps := ebiten.CurrentTPS(); tps != 0 {
		return tps
	}
	return ebiten.DefaultTPS
}

func (p *tickMgr) init() {
	p.currentTPS = getCurrentTPS()
	p.list.initList()
}

func (p *tickMgr) start(totalTick int64, onTick func(tick int64)) *tickHandler {
	if totalTick == -1 {
		totalTick = (1 << 63) - 1
	}
	base := atomic.LoadInt64(&p.tick)
	return p.list.insertNext(&tickHandler{
		base:      base,
		totalTick: totalTick,
		onTick:    onTick,
		self:      gco.Current(),
	})
}

func (p *tickMgr) update() {
	curr := atomic.AddInt64(&p.tick, 1)
	gco.CreateAndStart(true, nil, func(me coroutine.Thread) int {
		var next *tickHandlerBase
		tail := &p.list
		for h := tail.next; h != tail; h = next {
			next = h.next
			this := (*tickHandler)(unsafe.Pointer(h))
			tick := curr - this.base
			if this.self.Stopped() {
				tick = this.totalTick // ensure the last tick is always called
			}
			if tick >= this.totalTick {
				h.removeFromList()
			}
			this.onTick(tick)
		}
		return 0
	})
}

// -------------------------------------------------------------------------------------
