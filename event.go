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
	coreevent "github.com/goplus/spx/v2/internal/core/event"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/timer"
)

type eventSink = coreevent.Sink

type eventSinks struct {
	*eventSinkMgr
	pthis threadObj
}

type eventSinkMgr struct {
	coreevent.Manager
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
	p.eventSinkMgr.DeleteOwner(p.pthis)
}

func (p *eventSinks) doWhenSwipe(direction Direction, target threadObj) {
	p.eventSinkMgr.doWhenSwipe(direction, target)
}

func (p *eventSinks) onAwake(onAwake func()) {
	pthis := p.pthis
	p.eventSinkMgr.AddAwake(coreevent.NewSink(p.pthis, onAwake, coreevent.MatchOwnerOrNil(pthis)))
}

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

func (p *eventSinkMgr) doWhenStart() {
	sinks := p.SnapshotStartOnce()
	if len(sinks) == 0 {
		return
	}
	coreevent.DispatchAsync(sinks, false, nil, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onStart: %s", nameOf(ev.Owner))
		})()
		ev.Handler.(func())()
	})
}

func (p *eventSinkMgr) doWhenAwake(this threadObj) {
	sinks := p.SnapshotAwake()
	coreevent.DispatchSync(sinks, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onAwake: %s", nameOf(ev.Owner))
		})()
		ev.Handler.(func())()
	})
}

func (p *eventSinkMgr) doWhenTimer(time float64) {
	sinks := p.SnapshotTimer()
	coreevent.DispatchAsync(sinks, false, time, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(float64))(time)
	})
}

func (p *eventSinkMgr) doWhenKeyPressed(key Key) {
	sinks := p.SnapshotKeyPressed()
	coreevent.DispatchAsync(sinks, false, key, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(Key))(key)
	})
}

func (p *eventSinkMgr) doWhenSwipe(direction Direction, this threadObj) {
	sinks := p.SnapshotSwipe()
	coreevent.DispatchAsync(sinks, false, direction, eventDispatchHooks(), func(ev *eventSink) {
		if ev.Owner == this {
			ev.Handler.(func(Direction))(direction)
		}
	})
}

func (p *eventSinkMgr) doWhenClick(this threadObj) {
	sinks := p.SnapshotClick()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onClick: %s", nameOf(this))
		})()
		ev.Handler.(func())()
	})
}

func (p *eventSinkMgr) doWhenTouchStart(this threadObj, obj *SpriteImpl) {
	sinks := p.SnapshotTouchStart()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("===> onTouchStart: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenTouching(this threadObj, obj *SpriteImpl) {
	sinks := p.SnapshotTouching()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onTouching: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenTouchEnd(this threadObj, obj *SpriteImpl) {
	sinks := p.SnapshotTouchEnd()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("===> onTouchEnd: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenCloned(this threadObj, data any) {
	sinks := p.SnapshotCloned()
	coreevent.DispatchAsync(sinks, true, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onCloned: %s", nameOf(this))
		})()
		ev.Handler.(func(any))(data)
	})
}

func (p *eventSinkMgr) doWhenIReceive(msg string, data any, wait bool) {
	sinks := p.SnapshotIReceive()
	coreevent.Dispatch(sinks, wait, msg, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(string, any))(msg, data)
	})
}

func (p *eventSinkMgr) doWhenBackdropChanged(name BackdropName, wait bool) {
	sinks := p.SnapshotBackdropChanged()
	coreevent.Dispatch(sinks, wait, name, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(BackdropName))(name)
	})
}

func eventDispatchHooks() coreevent.DispatchHooks {
	return coreevent.DispatchHooks{
		Spawn: func(start bool, owner any, call func()) {
			gco.CreateAndStart(start, owner, func(coroutine.Thread) int {
				call()
				return 0
			})
		},
		Wait: engine.WaitToDo,
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

type StopKind = coreevent.StopKind

const (
	AllStop              StopKind = coreevent.AllStop              // -3: stop all scripts of stage/sprites and abort this script
	AllOtherScripts      StopKind = coreevent.AllOtherScripts      // stop all other scripts
	AllSprites           StopKind = coreevent.AllSprites           // stop all scripts of sprites
	ThisSprite           StopKind = coreevent.ThisSprite           // stop all scripts of this sprite
	ThisScript           StopKind = coreevent.ThisScript           // abort this script
	OtherScriptsInSprite StopKind = coreevent.OtherScriptsInSprite // stop other scripts of this sprite
)

func (p *eventSinks) OnStart(onStart func()) {
	p.eventSinkMgr.AddStart(coreevent.NewSink(p.pthis, onStart))
}

func (p *eventSinks) OnClick(onClick func()) {
	pthis := p.pthis
	p.eventSinkMgr.AddClick(coreevent.NewSink(pthis, onClick, coreevent.MatchOwner(pthis)))
}

func (p *eventSinks) OnAnyKey(onKey func(key Key)) {
	p.eventSinkMgr.AddKeyPressed(coreevent.NewSink(p.pthis, onKey))
}

func (p *eventSinks) OnTimer(time float64, call func()) {
	timer.RegisterTimer(time)
	p.eventSinkMgr.AddTimer(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(call, coreevent.If1(isDebugEventEnabled, func(float64) {
			spxlog.Debug("==> onTimer: %s", nameOf(p.pthis))
		})),
		coreevent.MatchApproxFloat(time, 0.001),
	))
}

func (p *eventSinks) OnKey__0(key Key, onKey func()) {
	p.eventSinkMgr.AddKeyPressed(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onKey, coreevent.If1(isDebugEventEnabled, func(Key) {
			spxlog.Debug("==> onKey: %v, %s", key, nameOf(p.pthis))
		})),
		coreevent.MatchValue(key),
	))
}

func (p *eventSinks) OnSwipe__0(direction Direction, onSwipe func()) {
	p.eventSinkMgr.AddSwipe(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onSwipe, coreevent.If1(isDebugEventEnabled, func(Direction) {
			spxlog.Debug("==> onSwipe: %v, %s", direction, nameOf(p.pthis))
		})),
		coreevent.MatchValue(direction),
	))
}

func (p *eventSinks) OnKey__1(keys []Key, onKey func(Key)) {
	p.eventSinkMgr.AddKeyPressed(coreevent.NewSink(
		p.pthis,
		coreevent.Tap1(onKey, coreevent.If1(isDebugEventEnabled, func(key Key) {
			spxlog.Debug("==> onKey: %v, %s", keys, nameOf(p.pthis))
		})),
		coreevent.MatchAnyOf(keys),
	))
}

func (p *eventSinks) OnKey__2(keys []Key, onKey func()) {
	p.OnKey__1(keys, coreevent.Ignore1[Key](onKey))
}

func (p *eventSinks) OnMsg__0(onMsg func(msg string, data any)) {
	p.eventSinkMgr.AddIReceive(coreevent.NewSink(p.pthis, onMsg))
}

func (p *eventSinks) OnMsg__1(msg string, onMsg func()) {
	p.eventSinkMgr.AddIReceive(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid2(onMsg, coreevent.If2(isDebugEventEnabled, func(msg string, data any) {
			spxlog.Debug("==> onMsg: %s, %s", msg, nameOf(p.pthis))
		})),
		coreevent.MatchValue(msg),
	))
}

func (p *eventSinks) OnBackdrop__0(onBackdrop func(name BackdropName)) {
	p.eventSinkMgr.AddBackdropChanged(coreevent.NewSink(p.pthis, onBackdrop))
}

func (p *eventSinks) OnBackdrop__1(name BackdropName, onBackdrop func()) {
	p.eventSinkMgr.AddBackdropChanged(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onBackdrop, coreevent.If1(isDebugEventEnabled, func(name BackdropName) {
			spxlog.Debug("==> onBackdrop: %s, %s", name, nameOf(p.pthis))
		})),
		coreevent.MatchValue(name),
	))
}

func (p *eventSinks) Stop(kind StopKind) {
	current := gco.Current()
	filter, abort := coreevent.ResolveStop(
		kind,
		p.pthis,
		func(obj any) bool { return isSprite(obj) },
		func(obj any) bool { return isGame(obj) },
	)
	if filter != nil {
		gco.StopIf(func(th coroutine.Thread) bool {
			return filter(th.Obj, th == current)
		})
	}
	if abort {
		gco.Abort()
	}
}

type eventQueuePolicy = coreevent.QueuePolicy

const defaultEventQueuePolicy = coreevent.DefaultQueuePolicy

func parseEventQueuePolicy(policy string) eventQueuePolicy {
	return coreevent.ParsePolicy(policy)
}

type eventQueueSnapshot = coreevent.QueueSnapshot

func (p *Game) initEventQueueState() {
	p.EventQueuePolicy = defaultEventQueuePolicy
	p.EventQueueStats.Reset()
}

func (p *Game) resetEventQueueStats() {
	p.EventQueueStats.Reset()
}

func (p *Game) setEventQueuePolicy(policy eventQueuePolicy) {
	p.EventQueuePolicy = policy
}

func (p *Game) eventQueueSnapshot() eventQueueSnapshot {
	queueLen, queueCap := 0, 0
	if p.events != nil {
		queueLen = len(p.events)
		queueCap = cap(p.events)
	}
	return coreevent.Snapshot(p.EventQueuePolicy, &p.EventQueueStats, queueLen, queueCap)
}

func (p *Game) queueEventWithPolicy(ev event) bool {
	return coreevent.EnqueueWithPolicy(p.events, ev, p.EventQueuePolicy, &p.EventQueueStats)
}
