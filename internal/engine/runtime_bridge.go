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
	"reflect"

	"github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type runtimeBridge struct{}

func (runtimeBridge) InternalUpdateEngine(delta float64) {
	updateTimers(delta)
	updateTweens(delta)
}

func (runtimeBridge) ClearAllSprites() {
	clearAllSprites()
}

func (runtimeBridge) RegisterSpriteType(t reflect.Type) {
	state.spriteTypes[t.Name()] = t
}

func (runtimeBridge) GetSprite(id Object) gdx.ISpriter {
	return lookupSprite(id)
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

func (runtimeBridge) Sprites() map[Object]gdx.ISpriter {
	return state.sprites
}

func (runtimeBridge) UiNodes() map[Object]gdx.IUiNode {
	return state.uiNodes
}

func (runtimeBridge) GetUINode(id Object) gdx.IUiNode {
	return state.uiNodes[id]
}

func (runtimeBridge) DeleteSprite(id Object) {
	delete(state.sprites, id)
}

func (runtimeBridge) DeleteUINode(id Object) {
	delete(state.uiNodes, id)
}

func (runtimeBridge) AdvanceTimeSinceGameStart(delta float64) float64 {
	state.timeSinceStart += delta
	return state.timeSinceStart
}

func (runtimeBridge) TimeSinceGameStarted() float64 {
	return state.timeSinceStart
}

func init() {
	gdx.SetRuntimeBridge(runtimeBridge{})
}
