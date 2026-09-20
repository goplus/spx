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
	owner threadObj
}

type scriptEventRegistry struct {
	game                *Game
	manager             coreevent.Manager
	messageExecutions   sync.Map // map[coroutine.Thread]*messageReceiverExecution
	stopAllEpoch        atomic.Uint64
	pendingStartThreads sync.Map    // map[coroutine.Thread]struct{}
	pendingConditions   []eventSink // engine frame thread only
}

// messageDispatchContext tracks receivers visited by one broadcast tree.
type messageDispatchContext struct {
	mu        sync.Mutex
	frame     int64
	round     uint64
	receivers map[*messageEventHandler]struct{}
}

type messageReceiverExecution struct {
	context  *messageDispatchContext
	receiver *messageEventHandler
}

type startEventDispatcher struct{}

// Click Dispatch
type clicker interface {
	threadObj
	doWhenClick(this threadObj)
	getProxy() *engine.Sprite
	Visible() bool
}

func (p *scriptEventBindings) OnStart(onStart func()) {
	sink := coreevent.NewSink(p.owner, onStart)
	if p.scriptEventRegistry.manager.TryAddStart(sink) {
		return
	}
	if sprite, ok := sink.Owner.(*SpriteImpl); ok && sprite.spriteState.Cloned {
		return
	}
	spxlog.Warn("Event: ignoring late OnStart registration for %s", nameOf(sink.Owner))
}

func (p *scriptEventBindings) OnClick(onClick func()) {
	owner := p.owner
	p.scriptEventRegistry.manager.AddClick(newScriptEventSink(owner, onClick, coroutine.RestartExisting, coreevent.MatchOwner(owner)))
}

func (p *scriptEventBindings) OnAnyKey(onKey func(key Key)) {
	p.registerKeyHandler([]Key{KeyAny}, onKey)
}

func (p *scriptEventBindings) OnTimer(time float64, call func()) {
	itime.RegisterTimer(time)
	p.scriptEventRegistry.manager.AddTimer(coreevent.NewSink(
		p.owner,
		coreevent.TapVoid1(call, coreevent.If1(isDebugEventEnabled, func(float64) {
			spxlog.Debug("OnTimer: %s", nameOf(p.owner))
		})),
		coreevent.MatchApproxFloat(time, 0.001),
	))
}

func (p *scriptEventBindings) OnKey__0(key Key, onKey func()) {
	handler := coreevent.TapVoid1(onKey, coreevent.If1(isDebugEventEnabled, func(Key) {
		spxlog.Debug("OnKey: %v, %s", key, nameOf(p.owner))
	}))
	p.registerKeyHandler([]Key{key}, handler)
}

func (p *scriptEventBindings) OnSwipe__0(direction Direction, onSwipe func()) {
	p.scriptEventRegistry.manager.AddSwipe(coreevent.NewSink(
		p.owner,
		coreevent.TapVoid1(onSwipe, coreevent.If1(isDebugEventEnabled, func(Direction) {
			spxlog.Debug("OnSwipe: %v, %s", direction, nameOf(p.owner))
		})),
		coreevent.MatchValue(direction),
	))
}

func (p *scriptEventBindings) OnKey__1(keys []Key, onKey func(Key)) {
	handler := coreevent.Tap1(onKey, coreevent.If1(isDebugEventEnabled, func(key Key) {
		spxlog.Debug("OnKey: %v, %s", keys, nameOf(p.owner))
	}))
	p.registerKeyHandler(keys, handler)
}

func (p *scriptEventBindings) OnKey__2(keys []Key, onKey func()) {
	p.OnKey__1(keys, coreevent.Ignore1[Key](onKey))
}

func (p *scriptEventBindings) OnMsg__0(onMsg func(msg MsgName, data any)) {
	p.registerMessageHandler(onMsg)
}

func (p *scriptEventBindings) OnMsg__1(msg MsgName, onMsg func()) {
	p.registerMessageHandler(
		coreevent.TapVoid2(onMsg, coreevent.If2(isDebugEventEnabled, func(msg string, data any) {
			spxlog.Debug("OnMsg: %s, %s", msg, nameOf(p.owner))
		})),
		coreevent.MatchValue(msg),
	)
}

func (p *scriptEventBindings) OnBackdrop__0(onBackdrop func(name BackdropName)) {
	p.scriptEventRegistry.manager.AddBackdropChanged(newScriptEventSink(p.owner, onBackdrop, coroutine.RestartExisting))
}

func (p *scriptEventBindings) OnBackdrop__1(name BackdropName, onBackdrop func()) {
	handler := coreevent.TapVoid1(onBackdrop, coreevent.If1(isDebugEventEnabled, func(name BackdropName) {
		spxlog.Debug("OnBackdrop: %s, %s", name, nameOf(p.owner))
	}))
	p.scriptEventRegistry.manager.AddBackdropChanged(newScriptEventSink(
		p.owner, handler, coroutine.RestartExisting, coreevent.MatchValue(name),
	))
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

func (p *Game) bindScriptEvents() {
	p.scriptEvents.game = p
	p.scriptEventBindings.bind(&p.scriptEvents, p)
}

func (p *scriptEventBindings) bind(registry *scriptEventRegistry, owner threadObj) {
	p.scriptEventRegistry = registry
	p.owner = owner
}

func (p *scriptEventBindings) clearHandlers() {
	p.scriptEventRegistry.manager.DeleteOwner(p.owner)
}

func (p *scriptEventBindings) doWhenSwipe(direction Direction, target threadObj) {
	p.scriptEventRegistry.doWhenSwipe(direction, target)
}

func (p *scriptEventBindings) onAwake(onAwake func()) {
	owner := p.owner
	p.scriptEventRegistry.manager.AddAwake(coreevent.NewSink(owner, onAwake, coreevent.MatchOwnerOrNil(owner)))
}

func (p *scriptEventBindings) registerKeyHandler(keys []Key, handler func(Key)) {
	if len(keys) == 0 {
		return
	}
	sink := newScriptEventSink(p.owner, handler, coroutine.IgnoreWhileRunning)
	if slices.Contains(keys, KeyAny) {
		p.scriptEventRegistry.manager.AddAnyKeyPressed(sink)
		return
	}
	sink.Cond = coreevent.MatchAnyOf(slices.Clone(keys))
	p.scriptEventRegistry.manager.AddKeyPressed(sink)
}

func (p *scriptEventBindings) registerMessageHandler(handler func(string, any), cond ...func(any) bool) {
	p.scriptEventRegistry.manager.AddIReceive(newScriptEventSink(
		p.owner, handler, coroutine.RestartExisting, cond...,
	))
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

	return engine.Managers().SpriteMgr.CheckCollisionWithPoint(syncSprite.GetId(), point, true)
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
			sinks, ok := p.takeStartSinks(e.generation)
			if !ok {
				return
			}
			p.scriptEvents.doWhenStart(sinks, func() bool {
				return p.isCurrentBootstrap(e.generation)
			})
			p.markStartDispatched(e.generation)
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
		spxlog.Warn("Event dropped (policy=%s): %v", p.eventQueueState.EventQueuePolicy, ev)
	}
}

// Event Dispatch
func (p *scriptEventRegistry) doWhenStart(sinks []eventSink, shouldRun func() bool) {
	p.dispatchStartSinks(sinksInScratchTargetOrder(p.game, sinks), scriptEventDispatch{
		mode: coroutine.BatchWaitFirstSlice,
		run: func(_ coroutine.Thread, ev *eventSink) {
			if shouldRun != nil && !shouldRun() {
				return
			}
			if isDebugEventEnabled() {
				spxlog.Debug("OnStart: %s", nameOf(ev.Owner))
			}
			ev.Handler.(func())()
		},
	})
}

func (p *scriptEventRegistry) doWhenAwake(this threadObj) {
	p.dispatchGlobal(coreevent.BucketAwake, scriptEventDispatch{
		mode:      coroutine.BatchWaitDone,
		matchData: this,
		run: func(_ coroutine.Thread, ev *eventSink) {
			if isDebugEventEnabled() {
				spxlog.Debug("OnAwake: %s", nameOf(ev.Owner))
			}
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

func (p *scriptEventRegistry) doWhenKeyPressed(key Key) {
	specific := p.globalSinks(coreevent.BucketKeyPressed)
	anyKey := p.globalSinks(coreevent.BucketAnyKeyPressed)
	p.dispatchSinks(slices.Concat(specific, anyKey), scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: key,
		run: func(_ coroutine.Thread, ev *eventSink) {
			ev.Handler.(*scriptEventHandler[func(Key)]).run(key)
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
			if isDebugEventEnabled() {
				spxlog.Debug("OnClick: %s", nameOf(this))
			}
			ev.Handler.(*scriptEventHandler[func()]).run()
		},
	})
}

func (p *scriptEventRegistry) doWhenTouchStart(this threadObj, obj *SpriteImpl) {
	p.dispatchTarget(coreevent.BucketTouchStart, this, scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: this,
		run: func(_ coroutine.Thread, ev *eventSink) {
			if isDebugEventEnabled() {
				spxlog.Debug("OnTouchStart: %s, %s", nameOf(this), obj.name)
			}
			ev.Handler.(func(Sprite))(obj.sprite)
		},
	})
}

func (p *scriptEventRegistry) doWhenCloned(this threadObj, data any) {
	p.dispatchTarget(coreevent.BucketCloned, this, scriptEventDispatch{
		mode:      coroutine.BatchWaitFirstSlice,
		matchData: this,
		run: func(_ coroutine.Thread, ev *eventSink) {
			if isDebugEventEnabled() {
				spxlog.Debug("OnCloned: %s", nameOf(this))
			}
			ev.Handler.(func(any))(data)
		},
	})
}

func (p *scriptEventRegistry) doWhenIReceive(msg string, data any, wait bool) {
	context := p.currentMessageDispatchContext()
	p.dispatchGlobal(coreevent.BucketIReceive, scriptEventDispatch{
		mode:      eventBatchMode(wait),
		matchData: msg,
		run: func(thread coroutine.Thread, ev *eventSink) {
			receiver := ev.Handler.(*messageEventHandler)
			if thread != nil {
				context.waitForTurn(thread, receiver)
				p.messageExecutions.Store(thread, &messageReceiverExecution{
					context:  context,
					receiver: receiver,
				})
				defer p.messageExecutions.Delete(thread)
			}
			receiver.run(msg, data)
		},
	})
}

func (p *scriptEventRegistry) currentMessageDispatchContext() *messageDispatchContext {
	if gco == nil || !gco.IsInCoroutine() {
		return new(messageDispatchContext)
	}
	value, ok := p.messageExecutions.Load(gco.Current())
	if !ok {
		return new(messageDispatchContext)
	}
	execution := value.(*messageReceiverExecution)
	execution.context.claimTurn(execution.receiver)
	return execution.context
}

func (p *messageDispatchContext) waitForTurn(thread coroutine.Thread, receiver *messageEventHandler) {
	for !p.claimTurn(receiver) {
		gco.YieldToNextRoundFor(thread)
	}
}

func (p *messageDispatchContext) claimTurn(receiver *messageEventHandler) bool {
	frame, round := itime.Frame(), gco.ScriptRound()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.frame != frame || p.round != round {
		p.frame, p.round = frame, round
		clear(p.receivers)
	}
	if p.receivers == nil {
		p.receivers = make(map[*messageEventHandler]struct{})
	}
	if _, claimed := p.receivers[receiver]; claimed {
		return false
	}
	p.receivers[receiver] = struct{}{}
	return true
}

func (p *scriptEventRegistry) doWhenBackdropChanged(name BackdropName, wait bool) {
	p.dispatchGlobal(coreevent.BucketBackdropChanged, scriptEventDispatch{
		mode:      eventBatchMode(wait),
		matchData: name,
		run: func(_ coroutine.Thread, ev *eventSink) {
			ev.Handler.(*scriptEventHandler[func(BackdropName)]).run(name)
		},
	})
}
