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
	"fmt"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	"github.com/goplus/spx/v4/internal/engine/profiler"
	"github.com/goplus/spx/v4/internal/engine/ui"
)

// Engine Callbacks.
func (p *Game) OnEngineStart() {
	p.lifecycle.startOnce.Do(func() {
		cachedBounds = make(map[string]mathf.Rect2)
		generation := p.lifecycle.generation()
		engine.Go(p, func(context.Context, any) {
			if !p.lifecycle.isCurrent(generation) {
				return
			}
			if me, ok := p.gamer.(interface{ MainEntry() }); ok {
				p.lifecycle.queue(generation, func() {
					runMainUntilYield(p, me.MainEntry)
				})
			}
			if !p.lifecycle.loaded.Load() {
				if err := p.loadGame("assets", generation); err != nil {
					engine.Panic(err)
					return
				}
			}
			if !p.lifecycle.markLoaded(generation) {
				return
			}
			p.startBootstrap(generation)
		})
	})
}

func (p *Game) OnEngineDestroy() {
	p.lifecycle.reset()
	p.discardPenCommands()
	p.abortInputSession("game destroyed")
}

func (p *Game) OnEngineReset() {
	p.reset()
}

// OnEngineBeforeUpdate snapshots physics, input, and conditions before the clock advances.
func (p *Game) OnEngineBeforeUpdate(delta float64) {
	p.scriptEvents.pendingConditions = nil
	if !p.lifecycle.loaded.Load() {
		return
	}
	runBetweenScripts(func() {
		p.pullPhysicsPositions()
		if !p.inputMgr.prepareInputFrame(delta) {
			return
		}
		if p.lifecycle.startDispatched.Load() {
			p.scriptEvents.sampleConditions()
		}
	})
}

func (p *Game) OnEngineUpdate(float64) {
	if !p.lifecycle.loaded.Load() || p.inputMgr.frame.pending == nil {
		return
	}
	// Use a managed dispatcher because update callbacks may run user code.
	withEventRegistrationBarrier(frameEventDispatcher{}, func() {
		p.soundMgr.Update()
		p.dispatchShapeUpdates()
		p.dispatchFrameEvents()
		// Register retained bootstrap contacts before OnStart.
		p.processTouchEvents()
		p.dispatchStartEventIfNeeded()
	})
}

func (p *Game) OnEngineRender(float64) {
	runBetweenScripts(func() {
		defer p.flushPenCommands()
		if !p.lifecycle.loaded.Load() {
			return
		}
		p.camera.onUpdate()
		activeShapes := p.scene.getTempShapes()
		p.flushSpriteProxyChanges(activeShapes)
		p.scene.flushBubbleVisuals(activeShapes)
	})
}

// OnEngineFrameEnd pauses completed replays after rendering and capture.
func (p *Game) OnEngineFrameEnd() {
	p.finishInputSessionFrame()
}

func (p *Game) OnEnginePause(bool) {
	// Pause is handled by engine managers.
}

// dispatchShapeUpdates runs under the managed frame dispatcher. Each shape's
// first slice precedes event registration; future-frame waits stay independent.
func (p *Game) dispatchShapeUpdates() {
	delta := clock.DeltaTime()
	generation := p.lifecycle.generation()
	var tasks []coroutine.Task
	// Task collection cannot run user code, so the live list is stable here.
	for _, shape := range p.scene.all() {
		switch updater := shape.(type) {
		case bubbleShape:
			// Bubble visuals are committed at frame end.
		case interface{ onUpdate(float64) }:
			tasks = append(tasks, coroutine.Task{
				Owner: p,
				Run: func(coroutine.Thread) {
					if currentGame() == p && p.lifecycle.isCurrent(generation) {
						updater.onUpdate(delta)
					}
				},
			})
		}
	}
	scheduler.StartBatch(tasks, coroutine.BatchWaitFirstSlice)
}

func (p *Game) logicLoop(coroutine.Thread) {
	tempAudios, tempAnimations := []string{}, []*animationPlayback{}
	for {
		tempAudios, tempAnimations = p.processLogicFrame(tempAudios, tempAnimations)
		engine.WaitNextFrame()
		p.showDebugPanel()
	}
}

func (p *Game) processLogicFrame(tempAudios []string, tempAnimations []*animationPlayback) ([]string, []*animationPlayback) {
	items := p.scene.getTempShapes()
	// Flush all pending audio before completing any animations.
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			tempAudios = sprite.flushPendingAudios(tempAudios)
		}
	}
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			tempAnimations = sprite.flushCompletedAnimations(tempAnimations)
		}
	}
	return tempAudios, tempAnimations
}

func (p *Game) showDebugPanel() {
	profiler.Debug = p.debugState.Debug
	if !p.debugState.Debug {
		if p.debugState.DebugPanel != nil {
			p.debugState.DebugPanel.Destroy()
			p.debugState.DebugPanel = nil
		}
		return
	}
	if p.debugState.DebugPanel == nil {
		p.debugState.DebugPanel = ui.NewUiDebug()
	}

	updateInfo, _ := profiler.GetStats("GameUpdate")
	coroInfo, _ := profiler.GetStats("CoroUpdateJobs")
	renderInfo, _ := profiler.GetStats("GameRender")
	lastInfo := scheduler.UpdateStats()

	msg := fmt.Sprintf("FPS: %.f\n", clock.FPS())
	msg += fmt.Sprintf("Shape: %v\n", p.scene.count())
	msg += fmt.Sprintf("GameUpdate: %v\n", updateInfo.ActualCall)
	msg += fmt.Sprintf("GameRender: %v\n", renderInfo.ActualCall)
	msg += fmt.Sprintf("CoroUpdateJobs: %v\n", coroInfo.ActualCall)
	msg += fmt.Sprintf("coro: MoveTime: %.2f\n", lastInfo.MoveTime)
	msg += fmt.Sprintf("coro: WaitTime: %.2f\n", lastInfo.WaitTime)
	msg += fmt.Sprintf("coro: TaskProcessing: %.2f\n", lastInfo.JobTime)
	msg += fmt.Sprintf("coro: TaskCounts: %v\n", lastInfo.JobCount)
	msg += fmt.Sprintf("coro: WaitFrameCount: %v\n", lastInfo.FrameWaitCount)
	msg += fmt.Sprintf("coro: WaitMainCount: %v\n", lastInfo.MainThreadCount)
	msg += fmt.Sprintf("coro: NextCount: %v\n", lastInfo.DeferredCount)
	msg += fmt.Sprintf("coro: GCCount: %v\n", lastInfo.GCCount)
	msg += fmt.Sprintf("coro: LoopIterations: %v\n", lastInfo.LoopIterations)
	p.debugState.DebugPanel.Show(msg)
}
