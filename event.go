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
	"log"
	"sync"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/timer"
)

// -------------------------------------------------------------------------------------

type eventSink struct {
	pthis threadObj
	cond  func(any) bool
	sink  any
}

func doDeleteClone(sinks []eventSink, this any) []eventSink {
	n := 0
	for _, sink := range sinks {
		if sink.pthis != this {
			sinks[n] = sink
			n++
		}
	}
	clear(sinks[n:])
	return sinks[:n]
}

func asyncCall(sinks []eventSink, start bool, data any, doSth func(*eventSink)) {
	for _, ev := range sinks {
		if ev.cond == nil || ev.cond(data) {
			gco.CreateAndStart(start, ev.pthis, func(coroutine.Thread) int {
				doSth(&ev)
				return 0
			})
		}
	}
}

func syncCall(sinks []eventSink, data any, doSth func(*eventSink)) {
	var wg sync.WaitGroup
	for _, ev := range sinks {
		if ev.cond == nil || ev.cond(data) {
			wg.Add(1)
			gco.CreateAndStart(false, ev.pthis, func(coroutine.Thread) int {
				defer wg.Done()
				doSth(&ev)
				return 0
			})
		}
	}
	engine.WaitToDo(wg.Wait)
}

func call(sinks []eventSink, wait bool, data any, doSth func(*eventSink)) {
	if wait {
		syncCall(sinks, data, doSth)
	} else {
		asyncCall(sinks, false, data, doSth)
	}
}

// -------------------------------------------------------------------------------------

type eventSinkMgr struct {
	allWhenStart           []eventSink
	allWhenAwake           []eventSink
	allWhenKeyPressed      []eventSink
	allWhenSwipe           []eventSink
	allWhenIReceive        []eventSink
	allWhenBackdropChanged []eventSink
	allWhenCloned          []eventSink
	allWhenTouchStart      []eventSink
	allWhenTouching        []eventSink
	allWhenTouchEnd        []eventSink
	allWhenClick           []eventSink
	allWhenTimer           []eventSink
	calledStart            bool
}

func (p *eventSinkMgr) reset() {
	p.allWhenStart = nil
	p.allWhenAwake = nil
	p.allWhenKeyPressed = nil
	p.allWhenSwipe = nil
	p.allWhenIReceive = nil
	p.allWhenBackdropChanged = nil
	p.allWhenCloned = nil
	p.allWhenTouchStart = nil
	p.allWhenTouching = nil
	p.allWhenTouchEnd = nil
	p.allWhenClick = nil
	p.allWhenTimer = nil
	p.calledStart = false
}

func (p *eventSinkMgr) doDeleteClone(this any) {
	p.allWhenAwake = doDeleteClone(p.allWhenAwake, this)
	p.allWhenStart = doDeleteClone(p.allWhenStart, this)
	p.allWhenKeyPressed = doDeleteClone(p.allWhenKeyPressed, this)
	p.allWhenSwipe = doDeleteClone(p.allWhenSwipe, this)
	p.allWhenIReceive = doDeleteClone(p.allWhenIReceive, this)
	p.allWhenBackdropChanged = doDeleteClone(p.allWhenBackdropChanged, this)
	p.allWhenCloned = doDeleteClone(p.allWhenCloned, this)
	p.allWhenTouchStart = doDeleteClone(p.allWhenTouchStart, this)
	p.allWhenTouching = doDeleteClone(p.allWhenTouching, this)
	p.allWhenTouchEnd = doDeleteClone(p.allWhenTouchEnd, this)
	p.allWhenClick = doDeleteClone(p.allWhenClick, this)
	p.allWhenTimer = doDeleteClone(p.allWhenTimer, this)
}

func (p *eventSinkMgr) doWhenStart() {
	if !p.calledStart {
		p.calledStart = true
		asyncCall(p.allWhenStart, false, nil, func(ev *eventSink) {
			if debugEvent {
				log.Println("==> onStart", nameOf(ev.pthis))
			}
			ev.sink.(func())()
		})
	}
}

func (p *eventSinkMgr) doWhenAwake(this threadObj) {
	syncCall(p.allWhenAwake, this, func(ev *eventSink) {
		if debugEvent {
			log.Println("==> onAwake", nameOf(ev.pthis))
		}
		ev.sink.(func())()
	})
}

func (p *eventSinkMgr) doWhenTimer(time float64) {
	asyncCall(p.allWhenTimer, false, time, func(ev *eventSink) {
		ev.sink.(func(float64))(time)
	})
}

func (p *eventSinkMgr) doWhenKeyPressed(key Key) {
	asyncCall(p.allWhenKeyPressed, false, key, func(ev *eventSink) {
		ev.sink.(func(Key))(key)
	})
}

func (p *eventSinkMgr) doWhenSwipe(direction Direction, this threadObj) {
	asyncCall(p.allWhenSwipe, false, direction, func(ev *eventSink) {
		if ev.pthis == this {
			ev.sink.(func(Direction))(direction)
		}
	})
}

func (p *eventSinkMgr) doWhenClick(this threadObj) {
	asyncCall(p.allWhenClick, false, this, func(ev *eventSink) {
		if debugEvent {
			log.Println("==> onClick", nameOf(this))
		}
		ev.sink.(func())()
	})
}

func (p *eventSinkMgr) doWhenTouchStart(this threadObj, obj *SpriteImpl) {
	asyncCall(p.allWhenTouchStart, false, this, func(ev *eventSink) {
		if debugEvent {
			log.Println("===> onTouchStart", nameOf(this), obj.name)
		}
		ev.sink.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenTouching(this threadObj, obj *SpriteImpl) {
	asyncCall(p.allWhenTouching, false, this, func(ev *eventSink) {
		if debugEvent {
			log.Println("==> onTouching", nameOf(this), obj.name)
		}
		ev.sink.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenTouchEnd(this threadObj, obj *SpriteImpl) {
	asyncCall(p.allWhenTouchEnd, false, this, func(ev *eventSink) {
		if debugEvent {
			log.Println("===> onTouchEnd", nameOf(this), obj.name)
		}
		ev.sink.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenCloned(this threadObj, data any) {
	asyncCall(p.allWhenCloned, true, this, func(ev *eventSink) {
		if debugEvent {
			log.Println("==> onCloned", nameOf(this))
		}
		ev.sink.(func(any))(data)
	})
}

func (p *eventSinkMgr) doWhenIReceive(msg string, data any, wait bool) {
	call(p.allWhenIReceive, wait, msg, func(ev *eventSink) {
		ev.sink.(func(string, any))(msg, data)
	})
}

func (p *eventSinkMgr) doWhenBackdropChanged(name BackdropName, wait bool) {
	call(p.allWhenBackdropChanged, wait, name, func(ev *eventSink) {
		ev.sink.(func(BackdropName))(name)
	})
}

// -------------------------------------------------------------------------------------
type IEventSinks interface {
	OnAnyKey(onKey func(key Key))
	OnBackdrop__0(onBackdrop func(name BackdropName))
	OnBackdrop__1(name BackdropName, onBackdrop func())
	OnClick(onClick func())
	OnKey__0(key Key, onKey func())
	OnKey__1(keys []Key, onKey func(Key))
	OnKey__2(keys []Key, onKey func())
	OnMsg__0(onMsg func(msg string, data any))
	OnMsg__1(msg string, onMsg func())
	OnStart(onStart func())
	OnSwipe__0(direction Direction, onSwipe func())
	OnTimer(time float64, onTimer func())
	Stop(kind StopKind)
}

type eventSinks struct {
	*eventSinkMgr
	pthis threadObj
}

func nameOf(this any) string {
	if spr, ok := this.(*SpriteImpl); ok {
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

// doWhenSwipe triggers swipe events for this specific object
func (p *eventSinks) doWhenSwipe(direction Direction, target threadObj) {
	p.eventSinkMgr.doWhenSwipe(direction, target)
}

// -------------------------------------------------------------------------------------

func (p *eventSinks) OnStart(onStart func()) {
	p.allWhenStart = append(p.allWhenStart, eventSink{
		pthis: p.pthis,
		sink:  onStart,
	})
}

func (p *eventSinks) onAwake(onAwake func()) {
	pthis := p.pthis
	p.allWhenAwake = append(p.allWhenAwake, eventSink{
		pthis: p.pthis,
		sink:  onAwake,
		cond: func(data any) bool {
			return data == nil || data == pthis
		},
	})
}

func (p *eventSinks) OnClick(onClick func()) {
	pthis := p.pthis
	p.allWhenClick = append(p.allWhenClick, eventSink{
		pthis: pthis,
		sink:  onClick,
		cond: func(data any) bool {
			return data == pthis
		},
	})
}

func (p *eventSinks) OnAnyKey(onKey func(key Key)) {
	p.allWhenKeyPressed = append(p.allWhenKeyPressed, eventSink{
		pthis: p.pthis,
		sink:  onKey,
	})
}

func (p *eventSinks) OnTimer(time float64, call func()) {
	timer.RegisterTimer(time)
	p.allWhenTimer = append(p.allWhenTimer, eventSink{
		pthis: p.pthis,
		sink: func(float64) {
			if debugEvent {
				log.Println("==> onTimer", nameOf(p.pthis))
			}
			call()
		},
		cond: func(data any) bool {
			return mathf.Absf(data.(float64)-time) < 0.001
		},
	})
}

func (p *eventSinks) OnKey__0(key Key, onKey func()) {
	p.allWhenKeyPressed = append(p.allWhenKeyPressed, eventSink{
		pthis: p.pthis,
		sink: func(Key) {
			if debugEvent {
				log.Println("==> onKey", key, nameOf(p.pthis))
			}
			onKey()
		},
		cond: func(data any) bool {
			return data.(Key) == key
		},
	})
}

func (p *eventSinks) OnSwipe__0(direction Direction, onSwipe func()) {
	p.allWhenSwipe = append(p.allWhenSwipe, eventSink{
		pthis: p.pthis,
		sink: func(Direction) {
			if debugEvent {
				log.Println("==> onSwipe", direction, nameOf(p.pthis))
			}
			onSwipe()
		},
		cond: func(data any) bool {
			return data.(Direction) == direction
		},
	})
}

func (p *eventSinks) OnKey__1(keys []Key, onKey func(Key)) {
	p.allWhenKeyPressed = append(p.allWhenKeyPressed, eventSink{
		pthis: p.pthis,
		sink: func(key Key) {
			if debugEvent {
				log.Println("==> onKey", keys, nameOf(p.pthis))
			}
			onKey(key)
		},
		cond: func(data any) bool {
			keyIn := data.(Key)
			for _, key := range keys {
				if key == keyIn {
					return true
				}
			}
			return false
		},
	})
}

func (p *eventSinks) OnKey__2(keys []Key, onKey func()) {
	p.OnKey__1(keys, func(Key) {
		onKey()
	})
}

func (p *eventSinks) OnMsg__0(onMsg func(msg string, data any)) {
	p.allWhenIReceive = append(p.allWhenIReceive, eventSink{
		pthis: p.pthis,
		sink:  onMsg,
	})
}

func (p *eventSinks) OnMsg__1(msg string, onMsg func()) {
	p.allWhenIReceive = append(p.allWhenIReceive, eventSink{
		pthis: p.pthis,
		sink: func(msg string, data any) {
			if debugEvent {
				log.Println("==> onMsg", msg, nameOf(p.pthis))
			}
			onMsg()
		},
		cond: func(data any) bool {
			return data.(string) == msg
		},
	})
}

func (p *eventSinks) OnBackdrop__0(onBackdrop func(name BackdropName)) {
	p.allWhenBackdropChanged = append(p.allWhenBackdropChanged, eventSink{
		pthis: p.pthis,
		sink:  onBackdrop,
	})
}

func (p *eventSinks) OnBackdrop__1(name BackdropName, onBackdrop func()) {
	p.allWhenBackdropChanged = append(p.allWhenBackdropChanged, eventSink{
		pthis: p.pthis,
		sink: func(name BackdropName) {
			if debugEvent {
				log.Println("==> onBackdrop", name, nameOf(p.pthis))
			}
			onBackdrop()
		},
		cond: func(data any) bool {
			return data.(BackdropName) == name
		},
	})
}

// -------------------------------------------------------------------------------------

type StopKind int

const (
	_All                 StopKind = All  // stop all scripts of stage/sprites and abort this script
	AllOtherScripts      StopKind = -100 // stop all other scripts
	AllSprites           StopKind = -101 // stop all scripts of sprites
	ThisSprite           StopKind = -102 // stop all scripts of this sprite
	ThisScript           StopKind = -103 // abort this script
	OtherScriptsInSprite StopKind = -104 // stop other scripts of this sprite
)

func (p *eventSinks) Stop(kind StopKind) {
	var filter func(th coroutine.Thread) bool
	switch kind {
	case AllSprites:
		filter = func(th coroutine.Thread) bool {
			return isSprite(th.Obj)
		}
	case ThisSprite:
		this := p.pthis
		filter = func(th coroutine.Thread) bool {
			return th.Obj == this
		}
	case OtherScriptsInSprite:
		this := p.pthis
		filter = func(th coroutine.Thread) bool {
			return th.Obj == this && th != gco.Current()
		}
	case AllOtherScripts:
		filter = func(th coroutine.Thread) bool {
			return (isSprite(th.Obj) || isGame(th.Obj)) && th != gco.Current()
		}
	case All:
		gco.StopIf(func(th coroutine.Thread) bool {
			return isSprite(th.Obj) || isGame(th.Obj)
		})
		fallthrough
	case ThisScript:
		gco.Abort()
	}
	gco.StopIf(filter)
}

func isGame(obj threadObj) bool {
	_, ok := obj.(*Game)
	return ok
}

func isSprite(obj threadObj) bool {
	_, ok := obj.(*SpriteImpl)
	return ok
}

// -------------------------------------------------------------------------------------
