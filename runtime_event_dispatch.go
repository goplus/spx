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
	"github.com/goplus/spx/v3/internal/engine"
)

type scriptEventDispatch struct {
	mode      coroutine.BatchMode
	matchData any
	before    func(coroutine.Thread)
	shouldRun func() bool
	run       func(coroutine.Thread, *eventSink)
}

func (p scriptEventDispatch) task(sink eventSink) coroutine.BatchTask {
	return coroutine.BatchTask{
		Owner:  sink.Owner,
		Before: p.before,
		Run: func(thread coroutine.Thread) {
			p.invoke(thread, &sink)
		},
	}
}

func (p scriptEventDispatch) invoke(thread coroutine.Thread, sink *eventSink) {
	if p.shouldRun == nil || p.shouldRun() {
		p.run(thread, sink)
	}
}

func eventBatchMode(wait bool) coroutine.BatchMode {
	if wait {
		return coroutine.BatchWaitDone
	}
	return coroutine.BatchAsync
}

// sinksInScratchTargetOrder uses the live front-to-back sprite order and puts
// the stage last while preserving registration order within each owner.
func sinksInScratchTargetOrder(game *Game, sinks []eventSink) []eventSink {
	if len(sinks) < 2 {
		return sinks
	}
	if game == nil {
		return slices.Clone(sinks)
	}

	shapes := game.getAllShapes()
	liveSprites := make(map[*SpriteImpl]struct{}, len(shapes))
	for _, shape := range shapes {
		if sprite, ok := shape.(*SpriteImpl); ok {
			liveSprites[sprite] = struct{}{}
		}
	}

	bySprite := make(map[*SpriteImpl][]eventSink, len(liveSprites))
	unknown := make([]eventSink, 0)
	stage := make([]eventSink, 0)
	for _, sink := range sinks {
		switch owner := sink.Owner.(type) {
		case *SpriteImpl:
			if _, ok := liveSprites[owner]; ok {
				bySprite[owner] = append(bySprite[owner], sink)
			} else {
				unknown = append(unknown, sink)
			}
		case *Game:
			if owner == game {
				stage = append(stage, sink)
			} else {
				unknown = append(unknown, sink)
			}
		default:
			unknown = append(unknown, sink)
		}
	}

	ordered := make([]eventSink, 0, len(sinks))
	for i := len(shapes) - 1; i >= 0; i-- {
		if sprite, ok := shapes[i].(*SpriteImpl); ok {
			ordered = append(ordered, bySprite[sprite]...)
		}
	}
	ordered = append(ordered, unknown...)
	return append(ordered, stage...)
}

func (p *scriptEventRegistry) globalSinks(bucket coreevent.Bucket) []eventSink {
	return sinksInScratchTargetOrder(activeGame(), p.manager.Snapshot(bucket))
}

func (p *scriptEventRegistry) dispatchGlobal(bucket coreevent.Bucket, event scriptEventDispatch) {
	p.dispatchSinks(p.globalSinks(bucket), event)
}

func (p *scriptEventRegistry) dispatchTarget(bucket coreevent.Bucket, owner any, event scriptEventDispatch) {
	sinks := p.manager.Snapshot(bucket)
	owned := make([]eventSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink.Owner == owner {
			owned = append(owned, sink)
		}
	}
	p.dispatchSinks(owned, event)
}

func (p *scriptEventRegistry) dispatchSinks(sinks []eventSink, event scriptEventDispatch) {
	runScriptEventDispatch(func() {
		dispatchScriptEventBatch(sinks, event)
	})
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

// dispatchScriptEventBatch completes matching before starting user handlers.
func dispatchScriptEventBatch(sinks []eventSink, event scriptEventDispatch) {
	matched := matchingEventSinks(sinks, event.matchData)
	if len(matched) == 0 {
		return
	}
	if gco == nil {
		for i := range matched {
			event.invoke(nil, &matched[i])
		}
		return
	}

	tasks := make([]coroutine.BatchTask, len(matched))
	for i, sink := range matched {
		tasks[i] = event.task(sink)
	}
	gco.StartBatch(tasks, event.mode)
}

func (p *scriptEventRegistry) dispatchStartSinks(sinks []eventSink, event scriptEventDispatch) {
	runScriptEventDispatch(func() {
		p.dispatchStartEventBatch(sinks, event)
	})
}

func (p *scriptEventRegistry) dispatchStartEventBatch(sinks []eventSink, event scriptEventDispatch) {
	matched := matchingEventSinks(sinks, event.matchData)
	if len(matched) == 0 {
		return
	}
	if gco == nil {
		for i := range matched {
			event.invoke(nil, &matched[i])
		}
		return
	}

	baseline := p.stopAllEpoch.Load()
	tasks := make([]coroutine.BatchTask, len(matched))
	threads := make([]coroutine.Thread, 0, len(matched))
	defer func() {
		for _, thread := range threads {
			p.pendingStartThreads.Delete(thread)
		}
	}()
	for i, sink := range matched {
		task := event.task(sink)
		run := task.Run
		task.OnRegistered = func(thread coroutine.Thread) {
			p.pendingStartThreads.Store(thread, struct{}{})
			threads = append(threads, thread)
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

// runScriptEventDispatch gives external callers a complete registration barrier.
func runScriptEventDispatch(call func()) {
	if gco == nil || gco.IsInCoroutine() {
		call()
		return
	}
	thread := gco.Create(engine.GetGame(), func(coroutine.Thread) int {
		call()
		return 0
	})
	gco.Join(thread)
}
