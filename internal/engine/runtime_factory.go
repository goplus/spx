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
	spxlog "github.com/goplus/spx/v3/internal/log"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func CreateBareSpriteForType[T any](pos mathf.Vec2) *T {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	value := createBareSprite(tType, pos)
	return value.Addr().Interface().(*T)
}

func CreateBackdropForType[T any]() *T {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	value := createBackdrop(tType)
	return value.Addr().Interface().(*T)
}

func CreateEngineUIForType[T any](prefabName string) *T {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	value := createUI(tType, prefabName, true)
	return value.Addr().Interface().(*T)
}

func BindUIForType[T any](parentNode Object, path string) *T {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	value := bindUI(tType, parentNode, path)
	if !value.IsValid() {
		return nil
	}
	return value.Addr().Interface().(*T)
}

func bindSceneInstantiatedSprite(id Object, typeName string) {
	if t, ok := state.spriteTypes[typeName]; ok {
		createSpriteValue(t, id)
		return
	}
	spxlog.Error("BindSceneInstantiatedSprite: type not found %s", typeName)
}

func createPrefabSprite(t reflect.Type, pos mathf.Vec2) reflect.Value {
	id := Managers().SpriteMgr.CreateSprite(getPrefabPath(t.Name()), pos)
	return createSpriteValue(t, id)
}

func createBareSprite(t reflect.Type, pos mathf.Vec2) reflect.Value {
	id := Managers().SpriteMgr.CreateBareSprite(pos)
	return createSpriteValue(t, id)
}

func createBackdrop(t reflect.Type) reflect.Value {
	id := Managers().SpriteMgr.CreateBackdrop("")
	return createSpriteValue(t, id)
}

func createUI(t reflect.Type, prefabName string, isEngine bool) reflect.Value {
	name := t.Name()
	if prefabName != "" {
		name = prefabName
	}
	nodeValue := reflect.New(t).Elem()
	id := Managers().UiMgr.CreateNode(getUIPath(name, isEngine))
	return initUIValue(nodeValue, id)
}

func bindUI(t reflect.Type, parentNode Object, path string) reflect.Value {
	id := Managers().UiMgr.BindNode(parentNode, path)
	if id == 0 {
		spxlog.Error("BindUI failed: parentNode=%d path=%s", parentNode, path)
		return reflect.Value{}
	}
	nodeValue := reflect.New(t).Elem()
	return initUIValue(nodeValue, id)
}

func initUIValue(value reflect.Value, id Object) reflect.Value {
	node := value.Addr().Interface().(gdx.IUiNode)
	gdx.InitUINodeInstance(id, node, func(id Object, node gdx.IUiNode) {
		state.uiNodes[id] = node
	})
	return value
}

func createSpriteValue(t reflect.Type, id Object) reflect.Value {
	spriteValue := reflect.New(t).Elem()
	sprite := spriteValue.Addr().Interface().(gdx.ISpriter)
	gdx.InitSpriteInstance(id, sprite, func(id Object, sprite gdx.ISpriter) {
		state.sprites[id] = sprite
	})
	return spriteValue
}

func getPrefabPath(name string) string {
	return "res://assets/prefabs/" + name + ".tscn"
}

func getUIPath(name string, isEngine bool) string {
	if isEngine {
		return "res://engine/ui/" + name + ".tscn"
	}
	return "res://assets/ui/" + name + ".tscn"
}
