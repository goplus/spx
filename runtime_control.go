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
	coreevent "github.com/goplus/spx/v4/internal/core/event"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
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

// Stop stops scripts selected by kind.
// Explicit receivers support interpreted direct calls.
func (p *Game) Stop(kind StopKind) {
	p.scriptEventBindings.Stop(kind)
}

// stopAllResources removes clones and resets effects and audio.
// The caller must remain alive for engine calls during cleanup.
func (p *Game) stopAllResources() {
	p.baseObj.clearGraphicEffects()
	p.soundMgr.StopAll()
	p.clearSoundEffects(p.audioState.SoundObj)
	// Destroying clones removes shapes from the live list.
	for _, shape := range p.shapeMgr.getTempShapes() {
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
		gco.StopThisScript()
		return
	}
	filter, stopCaller := coreevent.ResolveStop(
		kind,
		owner,
		func(obj any) bool { return isSprite(obj) },
		func(obj any) bool { return isGame(obj) },
	)
	if filter == nil {
		return
	}

	var caller coroutine.Thread
	if gco.IsInCoroutine() {
		caller = gco.Current()
	}
	shouldStop := func(thread coroutine.Thread) bool {
		return filter(thread.Owner, thread == caller)
	}

	if kind == AllStop {
		p.scriptEventRegistry.stopAllEpoch.Add(1)
		gco.StopIf(func(thread coroutine.Thread) bool {
			// Keep the caller for cleanup; pending starts use the new epoch.
			return thread != caller && !p.scriptEventRegistry.isPendingStartThread(thread) && shouldStop(thread)
		})
		if game := p.scriptEventRegistry.game; game != nil {
			game.stopAllResources()
		}
	} else {
		gco.StopIf(shouldStop)
	}
	if stopCaller {
		gco.StopCurrent()
	}
}

func isGame(obj any) bool {
	_, ok := obj.(*Game)
	return ok
}

func isSprite(obj any) bool {
	_, ok := obj.(*SpriteImpl)
	return ok
}
