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
	"fmt"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine/wrap"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type UiNode = gdx.UiNode

type layerSortMode int

const (
	layerSortModeNone layerSortMode = iota
	layerSortModeVertical
)

var managers wrap.EngineManagers

var windowScale = 1.0

// Managers returns the process-wide manager bridge used by the single runtime.
func Managers() *wrap.EngineManagers {
	return &managers
}

func SetDebugMode(isDebug bool) {
	Managers().PlatformMgr.SetDebugMode(isDebug)
}

func ReadAllText(path string) string {
	return Managers().ResMgr.ReadAllText(path)
}

func HasFile(path string) bool {
	return Managers().ResMgr.HasFile(path)
}

func ListDirectories(path string) string {
	return Managers().ResMgr.ListDirectories(path)
}

func SetWindowScale(scale float64) {
	windowScale = scale
}

func WindowScale() float64 {
	return windowScale
}

func BridgeSetCameraPosition(pos mathf.Vec2) {
	Managers().CameraMgr.SetCameraPosition(pos)
}

func BridgeWorldToView(pos mathf.Vec2) mathf.Vec2 {
	cameraOffset, viewScale := bridgeCameraTransform()
	return pos.Sub(cameraOffset).Mulf(viewScale)
}

// SetLayerSortMode selects sprite sorting by name.
func SetLayerSortMode(name string) error {
	mode := layerSortModeNone
	switch name {
	case "", "none":
	case "vertical":
		mode = layerSortModeVertical
	default:
		return fmt.Errorf("unknown layer sort mode: %s", name)
	}

	state.layerSortMode = mode
	Managers().ExtMgr.SetLayerSorterMode(int64(mode))
	return nil
}

func HasLayerSortMethod() bool {
	return state.layerSortMode != layerSortModeNone
}

// NewBackdropProxy creates and configures a backdrop on the engine main thread.
func NewBackdropProxy(obj any, path string, renderScale float64) *Sprite {
	return waitMainThreadValue(func() *Sprite {
		sprite := CreateBackdropForType[Sprite]()
		sprite.Target = obj
		sprite.SetZIndex(-1)
		sprite.DisablePhysic()
		sprite.UpdateTexture(path, renderScale, true)
		return sprite
	})
}

// BridgeNewBareSprite creates a proxy in the caller's main-thread operation.
func BridgeNewBareSprite(obj any, pos mathf.Vec2) *Sprite {
	syncSprite := CreateBareSpriteForType[Sprite](pos)
	syncSprite.Target = obj
	return syncSprite
}

func GetSprite(id gdx.Object) *Sprite {
	sprite, _ := lookupSprite(id).(*Sprite)
	return sprite
}

func ClearAllSprites() {
	WaitMainThread(func() {
		clearAllSprites()
	})
}

// NewUiNode creates and initializes a node on the engine main thread.
func NewUiNode[T any]() *T {
	return waitMainThreadValue(func() *T {
		return CreateEngineUIForType[T]("")
	})
}

// BridgeBindUI binds and initializes a node in the caller's main-thread operation.
func BridgeBindUI[T any](parentNode Object, path string) *T {
	return BindUIForType[T](parentNode, path)
}

func bridgeCameraTransform() (mathf.Vec2, float64) {
	cameraMgr := Managers().CameraMgr
	cameraPos := cameraMgr.GetCameraPosition()
	return cameraPos, cameraMgr.GetCameraZoom().X / WindowScale()
}

func waitMainThreadValue[T any](fn func() T) (value T) {
	WaitMainThread(func() {
		value = fn()
	})
	return value
}
