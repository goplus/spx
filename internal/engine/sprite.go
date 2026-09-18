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

// Sprite methods must run on the main thread.
type Sprite struct {
	gdx.Sprite
	Name    string
	PicPath string
	Target  any
}

func (s *Sprite) UpdateTexture(path string, renderScale float64, updateTexture bool) {
	if path == "" {
		return
	}
	s.PicPath = ToAssetPath(path)
	if updateTexture {
		s.SetTexture(s.PicPath)
	}
	s.SetRenderScale(UniformVec2(renderScale))
}

func (s *Sprite) UpdateTextureAtlas(path string, rect Rect2, renderScale float64, updateTexture bool) {
	if path == "" {
		return
	}
	s.PicPath = ToAssetPath(path)
	if updateTexture {
		s.SetTextureAtlas(s.PicPath, rect)
	}
	s.SetRenderScale(UniformVec2(renderScale))
}

func (s *Sprite) OnTriggerEnter(target gdx.ISpriter) {
	sprite, ok := target.(*Sprite)
	if ok {
		enqueueTriggerEvent(s, sprite)
	}
}

func (s *Sprite) RegisterOnAnimationLooped(fn func()) {
	s.Sprite.OnAnimationLoopedEvent.Subscribe(fn)
}

func (s *Sprite) UnRegisterOnAnimationLooped() {
	s.Sprite.OnAnimationLoopedEvent.UnsubscribeAll()
}

func (s *Sprite) RegisterOnAnimationFinished(fn func()) {
	s.Sprite.OnAnimationFinishedEvent.Subscribe(fn)
}

func (s *Sprite) UnRegisterOnAnimationFinished() {
	s.Sprite.OnAnimationFinishedEvent.UnsubscribeAll()
}

// Collider values use SPX coordinates.

func (s *Sprite) SetColliderShapeRect(trigger bool, center Vec2, size Vec2) {
	if trigger {
		s.Sprite.SetTriggerRect(center, size)
	} else {
		s.Sprite.SetColliderRect(center, size)
	}
}

func (s *Sprite) SetColliderShapeCircle(trigger bool, center Vec2, radius float64) {
	if trigger {
		s.Sprite.SetTriggerCircle(center, radius)
	} else {
		s.Sprite.SetColliderCircle(center, radius)
	}
}

func (s *Sprite) SetColliderShapeCapsule(trigger bool, center Vec2, size Vec2) {
	if trigger {
		s.Sprite.SetTriggerCapsule(center, size)
	} else {
		s.Sprite.SetColliderCapsule(center, size)
	}
}

func (s *Sprite) SetColliderShapePolygon(trigger bool, center Vec2, points []float64) {
	points32 := F64Tof32(points)
	if trigger {
		s.Sprite.SetTriggerPolygon(center, points32)
	} else {
		s.Sprite.SetColliderPolygon(center, points32)
	}
}

func (s *Sprite) SetColliderEnabled(trigger bool, enabled bool) {
	if trigger {
		s.Sprite.SetTriggerEnabled(enabled)
	} else {
		s.Sprite.SetCollisionEnabled(enabled)
	}
}
