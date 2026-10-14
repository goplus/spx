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
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
)

// -----------------------------------------------------------------------------
// Engine Callbacks
// -----------------------------------------------------------------------------
func (p *Game) OnEngineStart() {
	p.lifecycleState.RunOnce.Do(func() {
		cachedBounds = make(map[string]mathf.Rect2)
		generation := p.bootstrapGeneration()
		engine.Go(p, func(context.Context) {
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

// OnEngineBeforeUpdate samples input and conditions before the clock advances.
func (p *Game) OnEngineBeforeUpdate(delta float64) {
	p.scriptEvents.pendingConditions = nil
	running := p.lifecycleState.IsRunned.Load()
	if !running && !p.lifecycleState.StartDispatched.Load() {
		return
	}
	sample := func() {
		if running && !p.inputMgr.prepareInputFrame(delta) {
			return
		}
		if p.lifecycleState.StartDispatched.Load() {
			p.scriptEvents.sampleConditions()
		}
	}
	if gco == nil {
		sample()
	} else {
		gco.RunBetweenScripts(sample)
	}
}

func (p *Game) OnEngineUpdate(float64) {
	if !p.lifecycleState.IsRunned.Load() || p.inputMgr.frame.pending == nil {
		return
	}
	p.dispatchFrameEvents()
	p.soundMgr.Update()
	p.runFrameScripts()
	p.updateSpriteProxies()
	p.pullPhysicsPositions()
}

func (p *Game) OnEngineRender(float64) {
	defer p.flushPenCommands()
	if !p.lifecycleState.IsRunned.Load() {
		return
	}
	// Flush coroutine changes before drawing.
	p.syncPostCoroutineVisuals()
	// Drain bootstrap collisions before OnStart.
	p.processPhysicsTriggers()
}

// OnEngineFrameEnd pauses completed replays after rendering and capture.
func (p *Game) OnEngineFrameEnd() {
	p.finishInputSessionFrame()
}

func (p *Game) OnEnginePause(bool) {
	// Pause is handled by engine managers.
}

// -----------------------------------------------------------------------------
// Frame Execution
// -----------------------------------------------------------------------------
func (p *Game) runFrameScripts() {
	if p.lifecycleState.BootstrapDone.Load() && !p.lifecycleState.StartDispatched.Load() {
		p.dispatchStartEventIfNeeded()
		return
	}
	engine.RunFrameCallbacks()
}

// -----------------------------------------------------------------------------
// Bootstrap
// -----------------------------------------------------------------------------
func (p *Game) startBootstrap(generation uint64) {
	engine.Go(p, func(context.Context) {
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
// Main Execution Helpers
// -----------------------------------------------------------------------------

// runMainUntilYield starts Main in its own coroutine and waits for its first yield or exit.
func runMainUntilYield(owner coroutine.ThreadObj, mainFn func()) {
	thread := gco.Create(owner, func(coroutine.Thread) {
		runMain(mainFn)
	})
	gco.JoinYieldedOrDone(thread)
}

// runMain calls Main synchronously while tracking its execution timeout.
func runMain(call func()) {
	if thread := currentScriptThread(); thread != nil {
		end := thread.BeginMain(time.Now())
		defer end()
	}
	call()
}
