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
					runMain(me.MainEntry)
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
}

func (p *Game) OnEngineReset() {
	p.reset()
}

func (p *Game) OnEngineUpdate(delta float64) {
	if !p.lifecycleState.IsRunned.Load() {
		return
	}
	p.soundMgr.Update()
	if p.lifecycleState.BootstrapDone.Load() {
		p.dispatchStartEventIfNeeded()
	}
	p.updateSpriteProxies()
	p.pullPhysicsPositions()
}

func (p *Game) OnEngineRender(delta float64) {
	if !p.lifecycleState.StartDispatched.Load() {
		return
	}
	p.processPhysicsTriggers()
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
