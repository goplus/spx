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
	base.init(loaded.BaseDir, p, name, &loaded.Config, gamer, sprite)
	p.sprs[name] = sprite
	*(*uintptr)(unsafe.Pointer(vSpr.Field(1).Addr().Pointer())) = gamer.Addr().Pointer()
	return nil
}

func (p *Game) loadIndex(g reflect.Value, proj *projConfig) (err error) {
	p.setupDisplayConfig(proj)
	p.setupWorldAndWindow(proj)
	p.setupPlatformAndCamera(proj)

	inits := p.loadAndInitSprites(g, proj)
	p.runSpriteCallbacks(inits, proj, g)
	p.setupCollisionLayers(inits)
	p.loadAudioAndTilemap(proj)

	p.IsLoaded = true
	return
}

// setupDisplayConfig initializes display configuration.
func (p *Game) setupDisplayConfig(proj *projConfig) {
	display := coreproject.ResolveDisplaySettings(proj)
	p.WindowScale = display.WindowScale
	p.StretchMode = display.StretchMode
	p.Debug = display.Debug
	if p.Debug {
		spxlog.SetLevel(spxlog.LevelDebug)
	} else {
		spxlog.SetLevel(spxlog.LevelInfo)
	}
	engine.SetDebugMode(p.Debug)
}

// setupWorldAndWindow configures world and window sizes.
func (p *Game) setupWorldAndWindow(proj *projConfig) {
	proj.Map = coreproject.ResolveMapConfig(proj.Map, p.tilemapMgr.hasData(), baseScreenWidth, baseScreenHeight)
	backdrops := proj.GetBackdrops()
	if p.tilemapMgr.hasData() {
		backdrops = make([]*backdropConfig, 0)
	}

	p.WorldWidth = proj.Map.Width
	p.WorldHeight = proj.Map.Height

	if len(backdrops) > 0 {
		p.baseObj.initBackdrops("", backdrops, proj.GetBackdropIndex())
		p.doWorldSize()
	} else {
		p.baseObj.initWithSize(p.WorldWidth, p.WorldHeight)
	}
	spxlog.Debug("==> SetWorldSize: %d, %d", p.WorldWidth, p.WorldHeight)

	metrics := coreproject.ResolveWorldWindowMetrics(
		p.WorldWidth,
		p.WorldHeight,
		p.WindowWidth,
		p.WindowHeight,
		toMapMode(proj.Map.Mode),
	)
	p.WorldWidth = metrics.WorldWidth
	p.WorldHeight = metrics.WorldHeight
	p.MinWorldX = metrics.MinWorldX
	p.MinWorldY = metrics.MinWorldY
	p.MapMode = metrics.MapMode
	p.doWindowSize()
	spxlog.Debug("==> SetWindowSize: %d, %d", p.WindowWidth, p.WindowHeight)

	metrics = coreproject.ResolveWorldWindowMetrics(
		p.WorldWidth,
		p.WorldHeight,
		p.WindowWidth,
		p.WindowHeight,
		p.MapMode,
	)
	p.WindowWidth = metrics.WindowWidth
	p.WindowHeight = metrics.WindowHeight
}

// setupPlatformAndCamera configures platform settings and camera.
func (p *Game) setupPlatformAndCamera(proj *projConfig) {
	platformMgr := p.engine().PlatformMgr

	layout := coreproject.ResolvePlatformLayout(coreproject.PlatformLayoutInput{
		WindowWidth:       p.WindowWidth,
		WindowHeight:      p.WindowHeight,
		WindowScale:       p.WindowScale,
		Fullscreen:        proj.FullScreen,
		IsMobile:          platform.IsMobile(),
		IsWeb:             platform.IsWeb(),
		CurrentWindowSize: platformMgr.GetWindowSize(),
	})
	if layout.Fullscreen {
		platformMgr.SetWindowFullscreen(true)
	}
	p.WindowScale = layout.WindowScale
	platformMgr.SetWindowSize(layout.WindowWidth, layout.WindowHeight, true)
	platformMgr.SetMaxFps(int64(proj.MaxFPS))
	platformMgr.SetStretchMode(p.StretchMode)

	p.camera = &cameraImpl{}
	p.Camera = p.camera
	p.camera.init(p)

	isWindowMapSizeEqual := coreproject.IsWindowWorldSizeEqual(
		p.WorldWidth,
		p.WorldHeight,
		p.WindowWidth,
		p.WindowHeight,
	)
	engine.SetWindowScale(p.WindowScale)
	ui.SetWindowScale(p.WindowScale)
	ui.SetBaseScreenSize(baseScreenWidth, baseScreenHeight)
	ui.ClampUIPositionInScreen(isWindowMapSizeEqual)

	p.SyncSprite = engine.NewBackdropProxy(p, p.getCostumePath(), p.getCostumeRenderScale())
	p.setupBackdrop()
}

// loadAndInitSprites loads all sprites from project configuration.
func (p *Game) loadAndInitSprites(g reflect.Value, proj *projConfig) []Sprite {
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

// runSpriteCallbacks executes sprite initialization callbacks.
func (p *Game) runSpriteCallbacks(inits []Sprite, proj *projConfig, g reflect.Value) {
	var onLoaded func()
	if loader, ok := g.Addr().Interface().(interface{ OnLoaded() }); ok {
		onLoaded = loader.OnLoaded
	}
	cameraTarget := ""
	if proj.Camera != nil {
		cameraTarget = proj.Camera.On
	}
	coreproject.RunSpriteInitializers(coreproject.SpriteInitConfig[Sprite]{
		Items: inits,
		BeforeMain: func(ini Sprite) {
			spr := spriteOf(ini)
			if spr != nil {
				spr.onAwake(func() {
					spr.awake()
				})
			}
		},
		RunMain: func(ini Sprite) {
			runMain(ini.Main)
		},
		CameraTarget: cameraTarget,
		FollowCamera: p.Camera.Follow__1,
		OnLoaded:     onLoaded,
	})
}

// loadAudioAndTilemap loads tilemap and background music.
func (p *Game) loadAudioAndTilemap(proj *projConfig) {
	p.tilemapMgr.parseTilemap()
	p.SoundObj = p.soundMgr.allocSound()
	if proj.Bgm != "" {
		p.Play__0(proj.Bgm, true)
	}
}

func (p *Game) endLoad(g reflect.Value, proj *projConfig) (err error) {
	spxlog.Debug("==> EndLoad")
	return p.loadIndex(g, proj)
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
			p.spriteMgr.addShape(sm)
			return nil
		},
		Measure: func(shape coreproject.StageShape) error {
			p.spriteMgr.addShape(newMeasure(shape))
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
	target := v["target"].(string)
	var added Sprite
	err := coreproject.BindStageSprite(g, target, findObjPtr, func(val any) error {
		sp, ok := val.(Sprite)
		if !ok {
			return fmt.Errorf("stage sprite target is not a sprite")
		}
		dest := spriteOf(sp)
		applySpriteProps(dest, v)
		p.spriteMgr.addShape(dest)
		added = sp
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("addStageSprite: %w", err)
	}
	return added, nil
}

func (p *Game) addStageSprites(g reflect.Value, v coreproject.StageShape) ([]Sprite, error) {
	target := v["target"].(string)
	items := make([]Sprite, 0, len(v["items"].([]any)))
	err := coreproject.BindStageSprites(
		g,
		target,
		v["items"].([]any),
		findFieldPtr,
		func(typ reflect.Type) bool {
			return typ.Implements(tySprite)
		},
		func(newItem reflect.Value, shape coreproject.StageShape) error {
			spr := p.getSpriteProto(newItem.Type(), g)
			dest, sp := applySprite(newItem, spr, shape)
			p.spriteMgr.addShape(dest)
			items = append(items, sp)
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("addStageSprites: %w", err)
	}
	return items, nil
}
