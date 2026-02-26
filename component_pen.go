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

// ============================================================================
// Pen Component
// ============================================================================
// This component encapsulates all pen drawing functionality

type penComponent struct {
	componentBase

	// Pen properties
	penColor        mathf.Color
	penWidth        float64
	penHue          float64
	penSaturation   float64
	penBrightness   float64
	penTransparency float64

	// State
	penDown bool
	penObj  *engine.Object
}

// initialize initializes the pen component from config.
func (p *penComponent) initialize(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	p.componentBase.initialize(sprite, spriteCfg)
	// Always initialize with default pen values
	p.penColor = mathf.NewColor(66, 133, 244, 255)
	p.penWidth = 1
	p.penHue = 0.6
	p.penSaturation = 1
	p.penBrightness = 1
	p.penTransparency = 0
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
		penSaturation:   srcPen.penSaturation,
		penBrightness:   srcPen.penBrightness,
		penTransparency: srcPen.penTransparency,
		penDown:         srcPen.penDown,
		penObj:          nil, // Don't share pen object, will be created if needed
	}
}

// OnDestroy cleanup when component is destroyed
func (p *penComponent) onDestroy() {
	p.destroyPen()
}

// ============================================================================
// Pen Control
// ============================================================================

func (p *penComponent) PenUp() {
	p.checkOrCreatePen()
	p.penDown = false
	p.rt().penMgr.PenUp(*p.penObj)
}

func (p *penComponent) PenDown() {
	p.checkOrCreatePen()
	p.penDown = true
	p.movePen(p.sprite.getXY())
	p.rt().penMgr.PenDown(*p.penObj, false)
}

func (p *penComponent) Stamp() {
	p.checkOrCreatePen()
	p.rt().penMgr.SetPenStampTexture(*p.penObj, p.sprite.getCostumePath())
	p.rt().penMgr.PenStamp(*p.penObj)
}

// ============================================================================
// Pen Size Control
// ============================================================================

func (p *penComponent) SetPenSize(size float64) {
	p.checkOrCreatePen()
	p.penWidth = size
	p.rt().penMgr.SetPenSizeTo(*p.penObj, size)
}

func (p *penComponent) ChangePenSize(delta float64) {
	p.checkOrCreatePen()
	p.SetPenSize(p.penWidth + delta)
}

// ============================================================================
// Pen Color Control
// ============================================================================

func (p *penComponent) SetPenColor(color Color) {
	p.checkOrCreatePen()
	p.penColor = toMathfColor(color)
	p.applyPenColorProperty()
}

func (p *penComponent) SetPenColorParam(kind PenColorParam, value float64) {
	switch kind {
	case PenHue:
		p.setPenHue(value)
	case PenSaturation:
		p.setPenSaturation(value)
	case PenBrightness:
		p.setPenBrightness(value)
	case PenTransparency:
		p.setPenTransparency(value)
	}
}

func (p *penComponent) ChangePenColor(kind PenColorParam, delta float64) {
	switch kind {
	case PenHue:
		p.changePenHue(delta)
	case PenSaturation:
		p.changePenSaturation(delta)
	case PenBrightness:
		p.changePenBrightness(delta)
	case PenTransparency:
		p.changePenTransparency(delta)
	}
}

// ============================================================================
// Pen HSV Color Components
// ============================================================================

func (p *penComponent) setPenHue(value float64) {
	p.checkOrCreatePen()
	p.penHue = mathf.Clamp(value, 0, 100)
	p.applyPenHsvProperty()
}

func (p *penComponent) changePenHue(delta float64) {
	p.setPenHue(p.penHue + delta)
}

func (p *penComponent) setPenSaturation(value float64) {
	p.checkOrCreatePen()
	p.penSaturation = mathf.Clamp(value, 0, 100)
	p.applyPenHsvProperty()
}

func (p *penComponent) changePenSaturation(delta float64) {
	p.setPenSaturation(p.penSaturation + delta)
}

func (p *penComponent) setPenBrightness(value float64) {
	p.checkOrCreatePen()
	p.penBrightness = mathf.Clamp(value, 0, 100)
	p.applyPenHsvProperty()
}

func (p *penComponent) changePenBrightness(delta float64) {
	p.setPenBrightness(p.penBrightness + delta)
}

func (p *penComponent) setPenTransparency(value float64) {
	p.checkOrCreatePen()
	p.penTransparency = mathf.Clamp(value, 0, 100)
	p.applyPenHsvProperty()
}

func (p *penComponent) changePenTransparency(delta float64) {
	p.setPenTransparency(p.penTransparency + delta)
}

// ============================================================================
// Internal Pen Management
// ============================================================================

func (p *penComponent) checkOrCreatePen() {
	if p.penObj == nil {
		obj := p.rt().penMgr.CreatePen()
		p.penObj = &obj
		p.penTransparency = normalizedToPercent(p.penColor.A)
	}
}

func (p *penComponent) destroyPen() {
	if p.penObj != nil {
		p.rt().penMgr.DestroyPen(*p.penObj)
		p.penObj = nil
	}
}

func (p *penComponent) movePen(x, y float64) {
	if p.penObj == nil || !p.penDown {
		return
	}
	p.rt().penMgr.MovePenTo(*p.penObj, mathf.NewVec2(x, -y))
}

func (p *penComponent) applyPenColorProperty() {
	p.checkOrCreatePen()
	h, s, v := p.penColor.ToHSV()
	p.penHue = hueToPercent(h)
	p.penSaturation = normalizedToPercent(s)
	p.penBrightness = normalizedToPercent(v)
	p.penTransparency = normalizedToPercent(p.penColor.A)
	p.updatePenColor()
}

func (p *penComponent) applyPenHsvProperty() {
	p.penColor = mathf.NewColorHSV(percentToHue(p.penHue), percentToNormalized(p.penSaturation), percentToNormalized(p.penBrightness))
	p.penColor.A = percentToNormalized(p.penTransparency)
	p.updatePenColor()
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

func (p *penComponent) updatePenColor() {
	p.rt().penMgr.SetPenColorTo(*p.penObj, p.penColor)
}

func (p *penComponent) isPenDown() bool {
	return p.penDown
}
