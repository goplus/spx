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
	"sync"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/timer"
)

// -----------------------------------------------------------------------------
// Public interfaces and types
// -----------------------------------------------------------------------------

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

type StopKind int

const (
	AllStop              StopKind = All  // -3: stop all scripts of stage/sprites and abort this script
	AllOtherScripts      StopKind = -100 // stop all other scripts
	AllSprites           StopKind = -101 // stop all scripts of sprites
	ThisSprite           StopKind = -102 // stop all scripts of this sprite
	ThisScript           StopKind = -103 // abort this script
	OtherScriptsInSprite StopKind = -104 // stop other scripts of this sprite
)

// -----------------------------------------------------------------------------
// Public event handler registration methods
// -----------------------------------------------------------------------------

func (p *eventSinks) OnStart(onStart func()) {
	p.eventSinkMgr.addWhenStart(eventSink{
		pthis: p.pthis,
		sink:  onStart,
	})
}

func (p *eventSinks) OnClick(onClick func()) {
	pthis := p.pthis
	p.eventSinkMgr.addWhenClick(eventSink{
		pthis: pthis,
		sink:  onClick,
		cond: func(data any) bool {
			return data == pthis
		},
	})
}

func (p *eventSinks) OnAnyKey(onKey func(key Key)) {
	p.eventSinkMgr.addWhenKeyPressed(eventSink{
		pthis: p.pthis,
		sink:  onKey,
	})
}

func (p *eventSinks) OnTimer(time float64, call func()) {
	timer.RegisterTimer(time)
	p.eventSinkMgr.addWhenTimer(eventSink{
		pthis: p.pthis,
		sink: func(float64) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onTimer: %s", nameOf(p.pthis))
			}
			call()
		},
		cond: func(data any) bool {
			return mathf.Absf(data.(float64)-time) < 0.001
		},
	})
}

func (p *eventSinks) OnKey__0(key Key, onKey func()) {
	p.eventSinkMgr.addWhenKeyPressed(eventSink{
		pthis: p.pthis,
		sink: func(Key) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onKey: %v, %s", key, nameOf(p.pthis))
			}
			onKey()
		},
		cond: func(data any) bool {
			return data.(Key) == key
		},
	})
}

func (p *eventSinks) OnSwipe__0(direction Direction, onSwipe func()) {
	p.eventSinkMgr.addWhenSwipe(eventSink{
		pthis: p.pthis,
		sink: func(Direction) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onSwipe: %v, %s", direction, nameOf(p.pthis))
			}
			onSwipe()
		},
		cond: func(data any) bool {
			return data.(Direction) == direction
		},
	})
}

func (p *eventSinks) OnKey__1(keys []Key, onKey func(Key)) {
	p.eventSinkMgr.addWhenKeyPressed(eventSink{
		pthis: p.pthis,
		sink: func(key Key) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onKey: %v, %s", keys, nameOf(p.pthis))
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
	p.eventSinkMgr.addWhenIReceive(eventSink{
		pthis: p.pthis,
		sink:  onMsg,
	})
}

func (p *eventSinks) OnMsg__1(msg string, onMsg func()) {
	p.eventSinkMgr.addWhenIReceive(eventSink{
		pthis: p.pthis,
		sink: func(msg string, data any) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onMsg: %s, %s", msg, nameOf(p.pthis))
			}
			onMsg()
		},
		cond: func(data any) bool {
			return data.(string) == msg
		},
	})
}

func (p *eventSinks) OnBackdrop__0(onBackdrop func(name BackdropName)) {
	p.eventSinkMgr.addWhenBackdropChanged(eventSink{
		pthis: p.pthis,
		sink:  onBackdrop,
	})
}

func (p *eventSinks) OnBackdrop__1(name BackdropName, onBackdrop func()) {
	p.eventSinkMgr.addWhenBackdropChanged(eventSink{
		pthis: p.pthis,
		sink: func(name BackdropName) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onBackdrop: %s, %s", name, nameOf(p.pthis))
			}
			onBackdrop()
		},
		cond: func(data any) bool {
			return data.(BackdropName) == name
		},
	})
}

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
	case AllStop:
		gco.StopIf(func(th coroutine.Thread) bool {
			return isSprite(th.Obj) || isGame(th.Obj)
		})
		fallthrough
	case ThisScript:
		gco.Abort()
	}
	if filter != nil {
		gco.StopIf(filter)
	}
}

// -----------------------------------------------------------------------------
// Private types and implementation
// -----------------------------------------------------------------------------

type eventSink struct {
	pthis threadObj
	cond  func(any) bool
	sink  any
}

type eventSinks struct {
	*eventSinkMgr
	pthis threadObj
}

type eventSinkMgr struct {
	mu sync.RWMutex

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

	// cache of pointers returned by sinkBuckets()
	sinkBucketsCache []*[]eventSink
}

// -----------------------------------------------------------------------------
// Private event handler methods
// -----------------------------------------------------------------------------

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

func (p *eventSinks) doWhenSwipe(direction Direction, target threadObj) {
	p.eventSinkMgr.doWhenSwipe(direction, target)
}

func (p *eventSinks) onAwake(onAwake func()) {
	pthis := p.pthis
	p.eventSinkMgr.addWhenAwake(eventSink{
		pthis: p.pthis,
		sink:  onAwake,
		cond: func(data any) bool {
			return data == nil || data == pthis
		},
	})
}

func (p *eventSinkMgr) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, bucket := range p.sinkBuckets() {
		*bucket = nil
	}
	p.calledStart = false
}

func (p *eventSinkMgr) doDeleteClone(this any) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, bucket := range p.sinkBuckets() {
		*bucket = doDeleteClone(*bucket, this)
	}
}

func (p *eventSinkMgr) doWhenStart() {
	sinks := p.snapshotWhenStartOnce()
	if len(sinks) == 0 {
		return
	}
	asyncCall(sinks, false, nil, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("==> onStart: %s", nameOf(ev.pthis))
		}
		ev.sink.(func())()
	})
}

func (p *eventSinkMgr) doWhenAwake(this threadObj) {
	sinks := p.snapshotWhenAwake()
	syncCall(sinks, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("==> onAwake: %s", nameOf(ev.pthis))
		}
		ev.sink.(func())()
	})
}

func (p *eventSinkMgr) doWhenTimer(time float64) {
	sinks := p.snapshotWhenTimer()
	asyncCall(sinks, false, time, func(ev *eventSink) {
		ev.sink.(func(float64))(time)
	})
}

func (p *eventSinkMgr) doWhenKeyPressed(key Key) {
	sinks := p.snapshotWhenKeyPressed()
	asyncCall(sinks, false, key, func(ev *eventSink) {
		ev.sink.(func(Key))(key)
	})
}

func (p *eventSinkMgr) doWhenSwipe(direction Direction, this threadObj) {
	sinks := p.snapshotWhenSwipe()
	asyncCall(sinks, false, direction, func(ev *eventSink) {
		if ev.pthis == this {
			ev.sink.(func(Direction))(direction)
		}
	})
}

func (p *eventSinkMgr) doWhenClick(this threadObj) {
	sinks := p.snapshotWhenClick()
	asyncCall(sinks, false, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("==> onClick: %s", nameOf(this))
		}
		ev.sink.(func())()
	})
}

func (p *eventSinkMgr) doWhenTouchStart(this threadObj, obj *SpriteImpl) {
	sinks := p.snapshotWhenTouchStart()
	asyncCall(sinks, false, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("===> onTouchStart: %s, %s", nameOf(this), obj.name)
		}
		ev.sink.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenTouching(this threadObj, obj *SpriteImpl) {
	sinks := p.snapshotWhenTouching()
	asyncCall(sinks, false, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("==> onTouching: %s, %s", nameOf(this), obj.name)
		}
		ev.sink.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenTouchEnd(this threadObj, obj *SpriteImpl) {
	sinks := p.snapshotWhenTouchEnd()
	asyncCall(sinks, false, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("===> onTouchEnd: %s, %s", nameOf(this), obj.name)
		}
		ev.sink.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenCloned(this threadObj, data any) {
	sinks := p.snapshotWhenCloned()
	asyncCall(sinks, true, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("==> onCloned: %s", nameOf(this))
		}
		ev.sink.(func(any))(data)
	})
}

func (p *eventSinkMgr) doWhenIReceive(msg string, data any, wait bool) {
	sinks := p.snapshotWhenIReceive()
	call(sinks, wait, msg, func(ev *eventSink) {
		ev.sink.(func(string, any))(msg, data)
	})
}

func (p *eventSinkMgr) doWhenBackdropChanged(name BackdropName, wait bool) {
	sinks := p.snapshotWhenBackdropChanged()
	call(sinks, wait, name, func(ev *eventSink) {
		ev.sink.(func(BackdropName))(name)
	})
}

// -----------------------------------------------------------------------------
// Private utility functions
// -----------------------------------------------------------------------------

func nameOf(this any) string {
	if spr, ok := this.(*SpriteImpl); ok {
		return spr.name
	}
	if _, ok := this.(*Game); ok {
		return "Game"
	}
	engine.Panic("eventSinks: unexpected this object")
	return ""
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

func copyEventSinks(sinks []eventSink) []eventSink {
	if len(sinks) == 0 {
		return nil
	}
	out := make([]eventSink, len(sinks))
	copy(out, sinks)
	return out
}

// sinkBuckets returns pointers to all allWhen* slice fields.
// IMPORTANT: Every allWhen* field in eventSinkMgr must be listed here.
// Missing a field will cause reset() and doDeleteClone() to silently skip it.
func (p *eventSinkMgr) sinkBuckets() []*[]eventSink {
	if p.sinkBucketsCache != nil {
		return p.sinkBucketsCache
	}

	p.sinkBucketsCache = []*[]eventSink{
		&p.allWhenStart,
		&p.allWhenAwake,
		&p.allWhenKeyPressed,
		&p.allWhenSwipe,
		&p.allWhenIReceive,
		&p.allWhenBackdropChanged,
		&p.allWhenCloned,
		&p.allWhenTouchStart,
		&p.allWhenTouching,
		&p.allWhenTouchEnd,
		&p.allWhenClick,
		&p.allWhenTimer,
	}
	return p.sinkBucketsCache
}

func (p *eventSinkMgr) addSink(bucket *[]eventSink, sink eventSink) {
	p.mu.Lock()
	*bucket = append(*bucket, sink)
	p.mu.Unlock()
}

func (p *eventSinkMgr) snapshotSinks(sinks []eventSink) []eventSink {
	p.mu.RLock()
	out := copyEventSinks(sinks)
	p.mu.RUnlock()
	return out
}

func (p *eventSinkMgr) addWhenStart(sink eventSink) {
	p.addSink(&p.allWhenStart, sink)
}

func (p *eventSinkMgr) addWhenAwake(sink eventSink) {
	p.addSink(&p.allWhenAwake, sink)
}

func (p *eventSinkMgr) addWhenKeyPressed(sink eventSink) {
	p.addSink(&p.allWhenKeyPressed, sink)
}

func (p *eventSinkMgr) addWhenSwipe(sink eventSink) {
	p.addSink(&p.allWhenSwipe, sink)
}

func (p *eventSinkMgr) addWhenIReceive(sink eventSink) {
	p.addSink(&p.allWhenIReceive, sink)
}

func (p *eventSinkMgr) addWhenBackdropChanged(sink eventSink) {
	p.addSink(&p.allWhenBackdropChanged, sink)
}

func (p *eventSinkMgr) addWhenCloned(sink eventSink) {
	p.addSink(&p.allWhenCloned, sink)
}

func (p *eventSinkMgr) addWhenTouchStart(sink eventSink) {
	p.addSink(&p.allWhenTouchStart, sink)
}

func (p *eventSinkMgr) addWhenClick(sink eventSink) {
	p.addSink(&p.allWhenClick, sink)
}

func (p *eventSinkMgr) addWhenTimer(sink eventSink) {
	p.addSink(&p.allWhenTimer, sink)
}

func (p *eventSinkMgr) snapshotWhenStartOnce() []eventSink {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calledStart {
		return nil
	}
	p.calledStart = true
	return copyEventSinks(p.allWhenStart)
}

func (p *eventSinkMgr) snapshotWhenAwake() []eventSink {
	return p.snapshotSinks(p.allWhenAwake)
}

func (p *eventSinkMgr) snapshotWhenKeyPressed() []eventSink {
	return p.snapshotSinks(p.allWhenKeyPressed)
}

func (p *eventSinkMgr) snapshotWhenSwipe() []eventSink {
	return p.snapshotSinks(p.allWhenSwipe)
}

func (p *eventSinkMgr) snapshotWhenIReceive() []eventSink {
	return p.snapshotSinks(p.allWhenIReceive)
}

func (p *eventSinkMgr) snapshotWhenBackdropChanged() []eventSink {
	return p.snapshotSinks(p.allWhenBackdropChanged)
}

func (p *eventSinkMgr) snapshotWhenCloned() []eventSink {
	return p.snapshotSinks(p.allWhenCloned)
}

func (p *eventSinkMgr) snapshotWhenTouchStart() []eventSink {
	return p.snapshotSinks(p.allWhenTouchStart)
}

func (p *eventSinkMgr) snapshotWhenTouching() []eventSink {
	return p.snapshotSinks(p.allWhenTouching)
}

func (p *eventSinkMgr) snapshotWhenTouchEnd() []eventSink {
	return p.snapshotSinks(p.allWhenTouchEnd)
}

func (p *eventSinkMgr) snapshotWhenClick() []eventSink {
	return p.snapshotSinks(p.allWhenClick)
}

func (p *eventSinkMgr) snapshotWhenTimer() []eventSink {
	return p.snapshotSinks(p.allWhenTimer)
}

func asyncCall(sinks []eventSink, start bool, data any, doSth func(*eventSink)) {
	for _, ev := range sinks {
		ev := ev
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
		ev := ev
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

func isGame(obj threadObj) bool {
	_, ok := obj.(*Game)
	return ok
}

func isSprite(obj threadObj) bool {
	_, ok := obj.(*SpriteImpl)
	return ok
}
