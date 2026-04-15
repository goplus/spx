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
	"reflect"
	"unsafe"

	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/engine/platform"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/ui"
)

func (p *Game) loadSprite(sprite Sprite, name string, gamer reflect.Value) error {
	spxlog.Debug("==> LoadSprite: %s", name)
	loaded, err := coreproject.LoadSpriteConfig(p.fs, name)
	if err != nil {
		return err
	}

	vSpr := reflect.ValueOf(sprite).Elem()
	vSpr.Set(reflect.Zero(vSpr.Type()))
	base := vSpr.Field(0).Addr().Interface().(*SpriteImpl)
	// Paths in loaded.Config are already normalized by coreproject.LoadSpriteConfig.
	base.init(p, name, &loaded.Config, gamer, sprite)
	p.sprs[name] = sprite
	return bindSpriteOwner(vSpr, gamer)
}

func (p *Game) loadIndex(g reflect.Value, proj *coreproject.ProjectConfig, generation uint64) (err error) {
	p.setupDisplayConfig(proj)
	p.setupWorldAndWindow(proj)
	p.setupPlatformAndCamera(proj)
	p.setupAudioAndTilemap(proj)

	inits := p.loadAndInitSprites(g, proj)
	p.runSpriteCallbacks(inits, proj, g, generation)
	return
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
	spxlog.Debug("==> SetWorldSize: %d, %d", p.displayState.WorldWidth, p.displayState.WorldHeight)

	metrics := coreproject.ResolveWorldWindowMetrics(
		p.displayState.WorldWidth,
		p.displayState.WorldHeight,
		p.displayState.WindowWidth,
		p.displayState.WindowHeight,
		coreproject.ToMapMode(proj.Map.Mode),
	)
	p.displayState.WorldWidth = metrics.WorldWidth
	p.displayState.WorldHeight = metrics.WorldHeight
	p.displayState.MinWorldX = metrics.MinWorldX
	p.displayState.MinWorldY = metrics.MinWorldY
	p.displayState.MapMode = metrics.MapMode
	p.doWindowSize()
	spxlog.Debug("==> SetWindowSize: %d, %d", p.displayState.WindowWidth, p.displayState.WindowHeight)

	metrics = coreproject.ResolveWorldWindowMetrics(
		p.displayState.WorldWidth,
		p.displayState.WorldHeight,
		p.displayState.WindowWidth,
		p.displayState.WindowHeight,
		p.displayState.MapMode,
	)
	p.displayState.WindowWidth = metrics.WindowWidth
	p.displayState.WindowHeight = metrics.WindowHeight
}

func (p *Game) setupPlatformAndCamera(proj *coreproject.ProjectConfig) {
	platformMgr := p.engine().PlatformMgr

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
	platformMgr.SetStretchMode(p.displayState.StretchMode)

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

// -----------------------------------------------------------------------------
// Sprite Setup
// -----------------------------------------------------------------------------
func (p *Game) loadAndInitSprites(g reflect.Value, proj *coreproject.ProjectConfig) []Sprite {
	inits := make([]Sprite, 0, len(proj.Zorder))
	err := coreproject.WalkZOrder(
		proj.Zorder,
		func(layer int, name string) error {
			sp := p.getSpriteProtoByName(name, g)
			spr := spriteOf(sp)
			spr.setLayer(layer)
			p.addShape(spr)
			inits = append(inits, sp)
			return nil
		},
		func(layer int, shape coreproject.StageShape) error {
			var err error
			inits, err = p.addSpecialShape(g, shape, inits)
			if err != nil {
				return fmt.Errorf("addSpecialShape: %w", err)
			}
			return nil
		},
	)
	if err != nil {
		engine.Panic(err)
	}
	return inits
}

func (p *Game) runSpriteCallbacks(inits []Sprite, proj *coreproject.ProjectConfig, g reflect.Value, generation uint64) {
	var onLoaded func()
	if loader, ok := g.Addr().Interface().(interface{ OnLoaded() }); ok {
		onLoaded = loader.OnLoaded
	}
	cameraTarget := ""
	if proj.Camera != nil {
		cameraTarget = proj.Camera.On
	}
	// Apply the project camera target as an initial default so bootstrap hooks
	// such as MainEntry/Main/OnLoaded can still override it later.
	if cameraTarget != "" {
		p.Camera.Follow__1(cameraTarget)
	}
	coreproject.RunSpriteInitializers(coreproject.SpriteInitConfig[Sprite]{
		Items: inits,
		Setup: func(items []Sprite) {
			p.deferBootstrapFor(generation, func() {
				p.setupCollisionLayers(items)
			})
		},
		BeforeMain: func(ini Sprite) {
			spr := spriteOf(ini)
			if spr != nil {
				spr.onAwake(func() {
					spr.awake()
				})
			}
		},
		RunMain: func(ini Sprite) {
			p.deferBootstrapFor(generation, func() {
				runMain(ini.Main)
			})
		},
		OnLoaded: func() {
			p.deferBootstrapFor(generation, func() {
				if onLoaded != nil {
					onLoaded()
				}
			})
		},
	})
}

// -----------------------------------------------------------------------------
// Stage Items
// -----------------------------------------------------------------------------
func (p *Game) setupAudioAndTilemap(proj *coreproject.ProjectConfig) {
	p.tilemapMgr.parseTilemap()
	p.audioState.SoundObj = p.soundMgr.AllocSound()
	if proj.Bgm != "" {
		p.Play__1(proj.Bgm, true)
	}
}

func (p *Game) endLoad(g reflect.Value, proj *coreproject.ProjectConfig, generation uint64) (err error) {
	spxlog.Debug("==> EndLoad")
	return p.loadIndex(g, proj, generation)
}

func (p *Game) addSpecialShape(g reflect.Value, v coreproject.StageShape, inits []Sprite) ([]Sprite, error) {
	return coreproject.AppendStageItems(inits, v, coreproject.StageItemHandlers[Sprite]{
		StageMonitor: func(shape coreproject.StageShape) error {
			sm, err := newMonitor(g, shape)
			if err != nil {
				spxlog.Error("addSpecialShape type: %s", shape["type"])
				return nil
			}
			sm.game = p
			p.shapeMgr.addShape(sm)
			return nil
		},
		Measure: func(shape coreproject.StageShape) error {
			p.shapeMgr.addShape(ui.NewMeasureShape(shape))
			return nil
		},
		Sprites: func(shape coreproject.StageShape) ([]Sprite, error) {
			return p.addStageSprites(g, shape)
		},
		Sprite: func(shape coreproject.StageShape) (Sprite, error) {
			return p.addStageSprite(g, shape)
		},
	})
}

func (p *Game) addStageSprite(g reflect.Value, v coreproject.StageShape) (Sprite, error) {
	target, err := stageShapeTarget(v)
	if err != nil {
		return nil, err
	}
	var added Sprite
	err = coreproject.BindStageSprite(g, target, coreproject.FindObjectPtr, func(val any) error {
		sp, ok := val.(Sprite)
		if !ok {
			return fmt.Errorf("stage sprite target is not a sprite")
		}
		dest := spriteOf(sp)
		applySpriteProps(dest, v)
		p.shapeMgr.addShape(dest)
		added = sp
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("addStageSprite: %w", err)
	}
	return added, nil
}

func (p *Game) addStageSprites(g reflect.Value, v coreproject.StageShape) ([]Sprite, error) {
	target, err := stageShapeTarget(v)
	if err != nil {
		return nil, err
	}
	rawItems, err := stageShapeItems(v)
	if err != nil {
		return nil, err
	}
	items := make([]Sprite, 0, len(rawItems))
	err = coreproject.BindStageSprites(
		g,
		target,
		rawItems,
		coreproject.FindFieldPtr,
		func(typ reflect.Type) bool {
			return typ.Implements(tySprite)
		},
		func(newItem reflect.Value, shape coreproject.StageShape) error {
			spr := p.getSpriteProto(newItem.Type(), g)
			dest, sp := applySprite(newItem, spr, shape)
			p.shapeMgr.addShape(dest)
			items = append(items, sp)
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("addStageSprites: %w", err)
	}
	return items, nil
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

func stageShapeTarget(shape coreproject.StageShape) (string, error) {
	target, ok := shape["target"].(string)
	if !ok || target == "" {
		return "", fmt.Errorf("stage shape target must be a non-empty string")
	}
	return target, nil
}

func stageShapeItems(shape coreproject.StageShape) ([]any, error) {
	items, ok := shape["items"].([]any)
	if !ok {
		return nil, fmt.Errorf("stage shape items must be an array")
	}
	return items, nil
}
