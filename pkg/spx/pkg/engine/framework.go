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
)

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

type RuntimeBridge interface {
	InternalUpdateEngine(delta float64)
	ClearAllSprites()
	RegisterSpriteType(t reflect.Type)
	GetSprite(id Object) ISpriter
	BindSceneInstantiatedSprite(id Object, typeName string)
	CreateSprite(t reflect.Type, pos mathf.Vec2) reflect.Value
	CreateEmptySprite(t reflect.Type, pos mathf.Vec2) reflect.Value
	CreateBackdrop(t reflect.Type) reflect.Value
	CreateUI(t reflect.Type, prefabName string, isEngine bool) reflect.Value
	BindUI(t reflect.Type, parentNode Object, path string) reflect.Value
	DelayCall(delay float64, callback func())
	DelaySpriteCall(delay float64, sprite ISpriter, callback func())
	TweenPos(node ISpriter, pos mathf.Vec2, duration float64, callback func())
	TweenPos2(node ISpriter, pos mathf.Vec2, duration float64, pos2 mathf.Vec2, duration2 float64, callback func())
	Sprites() map[Object]ISpriter
	UiNodes() map[Object]IUiNode
	GetUINode(id Object) IUiNode
	DeleteSprite(id Object)
	DeleteUINode(id Object)
	AdvanceTimeSinceGameStart(delta float64) float64
	TimeSinceGameStarted() float64
}

var runtimeBridge RuntimeBridge

func SetRuntimeBridge(bridge RuntimeBridge) {
	runtimeBridge = bridge
}

func requireRuntimeBridge() RuntimeBridge {
	if runtimeBridge == nil {
		panic("engine runtime bridge is not initialized")
	}
	return runtimeBridge
}

func InternalUpdateEngine(delta float64) {
	requireRuntimeBridge().InternalUpdateEngine(delta)
}

func ClearAllSprites() {
	requireRuntimeBridge().ClearAllSprites()
}

// Recommended runtime accessors.
func RegisterSpriteType[T any]() {
	requireRuntimeBridge().RegisterSpriteType(typeOf[T]())
}

func GetSprite(id Object) ISpriter {
	return requireRuntimeBridge().GetSprite(id)
}

func GetUINode(id Object) IUiNode {
	return requireRuntimeBridge().GetUINode(id)
}

func Sprites() map[Object]ISpriter {
	return requireRuntimeBridge().Sprites()
}

func UiNodes() map[Object]IUiNode {
	return requireRuntimeBridge().UiNodes()
}

func DeleteSprite(id Object) {
	requireRuntimeBridge().DeleteSprite(id)
}

func DeleteUINode(id Object) {
	requireRuntimeBridge().DeleteUINode(id)
}

func AdvanceTimeSinceGameStart(delta float64) float64 {
	return requireRuntimeBridge().AdvanceTimeSinceGameStart(delta)
}

func TimeSinceGameStarted() float64 {
	return requireRuntimeBridge().TimeSinceGameStarted()
}

func BindSceneInstantiatedSprite(id Object, type_name string) {
	requireRuntimeBridge().BindSceneInstantiatedSprite(id, type_name)
}

// Public construction helpers.
func CreateSprite[T any](pos mathf.Vec2) *T {
	spriteValue := requireRuntimeBridge().CreateSprite(typeOf[T](), pos)
	return spriteValue.Addr().Interface().(*T)
}

func CreateEmptySprite[T any](pos mathf.Vec2) *T {
	spriteValue := requireRuntimeBridge().CreateEmptySprite(typeOf[T](), pos)
	return spriteValue.Addr().Interface().(*T)
}

func CreateBackdrop[T any]() *T {
	spriteValue := requireRuntimeBridge().CreateBackdrop(typeOf[T]())
	return spriteValue.Addr().Interface().(*T)
}

func CreateUI[T any](prefabName string) *T {
	return createUI[T](prefabName, false)
}
func CreateEngineUI[T any](prefabName string) *T {
	return createUI[T](prefabName, true)
}

func createUI[T any](prefabName string, isEngine bool) *T {
	nodeValue := requireRuntimeBridge().CreateUI(typeOf[T](), prefabName, isEngine)
	return nodeValue.Addr().Interface().(*T)
}

func BindUI[T any](parentNode Object, path string) *T {
	nodeValue := requireRuntimeBridge().BindUI(typeOf[T](), parentNode, path)
	if !nodeValue.IsValid() {
		return nil
	}
	return nodeValue.Addr().Interface().(*T)
}

// Lifecycle helpers used by the internal runtime implementation.
func InitSpriteInstance(id Object, sprite ISpriter, register func(Object, ISpriter)) {
	sprite.SetId(id)
	if register != nil {
		register(id, sprite)
	}
	sprite.onCreate()
	sprite.OnStart()
}

func InitUINodeInstance(id Object, node IUiNode, register func(Object, IUiNode)) {
	node.SetId(id)
	if register != nil {
		register(id, node)
	}
	node.onCreate()
	node.OnStart()
}
