package spx

import (
	"log"
	"sync"

	"github.com/goplus/spx/internal/coroutine"
)

// -------------------------------------------------------------------------------------

type eventSink struct {
	prev  *eventSink
	pthis threadObj
	cond  func(interface{}) bool
	sink  interface{}
}

func (p *eventSink) doDeleteClone(this interface{}) (ret *eventSink) {
	ret = p
	pp := &ret
	for {
		p := *pp
		if p == nil {
			return
		}
		if p.pthis == this {
			*pp = p.prev
		} else {
			pp = &p.prev
		}
	}
}

func (p *eventSink) asyncCall(start bool, data interface{}, doSth func(*eventSink)) {
	for p != nil {
		if p.cond == nil || p.cond(data) {
			copy := p
			gco.CreateAndStart(start, p.pthis, func(coroutine.Thread) int {
				doSth(copy)
				return 0
			})
		}
		p = p.prev
	}
}

func (p *eventSink) syncCall(data interface{}, doSth func(*eventSink)) {
	var wg sync.WaitGroup
	for p != nil {
		if p.cond == nil || p.cond(data) {
			wg.Add(1)
			copy := p
			gco.CreateAndStart(false, p.pthis, func(coroutine.Thread) int {
				defer wg.Done()
				doSth(copy)
				return 0
			})
		}
		p = p.prev
	}
	waitToDo(wg.Wait)
}

func (p *eventSink) call(wait bool, data interface{}, doSth func(*eventSink)) {
	if wait {
		p.syncCall(data, doSth)
	} else {
		p.asyncCall(false, data, doSth)
	}
}

// -------------------------------------------------------------------------------------

type eventSinkMgr struct {
	allWhenStart      *eventSink
	allWhenKeyPressed *eventSink
	allWhenIReceive   *eventSink
	allWhenSceneStart *eventSink
	allWhenCloned     *eventSink
	allWhenTouched    *eventSink
	allWhenClick      *eventSink
	allWhenMoving     *eventSink
	allWhenTurning    *eventSink
	calledStart       bool
}

func (p *eventSinkMgr) reset() {
	p.allWhenStart = nil
	p.allWhenKeyPressed = nil
	p.allWhenIReceive = nil
	p.allWhenSceneStart = nil
	p.allWhenCloned = nil
	p.allWhenTouched = nil
	p.allWhenClick = nil
	p.allWhenMoving = nil
	p.allWhenTurning = nil
	p.calledStart = false
}

func (p *eventSinkMgr) doDeleteClone(this interface{}) {
	p.allWhenStart = p.allWhenStart.doDeleteClone(this)
	p.allWhenKeyPressed = p.allWhenKeyPressed.doDeleteClone(this)
	p.allWhenIReceive = p.allWhenIReceive.doDeleteClone(this)
	p.allWhenSceneStart = p.allWhenSceneStart.doDeleteClone(this)
	p.allWhenCloned = p.allWhenCloned.doDeleteClone(this)
	p.allWhenTouched = p.allWhenTouched.doDeleteClone(this)
	p.allWhenClick = p.allWhenClick.doDeleteClone(this)
	p.allWhenMoving = p.allWhenMoving.doDeleteClone(this)
	p.allWhenTurning = p.allWhenTurning.doDeleteClone(this)
}

func (p *eventSinkMgr) doWhenStart() {
	if !p.calledStart {
		p.calledStart = true
		p.allWhenStart.asyncCall(false, nil, func(ev *eventSink) {
			if debugEvent {
				log.Println("==> onStart", nameOf(ev.pthis))
			}
			ev.sink.(func())()
		})
	}
}

func (p *eventSinkMgr) doWhenKeyPressed(key Key) {
	p.allWhenKeyPressed.asyncCall(false, key, func(ev *eventSink) {
		ev.sink.(func(Key))(key)
	})
}

func (p *eventSinkMgr) doWhenClick(this threadObj) {
	p.allWhenClick.asyncCall(false, this, func(ev *eventSink) {
		if debugEvent {
			log.Println("==> onClick", nameOf(this))
		}
		ev.sink.(func())()
	})
}

func (p *eventSinkMgr) doWhenTouched(this threadObj, obj *Sprite) {
	p.allWhenTouched.asyncCall(false, this, func(ev *eventSink) {
		if debugEvent {
			log.Println("==> onTouched", nameOf(this), obj.name)
		}
		ev.sink.(func(*Sprite))(obj)
	})
}

func (p *eventSinkMgr) doWhenCloned(this threadObj, data interface{}) {
	p.allWhenCloned.asyncCall(true, this, func(ev *eventSink) {
		if debugEvent {
			log.Println("==> onCloned", nameOf(this))
		}
		ev.sink.(func(interface{}))(data)
	})
}

func (p *eventSinkMgr) doWhenMoving(this threadObj, mi *MovingInfo) {
	p.allWhenMoving.asyncCall(true, this, func(ev *eventSink) {
		ev.sink.(func(*MovingInfo))(mi)
	})
}

func (p *eventSinkMgr) doWhenTurning(this threadObj, mi *TurningInfo) {
	p.allWhenTurning.asyncCall(true, this, func(ev *eventSink) {
		ev.sink.(func(*TurningInfo))(mi)
	})
}

func (p *eventSinkMgr) doWhenIReceive(msg string, data interface{}, wait bool) {
	p.allWhenIReceive.call(wait, msg, func(ev *eventSink) {
		ev.sink.(func(string, interface{}))(msg, data)
	})
}

func (p *eventSinkMgr) doWhenSceneStart(name string, wait bool) {
	p.allWhenSceneStart.call(wait, name, func(ev *eventSink) {
		ev.sink.(func(string))(name)
	})
}

// -------------------------------------------------------------------------------------

type eventSinks struct {
	*eventSinkMgr
	pthis threadObj
}

func nameOf(this interface{}) string {
	if spr, ok := this.(*Sprite); ok {
		return spr.name
	}
	if _, ok := this.(*Game); ok {
		return "Game"
	}
	panic("eventSinks: unexpected this object")
}

func (p *eventSinks) init(mgr *eventSinkMgr, this threadObj) {
	p.eventSinkMgr = mgr
	p.pthis = this
}

func (p *eventSinks) initFrom(src *eventSinks, this threadObj) {
	p.eventSinkMgr = src.eventSinkMgr
	p.pthis = this
}

func (p *eventSinks) doDeleteClone() {
	p.eventSinkMgr.doDeleteClone(p.pthis)
}

// -------------------------------------------------------------------------------------

func (p *eventSinks) OnStart(onStart func()) {
	p.allWhenStart = &eventSink{
		prev:  p.allWhenStart,
		pthis: p.pthis,
		sink:  onStart,
	}
}

func (p *eventSinks) OnClick(onClick func()) {
	pthis := p.pthis
	p.allWhenClick = &eventSink{
		prev:  p.allWhenClick,
		pthis: pthis,
		sink:  onClick,
		cond: func(data interface{}) bool {
			return data == pthis
		},
	}
}

func (p *eventSinks) OnAnyKey(onKey func(key Key)) {
	p.allWhenKeyPressed = &eventSink{
		prev:  p.allWhenKeyPressed,
		pthis: p.pthis,
		sink:  onKey,
	}
}

func (p *eventSinks) OnKey__0(key Key, onKey func()) {
	p.allWhenKeyPressed = &eventSink{
		prev:  p.allWhenKeyPressed,
		pthis: p.pthis,
		sink: func(Key) {
			if debugEvent {
				log.Println("==> onKey", key, nameOf(p.pthis))
			}
			onKey()
		},
		cond: func(data interface{}) bool {
			return data.(Key) == key
		},
	}
}

func (p *eventSinks) OnKey__1(keys []Key, onKey func(Key)) {
	p.allWhenKeyPressed = &eventSink{
		prev:  p.allWhenKeyPressed,
		pthis: p.pthis,
		sink: func(key Key) {
			if debugEvent {
				log.Println("==> onKey", keys, nameOf(p.pthis))
			}
			onKey(key)
		},
		cond: func(data interface{}) bool {
			keyIn := data.(Key)
			for _, key := range keys {
				if key == keyIn {
					return true
				}
			}
			return false
		},
	}
}

func (p *eventSinks) OnKey__2(keys []Key, onKey func()) {
	p.OnKey__1(keys, func(Key) {
		onKey()
	})
}

func (p *eventSinks) OnMsg__0(onMsg func(msg string, data interface{})) {
	p.allWhenIReceive = &eventSink{
		prev:  p.allWhenIReceive,
		pthis: p.pthis,
		sink:  onMsg,
	}
}

func (p *eventSinks) OnMsg__1(msg string, onMsg func()) {
	p.allWhenIReceive = &eventSink{
		prev:  p.allWhenIReceive,
		pthis: p.pthis,
		sink: func(msg string, data interface{}) {
			if debugEvent {
				log.Println("==> onMsg", msg, nameOf(p.pthis))
			}
			onMsg()
		},
		cond: func(data interface{}) bool {
			return data.(string) == msg
		},
	}
}

func (p *eventSinks) OnScene__0(onScene func(name string)) {
	p.allWhenSceneStart = &eventSink{
		prev:  p.allWhenSceneStart,
		pthis: p.pthis,
		sink:  onScene,
	}
}

func (p *eventSinks) OnScene__1(name string, onScene func()) {
	p.allWhenSceneStart = &eventSink{
		prev:  p.allWhenSceneStart,
		pthis: p.pthis,
		sink: func(name string) {
			if debugEvent {
				log.Println("==> onScene", name, nameOf(p.pthis))
			}
			onScene()
		},
		cond: func(data interface{}) bool {
			return data.(string) == name
		},
	}
}

// -------------------------------------------------------------------------------------
