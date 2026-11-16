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
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
)

// Stage Display.
func (p *Game) BackdropName() string {
	return p.getCostumeName()
}

func (p *Game) BackdropIndex() int {
	return p.getCostumeIndex()
}

func (p *Game) SetBackdrop__0(name BackdropName) {
	p.setBackdrop(name, false)
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

func (p *Game) SetBackdropAndWait__0(name BackdropName) {
	p.setBackdrop(name, true)
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

func (p *Game) SetWindowSize(width int64, height int64) {
	engine.Managers().PlatformMgr.SetWindowSize(width, height, false)
}

func (p *Game) SetGraphicEffect(kind EffectKind, value float64) {
	engine.RequestRedraw()
	p.visualObject.setGraphicEffect(kind, value)
}

func (p *Game) ChangeGraphicEffect(kind EffectKind, value float64) {
	engine.RequestRedraw()
	p.visualObject.changeGraphicEffect(kind, value)
}

func (p *Game) ClearGraphicEffects() {
	engine.RequestRedraw()
	p.visualObject.clearGraphicEffects()
}

func (p *Game) setupWorldAndWindow(proj *coreproject.ProjectConfig) {
	hasTilemap := p.tilemapMgr.hasData()
	proj.Map = coreproject.ResolveMapConfig(proj.Map, hasTilemap, baseScreenWidth, baseScreenHeight)

	p.displayState.WorldWidth = proj.Map.Width
	p.displayState.WorldHeight = proj.Map.Height

	if backdrops := proj.GetBackdrops(); !hasTilemap && len(backdrops) > 0 {
		p.visualObject.initBackdrops(backdrops, proj.GetBackdropIndex())
		p.ensureWorldSize()
	} else {
		p.visualObject.initWithSize(p.displayState.WorldWidth, p.displayState.WorldHeight)
	}
	spxlog.Debug("SetWorldSize: %d, %d", p.displayState.WorldWidth, p.displayState.WorldHeight)

	p.ensureWindowSize()
	metrics := coreproject.ResolveWorldWindowMetrics(
		p.displayState.WorldWidth,
		p.displayState.WorldHeight,
		p.displayState.WindowWidth,
		p.displayState.WindowHeight,
		coreproject.ToMapMode(proj.Map.Mode),
	)
	p.applyWorldWindowMetrics(metrics)
	spxlog.Debug("SetWindowSize: %d, %d", p.displayState.WindowWidth, p.displayState.WindowHeight)
}

func (p *Game) applyWorldWindowMetrics(metrics coreproject.WorldWindowMetrics) {
	p.displayState.WorldWidth = metrics.WorldWidth
	p.displayState.WorldHeight = metrics.WorldHeight
	p.displayState.MinWorldX = metrics.MinWorldX
	p.displayState.MinWorldY = metrics.MinWorldY
	p.displayState.MapMode = metrics.MapMode
	p.displayState.WindowWidth = metrics.WindowWidth
	p.displayState.WindowHeight = metrics.WindowHeight
}

// Backdrop.
func (p *Game) setBackdrop(backdrop any, wait bool) {
	if !p.selectBackdrop(backdrop) {
		return
	}
	engine.RequestRedraw()
	p.applyBackdropLayout()
	p.ensureWindowSize()
	p.doWhenBackdropChanged(p.getCostumeName(), wait)
}

func (p *Game) selectBackdrop(val any) bool {
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

func (p *Game) applyBackdropLayout() {
	imgW, imgH := p.getCostumeSize()
	spriteMgr := engine.Managers().SpriteMgr
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
	p.runtimeState.scale = 1
	engine.WaitMainThread(func() {
		p.applyLayerUpdate()
		p.applyCostumeTexture()
	})

	if p.displayState.MapMode == coreproject.MapModeActualSize {
		costume := p.currentCostume()
		x, y := getCostumeRenderOffset(costume, costume.pivot, layout.ScaleX, layout.ScaleY)
		spriteMgr.SetPosition(
			p.runtimeState.proxy.GetId(),
			mathf.NewVec2(x, y),
		)
	}
	spriteMgr.SetScale(p.runtimeState.proxy.GetId(), mathf.NewVec2(layout.ScaleX, layout.ScaleY))
}

func (p *Game) windowSize() (int, int) {
	p.ensureWindowSize()
	return p.displayState.WindowWidth, p.displayState.WindowHeight
}

func (p *Game) ensureWindowSize() {
	if p.displayState.WindowWidth == 0 {
		p.displayState.WindowWidth, p.displayState.WindowHeight = p.currentCostume().displaySize()
	}
}

func (p *Game) worldSize() (int, int) {
	p.ensureWorldSize()
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

func (p *Game) ensureWorldSize() {
	if p.displayState.WorldWidth == 0 {
		p.displayState.WorldWidth, p.displayState.WorldHeight = p.currentCostume().displaySize()
	}
}
