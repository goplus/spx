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
	"reflect"

	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/platform"
	spxlog "github.com/goplus/spx/v4/internal/log"
	"github.com/goplus/spx/v4/internal/ui"
)

func (p *Game) applySpriteFields(gamer reflect.Value, plan *projectPlan) {
	for i := range gamer.NumField() {
		name, sprite := p.allocSpriteField(gamer, i)
		if sprite == nil {
			continue
		}
		prepared := plan.sprites[name]
		p.loadPreparedSprite(sprite, name, gamer, &prepared)
	}
}

func (p *Game) applyPreparedStage(g reflect.Value, proj *coreproject.ProjectConfig, generation uint64, plan *projectPlan) {
	p.setupDisplayConfig(proj)
	p.setupWorldAndWindow(proj)
	p.setupPlatformAndCamera(proj)
	p.setupAudioAndTilemap(proj)
	sprites := p.applyStageEntries(g, plan)
	p.prepareStageBootstrap(sprites, proj, g, generation)
}

func (p *Game) applySpritePrototype(name string, gamer reflect.Value, plan *projectPlan) Sprite {
	if sprite := p.sprs[name]; sprite != nil {
		return sprite
	}
	prepared := plan.sprites[name]
	sprite := reflect.New(prepared.typ).Interface().(Sprite)
	p.loadPreparedSprite(sprite, name, gamer, &prepared)
	return sprite
}

// The plan validates the layout and owner before application allocates live sprites.
func (p *Game) loadPreparedSprite(sprite Sprite, name string, gamer reflect.Value, prepared *spritePlan) {
	vSpr := reflect.ValueOf(sprite).Elem()
	vSpr.Set(reflect.Zero(vSpr.Type()))
	coreproject.WritableField(vSpr.Field(spriteOwnerField)).Set(gamer.Addr())
	base := prepared.base(vSpr)
	// Paths in config are already normalized by coreproject.LoadSpriteConfig.
	config := &prepared.config
	base.initSpriteCostumes(config, prepared.layout)
	base.spriteState.DefaultCostumeIndex = base.costumeIndex
	base.scriptEventBindings.bind(&p.scriptEvents, base)

	base.g, base.name, base.sprite = p, name, sprite
	base.runtimeState.Scale = config.Size
	base.spriteState.IsVisible = config.Visible

	base.components.initComponents(base, config)
	base.initRuntimeProxy()
	p.sprs[name] = sprite
}

// -----------------------------------------------------------------------------
// Display Setup
// -----------------------------------------------------------------------------
func (p *Game) setupDisplayConfig(proj *coreproject.ProjectConfig) {
	display := coreproject.ResolveDisplaySettings(proj)
	p.displayState.WindowScale = display.WindowScale
	p.displayState.StretchMode = display.StretchMode
	p.debugState.Debug = display.Debug
	if p.debugState.Debug {
		spxlog.SetLevel(spxlog.LevelDebug)
	} else {
		spxlog.SetLevel(spxlog.LevelInfo)
	}
	engine.SetDebugMode(p.debugState.Debug)
}

func (p *Game) setupWorldAndWindow(proj *coreproject.ProjectConfig) {
	hasTilemap := p.tilemapMgr.hasData()
	proj.Map = coreproject.ResolveMapConfig(proj.Map, hasTilemap, baseScreenWidth, baseScreenHeight)

	p.displayState.WorldWidth = proj.Map.Width
	p.displayState.WorldHeight = proj.Map.Height

	if backdrops := proj.GetBackdrops(); !hasTilemap && len(backdrops) > 0 {
		p.baseObj.initBackdrops(backdrops, proj.GetBackdropIndex())
		p.doWorldSize()
	} else {
		p.baseObj.initWithSize(p.displayState.WorldWidth, p.displayState.WorldHeight)
	}
	spxlog.Debug("SetWorldSize: %d, %d", p.displayState.WorldWidth, p.displayState.WorldHeight)

	p.doWindowSize()
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

func (p *Game) setupPlatformAndCamera(proj *coreproject.ProjectConfig) {
	platformMgr := engine.Managers().PlatformMgr

	layout := coreproject.ResolvePlatformLayout(coreproject.PlatformLayoutInput{
		WindowWidth:       p.displayState.WindowWidth,
		WindowHeight:      p.displayState.WindowHeight,
		WindowScale:       p.displayState.WindowScale,
		Fullscreen:        proj.FullScreen,
		IsMobile:          platform.IsMobile(),
		IsWeb:             platform.IsWeb(),
		CurrentWindowSize: platformMgr.GetWindowSize(),
	})
	if layout.Fullscreen {
		platformMgr.SetWindowFullscreen(true)
	}
	p.displayState.WindowScale = layout.WindowScale
	platformMgr.SetWindowSize(layout.WindowWidth, layout.WindowHeight, true)
	platformMgr.SetMaxFps(int64(proj.MaxFPS))
	platformMgr.SetStretch(p.displayState.StretchMode, layout.ContentWidth, layout.ContentHeight)

	p.camera = &cameraImpl{}
	p.Camera = p.camera
	p.camera.init(p)

	isWindowMapSizeEqual := coreproject.IsWindowWorldSizeEqual(
		p.displayState.WorldWidth,
		p.displayState.WorldHeight,
		p.displayState.WindowWidth,
		p.displayState.WindowHeight,
	)
	engine.SetWindowScale(p.displayState.WindowScale)
	ui.SetBaseScreenSize(baseScreenWidth, baseScreenHeight)
	ui.ClampUIPositionInScreen(isWindowMapSizeEqual)

	p.runtimeState.SyncSprite = engine.NewBackdropProxy(p, p.getCostumePath(), p.getCostumeRenderScale())
	p.setupBackdrop()
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

// -----------------------------------------------------------------------------
// Stage Items
// -----------------------------------------------------------------------------
func (p *Game) setupAudioAndTilemap(proj *coreproject.ProjectConfig) {
	p.applyTilemap()
	p.ensureSoundObject(&p.audioState.SoundObj)
	if proj.Bgm != "" {
		p.Play__1(proj.Bgm, true)
	}
}

func (p *Game) applyTilemap() {
	p.tilemapMgr.parseTilemap()
	if p.camera != nil {
		p.camera.setLimits()
	}
	p.syncPenCanvasToWorld()
}

func (p *Game) syncPenCanvasToWorld() {
	width := int64(p.displayState.WorldWidth)
	height := int64(p.displayState.WorldHeight)
	p.penCommandBarrier(func() {
		engine.Managers().PenMgr.SetCanvasSize(width, height)
	})
}

// -----------------------------------------------------------------------------
// Bootstrap
// -----------------------------------------------------------------------------
func (p *Game) prepareStageBootstrap(sprites []Sprite, proj *coreproject.ProjectConfig, g reflect.Value, generation uint64) {
	// Bootstrap hooks may override the initial camera target.
	if proj.Camera != nil && proj.Camera.On != "" {
		p.Camera.Follow__1(proj.Camera.On)
	}
	p.queueBootstrap(generation, func() {
		p.setupCollisionData(sprites)
	})
	for _, sprite := range sprites {
		if spr := spriteOf(sprite); spr != nil {
			p.queueBootstrap(generation, spr.awake)
		}
	}
	p.queueBootstrap(generation, func() {
		// Start each Main in stage order.
		for _, sprite := range sprites {
			if spr := spriteOf(sprite); spr != nil {
				runMainUntilYield(spr.owner, sprite.Main)
			}
		}
	})
	if loader, ok := g.Addr().Interface().(interface{ OnLoaded() }); ok {
		p.queueBootstrap(generation, loader.OnLoaded)
	}
}
