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
	owner any
}

type scriptEventRegistry struct {
	game    *Game
	manager coreevent.Manager
	// Accessed only in managed script slices, under the scheduler's runMu.
	messageExecutions   map[coroutine.Thread]messageReceiverExecution
	stopAllEpoch        atomic.Uint64
	pendingStartThreads sync.Map    // map[coroutine.Thread]struct{}
	pendingConditions   []eventSink // engine frame thread only
}

// messageDispatchContext tracks receivers visited by one broadcast tree.
// Its state is accessed only by managed scripts holding the scheduler's runMu.
type messageDispatchContext struct {
	frame     int64
	round     uint64
	receivers map[*scriptEventHandler]struct{}
}

type messageReceiverExecution struct {
	context  *messageDispatchContext
	receiver *scriptEventHandler
}

type startEventDispatcher struct{}

// Click Dispatch
type clicker interface {
	doWhenClick(this any)
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
	p.register(coreevent.BucketClick, p.owner, onClick)
}

func (p *scriptEventBindings) OnAnyKey(onKey func(key Key)) {
	p.registerKeyHandler([]Key{KeyAny}, onKey)
}

func (p *scriptEventBindings) OnTimer(time float64, call func()) {
	timestamp := itime.RegisterTimer(time)
	p.register(coreevent.BucketTimer, p.owner,
		func() {
			if isDebugEventEnabled() {
				spxlog.Debug("OnTimer: %s", nameOf(p.owner))
			}
			if call != nil {
				call()
			}
		},
		coreevent.MatchValue(timestamp),
	)
}

func (p *scriptEventBindings) OnKey__0(key Key, onKey func()) {
	handler := func(Key) {
		if isDebugEventEnabled() {
			spxlog.Debug("OnKey: %v, %s", key, nameOf(p.owner))
		}
		if onKey != nil {
			onKey()
		}
	}
	p.registerKeyHandler([]Key{key}, handler)
}

func (p *scriptEventBindings) OnSwipe__0(direction Direction, onSwipe func()) {
	p.register(coreevent.BucketSwipe, p.owner,
		func(Direction) {
			if isDebugEventEnabled() {
				spxlog.Debug("OnSwipe: %v, %s", direction, nameOf(p.owner))
			}
			if onSwipe != nil {
				onSwipe()
			}
		},
		coreevent.MatchValue(direction),
	)
}

func (p *scriptEventBindings) OnKey__1(keys []Key, onKey func(Key)) {
	handler := func(key Key) {
		if isDebugEventEnabled() {
			spxlog.Debug("OnKey: %v, %s", keys, nameOf(p.owner))
		}
		if onKey != nil {
			onKey(key)
		}
	}
	p.registerKeyHandler(keys, handler)
}

func (p *scriptEventBindings) OnKey__2(keys []Key, onKey func()) {
	p.OnKey__1(keys, coreevent.Ignore1[Key](onKey))
}

func (p *scriptEventBindings) OnMsg__0(onMsg func(msg MsgName, data any)) {
	p.register(coreevent.BucketIReceive, p.owner, onMsg)
}

func (p *scriptEventBindings) OnMsg__1(msg MsgName, onMsg func()) {
	p.register(coreevent.BucketIReceive, p.owner,
		func(msg string, _ any) {
			if isDebugEventEnabled() {
				spxlog.Debug("OnMsg: %s, %s", msg, nameOf(p.owner))
			}
			if onMsg != nil {
				onMsg()
			}
		},
		coreevent.MatchValue(msg),
	)
}

func (p *scriptEventBindings) OnBackdrop__0(onBackdrop func(name BackdropName)) {
	p.register(coreevent.BucketBackdropChanged, p.owner, onBackdrop)
}

func (p *scriptEventBindings) OnBackdrop__1(name BackdropName, onBackdrop func()) {
	handler := func(name BackdropName) {
		if isDebugEventEnabled() {
			spxlog.Debug("OnBackdrop: %s, %s", name, nameOf(p.owner))
		}
		if onBackdrop != nil {
			onBackdrop()
		}
	}
	p.register(coreevent.BucketBackdropChanged, p.owner, handler, coreevent.MatchValue(name))
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

func (p *scriptEventBindings) bind(registry *scriptEventRegistry, owner any) {
	p.scriptEventRegistry = registry
	p.owner = owner
}

func (p *scriptEventBindings) clearHandlers() {
	p.scriptEventRegistry.manager.DeleteOwner(p.owner)
}

func (p *scriptEventBindings) registerKeyHandler(keys []Key, handler func(Key)) {
	if len(keys) == 0 {
		return
	}
	if slices.Contains(keys, KeyAny) {
		p.register(coreevent.BucketAnyKeyPressed, p.owner, handler)
		return
	}
	p.register(coreevent.BucketKeyPressed, p.owner, handler, coreevent.MatchAnyOf(slices.Clone(keys)))
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

func (p *Game) findClickTarget(point mathf.Vec2) clicker {
	for _, item := range slices.Backward(p.shapeMgr.getTempShapes()) {
		if target, ok := item.(clicker); ok && p.pointHitsClickTarget(target, point) {
			return target
		}
	}
	return nil
}

func (p *Game) doBroadcast(msg MsgName, data any, wait bool) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Broadcast: msg=%s, wait=%v", msg, wait)
	}
	p.scriptEvents.doWhenIReceive(msg, data, wait)
}

func (p *Game) handleEvent(ev event) {
	switch e := ev.(type) {
	case *eventLeftButtonUp:
		p.inputMgr.onLeftButtonUp(e.Pos)
	case *eventLeftButtonDown:
		p.inputMgr.onLeftButtonDown(e.Pos)
	case *eventMouseMove:
		p.inputMgr.onMouseMove(e.Pos)
	case *eventKeyDown:
		p.inputMgr.onKeyDown(e.Key)
	case *eventStart:
		p.scriptEvents.doWhenStart(e.generation)
	case *eventTimer:
		p.scriptEvents.doWhenTimer(e.Timestamp)
	}
}

// Event Dispatch
func (p *scriptEventRegistry) doWhenStart(generation uint64) {
	runStartPhase := func(coroutine.Thread) {
		sinks, ok := p.game.takeStartSinks(generation)
		if !ok {
			return
		}
		p.dispatchStartEventBatch(sinksInTargetOrder(p.game, sinks), scriptEventDispatch{
			mode: coroutine.BatchWaitFirstSlice,
			run: func(_ coroutine.Thread, ev *eventSink) {
				if !p.game.isCurrentBootstrap(generation) {
					return
				}
				if isDebugEventEnabled() {
					spxlog.Debug("OnStart: %s", nameOf(ev.Owner))
				}
				ev.Handler.(func())()
			},
		})
		p.game.markStartDispatched(generation)
	}
	if gco == nil {
		runStartPhase(nil)
		return
	}
	dispatcher := gco.Create(startEventDispatcher{}, runStartPhase)
	if gco.IsInCoroutine() {
		gco.Join(dispatcher)
	}
}

func (p *scriptEventRegistry) doWhenTimer(timestamp int64) {
	p.dispatchGlobal(coreevent.BucketTimer, scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: timestamp,
		run: func(_ coroutine.Thread, ev *eventSink) {
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
			handler := ev.Handler.(*scriptEventHandler)
			handler.run.(func(Key))(key)
		},
	})
}

func (p *scriptEventRegistry) doWhenSwipe(direction Direction, this any) {
	p.dispatchTarget(coreevent.BucketSwipe, this, scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: direction,
		run: func(_ coroutine.Thread, ev *eventSink) {
			ev.Handler.(func(Direction))(direction)
		},
	})
}

func (p *scriptEventRegistry) doWhenClick(this any) {
	p.dispatchTarget(coreevent.BucketClick, this, scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: this,
		run: func(_ coroutine.Thread, ev *eventSink) {
			if isDebugEventEnabled() {
				spxlog.Debug("OnClick: %s", nameOf(this))
			}
			handler := ev.Handler.(*scriptEventHandler)
			handler.run.(func())()
		},
	})
}

func (p *scriptEventRegistry) doWhenTouchStart(this any, obj *SpriteImpl) {
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

func (p *scriptEventRegistry) doWhenCloned(this any, data any) {
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
			receiver := ev.Handler.(*scriptEventHandler)
			if thread != nil {
				context.waitForTurn(thread, receiver)
				if p.messageExecutions == nil {
					p.messageExecutions = make(map[coroutine.Thread]messageReceiverExecution)
				}
				p.messageExecutions[thread] = messageReceiverExecution{
					context:  context,
					receiver: receiver,
				}
				// Yield reacquires runMu before cancellation unwinds this defer;
				// runThread releases it only after Run and its defers finish.
				defer delete(p.messageExecutions, thread)
			}
			receiver.run.(func(string, any))(msg, data)
		},
	})
}

func (p *scriptEventRegistry) currentMessageDispatchContext() *messageDispatchContext {
	if gco == nil || !gco.IsInCoroutine() {
		return new(messageDispatchContext)
	}
	execution, ok := p.messageExecutions[gco.Current()]
	if !ok {
		return new(messageDispatchContext)
	}
	execution.context.claimTurn(execution.receiver)
	return execution.context
}

func (p *messageDispatchContext) waitForTurn(thread coroutine.Thread, receiver *scriptEventHandler) {
	for !p.claimTurn(receiver) {
		gco.YieldToNextRoundFor(thread)
	}
}

func (p *messageDispatchContext) claimTurn(receiver *scriptEventHandler) bool {
	frame, round := itime.Frame(), gco.ScriptRound()
	if p.frame != frame || p.round != round {
		p.frame, p.round = frame, round
		clear(p.receivers)
	}
	if p.receivers == nil {
		p.receivers = make(map[*scriptEventHandler]struct{})
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
			handler := ev.Handler.(*scriptEventHandler)
			handler.run.(func(BackdropName))(name)
		},
	})
}
