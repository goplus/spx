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

package engine

import (
	"context"

	"github.com/goplus/spx/v4/internal/engine/coroutine"
	"github.com/goplus/spx/v4/internal/engine/profiler"
	itime "github.com/goplus/spx/v4/internal/time"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

// frameJobs owns pending work and reusable buffers for one game session.
// Each update keeps its active buffer local so nested frames cannot replace it.
type frameJobs struct {
	timers       []*timerCall
	timerScratch []*timerCall
	tweens       []*positionTween
	tweenScratch []*positionTween
}

// runtimeCallbacks batches SPX game callbacks so later callbacks can run when
// an earlier one yields. Other clients invoke callbacks synchronously.
type runtimeCallbacks struct {
	binding *gameBinding
	tasks   []coroutine.Task
}

func onUpdate(delta float64) {
	defer CheckPanic()
	binding := runningBinding()
	if binding == nil {
		return
	}
	profiler.BeginSample()
	defer profiler.EndSample()
	cacheFrameEvents()
	binding.callbacks.OnEngineBeforeUpdate(delta)
	itime.Update(delta, profiler.Calcfps())
	Execute(nil, func(context.Context, any) {
		updateRuntimeFrame(delta, nil)
	})
	if binding.loadPhase() != gameRunning {
		return
	}
	profiler.MeasureFunctionTime("GameUpdate", func() {
		binding.callbacks.OnEngineUpdate(delta)
	})
	profiler.MeasureFunctionTime("CoroUpdateJobs", scheduler.Update)
	if binding.loadPhase() != gameRunning {
		return
	}
	// Complete the frame once rendering starts.
	profiler.MeasureFunctionTime("GameRender", func() {
		binding.callbacks.OnEngineRender(delta)
	})
	if err := FlushCaptures(); err != nil {
		Panic(err)
		return
	}
	binding.callbacks.OnEngineFrameEnd()
}

func updateRuntimeFrame(delta float64, update func(float64)) {
	callbacks := runtimeCallbacks{binding: activeGame.Load()}
	callbacks.updateSprites(delta)
	// Run the hook before advancing timers and tweens it may create.
	if update != nil {
		update(delta)
	}
	state.jobs.updateTimers(delta, callbacks.emit)
	state.jobs.updateTweens(delta, callbacks.emit)
	if len(callbacks.tasks) != 0 && isRuntimeWorkCurrent(callbacks.binding) {
		scheduler.StartBatch(callbacks.tasks, coroutine.BatchAsync)
	}
}

func (p *runtimeCallbacks) updateSprites(delta float64) {
	if p.binding == nil || p.binding.callbacks == nil {
		// Sprite callbacks may change the registry; finish this frame's snapshot.
		sprites := snapshotSprites()
		for _, sprite := range sprites {
			sprite.OnUpdate(delta)
		}
		return
	}
	for _, sprite := range state.sprites {
		switch sprite.(type) {
		case *Sprite, *gdx.Sprite:
			// Standard proxies inherit the empty OnUpdate method.
			continue
		}
		// Each frame starts an independent update; waiting calls may overlap.
		// Object deletion cancels every outstanding invocation.
		p.emit(sprite, func() { sprite.OnUpdate(delta) })
	}
}

func (p *runtimeCallbacks) emit(sprite gdx.ISpriter, call func()) {
	binding := p.binding
	if binding == nil || binding.callbacks == nil {
		invokeRuntimeCallback(sprite, call)
		return
	}
	p.tasks = append(p.tasks, coroutine.Task{
		Owner: resolveOwner(runtimeSpriteOwner(sprite)),
		Run: func(coroutine.Thread) {
			if isRuntimeWorkCurrent(binding) {
				invokeRuntimeCallback(sprite, call)
			}
		},
	})
}

// A nil sprite identifies a global callback; object callbacks recheck lifetime
// when invoked, after state advancement or an asynchronous batch start.
func invokeRuntimeCallback(sprite gdx.ISpriter, call func()) {
	if sprite == nil || runtimeSpriteAlive(sprite) {
		call()
	}
}

func cacheFrameEvents() {
	cacheTriggerEvents()
	cacheInputState()
}

func resetFrameState() {
	resetCaptures()
	resetInputState()
	resetTriggerEvents()
}
