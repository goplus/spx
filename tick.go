package spx

import (
	"sync/atomic"
	"unsafe"

	"github.com/goplus/spx/internal/coroutine"
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
	p.prev, p.next = nil, nil
}

func (p *tickHandlerBase) insertNext(h *tickHandler) *tickHandler {
	next := p.next
	h.prev, h.next = p, next
	this := (*tickHandlerBase)(unsafe.Pointer(h))
	p.next, next.prev = this, this
	return h
}

type tickHandler struct {
	tickHandlerBase
	base      int64
	totalTick int64
	onTick    func(tick int64) // tick = 1..totalTick
}

// Stop stops listening `onTick` event.
func (p *tickHandler) Stop() {
	p.removeFromList()
}

// -------------------------------------------------------------------------------------

type tickMgr struct {
	tick int64
	list tickHandlerBase
}

func (p *tickMgr) init() {
	p.list.initList()
}

func (p *tickMgr) start(totalTick int64, onTick func(tick int64)) *tickHandler {
	base := atomic.LoadInt64(&p.tick)
	return p.list.insertNext(&tickHandler{
		base:      base,
		totalTick: totalTick,
		onTick:    onTick,
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
			if tick >= this.totalTick {
				h.removeFromList()
			}
			this.onTick(tick)
		}
		return 0
	})
}

// -------------------------------------------------------------------------------------
