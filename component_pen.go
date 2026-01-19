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
	isPenDown bool
	penObj    *engine.Object
}

// Initialize initializes the pen component
func (pc *penComponent) initialize(sprite *SpriteImpl) {
	pc.componentBase.initialize(sprite)
	pc.penColor = sprite.penColor
	pc.penWidth = sprite.penWidth
	pc.penHue = sprite.penHue
	pc.penSaturation = sprite.penSaturation
	pc.penBrightness = sprite.penBrightness
	pc.penTransparency = sprite.penTransparency
	pc.isPenDown = sprite.isPenDown
	pc.penObj = sprite.penObj
}

// cloneFrom creates a new pen component by cloning from source
func (pc *penComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	srcPen := src.(*penComponent)
	return &penComponent{
		componentBase:   componentBase{sprite: newSprite},
		penColor:        srcPen.penColor,
		penWidth:        srcPen.penWidth,
		penHue:          srcPen.penHue,
		penSaturation:   srcPen.penSaturation,
		penBrightness:   srcPen.penBrightness,
		penTransparency: srcPen.penTransparency,
		isPenDown:       srcPen.isPenDown,
		penObj:          nil, // Don't share pen object, will be created if needed
	}
}

// OnDestroy cleanup when component is destroyed
func (pc *penComponent) onDestroy() {
	pc.destroyPen()
}

// ============================================================================
// Pen Control
// ============================================================================

func (pc *penComponent) PenUp() {
	pc.checkOrCreatePen()
	pc.isPenDown = false
	penMgr.PenUp(*pc.penObj)
}

func (pc *penComponent) PenDown() {
	pc.checkOrCreatePen()
	pc.isPenDown = true
	pc.movePen(pc.sprite.x, pc.sprite.y)
	penMgr.PenDown(*pc.penObj, false)
}

func (pc *penComponent) Stamp() {
	pc.checkOrCreatePen()
	penMgr.SetPenStampTexture(*pc.penObj, pc.sprite.getCostumePath())
	penMgr.PenStamp(*pc.penObj)
}

// ============================================================================
// Pen Color Control
// ============================================================================

func (pc *penComponent) SetPenColor(color Color) {
	pc.checkOrCreatePen()
	pc.penColor = toMathfColor(color)
	pc.applyPenColorProperty()
}

func (pc *penComponent) SetPenColorParam(kind PenColorParam, value float64) {
	switch kind {
	case PenHue:
		pc.setPenHue(value)
	case PenSaturation:
		pc.setPenSaturation(value)
	case PenBrightness:
		pc.setPenBrightness(value)
	case PenTransparency:
		pc.setPenTransparency(value)
	}
}

func (pc *penComponent) ChangePenColor(kind PenColorParam, delta float64) {
	switch kind {
	case PenHue:
		pc.changePenHue(delta)
	case PenSaturation:
		pc.changePenSaturation(delta)
	case PenBrightness:
		pc.changePenBrightness(delta)
	case PenTransparency:
		pc.changePenTransparency(delta)
	}
}

// ============================================================================
// Pen HSV Color Components
// ============================================================================

func (pc *penComponent) setPenHue(value float64) {
	pc.checkOrCreatePen()
	pc.penHue = mathf.Clamp(value, 0, 100)
	pc.applyPenHsvProperty()
}

func (pc *penComponent) changePenHue(delta float64) {
	pc.setPenHue(pc.penHue + delta)
}

func (pc *penComponent) setPenSaturation(value float64) {
	pc.checkOrCreatePen()
	pc.penSaturation = mathf.Clamp(value, 0, 100)
	pc.applyPenHsvProperty()
}

func (pc *penComponent) changePenSaturation(delta float64) {
	pc.setPenSaturation(pc.penSaturation + delta)
}

func (pc *penComponent) setPenBrightness(value float64) {
	pc.checkOrCreatePen()
	pc.penBrightness = mathf.Clamp(value, 0, 100)
	pc.applyPenHsvProperty()
}

func (pc *penComponent) changePenBrightness(delta float64) {
	pc.setPenBrightness(pc.penBrightness + delta)
}

func (pc *penComponent) setPenTransparency(value float64) {
	pc.checkOrCreatePen()
	pc.penTransparency = mathf.Clamp(value, 0, 100)
	pc.applyPenHsvProperty()
}

func (pc *penComponent) changePenTransparency(delta float64) {
	pc.setPenTransparency(pc.penTransparency + delta)
}

// ============================================================================
// Pen Size Control
// ============================================================================

func (pc *penComponent) SetPenSize(size float64) {
	pc.checkOrCreatePen()
	pc.penWidth = size
	penMgr.SetPenSizeTo(*pc.penObj, size)
}

func (pc *penComponent) ChangePenSize(delta float64) {
	pc.checkOrCreatePen()
	pc.SetPenSize(pc.penWidth + delta)
}

// ============================================================================
// Internal Pen Management
// ============================================================================

func (pc *penComponent) checkOrCreatePen() {
	if pc.penObj == nil {
		obj := penMgr.CreatePen()
		pc.penObj = &obj
		pc.penTransparency = pc.penColor.A * 100
	}
}

func (pc *penComponent) destroyPen() {
	if pc.penObj != nil {
		penMgr.DestroyPen(*pc.penObj)
		pc.penObj = nil
	}
}

func (pc *penComponent) movePen(x, y float64) {
	if pc.penObj == nil {
		return
	}
	penMgr.MovePenTo(*pc.penObj, mathf.NewVec2(x, -y))
}

func (pc *penComponent) applyPenColorProperty() {
	pc.checkOrCreatePen()
	h, s, v := pc.penColor.ToHSV()
	pc.penHue = (h / 360) * 100
	pc.penSaturation = s * 100
	pc.penBrightness = v * 100
	pc.penTransparency = pc.penColor.A * 100
	penMgr.SetPenColorTo(*pc.penObj, pc.penColor)
}

func (pc *penComponent) applyPenHsvProperty() {
	color := mathf.NewColorHSV((pc.penHue/100)*360, pc.penSaturation/100, pc.penBrightness/100)
	pc.penColor = color
	pc.penColor.A = pc.penTransparency / 100
	penMgr.SetPenColorTo(*pc.penObj, pc.penColor)
}

// IsPenDown returns whether the pen is down
func (pc *penComponent) IsPenDown() bool {
	return pc.isPenDown
}
