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
	"runtime/debug"
	"time"

	spxlog "github.com/goplus/spx/v4/internal/base/log"
	coreevent "github.com/goplus/spx/v4/internal/core/event"
	coreruntime "github.com/goplus/spx/v4/internal/core/runtime"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
)

// IsRunWithoutScreenRefresh reports whether the current coroutine is in warp mode.
func IsRunWithoutScreenRefresh() bool {
	return engine.IsRunWithoutScreenRefresh()
}

// SetRunWithoutScreenRefresh changes warp mode and returns its previous value.
func SetRunWithoutScreenRefresh(enabled bool) (previous bool) {
	return engine.SetRunWithoutScreenRefresh(enabled)
}

// Warp runs call in warp mode and restores the previous mode afterward.
func Warp(call func()) {
	engine.RunWithoutScreenRefresh(call)
}

// Procedure runs call at a Scratch custom-procedure boundary.
// A stop-this-script signal exits the procedure and resumes its caller.
func Procedure(call func()) {
	engine.RunStopThisScript(call)
}

// Scheduling.
func SchedNow() int {
	if thread := scheduler.Caller(); thread != nil && !handleMainExecutionTimeout(thread) {
		scheduler.Sched(thread)
	}
	return 0
}

func Sched() int {
	thread := scheduler.Caller()
	if thread == nil || handleMainExecutionTimeout(thread) || !thread.IsSchedTimeout(schedTimeoutMs) {
		return 0
	}
	spxlog.Warn("%s\n%s", coreruntime.LoopExecutionTimedOutMsg, debug.Stack())
	engine.WaitNextFrame()
	engine.Panic(coreruntime.LoopExecutionTimedOutMsg)
	return 0
}

func Forever(call func()) {
	coreruntime.Forever(call, engine.NewControlFlowWaiter())
}

func Repeat(times int, call func()) {
	coreruntime.Repeat(times, call, engine.NewControlFlowWaiter())
}

// The __xgo_autoclosure_ prefix preserves XGo's command-style condition syntax.
func RepeatUntil(__xgo_autoclosure_condition func() bool, call func()) {
	coreruntime.RepeatUntil(__xgo_autoclosure_condition, call, engine.NewControlFlowWaiter())
}

func WaitUntil(__xgo_autoclosure_condition func() bool) {
	coreruntime.WaitUntil(__xgo_autoclosure_condition, engine.NewControlFlowWaiter())
}

// Stop stops scripts selected by kind.
// Explicit receivers support interpreted direct calls.
func (p *Game) Stop(kind StopKind) {
	p.scriptEventBindings.Stop(kind)
}

// stopAllResources removes clones and resets effects and audio.
// The caller must remain alive for engine calls during cleanup.
func (p *Game) stopAllResources() {
	p.visualObject.clearGraphicEffects()
	p.soundMgr.StopAll()
	p.clearSoundEffects(p.audioState.SoundObj)
	// Destroying clones removes shapes from the live list.
	for _, shape := range p.scene.getTempShapes() {
		sprite, ok := shape.(*SpriteImpl)
		if !ok {
			continue
		}
		sprite.clearGraphicEffects()
		if sprite.IsCloned() {
			sprite.destroy()
		} else if sound := sprite.components.sound; sound != nil {
			p.clearSoundEffects(sound.soundObj)
		}
	}
}

// Stop stops scripts selected by kind.
func (p *SpriteImpl) Stop(kind StopKind) {
	p.scriptEventBindings.Stop(kind)
}

func (p *scriptEventBindings) Stop(kind StopKind) {
	// Preserve nil-receiver behavior for ThisScript.
	owner := p.owner
	if kind == ThisScript {
		// Procedure handles this signal without stopping its caller.
		scheduler.StopThisScript()
		return
	}
	filter, stopCaller := coreevent.ResolveStop(kind, owner, isSprite, isGame)
	if filter == nil {
		return
	}

	caller := scheduler.Caller()
	shouldStop := func(thread coroutine.Thread) bool {
		return filter(thread.Owner, thread == caller)
	}

	if kind == AllStop {
		p.scriptEventRegistry.stopAllEpoch.Add(1)
		scheduler.StopIf(func(thread coroutine.Thread) bool {
			// Keep the caller for cleanup; pending starts use the new epoch.
			return thread != caller && !p.scriptEventRegistry.isPendingStartThread(thread) && shouldStop(thread)
		})
		if game := p.scriptEventRegistry.game; game != nil {
			game.stopAllResources()
		}
	} else {
		scheduler.StopIf(shouldStop)
	}
	if stopCaller {
		scheduler.StopCurrent()
	}
}

// runBetweenScripts excludes script slices during frame snapshots and commits.
func runBetweenScripts(call func()) {
	if scheduler == nil {
		call()
		return
	}
	scheduler.RunBetweenScripts(call)
}

func runMainUntilYield(owner any, mainFn func()) {
	thread := scheduler.Create(owner, func(coroutine.Thread) {
		runMain(mainFn)
	})
	scheduler.JoinFirstSlice(thread)
}

// runMain calls Main synchronously while tracking its execution timeout.
func runMain(call func()) {
	if thread := scheduler.Caller(); thread != nil {
		end := thread.BeginMain(time.Now())
		defer end()
	}
	call()
}

func handleMainExecutionTimeout(thread coroutine.Thread) bool {
	if !coreruntime.MainExecutionTimedOut(thread.MainStartedAt(), time.Now(), mainExecTimeoutSec*time.Second) {
		return false
	}
	spxlog.Warn("%s\n%s", coreruntime.MainExecutionTimedOutMsg, debug.Stack())
	// Warn once, then schedule Main as a regular coroutine.
	thread.DisableMainTimeout()
	scheduler.Sched(thread)
	return true
}

func isGame(obj any) bool {
	_, ok := obj.(*Game)
	return ok
}

func isSprite(obj any) bool {
	_, ok := obj.(*SpriteImpl)
	return ok
}
