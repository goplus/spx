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
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

func (p *SpriteImpl) initEngineObjects() {
	p.resetRuntimeProxy(false)
}

func (p *SpriteImpl) awake() {
	p.animation().playDefaultAnimIfIdle()
}

func (p *SpriteImpl) resetRuntimeProxy(applyCostume bool) {
	p.runtimeState.SyncSprite = nil
	engine.WaitMainThread(func() {
		p.ensureProxyInitialized()
		if applyCostume {
			p.baseObj.applyCostumeUpdate()
		}
	})
}

// dispatchStartEventIfNeeded fires the start event once after bootstrap completes.
func (p *Game) dispatchStartEventIfNeeded() {
	coreruntime.SyncOnce(&p.lifecycleState.StartFlag, func() {
		p.fireEvent(&eventStart{})
	})
}

// updateSpriteProxies activates pending shapes, batches dirty sprite proxy changes,
// processes pending destroys, flushes the batch to the engine, and updates camera state.
func (p *Game) updateSpriteProxies() {
	p.camera.onUpdate()
	activeShapes := p.shapeMgr.getTempShapes()
	p.shapeMgr.flushActivate(activeShapes)
	p.syncBuffer.Clear()
	p.shapeMgr.collectProxyUpdates(activeShapes, p.syncBuffer)
	p.shapeMgr.flushDestroy(p.syncBuffer)
	p.flushSyncBuffer()
	p.camera.setDirtyFlag(false)
}

// flushSyncBuffer sends batched updates to the engine if there are any changes.
func (p *Game) flushSyncBuffer() {
	coreruntime.FlushSerializedBuffer(
		p.syncBuffer.UpdateCount(),
		p.syncBuffer.DeleteCount(),
		p.syncBuffer.Serialize,
		engine.SyncBatchUpdateSprites,
	)
}

// pullPhysicsPositions retrieves sprite positions from the physics engine in batch.
func (p *Game) pullPhysicsPositions() {
	coreruntime.SyncBatchPositions(
		p.getTempShapes(),
		func(item Shape) bool {
			sprite, ok := item.(*SpriteImpl)
			return ok && sprite.shouldPullPhysicsPosition()
		},
		func(item Shape) int64 {
			return int64(item.(*SpriteImpl).runtimeState.SyncSprite.Id)
		},
		engine.SyncBatchGetPositions,
		func(item Shape, x, y float64) {
			item.(*SpriteImpl).applyPhysicsPosition(x, y)
		},
	)
}

// processPhysicsTriggers consumes trigger events and fires collision callbacks.
func (p *Game) processPhysicsTriggers() {
	p.triggerEvents = engine.GetTriggerEvents(p.triggerEvents[:0])
	coreruntime.ProcessTriggerPairs(
		p.triggerEvents,
		func(target any) (*SpriteImpl, bool) {
			sprite, ok := target.(*SpriteImpl)
			return sprite, ok
		},
		isSpriteTouchable,
		func(srcSprite, dstSprite *SpriteImpl) {
			srcSprite.spriteState.HasOnTouchStart = true
			srcSprite.fireTouchStart(dstSprite)
		},
		func() {
			spxlog.Info("Physics error: unexpected trigger pair - invalid sprite types")
		},
	)
	clear(p.triggerEvents)
	p.triggerEvents = p.triggerEvents[:0]
}

func isSpriteTouchable(sprite *SpriteImpl) bool {
	return sprite.spriteState.IsVisible && !sprite.spriteState.IsDying
}

// scheduleCostumeUpdate schedules a costume update on the main thread.
func (p *baseObj) scheduleCostumeUpdate() {
	engine.WaitMainThread(func() {
		p.applyCostumeUpdate()
	})
}

// applyCostumeUpdate pushes pending costume and layer changes to the proxy.
func (p *baseObj) applyCostumeUpdate() {
	syncSprite := p.runtimeState.SyncSprite
	if p.runtimeState.IsLayerDirty {
		if !engine.HasLayerSortMethod() {
			syncSprite.SetZIndex(int64(p.runtimeState.Layer))
		}
		p.runtimeState.IsLayerDirty = false
	}
	if !p.runtimeState.IsCostumeDirty {
		return
	}
	p.runtimeState.IsCostumeDirty = false
	path := p.getCostumePath()
	renderScale := p.getCostumeRenderScale()
	if p.isCostumeAtlas() {
		rect := p.getCostumeAtlasRegion()
		syncSprite.UpdateTextureAtlas(path, rect, renderScale, !p.runtimeState.IsAnimating)
		p.applyAtlasUVRemap()
		return
	}
	syncSprite.UpdateTexture(path, renderScale, !p.runtimeState.IsAnimating)
}

func (p *baseObj) applyAtlasUVRemap() {
	uvRemap := p.getCostumeAtlasUvRemap()
	val := mathf.NewVec4(uvRemap.Position.X, uvRemap.Position.Y, uvRemap.Size.X, uvRemap.Size.Y)
	p.setMaterialParamsVec4("atlas_uv_rect2", val, true)
}

// ensureProxyInitialized initializes the sprite's engine proxy if it hasn't been created yet.
func (sprite *SpriteImpl) ensureProxyInitialized() {
	if sprite.runtimeState.SyncSprite != nil || sprite.isDestroyed() {
		return
	}
	sprite.runtimeState.SyncSprite = engine.BridgeNewBareSprite(sprite, mathf.NewVec2(sprite.getXYWithRenderOffset()))
	sprite.applyPhysicsProxyConfig()
	sprite.runtimeState.SyncSprite.SetVisible(sprite.spriteState.IsVisible)
	sprite.runtimeState.SyncSprite.Name = sprite.name
	sprite.runtimeState.SyncSprite.SetTypeName(sprite.name)
	sprite.applyGraphicEffects(true)
	sprite.animation().registerOnAnimationLooped(sprite.handleAnimationLooped)
	sprite.animation().registerOnAnimationFinished(sprite.handleAnimationFinished)
	sprite.spriteState.IsDirty = true
}

// handleAnimationFinished records completed animation events from the proxy.
func (sprite *SpriteImpl) handleAnimationFinished() {
	engine.Lock()
	defer engine.Unlock()
	if sprite.isDestroyed() || sprite.runtimeState.SyncSprite == nil {
		return
	}
	state := sprite.animation().getCurAnimState()
	if state != nil && state.Name != "" {
		sprite.animation().addDonedAnimation(state.Name)
	}
}

// handleAnimationLooped records looped animation audio playback requests.
func (sprite *SpriteImpl) handleAnimationLooped() {
	engine.Lock()
	defer engine.Unlock()
	if sprite.isDestroyed() || sprite.runtimeState.SyncSprite == nil {
		return
	}
	state := sprite.animation().getCurTweenState()
	if state != nil && state.LoopReplayAudioName != "" {
		sprite.sound().addPendingAudio(state.LoopReplayAudioName)
	}
}

func (sprite *SpriteImpl) applyPhysicsProxyConfig() {
	if sprite.runtimeState.SyncSprite == nil {
		return
	}
	sprite.physics().applyPhysicsProxyConfig(sprite.runtimeState.SyncSprite)
}

func (sprite *SpriteImpl) shouldPullPhysicsPosition() bool {
	return sprite.runtimeState.SyncSprite != nil && sprite.PhysicsMode() != NoPhysics
}

func (sprite *SpriteImpl) applyPhysicsPosition(x, y float64) {
	revertRenderOffset(sprite, &x, &y)
	sprite.transform().setXY(x, y)
}

func (sprite *SpriteImpl) collectProxyUpdate(buffer *engine.SpriteSyncBuffer) {
	if sprite.isDestroyed() || sprite.runtimeState.SyncSprite == nil {
		return
	}
	if sprite.spriteState.IsVisible {
		sprite.baseObj.applyCostumeUpdate()
	}
	if !sprite.spriteState.IsDirty {
		return
	}
	sprite.appendTransformUpdate(buffer)
	sprite.spriteState.IsDirty = false
}

func (sprite *SpriteImpl) appendTransformUpdate(buffer *engine.SpriteSyncBuffer) {
	x, y := sprite.getXY()
	offsetX, offsetY := getRenderOffset(sprite)
	rot, scaleX, scaleY := getRenderRotationAndScale(sprite)
	buffer.Add(
		int64(sprite.runtimeState.SyncSprite.Id),
		x+offsetX, y+offsetY,
		engine.DegToRad(rot),
		scaleX, scaleY,
		offsetX, offsetY,
		sprite.spriteState.IsVisible,
	)
}
