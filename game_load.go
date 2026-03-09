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
	"reflect"
	"unsafe"

	"github.com/goplus/spx/v2/internal/base/valueutil"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/engine/platform"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/ui"
)

func (p *Game) loadSprite(sprite Sprite, name string, gamer reflect.Value) error {
	spxlog.Debug("==> LoadSprite: %s", name)
	baseDir := "sprites/" + name + "/"
	var conf spriteConfig
	if err := loadJson(&conf, p.fs, baseDir+"index.json"); err != nil {
		return err
	}

	vSpr := reflect.ValueOf(sprite).Elem()
	vSpr.Set(reflect.Zero(vSpr.Type()))
	base := vSpr.Field(0).Addr().Interface().(*SpriteImpl)
	base.init(baseDir, p, name, &conf, gamer, sprite)
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

	p.isLoaded = true
	return
}

// setupDisplayConfig initializes display configuration.
func (p *Game) setupDisplayConfig(proj *projConfig) {
	windowScale := 1.0
	if proj.WindowScale >= 0.001 {
		windowScale = proj.WindowScale
	}

	p.windowScale = windowScale
	p.stretchMode = proj.StretchMode == nil || *proj.StretchMode
	p.debug = proj.Debug
	if p.debug {
		spxlog.SetLevel(spxlog.LevelDebug)
	} else {
		spxlog.SetLevel(spxlog.LevelInfo)
	}
	engine.SetDebugMode(p.debug)
}

// setupWorldAndWindow configures world and window sizes.
func (p *Game) setupWorldAndWindow(proj *projConfig) {
	backdrops := proj.getBackdrops()
	if p.tilemapMgr.hasData() {
		backdrops = make([]*backdropConfig, 0)
		valueutil.SetDefaultIfZero(&proj.Map.Width, baseScreenWidth)
		valueutil.SetDefaultIfZero(&proj.Map.Height, baseScreenHeight)
	}

	p.worldWidth = proj.Map.Width
	p.worldHeight = proj.Map.Height

	if len(backdrops) > 0 {
		p.baseObj.initBackdrops("", backdrops, proj.getBackdropIndex())
		p.doWorldSize()
	} else {
		p.baseObj.initWithSize(p.worldWidth, p.worldHeight)
	}
	spxlog.Debug("==> SetWorldSize: %d, %d", p.worldWidth, p.worldHeight)

	p.minWorldX = -p.worldWidth / 2
	p.minWorldY = -p.worldHeight / 2

	p.mapMode = toMapMode(proj.Map.Mode)
	p.doWindowSize()
	spxlog.Debug("==> SetWindowSize: %d, %d", p.windowWidth, p.windowHeight)

	p.windowWidth = int(math.Min(float64(p.windowWidth), float64(p.worldWidth)))
	p.windowHeight = int(math.Min(float64(p.windowHeight), float64(p.worldHeight)))
}

// setupPlatformAndCamera configures platform settings and camera.
func (p *Game) setupPlatformAndCamera(proj *projConfig) {
	platformMgr := p.engine().PlatformMgr

	if platform.IsMobile() || proj.FullScreen || platform.IsWeb() {
		if proj.FullScreen || platform.IsMobile() {
			platformMgr.SetWindowFullscreen(true)
		}
		winSize := platformMgr.GetWindowSize()
		scaleX := winSize.X / float64(p.windowWidth)
		scaleY := winSize.Y / float64(p.windowHeight)
		p.windowScale = math.Min(scaleX, scaleY)
	}

	winWidth := int64(float64(p.windowWidth) * p.windowScale)
	winHeight := int64(float64(p.windowHeight) * p.windowScale)
	if platform.IsWeb() {
		size := platformMgr.GetWindowSize()
		winWidth = int64(size.X)
		winHeight = int64(size.Y)
	}

	platformMgr.SetWindowSize(winWidth, winHeight, true)
	platformMgr.SetMaxFps(int64(proj.MaxFPS))
	platformMgr.SetStretchMode(p.stretchMode)

	p.camera = &cameraImpl{}
	p.Camera = p.camera
	p.camera.init(p)

	isWindowMapSizeEqual := p.worldHeight == p.windowHeight && p.worldWidth == p.windowWidth
	engine.SetWindowScale(p.windowScale)
	ui.SetWindowScale(p.windowScale)
	ui.SetBaseScreenSize(baseScreenWidth, baseScreenHeight)
	ui.ClampUIPositionInScreen(isWindowMapSizeEqual)

	p.syncSprite = engine.NewBackdropProxy(p, p.getCostumePath(), p.getCostumeRenderScale())
	p.setupBackdrop()
}

// loadAndInitSprites loads all sprites from project configuration.
func (p *Game) loadAndInitSprites(g reflect.Value, proj *projConfig) []Sprite {
	inits := make([]Sprite, 0, len(proj.Zorder))
	for layer, v := range proj.Zorder {
		if name, ok := v.(string); ok {
			sp := p.getSpriteProtoByName(name, g)
			spr := spriteOf(sp)
			spr.setLayer(layer)
			p.addShape(spr)
			inits = append(inits, sp)
		} else {
			inits = p.addSpecialShape(g, v.(specsp), inits)
		}
	}
	return inits
}

// runSpriteCallbacks executes sprite initialization callbacks.
func (p *Game) runSpriteCallbacks(inits []Sprite, proj *projConfig, g reflect.Value) {
	for _, ini := range inits {
		spr := spriteOf(ini)
		if spr != nil {
			spr.onAwake(func() {
				spr.awake()
			})
		}
		runMain(ini.Main)
	}

	if proj.Camera != nil && proj.Camera.On != "" {
		p.Camera.Follow__1(proj.Camera.On)
	}
	if loader, ok := g.Addr().Interface().(interface{ OnLoaded() }); ok {
		loader.OnLoaded()
	}
}

// loadAudioAndTilemap loads tilemap and background music.
func (p *Game) loadAudioAndTilemap(proj *projConfig) {
	p.tilemapMgr.parseTilemap()
	p.soundObj = p.soundMgr.allocSound()
	if proj.Bgm != "" {
		p.Play__0(proj.Bgm, true)
	}
}

func (p *Game) endLoad(g reflect.Value, proj *projConfig) (err error) {
	spxlog.Debug("==> EndLoad")
	return p.loadIndex(g, proj)
}

type specsp = map[string]any

func (p *Game) addSpecialShape(g reflect.Value, v specsp, inits []Sprite) []Sprite {
	switch typ := v["type"].(string); typ {
	case "stageMonitor", "monitor":
		if sm, err := newMonitor(g, v); err == nil {
			sm.game = p
			p.spriteMgr.addShape(sm)
		} else {
			spxlog.Error("addSpecialShape type: %s", typ)
		}
	case "measure":
		p.spriteMgr.addShape(newMeasure(v))
	case "sprites":
		return p.addStageSprites(g, v, inits)
	case "sprite":
		return p.addStageSprite(g, v, inits)
	default:
		engine.Panic("addSpecialShape: unknown shape - " + typ)
	}
	return inits
}

func (p *Game) addStageSprite(g reflect.Value, v specsp, inits []Sprite) []Sprite {
	target := v["target"].(string)
	if val := findObjPtr(g, target, 0); val != nil {
		if sp, ok := val.(Sprite); ok {
			dest := spriteOf(sp)
			applySpriteProps(dest, v)
			p.spriteMgr.addShape(dest)
			inits = append(inits, sp)
			return inits
		}
	}
	engine.Panic("addStageSprite: unexpected - " + target)
	return inits
}

func (p *Game) addStageSprites(g reflect.Value, v specsp, inits []Sprite) []Sprite {
	target := v["target"].(string)
	if val := findFieldPtr(g, target, 0); val != nil {
		fldSlice := reflect.ValueOf(val).Elem()
		if fldSlice.Kind() == reflect.Slice {
			var typItemPtr reflect.Type
			typSlice := fldSlice.Type()
			typItem := typSlice.Elem()
			isPtr := typItem.Kind() == reflect.Pointer
			if isPtr {
				typItem, typItemPtr = typItem.Elem(), typItem
			} else {
				typItemPtr = reflect.PointerTo(typItem)
			}
			if typItemPtr.Implements(tySprite) {
				spr := p.getSpriteProto(typItem, g)
				items := v["items"].([]any)
				n := len(items)
				newSlice := reflect.MakeSlice(typSlice, n, n)
				for i := range n {
					newItem := newSlice.Index(i)
					if isPtr {
						newItem.Set(reflect.New(typItem))
						newItem = newItem.Elem()
					}
					dest, sp := applySprite(newItem, spr, items[i].(specsp))
					p.spriteMgr.addShape(dest)
					inits = append(inits, sp)
				}
				fldSlice.Set(newSlice)
				return inits
			}
		}
	}
	engine.Panic("addStageSprites: unexpected - " + target)
	return inits
}
