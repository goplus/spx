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
	"context"
	"sync"
	"sync/atomic"

	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/engine"
)

// gameLifecycle serializes generation changes and startup admission.
// Callbacks run outside mu; frame callbacks read the atomic gates.
type gameLifecycle struct {
	startOnce       sync.Once
	loaded          atomic.Bool // Project loaded; bootstrap and OnStart may still be pending.
	bootstrapDone   atomic.Bool
	startDispatched atomic.Bool
	mu              sync.Mutex
	generationID    uint64
	claimed         bool
	startScheduled  bool
	tasks           []func()
}

// Bootstrap.
func (p *Game) startBootstrap(generation uint64) {
	engine.Go(p, func(context.Context, any) {
		if currentGame() != p || !p.lifecycle.claim(generation) {
			return
		}
		p.lifecycle.runTasks(generation)
		p.lifecycle.completeBootstrap(generation)
	})
}

func (p *Game) reset() {
	p.lifecycle.reset()

	p.releaseGameAudio()
	p.EraseAll()

	p.scriptEvents.manager.Reset()
	p.scene.reset()

	p.debugState.DebugPanel = nil
	p.dialogState.AskPanel = nil

	p.Stop(AllOtherScripts)

	p.lifecycle.startOnce = sync.Once{}
	p.pathfindingState.once = sync.Once{}
	p.spritePrototypes = make(map[string]Sprite)

	p.abortInputSession("game reset")
	p.touchFilters = nil
	p.touchEntries = nil
	costumeSizeCache.Clear()
	p.inputMgr.init(p)

	clock.ClearTimers()
}

// Start Events.
func (p *Game) dispatchStartEventIfNeeded() {
	if ev := p.lifecycle.scheduleStart(); ev != nil {
		p.handleEvent(ev)
	}
}

func (p *Game) takeStartSinks(generation uint64) ([]eventSink, bool) {
	p.lifecycle.mu.Lock()
	defer p.lifecycle.mu.Unlock()
	if generation != p.lifecycle.generationID {
		return nil, false
	}
	return p.scriptEvents.manager.SnapshotStartOnce(), true
}

func (p *gameLifecycle) reset() {
	p.mu.Lock()
	p.generationID++
	p.claimed = false
	p.startScheduled = false
	p.tasks = nil
	p.loaded.Store(false)
	p.bootstrapDone.Store(false)
	p.startDispatched.Store(false)
	p.mu.Unlock()
}

func (p *gameLifecycle) markLoaded(generation uint64) bool {
	return p.markFlag(generation, &p.loaded)
}

func (p *gameLifecycle) completeBootstrap(generation uint64) bool {
	return p.markFlag(generation, &p.bootstrapDone)
}

func (p *gameLifecycle) scheduleStart() *eventStart {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.bootstrapDone.Load() || p.startDispatched.Load() || p.startScheduled {
		return nil
	}
	p.startScheduled = true
	return &eventStart{generation: p.generationID}
}

func (p *gameLifecycle) markStartDispatched(generation uint64) bool {
	return p.markFlag(generation, &p.startDispatched)
}

func (p *gameLifecycle) queue(generation uint64, call func()) bool {
	if call == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if generation != p.generationID {
		return false
	}
	p.tasks = append(p.tasks, call)
	return true
}

func (p *gameLifecycle) claim(generation uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if generation != p.generationID || p.claimed {
		return false
	}
	p.claimed = true
	return true
}

func (p *gameLifecycle) runTasks(generation uint64) {
	for {
		tasks := p.takeTasks(generation)
		if len(tasks) == 0 {
			return
		}
		// Also drain tasks queued by earlier tasks.
		for _, task := range tasks {
			if !p.isCurrent(generation) {
				return
			}
			task()
		}
	}
}

func (p *gameLifecycle) takeTasks(generation uint64) []func() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if generation != p.generationID {
		return nil
	}
	tasks := p.tasks
	p.tasks = nil
	return tasks
}

func (p *gameLifecycle) generation() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.generationID
}

func (p *gameLifecycle) isCurrent(generation uint64) bool {
	return generation == p.generation()
}

// Validate and publish under the reset lock so stale work cannot reopen a gate.
func (p *gameLifecycle) markFlag(generation uint64, flag *atomic.Bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if generation != p.generationID {
		return false
	}
	flag.Store(true)
	return true
}
