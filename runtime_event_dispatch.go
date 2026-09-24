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

	coreevent "github.com/goplus/spx/v3/internal/core/event"
	"github.com/goplus/spx/v3/internal/coroutine"
)

// scriptEventLifecycle registers an invocation before it can run and returns
// its cleanup. Handlers without this interface allow overlapping invocations.
type scriptEventLifecycle interface {
	Start(coroutine.Thread) func()
}

// Each registration owns its execution state; clones re-register their handlers.
type scriptEventHandler[F any] struct {
	coroutine.HandlerState
	run F
}

type messageEventHandler = scriptEventHandler[func(string, any)]

func newScriptEventSink[F any](owner threadObj, run F, policy coroutine.HandlerPolicy, cond ...func(any) bool) eventSink {
	handler := &scriptEventHandler[F]{HandlerState: coroutine.NewHandlerState(policy), run: run}
	return coreevent.NewSink(owner, handler, cond...)
}

type scriptEventDispatch struct {
	mode      coroutine.BatchMode
	matchData any
	run       func(coroutine.Thread, *eventSink)
}

func (event scriptEventDispatch) task(sink eventSink) coroutine.Task {
	task := coroutine.Task{
		Owner: sink.Owner,
		Run:   func(thread coroutine.Thread) { event.run(thread, &sink) },
	}
	if handler, ok := sink.Handler.(scriptEventLifecycle); ok {
		task.Setup = handler.Start
	}
	return task
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

func (p *scriptEventRegistry) globalSinks(bucket coreevent.Bucket) []eventSink {
	return sinksInTargetOrder(p.game, p.manager.Snapshot(bucket))
}

func (p *scriptEventRegistry) dispatchGlobal(bucket coreevent.Bucket, event scriptEventDispatch) {
	p.dispatchSinks(p.globalSinks(bucket), event)
}

func (p *scriptEventRegistry) dispatchTarget(bucket coreevent.Bucket, owner any, event scriptEventDispatch) {
	sinks := p.manager.Snapshot(bucket)
	var owned []eventSink
	for _, sink := range sinks {
		if sink.Owner == owner {
			if owned == nil {
				owned = make([]eventSink, 0, min(len(sinks), 8))
			}
			owned = append(owned, sink)
		}
	}
	if len(owned) == 0 {
		return
	}
	p.dispatchSinks(owned, event)
}

func (p *scriptEventRegistry) dispatchSinks(sinks []eventSink, event scriptEventDispatch) {
	withEventRegistrationBarrier(p.game, func() {
		// Complete matching before starting user handlers.
		matched := matchingEventSinks(sinks, event.matchData)
		dispatchMatchedScriptEventBatch(matched, event)
	})
}

func (p *scriptEventRegistry) dispatchStartSinks(sinks []eventSink, event scriptEventDispatch) {
	withEventRegistrationBarrier(p.game, func() {
		p.dispatchStartEventBatch(sinks, event)
	})
}

func (p *scriptEventRegistry) dispatchStartEventBatch(sinks []eventSink, event scriptEventDispatch) {
	matched := matchingEventSinks(sinks, event.matchData)
	if len(matched) == 0 || gco == nil {
		dispatchMatchedScriptEventBatch(matched, event)
		return
	}

	baseline := p.stopAllEpoch.Load()
	tasks := make([]coroutine.Task, len(matched))
	threads := make([]coroutine.Thread, 0, len(matched))
	defer func() {
		for _, thread := range threads {
			p.pendingStartThreads.Delete(thread)
		}
	}()
	for i, sink := range matched {
		task := event.task(sink)
		run := task.Run
		task.Setup = func(thread coroutine.Thread) func() {
			p.pendingStartThreads.Store(thread, struct{}{})
			threads = append(threads, thread)
			return nil
		}
		task.Run = func(thread coroutine.Thread) {
			p.pendingStartThreads.Delete(thread)
			if p.stopAllEpoch.Load() != baseline {
				gco.StopAtNextYield(thread)
			}
			run(thread)
		}
		tasks[i] = task
	}

	gco.StartBatch(tasks, event.mode)
}

func (p *scriptEventRegistry) isPendingStartThread(thread coroutine.Thread) bool {
	_, ok := p.pendingStartThreads.Load(thread)
	return ok
}

func eventBatchMode(wait bool) coroutine.BatchMode {
	if wait {
		return coroutine.BatchWaitDone
	}
	return coroutine.BatchAsync
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
	// Group by links into the immutable snapshot instead of allocating a slice
	// per sprite. Clone-heavy broadcasts otherwise allocate hundreds of small
	// slices before any handler runs, creating avoidable GC work on Web.
	// Indices are one-based so zero is the end of a group.
	heads := make(map[*SpriteImpl]int, len(shapes))
	for _, shape := range shapes {
		if sprite, ok := shape.(*SpriteImpl); ok {
			heads[sprite] = 0
		}
	}
	next := make([]int, len(sinks))
	unknown, stage := 0, 0
	for i := len(sinks) - 1; i >= 0; i-- {
		switch owner := sinks[i].Owner.(type) {
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
	for i := len(shapes) - 1; i >= 0; i-- {
		if sprite, ok := shapes[i].(*SpriteImpl); ok {
			appendGroup(heads[sprite])
		}
	}
	appendGroup(unknown)
	appendGroup(stage)
	return ordered
}

func matchingEventSinks(sinks []eventSink, matchData any) []eventSink {
	matched := make([]eventSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink.Cond == nil || sink.Cond(matchData) {
			matched = append(matched, sink)
		}
	}
	return matched
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
