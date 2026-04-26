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

//lint:file-ignore ST1001 Bridge code intentionally dot-imports mathf to mirror engine type names.

import (
	"math"

	. "github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/time"
	gdx "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

var (
	windowScale float64 = 1.0
)

func SetWindowScale(scale float64) {
	windowScale = scale
}

func WindowScale() float64 {
	return windowScale
}

type UiNode = gdx.UiNode

func NewUiNode[T any]() *T {
	var ret *T
	WaitMainThread(func() {
		ret = CreateEngineUIForType[T]("")
	})
	return ret
}

func NewBackdropProxy(obj any, path string, renderScale float64) *Sprite {
	var ret *Sprite
	WaitMainThread(func() {
		ret = CreateBackdropForType[Sprite]()
		ret.Target = obj
		ret.SetZIndex(-1)
		ret.DisablePhysic()
		ret.UpdateTexture(path, renderScale, true)
	})
	return ret
}

func BridgeNewBareSprite(obj any, pos Vec2) *Sprite {
	syncSprite := CreateBareSpriteForType[Sprite](pos)
	syncSprite.Target = obj
	return syncSprite
}

func BridgeBindUI[T any](parentNode Object, path string) *T {
	return BindUIForType[T](parentNode, path)
}

func ReadAllText(path string) string {
	return Managers().ResMgr.ReadAllText(path)
}

func HasFile(path string) bool {
	return Managers().ResMgr.HasFile(path)
}

func SetDebugMode(isDebug bool) {
	Managers().PlatformMgr.SetDebugMode(isDebug)
}

func SetDefaultFont(path string) {
	Managers().ResMgr.SetDefaultFont(path)
}

func BridgeSetCameraPosition(pos Vec2) {
	Managers().CameraMgr.SetPosition(pos)
}

func BridgeViewToWorld(pos Vec2) Vec2 {
	cameraOffset, viewScale := bridgeCameraTransform()
	return pos.Divf(viewScale).Add(cameraOffset)
}

func BridgeWorldToView(pos Vec2) Vec2 {
	cameraOffset, viewScale := bridgeCameraTransform()
	return pos.Sub(cameraOffset).Mulf(viewScale)
}

func bridgeCameraTransform() (Vec2, float64) {
	cameraMgr := Managers().CameraMgr
	cameraPos := cameraMgr.GetPosition()
	return cameraPos, cameraMgr.GetCameraZoom().X / WindowScale()
}

func ViewToWorld(pos Vec2) Vec2 {
	var ret Vec2
	WaitMainThread(func() {
		ret = BridgeViewToWorld(pos)
	})
	return ret
}

func WorldToView(pos Vec2) Vec2 {
	var ret Vec2
	WaitMainThread(func() {
		ret = BridgeWorldToView(pos)
	})
	return ret
}

func ClearAllSprites() {
	WaitMainThread(func() {
		clearAllSprites()
	})
}

func GetSprite(id gdx.Object) *Sprite {
	target := lookupSprite(id)
	sprite, ok := target.(*Sprite)
	if ok {
		return sprite
	}
	return nil
}

func GetFPS() float64 {
	return time.FPS()
}

func DegToRad(degree float64) float64 {
	return gdx.DegToRad(degree)
}

func RadToDeg(radians float64) float64 {
	return gdx.RadToDeg(radians)
}

func Sincos(rad float64) Vec2 {
	return NewVec2(math.Sincos(rad))
}

func UniformVec2(v float64) Vec2 {
	return NewVec2(v, v)
}

func F64Tof32(slice []float64) []float32 {
	if slice == nil {
		return nil
	}
	out := make([]float32, len(slice))
	for i, v := range slice {
		out[i] = float32(v)
	}
	return out
}

func F32Tof64(slice []float32) []float64 {
	if slice == nil {
		return nil
	}
	out := make([]float64, len(slice))
	for i, v := range slice {
		out[i] = float64(v)
	}
	return out
}
