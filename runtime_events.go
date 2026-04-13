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
	coreevent "github.com/goplus/spx/v2/internal/core/event"
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	itime "github.com/goplus/spx/v2/internal/time"
)

const (
	clickTimerGlobal = -1
	clickTimerStage  = 0
)

var scriptEventDispatchHooks = coreevent.DispatchHooks{
	Spawn: func(start bool, owner any, call func()) {
		gco.CreateAndStart(start, owner, func(coroutine.Thread) int {
			call()
			return 0
		})
	},
	Wait: engine.WaitToDo,
}

// -----------------------------------------------------------------------------
// Event Bindings
// -----------------------------------------------------------------------------
type eventSink = coreevent.Sink

type scriptEventBindings struct {
	*scriptEventRegistry
	pthis threadObj
}

type scriptEventRegistry struct {
	coreevent.Manager
}

func (p *scriptEventBindings) init(mgr *scriptEventRegistry, this threadObj) {
	p.scriptEventRegistry = mgr
	p.pthis = this
}

func (p *scriptEventBindings) initFrom(src *scriptEventBindings, this threadObj) {
	p.scriptEventRegistry = src.scriptEventRegistry
	p.pthis = this
}

func (p *scriptEventRegistry) dispatchAsync(bucket coreevent.Bucket, start bool, data any, do func(*eventSink)) {
	p.DispatchBucketAsync(bucket, start, data, scriptEventDispatchHooks, do)
}

func (p *scriptEventRegistry) dispatchSync(bucket coreevent.Bucket, data any, do func(*eventSink)) {
	p.DispatchBucketSync(bucket, data, scriptEventDispatchHooks, do)
}

func (p *scriptEventRegistry) dispatch(bucket coreevent.Bucket, wait bool, data any, do func(*eventSink)) {
	p.DispatchBucket(bucket, wait, data, scriptEventDispatchHooks, do)
}

func (p *scriptEventRegistry) dispatchStartOnce(data any, do func(*eventSink)) {
	p.DispatchStartOnce(data, scriptEventDispatchHooks, do)
}

func (p *scriptEventBindings) doDeleteClone() {
	p.scriptEventRegistry.DeleteOwner(p.pthis)
}

func (p *scriptEventBindings) doWhenSwipe(direction Direction, target threadObj) {
	p.scriptEventRegistry.doWhenSwipe(direction, target)
}

func (p *scriptEventBindings) onAwake(onAwake func()) {
	pthis := p.pthis
	p.scriptEventRegistry.AddAwake(coreevent.NewSink(p.pthis, onAwake, coreevent.MatchOwnerOrNil(pthis)))
}

func (p *scriptEventBindings) OnStart(onStart func()) {
	sink := coreevent.NewSink(p.pthis, onStart)
	if p.scriptEventRegistry.TryAddStart(sink) {
		return
	}
	if sprite, ok := sink.Owner.(*SpriteImpl); ok && sprite.spriteState.Cloned {
		return
	}
	spxlog.Warn("event: ignoring late OnStart registration for %s", nameOf(sink.Owner))
}

func (p *scriptEventBindings) OnClick(onClick func()) {
	pthis := p.pthis
	p.scriptEventRegistry.AddClick(coreevent.NewSink(pthis, onClick, coreevent.MatchOwner(pthis)))
}

func (p *scriptEventBindings) OnAnyKey(onKey func(key Key)) {
	p.scriptEventRegistry.AddKeyPressed(coreevent.NewSink(p.pthis, onKey))
}

func (p *scriptEventBindings) OnTimer(time float64, call func()) {
	itime.RegisterTimer(time)
	p.scriptEventRegistry.AddTimer(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(call, coreevent.If1(isDebugEventEnabled, func(float64) {
			spxlog.Debug("==> onTimer: %s", nameOf(p.pthis))
		})),
		coreevent.MatchApproxFloat(time, 0.001),
	))
}

func (p *scriptEventBindings) OnKey__0(key Key, onKey func()) {
	p.scriptEventRegistry.AddKeyPressed(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onKey, coreevent.If1(isDebugEventEnabled, func(Key) {
			spxlog.Debug("==> onKey: %v, %s", key, nameOf(p.pthis))
		})),
		coreevent.MatchValue(key),
	))
}

func (p *scriptEventBindings) OnSwipe__0(direction Direction, onSwipe func()) {
	p.scriptEventRegistry.AddSwipe(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onSwipe, coreevent.If1(isDebugEventEnabled, func(Direction) {
			spxlog.Debug("==> onSwipe: %v, %s", direction, nameOf(p.pthis))
		})),
		coreevent.MatchValue(direction),
	))
}

func (p *scriptEventBindings) OnKey__1(keys []Key, onKey func(Key)) {
	p.scriptEventRegistry.AddKeyPressed(coreevent.NewSink(
		p.pthis,
		coreevent.Tap1(onKey, coreevent.If1(isDebugEventEnabled, func(key Key) {
			spxlog.Debug("==> onKey: %v, %s", keys, nameOf(p.pthis))
		})),
		coreevent.MatchAnyOf(keys),
	))
}

func (p *scriptEventBindings) OnKey__2(keys []Key, onKey func()) {
	p.OnKey__1(keys, coreevent.Ignore1[Key](onKey))
}

func (p *scriptEventBindings) OnMsg__0(onMsg func(msg MsgName, data any)) {
	p.scriptEventRegistry.AddIReceive(coreevent.NewSink(p.pthis, onMsg))
}

func (p *scriptEventBindings) OnMsg__1(msg MsgName, onMsg func()) {
	p.scriptEventRegistry.AddIReceive(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid2(onMsg, coreevent.If2(isDebugEventEnabled, func(msg string, data any) {
			spxlog.Debug("==> onMsg: %s, %s", msg, nameOf(p.pthis))
		})),
		coreevent.MatchValue(msg),
	))
}

func (p *scriptEventBindings) OnBackdrop__0(onBackdrop func(name BackdropName)) {
	p.scriptEventRegistry.AddBackdropChanged(coreevent.NewSink(p.pthis, onBackdrop))
}

func (p *scriptEventBindings) OnBackdrop__1(name BackdropName, onBackdrop func()) {
	p.scriptEventRegistry.AddBackdropChanged(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onBackdrop, coreevent.If1(isDebugEventEnabled, func(name BackdropName) {
			spxlog.Debug("==> onBackdrop: %s, %s", name, nameOf(p.pthis))
		})),
		coreevent.MatchValue(name),
	))
}

func (p *scriptEventBindings) Stop(kind StopKind) {
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

// -----------------------------------------------------------------------------
// Click Dispatch
// -----------------------------------------------------------------------------
type clicker interface {
	threadObj
	doWhenClick(this threadObj)
	getProxy() *engine.Sprite
	Visible() bool
}

func (p *Game) findClickTarget(point mathf.Vec2) (coreruntime.ClickSelection[clicker, *SpriteImpl], bool) {
	return coreruntime.FindClickTarget(p.getTempShapes(), func(item Shape) (coreruntime.ClickSelection[clicker, *SpriteImpl], bool) {
		o, ok := item.(clicker)
		if !ok {
			return coreruntime.ClickSelection[clicker, *SpriteImpl]{}, false
		}
		syncSprite := o.getProxy()
		if syncSprite == nil || !o.Visible() {
			return coreruntime.ClickSelection[clicker, *SpriteImpl]{}, false
		}
		if !p.engine().SpriteMgr.CheckCollisionWithPoint(syncSprite.GetId(), point, true) {
			return coreruntime.ClickSelection[clicker, *SpriteImpl]{}, false
		}
		if sprite, ok := o.(*SpriteImpl); ok {
			return coreruntime.ClickSelection[clicker, *SpriteImpl]{Target: o, SwipeTarget: sprite}, true
		}
		return coreruntime.ClickSelection[clicker, *SpriteImpl]{Target: o}, true
	})
}

func (p *Game) doWhenLeftButtonUp(ev *eventLeftButtonUp) {
	p.inputMgr.finishSwipeTracking(ev.Pos)
}

func (p *Game) doWhenLeftButtonDown(ev *eventLeftButtonDown) {
	coreruntime.HandleLeftButtonDown(ev.Pos, coreruntime.ClickDownHooks[clicker, *SpriteImpl, int64]{
		FindTarget: p.findClickTarget,
		BeginSwipe: p.inputMgr.beginSwipeTracking,
		CanTrigger: func(id int64) bool {
			return p.inputMgr.canTriggerClickEvent(id)
		},
		GlobalID: clickTimerGlobal,
		StageID:  clickTimerStage,
		TargetID: func(target clicker) (int64, bool) {
			syncSprite := target.getProxy()
			if syncSprite == nil {
				return 0, false
			}
			return syncSprite.GetId(), true
		},
		DispatchTarget: func(target clicker) {
			target.doWhenClick(target)
		},
		DispatchStage: func() {
			p.scriptEvents.doWhenClick(p)
		},
	})
}

// -----------------------------------------------------------------------------
// Message Broadcast
// -----------------------------------------------------------------------------
func (p *Game) Broadcast__0(msg MsgName) {
	p.doBroadcast(msg, nil, false)
}

func (p *Game) Broadcast__1(msg MsgName, data any) {
	p.doBroadcast(msg, data, false)
}

func (p *Game) BroadcastAndWait__0(msg MsgName) {
	p.doBroadcast(msg, nil, true)
}

func (p *Game) BroadcastAndWait__1(msg MsgName, data any) {
	p.doBroadcast(msg, data, true)
}

func (p *Game) doBroadcast(msg MsgName, data any, wait bool) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Broadcast: msg=%s, wait=%v", msg, wait)
	}
	p.scriptEvents.doWhenIReceive(msg, data, wait)
}

// -----------------------------------------------------------------------------
// Event Routing
// -----------------------------------------------------------------------------
func (p *Game) handleEvent(ev event) {
	switch e := ev.(type) {
	case *eventLeftButtonUp:
		p.doWhenLeftButtonUp(e)
	case *eventLeftButtonDown:
		p.doWhenLeftButtonDown(e)
	case *eventKeyDown:
		// Note: key-up callbacks are not part of the current event sink API.
		p.scriptEvents.doWhenKeyPressed(e.Key)
	case *eventStart:
		p.scriptEvents.doWhenAwake(nil)
		p.scriptEvents.doWhenStart()
		p.lifecycleState.StartDispatched.Store(true)
	case *eventTimer:
		p.scriptEvents.doWhenTimer(e.Time)
	}
}

func (p *Game) fireEvent(ev event) {
	if p.queueEventWithPolicy(ev) {
		return
	}
	if isDebugInstrEnabled() {
		spxlog.Warn("Event buffer is full (policy=%s). Drop event: %v", p.gameRuntimeState.EventQueuePolicy, ev)
	}
}

// -----------------------------------------------------------------------------
// Event Dispatch
// -----------------------------------------------------------------------------
func (p *scriptEventRegistry) doWhenStart() {
	p.dispatchStartOnce(nil, func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onStart: %s", nameOf(ev.Owner))
		})()
		ev.Handler.(func())()
	})
}

func (p *scriptEventRegistry) doWhenAwake(this threadObj) {
	p.dispatchSync(coreevent.BucketAwake, this, func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onAwake: %s", nameOf(ev.Owner))
		})()
		ev.Handler.(func())()
	})
}

func (p *scriptEventRegistry) doWhenTimer(time float64) {
	p.dispatchAsync(coreevent.BucketTimer, false, time, func(ev *eventSink) {
		ev.Handler.(func(float64))(time)
	})
}

func (p *scriptEventRegistry) doWhenKeyPressed(key Key) {
	p.dispatchAsync(coreevent.BucketKeyPressed, false, key, func(ev *eventSink) {
		ev.Handler.(func(Key))(key)
	})
}

func (p *scriptEventRegistry) doWhenSwipe(direction Direction, this threadObj) {
	p.dispatchAsync(coreevent.BucketSwipe, false, direction, func(ev *eventSink) {
		if ev.Owner == this {
			ev.Handler.(func(Direction))(direction)
		}
	})
}

func (p *scriptEventRegistry) doWhenClick(this threadObj) {
	p.dispatchAsync(coreevent.BucketClick, false, this, func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onClick: %s", nameOf(this))
		})()
		ev.Handler.(func())()
	})
}

func (p *scriptEventRegistry) doWhenTouchStart(this threadObj, obj *SpriteImpl) {
	p.dispatchAsync(coreevent.BucketTouchStart, false, this, func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("===> onTouchStart: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *scriptEventRegistry) doWhenTouching(this threadObj, obj *SpriteImpl) {
	p.dispatchAsync(coreevent.BucketTouching, false, this, func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onTouching: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *scriptEventRegistry) doWhenTouchEnd(this threadObj, obj *SpriteImpl) {
	p.dispatchAsync(coreevent.BucketTouchEnd, false, this, func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("===> onTouchEnd: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *scriptEventRegistry) doWhenCloned(this threadObj, data any) {
	p.dispatchAsync(coreevent.BucketCloned, true, this, func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onCloned: %s", nameOf(this))
		})()
		ev.Handler.(func(any))(data)
	})
}

func (p *scriptEventRegistry) doWhenIReceive(msg string, data any, wait bool) {
	p.dispatch(coreevent.BucketIReceive, wait, msg, func(ev *eventSink) {
		ev.Handler.(func(string, any))(msg, data)
	})
}

func (p *scriptEventRegistry) doWhenBackdropChanged(name BackdropName, wait bool) {
	p.dispatch(coreevent.BucketBackdropChanged, wait, name, func(ev *eventSink) {
		ev.Handler.(func(BackdropName))(name)
	})
}
