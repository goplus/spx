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
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	itime "github.com/goplus/spx/v4/internal/time"
)

// -----------------------------------------------------------------------------
// Engine Callbacks
// -----------------------------------------------------------------------------
func (p *Game) OnEngineStart() {
	p.lifecycleState.RunOnce.Do(func() {
		cachedBounds = make(map[string]mathf.Rect2)
		generation := p.bootstrapGeneration()
		engine.Go(p, func(context.Context, any) {
			if !p.isCurrentBootstrap(generation) {
				return
			}
			if me, ok := p.gamer.(interface{ MainEntry() }); ok {
				p.queueBootstrap(generation, func() {
					runMainUntilYield(p, me.MainEntry)
				})
			}
			if !p.lifecycleState.IsRunned.Load() {
				if err := p.loadGame("assets", generation); err != nil {
					engine.Panic(err)
					return
				}
			}
			if !p.markGameStarted(generation) {
				return
			}
			p.startBootstrap(generation)
		})
	})
}

func (p *Game) OnEngineDestroy() {
	p.resetBootstrap()
	p.discardPenCommands()
	p.abortInputSession("game destroyed")
}

func (p *Game) OnEngineReset() {
	p.reset()
}

// OnEngineBeforeUpdate snapshots physics, input, and conditions before the clock advances.
func (p *Game) OnEngineBeforeUpdate(delta float64) {
	p.scriptEvents.pendingConditions = nil
	if !p.lifecycleState.IsRunned.Load() {
		return
	}
	runBetweenScripts(func() {
		p.pullPhysicsPositions()
		if !p.inputMgr.prepareInputFrame(delta) {
			return
		}
		if p.lifecycleState.StartDispatched.Load() {
			p.scriptEvents.sampleConditions()
		}
	})
}

func (p *Game) OnEngineUpdate(float64) {
	if !p.lifecycleState.IsRunned.Load() || p.inputMgr.frame.pending == nil {
		return
	}
	// Use a managed dispatcher because update callbacks may run user code.
	withEventRegistrationBarrier(frameEventDispatcher{}, func() {
		p.soundMgr.Update()
		p.dispatchShapeUpdates()
		p.dispatchFrameEvents()
		// Register retained bootstrap contacts before OnStart.
		p.processPhysicsTriggers()
		p.dispatchStartEventIfNeeded()
	})
}

// dispatchShapeUpdates runs under the managed frame dispatcher. Each shape's
// first slice precedes event registration; future-frame waits stay independent.
func (p *Game) dispatchShapeUpdates() {
	delta := itime.DeltaTime()
	generation := p.bootstrapGeneration()
	var tasks []coroutine.Task
	// Task collection cannot run user code, so the live list is stable here.
	for _, shape := range p.shapeMgr.all() {
		switch updater := shape.(type) {
		case bubbleShape:
			// Bubble visuals are committed at frame end.
		case interface{ onUpdate(float64) }:
			tasks = append(tasks, coroutine.Task{
				Owner: p,
				Run: func(coroutine.Thread) {
					if currentGame() == p && p.isCurrentBootstrap(generation) {
						updater.onUpdate(delta)
					}
				},
			})
		}
	}
	gco.StartBatch(tasks, coroutine.BatchWaitFirstSlice)
}

func (p *Game) OnEngineRender(float64) {
	runBetweenScripts(func() {
		defer p.flushPenCommands()
		if !p.lifecycleState.IsRunned.Load() {
			return
		}
		p.camera.onUpdate()
		activeShapes := p.shapeMgr.getTempShapes()
		p.flushSpriteProxyChanges(activeShapes)
		p.shapeMgr.flushBubbleVisuals(activeShapes)
	})
}

// OnEngineFrameEnd pauses completed replays after rendering and capture.
func (p *Game) OnEngineFrameEnd() {
	p.finishInputSessionFrame()
}

func (p *Game) OnEnginePause(bool) {
	// Pause is handled by engine managers.
}

// -----------------------------------------------------------------------------
// Bootstrap
// -----------------------------------------------------------------------------
func (p *Game) startBootstrap(generation uint64) {
	engine.Go(p, func(context.Context, any) {
		if currentGame() != p || !p.claimBootstrap(generation) {
			return
		}
		p.runBootstrapTasks(generation)
		p.completeBootstrap(generation)
	})
}

func (p *Game) resetBootstrap() {
	p.bootstrapMu.Lock()
	p.bootstrapGen++
	p.bootstrapStarted = false
	p.startScheduled = false
	p.pendingBootstrap = nil
	p.lifecycleState.IsRunned.Store(false)
	p.lifecycleState.BootstrapDone.Store(false)
	p.lifecycleState.StartDispatched.Store(false)
	p.bootstrapMu.Unlock()
}

func (p *Game) completeBootstrap(generation uint64) bool {
	return p.markBootstrapFlag(generation, &p.lifecycleState.BootstrapDone)
}

func (p *Game) markGameStarted(generation uint64) bool {
	return p.markBootstrapFlag(generation, &p.lifecycleState.IsRunned)
}

// -----------------------------------------------------------------------------
// Start Events
// -----------------------------------------------------------------------------
func (p *Game) dispatchStartEventIfNeeded() {
	if ev := p.scheduleStartEvent(); ev != nil {
		p.handleEvent(ev)
	}
}

func (p *Game) scheduleStartEvent() *eventStart {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if !p.lifecycleState.BootstrapDone.Load() || p.lifecycleState.StartDispatched.Load() || p.startScheduled {
		return nil
	}
	p.startScheduled = true
	return &eventStart{generation: p.bootstrapGen}
}

func (p *Game) takeStartSinks(generation uint64) ([]eventSink, bool) {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return nil, false
	}
	return p.scriptEvents.manager.SnapshotStartOnce(), true
}

func (p *Game) markStartDispatched(generation uint64) bool {
	return p.markBootstrapFlag(generation, &p.lifecycleState.StartDispatched)
}

// -----------------------------------------------------------------------------
// Execution Helpers
// -----------------------------------------------------------------------------

// runBetweenScripts excludes script slices during frame snapshots and commits.
func runBetweenScripts(call func()) {
	if gco == nil {
		call()
		return
	}
	gco.RunBetweenScripts(call)
}

func runMainUntilYield(owner any, mainFn func()) {
	thread := gco.Create(owner, func(coroutine.Thread) {
		runMain(mainFn)
	})
	gco.JoinFirstSlice(thread)
}

// runMain calls Main synchronously while tracking its execution timeout.
func runMain(call func()) {
	if thread := currentScriptThread(); thread != nil {
		end := thread.BeginMain(time.Now())
		defer end()
	}
	call()
}
