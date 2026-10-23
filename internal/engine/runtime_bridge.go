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
	"maps"
	"reflect"
	"slices"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/coroutine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type runtimeBridge struct{}

// runtimeCallbacks batches SPX game callbacks so later callbacks can run when
// an earlier one yields. Other clients invoke callbacks synchronously.
type runtimeCallbacks struct {
	binding *gameBinding
	tasks   []coroutine.Task
}

func init() {
	gdx.SetRuntimeBridge(runtimeBridge{})
}

func (runtimeBridge) InternalUpdateFrame(delta float64, update func(float64)) {
	callbacks := runtimeCallbacks{binding: activeGame.Load()}
	callbacks.updateSprites(delta)
	// Run the hook before advancing timers and tweens it may create.
	if update != nil {
		update(delta)
	}
	updateTimers(delta, callbacks.emit)
	updateTweens(delta, callbacks.emit)
	if len(callbacks.tasks) != 0 && isRuntimeWorkCurrent(callbacks.binding) {
		gco.StartBatch(callbacks.tasks, coroutine.BatchAsync)
	}
}

func (runtimeBridge) AdvanceTimeSinceGameStart(delta float64) float64 {
	state.timeSinceStart += delta
	return state.timeSinceStart
}

func (runtimeBridge) TimeSinceGameStarted() float64 {
	return state.timeSinceStart
}

func (runtimeBridge) Sprites() map[Object]gdx.ISpriter {
	return state.sprites
}

func (runtimeBridge) GetSprite(id Object) gdx.ISpriter {
	return lookupSprite(id)
}

func (runtimeBridge) DeleteSprite(id Object) {
	deleteSprite(id)
}

func (runtimeBridge) ClearAllSprites() {
	clearAllSprites()
}

func (runtimeBridge) UiNodes() map[Object]gdx.IUiNode {
	return state.uiNodes
}

func (runtimeBridge) GetUINode(id Object) gdx.IUiNode {
	return state.uiNodes[id]
}

func (runtimeBridge) DeleteUINode(id Object) {
	delete(state.uiNodes, id)
}

func (runtimeBridge) RegisterSpriteType(t reflect.Type) {
	state.spriteTypes[t.Name()] = t
}

func (runtimeBridge) BindSceneInstantiatedSprite(id Object, typeName string) {
	bindSceneInstantiatedSprite(id, typeName)
}

func (runtimeBridge) CreateSprite(t reflect.Type, pos mathf.Vec2) reflect.Value {
	return createPrefabSprite(t, pos)
}

func (runtimeBridge) CreateEmptySprite(t reflect.Type, pos mathf.Vec2) reflect.Value {
	return createBareSprite(t, pos)
}

func (runtimeBridge) CreateBackdrop(t reflect.Type) reflect.Value {
	return createBackdrop(t)
}

func (runtimeBridge) CreateUI(t reflect.Type, prefabName string, isEngine bool) reflect.Value {
	return createUI(t, prefabName, isEngine)
}

func (runtimeBridge) BindUI(t reflect.Type, parentNode Object, path string) reflect.Value {
	return bindUI(t, parentNode, path)
}

func (runtimeBridge) DelayCall(delay float64, callback func()) {
	delayCall(delay, callback)
}

func (runtimeBridge) DelaySpriteCall(delay float64, sprite gdx.ISpriter, callback func()) {
	delaySpriteCall(delay, sprite, callback)
}

func (runtimeBridge) TweenPos(node gdx.ISpriter, pos mathf.Vec2, duration float64, callback func()) {
	tweenPos(node, pos, duration, callback)
}

func (runtimeBridge) TweenPos2(node gdx.ISpriter, pos mathf.Vec2, duration float64, pos2 mathf.Vec2, duration2 float64, callback func()) {
	tweenPos2(node, pos, duration, pos2, duration2, callback)
}

func (p *runtimeCallbacks) updateSprites(delta float64) {
	if p.binding == nil || p.binding.callbacks == nil {
		// Sprite callbacks may change the registry; finish this frame's snapshot.
		sprites := slices.AppendSeq(make([]gdx.ISpriter, 0, len(state.sprites)), maps.Values(state.sprites))
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
