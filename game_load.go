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
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"unsafe"

	"github.com/goplus/spx/v3/internal/base/defaults"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/engine/platform"
	spxlog "github.com/goplus/spx/v3/internal/log"
	"github.com/goplus/spx/v3/internal/ui"
)

type spriteLoader func(sprite Sprite, name string, gamer reflect.Value) error

type preparedSprite struct {
	config coreproject.SpriteConfig
	layout *coreproject.CostumeLayout
}

func (p *Game) loadSprite(sprite Sprite, name string, gamer reflect.Value) error {
	spxlog.Debug("LoadSprite: %s", name)
	config, err := coreproject.LoadSpriteConfig(p.fs, name)
	if err != nil {
		return err
	}
	prepared, err := prepareSpriteConfig(&config)
	if err != nil {
		return fmt.Errorf("sprite config %q: %w", name, err)
	}
	return p.loadPreparedSprite(sprite, name, gamer, &prepared)
}

func prepareSpriteConfig(config *coreproject.SpriteConfig) (preparedSprite, error) {
	layout, err := coreproject.PrepareCostumeLayout(config)
	if err != nil {
		return preparedSprite{}, err
	}
	preparedAnimations, err := prepareFrameAnimations(config.FAnimations, layout)
	if err != nil {
		return preparedSprite{}, err
	}
	preparedConfig := *config
	preparedConfig.FAnimations = preparedAnimations
	return preparedSprite{config: preparedConfig, layout: layout}, nil
}

func prepareFrameAnimations(animations map[string]*coreproject.AniConfig, layout *coreproject.CostumeLayout) (map[string]*coreproject.AniConfig, error) {
	if animations == nil {
		return nil, nil
	}
	prepared := make(map[string]*coreproject.AniConfig, len(animations))
	for _, name := range slices.Sorted(maps.Keys(animations)) {
		animation := animations[name]
		if animation == nil {
			return nil, fmt.Errorf("fAnimations[%q] is null", name)
		}
		frameFrom, err := prepareAnimationFrame(name, "frameFrom", animation.FrameFrom, layout)
		if err != nil {
			return nil, err
		}
		frameTo, err := prepareAnimationFrame(name, "frameTo", animation.FrameTo, layout)
		if err != nil {
			return nil, err
		}

		definition := *animation
		defaults.SetDefaultIfZero(&definition.FrameFps, 25)
		defaults.SetDefaultIfZero(&definition.TurnToDuration, 1.0)
		defaults.SetDefaultIfZero(&definition.StepDuration, 0.01)
		definition.IFrameFrom = frameFrom
		definition.IFrameTo = frameTo
		definition.Speed = 1
		definition.Duration = (math.Abs(float64(frameFrom-frameTo)) + 1) / float64(definition.FrameFps)
		prepared[name] = &definition
	}
	return prepared, nil
}

func prepareAnimationFrame(animation, field string, value any, layout *coreproject.CostumeLayout) (int, error) {
	index, ok := layout.ResolveFrameIndex(value)
	if !ok {
		return 0, fmt.Errorf("fAnimations[%q].%s references missing costume %q", animation, field, value)
	}
	costumeCount := len(layout.Frames)
	if index < 0 || index >= costumeCount {
		return 0, fmt.Errorf("fAnimations[%q].%s index %d is outside %d costumes", animation, field, index, costumeCount)
	}
	return index, nil
}

func (p *Game) loadPreparedSprite(sprite Sprite, name string, gamer reflect.Value, prepared *preparedSprite) error {
	vSpr := reflect.ValueOf(sprite).Elem()
	vSpr.Set(reflect.Zero(vSpr.Type()))
	base := vSpr.Field(0).Addr().Interface().(*SpriteImpl)
	// Paths in config are already normalized by coreproject.LoadSpriteConfig.
	config := &prepared.config
	base.initSpriteCostumes(config, prepared.layout)
	base.spriteState.DefaultCostumeIndex = base.costumeIndex
	base.scriptEventBindings.bind(&p.scriptEvents, base)

	base.gamer = gamer
	base.g, base.name, base.sprite = p, name, sprite
	base.runtimeState.Scale = config.Size
	base.spriteState.IsVisible = config.Visible

	base.components.initComponents(base, config)
	base.initRuntimeProxy()
	p.sprs[name] = sprite
	return bindSpriteOwner(vSpr, gamer)
}

func (p *Game) loadStage(
	g reflect.Value,
	proj *coreproject.ProjectConfig,
	generation uint64,
	loadSprite spriteLoader,
	entries []preparedStageEntry,
) {
	p.setupDisplayConfig(proj)
	p.setupWorldAndWindow(proj)
	p.setupPlatformAndCamera(proj)
	p.setupAudioAndTilemap(proj)

	inits := p.loadAndInitSprites(g, proj, loadSprite, entries)
	p.runSpriteCallbacks(inits, proj, g, generation)
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

func (p *Game) applyWorldWindowMetrics(metrics coreproject.WorldWindowMetrics) {
	p.displayState.WorldWidth = metrics.WorldWidth
	p.displayState.WorldHeight = metrics.WorldHeight
	p.displayState.MinWorldX = metrics.MinWorldX
	p.displayState.MinWorldY = metrics.MinWorldY
	p.displayState.MapMode = metrics.MapMode
	p.displayState.WindowWidth = metrics.WindowWidth
	p.displayState.WindowHeight = metrics.WindowHeight
}

func (p *Game) setupWorldAndWindow(proj *coreproject.ProjectConfig) {
	proj.Map = coreproject.ResolveMapConfig(proj.Map, p.tilemapMgr.hasData(), baseScreenWidth, baseScreenHeight)
	backdrops := proj.GetBackdrops()
	if p.tilemapMgr.hasData() {
		backdrops = make([]*coreproject.BackdropConfig, 0)
	}

	p.displayState.WorldWidth = proj.Map.Width
	p.displayState.WorldHeight = proj.Map.Height

	if len(backdrops) > 0 {
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

func (p *Game) syncPenCanvasToWorld() {
	width := int64(p.displayState.WorldWidth)
	height := int64(p.displayState.WorldHeight)
	p.penCommandBarrier(func() {
		engine.Managers().PenMgr.SetCanvasSize(width, height)
	})
}

func (p *Game) applyStageGeometry() {
	if p.camera != nil {
		p.camera.setLimits()
	}
	p.syncPenCanvasToWorld()
}

// -----------------------------------------------------------------------------
// Sprite Setup
// -----------------------------------------------------------------------------
func (p *Game) runSpriteCallbacks(inits []Sprite, proj *coreproject.ProjectConfig, g reflect.Value, generation uint64) {
	var onLoaded func()
	if loader, ok := g.Addr().Interface().(interface{ OnLoaded() }); ok {
		onLoaded = loader.OnLoaded
	}
	queueBootstrap := func(call func()) {
		p.queueBootstrap(generation, call)
	}
	// Bootstrap hooks may override the initial camera target.
	if proj.Camera != nil && proj.Camera.On != "" {
		p.Camera.Follow__1(proj.Camera.On)
	}
	queueBootstrap(func() {
		p.setupCollisionData(inits)
	})
	for _, ini := range inits {
		if spr := spriteOf(ini); spr != nil {
			queueBootstrap(spr.awake)
		}
	}
	queueBootstrap(func() {
		runSpriteMainsUntilYield(inits)
	})
	queueBootstrap(onLoaded)
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
	p.applyStageGeometry()
}

func bindSpriteOwner(spriteValue reflect.Value, gamer reflect.Value) error {
	if spriteValue.NumField() < 2 {
		return fmt.Errorf("sprite %s is missing owner field", spriteValue.Type())
	}

	ownerField := spriteValue.Field(1)
	if !ownerField.CanAddr() {
		return fmt.Errorf("sprite %s owner field is not addressable", spriteValue.Type())
	}

	gamerPtr := gamer.Addr()
	if !gamerPtr.Type().AssignableTo(ownerField.Type()) {
		return fmt.Errorf(
			"sprite %s owner field type %s cannot hold %s",
			spriteValue.Type(), ownerField.Type(), gamerPtr.Type(),
		)
	}

	// ownerField may be unexported in generated sprite structs, so Set would panic.
	// reflect.NewAt gives us a typed, settable view over the same storage.
	reflect.NewAt(ownerField.Type(), unsafe.Pointer(ownerField.UnsafeAddr())).Elem().Set(gamerPtr)
	return nil
}
