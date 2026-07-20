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
	"time"

	"github.com/goplus/spx/v2/internal/coroutine"

	"github.com/goplus/spbase/mathf"
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	itime "github.com/goplus/spx/v2/internal/time"
)

// -----------------------------------------------------------------------------
// Engine Callbacks
// -----------------------------------------------------------------------------
func (p *Game) OnEngineStart() {
	p.lifecycleState.RunOnce.Do(func() {
		cachedBounds = make(map[string]mathf.Rect2)
		generation := p.currentBootstrapGeneration()
		go func() {
			defer engine.CheckPanic()
			if me, ok := p.gamer.(interface{ MainEntry() }); ok {
				p.deferBootstrapFor(generation, func() {
					p.runBootstrapMainUntilYield(p, me.MainEntry)
				})
			}
			if !p.lifecycleState.IsRunned.Load() {
				builder := newGameBuilder(p.gamer, "assets", generation)
				if err := builder.buildAndRun(); err != nil {
					engine.Panic(err)
				}
			}
			engine.OnGameStarted()
			p.lifecycleState.IsRunned.Store(true)
			p.startBootstrapPhaseFor(generation)
		}()
	})
}

func (p *Game) OnEngineDestroy() {
	p.lifecycleState.IsRunned.Store(false)
	p.abortInputSession("game destroyed")
}

func (p *Game) OnEngineReset() {
	p.reset()
}

func (p *Game) OnEngineUpdate(delta float64) {
	if !p.lifecycleState.IsRunned.Load() {
		return
	}
	// Recording and playback consume exactly one input tick per engine update.
	// Idle games retain the long-lived input coroutine and its original dispatch
	// path.
	if session := p.currentInputSession(); session != nil {
		if !session.beginFrame() {
			return
		}
		p.inputMgr.processInputSessionTick(session, itime.DeltaTime())
	}
	p.soundMgr.Update()
	p.runScriptFramePhase()
	p.updateSpriteProxies()
	p.pullPhysicsPositions()
}

// runScriptFramePhase dispatches either the initial start event or due frame callbacks.
func (p *Game) runScriptFramePhase() {
	if p.lifecycleState.BootstrapDone.Load() && !p.lifecycleState.StartDispatched.Load() {
		p.dispatchStartEventIfNeeded()
		return
	}
	engine.RunFrameCallbacks()
}

func (p *Game) OnEngineRender(delta float64) {
	if !p.lifecycleState.IsRunned.Load() {
		return
	}
	// Coroutines run between OnEngineUpdate and OnEngineRender. If one requested
	// a capture after changing visual state, flush those proxy changes now so the
	// end-of-frame screenshot observes the capture body rather than the previous
	// update's scene.
	if engine.HasPendingCaptures() || p.inputSessionFrameCompletionPending() {
		p.syncPostCoroutineVisuals()
	}
	// Initial sprite Main hooks can move and collide during bootstrap, so
	// trigger pairs must be drained before the start event is dispatched.
	p.processPhysicsTriggers()
}

// OnEngineFrameEnd runs after the current update's coroutine work, render-side
// proxy synchronization, and capture dispatch have completed. Replays pause at
// this boundary so their final effective input frame is fully observable before
// any later update can run.
func (p *Game) OnEngineFrameEnd() {
	p.finishInputSessionFrame()
}

func (p *Game) OnEnginePause(bool) {
	// Pause lifecycle hooks are intentionally handled by engine-level managers.
}

// -----------------------------------------------------------------------------
// Loop Setup
// -----------------------------------------------------------------------------
func (p *Game) runLoop(cfg *Config) (err error) {
	spxlog.Debug("RunLoop")
	if !cfg.DontRunOnUnfocused {
		p.engine().PlatformMgr.SetRunnableOnUnfocused(true)
	}
	p.initEventLoop()
	p.engine().PlatformMgr.SetWindowTitle(cfg.Title)
	return nil
}

func runMain(call func()) {
	coreruntime.RunMain(call, time.Now(), setSchedInMain, setMainSchedTime)
}

func (p *Game) runBootstrapMainUntilYield(owner coroutine.ThreadObj, mainFn func()) {
	if mainFn == nil {
		return
	}

	thread := gco.CreateAndStart(false, owner, func(coroutine.Thread) int {
		runMain(mainFn)
		return 0
	})
	gco.JoinYieldedOrDone(thread)
}

func (p *Game) runBootstrapSpriteMainsUntilYield(inits []Sprite) {
	if len(inits) == 0 {
		return
	}

	// Advance initial sprite Mains in load/Z-order until each reaches its first
	// yield (or return) before continuing bootstrap. This preserves deterministic
	// pre-yield setup while still releasing startup once a long-running Main
	// yields for the first time.
	for _, ini := range inits {
		spr := spriteOf(ini)
		if spr == nil {
			continue
		}

		p.runBootstrapMainUntilYield(spr.pthis, ini.Main)
	}
}

func (p *Game) deferBootstrap(call func()) {
	p.deferBootstrapFor(p.currentBootstrapGeneration(), call)
}

func (p *Game) deferBootstrapFor(generation uint64, call func()) bool {
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

func (p *Game) currentBootstrapGeneration() uint64 {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	return p.bootstrapGen
}

func (p *Game) resetBootstrapState() {
	p.bootstrapMu.Lock()
	p.bootstrapGen++
	p.bootstrapStarted = false
	p.pendingBootstrap = nil
	p.bootstrapMu.Unlock()
	p.lifecycleState.BootstrapDone.Store(false)
}

func (p *Game) runBootstrapTasks() {
	p.runBootstrapTasksFor(p.currentBootstrapGeneration())
}

func (p *Game) runBootstrapTasksFor(generation uint64) {
	for {
		tasks, ok := p.takeBootstrapTasksFor(generation)
		if !ok || len(tasks) == 0 {
			return
		}
		// Re-check after each pass so tasks queued by tasks are also consumed.
		for _, task := range tasks {
			if !p.isBootstrapGenerationCurrent(generation) {
				return
			}
			task()
		}
	}
}

func (p *Game) takeBootstrapTasksFor(generation uint64) ([]func(), bool) {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return nil, false
	}
	if len(p.pendingBootstrap) == 0 {
		return nil, true
	}
	tasks := p.pendingBootstrap
	p.pendingBootstrap = nil
	return tasks, true
}

func (p *Game) isBootstrapGenerationCurrent(generation uint64) bool {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	return generation == p.bootstrapGen
}

func (p *Game) claimBootstrapPhaseFor(generation uint64) (hasTasks, ok bool) {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen || p.bootstrapStarted {
		return false, false
	}
	p.bootstrapStarted = true
	return len(p.pendingBootstrap) > 0, true
}

func (p *Game) startBootstrapPhaseFor(generation uint64) {
	engine.WaitMainThread(func() {
		hasTasks, ok := p.claimBootstrapPhaseFor(generation)
		if !ok {
			return
		}
		if !hasTasks {
			p.lifecycleState.BootstrapDone.Store(true)
			return
		}

		p.lifecycleState.BootstrapDone.Store(false)
		gco.CreateAndStart(false, p, func(coroutine.Thread) int {
			p.runBootstrapTasksFor(generation)
			if p.isBootstrapGenerationCurrent(generation) {
				p.lifecycleState.BootstrapDone.Store(true)
			}
			return 0
		})
	})
}
