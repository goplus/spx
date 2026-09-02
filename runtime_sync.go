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
	"sync/atomic"

	"github.com/goplus/spbase/mathf"
	coreruntime "github.com/goplus/spx/v3/internal/core/runtime"
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

// dispatchStartEventIfNeeded fires the start event once after bootstrap completes.
func (p *Game) dispatchStartEventIfNeeded() {
	if ev := p.scheduleStartEvent(); ev != nil {
		p.handleEvent(ev)
	}
}

// updateSpriteProxies activates pending shapes, batches dirty sprite proxy changes,
// processes pending destroys, flushes the batch to the engine, and updates camera state.
func (p *Game) updateSpriteProxies() {
	p.camera.onUpdate()
	activeShapes := p.shapeMgr.getTempShapes()
	p.shapeMgr.flushActivate(activeShapes)
	p.flushSpriteProxyChanges(activeShapes)
}

// syncPostCoroutineVisuals flushes visual changes without advancing frame logic.
func (p *Game) syncPostCoroutineVisuals() {
	p.camera.onUpdate()
	p.flushSpriteProxyChanges(p.shapeMgr.getTempShapes())
}

func (p *Game) flushSpriteProxyChanges(activeShapes []Shape) {
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

// -----------------------------------------------------------------------------
// Base Object Visual Sync
// -----------------------------------------------------------------------------
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

// -----------------------------------------------------------------------------
// Sprite Proxy Lifecycle
// -----------------------------------------------------------------------------
const (
	cloneProxyPublished uint32 = iota
	cloneProxyPending
	cloneProxyReady
)

func (p *SpriteImpl) beginCloneProxyPublication() {
	state := new(uint32)
	atomic.StoreUint32(state, cloneProxyPending)
	p.proxyPublication = state
}

func (p *SpriteImpl) cloneProxyPublicationState() uint32 {
	if p.proxyPublication == nil {
		return cloneProxyPublished
	}
	return atomic.LoadUint32(p.proxyPublication)
}

func (p *SpriteImpl) isCloneProxyPublicationBlocked() bool {
	return p.cloneProxyPublicationState() != cloneProxyPublished
}

func (p *SpriteImpl) isCloneProxyPublicationReady() bool {
	return p.cloneProxyPublicationState() == cloneProxyReady
}

func (p *SpriteImpl) initRuntimeProxy() {
	p.rebuildRuntimeProxy(true)
}

func (p *SpriteImpl) awake() {
	p.animation().playDefaultAnimIfIdle()
	p.spriteState.IsAwakened = true
}

func (p *SpriteImpl) rebuildRuntimeProxy(applyCostume bool) {
	p.runtimeState.SyncSprite = nil
	engine.WaitMainThread(func() {
		p.ensureProxyInitialized()
		if applyCostume {
			p.baseObj.applyCostumeUpdate()
		}
	})
}

// ensureProxyInitialized initializes the sprite's engine proxy if it hasn't been created yet.
func (p *SpriteImpl) ensureProxyInitialized() {
	if p.runtimeState.SyncSprite != nil || p.isDestroyed() {
		return
	}
	p.runtimeState.SyncSprite = engine.BridgeNewBareSprite(p, mathf.NewVec2(p.getXY()))
	p.applyPhysicsProxyConfig()
	p.runtimeState.SyncSprite.SetVisible(p.effectiveProxyVisibility())
	p.runtimeState.SyncSprite.Name = p.name
	p.runtimeState.SyncSprite.SetTypeName(p.name)
	p.applyGraphicEffects(true)
	p.animation().registerOnAnimationLooped(p.handleAnimationLooped)
	p.animation().registerOnAnimationFinished(p.handleAnimationFinished)
	p.markProxyDirty()
}

func (p *SpriteImpl) effectiveProxyVisibility() bool {
	return p.spriteState.IsVisible && !p.isCloneProxyPublicationBlocked()
}

// finishCloneInitialization makes the clone eligible for the next proxy batch.
// The batch applies costume and layer changes before it submits visibility, so
// no render can observe the inherited initialization state or stale peer layers.
func (p *SpriteImpl) finishCloneInitialization() {
	if p.isDestroyed() || p.proxyPublication == nil ||
		!atomic.CompareAndSwapUint32(p.proxyPublication, cloneProxyPending, cloneProxyReady) {
		return
	}
	p.g.shapeMgr.markCloneProxyPublicationReady()
}

// handleAnimationFinished records completed animation events from the proxy.
func (p *SpriteImpl) handleAnimationFinished() {
	engine.Lock()
	defer engine.Unlock()
	if p.isDestroyed() || p.runtimeState.SyncSprite == nil {
		return
	}
	state := p.animation().getCurAnimState()
	if state != nil && state.Name != "" {
		p.animation().addDonedAnimation(state.Name)
	}
}

// handleAnimationLooped records audio work for animation loop boundaries.
func (p *SpriteImpl) handleAnimationLooped() {
	engine.Lock()
	defer engine.Unlock()
	if p.isDestroyed() || p.runtimeState.SyncSprite == nil {
		return
	}
	p.queueAnimationLoopAudio(p.animation().getCurAnimState())
	p.queueAnimationLoopAudio(p.animation().getCurTweenState())
}

// -----------------------------------------------------------------------------
// Sprite Sync Helpers
// -----------------------------------------------------------------------------
func (p *SpriteImpl) applyPhysicsProxyConfig() {
	if p.runtimeState.SyncSprite == nil {
		return
	}
	p.physics().applyPhysicsProxyConfig(p.runtimeState.SyncSprite)
}

func (p *SpriteImpl) shouldPullPhysicsPosition() bool {
	return p.runtimeState.SyncSprite != nil && p.PhysicsMode() != NoPhysics
}

func (p *SpriteImpl) applyPhysicsPosition(x, y float64) {
	p.transform().setXY(x, y)
}

func (p *SpriteImpl) ensureProxyQueryStateSynced() {
	if p.isDestroyed() || p.runtimeState.SyncSprite == nil {
		return
	}
	// Query paths need the latest silhouette even when the sprite is hidden.
	p.baseObj.applyCostumeUpdate()
	p.syncAutoPhysicsShapesAfterCostumeChange()
	if !p.spriteState.IsDirty {
		return
	}
	if p.spriteState.ProxySyncVersion == p.spriteState.DirtyVersion {
		return
	}

	x, y := p.getXY()
	renderOffsetX, renderOffsetY := getRenderOffset(p)
	rot, scaleX, scaleY := getRenderRotationAndScale(p)

	p.runtimeState.SyncSprite.SetTransform(
		mathf.NewVec2(x, y),
		engine.DegToRad(rot),
		mathf.NewVec2(scaleX, scaleY),
		p.effectiveProxyVisibility(),
		mathf.NewVec2(renderOffsetX, renderOffsetY),
	)
	p.spriteState.ProxySyncVersion = p.spriteState.DirtyVersion
}

func (p *SpriteImpl) collectProxyUpdate(buffer *engine.SpriteSyncBuffer) {
	if p.isDestroyed() || p.runtimeState.SyncSprite == nil {
		return
	}
	publishingClone := p.cloneProxyPublicationState() == cloneProxyReady
	if p.spriteState.IsVisible || publishingClone {
		p.baseObj.applyCostumeUpdate()
	}
	p.syncAutoPhysicsShapesAfterCostumeChange()
	if publishingClone {
		atomic.CompareAndSwapUint32(p.proxyPublication, cloneProxyReady, cloneProxyPublished)
		// Query synchronization may already have written the latest logical
		// transform while keeping it hidden. Force a new batch entry after the
		// gate opens so visibility cannot be skipped by version coalescing.
		p.markProxyDirty()
	}
	if !p.spriteState.IsDirty {
		return
	}
	if p.spriteState.ProxySyncVersion != p.spriteState.DirtyVersion {
		p.appendTransformUpdate(buffer)
	}
	p.spriteState.IsDirty = false
}

func (p *SpriteImpl) appendTransformUpdate(buffer *engine.SpriteSyncBuffer) {
	x, y := p.getXY()
	renderOffsetX, renderOffsetY := getRenderOffset(p)
	rot, scaleX, scaleY := getRenderRotationAndScale(p)
	buffer.Add(
		int64(p.runtimeState.SyncSprite.Id),
		x, y,
		engine.DegToRad(rot),
		scaleX, scaleY,
		renderOffsetX, renderOffsetY,
		p.effectiveProxyVisibility(),
	)
	p.spriteState.ProxySyncVersion = p.spriteState.DirtyVersion
}
