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
	if p.lifecycleState.IsRunned.Load() {
		if session := p.currentInputSession(); session != nil && !p.inputMgr.prepareInputSessionTick(session, delta) {
			return
		}
	}
	if p.lifecycleState.StartDispatched.Load() {
		p.scriptEvents.sampleConditions()
	}
}

func (p *Game) OnEngineUpdate(float64) {
	if !p.lifecycleState.IsRunned.Load() {
		return
	}
	session := p.currentInputSession()
	if session != nil && session.input.pending == nil {
		return
	}
	p.scriptEvents.dispatchConditions()
	if session != nil {
		p.inputMgr.dispatchInputSessionTick(session)
	}
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
	p.shapeMgr.takeCloneProxyPublications()
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

func (p *Game) runFrameScripts() {
	if p.lifecycleState.BootstrapDone.Load() && !p.lifecycleState.StartDispatched.Load() {
		p.dispatchStartEventIfNeeded()
		return
	}
	engine.RunFrameCallbacks()
}

func runMainUntilYield(owner coroutine.ThreadObj, mainFn func()) {
	thread := gco.Create(owner, func(coroutine.Thread) int {
		runMain(mainFn)
		return 0
	})
	gco.JoinYieldedOrDone(thread)
}

func runSpriteMainsUntilYield(inits []Sprite) {
	// Preserve load/Z-order while letting each Main run until its first yield.
	for _, ini := range inits {
		spr := spriteOf(ini)
		if spr == nil {
			continue
		}

		runMainUntilYield(spr.owner, ini.Main)
	}
}

func (p *Game) queueBootstrap(generation uint64, call func()) bool {
	if call == nil {
		return false
	}
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return false
	}
	p.pendingBootstrap = append(p.pendingBootstrap, call)
	return true
}

func (p *Game) bootstrapGeneration() uint64 {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	return p.bootstrapGen
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

func (p *Game) runBootstrapTasks(generation uint64) {
	for {
		tasks := p.takeBootstrapTasks(generation)
		if len(tasks) == 0 {
			return
		}
		// Also drain tasks queued by earlier tasks.
		for _, task := range tasks {
			if !p.isCurrentBootstrap(generation) {
				return
			}
			task()
		}
	}
}

func (p *Game) takeBootstrapTasks(generation uint64) []func() {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return nil
	}
	tasks := p.pendingBootstrap
	p.pendingBootstrap = nil
	return tasks
}

func (p *Game) isCurrentBootstrap(generation uint64) bool {
	return generation == p.bootstrapGeneration()
}

func (p *Game) completeBootstrap(generation uint64) bool {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return false
	}
	p.lifecycleState.BootstrapDone.Store(true)
	return true
}

func (p *Game) markGameStarted(generation uint64) bool {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return false
	}
	engine.OnGameStarted()
	p.lifecycleState.IsRunned.Store(true)
	return true
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
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return false
	}
	p.lifecycleState.StartDispatched.Store(true)
	return true
}

func (p *Game) claimBootstrap(generation uint64) bool {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen || p.bootstrapStarted {
		return false
	}
	p.bootstrapStarted = true
	return true
}

func (p *Game) startBootstrap(generation uint64) {
	engine.Go(p, func(context.Context) {
		if currentGame() != p || !p.claimBootstrap(generation) {
			return
		}
		p.runBootstrapTasks(generation)
		p.completeBootstrap(generation)
	})
}

func runMain(call func()) {
	if gco == nil || !gco.IsInCoroutine() {
		call()
		return
	}
	thread := gco.Current()
	end := thread.BeginMain(time.Now())
	defer end()
	call()
}
