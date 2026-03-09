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

import (
	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
)

// syncCheckInitProxy initializes the sprite's engine proxy if it hasn't been created yet.
func (sprite *SpriteImpl) syncCheckInitProxy() {
	if sprite.SyncSprite != nil || sprite.HasDestroyed {
		return
	}
	sprite.SyncSprite = engine.MainThreadNewSprite(sprite, mathf.NewVec2(sprite.getXYWithRenderOffset()))
	syncInitSpritePhysicInfo(sprite, sprite.SyncSprite)
	sprite.SyncSprite.SetVisible(sprite.IsVisible)
	sprite.SyncSprite.Name = sprite.name
	sprite.SyncSprite.SetTypeName(sprite.name)
	sprite.applyGraphicEffects(true)
	sprite.animation().registerOnAnimationLooped(sprite.syncOnAnimationLooped)
	sprite.animation().registerOnAnimationFinished(sprite.syncOnAnimationFinished)
	sprite.IsDirty = true
}

// syncOnAnimationFinished is called when an animation finishes.
func (sprite *SpriteImpl) syncOnAnimationFinished() {
	engine.Lock()
	defer engine.Unlock()
	state := sprite.animation().getCurAnimState()
	if state != nil && state.Name != "" && sprite.SyncSprite != nil {
		sprite.animation().addDonedAnimation(sprite.SyncSprite.GetCurrentAnimName())
	}
}

// syncOnAnimationLooped is called when an animation loops.
func (sprite *SpriteImpl) syncOnAnimationLooped() {
	engine.Lock()
	defer engine.Unlock()
	state := sprite.animation().getCurTweenState()
	if state != nil && state.AudioName != "" {
		sprite.sound().addPendingAudio(state.AudioName)
	}
}

func syncInitSpritePhysicInfo(sprite *SpriteImpl, syncProxy *engine.Sprite) {
	sprite.physics().syncInitPhysicInfo(syncProxy)
}
