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
	"strconv"

	spxlog "github.com/goplus/spx/v2/internal/log"
)

// -----------------------------------------------------------------------------
// Visibility
// -----------------------------------------------------------------------------
func (p *SpriteImpl) setVisible(visible bool) {
	if isDebugInstrEnabled() {
		spxlog.Debug("%s visible is %t", p.name, visible)
	}

	if visible == p.spriteState.IsVisible {
		return
	}

	p.spriteState.IsVisible = visible
	p.spriteState.IsDirty = true
}

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

// -----------------------------------------------------------------------------
// Costume
// -----------------------------------------------------------------------------
func (p *SpriteImpl) CostumeName() SpriteCostumeName {
	return p.getCostumeName()
}

func (p *SpriteImpl) CostumeIndex() int {
	return p.getCostumeIndex()
}

func (p *SpriteImpl) setCostume(costume any) {
	if isDebugInstrEnabled() {
		spxlog.Debug("SetCostume: sprite=%s, costume=%v", p.name, costume)
	}
	if !p.goSetCostume(costume) {
		return
	}
	p.spriteState.DefaultCostumeIndex = p.costumeIndex
	p.spriteState.IsDirty = true
}

func (p *SpriteImpl) SetCostumeByNameOrIndex(costume string) {
	if isDebugInstrEnabled() {
		spxlog.Debug("SetCostumeByNameOrIndex: sprite=%s, costume=%v", p.name, costume)
	}
	if idx := p.findCostume(costume); idx >= 0 {
		p.setCostumeByIndex(idx)
		return
	}

	if idx, err := strconv.Atoi(costume); err == nil {
		p.setCostumeByIndex(idx)
	}
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

// -----------------------------------------------------------------------------
// Effects
// -----------------------------------------------------------------------------
func (p *SpriteImpl) SetGraphicEffect(kind EffectKind, val float64) {
	p.setGraphicEffect(kind, val)
}

func (p *SpriteImpl) ChangeGraphicEffect(kind EffectKind, delta float64) {
	p.changeGraphicEffect(kind, delta)
}

func (p *SpriteImpl) ClearGraphicEffects() {
	p.clearGraphicEffects()
}

func (p *SpriteImpl) isFullyGhosted() bool {
	if p.greffUniforms == nil {
		return false
	}
	val, ok := p.greffUniforms[GhostEffect]
	return ok && normalizeEffectValue(GhostEffect, val) >= 1
}

// -----------------------------------------------------------------------------
// Layer
// -----------------------------------------------------------------------------
func (p *SpriteImpl) SetLayer__0(layer layerAction) {
	p.SetLayerTo(layer)
}

func (p *SpriteImpl) SetLayer__1(dir dirAction, delta int) {
	p.ChangeLayer(dir, delta)
}

func (p *SpriteImpl) SetLayerTo(layer layerAction) {
	switch layer {
	case Front:
		p.g.gotoFront(p)
	case Back:
		p.g.gotoBack(p)
	}
}

func (p *SpriteImpl) ChangeLayer(dir dirAction, delta int) {
	switch dir {
	case Forward:
		p.g.goBackLayers(p, -delta)
	case Backward:
		p.g.goBackLayers(p, delta)
	}
}
