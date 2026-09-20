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

	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type runtimeState struct {
	sprites        map[Object]gdx.ISpriter
	uiNodes        map[Object]gdx.IUiNode
	spriteTypes    map[string]reflect.Type
	timeSinceStart float64
}

var state = runtimeState{
	sprites:     make(map[Object]gdx.ISpriter),
	uiNodes:     make(map[Object]gdx.IUiNode),
	spriteTypes: make(map[string]reflect.Type),
}

func IsNodeExist(id Object) bool {
	if _, ok := state.uiNodes[id]; ok {
		return true
	}
	_, ok := state.sprites[id]
	return ok
}

func clearAllSprites() {
	for id, sprite := range state.sprites {
		sprite.Destroy()
		delete(state.sprites, id)
	}
	for id, node := range state.uiNodes {
		node.Destroy()
		delete(state.uiNodes, id)
	}
}

func lookupSprite(id Object) gdx.ISpriter {
	return state.sprites[id]
}
