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

	"github.com/goplus/spbase/mathf"
	coreevent "github.com/goplus/spx/v4/internal/core/event"
	"github.com/goplus/spx/v4/internal/coroutine"
)

// -----------------------------------------------------------------------------
// Shared Types
// -----------------------------------------------------------------------------
type event any

// -----------------------------------------------------------------------------
// Event Payloads
// -----------------------------------------------------------------------------
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

// Retrigger behavior belongs to the event kind; execution state belongs to
// each registration. Events without a policy keep their plain callbacks.
func (p *scriptEventRegistry) register(bucket coreevent.Bucket, owner, run any, cond ...func(any) bool) {
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
	p.manager.Add(bucket, coreevent.NewSink(owner, handler, cond...))
}

func (p *scriptEventRegistry) dispatchGlobal(bucket coreevent.Bucket, event scriptEventDispatch) {
	p.dispatchSinks(p.globalSinks(bucket), event)
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

func (p *scriptEventRegistry) globalSinks(bucket coreevent.Bucket) []eventSink {
	return sinksInTargetOrder(p.game, p.manager.Snapshot(bucket))
}

// Register all handlers before returning; keep engine calls live while waiting.
func withEventRegistrationBarrier(owner any, dispatch func()) {
	if gco == nil || gco.IsInCoroutine() {
		dispatch()
		return
	}
	if gco.TryRunFromEngine(owner, dispatch) {
		return
	}
	dispatcher := gco.Create(owner, func(coroutine.Thread) {
		dispatch()
	})
	gco.Join(dispatcher)
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
