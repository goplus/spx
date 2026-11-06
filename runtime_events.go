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
	coreevent "github.com/goplus/spx/v4/internal/core/event"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	spxlog "github.com/goplus/spx/v4/internal/log"
	itime "github.com/goplus/spx/v4/internal/time"
)

// Event Types
type event any

type eventSink = coreevent.Sink

type eventStart struct {
	generation uint64
}

type eventKeyDown struct {
	Key Key
}

type eventLeftButtonDown struct {
	Pos mathf.Vec2
}

type eventLeftButtonUp struct {
	Pos mathf.Vec2
}

type eventTimer struct {
	Timestamp int64
}

// scriptEventLifecycle registers an invocation before it can run and returns
// its cleanup. Handlers without this interface allow overlapping invocations.
type scriptEventLifecycle interface {
	RegisterInvocation(coroutine.Thread) func()
}

// Each registration owns its execution state; clones re-register their handlers.
type scriptEventHandler struct {
	coroutine.HandlerState
	run any
}

type scriptEventDispatch struct {
	mode      coroutine.BatchMode
	matchData any
	run       func(coroutine.Thread, *eventSink)
}

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

// Event Bindings
func (p *scriptEventBindings) OnStart(onStart func()) {
	sink := coreevent.NewSink(p.owner, onStart)
	if p.scriptEventRegistry.manager.TryAddStart(sink) {
		return
	}
	if sprite, ok := sink.Owner.(*SpriteImpl); ok && sprite.spriteState.cloned {
		return
	}
	spxlog.Warn("Event: ignoring late OnStart registration for %s", nameOf(sink.Owner))
}

func (p *scriptEventBindings) OnClick(onClick func()) {
	p.register(coreevent.BucketClick, onClick)
}

func (p *scriptEventBindings) OnAnyKey(onKey func(key Key)) {
	p.register(coreevent.BucketAnyKeyPressed, onKey)
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

func (p *scriptEventBindings) OnTimer(time float64, call func()) {
	timestamp := itime.RegisterTimer(time)
	p.register(coreevent.BucketTimer,
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

func (p *scriptEventBindings) OnSwipe__0(direction Direction, onSwipe func()) {
	p.register(coreevent.BucketSwipe,
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

func (p *scriptEventBindings) OnMsg__0(onMsg func(msg MsgName, data any)) {
	p.register(coreevent.BucketIReceive, onMsg)
}

func (p *scriptEventBindings) OnMsg__1(msg MsgName, onMsg func()) {
	p.register(coreevent.BucketIReceive,
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
	p.register(coreevent.BucketBackdropChanged, onBackdrop)
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
	p.register(coreevent.BucketBackdropChanged, handler, coreevent.MatchValue(name))
}

// OnCond handles rising edges without reentry, skipping polls while active.
// Conditions must be fast and non-blocking.
func (p *scriptEventBindings) OnCond(__xgo_autoclosure_condition func() bool, onCondition func()) {
	if __xgo_autoclosure_condition == nil || onCondition == nil {
		return
	}
	running := false
	edge := coreevent.MatchRisingEdge(__xgo_autoclosure_condition)
	p.register(coreevent.BucketCondition,
		func() {
			running = true
			defer func() { running = false }()
			onCondition()
		},
		func(data any) bool { return !running && edge(data) },
	)
}

func (p *scriptEventBindings) bind(registry *scriptEventRegistry, owner any) {
	p.scriptEventRegistry = registry
	p.owner = owner
}

func (p *scriptEventBindings) clearHandlers() {
	p.scriptEventRegistry.manager.DeleteOwner(p.owner)
}

// Retrigger behavior belongs to the event kind; execution state belongs to
// each registration. Events without a policy keep their plain callbacks.
func (p *scriptEventBindings) register(bucket coreevent.Bucket, run any, cond ...func(any) bool) {
	handler := run
	switch bucket {
	case coreevent.BucketClick, coreevent.BucketBackdropChanged, coreevent.BucketIReceive:
		handler = &scriptEventHandler{
			HandlerState: coroutine.NewHandlerState(coroutine.RestartExisting),
			run:          run,
		}
	case coreevent.BucketKeyPressed, coreevent.BucketAnyKeyPressed:
		handler = &scriptEventHandler{
			HandlerState: coroutine.NewHandlerState(coroutine.IgnoreWhileRunning),
			run:          run,
		}
	}
	p.manager.Add(bucket, coreevent.NewSink(p.owner, handler, cond...))
}

func (p *scriptEventBindings) registerKeyHandler(keys []Key, handler func(Key)) {
	if len(keys) == 0 {
		return
	}
	if slices.Contains(keys, KeyAny) {
		p.register(coreevent.BucketAnyKeyPressed, handler)
		return
	}
	p.register(coreevent.BucketKeyPressed, handler, coreevent.MatchAnyOf(slices.Clone(keys)))
}

// Event Entry Points
func (p *scriptEventBindings) doWhenStart(generation uint64) {
	registry := p.scriptEventRegistry
	runStartPhase := func(coroutine.Thread) {
		sinks, ok := registry.game.takeStartSinks(generation)
		if !ok {
			return
		}
		registry.dispatchStartEventBatch(sinksInTargetOrder(registry.game, sinks), scriptEventDispatch{
			mode: coroutine.BatchWaitFirstSlice,
			run: func(_ coroutine.Thread, ev *eventSink) {
				if !registry.game.isCurrentBootstrap(generation) {
					return
				}
				if isDebugEventEnabled() {
					spxlog.Debug("OnStart: %s", nameOf(ev.Owner))
				}
				ev.Handler.(func())()
			},
		})
		registry.game.markStartDispatched(generation)
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

func (p *scriptEventBindings) doWhenClick() {
	owner := p.owner
	p.dispatchTarget(coreevent.BucketClick, owner, scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: owner,
		run: func(_ coroutine.Thread, ev *eventSink) {
			if isDebugEventEnabled() {
				spxlog.Debug("OnClick: %s", nameOf(owner))
			}
			handler := ev.Handler.(*scriptEventHandler)
			handler.run.(func())()
		},
	})
}

func (p *scriptEventBindings) doWhenTimer(timestamp int64) {
	p.dispatchGlobal(coreevent.BucketTimer, scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: timestamp,
		run: func(_ coroutine.Thread, ev *eventSink) {
			ev.Handler.(func())()
		},
	})
}

func (p *scriptEventBindings) doWhenKeyPressed(key Key) {
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

func (p *scriptEventBindings) doWhenSwipe(direction Direction) {
	p.dispatchTarget(coreevent.BucketSwipe, p.owner, scriptEventDispatch{
		mode:      coroutine.BatchAsync,
		matchData: direction,
		run: func(_ coroutine.Thread, ev *eventSink) {
			ev.Handler.(func(Direction))(direction)
		},
	})
}

func (p *scriptEventBindings) doWhenIReceive(msg string, data any, wait bool) {
	registry := p.scriptEventRegistry
	context := registry.currentMessageDispatchContext()
	registry.dispatchGlobal(coreevent.BucketIReceive, scriptEventDispatch{
		mode:      eventBatchMode(wait),
		matchData: msg,
		run: func(thread coroutine.Thread, ev *eventSink) {
			receiver := ev.Handler.(*scriptEventHandler)
			if thread != nil {
				context.waitForTurn(thread, receiver)
				if registry.messageExecutions == nil {
					registry.messageExecutions = make(map[coroutine.Thread]messageReceiverExecution)
				}
				registry.messageExecutions[thread] = messageReceiverExecution{
					context:  context,
					receiver: receiver,
				}
				// Yield reacquires runMu before cancellation unwinds this defer;
				// runThread releases it only after Run and its defers finish.
				defer delete(registry.messageExecutions, thread)
			}
			receiver.run.(func(string, any))(msg, data)
		},
	})
}

func (p *scriptEventBindings) doWhenBackdropChanged(name BackdropName, wait bool) {
	p.dispatchGlobal(coreevent.BucketBackdropChanged, scriptEventDispatch{
		mode:      eventBatchMode(wait),
		matchData: name,
		run: func(_ coroutine.Thread, ev *eventSink) {
			handler := ev.Handler.(*scriptEventHandler)
			handler.run.(func(BackdropName))(name)
		},
	})
}

// Event Dispatch
func (p *scriptEventRegistry) dispatchGlobal(bucket coreevent.Bucket, event scriptEventDispatch) {
	sinks := p.manager.Snapshot(bucket)
	withEventRegistrationBarrier(p.game, func() {
		// Registrations are immutable, but live target order changes with scripts.
		ordered := sinksInTargetOrder(p.game, sinks)
		matched := matchingEventSinks(ordered, event.matchData)
		dispatchMatchedScriptEventBatch(matched, event)
	})
}

func (p *scriptEventRegistry) dispatchTarget(bucket coreevent.Bucket, owner any, event scriptEventDispatch) {
	sinks := p.manager.Snapshot(bucket)
	// Resolve ownership before entering the registration barrier.
	var ownerIndexes []int
	for i, sink := range sinks {
		if sink.Owner == owner {
			if ownerIndexes == nil {
				ownerIndexes = make([]int, 0, min(len(sinks), 8))
			}
			ownerIndexes = append(ownerIndexes, i)
		}
	}
	if len(ownerIndexes) == 0 {
		return
	}
	withEventRegistrationBarrier(p.game, func() {
		// Complete matching before starting user handlers.
		var matched []eventSink
		for _, i := range ownerIndexes {
			sink := sinks[i]
			if sink.Cond == nil || sink.Cond(event.matchData) {
				if matched == nil {
					matched = make([]eventSink, 0, len(ownerIndexes))
				}
				matched = append(matched, sink)
			}
		}
		dispatchMatchedScriptEventBatch(matched, event)
	})
}

func (p *scriptEventRegistry) dispatchSinks(sinks []eventSink, event scriptEventDispatch) {
	withEventRegistrationBarrier(p.game, func() {
		// Complete matching before starting user handlers.
		matched := matchingEventSinks(sinks, event.matchData)
		dispatchMatchedScriptEventBatch(matched, event)
	})
}

// The start dispatcher already runs in a managed coroutine when gco is non-nil.
func (p *scriptEventRegistry) dispatchStartEventBatch(sinks []eventSink, event scriptEventDispatch) {
	matched := matchingEventSinks(sinks, event.matchData)
	if len(matched) == 0 || gco == nil {
		dispatchMatchedScriptEventBatch(matched, event)
		return
	}

	baseline := p.stopAllEpoch.Load()
	tasks := make([]coroutine.Task, len(matched))
	for i, sink := range matched {
		tasks[i] = coroutine.Task{
			Owner: sink.Owner,
			RegisterInvocation: func(thread coroutine.Thread) func() {
				p.pendingStartThreads.Store(thread, struct{}{})
				// A canceled start may exit before Run clears its pending marker.
				return func() { p.pendingStartThreads.Delete(thread) }
			},
			Run: func(thread coroutine.Thread) {
				p.pendingStartThreads.Delete(thread)
				if p.stopAllEpoch.Load() != baseline {
					gco.StopAtNextYield(thread)
				}
				event.run(thread, &sink)
			},
		}
	}

	gco.StartBatch(tasks, event.mode)
}

func (p *scriptEventRegistry) isPendingStartThread(thread coroutine.Thread) bool {
	_, ok := p.pendingStartThreads.Load(thread)
	return ok
}

// globalSinks reads live target order; callers must serialize with script slices.
func (p *scriptEventRegistry) globalSinks(bucket coreevent.Bucket) []eventSink {
	return sinksInTargetOrder(p.game, p.manager.Snapshot(bucket))
}

// Shared Event State
func (p *scriptEventRegistry) currentMessageDispatchContext() *messageDispatchContext {
	thread := currentScriptThread()
	if thread == nil {
		return new(messageDispatchContext)
	}
	execution, ok := p.messageExecutions[thread]
	if !ok {
		return new(messageDispatchContext)
	}
	execution.context.claimTurn(execution.receiver)
	return execution.context
}

// sampleConditions matches predicates within the caller's sampling barrier,
// before the frame clock advances.
func (p *scriptEventRegistry) sampleConditions() {
	p.pendingConditions = matchingEventSinks(p.globalSinks(coreevent.BucketCondition), nil)
}

// dispatchConditions registers sampled handlers within the caller's event barrier,
// without reevaluating conditions.
func (p *scriptEventRegistry) dispatchConditions() {
	sinks := p.pendingConditions
	p.pendingConditions = nil
	dispatchMatchedScriptEventBatch(sinks, scriptEventDispatch{
		mode: coroutine.BatchAsync,
		run: func(_ coroutine.Thread, sink *eventSink) {
			sink.Handler.(func())()
		},
	})
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

// Runtime Helpers
func (e scriptEventDispatch) task(sink eventSink) coroutine.Task {
	task := coroutine.Task{
		Owner: sink.Owner,
		Run:   func(thread coroutine.Thread) { e.run(thread, &sink) },
	}
	if handler, ok := sink.Handler.(scriptEventLifecycle); ok {
		task.RegisterInvocation = handler.RegisterInvocation
	}
	return task
}

// Register all handlers before returning; keep engine calls live while waiting.
func withEventRegistrationBarrier(owner any, dispatch func()) {
	if gco == nil {
		dispatch()
		return
	}
	gco.RunSync(owner, func(coroutine.Thread) {
		dispatch()
	})
}

func dispatchMatchedScriptEventBatch(matched []eventSink, event scriptEventDispatch) {
	if len(matched) == 0 {
		return
	}
	if gco == nil {
		for i := range matched {
			event.run(nil, &matched[i])
		}
		return
	}

	tasks := make([]coroutine.Task, len(matched))
	for i, sink := range matched {
		tasks[i] = event.task(sink)
	}
	gco.StartBatch(tasks, event.mode)
}

func eventBatchMode(wait bool) coroutine.BatchMode {
	if wait {
		return coroutine.BatchWaitDone
	}
	return coroutine.BatchAsync
}

func matchingEventSinks(sinks []eventSink, matchData any) []eventSink {
	var matched []eventSink
	for _, sink := range sinks {
		if sink.Cond == nil || sink.Cond(matchData) {
			if matched == nil {
				matched = make([]eventSink, 0, len(sinks))
			}
			matched = append(matched, sink)
		}
	}
	return matched
}

// sinksInTargetOrder uses the live front-to-back sprite order and puts
// the stage last while preserving registration order within each owner.
func sinksInTargetOrder(game *Game, sinks []eventSink) []eventSink {
	if len(sinks) < 2 {
		return sinks
	}
	if game == nil {
		return slices.Clone(sinks)
	}

	shapes := game.getAllShapes()
	// Link immutable snapshot entries to avoid per-sprite slices.
	// Indices are one-based; zero ends a group.
	heads := make(map[*SpriteImpl]int, len(shapes))
	for _, shape := range shapes {
		if sprite, ok := shape.(*SpriteImpl); ok {
			heads[sprite] = 0
		}
	}
	next := make([]int, len(sinks))
	unknown, stage := 0, 0
	for i, sink := range slices.Backward(sinks) {
		switch owner := sink.Owner.(type) {
		case *SpriteImpl:
			if head, live := heads[owner]; live {
				next[i], heads[owner] = head, i+1
				continue
			}
		case *Game:
			if owner == game {
				next[i], stage = stage, i+1
				continue
			}
		}
		next[i], unknown = unknown, i+1
	}

	ordered := make([]eventSink, 0, len(sinks))
	appendGroup := func(head int) {
		for head != 0 {
			ordered = append(ordered, sinks[head-1])
			head = next[head-1]
		}
	}
	for _, shape := range slices.Backward(shapes) {
		if sprite, ok := shape.(*SpriteImpl); ok {
			appendGroup(heads[sprite])
		}
	}
	appendGroup(unknown)
	appendGroup(stage)
	return ordered
}

func nameOf(this any) string {
	if spr, ok := this.(*SpriteImpl); ok {
		return spr.name
	}
	if _, ok := this.(*Game); ok {
		return "Game"
	}
	engine.Panic("scriptEventBindings: unexpected this object")
	return ""
}
