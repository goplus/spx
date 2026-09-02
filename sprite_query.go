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
	"github.com/goplus/spx/v3/internal/engine"
)

func (p *SpriteImpl) bounds() *mathf.Rect2 {
	if len(p.costumes) == 0 || p.costumeIndex < 0 || p.costumeIndex >= len(p.costumes) {
		return nil
	}

	x, y := p.getXY()
	w, h := p.adjustPositionAndGetDimensions(&x, &y)

	return &mathf.Rect2{
		Position: mathf.Vec2{X: x - w*0.5, Y: y - h*0.5},
		Size:     mathf.Vec2{X: w, Y: h},
	}
}

// fenceBounds returns the axis-aligned bounds of the rendered costume. Scratch
// fences drawables by their full skin bounds, independent of collision shapes.
func (p *SpriteImpl) fenceBounds() *mathf.Rect2 {
	if len(p.costumes) == 0 || p.costumeIndex < 0 || p.costumeIndex >= len(p.costumes) {
		return nil
	}

	costume := p.currentCostume()
	width, height := costume.getSizeF()
	width *= p.runtimeState.Scale
	height *= p.runtimeState.Scale

	centerX, centerY := p.getXY()
	offsetX, offsetY := getWorldRenderOffset(p)
	centerX += offsetX
	centerY += offsetY

	rotation, _, _ := getRenderRotationAndScale(p)
	sin, cos := math.Sincos(toRadian(rotation))
	aabbWidth := math.Abs(cos)*width + math.Abs(sin)*height
	aabbHeight := math.Abs(sin)*width + math.Abs(cos)*height

	return &mathf.Rect2{
		Position: mathf.NewVec2(centerX-aabbWidth/2, centerY-aabbHeight/2),
		Size:     mathf.NewVec2(aabbWidth, aabbHeight),
	}
}

func (p *SpriteImpl) adjustPositionAndGetDimensions(x, y *float64) (width, height float64) {
	triggerInfo := p.physics().getTriggerInfo()

	if triggerInfo.Type == physicsColliderNone {
		applyRenderOffset(p, x, y)
		width, height := p.costumes[p.costumeIndex].getSizeF()
		return width * p.runtimeState.Scale, height * p.runtimeState.Scale
	}

	if triggerInfo.Type == physicsColliderAuto && p.runtimeState.SyncSprite == nil {
		center, size := getCostumeBoundByAlpha(p, false)
		triggerInfo.Pivot = center
		triggerInfo.Params = []float64{size.X, size.Y}
	}
	if triggerInfo.Type == physicsColliderAuto {
		applyRenderOffset(p, x, y)
	}

	*x += triggerInfo.Pivot.X * p.runtimeState.Scale
	*y += triggerInfo.Pivot.Y * p.runtimeState.Scale

	w, h := triggerInfo.getDimensions()
	return w * p.runtimeState.Scale, h * p.runtimeState.Scale
}

func (p *SpriteImpl) TouchingColor__0(color Color) bool {
	return p.touchingColor(toMathfColor(color))
}

func (p *SpriteImpl) TouchingColor__1(spriteColor, targetColor Color) bool {
	return p.touchingColors(toMathfColor(spriteColor), toMathfColor(targetColor))
}

func (p *SpriteImpl) Touching__0(sprite Sprite) bool {
	return p.touching(sprite)
}

func (p *SpriteImpl) Touching__1(sprite SpriteName) bool {
	return p.touching(sprite)
}

func (p *SpriteImpl) Touching__2(obj specialObj) bool {
	return p.touching(obj)
}

func (p *SpriteImpl) TouchingWith(target Target) bool {
	return p.touching(target)
}

func (p *SpriteImpl) touching(obj Target) bool {
	if p.spriteState.IsDying {
		return false
	}
	switch v := obj.(type) {
	case SpriteName:
		return p.g.touchingSpriteBy(p, v) != nil
	case specialObj:
		if v > 0 {
			return p.checkTouchingScreen(int(v), edgeAreaStage) != 0
		}
		if v == Mouse {
			x, y := p.g.getMousePos()
			return p.g.touchingPoint(p, x, y)
		}
	case Sprite:
		src := spriteOf(v)
		if src == nil || src.spriteState.IsDying {
			return false
		}
		return src.touchingSprite(p)
	}
	panic("Touching: unexpected input")
}

func (p *SpriteImpl) prepareSelfCollisionQuery() bool {
	if p.runtimeState.SyncSprite == nil {
		return false
	}
	p.ensureProxyQueryStateSynced()
	return true
}

func (p *SpriteImpl) touchPoint(x, y float64) bool {
	if !p.prepareSelfCollisionQuery() {
		return false
	}
	// Scratch's sensing path ignores visible/ghost when checking whether a sprite
	// is touching the mouse, so use the sensing branch in the engine query.
	return p.engine().SpriteMgr.CheckCollisionWithPoint(p.runtimeState.SyncSprite.GetId(), mathf.NewVec2(x, y), false)
}

func (p *SpriteImpl) touchingColor(color mathf.Color) bool {
	if !p.prepareSelfCollisionQuery() {
		return false
	}
	return p.engine().SpriteMgr.CheckCollisionByColor(p.runtimeState.SyncSprite.GetId(), color, colorThreshold, alphaThreshold)
}

func (p *SpriteImpl) touchingColors(spriteColor, targetColor mathf.Color) bool {
	if !p.prepareSelfCollisionQuery() {
		return false
	}
	return p.engine().SpriteMgr.CheckCollisionByColors(
		p.runtimeState.SyncSprite.GetId(),
		spriteColor,
		targetColor,
		colorThreshold,
		alphaThreshold,
	)
}

func (p *SpriteImpl) touchingSprite(dst *SpriteImpl) bool {
	if !p.prepareSelfCollisionQuery() || !dst.prepareSelfCollisionQuery() {
		return false
	}
	usePixelPerfect := !isPhysicsEnabled()
	if !p.isCloneProxyPublicationBlocked() && !dst.isCloneProxyPublicationBlocked() {
		return p.engine().SpriteMgr.CheckCollisionWithSprite(
			p.runtimeState.SyncSprite.GetId(),
			dst.runtimeState.SyncSprite.GetId(),
			alphaThreshold,
			usePixelPerfect,
		)
	}
	for _, sprite := range []*SpriteImpl{p, dst} {
		if sprite.isCloneProxyPublicationBlocked() && !sprite.spriteState.IsVisible {
			return false
		}
	}

	var touching bool
	engine.WaitMainThread(func() {
		withPendingCloneSensingVisibility([]*SpriteImpl{p, dst}, func() {
			touching = p.runtimeState.SyncSprite.CheckCollisionWithSprite(
				dst.runtimeState.SyncSprite.GetId(),
				alphaThreshold,
				usePixelPerfect,
			)
		})
	})
	return touching
}

type pendingCloneSensingVisibilityLease struct {
	sprite      *SpriteImpl
	publication *cloneProxyPublication
	proxy       *engine.Sprite
}

// withPendingCloneSensingVisibility makes logical visibility queryable without
// making a pending clone renderable. The native visibility bit is leased only
// inside one main-thread callback, so rendering and physics cannot interleave
// the temporary exposure.
func withPendingCloneSensingVisibility(sprites []*SpriteImpl, call func()) {
	leases := make([]pendingCloneSensingVisibilityLease, 0, len(sprites))
	defer func() {
		failure := recover()
		var cleanupFailure any
		for i := len(leases) - 1; i >= 0; i-- {
			if recovered := capturePendingCloneSensingPanic(func() {
				releasePendingCloneSensingVisibility(leases[i])
			}); cleanupFailure == nil {
				cleanupFailure = recovered
			}
		}
		if failure != nil {
			panic(failure)
		}
		if cleanupFailure != nil {
			panic(cleanupFailure)
		}
	}()

	for _, sprite := range sprites {
		publication := sprite.proxyPublication
		proxy := sprite.runtimeState.SyncSprite
		if publication == nil || proxy == nil ||
			atomic.LoadUint32(&publication.state) == cloneProxyPublished {
			continue
		}
		publication.sensingVisibilityLeases++
		leases = append(leases, pendingCloneSensingVisibilityLease{
			sprite: sprite, publication: publication, proxy: proxy,
		})
		if publication.sensingVisibilityLeases == 1 {
			proxy.SetVisible(true)
		}
	}
	call()
}

func releasePendingCloneSensingVisibility(lease pendingCloneSensingVisibilityLease) {
	publication := lease.publication
	publication.sensingVisibilityLeases--
	if publication.sensingVisibilityLeases < 0 {
		panic("spx: negative pending clone sensing visibility lease count")
	}
	if publication.sensingVisibilityLeases == 0 {
		lease.proxy.SetVisible(lease.sprite.effectiveProxyVisibility())
	}
}

func capturePendingCloneSensingPanic(call func()) (recovered any) {
	defer func() {
		recovered = recover()
	}()
	call()
	return nil
}

func (p *SpriteImpl) checkTouchingScreen(where int, area string) (touching int) {
	if !p.prepareSelfCollisionQuery() {
		return 0
	}
	switch normalizeEdgeArea(area) {
	case edgeAreaCamera:
		touching = int(p.engine().PhysicsMgr.CheckTouchedCameraBoundaries(p.runtimeState.SyncSprite.GetId()))
	default:
		touching = int(p.engine().PhysicsMgr.CheckTouchedStageBoundaries(p.runtimeState.SyncSprite.GetId()))
	}
	return touching & where
}

func (p *SpriteImpl) checkNearestTouchedBoundary(area string) int {
	if !p.prepareSelfCollisionQuery() {
		return 0
	}
	switch normalizeEdgeArea(area) {
	case edgeAreaCamera:
		return int(p.engine().PhysicsMgr.CheckNearestTouchedCameraBoundary(p.runtimeState.SyncSprite.GetId()))
	default:
		return int(p.engine().PhysicsMgr.CheckNearestTouchedStageBoundary(p.runtimeState.SyncSprite.GetId()))
	}
}

func (p *SpriteImpl) HideVar(name PropertyName) {
	if !p.g.setStageMonitor(p.name, name, false) {
		p.g.setStageMonitor("", name, false)
	}
}

func (p *SpriteImpl) ShowVar(name PropertyName) {
	if !p.g.setStageMonitor(p.name, name, true) {
		p.g.setStageMonitor("", name, true)
	}
}
