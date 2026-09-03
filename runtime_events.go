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
	"slices"
	"sync"
	"sync/atomic"

	"github.com/goplus/spbase/mathf"
	coreevent "github.com/goplus/spx/v3/internal/core/event"
	coreruntime "github.com/goplus/spx/v3/internal/core/runtime"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
	itime "github.com/goplus/spx/v3/internal/time"
)

const (
	clickTimerGlobal = -1
	clickTimerStage  = 0
)

// Event Bindings
type eventSink = coreevent.Sink

type scriptEventBindings struct {
	*scriptEventRegistry
	pthis threadObj
}

type scriptEventRegistry struct {
	manager              coreevent.Manager
	messageHandlerFrames sync.Map // map[coroutine.Thread]int64
	stopAllEpoch         atomic.Uint64
	pendingStartThreads  sync.Map // map[coroutine.Thread]struct{}
}

// messageEventHandler tracks one broadcast script's active thread.
type messageEventHandler struct {
	mu     sync.Mutex
	active coroutine.Thread
	run    func(string, any)
}

func (p *messageEventHandler) start(thread coroutine.Thread) func() {
	p.mu.Lock()
	previous := p.active
	p.active = thread
	p.mu.Unlock()

	if previous != nil && previous != thread {
		gco.Stop(previous)
	}
	return func() {
		p.mu.Lock()
		if p.active == thread {
			p.active = nil
		}
		p.mu.Unlock()
	}
}

type startEventDispatcher struct{}

func (p *scriptEventBindings) init(registry *scriptEventRegistry, this threadObj) {
	p.scriptEventRegistry = registry
	p.pthis = this
}

func (p *scriptEventBindings) initFrom(src *scriptEventBindings, this threadObj) {
	p.scriptEventRegistry = src.scriptEventRegistry
	p.pthis = this
}

func (p *scriptEventBindings) doDeleteClone() {
	p.scriptEventRegistry.manager.DeleteOwner(p.pthis)
}

func (p *scriptEventBindings) doWhenSwipe(direction Direction, target threadObj) {
	p.scriptEventRegistry.doWhenSwipe(direction, target)
}

func (p *scriptEventBindings) onAwake(onAwake func()) {
	pthis := p.pthis
	p.scriptEventRegistry.manager.AddAwake(coreevent.NewSink(p.pthis, onAwake, coreevent.MatchOwnerOrNil(pthis)))
}

func (p *scriptEventBindings) OnStart(onStart func()) {
	sink := coreevent.NewSink(p.pthis, onStart)
	if p.scriptEventRegistry.manager.TryAddStart(sink) {
		return
	}
	if sprite, ok := sink.Owner.(*SpriteImpl); ok && sprite.spriteState.Cloned {
		return
	}
	spxlog.Warn("Event: ignoring late OnStart registration for %s", nameOf(sink.Owner))
}

func (p *scriptEventBindings) OnClick(onClick func()) {
	pthis := p.pthis
	p.scriptEventRegistry.manager.AddClick(coreevent.NewSink(pthis, onClick, coreevent.MatchOwner(pthis)))
}

// OnCond runs the handler on each false-to-true transition.
// condition is polled after startup and must be fast and non-blocking.
func (p *scriptEventBindings) OnCond(__xgo_autoclosure_condition func() bool, onCondition func()) {
	if __xgo_autoclosure_condition == nil || onCondition == nil {
		return
	}
	p.scriptEventRegistry.manager.AddCondition(coreevent.NewSink(
		p.pthis,
		onCondition,
		coreevent.MatchRisingEdge(__xgo_autoclosure_condition),
	))
}

func (p *scriptEventBindings) registerKeyHandler(keys []Key, handler func(Key)) {
	if len(keys) == 0 {
		return
	}
	keys = slices.Clone(keys)
	sink := coreevent.NewSink(p.pthis, handler)
	if slices.Contains(keys, KeyAny) {
		p.scriptEventRegistry.manager.AddAnyKeyPressed(sink)
		return
	}
	sink.Cond = coreevent.MatchAnyOf(keys)
	p.scriptEventRegistry.manager.AddKeyPressed(sink)
}

func (p *scriptEventBindings) OnAnyKey(onKey func(key Key)) {
	p.registerKeyHandler([]Key{KeyAny}, onKey)
}

func (p *scriptEventBindings) OnTimer(time float64, call func()) {
	itime.RegisterTimer(time)
	p.scriptEventRegistry.manager.AddTimer(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(call, coreevent.If1(isDebugEventEnabled, func(float64) {
			spxlog.Debug("OnTimer: %s", nameOf(p.pthis))
		})),
		coreevent.MatchApproxFloat(time, 0.001),
	))
}

func (p *scriptEventBindings) OnKey__0(key Key, onKey func()) {
	handler := coreevent.TapVoid1(onKey, coreevent.If1(isDebugEventEnabled, func(Key) {
		spxlog.Debug("OnKey: %v, %s", key, nameOf(p.pthis))
	}))
	p.registerKeyHandler([]Key{key}, handler)
}

func (p *scriptEventBindings) OnSwipe__0(direction Direction, onSwipe func()) {
	p.scriptEventRegistry.manager.AddSwipe(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onSwipe, coreevent.If1(isDebugEventEnabled, func(Direction) {
			spxlog.Debug("OnSwipe: %v, %s", direction, nameOf(p.pthis))
		})),
		coreevent.MatchValue(direction),
	))
}

func (p *scriptEventBindings) OnKey__1(keys []Key, onKey func(Key)) {
	handler := coreevent.Tap1(onKey, coreevent.If1(isDebugEventEnabled, func(key Key) {
		spxlog.Debug("OnKey: %v, %s", keys, nameOf(p.pthis))
	}))
	p.registerKeyHandler(keys, handler)
}

func (p *scriptEventBindings) OnKey__2(keys []Key, onKey func()) {
	p.OnKey__1(keys, coreevent.Ignore1[Key](onKey))
}

func (p *scriptEventBindings) registerMessageHandler(handler func(string, any), cond ...func(any) bool) {
	p.scriptEventRegistry.manager.AddIReceive(coreevent.NewSink(
		p.pthis,
		&messageEventHandler{run: handler},
		cond...,
	))
}

func (p *scriptEventBindings) OnMsg__0(onMsg func(msg MsgName, data any)) {
	p.registerMessageHandler(onMsg)
}

func (p *scriptEventBindings) OnMsg__1(msg MsgName, onMsg func()) {
	p.registerMessageHandler(
		coreevent.TapVoid2(onMsg, coreevent.If2(isDebugEventEnabled, func(msg string, data any) {
			spxlog.Debug("OnMsg: %s, %s", msg, nameOf(p.pthis))
		})),
		coreevent.MatchValue(msg),
	)
}

func (p *scriptEventBindings) OnBackdrop__0(onBackdrop func(name BackdropName)) {
	p.scriptEventRegistry.manager.AddBackdropChanged(coreevent.NewSink(p.pthis, onBackdrop))
}

func (p *scriptEventBindings) OnBackdrop__1(name BackdropName, onBackdrop func()) {
	p.scriptEventRegistry.manager.AddBackdropChanged(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onBackdrop, coreevent.If1(isDebugEventEnabled, func(name BackdropName) {
			spxlog.Debug("OnBackdrop: %s, %s", name, nameOf(p.pthis))
		})),
		coreevent.MatchValue(name),
	))
}

func (p *scriptEventBindings) Stop(kind StopKind) {
	if kind == AllStop {
		p.scriptEventRegistry.stopAllEpoch.Add(1)
		if game := activeGame(); game != nil {
			game.resetGraphicEffectsOnStopAll()
		}
	}

	var current coroutine.Thread
	if gco.IsInCoroutine() {
		current = gco.Current()
	}
	filter, abort := coreevent.ResolveStop(
		kind,
		p.pthis,
		func(obj any) bool { return isSprite(obj) },
		func(obj any) bool { return isGame(obj) },
	)
	if filter != nil {
		gco.StopIf(func(th coroutine.Thread) bool {
			if !filter(th.Obj, th == current) {
				return false
			}
			return kind != AllStop || !p.scriptEventRegistry.isPendingStartThread(th)
		})
	}
	if abort {
		gco.Abort()
	}
}

// Scratch clears graphic effects for the stage and every sprite when a
// project-wide stop is triggered, including the `stop all` control block.
func (p *Game) resetGraphicEffectsOnStopAll() {
	p.baseObj.clearGraphicEffects()

	shapes := append([]Shape(nil), p.getAllShapes()...)
	for _, item := range shapes {
		sprite, ok := item.(*SpriteImpl)
		if !ok {
			continue
		}
		sprite.clearGraphicEffects()
	}
}

// Click Dispatch
type clicker interface {
	threadObj
	doWhenClick(this threadObj)
	getProxy() *engine.Sprite
	Visible() bool
}

func (p *Game) pointHitsClickTarget(target clicker, point mathf.Vec2) bool {
	syncSprite := target.getProxy()
	if syncSprite == nil || !target.Visible() {
		return false
	}

	sprite, ok := target.(*SpriteImpl)
	if ok {
		sprite.ensureProxyQueryStateSynced()
		if sprite.isFullyGhosted() {
			return false
		}
	}

	return p.engine().SpriteMgr.CheckCollisionWithPoint(syncSprite.GetId(), point, true)
}

func (p *Game) findClickTarget(point mathf.Vec2) (coreruntime.ClickSelection[clicker, *SpriteImpl], bool) {
	return coreruntime.FindClickTarget(p.getTempShapes(), func(item Shape) (coreruntime.ClickSelection[clicker, *SpriteImpl], bool) {
		o, ok := item.(clicker)
		if !ok {
			return coreruntime.ClickSelection[clicker, *SpriteImpl]{}, false
		}
		if !p.pointHitsClickTarget(o, point) {
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

// Message Broadcast
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

// Event Routing
func (p *Game) handleEvent(ev event) {
	switch e := ev.(type) {
	case *eventLeftButtonUp:
		p.doWhenLeftButtonUp(e)
	case *eventLeftButtonDown:
		p.doWhenLeftButtonDown(e)
	case *eventMouseMove:
		p.inputMgr.onMouseMove(e.Pos)
	case *eventKeyDown:
		// Note: key-up callbacks are not part of the current event sink API.
		p.scriptEvents.doWhenKeyPressed(e.Key)
	case *eventStart:
		runStartPhase := func() {
			sinks, ok := p.takeStartSinksFor(e.generation)
			if !ok {
				return
			}
			p.scriptEvents.doWhenStart(sinks, func() bool {
				return p.isBootstrapGenerationCurrent(e.generation)
			})
			p.markStartDispatchedFor(e.generation)
		}
		if gco == nil {
			runStartPhase()
			break
		}
		dispatcher := gco.Create(startEventDispatcher{}, func(coroutine.Thread) int {
			runStartPhase()
			return 0
		})
		if gco.IsInCoroutine() {
			gco.Join(dispatcher)
		}
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

// Event Dispatch
func (p *scriptEventRegistry) doWhenStart(sinks []eventSink, shouldRun func() bool) {
	p.dispatchStartSinks(sinksInScratchTargetOrder(activeGame(), sinks), scriptEventDispatch{
		mode:      coroutine.BatchWaitFirstSlice,
		shouldRun: shouldRun,
		run: func(_ coroutine.Thread, ev *eventSink) {
			coreevent.If0(isDebugEventEnabled, func() {
				spxlog.Debug("OnStart: %s", nameOf(ev.Owner))
			})()
			ev.Handler.(func())()
		},
	})
}

func (p *scriptEventRegistry) doWhenAwake(this threadObj) {
	p.dispatchGlobal(coreevent.BucketAwake, scriptEventDispatch{
		mode:      coroutine.BatchWaitDone,
		matchData: this,
		run: func(_ coroutine.Thread, ev *eventSink) {
			coreevent.If0(isDebugEventEnabled, func() {
				spxlog.Debug("OnAwake: %s", nameOf(ev.Owner))
			})()
			ev.Handler.(func())()
		},
	})
}

func (p *scriptEventRegistry) doWhenTimer(time float64) {
	p.dispatchGlobal(coreevent.BucketTimer, scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: time,
		run: func(_ coroutine.Thread, ev *eventSink) {
			ev.Handler.(func(float64))(time)
		},
	})
}

func (p *scriptEventRegistry) doWhenCondition() {
	p.dispatchGlobal(coreevent.BucketCondition, scriptEventDispatch{
		mode: coroutine.BatchAsync,
		run: func(_ coroutine.Thread, ev *eventSink) {
			coreevent.If0(isDebugEventEnabled, func() {
				spxlog.Debug("OnCond: %s", nameOf(ev.Owner))
			})()
			ev.Handler.(func())()
		},
	})
}

func (p *scriptEventRegistry) doWhenKeyPressed(key Key) {
	specific := p.globalSinks(coreevent.BucketKeyPressed)
	anyKey := p.globalSinks(coreevent.BucketAnyKeyPressed)
	p.dispatchSinks(slices.Concat(specific, anyKey), scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: key,
		run: func(_ coroutine.Thread, ev *eventSink) {
			ev.Handler.(func(Key))(key)
		},
	})
}

func (p *scriptEventRegistry) doWhenSwipe(direction Direction, this threadObj) {
	p.dispatchTarget(coreevent.BucketSwipe, this, scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: direction,
		run: func(_ coroutine.Thread, ev *eventSink) {
			ev.Handler.(func(Direction))(direction)
		},
	})
}

func (p *scriptEventRegistry) doWhenClick(this threadObj) {
	p.dispatchTarget(coreevent.BucketClick, this, scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: this,
		run: func(_ coroutine.Thread, ev *eventSink) {
			coreevent.If0(isDebugEventEnabled, func() {
				spxlog.Debug("OnClick: %s", nameOf(this))
			})()
			ev.Handler.(func())()
		},
	})
}

func (p *scriptEventRegistry) doWhenTouchStart(this threadObj, obj *SpriteImpl) {
	p.dispatchTarget(coreevent.BucketTouchStart, this, scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: this,
		run: func(_ coroutine.Thread, ev *eventSink) {
			coreevent.If0(isDebugEventEnabled, func() {
				spxlog.Debug("OnTouchStart: %s, %s", nameOf(this), obj.name)
			})()
			ev.Handler.(func(Sprite))(obj.sprite)
		},
	})
}

func (p *scriptEventRegistry) doWhenCloned(this threadObj, data any) {
	p.dispatchTarget(coreevent.BucketCloned, this, scriptEventDispatch{
		mode:      coroutine.BatchWaitFirstSlice,
		matchData: this,
		run: func(_ coroutine.Thread, ev *eventSink) {
			coreevent.If0(isDebugEventEnabled, func() {
				spxlog.Debug("OnCloned: %s", nameOf(this))
			})()
			ev.Handler.(func(any))(data)
		},
	})
}

func (p *scriptEventRegistry) doWhenIReceive(msg string, data any, wait bool) {
	deferToNextFrame := p.shouldDeferMessageReceivers(wait)
	p.dispatchGlobal(coreevent.BucketIReceive, scriptEventDispatch{
		mode:      eventBatchMode(wait),
		matchData: msg,
		lifecycle: func(thread coroutine.Thread, ev *eventSink) func() {
			return ev.Handler.(*messageEventHandler).start(thread)
		},
		before: func(coroutine.Thread) {
			if deferToNextFrame {
				engine.WaitNextFrame()
			}
		},
		run: func(thread coroutine.Thread, ev *eventSink) {
			p.messageHandlerFrames.Store(thread, itime.Frame())
			defer p.messageHandlerFrames.Delete(thread)
			ev.Handler.(*messageEventHandler).run(msg, data)
		},
	})
}

// Defer only broadcasts made in the frame where their handler began. A
// handler that already yielded has established its own frame boundary.
func (p *scriptEventRegistry) shouldDeferMessageReceivers(wait bool) bool {
	if wait || gco == nil || !gco.IsInCoroutine() {
		return false
	}
	thread := gco.Current()
	if thread == nil {
		return false
	}
	handlerFrame, ok := p.messageHandlerFrames.Load(thread)
	return ok && handlerFrame == itime.Frame()
}

func (p *scriptEventRegistry) doWhenBackdropChanged(name BackdropName, wait bool) {
	p.dispatchGlobal(coreevent.BucketBackdropChanged, scriptEventDispatch{
		mode:      eventBatchMode(wait),
		matchData: name,
		run: func(_ coroutine.Thread, ev *eventSink) {
			ev.Handler.(func(BackdropName))(name)
		},
	})
}
