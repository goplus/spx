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
	"github.com/goplus/spx/v4/internal/core/scratch"
	"github.com/goplus/spx/v4/internal/engine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

// Color.
type PenColorParam int

const (
	PenHue          PenColorParam = scratch.PenHue
	PenSaturation   PenColorParam = scratch.PenSaturation
	PenBrightness   PenColorParam = scratch.PenBrightness
	PenTransparency PenColorParam = scratch.PenTransparency
	PenNone         PenColorParam = scratch.PenNone
)

// legacyPenState models Scratch 2's pen hue/shade pair, which is
// distinct from the HSV pen color params exposed elsewhere in the engine.
type legacyPenState struct {
	hue   float64
	shade float64
}

const (
	legacyDefaultPenHue   = 66.66
	legacyDefaultPenShade = 50
)

// penState is copied by value; drawing resources remain component-local.
type penState struct {
	penColor        mathf.Color
	penWidth        float64
	penHue          float64
	legacyPenColor  legacyPenState
	penSaturation   float64
	penBrightness   float64
	penTransparency float64
	isPenDown       bool
}

type penComponent struct {
	sprite *SpriteImpl
	penState
	penObj *gdx.Object
	// Dragging must not draw a connecting stroke.
	needsPositionReset bool
}

func (p *Game) EraseAll() {
	engine.RequestRedraw()
	p.penCommandBarrier(engine.Managers().PenMgr.DestroyAllPens)
}

func (p *Game) queuePenMove(obj engine.Object, position mathf.Vec2) {
	if p.penSyncBuffer == nil {
		engine.Managers().PenMgr.MovePenTo(obj, position)
		return
	}
	if p.penSyncBuffer.AddMove(obj, position.X, position.Y) {
		p.flushPenCommands()
	}
}

func (p *Game) queuePenRestoreState(obj engine.Object, position mathf.Vec2, isDown bool) {
	if p.penSyncBuffer == nil {
		engine.Managers().PenMgr.RestorePenState(obj, position, isDown)
		return
	}
	if p.penSyncBuffer.AddRestoreState(obj, position.X, position.Y, isDown) {
		p.flushPenCommands()
	}
}

func (p *Game) queuePenDown(obj engine.Object) {
	if p.penSyncBuffer == nil {
		engine.Managers().PenMgr.PenDown(obj, false)
		return
	}
	if p.penSyncBuffer.AddDown(obj, false) {
		p.flushPenCommands()
	}
}

func (p *Game) queuePenUp(obj engine.Object) {
	if p.penSyncBuffer == nil {
		engine.Managers().PenMgr.PenUp(obj)
		return
	}
	if p.penSyncBuffer.AddUp(obj) {
		p.flushPenCommands()
	}
}

func (p *Game) queuePenColor(obj engine.Object, color mathf.Color) {
	if p.penSyncBuffer == nil {
		engine.Managers().PenMgr.SetPenColorTo(obj, color)
		return
	}
	if p.penSyncBuffer.AddColor(obj, color.R, color.G, color.B, color.A) {
		p.flushPenCommands()
	}
}

func (p *Game) queuePenSize(obj engine.Object, size float64) {
	if p.penSyncBuffer == nil {
		engine.Managers().PenMgr.SetPenSizeTo(obj, size)
		return
	}
	if p.penSyncBuffer.AddSetSize(obj, size) {
		p.flushPenCommands()
	}
}

func (p *Game) flushPenCommands() {
	if p.penSyncBuffer == nil {
		return
	}
	p.penSyncBuffer.Flush(engine.Managers().PenMgr.BatchUpdateCommands)
}

func (p *Game) discardPenCommands() {
	if p.penSyncBuffer != nil {
		p.penSyncBuffer.Discard()
	}
}

func (p *Game) penCommandBarrier(operation func()) {
	if p.penSyncBuffer == nil {
		operation()
		return
	}
	p.penSyncBuffer.Barrier(engine.Managers().PenMgr.BatchUpdateCommands, operation)
}

// Pen.
func (p *SpriteImpl) PenUp() {
	p.pen().penUp()
}

func (p *SpriteImpl) PenDown() {
	p.pen().penDown()
}

func (p *SpriteImpl) Stamp() {
	p.pen().stamp()
}

// Color Control.
func (p *SpriteImpl) SetPenColor__0(color Color) {
	p.pen().setPenColor(color)
}

func (p *SpriteImpl) SetPenColor__1(kind PenColorParam, value float64) {
	p.pen().setPenColorParam(kind, value)
}

func (p *SpriteImpl) ChangePenColor(kind PenColorParam, delta float64) {
	p.pen().changePenColor(kind, delta)
}

func (p *SpriteImpl) SetPenHue(value float64) {
	p.pen().setPenHue(value)
}

func (p *SpriteImpl) ChangePenHue(delta float64) {
	p.pen().changePenHue(delta)
}

func (p *SpriteImpl) SetPenShade(value float64) {
	p.pen().setPenShade(value)
}

func (p *SpriteImpl) ChangePenShade(delta float64) {
	p.pen().changePenShade(delta)
}

// Size.
func (p *SpriteImpl) SetPenSize(size float64) {
	p.pen().setPenSize(size)
}

func (p *SpriteImpl) ChangePenSize(delta float64) {
	p.pen().changePenSize(delta)
}

// Internals.
func (p *SpriteImpl) movePen(x, y float64) {
	p.pen().movePen(x, y)
}

func (p *penComponent) initialize(sprite *SpriteImpl) {
	p.sprite = sprite
	p.penColor = mathf.NewColorRGBAi(66, 133, 244, 255)
	p.penWidth = 1
	p.syncPenColorComponents()
	p.legacyPenColor = legacyPenState{hue: legacyDefaultPenHue, shade: legacyDefaultPenShade}
}

// cloneFor creates a new pen component for newSprite without sharing drawing resources.
func (p *penComponent) cloneFor(newSprite *SpriteImpl) *penComponent {
	return &penComponent{
		sprite:   newSprite,
		penState: p.penState,
	}
}

func (p *penComponent) onDestroy() {
	p.destroyPen()
}

func (p *penComponent) penUp() {
	if !p.isPenDown {
		return
	}
	p.isPenDown = false
	p.needsPositionReset = false
	if p.penObj == nil {
		return
	}
	p.sprite.g.queuePenUp(*p.penObj)
}

func (p *penComponent) penDown() {
	engine.RequestRedraw()
	if p.checkOrCreatePen() || !p.isPenDown {
		p.syncPenAppearance()
	}
	if p.needsPositionReset {
		p.restorePenState()
	} else {
		x, y := p.sprite.getXY()
		p.syncPenPosition(x, y)
	}
	p.isPenDown = true
	p.sprite.g.queuePenDown(*p.penObj)
}

func (p *penComponent) stamp() {
	if p.sprite.isDestroyed() || p.sprite.runtimeState.proxy == nil {
		return
	}
	engine.RequestRedraw()
	p.sprite.g.penCommandBarrier(func() {
		p.sprite.syncProxyForQuery()
		engine.Managers().PenMgr.PenStampSprite(p.sprite.runtimeState.proxy.Id)
	})
}

func (p *penComponent) setPenSize(size float64) {
	if p.penObj != nil && nearlyEqualPenValue(p.penWidth, size) {
		return
	}
	p.ensurePenReady()
	p.penWidth = size
	p.sprite.g.queuePenSize(*p.penObj, size)
}

func (p *penComponent) changePenSize(delta float64) {
	p.setPenSize(p.penWidth + delta)
}

func (p *penComponent) setPenColor(color Color) {
	nextColor := toMathfColor(color)
	if p.penObj != nil && samePenColor(p.penColor, nextColor) {
		return
	}
	p.penColor = nextColor
	p.ensurePenReady()
	p.syncPenColorComponents()
	p.updatePenColor()
	p.legacyPenColor = legacyPenState{hue: p.penHue, shade: p.penBrightness / 2}
}

func (p *penComponent) setPenHue(value float64) {
	nextValue := scratch.WrapPenHue(value / 2)
	if p.penObj != nil &&
		nearlyEqualPenValue(p.legacyPenColor.hue, nextValue) &&
		nearlyEqualPenValue(p.penTransparency, 0) {
		return
	}
	p.updateLegacyPenState(legacyPenState{hue: nextValue, shade: p.legacyPenColor.shade}, true)
}

func (p *penComponent) changePenHue(delta float64) {
	nextValue := scratch.WrapPenHue(p.legacyPenColor.hue + delta/2)
	if p.penObj != nil && nearlyEqualPenValue(p.legacyPenColor.hue, nextValue) {
		return
	}
	p.updateLegacyPenState(legacyPenState{hue: nextValue, shade: p.legacyPenColor.shade}, false)
}

func (p *penComponent) setPenShade(value float64) {
	nextValue := scratch.WrapLegacyPenShade(value)
	if p.penObj != nil && nearlyEqualPenValue(p.legacyPenColor.shade, nextValue) {
		return
	}
	p.updateLegacyPenState(legacyPenState{hue: p.legacyPenColor.hue, shade: nextValue}, false)
}

func (p *penComponent) changePenShade(delta float64) {
	p.setPenShade(p.legacyPenColor.shade + delta)
}

func (p *penComponent) setPenColorParam(kind PenColorParam, value float64) {
	component := p.colorParam(kind)
	if component == nil {
		return
	}
	if kind == PenHue {
		value = scratch.WrapPenHue(value)
	} else {
		value = mathf.Clamp(value, 0, 100)
	}
	if p.penObj != nil && nearlyEqualPenValue(*component, value) {
		return
	}
	p.ensurePenReady()
	*component = value
	if kind == PenHue {
		p.legacyPenColor.hue = value
	}
	p.applyPenHsvProperty()
}

func (p *penComponent) changePenColor(kind PenColorParam, delta float64) {
	if component := p.colorParam(kind); component != nil {
		p.setPenColorParam(kind, *component+delta)
	}
}

func (p *penComponent) colorParam(kind PenColorParam) *float64 {
	switch kind {
	case PenHue:
		return &p.penHue
	case PenSaturation:
		return &p.penSaturation
	case PenBrightness:
		return &p.penBrightness
	case PenTransparency:
		return &p.penTransparency
	default:
		return nil
	}
}

func (p *penComponent) checkOrCreatePen() bool {
	if p.penObj == nil {
		obj := engine.Managers().PenMgr.CreatePen()
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
			engine.Managers().PenMgr.DestroyPen(obj)
		})
		p.penObj = nil
	}
}

func (p *penComponent) movePen(x, y float64) {
	if !p.isPenDown {
		return
	}
	engine.RequestRedraw()
	p.ensurePenReady()
	if p.needsPositionReset {
		p.restorePenState()
	}
	p.syncPenPosition(x, y)
}

// Restore the pen origin and state without drawing a point.
func (p *penComponent) restorePenState() {
	x, y := p.sprite.getXY()
	p.sprite.g.queuePenRestoreState(*p.penObj, mathf.NewVec2(x, y), p.isPenDown)
	p.needsPositionReset = false
}

func (p *penComponent) syncPenPosition(x, y float64) {
	if p.penObj == nil {
		return
	}
	p.sprite.g.queuePenMove(*p.penObj, mathf.NewVec2(x, y))
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
	p.penColor = scratch.LegacyPenColor(p.legacyPenColor.hue, p.legacyPenColor.shade, p.penTransparency)
	p.syncPenColorComponents()
	p.legacyPenColor.hue = p.penHue
	p.updatePenColor()
}

func (p *penComponent) syncPenAppearance() {
	p.sprite.g.queuePenSize(*p.penObj, p.penWidth)
	p.sprite.g.queuePenColor(*p.penObj, p.penColor)
}

func (p *penComponent) ensurePenReady() {
	if !p.checkOrCreatePen() || !p.isPenDown {
		return
	}
	p.syncPenAppearance()
	p.restorePenState()
}

func (p *penComponent) updateLegacyPenState(state legacyPenState, resetTransparency bool) {
	p.ensurePenReady()
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

func nearlyEqualPenValue(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}

func samePenColor(a, b mathf.Color) bool {
	return nearlyEqualPenValue(a.R, b.R) &&
		nearlyEqualPenValue(a.G, b.G) &&
		nearlyEqualPenValue(a.B, b.B) &&
		nearlyEqualPenValue(a.A, b.A)
}
