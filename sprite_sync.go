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
	"slices"
	"sync/atomic"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/base"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/engine"
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
	query   engine.PositionQueryBuffer
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
	transform engine.SpriteTransform
	change    proxyChange
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

// proxyQueue retains dirty sprites and deletions until their batch commits.
type proxyQueue struct {
	dirtySprites      []*SpriteImpl
	dirtySpriteIndex  map[*SpriteImpl]int
	destroyItems      []*SpriteImpl
	proxyChangeBuffer []proxyChange
}

// Engine Callbacks.
// mirrorPosition applies engine position writes without movement side effects.
func (p *SpriteImpl) mirrorPosition(proxy *engine.Sprite, position mathf.Vec2, axes engine.PositionAxes) {
	if p.isDestroyed() || p.runtimeState.proxy != proxy {
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
	if p.preparedVisual.proxy == p.runtimeState.proxy && p.preparedVisual.preparing {
		return
	}
	p.applyLayerUpdate()
	// Hidden transforms can be acknowledged while their resources stay dirty.
	// Preparing those resources must submit their matching geometry again.
	if p.runtimeState.isCostumeDirty && p.preparedVisual.proxy == p.runtimeState.proxy &&
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

// Sprite Proxy Lifecycle.
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
	p.runtimeState.proxy = nil
	engine.WaitMainThread(func() {
		p.ensureProxyInitialized()
		p.applyCostumeUpdate()
	})
}

func (p *SpriteImpl) awake() {
	p.animation().playDefaultAnimIfIdle()
}

func (p *SpriteImpl) ensureProxyInitialized() {
	if p.runtimeState.proxy != nil || p.isDestroyed() {
		return
	}
	p.runtimeState.proxy = engine.BridgeNewBareSprite(p, mathf.NewVec2(p.getXY()))
	p.runtimeState.proxy.CallbackAlive = func() bool { return !p.isDestroyed() }
	p.runtimeState.proxy.MirrorPosition = p.mirrorPosition
	p.physics().initializeProxy(p.runtimeState.proxy)
	p.runtimeState.proxy.SetVisible(p.effectiveProxyVisibility())
	p.runtimeState.proxy.SetTypeName(p.name)
	p.applyGraphicEffects(true)
	p.runtimeState.proxy.RegisterOnAnimationLooped(p.handleAnimationLooped)
	p.runtimeState.proxy.RegisterOnAnimationFinished(p.handleAnimationFinished)
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
	if p.isDestroyed() || p.runtimeState.proxy == nil {
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
	if p.isDestroyed() || p.runtimeState.proxy == nil {
		return
	}
	p.queueAnimationLoopAudio(p.animation().curAnimState)
	p.queueAnimationLoopAudio(p.animation().getCurTweenState())
}

// Sprite Sync.
func (p *SpriteImpl) shouldPullPhysicsPosition() bool {
	return p.runtimeState.proxy != nil && p.PhysicsMode() != NoPhysics
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
	engine.WaitMainThread(p.syncProxyForQueryOnMainThread)
}

// The caller must already be on the engine main thread.
func (p *SpriteImpl) syncProxyForQueryOnMainThread() {
	p.prepareProxyForQueryOnMainThread()
	p.submitProxyForQueryOnMainThread()
}

// The caller must already be on the engine main thread.
func (p *SpriteImpl) prepareProxyForQueryOnMainThread() {
	if p.isDestroyed() || p.runtimeState.proxy == nil {
		if p.g != nil {
			p.g.scene.removeDirtySprite(p)
		}
		return
	}
	// Queries need the latest silhouette even when the sprite is hidden.
	p.applyCostumeUpdate()
	p.syncAutoPhysicsShapesAfterCostumeChange()
}

// The caller must already be on the engine main thread.
func (p *SpriteImpl) submitProxyForQueryOnMainThread() {
	if p.isDestroyed() || p.runtimeState.proxy == nil {
		if p.g != nil {
			p.g.scene.removeDirtySprite(p)
		}
		return
	}
	if !p.needsProxyUpdate() {
		if p.g != nil {
			p.g.scene.removeDirtySprite(p)
		}
		return
	}
	// Read and submit in one engine job; queued queries may precede the frame pull.
	if p.shouldPullPhysicsPosition() {
		position := p.runtimeState.proxy.GetPosition()
		p.mergeEnginePosition(position.X, position.Y)
	}
	snapshot := p.snapshotProxy()
	snapshot.change.proxy.ApplyTransform(snapshot.transform)
	commitProxyChange(snapshot.change)
}

func (p *SpriteImpl) prepareProxyUpdate() {
	if p.isDestroyed() || p.runtimeState.proxy == nil {
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
	buffer.AddTransform(snapshot.change.proxy.Id, snapshot.transform)
	return snapshot.change
}

func (p *SpriteImpl) needsProxyUpdate() bool {
	return !p.isDestroyed() && p.runtimeState.proxy != nil &&
		p.spriteState.proxySyncVersion != p.spriteState.dirtyVersion
}

func (p *SpriteImpl) needsQueryProxyUpdate() bool {
	return !p.isDestroyed() && p.runtimeState.proxy != nil &&
		(p.needsProxyUpdate() || p.runtimeState.isCostumeDirty || p.runtimeState.isLayerDirty)
}

func (p *SpriteImpl) snapshotProxy() proxySnapshot {
	prepared := &p.preparedVisual
	visual := *prepared
	resourcesReady := !p.runtimeState.isCostumeDirty && !prepared.preparing
	// Reentrant costume changes wait for their resources, while position and
	// visibility still include writes made by later sprites' callbacks.
	if prepared.proxy != p.runtimeState.proxy || resourcesReady {
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
		transform: engine.SpriteTransform{
			Position:     mathf.NewVec2(p.getXY()),
			Rotation:     visual.rotation,
			Scale:        visual.scale,
			RenderOffset: visual.renderOffset,
			Visible:      p.effectiveProxyVisibility(),
		},
		change: proxyChange{
			sprite: p, proxy: visual.proxy,
			dirtyVersion: dirtyVersion, positionVersions: p.transform().pendingPosition,
		},
	}
}

func (p *SpriteImpl) captureProxyVisual() preparedProxyVisual {
	offsetX, offsetY := getRenderOffset(p)
	rotation, flipX, flipY := renderRotationAndFlip(p)
	return preparedProxyVisual{
		proxyVisual: proxyVisual{
			scale:        mathf.NewVec2(flipX, flipY),
			renderOffset: mathf.NewVec2(offsetX, offsetY),
			rotation:     engine.DegToRad(rotation),
		},
		proxy:        p.runtimeState.proxy,
		dirtyVersion: p.spriteState.dirtyVersion,
	}
}

// Frame Sync.
// Color queries sample every native scene sprite, so synchronize all queued
// proxy changes before querying.
func (p *Game) syncSpriteProxiesForQuery(owner *SpriteImpl) {
	if len(p.scene.dirtySprites) == 0 &&
		(owner == nil || !owner.needsQueryProxyUpdate()) {
		return
	}
	engine.WaitMainThread(func() {
		sprites := make([]*SpriteImpl, 0, len(p.scene.dirtySprites)+1)
		seen := make(map[*SpriteImpl]struct{}, cap(sprites))
		appendPending := func() {
			for _, sprite := range p.scene.dirtySprites {
				if _, ok := seen[sprite]; ok {
					continue
				}
				seen[sprite] = struct{}{}
				sprites = append(sprites, sprite)
			}
			if owner != nil && owner.needsQueryProxyUpdate() {
				if _, ok := seen[owner]; !ok {
					seen[owner] = struct{}{}
					sprites = append(sprites, owner)
				}
			}
		}

		appendPending()
		// Resource writes may synchronously dirty more sprites. Each sprite is
		// prepared once, so the worklist remains bounded.
		for i := 0; i < len(sprites); i++ {
			sprites[i].prepareProxyForQueryOnMainThread()
			appendPending()
		}
		// Capture transforms only after resource callbacks have completed.
		for _, sprite := range sprites {
			sprite.submitProxyForQueryOnMainThread()
		}
	})
}

func (p *Game) flushSpriteProxyChanges(activeShapes []Shape) {
	batch := proxyBatch{changes: p.scene.proxyChangeBuffer}
	p.scene.proxyChangeBuffer = nil
	defer func() {
		p.scene.proxyChangeBuffer = base.Reset(batch.changes, 0)
	}()
	p.syncBuffer.Clear()
	p.scene.collectProxyChanges(activeShapes, p.syncBuffer, &batch)
	p.flushSyncBuffer()
	p.scene.commitProxyChanges(batch)
	p.camera.setDirtyFlag(false)
}

func (p *Game) flushSyncBuffer() {
	engine.SyncBatchUpdateSprites(p.syncBuffer.Serialize())
}

func (p *Game) pullPhysicsPositions() {
	pull := &p.physicsPull
	reuse := pull.busy.CompareAndSwap(false, true)
	if !reuse {
		pull = &physicsPullBuffer{}
	}
	pull.ids = pull.ids[:0]
	defer func() {
		pull.sprites = base.Reset(pull.sprites, 0)
		if reuse {
			pull.busy.Store(false)
		}
	}()
	for _, shape := range p.scene.items {
		sprite, ok := shape.(*SpriteImpl)
		if !ok || !sprite.shouldPullPhysicsPosition() {
			continue
		}
		pull.ids = append(pull.ids, sprite.runtimeState.proxy.Id)
		pull.sprites = append(pull.sprites, sprite)
	}
	if len(pull.ids) == 0 {
		return
	}
	applyPhysicsPositions(pull.sprites, pull.query.GetPositions(pull.ids))
}

func (p *Game) processTouchEvents() {
	p.triggerEvents = engine.GetTriggerEvents(p.triggerEvents)
	p.dispatchTouchEvents()
}

// dispatchTouchEvents retains the batch on panic.
func (p *Game) dispatchTouchEvents() {
	for _, pair := range p.triggerEvents {
		if pair.Src == nil || pair.Dst == nil {
			spxlog.Info("Touch event skipped: missing sprite proxy")
			continue
		}
		src, ok1 := pair.Src.Target.(*SpriteImpl)
		dst, ok2 := pair.Dst.Target.(*SpriteImpl)
		if !ok1 || !ok2 {
			spxlog.Info("Touch event skipped: invalid sprite target type")
			continue
		}
		if !src.isTouchable() || !dst.isTouchable() || dst.isDragging() {
			continue
		}
		// Geometry may disappear between detection and dispatch.
		if p.touchByShape && (!src.hasTouchShape() || !dst.hasTouchShape()) {
			continue
		}
		src.doWhenTouchStart(dst)
	}
	p.triggerEvents = base.Reset(p.triggerEvents, 0)
}

func (p *visualObject) applyLayerUpdate() {
	applyDirtyUpdate(&p.runtimeState.isLayerDirty, func() {
		if !engine.HasLayerSortMethod() {
			p.runtimeState.proxy.SetZIndex(int64(p.runtimeState.layer))
		}
	})
}

func (p *visualObject) applyCostumeTexture() {
	applyDirtyUpdate(&p.runtimeState.isCostumeDirty, func() {
		proxy := p.runtimeState.proxy
		path := p.getCostumePath()
		renderScale := p.getCostumeRenderScale()
		if p.isCostumeAtlas() {
			rect := p.getCostumeAtlasRegion()
			uvRemap := p.currentCostume().frame.AtlasUVRect
			proxy.UpdateTextureAtlas(path, rect, renderScale, !p.runtimeState.isAnimating)
			p.setMaterialParamsVec4("atlas_uv_rect2", uvRemap, true)
		} else {
			proxy.UpdateTexture(path, renderScale, !p.runtimeState.isAnimating)
		}
	})
}

func (s *proxyQueue) reset() {
	s.dirtySprites = base.Reset(s.dirtySprites, 0)
	clear(s.dirtySpriteIndex)
	s.destroyItems = base.Reset(s.destroyItems, 16)
	s.proxyChangeBuffer = base.Reset(s.proxyChangeBuffer, 0)
}

func (s *proxyQueue) noteProxyDirty(sprite *SpriteImpl) {
	if s.dirtySpriteIndex == nil {
		s.dirtySpriteIndex = make(map[*SpriteImpl]int)
	}
	if _, exists := s.dirtySpriteIndex[sprite]; exists {
		return
	}
	s.dirtySpriteIndex[sprite] = len(s.dirtySprites)
	s.dirtySprites = append(s.dirtySprites, sprite)
}

// Track sprites dirtied before they entered the active shape list.
func (s *proxyQueue) trackExistingDirtySprite(sprite *SpriteImpl) {
	if sprite == nil || !sprite.needsProxyUpdate() {
		return
	}
	s.noteProxyDirty(sprite)
}

func (s *proxyQueue) removeDirtySprite(sprite *SpriteImpl) {
	if sprite == nil {
		return
	}
	idx, exists := s.dirtySpriteIndex[sprite]
	if !exists {
		return
	}
	last := len(s.dirtySprites) - 1
	if idx != last {
		swapped := s.dirtySprites[last]
		s.dirtySprites[idx] = swapped
		s.dirtySpriteIndex[swapped] = idx
	}
	s.dirtySprites[last] = nil
	s.dirtySprites = s.dirtySprites[:last]
	delete(s.dirtySpriteIndex, sprite)
}

func (s *proxyQueue) collectProxyChanges(items []Shape, buffer *engine.SpriteSyncBuffer, batch *proxyBatch) {
	// Prepare resources first, then capture the latest position and visibility.
	// Each sprite keeps the geometry paired with its prepared resources.
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			sprite.prepareProxyUpdate()
		}
	}
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			if update := sprite.collectProxyUpdate(buffer); update.sprite != nil {
				batch.changes = append(batch.changes, update)
			}
		}
	}

	// Keep deletion requests pending until this batch commits.
	batch.destroyCount = len(s.destroyItems)
	for _, sprite := range s.destroyItems {
		proxy := sprite.runtimeState.proxy
		if proxy == nil {
			continue
		}
		buffer.AddDelete(proxy.Id)
		batch.changes = append(batch.changes, proxyChange{sprite: sprite, proxy: proxy, destroy: true})
	}
}

func (s *proxyQueue) commitProxyChanges(batch proxyBatch) {
	for _, change := range batch.changes {
		commitProxyChange(change)
	}
	// Deletions queued during submission belong to the next batch.
	s.destroyItems = slices.Delete(s.destroyItems, 0, batch.destroyCount)
}

// Base Object Visual Sync.
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

// Helpers.
func commitProxyChange(update proxyChange) {
	p := update.sprite
	if p == nil || p.runtimeState.proxy != update.proxy {
		return
	}
	if update.destroy {
		p.runtimeState.proxy = nil
		if p.g != nil {
			p.g.scene.removeDirtySprite(p)
		}
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
	if p.g != nil && !p.needsProxyUpdate() {
		p.g.scene.removeDirtySprite(p)
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
