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
	return sprite.spriteState.IsVisible && !sprite.spriteState.IsDying &&
		!sprite.spriteState.IsProxyPublicationPending
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
	p.applyLayerUpdate()
	if !p.runtimeState.IsCostumeDirty {
		return
	}
	syncSprite := p.runtimeState.SyncSprite
	if syncSprite == nil {
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

func (p *baseObj) applyLayerUpdate() {
	if !p.runtimeState.IsLayerDirty || p.runtimeState.SyncSprite == nil {
		return
	}
	if !engine.HasLayerSortMethod() {
		p.runtimeState.SyncSprite.SetZIndex(int64(p.runtimeState.Layer))
	}
	p.runtimeState.IsLayerDirty = false
}

func (p *baseObj) applyAtlasUVRemap() {
	uvRemap := p.getCostumeAtlasUvRemap()
	val := mathf.NewVec4(uvRemap.Position.X, uvRemap.Position.Y, uvRemap.Size.X, uvRemap.Size.Y)
	p.setMaterialParamsVec4("atlas_uv_rect2", val, true)
}

// -----------------------------------------------------------------------------
// Sprite Proxy Lifecycle
// -----------------------------------------------------------------------------
func (p *SpriteImpl) initRuntimeProxy() {
	p.spriteState.IsProxyPublicationPending = false
	p.rebuildRuntimeProxy(true)
}

// initCloneRuntimeProxy creates a fully configured proxy without exposing it.
// Scratch runs a clone's initialization hat before the clone can be rendered.
func (p *SpriteImpl) initCloneRuntimeProxy() {
	p.spriteState.IsProxyPublicationPending = true
	p.physics().beginPendingProxyPhysics()
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
	return !p.spriteState.IsProxyPublicationPending && p.spriteState.IsVisible
}

// publishCloneRuntimeProxy applies initialization changes made by onCloned and
// publishes the clone only if its creation transaction commits. Main-thread
// callback admission is atomic with lifecycle cancellation, so rollback cannot
// run concurrently with this publication barrier.
func (p *SpriteImpl) publishCloneRuntimeProxy(creation *cloneCreation) bool {
	published := false
	engine.WaitMainThread(func() {
		// WaitMainThread callback admission is the cancellation linearization
		// point. Epochs were validated before enqueueing; after admission only
		// structural invalidation may reject the transaction. A later lifecycle
		// cancellation waits for this callback and loses to commit.
		if !creation.canCommit() || p.g.shapeMgr.findShapeIndex(p) < 0 ||
			p.runtimeState.SyncSprite == nil || !p.spriteState.IsProxyPublicationPending {
			return
		}

		// Finish all clone-local, fallible bridge work while the clone is hidden.
		// This includes its final costume, physics shape, and transform.
		p.ensureProxyQueryStateSynced()

		layersStaged := false
		physicsStaged := false
		defer func() {
			failure := recover()
			if !published && (layersStaged || physicsStaged) {
				// Restore the published layer topology before rollback removes the
				// pending clone. Each bridge action is isolated so one cleanup panic
				// cannot prevent the remaining compensation; preserve the original
				// panic when publication itself failed.
				if layersStaged {
					p.g.shapeMgr.updateRenderLayers()
				}
				physicsModeFailure := captureProxyCallPanic(func() {
					p.runtimeState.SyncSprite.SetPhysicsMode(NoPhysics)
				})
				collisionFailure := captureProxyCallPanic(func() {
					p.runtimeState.SyncSprite.SetCollisionEnabled(false)
				})
				triggerFailure := captureProxyCallPanic(func() {
					p.runtimeState.SyncSprite.SetTriggerEnabled(false)
				})
				hideFailure := captureProxyCallPanic(func() {
					p.syncProxyTransform(false)
				})
				var layerFailure any
				if layersStaged {
					layerFailure = captureProxyCallPanic(func() {
						p.g.shapeMgr.syncDirtySpriteLayers()
					})
				}
				if failure == nil {
					for _, cleanupFailure := range []any{
						physicsModeFailure,
						collisionFailure,
						triggerFailure,
						hideFailure,
						layerFailure,
					} {
						if cleanupFailure != nil {
							failure = cleanupFailure
							break
						}
					}
				}
			}
			if failure != nil {
				panic(failure)
			}
		}()

		// Only layer staging can mutate already-published peers. Keep it at the
		// commit tail and compensate it on every pre-commit exit or panic.
		layersStaged = true
		p.g.shapeMgr.updateRenderLayersIncludingPending(p)
		p.g.shapeMgr.syncDirtySpriteLayers()

		// Native physics and visibility form the final publication tail. No
		// physics step can interleave main-thread bridge calls, so a proxy is
		// either fully isolated or fully configured when the transaction CAS
		// makes it observable to the runtime.
		physicsStaged = true
		p.physics().publishPendingProxyPhysics(p.runtimeState.SyncSprite)
		p.syncProxyTransform(p.spriteState.IsVisible)
		if !creation.tryPublish() {
			return
		}
		p.spriteState.IsProxyPublicationPending = false
		published = true
	})
	return published
}

func captureProxyCallPanic(call func()) (recovered any) {
	defer func() {
		recovered = recover()
	}()
	call()
	return nil
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
	return p.runtimeState.SyncSprite != nil &&
		!p.spriteState.IsProxyPublicationPending && p.PhysicsMode() != NoPhysics
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

	p.syncProxyTransform(p.effectiveProxyVisibility())
}

func (p *SpriteImpl) syncProxyTransform(visible bool) {
	x, y := p.getXY()
	renderOffsetX, renderOffsetY := getRenderOffset(p)
	rot, scaleX, scaleY := getRenderRotationAndScale(p)
	p.runtimeState.SyncSprite.SetTransform(
		mathf.NewVec2(x, y),
		engine.DegToRad(rot),
		mathf.NewVec2(scaleX, scaleY),
		visible,
		mathf.NewVec2(renderOffsetX, renderOffsetY),
	)
	p.spriteState.ProxySyncVersion = p.spriteState.DirtyVersion
}

func (p *SpriteImpl) collectProxyUpdate(buffer *engine.SpriteSyncBuffer) {
	if p.isDestroyed() || p.runtimeState.SyncSprite == nil {
		return
	}
	if p.spriteState.IsVisible {
		p.baseObj.applyCostumeUpdate()
	}
	p.syncAutoPhysicsShapesAfterCostumeChange()
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
