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

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	engine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

// ============================================================================
// Pen Component
// ============================================================================
// This component encapsulates all pen drawing functionality.

type penComponent struct {
	componentBase

	// Pen properties
	penColor        mathf.Color
	penWidth        float64
	penHue          float64
	legacyPenColor  scratchLegacyPenState
	penSaturation   float64
	penBrightness   float64
	penTransparency float64

	// State
	penDown bool
	penObj  *engine.Object
}

// ============================================================================
// Lifecycle
// ============================================================================

// initialize initializes the pen component from config.
func (p *penComponent) initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	p.componentBase.initialize(sprite, spriteCfg)
	// Always initialize with default pen values
	p.penColor = mathf.NewColorRGBAi(66, 133, 244, 255)
	p.penWidth = 1
	p.syncPenColorComponents()
	p.legacyPenColor = newScratchLegacyPenState()
	p.penDown = false
	p.penObj = nil
}

// cloneFrom creates a new pen component by cloning from source.
func (p *penComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	srcPen := src.(*penComponent)
	return &penComponent{
		componentBase:   componentBase{sprite: newSprite},
		penColor:        srcPen.penColor,
		penWidth:        srcPen.penWidth,
		penHue:          srcPen.penHue,
		legacyPenColor:  srcPen.legacyPenColor,
		penSaturation:   srcPen.penSaturation,
		penBrightness:   srcPen.penBrightness,
		penTransparency: srcPen.penTransparency,
		penDown:         srcPen.penDown,
		penObj:          nil, // Don't share pen object, will be created if needed
	}
}

// onDestroy cleans up when the component is destroyed.
func (p *penComponent) onDestroy() {
	p.destroyPen()
}

// ============================================================================
// Pen Control
// ============================================================================

func (p *penComponent) PenUp() {
	if !p.penDown {
		return
	}
	p.penDown = false
	if p.penObj == nil {
		return
	}
	p.sprite.g.queuePenUp(*p.penObj)
}

func (p *penComponent) PenDown() {
	wasDown := p.penDown
	created := p.checkOrCreatePen()
	if !wasDown || created {
		p.syncPenAppearance()
	}
	x, y := p.sprite.getXY()
	p.syncPenPosition(x, y)
	p.penDown = true
	p.sprite.g.queuePenDown(*p.penObj, false)
}

func (p *penComponent) Stamp() {
	p.checkOrCreatePen()
	x, y := p.sprite.getXY()
	applyRenderOffset(p.sprite, &x, &y)
	rotationRadians, scale := p.getPenStampTransform()
	obj := *p.penObj
	texturePath := p.sprite.getCostumeAssetPath()
	p.sprite.g.penCommandBarrier(func() {
		p.engine().PenMgr.PenStampWithTransform(
			obj,
			texturePath,
			mathf.NewVec2(x, y),
			rotationRadians,
			scale,
		)
	})
}

// ============================================================================
// Pen Size Control
// ============================================================================

func (p *penComponent) SetPenSize(size float64) {
	if p.penObj != nil && nearlyEqualPenValue(p.penWidth, size) {
		return
	}
	p.ensureClonePenReady()
	p.penWidth = size
	p.sprite.g.queuePenSize(*p.penObj, size)
}

func (p *penComponent) ChangePenSize(delta float64) {
	p.SetPenSize(p.penWidth + delta)
}

// ============================================================================
// Pen Color Control
// ============================================================================

func (p *penComponent) SetPenColor(color Color) {
	nextColor := toMathfColor(color)
	if p.penObj != nil && samePenColor(p.penColor, nextColor) {
		return
	}
	p.penColor = nextColor
	p.ensureClonePenReady()
	p.applyPenColorProperty()
	p.syncLegacyPenStateFromColor()
}

func (p *penComponent) SetPenColorParam(kind PenColorParam, value float64) {
	switch kind {
	case PenHue:
		p.setPenHueParam(value)
	case PenSaturation:
		p.setPenSaturation(value)
	case PenBrightness:
		p.setPenBrightness(value)
	case PenTransparency:
		p.setPenTransparency(value)
	case PenNone:
		return
	}
}

func (p *penComponent) ChangePenColor(kind PenColorParam, delta float64) {
	switch kind {
	case PenHue:
		p.changePenHueParam(delta)
	case PenSaturation:
		p.changePenSaturation(delta)
	case PenBrightness:
		p.changePenBrightness(delta)
	case PenTransparency:
		p.changePenTransparency(delta)
	case PenNone:
		return
	}
}

func (p *penComponent) SetPenShade(value float64) {
	nextValue := wrapScratchLegacyPenShade(value)
	if p.penObj != nil && nearlyEqualPenValue(p.legacyPenColor.shade, nextValue) {
		return
	}
	p.updateLegacyPenState(p.legacyPenColor.withShade(nextValue), false)
}

func (p *penComponent) ChangePenShade(delta float64) {
	p.SetPenShade(p.legacyPenColor.shade + delta)
}

func (p *penComponent) setPenHue(value float64) {
	nextValue := wrapScratchPenColorPercent(value / 2)
	if p.penObj != nil &&
		nearlyEqualPenValue(p.legacyPenColor.hue, nextValue) &&
		nearlyEqualPenValue(p.penTransparency, 0) {
		return
	}
	p.updateLegacyPenState(p.legacyPenColor.withHue(nextValue), true)
}

func (p *penComponent) changePenHue(delta float64) {
	nextValue := wrapScratchPenColorPercent(p.legacyPenColor.hue + delta/2)
	if p.penObj != nil && nearlyEqualPenValue(p.legacyPenColor.hue, nextValue) {
		return
	}
	p.updateLegacyPenState(p.legacyPenColor.withHue(nextValue), false)
}

// ============================================================================
// Pen HSV Color Components
// ============================================================================

func (p *penComponent) setPenHueParam(value float64) {
	nextValue := wrapScratchPenColorPercent(value)
	if p.penObj != nil && nearlyEqualPenValue(p.penHue, nextValue) {
		return
	}
	p.ensureClonePenReady()
	p.penHue = nextValue
	p.legacyPenColor.hue = p.penHue
	p.applyPenHsvProperty()
}

func (p *penComponent) changePenHueParam(delta float64) {
	p.setPenHueParam(p.penHue + delta)
}

func (p *penComponent) setPenSaturation(value float64) {
	p.setPenHsvComponent(&p.penSaturation, value)
}

func (p *penComponent) changePenSaturation(delta float64) {
	p.setPenSaturation(p.penSaturation + delta)
}

func (p *penComponent) setPenBrightness(value float64) {
	p.setPenHsvComponent(&p.penBrightness, value)
}

func (p *penComponent) changePenBrightness(delta float64) {
	p.setPenBrightness(p.penBrightness + delta)
}

func (p *penComponent) setPenTransparency(value float64) {
	p.setPenHsvComponent(&p.penTransparency, value)
}

func (p *penComponent) changePenTransparency(delta float64) {
	p.setPenTransparency(p.penTransparency + delta)
}

// ============================================================================
// Internal Pen Management
// ============================================================================

func (p *penComponent) checkOrCreatePen() bool {
	if p.penObj == nil {
		obj := p.engine().PenMgr.CreatePen()
		p.penObj = &obj
		p.penTransparency = alphaToTransparency(p.penColor.A)
		return true
	}
	return false
}

func (p *penComponent) destroyPen() {
	if p.penObj != nil {
		obj := *p.penObj
		p.sprite.g.penCommandBarrier(func() {
			p.engine().PenMgr.DestroyPen(obj)
		})
		p.penObj = nil
	}
}

func (p *penComponent) movePen(x, y float64) {
	if !p.penDown {
		return
	}
	p.ensureClonePenReady()
	p.syncPenPosition(x, y)
}

func (p *penComponent) syncPenPosition(x, y float64) {
	if p.penObj == nil {
		return
	}
	p.sprite.g.queuePenMove(*p.penObj, mathf.NewVec2(x, y))
}

func (p *penComponent) getPenStampTransform() (rotationRadians float64, scale mathf.Vec2) {
	rotation, scaleX, scaleY := getRenderRotationAndScale(p.sprite)
	renderScale := p.sprite.getCostumeRenderScale()
	return engine.DegToRad(rotation), mathf.NewVec2(scaleX*renderScale, scaleY*renderScale)
}

func (p *penComponent) applyPenColorProperty() {
	p.checkOrCreatePen()
	p.syncPenColorComponents()
	p.updatePenColor()
}

func (p *penComponent) syncPenColorComponents() {
	h, s, v := p.penColor.ToHSV()
	p.penHue = hueToPercent(h)
	p.penSaturation = normalizedToPercent(s)
	p.penBrightness = normalizedToPercent(v)
	p.penTransparency = alphaToTransparency(p.penColor.A)
}

func (p *penComponent) applyPenHsvProperty() {
	p.penColor = mathf.NewColorHSV(percentToHue(p.penHue), percentToNormalized(p.penSaturation), percentToNormalized(p.penBrightness))
	p.penColor.A = transparencyToAlpha(p.penTransparency)
	p.updatePenColor()
}

func (p *penComponent) applyLegacyPenColor() {
	p.penColor = p.legacyPenColor.color(p.penTransparency)
	p.syncPenColorComponents()
	p.legacyPenColor.hue = p.penHue
	p.updatePenColor()
}

func (p *penComponent) setPenHsvComponent(component *float64, value float64) {
	nextValue := mathf.Clamp(value, 0, 100)
	if p.penObj != nil && nearlyEqualPenValue(*component, nextValue) {
		return
	}
	p.ensureClonePenReady()
	*component = nextValue
	p.applyPenHsvProperty()
}

func (p *penComponent) syncPenAppearance() {
	p.sprite.g.queuePenSize(*p.penObj, p.penWidth)
	p.sprite.g.queuePenColor(*p.penObj, p.penColor)
}

func (p *penComponent) ensureClonePenReady() {
	created := p.checkOrCreatePen()
	if !created || !p.penDown {
		return
	}
	p.syncPenAppearance()
	x, y := p.sprite.getXY()
	p.syncPenPosition(x, y)
	p.sprite.g.queuePenDown(*p.penObj, false)
}

func (p *penComponent) syncLegacyPenStateFromColor() {
	p.legacyPenColor.syncFromHSV(p.penHue, p.penBrightness)
}

func (p *penComponent) updateLegacyPenState(state scratchLegacyPenState, resetTransparency bool) {
	p.ensureClonePenReady()
	p.legacyPenColor = state
	if resetTransparency {
		p.penTransparency = 0
	}
	p.applyLegacyPenColor()
}

func (p *penComponent) updatePenColor() {
	p.sprite.g.queuePenColor(*p.penObj, p.penColor)
}

func hueToPercent(hue float64) float64 {
	return (hue / 360) * 100
}

func percentToHue(percent float64) float64 {
	return (percent / 100) * 360
}

func normalizedToPercent(normalized float64) float64 {
	return normalized * 100
}

func percentToNormalized(percent float64) float64 {
	return percent / 100
}

func alphaToTransparency(alpha float64) float64 {
	return normalizedToPercent(1 - alpha)
}

func transparencyToAlpha(transparency float64) float64 {
	return 1 - percentToNormalized(transparency)
}

// scratchLegacyPenState models Scratch 2's pen hue/shade pair, which is
// distinct from the HSV pen color params exposed elsewhere in the engine.
type scratchLegacyPenState struct {
	hue   float64
	shade float64
}

func newScratchLegacyPenState() scratchLegacyPenState {
	return scratchLegacyPenState{
		hue:   scratchLegacyDefaultPenHue,
		shade: scratchLegacyDefaultPenShade,
	}
}

func (s scratchLegacyPenState) withHue(hue float64) scratchLegacyPenState {
	s.hue = hue
	return s
}

func (s scratchLegacyPenState) withShade(shade float64) scratchLegacyPenState {
	s.shade = shade
	return s
}

func (s *scratchLegacyPenState) syncFromHSV(hue, brightness float64) {
	s.hue = hue
	s.shade = brightness / 2
}

func (s scratchLegacyPenState) color(transparency float64) mathf.Color {
	return makeScratchLegacyPenColor(s.hue, s.shade, transparency)
}

func makeScratchLegacyPenColor(hue, shade, transparency float64) mathf.Color {
	baseColor := mathf.NewColorHSV(percentToHue(hue), 1, 1)
	normalizedShade := shade
	if normalizedShade > 100 {
		normalizedShade = 200 - normalizedShade
	}
	if normalizedShade < 50 {
		baseColor = mathf.ColorBlack().Lerp(baseColor, (10+normalizedShade)/60)
	} else {
		baseColor = baseColor.Lerp(mathf.ColorWhite(), (normalizedShade-50)/60)
	}
	baseColor.A = transparencyToAlpha(transparency)
	return baseColor
}

func wrapScratchLegacyPenShade(shade float64) float64 {
	shade = math.Mod(shade, 200)
	if shade < 0 {
		shade += 200
	}
	return shade
}

func wrapScratchPenColorPercent(value float64) float64 {
	value = math.Mod(value, 100)
	if value < 0 {
		value += 100
	}
	return value
}

const (
	scratchLegacyDefaultPenHue   = 66.66
	scratchLegacyDefaultPenShade = 50
)

func nearlyEqualPenValue(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}

func samePenColor(a, b mathf.Color) bool {
	return nearlyEqualPenValue(a.R, b.R) &&
		nearlyEqualPenValue(a.G, b.G) &&
		nearlyEqualPenValue(a.B, b.B) &&
		nearlyEqualPenValue(a.A, b.A)
}
