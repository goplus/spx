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
	"strconv"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine"
	spxlog "github.com/goplus/spx/v4/internal/log"
)

var (
	cachedBounds map[string]mathf.Rect2
)

// -----------------------------------------------------------------------------
// Public API
// -----------------------------------------------------------------------------
func (p *SpriteImpl) Hide() {
	if bubble := p.components.bubble; bubble != nil {
		bubble.stopAll()
	}
	p.setVisible(false)
}

func (p *SpriteImpl) Show() {
	p.setVisible(true)
}

func (p *SpriteImpl) Visible() bool {
	return p.spriteState.IsVisible
}

func (p *SpriteImpl) CostumeName() SpriteCostumeName {
	return p.getCostumeName()
}

func (p *SpriteImpl) CostumeIndex() int {
	return p.getCostumeIndex()
}

func (p *SpriteImpl) ResolveCostumeIndex(costume string) int {
	if isDebugInstrEnabled() {
		spxlog.Debug("ResolveCostumeIndex: sprite=%s, costume=%v", p.name, costume)
	}
	if idx := p.findCostume(SpriteCostumeName(costume)); idx >= 0 {
		return idx
	}

	idx, err := strconv.Atoi(costume)
	if err != nil {
		return -1
	}
	if len(p.costumes) == 0 {
		return idx - 1
	}
	if idx >= 0 && idx <= len(p.costumes) {
		return idx - 1
	}
	idx = (idx - 1) % len(p.costumes)
	return idx
}

func (p *SpriteImpl) SetCostume__0(costume SpriteCostumeName) {
	p.setCostume(costume)
}

func (p *SpriteImpl) SetCostume__1(index float64) {
	p.setCostume(index)
}

func (p *SpriteImpl) SetCostume__2(index int) {
	p.setCostume(index)
}

func (p *SpriteImpl) SetCostume__3(action switchAction) {
	p.setCostume(action)
}

func (p *SpriteImpl) SetGraphicEffect(kind EffectKind, val float64) {
	p.requestRedrawIfVisible()
	p.setGraphicEffect(kind, val)
}

func (p *SpriteImpl) ChangeGraphicEffect(kind EffectKind, delta float64) {
	p.requestRedrawIfVisible()
	p.changeGraphicEffect(kind, delta)
}

func (p *SpriteImpl) ClearGraphicEffects() {
	p.requestRedrawIfVisible()
	p.clearGraphicEffects()
}

func (p *SpriteImpl) SetLayer__0(layer layerAction) {
	p.SetLayerTo(layer)
}

func (p *SpriteImpl) SetLayer__1(dir dirAction, delta int) {
	switch dir {
	case Forward:
		// Saturate math.MinInt before negation to preserve Scratch's negative-input direction reversal.
		if delta == math.MinInt {
			delta = math.MaxInt
		} else {
			delta = -delta
		}
		p.g.shapeMgr.goBackLayers(p, delta)
	case Backward:
		p.g.shapeMgr.goBackLayers(p, delta)
	}
}

func (p *SpriteImpl) SetLayerTo(layer layerAction) {
	switch layer {
	case Front:
		p.g.shapeMgr.goBackLayers(p, math.MinInt32)
	case Back:
		p.g.shapeMgr.goBackLayers(p, math.MaxInt32)
	}
}

// -----------------------------------------------------------------------------
// Internal Rendering
// -----------------------------------------------------------------------------
func (p *SpriteImpl) requestRedrawIfVisible() {
	if p.Visible() {
		engine.RequestRedraw()
	}
}

func (p *SpriteImpl) setVisible(visible bool) {
	if isDebugInstrEnabled() {
		spxlog.Debug("%s visible is %t", p.name, visible)
	}

	if visible == p.spriteState.IsVisible {
		return
	}

	p.spriteState.IsVisible = visible
	engine.RequestRedraw()
	p.markProxyDirty()
}

func (p *SpriteImpl) setCostume(costume any) {
	if isDebugInstrEnabled() {
		spxlog.Debug("SetCostume: sprite=%s, costume=%v", p.name, costume)
	}
	if !p.goSetCostume(costume) {
		return
	}
	p.spriteState.DefaultCostumeIndex = p.costumeIndex
	p.markAutoPhysicsShapesDirty()
	p.markProxyDirty()
}

func (p *SpriteImpl) isFullyGhosted() bool {
	if p.greffUniforms == nil {
		return false
	}
	val, ok := p.greffUniforms[GhostEffect]
	return ok && normalizeEffectValue(GhostEffect, val) >= 1
}

func getRenderOffset(p *SpriteImpl) (float64, float64) {
	return getCostumeRenderOffset(p.currentCostume(), p.getPivot(), p.runtimeState.Scale, p.runtimeState.Scale)
}

func getCostumeRenderOffset(c *costume, pivot mathf.Vec2, scaleX, scaleY float64) (float64, float64) {
	anchor := c.renderAnchorInSPX().Add(pivot)
	return -anchor.X * scaleX, -anchor.Y * scaleY
}

func applyRenderOffset(p *SpriteImpl, cx, cy *float64) {
	x, y := getWorldRenderOffset(p)
	*cx += x
	*cy += y
}

// getWorldRenderOffset applies the sprite root's flip and rotation to the
// local render offset. Godot performs the same transform through RenderRoot.
func getWorldRenderOffset(p *SpriteImpl) (float64, float64) {
	x, y := getRenderOffset(p)
	rotation, scaleX, scaleY := getRenderRotationAndScale(p)
	x *= scaleX
	y *= scaleY

	sin, cos := math.Sincos(-engine.DegToRad(rotation))
	return x*cos - y*sin, x*sin + y*cos
}

func getRenderRotationAndScale(p *SpriteImpl) (rotation, scaleX, scaleY float64) {
	transform := p.transform()
	if transform.rotationStyle == None {
		return 0, 1.0, 1.0
	}
	cs := p.costumes[p.costumeIndex]
	rotation = p.Heading() + cs.faceRight
	rotation -= 90
	scaleX = 1.0
	scaleY = 1.0
	if transform.rotationStyle == LeftRight {
		rotation = 0
		if transform.direction < 0 {
			scaleX = -1.0
		}
	}
	return rotation, scaleX, scaleY
}

func getCostumeBoundByAlpha(p *SpriteImpl) (mathf.Vec2, mathf.Vec2) {
	cs := p.costumes[p.costumeIndex]
	var rect mathf.Rect2
	if cs.isAtlas() {
		rect = p.getCostumeAtlasRegion()
		rect.Position.X = 0
		rect.Position.Y = 0
	} else {
		if cache, ok := cachedBounds[cs.path]; ok {
			rect = cache
		} else {
			assetPath := cs.getAssetPath()
			rect = engine.Managers().ResMgr.GetBoundFromAlpha(assetPath)
		}
		cachedBounds[cs.path] = rect
	}
	scale := 1 / float64(cs.frame.BitmapResolution)
	posX := float64(rect.Position.X) * scale
	posY := float64(rect.Position.Y) * scale
	sizeX := float64(rect.Size.X) * scale
	sizeY := float64(rect.Size.Y) * scale

	w, h := p.getCostumeSize()
	offsetX := float64(posX + sizeX/2 - w/2)
	offsetY := -float64(posY + sizeY/2 - h/2)

	center := mathf.NewVec2(offsetX, offsetY)
	size := mathf.NewVec2(sizeX, sizeY)
	return center, size
}
