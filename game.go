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
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/audiorecord"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/debug"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/engine/platform"
	"github.com/goplus/spx/v2/internal/timer"
	"github.com/goplus/spx/v2/internal/ui"

	spxfs "github.com/goplus/spx/v2/fs"
	_ "github.com/goplus/spx/v2/fs/asset"
	_ "github.com/goplus/spx/v2/fs/zip"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

const (
	GopPackage = true
	Gop_sched  = "Sched,SchedNow"
)

// -------------------------------------------------------------------------------------
// Debug flags

type dbgFlags int

const (
	DbgFlagLoad dbgFlags = 1 << iota
	DbgFlagInstr
	DbgFlagEvent
	DbgFlagPerf
	DbgFlagAll = DbgFlagLoad | DbgFlagInstr | DbgFlagEvent | DbgFlagPerf
)

// -------------------------------------------------------------------------------------
// Mouse button constants

const (
	MOUSE_BUTTON_LEFT   int64 = 1
	MOUSE_BUTTON_RIGHT  int64 = 2
	MOUSE_BUTTON_MIDDLE int64 = 3
)

// -------------------------------------------------------------------------------------
// Configuration constants

const (
	eventBufferSize             = 16   // size of event channel buffer
	schedTimeoutMs              = 3000 // timeout in milliseconds for scheduler
	mainExecTimeoutSec          = 3    // timeout in seconds for main execution
	mouseMovementThreshold      = 1.0  // minimum movement to trigger mouse event (pixels)
	defaultPathCellSize         = 16   // default path finding cell size
	defaultAudioMaxDist         = 2000 // default maximum audio distance
	initialSpriteSyncBufferSize = 100  // initial buffer size for sprite synchronization
)

var (
	debugInstr bool
	debugLoad  bool
	debugEvent bool
	debugPerf  bool
)

var (
	isSchedInMain bool
	mainSchedTime time.Time

	enabledPhysics bool
)

// -------------------------------------------------------------------------------------
// Core Types

type Shape any

// Game represents the main game instance with all core systems
type Game struct {
	baseObj
	eventSinks
	Camera Camera
	camera *cameraImpl

	fs spxfs.Dir

	inputs inputManager
	sounds soundMgr
	typs   map[string]reflect.Type // map: name => sprite type, for all sprites
	sprs   map[string]Sprite       // map: name => sprite prototype, for loaded sprites

	spriteMgr *spriteManager

	events    chan event
	aurec     *audiorecord.Recorder
	startFlag sync.Once

	// map world
	worldWidth_  int
	worldHeight_ int
	minWorldX_   int
	minWorldY_   int
	mapMode      int

	// window
	windowWidth_  int
	windowHeight_ int

	mousePos mathf.Vec2

	sinkMgr  eventSinkMgr
	isLoaded bool
	isRunned bool
	gamer_   Gamer

	windowScale float64
	stretchMode bool
	soundObj    engine.Object

	askPanel  *ui.UiAsk
	answerVal string

	oncePathFinder sync.Once
	pathCellSizeX  int
	pathCellSizeY  int

	// debug
	debug      bool
	debugPanel *ui.UiDebug

	sprCollisionInfos       map[string]*spriteCollisionInfo
	isCollisionByPixel      bool
	isAutoSetCollisionLayer bool

	audioAttenuation float64
	audioMaxDistance float64

	tilemapMgr gameTilemapMgr

	// Batch synchronization buffer (reused every frame)
	syncBuffer *engine.SpriteSyncBuffer
}

const maxCollisionLayerIdx = 32 // engine limit support 32 layers
var imageSizeCache sync.Map

type spriteCollisionInfo struct {
	Id    int
	Layer int64
	Mask  int64
}

type Gamer interface {
	engine.IGame
	initGame(sprites []Sprite) *Game
}

func (p *Game) getSpriteCollisionInfo(name string) *spriteCollisionInfo {
	if info, ok := p.sprCollisionInfos[name]; ok {
		return info
	}
	engine.Panic("Unknown sprite " + name)
	return &spriteCollisionInfo{}
}

func (p *Game) newSpriteAndLoad(name string, tySpr reflect.Type, g reflect.Value) Sprite {
	spr := reflect.New(tySpr).Interface().(Sprite)
	if err := p.loadSprite(spr, name, g); err != nil {
		panic(err)
	}
	// p.sprs[name] = spr (has been set by loadSprite)
	return spr
}

func (p *Game) getSpriteProto(tySpr reflect.Type, g reflect.Value) Sprite {
	name := tySpr.Name()
	spr, ok := p.sprs[name]
	if !ok {
		spr = p.newSpriteAndLoad(name, tySpr, g)
	}
	return spr
}

func (p *Game) getSpriteProtoByName(name string, g reflect.Value) Sprite {
	spr, ok := p.sprs[name]
	if !ok {
		tySpr, ok := p.typs[name]
		if !ok {
			log.Panicf("sprite %s is not defined\n", name)
		}
		spr = p.newSpriteAndLoad(name, tySpr, g)
	}
	return spr
}

func (p *Game) reset() {
	p.releaseGameAudio()
	p.EraseAll()

	p.sinkMgr.reset()
	p.spriteMgr.reset()

	p.debugPanel = nil
	p.askPanel = nil
	p.isLoaded = false

	p.startFlag = sync.Once{}
	p.oncePathFinder = sync.Once{}
	imageSizeCache = sync.Map{}
	p.sprs = make(map[string]Sprite)

	timer.OnReload()
	close(p.events)
	p.Stop(AllOtherScripts)
}

func (p *Game) initGame(sprites []Sprite) *Game {
	engine.SetGame(p)
	p.eventSinks.init(&p.sinkMgr, p)
	p.sprs = make(map[string]Sprite)
	p.typs = make(map[string]reflect.Type)
	p.initSpriteMgr()
	// Initialize batch sync buffer (preallocate for ~100 sprites, will grow if needed)
	p.syncBuffer = engine.NewSpriteSyncBuffer(initialSpriteSyncBufferSize)
	for _, spr := range sprites {
		tySpr := reflect.TypeOf(spr).Elem()
		p.typs[tySpr.Name()] = tySpr
	}
	return p
}

func (p *Game) initSpriteMgr() {
	if p.spriteMgr == nil {
		p.spriteMgr = newSpriteManager()
	}
}

// -------------------------------------------------------------------------------------
// Public API

// SetDebug sets debug flags for the game
func SetDebug(flags dbgFlags) {
	debugLoad = true
	debugInstr = (flags & DbgFlagInstr) != 0
	debugEvent = (flags & DbgFlagEvent) != 0
	debugPerf = (flags & DbgFlagPerf) != 0
}

// Gopt_Game_Main is required by XGo compiler as the entry of a .gmx project.
func Gopt_Game_Main(game Gamer, sprites ...Sprite) {
	g := game.initGame(sprites)
	g.gamer_ = game
	engine.Main(game)
}

// Gopt_Game_Run runs the game using the builder pattern
func Gopt_Game_Run(game Gamer, resource any, gameConf ...*Config) {
	builder := newGameBuilder(game, resource, gameConf...)
	if err := builder.buildAndRun(); err != nil {
		engine.Panic(err)
	}
}

// Gopt_Game_Reload reloads the game with new configuration
func Gopt_Game_Reload(game Gamer, index any) (err error) {
	v := reflect.ValueOf(game).Elem()
	g := instance(v)
	g.reset()
	engine.ClearAllSprites()

	// Recreate events channel after reset closed it
	g.events = make(chan event, eventBufferSize)

	for i, n := 0, v.NumField(); i < n; i++ {
		name, val := getFieldPtrOrAlloc(g, v, i)
		if fld, ok := val.(Sprite); ok {
			if err := g.loadSprite(fld, name, v); err != nil {
				engine.Panic(err)
			}
		}
	}
	var proj projConfig
	if err = loadProjConfig(&proj, g.fs, index); err != nil {
		return
	}
	gco.OnRestart()
	err = g.loadIndex(v, &proj)
	gco.OnInited()

	// Restart event loops after reload
	g.initEventLoop()
	return
}

// SchedNow performs immediate scheduling without timeout check
func SchedNow() int {
	if isSchedInMain {
		if time.Since(mainSchedTime) >= time.Second*mainExecTimeoutSec {
			engine.Panic("Main execution timed out. Please check if there is an infinite loop in the code.")
		}
	}
	if me := gco.Current(); me != nil {
		gco.Sched(me)
	}
	return 0
}

// Sched performs scheduling with timeout check
func Sched() int {
	if isSchedInMain {
		if time.Since(mainSchedTime) >= time.Second*mainExecTimeoutSec {
			engine.Panic("Main execution timed out. Please check if there is an infinite loop in the code.")
		}
	} else {
		if me := gco.Current(); me != nil {
			if me.IsSchedTimeout(schedTimeoutMs) {
				spxlog.Warn("For loop execution timed out. Please check if there is an infinite loop in the code.\n%s", debug.GetStackTrace())
				engine.WaitNextFrame()
			}
		}
	}
	return 0
}

// Forever executes a function indefinitely
func Forever(call func()) {
	if call == nil {
		return
	}
	for {
		call()
		engine.WaitNextFrame()
	}
}

// Repeat executes a function for a specified number of times
func Repeat(loopCount int, call func()) {
	if call == nil {
		return
	}
	for range loopCount {
		call()
		engine.WaitNextFrame()
	}
}

// RepeatUntil executes a function until a condition is met
func RepeatUntil(condition func() bool, call func()) {
	if call == nil || condition == nil {
		return
	}
	for {
		if condition() {
			return
		}
		call()
		engine.WaitNextFrame()
	}
}

// WaitUntil waits until a condition is met
func WaitUntil(condition func() bool) {
	if condition == nil {
		return
	}
	for {
		if condition() {
			return
		}
		engine.WaitNextFrame()
	}
}

// -------------------------------------------------------------------------------------

// parseCommandLineFlags handles command line arguments
func parseCommandLineFlags(conf *Config) {
	if conf.DontParseFlags {
		return
	}

	f := flag.CommandLine
	verbose := f.Bool("v", false, "print verbose information")
	fullscreen := f.Bool("f", false, "full screen")
	help := f.Bool("h", false, "show help information")
	fullscreen2 := f.Bool("fullscreen", false, "server mode")

	f.String("controller", "", "controller's name")
	f.Bool("servermode", false, "server mode")
	f.String("serveraddr", "", "server address")
	f.Bool("nomap", false, "server mode")
	f.Bool("debugweb", false, "server mode")
	f.String("gdextpath", "", "godot extension path")
	f.String("write-movie", "", "movie mode")

	f.String("path", "", "gdspx project path")
	f.Bool("e", false, "editor mode")
	f.Bool("headless", false, "Headless Mode")
	f.Bool("remote-debug", false, "remote Debug Mode")
	f.Bool("no-header", false, "disable engine's header output")
	flag.Parse()

	if *help {
		fmt.Fprintf(os.Stderr, "Usage: %v [-v -f -h]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *verbose {
		spxlog.SetLevel(spxlog.LevelDebug)
		SetDebug(DbgFlagAll)
	}
	conf.FullScreen = conf.FullScreen || *fullscreen2 || *fullscreen
}

// setupGameConfig configures game settings
func setupGameConfig(g *Game, conf *Config, proj *projConfig) {
	if conf.Title == "" {
		dir, _ := os.Getwd()
		appName := filepath.Base(dir)
		conf.Title = appName + " (by XGo Builder)"
	}

	proj.FullScreen = proj.FullScreen || conf.FullScreen
	enabledPhysics = proj.Physics
	physicMgr.SetGlobalGravity(parseDefaultFloatValue(proj.GlobalGravity, 1))
	physicMgr.SetGlobalAirDrag(parseDefaultFloatValue(proj.GlobalAirDrag, 1))
	physicMgr.SetGlobalFriction(parseDefaultFloatValue(proj.GlobalFriction, 1))

	g.windowHeight_ = conf.Height
	g.windowWidth_ = conf.Width

	key := conf.ScreenshotKey
	if key == "" {
		key = os.Getenv("SPX_SCREENSHOT_KEY")
	}
	if key != "" {
		if err := os.Setenv("SPX_SCREENSHOT_KEY", key); err != nil {
			engine.Panic(err)
		}
	}
}

// setupGameSystems initializes game subsystems
func setupGameSystems(g *Game, proj *projConfig) {
	if debugLoad {
		spxlog.Debug("==> isCollisionByPixel: %v", !proj.CollisionByShape && !proj.Physics)
		spxlog.Debug("==> isAutoSetCollisionLayer: %v", proj.AutoSetCollisionLayer == nil || *proj.AutoSetCollisionLayer)
	}

	g.isCollisionByPixel = !proj.CollisionByShape && !proj.Physics
	g.isAutoSetCollisionLayer = proj.AutoSetCollisionLayer == nil || *proj.AutoSetCollisionLayer
	g.pathCellSizeX = parseDefaultNumber(proj.PathCellSizeX, defaultPathCellSize)
	g.pathCellSizeY = parseDefaultNumber(proj.PathCellSizeY, defaultPathCellSize)

	engine.SetLayerSortMode(proj.LayerSortMode)
	g.audioAttenuation = parseDefaultFloatValue(proj.AudioAttenuation, 0)
	g.audioMaxDistance = parseDefaultFloatValue(proj.AudioMaxDistance, defaultAudioMaxDist)

	physicMgr.SetCollisionSystemType(g.isCollisionByPixel)
	if g.isAutoSetCollisionLayer {
		g.sprCollisionInfos = make(map[string]*spriteCollisionInfo)
		idx := 0
		for name := range g.typs {
			modIdx := int(math.Mod(float64(idx), maxCollisionLayerIdx))
			info := &spriteCollisionInfo{Id: idx, Layer: 1 << modIdx}
			g.sprCollisionInfos[name] = info
			idx++
		}
	}
}

// loadGameSprites loads all sprites
func loadGameSprites(g *Game, v reflect.Value, fs spxfs.Dir, proj *projConfig) {
	if debugLoad {
		spxlog.Debug("==> StartLoad")
	}

	g.startLoad(fs)
	for i, n := 0, v.NumField(); i < n; i++ {
		name, val := getFieldPtrOrAlloc(g, v, i)
		if fld, ok := val.(Sprite); ok {
			if g.canBindSprite(name) {
				if err := g.loadSprite(fld, name, v); err != nil {
					engine.Panic(err)
				}
			}
		}
	}
	g.tilemapMgr.init(g, fs, proj.TilemapPath)
}

func instance(gamer reflect.Value) *Game {
	fld := gamer.FieldByName("Game")
	if !fld.IsValid() {
		panic("type doesn't have field spx.Game")
	}
	return fld.Addr().Interface().(*Game)
}

func getFieldPtrOrAlloc(g *Game, v reflect.Value, i int) (name string, val any) {
	tFld := v.Type().Field(i)
	vFld := v.Field(i)
	typ := tFld.Type
	word := unsafe.Pointer(vFld.Addr().Pointer())
	ret := reflect.NewAt(typ, word).Interface()

	if vFld.Kind() == reflect.Pointer && typ.Implements(tySprite) {
		obj := reflect.New(typ.Elem())
		reflect.ValueOf(ret).Elem().Set(obj)
		ret = obj.Interface()
	}

	if vFld.Kind() == reflect.Interface && typ.Implements(tySprite) {
		if typ2, ok := g.typs[tFld.Name]; ok {
			obj := reflect.New(typ2)
			reflect.ValueOf(ret).Elem().Set(obj)
			ret = obj.Interface()
		}
	}
	return tFld.Name, ret
}

func findFieldPtr(v reflect.Value, name string, from int) any {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()
	for i, n := from, v.NumField(); i < n; i++ {
		tFld := t.Field(i)
		if tFld.Name == name {
			word := unsafe.Pointer(v.Field(i).Addr().Pointer())
			return reflect.NewAt(tFld.Type, word).Interface()
		}
	}
	return nil
}

func findObjPtr(v reflect.Value, name string, from int) any {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()
	for i, n := from, v.NumField(); i < n; i++ {
		tFld := t.Field(i)
		if tFld.Name == name {
			typ := tFld.Type
			vFld := v.Field(i)
			if vFld.Kind() == reflect.Pointer {
				word := unsafe.Pointer(vFld.Pointer())
				return reflect.NewAt(typ.Elem(), word).Interface()
			}
			if vFld.Kind() == reflect.Interface {
				word := unsafe.Pointer(vFld.Addr().Pointer())
				return reflect.NewAt(tFld.Type, word).Elem().Interface()
			}
			word := unsafe.Pointer(vFld.Addr().Pointer())
			return reflect.NewAt(typ, word).Interface()
		}
	}
	return nil
}

func (p *Game) startLoad(fs spxfs.Dir) {
	p.sounds.init(p)
	p.inputs.init(p)
	p.events = make(chan event, eventBufferSize)
	p.fs = fs
}

func (p *Game) canBindSprite(name string) bool {
	return p.typs[name] != nil
}

func (p *Game) loadSprite(sprite Sprite, name string, gamer reflect.Value) error {
	if debugLoad {
		spxlog.Debug("==> LoadSprite: %s", name)
	}
	var baseDir = "sprites/" + name + "/"
	var conf spriteConfig
	err := loadJson(&conf, p.fs, baseDir+"index.json")
	if err != nil {
		return err
	}
	//
	// init sprite (field 0)
	vSpr := reflect.ValueOf(sprite).Elem()
	vSpr.Set(reflect.Zero(vSpr.Type()))
	base := vSpr.Field(0).Addr().Interface().(*SpriteImpl)
	base.init(baseDir, p, name, &conf, gamer, sprite)
	p.sprs[name] = sprite
	//
	// init gamer pointer (field 1)
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

// setupDisplayConfig initializes display configuration
func (p *Game) setupDisplayConfig(proj *projConfig) {
	windowScale := 1.0
	if proj.WindowScale >= 0.001 {
		windowScale = proj.WindowScale
	}
	p.windowScale = windowScale
	p.stretchMode = proj.StretchMode == nil || *proj.StretchMode
	p.debug = proj.Debug
}

// setupWorldAndWindow configures world and window sizes
func (p *Game) setupWorldAndWindow(proj *projConfig) {
	backdrops := proj.getBackdrops()
	if p.tilemapMgr.hasData() {
		backdrops = make([]*backdropConfig, 0)
		if proj.Map.Width == 0 {
			proj.Map.Width = 480
		}
		if proj.Map.Height == 0 {
			proj.Map.Height = 360
		}
	}

	if len(backdrops) > 0 {
		p.baseObj.initBackdrops("", backdrops, proj.getBackdropIndex())
		p.worldWidth_ = proj.Map.Width
		p.worldHeight_ = proj.Map.Height
		p.minWorldX_ = -p.worldWidth_ / 2
		p.minWorldY_ = -p.worldHeight_ / 2
		p.doWorldSize()
	} else {
		p.worldWidth_ = proj.Map.Width
		p.worldHeight_ = proj.Map.Height
		p.minWorldX_ = -p.worldWidth_ / 2
		p.minWorldY_ = -p.worldHeight_ / 2
		p.baseObj.initWithSize(p.worldWidth_, p.worldHeight_)
	}

	if debugLoad {
		spxlog.Debug("==> SetWorldSize: %d, %d", p.worldWidth_, p.worldHeight_)
	}
	p.mapMode = toMapMode(proj.Map.Mode)
	p.doWindowSize()

	engine.SetDebugMode(p.debug)
	if debugLoad {
		spxlog.Debug("==> SetWindowSize: %d, %d", p.windowWidth_, p.windowHeight_)
	}
	if p.windowWidth_ > p.worldWidth_ {
		p.windowWidth_ = p.worldWidth_
	}
	if p.windowHeight_ > p.worldHeight_ {
		p.windowHeight_ = p.worldHeight_
	}
}

// setupPlatformAndCamera configures platform settings and camera
func (p *Game) setupPlatformAndCamera(proj *projConfig) {
	if platform.IsMobile() || proj.FullScreen || platform.IsWeb() {
		if proj.FullScreen || platform.IsMobile() {
			platformMgr.SetWindowFullscreen(true)
		}
		winSize := platformMgr.GetWindowSize()
		scale := math.Min(winSize.X/float64(p.windowWidth_), winSize.Y/float64(p.windowHeight_))
		p.windowScale = scale
	}

	if platform.IsWeb() {
		platformMgr.SetWindowSize(int64(platformMgr.GetWindowSize().X), int64(platformMgr.GetWindowSize().Y), true)
	} else {
		platformMgr.SetWindowSize(int64(float64(p.windowWidth_)*p.windowScale), int64(float64(p.windowHeight_)*p.windowScale), true)
	}
	platformMgr.SetStretchMode(p.stretchMode)

	p.camera = &cameraImpl{}
	p.Camera = p.camera
	p.camera.init(p)

	isWindowMapSizeEqual := p.worldHeight_ == p.windowHeight_ && p.worldWidth_ == p.windowWidth_
	engine.SetWindowScale(p.windowScale)
	ui.SetWindowScale(p.windowScale)
	ui.ClampUIPositionInScreen(isWindowMapSizeEqual)

	p.syncSprite = engine.NewBackdropProxy(p, p.getCostumePath(), p.getCostumeRenderScale())
	p.setupBackdrop()
}

// loadAndInitSprites loads all sprites from project configuration
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

// runSpriteCallbacks executes sprite initialization callbacks
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

// getCollisionLayerIndex returns the collision layer index for a sprite
func getCollisionLayerIndex(info *spriteCollisionInfo) int {
	return int(math.Mod(float64(info.Id), maxCollisionLayerIdx))
}

// spriteCollisionData caches sprite collision information
type spriteCollisionData struct {
	sprite *SpriteImpl
	info   *spriteCollisionInfo
	modIdx int
}

// buildSpriteCollisionData builds a cache of sprite collision data to avoid repeated lookups
func (p *Game) buildSpriteCollisionData(inits []Sprite) []*spriteCollisionData {
	spriteData := make([]*spriteCollisionData, 0, len(inits))
	for _, ini := range inits {
		spr := spriteOf(ini)
		info := p.getSpriteCollisionInfo(spr.name)
		spriteData = append(spriteData, &spriteCollisionData{
			sprite: spr,
			info:   info,
			modIdx: getCollisionLayerIndex(info),
		})
	}
	return spriteData
}

// setupCollisionLayers configures collision detection layers
func (p *Game) setupCollisionLayers(inits []Sprite) {
	if !p.isAutoSetCollisionLayer {
		return
	}

	spriteData := p.buildSpriteCollisionData(inits)
	maskMap := make([]int64, maxCollisionLayerIdx)

	// Gather collision masks
	for _, data := range spriteData {
		data.info.Mask = 0
		for target := range data.sprite.collisionTargets {
			targetInfo := p.getSpriteCollisionInfo(target)
			maskMap[data.modIdx] |= targetInfo.Layer
		}
	}

	// Apply collision masks
	for _, data := range spriteData {
		data.info.Mask = maskMap[data.modIdx]
		if debugLoad {
			spxlog.Debug("init sprite collision info: name=%s, layer=%d, mask=%d", data.sprite.name, data.info.Layer, data.info.Mask)
		}
	}

	// Recalculate physics info
	engine.WaitMainThread(func() {
		for _, data := range spriteData {
			syncInitSpritePhysicInfo(data.sprite, data.sprite.syncSprite)
		}
	})
}

// loadAudioAndTilemap loads tilemap and background music
func (p *Game) loadAudioAndTilemap(proj *projConfig) {
	p.tilemapMgr.parseTilemap()
	p.soundObj = p.sounds.allocSound()
	if proj.Bgm != "" {
		p.Play__0(proj.Bgm, true)
	}
}

func (p *Game) endLoad(g reflect.Value, proj *projConfig) (err error) {
	if debugLoad {
		spxlog.Debug("==> EndLoad")
	}
	return p.loadIndex(g, proj)
}

// -------------------------------------------------------------------------------------
// Stage Setup and Special Shapes

type specsp = map[string]any

func (p *Game) addSpecialShape(g reflect.Value, v specsp, inits []Sprite) []Sprite {
	switch typ := v["type"].(string); typ {
	case "stageMonitor", "monitor":
		if sm, err := newMonitor(g, v); err == nil {
			sm.game = p
			p.spriteMgr.addShape(sm)
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

/*
	 {
	   "type": "sprites",
	   "target": "bananas",
	   "items": [
		 {
		   "x": -100,
		   "y": -21
		 },
		 {
		   "x": 50,
		   "y": -21
		 }
	   ]
	 }
*/
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

var (
	tySprite = reflect.TypeOf((*Sprite)(nil)).Elem()
)

// -------------------------------------------------------------------------------------
// Game Loop and Scheduler

func (p *Game) runLoop(cfg *Config) (err error) {
	if debugLoad {
		spxlog.Debug("==> RunLoop")
	}
	if !cfg.DontRunOnUnfocused {
		platformMgr.SetRunnableOnUnfocused(true)
	}
	p.initEventLoop()
	platformMgr.SetWindowTitle(cfg.Title)
	p.isRunned = true
	return nil
}

func init() {
	gco = coroutine.New(engine.OnPanic)
	engine.SetCoroutines(gco)
}

var (
	gco *coroutine.Coroutines
)

type threadObj = coroutine.ThreadObj

func runMain(call func()) {
	isSchedInMain = true
	mainSchedTime = time.Now()
	call()
	isSchedInMain = false
}

// -----------------------------------------------------------------------------
