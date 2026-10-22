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

	"github.com/goplus/spx/v3/internal/coroutine"
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
		deleteSprite(id)
	}
	for id, node := range state.uiNodes {
		node.Destroy()
		delete(state.uiNodes, id)
	}
}

func lookupSprite(id Object) gdx.ISpriter {
	return state.sprites[id]
}

func deleteSprite(id Object) {
	sprite := lookupSprite(id)
	delete(state.sprites, id)
	if sprite != nil && gco != nil {
		owner := runtimeSpriteOwner(sprite)
		gco.StopIf(func(thread coroutine.Thread) bool { return thread.Owner == owner })
	}
}

func runtimeSpriteOwner(sprite gdx.ISpriter) any {
	if proxy, ok := sprite.(*Sprite); ok && proxy.Target != nil {
		return proxy.Target
	}
	return sprite
}

func runtimeSpriteAlive(sprite gdx.ISpriter) bool {
	if sprite == nil || lookupSprite(sprite.GetId()) != sprite {
		return false
	}
	if proxy, ok := sprite.(*Sprite); ok && proxy.CallbackAlive != nil {
		return proxy.CallbackAlive()
	}
	return true
}

// resolveRuntimeSprite accepts an embedded view only when it shares the live
// instance's base Sprite. Capturing that instance keeps later ID reuse isolated.
func resolveRuntimeSprite(sprite gdx.ISpriter) gdx.ISpriter {
	if sprite == nil {
		return nil
	}
	registered := lookupSprite(sprite.GetId())
	if registered == nil || registered == sprite {
		return registered
	}
	type spriteView interface{ BaseSprite() *gdx.Sprite }
	view, viewOK := sprite.(spriteView)
	live, liveOK := registered.(spriteView)
	if viewOK && liveOK {
		if base := view.BaseSprite(); base != nil && base == live.BaseSprite() {
			return registered
		}
	}
	return nil
}
