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
	"strings"

	"github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/core/collision"
	"github.com/goplus/spx/v4/internal/engine"
)

type pendingCloneSensingVisibilityLease struct {
	sprite      *SpriteImpl
	publication *cloneProxyPublication
	proxy       *engine.Sprite
}

const (
	edgeAreaStage    = "stage"
	edgeAreaCamera   = "camera"
	edgeAreaViewport = "viewport"
)

// -----------------------------------------------------------------------------
// Public API
// -----------------------------------------------------------------------------
func (p *SpriteImpl) TouchingColor__0(color Color) bool {
	return p.touchingColor(toMathfColor(color))
}

func (p *SpriteImpl) TouchingColor__1(spriteColor, targetColor Color) bool {
	return p.touchingColors(toMathfColor(spriteColor), toMathfColor(targetColor))
}

func (p *SpriteImpl) Touching__0(sprite Sprite) bool {
	return p.touching(sprite)
}

func (p *SpriteImpl) Touching__1(name SpriteName) bool {
	return p.touching(name)
}

func (p *SpriteImpl) Touching__2(object specialObj) bool {
	return p.touching(object)
}

func (p *SpriteImpl) TouchingWith(target Target) bool {
	return p.touching(target)
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

// -----------------------------------------------------------------------------
// Internal Queries
// -----------------------------------------------------------------------------
func (p *SpriteImpl) hasCostume() bool {
	return p.costumeIndex >= 0 && p.costumeIndex < len(p.costumes) && p.costumes[p.costumeIndex] != nil
}

func (p *SpriteImpl) bounds() *mathf.Rect2 {
	if !p.hasCostume() {
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
	bounds, ok := p.renderBounds()
	if !ok {
		return nil
	}
	return &bounds
}

func (p *SpriteImpl) renderBounds() (mathf.Rect2, bool) {
	if !p.hasCostume() {
		return mathf.Rect2{}, false
	}

	costume := p.currentCostume()
	width, height := costume.sizeInSPX()
	width *= p.runtimeState.scale
	height *= p.runtimeState.scale

	centerX, centerY := p.getXY()
	offsetX, offsetY := getWorldRenderOffset(p)
	centerX += offsetX
	centerY += offsetY

	rotation, _, _ := getRenderRotationAndScale(p)
	sin, cos := math.Sincos(toRadian(rotation))
	aabbWidth := math.Abs(cos)*width + math.Abs(sin)*height
	aabbHeight := math.Abs(sin)*width + math.Abs(cos)*height

	return mathf.Rect2{
		Position: mathf.NewVec2(centerX-aabbWidth/2, centerY-aabbHeight/2),
		Size:     mathf.NewVec2(aabbWidth, aabbHeight),
	}, true
}

func (p *SpriteImpl) adjustPositionAndGetDimensions(x, y *float64) (width, height float64) {
	triggerCollider := &p.physics().triggerCollider

	if triggerCollider.Type == collision.ColliderNone {
		applyRenderOffset(p, x, y)
		width, height := p.costumes[p.costumeIndex].sizeInSPX()
		return width * p.runtimeState.scale, height * p.runtimeState.scale
	}

	if triggerCollider.Type == collision.ColliderAuto && p.runtimeState.syncSprite == nil {
		center, size := getCostumeBoundByAlpha(p)
		triggerCollider.Pivot = center
		triggerCollider.Params = []float64{size.X, size.Y}
	}
	if triggerCollider.Type == collision.ColliderAuto {
		applyRenderOffset(p, x, y)
	}

	*x += triggerCollider.Pivot.X * p.runtimeState.scale
	*y += triggerCollider.Pivot.Y * p.runtimeState.scale

	w, h := triggerCollider.getDimensions()
	return w * p.runtimeState.scale, h * p.runtimeState.scale
}

func (p *SpriteImpl) touching(obj Target) bool {
	if p.spriteState.isDying {
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
		if src == nil || src.spriteState.isDying || src.isDragging() {
			return false
		}
		return src.touchingSprite(p)
	}
	panic("Touching: unexpected input")
}

func (p *SpriteImpl) prepareSelfCollisionQuery() bool {
	if p.runtimeState.syncSprite == nil {
		return false
	}
	p.syncProxyForQuery()
	return true
}

func (p *SpriteImpl) touchPoint(x, y float64) bool {
	if !p.prepareSelfCollisionQuery() {
		return false
	}
	// Scratch's sensing path ignores visible/ghost when checking whether a sprite
	// is touching the mouse, so use the sensing branch in the engine query.
	return engine.Managers().SpriteMgr.CheckCollisionWithPoint(p.runtimeState.syncSprite.GetId(), mathf.NewVec2(x, y), false)
}

func (p *SpriteImpl) touchingColor(color mathf.Color) bool {
	if !p.prepareSceneColorQuery() {
		return false
	}
	return engine.Managers().SpriteMgr.CheckCollisionByColor(p.runtimeState.syncSprite.GetId(), color, colorThreshold, alphaThreshold)
}

func (p *SpriteImpl) touchingColors(spriteColor, targetColor mathf.Color) bool {
	if !p.prepareSceneColorQuery() {
		return false
	}
	return engine.Managers().SpriteMgr.CheckCollisionByColors(
		p.runtimeState.syncSprite.GetId(),
		spriteColor,
		targetColor,
		colorThreshold,
		alphaThreshold,
	)
}

func (p *SpriteImpl) prepareSceneColorQuery() bool {
	if p.g == nil || p.runtimeState.syncSprite == nil {
		return false
	}
	p.g.syncSpriteProxiesForQuery(p)
	p.g.flushPenCommands()
	return true
}

func (p *SpriteImpl) isTouchable() bool {
	return p.spriteState.isVisible && !p.spriteState.isDying
}

// hasTouchShape ignores notification enablement.
func (p *SpriteImpl) hasTouchShape() bool {
	return p.physics().triggerCollider.Type != collision.ColliderNone
}

func (p *SpriteImpl) touchingSprite(dst *SpriteImpl) bool {
	if !p.prepareSelfCollisionQuery() || !dst.prepareSelfCollisionQuery() {
		return false
	}
	usePixels := !p.g.touchByShape
	if !usePixels && (!p.hasTouchShape() || !dst.hasTouchShape()) {
		return false
	}
	if !p.isCloneProxyPublicationBlocked() && !dst.isCloneProxyPublicationBlocked() {
		return engine.Managers().SpriteMgr.CheckCollisionWithSprite(
			p.runtimeState.syncSprite.GetId(),
			dst.runtimeState.syncSprite.GetId(),
			alphaThreshold,
			usePixels,
		)
	}
	for _, sprite := range []*SpriteImpl{p, dst} {
		if sprite.isCloneProxyPublicationBlocked() && !sprite.spriteState.isVisible {
			return false
		}
	}

	var touching bool
	engine.WaitMainThread(func() {
		withPendingCloneSensingVisibility([]*SpriteImpl{p, dst}, func() {
			touching = p.runtimeState.syncSprite.CheckCollisionWithSprite(
				dst.runtimeState.syncSprite.GetId(),
				alphaThreshold,
				usePixels,
			)
		})
	})
	return touching
}

func (p *SpriteImpl) touchBounds(usePixels bool) (collision.AABB, bool) {
	if usePixels {
		return p.costumeTouchBounds(false)
	}

	cfg := &p.physics().triggerCollider
	box, ok := cfg.worldBounds(p)
	if !ok || cfg.Type != collision.ColliderAuto {
		return box, ok
	}

	costumeBox, ok := p.costumeTouchBounds(true)
	if !ok {
		return collision.AABB{}, false
	}
	// Include explicit auto-shape edits and any costume refresh still pending.
	box.MinX, box.MinY = math.Min(box.MinX, costumeBox.MinX), math.Min(box.MinY, costumeBox.MinY)
	box.MaxX, box.MaxY = math.Max(box.MaxX, costumeBox.MaxX), math.Max(box.MaxY, costumeBox.MaxY)
	return box, true
}

func (p *SpriteImpl) costumeTouchBounds(withTriggerPadding bool) (collision.AABB, bool) {
	bounds, ok := p.renderBounds()
	if !ok {
		return collision.AABB{}, false
	}
	padding := 0.0
	if withTriggerPadding {
		// Auto triggers extend beyond the costume, including after rotation.
		rotation, _, _ := getRenderRotationAndScale(p)
		sin, cos := math.Sincos(toRadian(rotation))
		padding = TriggerExtraPixel * math.Abs(p.runtimeState.scale) * (math.Abs(sin) + math.Abs(cos)) / 2
	}
	return collision.AABB{
		MinX: bounds.Position.X - padding,
		MinY: bounds.Position.Y - padding,
		MaxX: bounds.Position.X + bounds.Size.X + padding,
		MaxY: bounds.Position.Y + bounds.Size.Y + padding,
	}, true
}

func (p *SpriteImpl) checkTouchingScreen(where int, area string) (touching int) {
	if !p.prepareSelfCollisionQuery() {
		return 0
	}
	switch normalizeEdgeArea(area) {
	case edgeAreaCamera:
		touching = int(engine.Managers().PhysicsMgr.CheckTouchedCameraBoundaries(p.runtimeState.syncSprite.GetId()))
	default:
		touching = int(engine.Managers().PhysicsMgr.CheckTouchedStageBoundaries(p.runtimeState.syncSprite.GetId()))
	}
	return touching & where
}

func (p *SpriteImpl) checkNearestTouchedBoundary(area string) int {
	if !p.prepareSelfCollisionQuery() {
		return 0
	}
	switch normalizeEdgeArea(area) {
	case edgeAreaCamera:
		return int(engine.Managers().PhysicsMgr.CheckNearestTouchedCameraBoundary(p.runtimeState.syncSprite.GetId()))
	default:
		return int(engine.Managers().PhysicsMgr.CheckNearestTouchedStageBoundary(p.runtimeState.syncSprite.GetId()))
	}
}

// -----------------------------------------------------------------------------
// Game Queries
// -----------------------------------------------------------------------------
func (p *Game) touchingSpriteBy(dst *SpriteImpl, name string) *SpriteImpl {
	if dst == nil || dst.runtimeState.syncSprite == nil {
		return nil
	}
	usePixels := !p.touchByShape
	dstBox, dstBounded := dst.touchBounds(usePixels)
	for _, sp := range p.shapeMgr.spritesNamed(name) {
		if sp == dst || !sp.isTouchable() || sp.isDragging() || sp.runtimeState.syncSprite == nil {
			continue
		}
		if box, bounded := sp.touchBounds(usePixels); dstBounded && bounded && !dstBox.Intersects(box) {
			continue
		}
		if sp.touchingSprite(dst) {
			return sp
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------
// withPendingCloneSensingVisibility makes logical visibility queryable without
// making a pending clone renderable. The native visibility bit is leased only
// inside one main-thread callback, so rendering and physics cannot interleave
// the temporary exposure.
func withPendingCloneSensingVisibility(sprites []*SpriteImpl, call func()) {
	leases := make([]pendingCloneSensingVisibilityLease, 0, len(sprites))
	defer func() {
		failure := recover()
		var cleanupFailure any
		for _, lease := range slices.Backward(leases) {
			if recovered := capturePendingCloneSensingPanic(func() {
				releasePendingCloneSensingVisibility(lease)
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
		proxy := sprite.runtimeState.syncSprite
		if publication == nil || proxy == nil ||
			publication.state.Load() == cloneProxyPublished {
			continue
		}
		leaseCount := publication.sensingVisibilityLeases.Add(1)
		leases = append(leases, pendingCloneSensingVisibilityLease{
			sprite: sprite, publication: publication, proxy: proxy,
		})
		if leaseCount == 1 {
			proxy.SetVisible(true)
		}
	}
	call()
}

func releasePendingCloneSensingVisibility(lease pendingCloneSensingVisibilityLease) {
	publication := lease.publication
	leaseCount := publication.sensingVisibilityLeases.Add(-1)
	if leaseCount < 0 {
		panic("spx: negative pending clone sensing visibility lease count")
	}
	if leaseCount == 0 {
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

func normalizeEdgeArea(area string) string {
	switch area = strings.ToLower(area); area {
	case "", edgeAreaStage:
		return edgeAreaStage
	case edgeAreaCamera, edgeAreaViewport:
		return edgeAreaCamera
	default:
		spxlog.Warn("unrecognized edge area %q, using %q", area, edgeAreaStage)
		return edgeAreaStage
	}
}
