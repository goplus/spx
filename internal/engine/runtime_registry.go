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

	"github.com/goplus/spx/v4/internal/engine/coroutine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

// runtimeState contains data owned by one game session. Access is serialized
// with script execution and main-thread callbacks, as with the object registry.
type runtimeState struct {
	sprites        map[Object]gdx.ISpriter
	uiNodes        map[Object]gdx.IUiNode
	timeSinceStart float64
	layerSortMode  layerSortMode
	jobs           frameJobs
}

var state = runtimeState{
	sprites: make(map[Object]gdx.ISpriter),
	uiNodes: make(map[Object]gdx.IUiNode),
}

// Type registrations belong to the process and survive game sessions.
var spriteTypes = make(map[string]reflect.Type)

func IsNodeExist(id Object) bool {
	if _, ok := state.uiNodes[id]; ok {
		return true
	}
	_, ok := state.sprites[id]
	return ok
}

func registerSpriteType(t reflect.Type) {
	spriteTypes[t.Name()] = t
}

func registerSprite(id Object, sprite gdx.ISpriter) {
	state.sprites[id] = sprite
}

func lookupSprite(id Object) gdx.ISpriter {
	return state.sprites[id]
}

func deleteSprite(id Object) {
	sprite := lookupSprite(id)
	delete(state.sprites, id)
	if sprite != nil && scheduler != nil {
		owner := runtimeSpriteOwner(sprite)
		scheduler.StopIf(func(thread coroutine.Thread) bool { return thread.Owner == owner })
	}
}

// A separate snapshot lets callbacks mutate the registry or start a nested frame.
func snapshotSprites() []gdx.ISpriter {
	sprites := make([]gdx.ISpriter, 0, len(state.sprites))
	for _, sprite := range state.sprites {
		sprites = append(sprites, sprite)
	}
	return sprites
}

func registerUINode(id Object, node gdx.IUiNode) {
	state.uiNodes[id] = node
}

func lookupUINode(id Object) gdx.IUiNode {
	return state.uiNodes[id]
}

func deleteUINode(id Object) {
	delete(state.uiNodes, id)
}

func clearAllSprites() {
	for id, sprite := range state.sprites {
		sprite.Destroy()
		deleteSprite(id)
	}
	for id, node := range state.uiNodes {
		node.Destroy()
		deleteUINode(id)
	}
}

// resetRuntimeState releases Go session data after managed work has drained and
// game cleanup has finished. It makes no backend calls. Clearing the registries
// in place preserves legacy callers' live map views; type registrations survive.
func resetRuntimeState() {
	clear(state.sprites)
	clear(state.uiNodes)
	state = runtimeState{sprites: state.sprites, uiNodes: state.uiNodes}
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
