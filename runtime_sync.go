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
	"math"
	"sync/atomic"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/base/sliceutil"
	"github.com/goplus/spx/v4/internal/engine"
	spxlog "github.com/goplus/spx/v4/internal/log"
)

const (
	cloneProxyPublished uint32 = iota
	cloneProxyPending
	cloneProxyReady
)

type cloneProxyPublication struct {
	state atomic.Uint32
	// Engine-thread lease count; atomic for proxy sync readers.
	sensingVisibilityLeases atomic.Int32
}

type physicsPullBuffer struct {
	ids     []int64
	sprites []*SpriteImpl
	busy    atomic.Bool
}

type proxyChange struct {
	sprite           *SpriteImpl
	proxy            *engine.Sprite
	dirtyVersion     uint64
	positionVersions positionVersions
	destroy          bool
}

type proxyBatch struct {
	changes      []proxyChange
	destroyCount int
}

type proxySnapshot struct {
	proxyVisual
	change   proxyChange
	position mathf.Vec2
	visible  bool
}

type proxyVisual struct {
	scale, renderOffset mathf.Vec2
	rotation            float64
}

// Keep the geometry paired with the texture actually sent to this proxy.
type preparedProxyVisual struct {
	proxyVisual
	proxy        *engine.Sprite
	dirtyVersion uint64
	preparing    bool
}

// -----------------------------------------------------------------------------
// Engine Callbacks
// -----------------------------------------------------------------------------
// mirrorPosition applies engine position writes without movement side effects.
func (p *SpriteImpl) mirrorPosition(proxy *engine.Sprite, position mathf.Vec2, axes engine.PositionAxes) {
	if p.isDestroyed() || p.runtimeState.syncSprite != proxy {
		return
	}
	pending := &p.transform().pendingPosition
	if axes&engine.PositionX != 0 {
		pending.x = 0
	}
	if axes&engine.PositionY != 0 {
		pending.y = 0
	}
	p.mergeEnginePosition(position.X, position.Y)
}

func (p *SpriteImpl) applyCostumeUpdate() {
	// Texture setters emit synchronous callbacks. Nested queries must not
	// interleave another texture write with this preparation's scale and UVs.
	if p.preparedVisual.proxy == p.runtimeState.syncSprite && p.preparedVisual.preparing {
		return
	}
	p.applyLayerUpdate()
	// Hidden transforms can be acknowledged while their resources stay dirty.
	// Preparing those resources must submit their matching geometry again.
	if p.runtimeState.isCostumeDirty && p.preparedVisual.proxy == p.runtimeState.syncSprite &&
		p.spriteState.proxySyncVersion == p.spriteState.dirtyVersion {
		p.markProxyDirty()
	}
	if !p.runtimeState.isCostumeDirty && !p.hasCostume() {
		return // Clean proxies without a costume use native collision fallback.
	}
	// Refresh geometry even when the texture is clean: later callbacks may
	// change the costume after a heading-only update has already committed.
	p.preparedVisual = p.captureProxyVisual()
	if !p.runtimeState.isCostumeDirty {
		return
	}
	p.preparedVisual.preparing = true
	defer func() { p.preparedVisual.preparing = false }()
	p.applyCostumeTexture()
}

// -----------------------------------------------------------------------------
// Sprite Proxy Lifecycle
// -----------------------------------------------------------------------------
func (p *SpriteImpl) beginCloneProxyPublication() {
	publication := &cloneProxyPublication{}
	publication.state.Store(cloneProxyPending)
	p.proxyPublication = publication
}

func (p *SpriteImpl) cloneProxyPublicationState() uint32 {
	if p.proxyPublication == nil {
		return cloneProxyPublished
	}
	return p.proxyPublication.state.Load()
}

func (p *SpriteImpl) isCloneProxyPublicationBlocked() bool {
	return p.cloneProxyPublicationState() != cloneProxyPublished
}

func (p *SpriteImpl) initRuntimeProxy() {
	p.runtimeState.syncSprite = nil
	engine.WaitMainThread(func() {
		p.ensureProxyInitialized()
		p.applyCostumeUpdate()
	})
}

func (p *SpriteImpl) awake() {
	p.animation().playDefaultAnimIfIdle()
}

func (p *SpriteImpl) ensureProxyInitialized() {
	if p.runtimeState.syncSprite != nil || p.isDestroyed() {
		return
	}
	p.runtimeState.syncSprite = engine.BridgeNewBareSprite(p, mathf.NewVec2(p.getXY()))
	p.runtimeState.syncSprite.CallbackAlive = func() bool { return !p.isDestroyed() }
	p.runtimeState.syncSprite.MirrorPosition = p.mirrorPosition
	p.applyPhysicsProxyConfig()
	p.runtimeState.syncSprite.SetVisible(p.effectiveProxyVisibility())
	p.runtimeState.syncSprite.SetTypeName(p.name)
	p.applyGraphicEffects()
	p.runtimeState.syncSprite.RegisterOnAnimationLooped(p.handleAnimationLooped)
	p.runtimeState.syncSprite.RegisterOnAnimationFinished(p.handleAnimationFinished)
	p.markProxyDirty()
}

func (p *SpriteImpl) effectiveProxyVisibility() bool {
	if !p.spriteState.isVisible {
		return false
	}
	publication := p.proxyPublication
	if publication == nil || publication.state.Load() == cloneProxyPublished {
		return true
	}
	return publication.sensingVisibilityLeases.Load() > 0
}

// finishCloneInitialization queues visibility for the next proxy batch.
func (p *SpriteImpl) finishCloneInitialization() {
	if p.isDestroyed() || p.proxyPublication == nil {
		return
	}
	p.proxyPublication.state.CompareAndSwap(cloneProxyPending, cloneProxyReady)
}

func (p *SpriteImpl) handleAnimationFinished() {
	engine.Lock()
	defer engine.Unlock()
	if p.isDestroyed() || p.runtimeState.syncSprite == nil {
		return
	}
	state := p.animation().curAnimState
	if state != nil && state.Name != "" {
		p.animation().addDoneAnimation(state)
	}
}

func (p *SpriteImpl) handleAnimationLooped() {
	engine.Lock()
	defer engine.Unlock()
	if p.isDestroyed() || p.runtimeState.syncSprite == nil {
		return
	}
	p.queueAnimationLoopAudio(p.animation().curAnimState)
	p.queueAnimationLoopAudio(p.animation().getCurTweenState())
}

// -----------------------------------------------------------------------------
// Sprite Sync
// -----------------------------------------------------------------------------
func (p *SpriteImpl) applyPhysicsProxyConfig() {
	if p.runtimeState.syncSprite == nil {
		return
	}
	p.physics().applyPhysicsProxyConfig(p.runtimeState.syncSprite)
}

func (p *SpriteImpl) shouldPullPhysicsPosition() bool {
	return p.runtimeState.syncSprite != nil && p.PhysicsMode() != NoPhysics
}

// mergeEnginePosition preserves pending Go writes per axis while merging engine coordinates.
func (p *SpriteImpl) mergeEnginePosition(x, y float64) {
	transform := p.transform()
	if transform.pendingPosition.x != 0 {
		x = transform.x
	}
	if transform.pendingPosition.y != 0 {
		y = transform.y
	}
	if transform.setPositionRaw(x, y) {
		p.markVisualDirty()
	}
}

func (p *SpriteImpl) syncProxyForQuery() {
	engine.WaitMainThread(func() {
		if p.isDestroyed() || p.runtimeState.syncSprite == nil {
			return
		}
		// Queries need the latest silhouette even when the sprite is hidden.
		p.applyCostumeUpdate()
		p.syncAutoPhysicsShapesAfterCostumeChange()
		if !p.needsProxyUpdate() {
			return
		}
		// Read and submit in one engine job; queued queries may precede the frame pull.
		if p.shouldPullPhysicsPosition() {
			position := p.runtimeState.syncSprite.GetPosition()
			p.mergeEnginePosition(position.X, position.Y)
		}
		snapshot := p.snapshotProxy()
		proxy := snapshot.change.proxy
		engine.ValidateSpriteTransform(proxy.Id, snapshot.position, snapshot.rotation, snapshot.scale, snapshot.renderOffset)
		proxy.SetTransform(
			snapshot.position,
			snapshot.rotation,
			snapshot.scale,
			snapshot.visible,
			snapshot.renderOffset,
		)
		commitProxyChange(snapshot.change)
	})
}

func (p *SpriteImpl) prepareProxyUpdate() {
	if p.isDestroyed() || p.runtimeState.syncSprite == nil {
		return
	}
	publishingClone := p.cloneProxyPublicationState() == cloneProxyReady
	if p.spriteState.isVisible || publishingClone {
		p.applyCostumeUpdate()
	}
	p.syncAutoPhysicsShapesAfterCostumeChange()
	if publishingClone {
		p.proxyPublication.state.CompareAndSwap(cloneProxyReady, cloneProxyPublished)
		// Publish even if a query already synced the clone's hidden transform.
		p.markProxyDirty()
	}
}

func (p *SpriteImpl) collectProxyUpdate(buffer *engine.SpriteSyncBuffer) proxyChange {
	if !p.needsProxyUpdate() {
		return proxyChange{}
	}

	snapshot := p.snapshotProxy()
	buffer.Add(
		snapshot.change.proxy.Id,
		snapshot.position.X, snapshot.position.Y,
		snapshot.rotation,
		snapshot.scale.X, snapshot.scale.Y,
		snapshot.renderOffset.X, snapshot.renderOffset.Y,
		snapshot.visible,
	)
	return snapshot.change
}

func (p *SpriteImpl) needsProxyUpdate() bool {
	return !p.isDestroyed() && p.runtimeState.syncSprite != nil &&
		p.spriteState.proxySyncVersion != p.spriteState.dirtyVersion
}

func (p *SpriteImpl) snapshotProxy() proxySnapshot {
	prepared := &p.preparedVisual
	visual := *prepared
	resourcesReady := !p.runtimeState.isCostumeDirty && !prepared.preparing
	// Reentrant costume changes wait for their resources, while position and
	// visibility still include writes made by later sprites' callbacks.
	if prepared.proxy != p.runtimeState.syncSprite || resourcesReady {
		visual = p.captureProxyVisual()
		if resourcesReady {
			// Hidden heading updates also refresh the geometry that can later
			// be published with these same, already prepared resources.
			*prepared = visual
		}
	}
	dirtyVersion := visual.dirtyVersion
	if !p.spriteState.isVisible && !prepared.preparing {
		// Confirm the submitted transform, not deferred resource preparation.
		// IsCostumeDirty independently keeps the next resource update pending.
		dirtyVersion = p.spriteState.dirtyVersion
	}
	return proxySnapshot{
		proxyVisual: visual.proxyVisual,
		change: proxyChange{
			sprite: p, proxy: visual.proxy,
			dirtyVersion: dirtyVersion, positionVersions: p.transform().pendingPosition,
		},
		position: mathf.NewVec2(p.getXY()),
		visible:  p.effectiveProxyVisibility(),
	}
}

func (p *SpriteImpl) captureProxyVisual() preparedProxyVisual {
	offsetX, offsetY := getRenderOffset(p)
	rotation, scaleX, scaleY := getRenderRotationAndScale(p)
	return preparedProxyVisual{
		proxyVisual: proxyVisual{
			scale:        mathf.NewVec2(scaleX, scaleY),
			renderOffset: mathf.NewVec2(offsetX, offsetY),
			rotation:     engine.DegToRad(rotation),
		},
		proxy:        p.runtimeState.syncSprite,
		dirtyVersion: p.spriteState.dirtyVersion,
	}
}

// -----------------------------------------------------------------------------
// Frame Sync
// -----------------------------------------------------------------------------
func (p *Game) flushSpriteProxyChanges(activeShapes []Shape) {
	batch := proxyBatch{changes: p.shapeMgr.proxyChangeBuffer}
	p.shapeMgr.proxyChangeBuffer = nil
	defer func() {
		p.shapeMgr.proxyChangeBuffer = sliceutil.Reset(batch.changes, 0)
	}()
	p.syncBuffer.Clear()
	p.shapeMgr.collectProxyChanges(activeShapes, p.syncBuffer, &batch)
	p.flushSyncBuffer()
	p.shapeMgr.commitProxyChanges(batch)
	p.camera.setDirtyFlag(false)
}

func (p *Game) flushSyncBuffer() {
	engine.SyncBatchUpdateSprites(p.syncBuffer.Serialize())
}

func (p *Game) pullPhysicsPositions() {
	pull := &p.physicsPull
	buffer := p.syncBuffer
	reuse := pull.busy.CompareAndSwap(false, true)
	if !reuse {
		pull = &physicsPullBuffer{}
		buffer = &engine.SpriteSyncBuffer{}
	}
	pull.ids = pull.ids[:0]
	defer func() {
		pull.sprites = sliceutil.Reset(pull.sprites, 0)
		if reuse {
			pull.busy.Store(false)
		}
	}()
	for _, shape := range p.shapeMgr.items {
		sprite, ok := shape.(*SpriteImpl)
		if !ok || !sprite.shouldPullPhysicsPosition() {
			continue
		}
		pull.ids = append(pull.ids, sprite.runtimeState.syncSprite.Id)
		pull.sprites = append(pull.sprites, sprite)
	}
	if len(pull.ids) == 0 {
		return
	}
	applyPhysicsPositions(pull.sprites, buffer.GetPositions(pull.ids))
}

func (p *Game) processPhysicsTriggers() {
	p.triggerEvents = engine.GetTriggerEvents(p.triggerEvents)
	p.dispatchPhysicsTriggers()
}

// dispatchPhysicsTriggers keeps the batch intact if a touch dispatch panics.
func (p *Game) dispatchPhysicsTriggers() {
	for _, pair := range p.triggerEvents {
		if pair.Src == nil || pair.Dst == nil {
			spxlog.Info("Physics error: unexpected trigger pair - invalid sprite types")
			continue
		}
		src, ok1 := pair.Src.Target.(*SpriteImpl)
		dst, ok2 := pair.Dst.Target.(*SpriteImpl)
		if !ok1 || !ok2 {
			spxlog.Info("Physics error: unexpected trigger pair - invalid sprite types")
			continue
		}
		if !isSpriteTouchable(src) || !isSpriteTouchable(dst) {
			continue
		}
		src.fireTouchStart(dst)
	}
	p.triggerEvents = sliceutil.Reset(p.triggerEvents, 0)
}

func (p *baseObj) applyLayerUpdate() {
	applyDirtyUpdate(&p.runtimeState.isLayerDirty, func() {
		if !engine.HasLayerSortMethod() {
			p.runtimeState.syncSprite.SetZIndex(int64(p.runtimeState.layer))
		}
	})
}

func (p *baseObj) applyCostumeTexture() {
	applyDirtyUpdate(&p.runtimeState.isCostumeDirty, func() {
		syncSprite := p.runtimeState.syncSprite
		path := p.getCostumePath()
		renderScale := p.getCostumeRenderScale()
		if p.isCostumeAtlas() {
			rect := p.getCostumeAtlasRegion()
			uvRemap := p.currentCostume().frame.AtlasUVRect
			syncSprite.UpdateTextureAtlas(path, rect, renderScale, !p.runtimeState.isAnimating)
			p.setMaterialParamsVec4("atlas_uv_rect2", uvRemap, true)
		} else {
			syncSprite.UpdateTexture(path, renderScale, !p.runtimeState.isAnimating)
		}
	})
}

// -----------------------------------------------------------------------------
// Base Object Visual Sync
// -----------------------------------------------------------------------------
// applyDirtyUpdate consumes only this update, preserving reentrant changes and
// restoring the retry flag if the engine write fails.
func applyDirtyUpdate(dirty *bool, update func()) {
	if !*dirty {
		return
	}
	*dirty = false
	completed := false
	defer func() {
		if !completed {
			*dirty = true
		}
	}()
	update()
	completed = true
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------
func commitProxyChange(update proxyChange) {
	p := update.sprite
	if p == nil || p.runtimeState.syncSprite != update.proxy {
		return
	}
	if update.destroy {
		p.runtimeState.syncSprite = nil
		return
	}
	if !p.preparedVisual.preparing && p.spriteState.dirtyVersion == update.dirtyVersion {
		p.spriteState.proxySyncVersion = update.dirtyVersion
	}
	// Confirm each axis separately; preserve writes made after collection.
	pending := &p.transform().pendingPosition
	if pending.x == update.positionVersions.x {
		pending.x = 0
	}
	if pending.y == update.positionVersions.y {
		pending.y = 0
	}
}

func applyPhysicsPositions(sprites []*SpriteImpl, positions []float32) {
	for i, sprite := range sprites {
		at := i * 2
		if at+1 >= len(positions) {
			return
		}
		x, y := float64(positions[at]), float64(positions[at+1])
		if math.IsNaN(x) || math.IsNaN(y) {
			continue
		}
		sprite.mergeEnginePosition(x, y)
	}
}

func isSpriteTouchable(sprite *SpriteImpl) bool {
	return sprite.spriteState.isVisible && !sprite.spriteState.isDying
}
