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

package spx

import "github.com/goplus/spx/v2/internal/engine"

func (sprite *SpriteImpl) shouldSyncPhysicsPosition() bool {
	return sprite.SyncSprite != nil && sprite.PhysicsMode() != NoPhysics
}

func (sprite *SpriteImpl) syncPhysicsPosition(x, y float64) {
	revertRenderOffset(sprite, &x, &y)
	sprite.transform().setXY(x, y)
}

func (sprite *SpriteImpl) syncProxyState(buffer *engine.SpriteSyncBuffer) {
	if sprite.HasDestroyed || sprite.SyncSprite == nil {
		return
	}
	if sprite.IsVisible {
		syncCheckUpdateCostume(&sprite.baseObj)
	}
	if !sprite.IsDirty {
		return
	}
	sprite.appendSyncTransform(buffer)
	sprite.IsDirty = false
}

func (sprite *SpriteImpl) appendSyncTransform(buffer *engine.SpriteSyncBuffer) {
	x, y := sprite.getXY()
	offsetX, offsetY := getRenderOffset(sprite)
	rot, scaleX, scaleY := getRenderRotationAndScale(sprite)
	buffer.Add(
		int64(sprite.SyncSprite.Id),
		x+offsetX, y+offsetY,
		engine.DegToRad(rot),
		scaleX, scaleY,
		offsetX, offsetY,
		sprite.IsVisible,
	)
}
