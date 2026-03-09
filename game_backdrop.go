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
	coreproject "github.com/goplus/spx/v2/internal/core/project"
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
	layout := coreproject.ResolveBackdropLayout(
		imgW,
		imgH,
		float64(p.WorldWidth),
		float64(p.WorldHeight),
		p.MapMode,
	)
	if layout.RepeatScale != nil {
		p.setMaterialParamsVec4("repeat_scale", *layout.RepeatScale, false)
	}
	p.Scale = 1
	checkUpdateCostume(&p.baseObj)
	p.engine().SpriteMgr.SetScale(p.SyncSprite.GetId(), mathf.NewVec2(layout.ScaleX, layout.ScaleY))
}
