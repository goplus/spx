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
)

// ============================================================================
// Backdrop Types
// ============================================================================

type BackdropName = string

// ============================================================================
// Backdrop Query Methods
// ============================================================================

func (p *Game) BackdropName() string {
	return p.getCostumeName()
}

func (p *Game) BackdropIndex() int {
	return p.getCostumeIndex()
}

// ============================================================================
// Backdrop Setting Methods
// ============================================================================

// SetBackdrop func:
//
//	SetBackdrop(backdrop) or
//	SetBackdrop(index) or
//	SetBackdrop(spx.Next)
//	SetBackdrop(spx.Prev)
func (p *Game) setBackdrop(backdrop any, wait bool) {
	if p.goSetCostume(backdrop) {
		p.setupBackdrop()
		p.doWindowSize()
		p.doWhenBackdropChanged(p.getCostumeName(), wait)
	}
}

func (p *Game) SetBackdrop__0(backdrop BackdropName) {
	p.setBackdrop(backdrop, false)
}

func (p *Game) SetBackdrop__1(index float64) {
	p.setBackdrop(index, false)
}

func (p *Game) SetBackdrop__2(index int) {
	p.setBackdrop(index, false)
}

func (p *Game) SetBackdrop__3(action switchAction) {
	p.setBackdrop(action, false)
}

func (p *Game) SetBackdropAndWait__0(backdrop BackdropName) {
	p.setBackdrop(backdrop, true)
}

func (p *Game) SetBackdropAndWait__1(index float64) {
	p.setBackdrop(index, true)
}

func (p *Game) SetBackdropAndWait__2(index int) {
	p.setBackdrop(index, true)
}

func (p *Game) SetBackdropAndWait__3(action switchAction) {
	p.setBackdrop(action, true)
}

// ============================================================================
// Backdrop Setup
// ============================================================================

func (p *Game) setupBackdrop() {
	imgW, imgH := p.getCostumeSize()
	dstW := float64(p.worldWidth)
	dstH := float64(p.worldHeight)
	imgRadio := (imgW / imgH)
	worldRadio := (dstW / dstH)
	// scale image's height to fit world's height
	isScaleHeight := imgRadio > worldRadio

	switch p.mapMode {
	case mapModeRepeat:
		repeatX := dstW / imgW
		repeatY := dstH / imgH
		p.setMaterialParamsVec4("repeat_scale", mathf.Vec4{
			X: repeatX,
			Y: repeatY,
			Z: 0,
			W: 0,
		}, false)
	case mapModeFillCut:
		if isScaleHeight {
			dstH = dstW / imgRadio
		} else {
			dstW = dstH * imgRadio
		}
	case mapModeFillRatio:
		if isScaleHeight {
			dstW = dstH * imgRadio
		} else {
			dstH = dstW / imgRadio
		}
	default:
	}

	scaleX := dstW / imgW
	scaleY := dstH / imgH
	p.scale = 1
	checkUpdateCostume(&p.baseObj)
	p.engine().SpriteMgr.SetScale(p.syncSprite.GetId(), mathf.NewVec2(scaleX, scaleY))
}
