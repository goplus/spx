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
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// -----------------------------------------------------------------------------
// Backdrop
// -----------------------------------------------------------------------------
func (p *Game) setBackdrop(backdrop any, wait bool) {
	if p.goSetBackdrop(backdrop) {
		p.setupBackdrop()
		p.doWindowSize()
		p.doWhenBackdropChanged(p.getCostumeName(), wait)
	}
}

func (p *Game) goSetBackdrop(val any) bool {
	switch v := val.(type) {
	case int:
		if Pos(v) == Random {
			return p.setRandomBackdrop()
		}
	case float64:
		if v == float64(Random) {
			return p.setRandomBackdrop()
		}
	}
	return p.goSetCostume(val)
}

func (p *Game) setRandomBackdrop() bool {
	switch len(p.costumes) {
	case 0:
		spxlog.Error("setBackdrop: no backdrops available")
		return false
	case 1:
		return true
	}

	nextIndex := randomIntn(len(p.costumes) - 1)
	if nextIndex >= p.costumeIndex {
		nextIndex++
	}
	p.setCostumeIndex(nextIndex)
	return true
}

func (p *Game) setupBackdrop() {
	imgW, imgH := p.getCostumeSize()
	spriteMgr := p.engine().SpriteMgr
	layout := coreproject.ResolveBackdropLayout(
		imgW,
		imgH,
		float64(p.displayState.WorldWidth),
		float64(p.displayState.WorldHeight),
		p.displayState.MapMode,
	)
	if layout.RepeatScale != nil {
		p.setMaterialParamsVec4("repeat_scale", *layout.RepeatScale, false)
	}
	p.runtimeState.Scale = 1
	p.baseObj.scheduleCostumeUpdate()

	if p.displayState.MapMode == coreproject.MapModeActualSize {
		x, y := getCostumeRenderOffset(p.currentCostume(), p.currentCostume().pivot, layout.ScaleX, layout.ScaleY)
		spriteMgr.SetPosition(
			p.runtimeState.SyncSprite.GetId(),
			mathf.NewVec2(x, y),
		)
	}
	spriteMgr.SetScale(p.runtimeState.SyncSprite.GetId(), mathf.NewVec2(layout.ScaleX, layout.ScaleY))
}

// -----------------------------------------------------------------------------
// Display Size
// -----------------------------------------------------------------------------
func (p *Game) getWindowSize() mathf.Vec2 {
	x, y := p.windowSize()
	return mathf.NewVec2(float64(x), float64(y))
}

func (p *Game) windowSize() (int, int) {
	if p.displayState.WindowWidth == 0 {
		p.doWindowSize()
	}
	return p.displayState.WindowWidth, p.displayState.WindowHeight
}

func (p *Game) doWindowSize() {
	if p.displayState.WindowWidth == 0 {
		c := p.costumes[p.costumeIndex]
		p.displayState.WindowWidth, p.displayState.WindowHeight = c.getSize()
	}
}

func (p *Game) worldSize() (int, int) {
	if p.displayState.WorldWidth == 0 {
		p.doWorldSize()
	}
	return p.displayState.WorldWidth, p.displayState.WorldHeight
}

func (p *Game) worldBounds() (left, top, right, bottom int) {
	worldW, worldH := p.worldSize()
	left = p.displayState.MinWorldX
	right = left + worldW
	top = -p.displayState.MinWorldY
	bottom = top - worldH
	return
}

func (p *Game) doWorldSize() {
	if p.displayState.WorldWidth == 0 {
		c := p.costumes[p.costumeIndex]
		p.displayState.WorldWidth, p.displayState.WorldHeight = c.getSize()
	}
}
