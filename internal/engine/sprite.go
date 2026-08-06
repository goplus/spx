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
	. "github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

// !!!Warning all method belong to this class can only be called in main thread
type Sprite struct {
	gdx.Sprite
	Name    string
	PicPath string
	Target  any
}

func (pself *Sprite) UpdateTexture(path string, renderScale float64, isUpdateTexture bool) {
	if path == "" {
		return
	}
	resPath := ToAssetPath(path)
	pself.PicPath = resPath
	if isUpdateTexture {
		pself.SetTexture(pself.PicPath)
	}
	pself.SetRenderScale(UniformVec2(renderScale))
}

func (pself *Sprite) UpdateTextureAtlas(path string, rect2 Rect2, renderScale float64, isUpdateTexture bool) {
	if path == "" {
		return
	}
	resPath := ToAssetPath(path)
	pself.PicPath = resPath
	if isUpdateTexture {
		pself.SetTextureAtlas(pself.PicPath, rect2)
	}
	pself.SetRenderScale(UniformVec2(renderScale))
}

func (pself *Sprite) OnTriggerEnter(target gdx.ISpriter) {
	sprite, ok := target.(*Sprite)
	if ok {
		triggerEventsTemp = append(triggerEventsTemp, TriggerEvent{Src: pself, Dst: sprite})
	}
}

func (pself *Sprite) RegisterOnAnimationLooped(f func()) {
	pself.Sprite.OnAnimationLoopedEvent.Subscribe(f)
}

func (pself *Sprite) UnRegisterOnAnimationLooped() {
	pself.Sprite.OnAnimationLoopedEvent.UnsubscribeAll()
}

func (pself *Sprite) RegisterOnAnimationFinished(f func()) {
	pself.Sprite.OnAnimationFinishedEvent.Subscribe(f)
}

func (pself *Sprite) UnRegisterOnAnimationFinished() {
	pself.Sprite.OnAnimationFinishedEvent.UnsubscribeAll()
}

// --------------------------------------------------------------------------
// Collider convenience methods. Coordinate conversion belongs to the Godot
// module boundary; these methods keep all values in SPX coordinates.

func (pself *Sprite) SetColliderShapeRect(isTrigger bool, center Vec2, size Vec2) {
	if isTrigger {
		pself.Sprite.SetTriggerRect(center, size)
	} else {
		pself.Sprite.SetColliderRect(center, size)
	}
}

func (pself *Sprite) SetColliderShapeCircle(isTrigger bool, center Vec2, radius float64) {
	if isTrigger {
		pself.Sprite.SetTriggerCircle(center, radius)
	} else {
		pself.Sprite.SetColliderCircle(center, radius)
	}
}

func (pself *Sprite) SetColliderShapeCapsule(isTrigger bool, center Vec2, size Vec2) {
	if isTrigger {
		pself.Sprite.SetTriggerCapsule(center, size)
	} else {
		pself.Sprite.SetColliderCapsule(center, size)
	}
}

func (pself *Sprite) SetColliderShapePolygon(isTrigger bool, center Vec2, points []float64) {
	points32 := F64Tof32(points)
	if isTrigger {
		pself.Sprite.SetTriggerPolygon(center, points32)
	} else {
		pself.Sprite.SetColliderPolygon(center, points32)
	}
}

func (pself *Sprite) SetColliderEnabled(isTrigger bool, enabled bool) {
	if isTrigger {
		pself.Sprite.SetTriggerEnabled(enabled)
	} else {
		pself.Sprite.SetCollisionEnabled(enabled)
	}
}
