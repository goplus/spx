/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/goplus/spx/internal/audiorecord"
	"github.com/goplus/spx/internal/coroutine"
	"github.com/goplus/spx/internal/debug"
	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/engine/platform"
	gtime "github.com/goplus/spx/internal/time"
	"github.com/goplus/spx/internal/timer"
	"github.com/goplus/spx/internal/ui"
	"github.com/realdream-ai/mathf"

	spxfs "github.com/goplus/spx/fs"
	_ "github.com/goplus/spx/fs/asset"
	_ "github.com/goplus/spx/fs/zip"
)

const (
	GopPackage = true
	Gop_sched  = "Sched,SchedNow"
)

type dbgFlags int

const (
	DbgFlagLoad dbgFlags = 1 << iota
	DbgFlagInstr
	DbgFlagEvent
	DbgFlagPerf
	DbgFlagAll = DbgFlagLoad | DbgFlagInstr | DbgFlagEvent | DbgFlagPerf
)

const (
	MOUSE_BUTTON_LEFT   int64 = 1
	MOUSE_BUTTON_RIGHT  int64 = 2
	MOUSE_BUTTON_MIDDLE int64 = 3
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
)

func SetDebug(flags dbgFlags) {
	debugLoad = true
	debugInstr = (flags & DbgFlagInstr) != 0
	debugEvent = (flags & DbgFlagEvent) != 0
	debugPerf = (flags & DbgFlagPerf) != 0
}

// -------------------------------------------------------------------------------------

type Shape interface {
}

type Game struct {
	baseObj
	eventSinks
	Camera

	fs spxfs.Dir

	inputs       inputManager
	sounds       soundMgr
	typs         map[string]reflect.Type // map: name => sprite type, for all sprites
	sprs         map[string]Sprite       // map: name => sprite prototype, for loaded sprites
	items        []Shape                 // shapes on stage (in Zorder), not only sprites
	destroyItems []Shape                 // shapes on stage (in Zorder), not only sprites
	tempItems    []Shape                 // temp items

	events    chan event
	aurec     *audiorecord.Recorder
	startFlag sync.Once

	// map world
	worldWidth_  int
	worldHeight_ int
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
	audioId     engine.Object

	askPanel  *ui.UiAsk
	answerVal string

	// debug
	debug      bool
	debugPanel *ui.UiDebug
}

type Gamer interface {
	engine.IGame
	initGame(sprites []Sprite) *Game
}

func (p *Game) IsRunned() bool {
	return p.isRunned
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
	if p.audioId != 0 {
		p.sounds.releaseAudio(p.audioId)
		p.audioId = 0
	}
	p.sinkMgr.reset()
	p.EraseAll() // clear pens
	p.startFlag = sync.Once{}
	p.Stop(AllOtherScripts)
	p.items = nil
	p.debugPanel = nil
	p.askPanel = nil
	p.destroyItems = nil
	p.isLoaded = false
	p.sprs = make(map[string]Sprite)
	timer.OnReload()
}

func (p *Game) getGame() *Game {
	return p
}

func (p *Game) initGame(sprites []Sprite) *Game {
	p.eventSinks.init(&p.sinkMgr, p)
	p.sprs = make(map[string]Sprite)
	p.typs = make(map[string]reflect.Type)
	for _, spr := range sprites {
		tySpr := reflect.TypeOf(spr).Elem()
		p.typs[tySpr.Name()] = tySpr
	}
	return p
}

// Gopt_Game_Main is required by Go+ compiler as the entry of a .gmx project.
func Gopt_Game_Main(game Gamer, sprites ...Sprite) {
	g := game.initGame(sprites)
	g.gamer_ = game
	engine.Main(game)
}

// Gopt_Game_Run runs the game.
// resource can be a string or fs.Dir object.
func Gopt_Game_Run(game Gamer, resource interface{}, gameConf ...*Config) {
	switch resfld := resource.(type) {
	case string:
		if resfld != "" {
			engine.SetAssetDir(resfld)
		} else {
			engine.SetAssetDir("assets")
		}
	}
	fs, err := resourceDir(resource)
	if err != nil {
		panic(err)
	}

	var conf Config
	var proj projConfig
	if gameConf != nil {
		conf = *gameConf[0]
		err = loadProjConfig(&proj, fs, conf.Index)
	} else {
		err = loadProjConfig(&proj, fs, nil)
		if proj.Run != nil { // load Config from index.json
			conf = *proj.Run
		}
	}
	if err != nil {
		panic(err)
	}
	if !conf.DontParseFlags {
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

		// godot args
		f.String("path", "", "gdspx project path")
		f.Bool("e", false, "editor mode")
		f.Bool("headless", false, "Headless Mode")
		f.Bool("remote-debug", false, "remote Debug Mode")
		flag.Parse()

		if *help {
			fmt.Fprintf(os.Stderr, "Usage: %v [-v -f -h]\n", os.Args[0])
			flag.PrintDefaults()
			return
		}
		if *verbose {
			SetDebug(DbgFlagAll)
		}
		conf.FullScreen = conf.FullScreen || *fullscreen2 || *fullscreen
	}
	if conf.Title == "" {
		dir, _ := os.Getwd()
		appName := filepath.Base(dir)
		conf.Title = appName + " (by Go+ Builder)"
	}
	proj.FullScreen = proj.FullScreen || conf.FullScreen

	key := conf.ScreenshotKey
	if key == "" {
		key = os.Getenv("SPX_SCREENSHOT_KEY")
	}
	if key != "" {
		err := os.Setenv("SPX_SCREENSHOT_KEY", key)
		if err != nil {
			panic(err)
		}
	}

	v := reflect.ValueOf(game).Elem()
	g := instance(v)
	if debugLoad {
		log.Println("==> StartLoad", resource)
	}
	g.startLoad(fs, &conf)
	for i, n := 0, v.NumField(); i < n; i++ {
		name, val := getFieldPtrOrAlloc(g, v, i)
		switch fld := val.(type) {
		case *Sound:
			if g.canBindSound(name) {
				media, err := g.loadSound(name)
				if err != nil {
					panic(err)
				}
				*fld = media
			}
		case Sprite:
			if g.canBindSprite(name) {
				if err := g.loadSprite(fld, name, v); err != nil {
					panic(err)
				}
			}
			// p.sprs[name] = fld (has been set by loadSprite)
		}
	}
	if err := g.endLoad(v, &proj); err != nil {
		panic(err)
	}

	if err := g.runLoop(&conf); err != nil {
		panic(err)
	}
}

// MouseHitItem returns the topmost item which is hit by mouse.
func (p *Game) MouseHitItem() (target *SpriteImpl, ok bool) {
	//x, y := engine.GetMousePos()
	// TODO(tanjp) use engine api
	return
}

func instance(gamer reflect.Value) *Game {
	fld := gamer.FieldByName("Game")
	if !fld.IsValid() {
		log.Panicf("type %v doesn't has field spx.Game", gamer.Type())
	}
	return fld.Addr().Interface().(*Game)
}

func getFieldPtrOrAlloc(g *Game, v reflect.Value, i int) (name string, val interface{}) {
	tFld := v.Type().Field(i)
	vFld := v.Field(i)
	typ := tFld.Type
	word := unsafe.Pointer(vFld.Addr().Pointer())
	ret := reflect.NewAt(typ, word).Interface()

	if vFld.Kind() == reflect.Ptr && typ.Implements(tySprite) {
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

func findFieldPtr(v reflect.Value, name string, from int) interface{} {
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

func findObjPtr(v reflect.Value, name string, from int) interface{} {
	t := v.Type()
	for i, n := from, v.NumField(); i < n; i++ {
		tFld := t.Field(i)
		if tFld.Name == name {
			typ := tFld.Type
			vFld := v.Field(i)
			if vFld.Kind() == reflect.Ptr {
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

func (p *Game) startLoad(fs spxfs.Dir, cfg *Config) {
	p.sounds.init(p)
	p.inputs.init(p)
	p.events = make(chan event, 16)
	p.fs = fs
	p.windowWidth_ = cfg.Width
	p.windowHeight_ = cfg.Height
}

func (p *Game) canBindSprite(name string) bool {
	return hasAsset("sprites/" + name + "/index.json")
}

func (p *Game) loadSprite(sprite Sprite, name string, gamer reflect.Value) error {
	if debugLoad {
		log.Println("==> LoadSprite", name)
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

func spriteOf(sprite Sprite) *SpriteImpl {
	vSpr := reflect.ValueOf(sprite)
	if vSpr.Kind() != reflect.Ptr {
		return nil
	}
	vSpr = vSpr.Elem()
	if vSpr.Kind() != reflect.Struct || vSpr.NumField() < 1 {
		return nil
	}
	spriteField := vSpr.Field(0)
	if spriteField.Type() != reflect.TypeOf(SpriteImpl{}) {
		return nil
	}
	return spriteField.Addr().Interface().(*SpriteImpl)
}

func (p *Game) loadIndex(g reflect.Value, proj *projConfig) (err error) {
	windowScale := 1.0
	if proj.WindowScale >= 0.001 {
		windowScale = proj.WindowScale
	}
	p.windowScale = windowScale

	p.debug = proj.Debug
	if backdrops := proj.getBackdrops(); len(backdrops) > 0 {
		p.baseObj.initBackdrops("", backdrops, proj.getBackdropIndex())
		p.worldWidth_ = proj.Map.Width
		p.worldHeight_ = proj.Map.Height
		p.doWorldSize() // set world size
	} else {
		p.worldWidth_ = proj.Map.Width
		p.worldHeight_ = proj.Map.Height
		p.baseObj.initWithSize(p.worldWidth_, p.worldHeight_)
	}
	if debugLoad {
		log.Println("==> SetWorldSize", p.worldWidth_, p.worldHeight_)
	}
	p.mapMode = toMapMode(proj.Map.Mode)

	p.doWindowSize() // set window size

	if debugLoad {
		log.Println("==> SetWindowSize", p.windowWidth_, p.windowHeight_)
	}
	if p.windowWidth_ > p.worldWidth_ {
		p.windowWidth_ = p.worldWidth_
	}
	if p.windowHeight_ > p.worldHeight_ {
		p.windowHeight_ = p.worldHeight_
	}

	// fullscreen when on mobile platform
	if platform.IsMobile() || proj.FullScreen || platform.IsWeb() {
		if proj.FullScreen || platform.IsMobile() {
			platformMgr.SetWindowFullscreen(true)
		}
		winSize := platformMgr.GetWindowSize()
		scale := math.Min(winSize.X/float64(p.windowWidth_), winSize.Y/float64(p.windowHeight_))
		p.windowScale = scale
	}

	platformMgr.SetWindowSize(int64(float64(p.windowWidth_)*p.windowScale), int64(float64(p.windowHeight_)*p.windowScale))
	p.Camera.init(p)
	p.Camera.SetCameraZoom(p.windowScale)
	ui.SetWindowScale(p.windowScale)

	physicMgr.SetCollisionSystemType(!proj.CollisionByShape)

	// setup syncSprite's property
	p.syncSprite = engine.NewBackdropProxy(p, p.getCostumePath(), p.getCostumeRenderScale())
	p.setupBackdrop()
	inits := make([]Sprite, 0, len(proj.Zorder))
	for layer, v := range proj.Zorder {
		if name, ok := v.(string); ok {
			sp := p.getSpriteProtoByName(name, g)
			spr := spriteOf(sp)
			spr.setLayer(layer)
			p.addShape(spr)
			inits = append(inits, sp)
		} else {
			// not a prototype sprite
			inits = p.addSpecialShape(g, v.(specsp), inits)
		}
	}
	for _, ini := range inits {
		spr := spriteOf(ini)
		if spr != nil {
			spr.OnStart(func() {
				spr.awake()
			})
		}
		runMain(ini.Main)
	}

	if proj.Camera != nil && proj.Camera.On != "" {
		p.Camera.On__2(proj.Camera.On)
	}
	if loader, ok := g.Addr().Interface().(interface{ OnLoaded() }); ok {
		loader.OnLoaded()
	}

	p.audioId = p.sounds.allocAudio()
	if proj.Bgm != "" {
		p.Play__5(proj.Bgm, &PlayOptions{Action: PlayRewind, Loop: true, Wait: false, Music: true})
	}
	// game load success
	p.isLoaded = true
	return
}

func (p *Game) setupBackdrop() {
	var imgW, imgH, dstW, dstH float64
	imgW, imgH = p.getCostumeSize()
	worldW := float64(p.worldWidth_)
	worldH := float64(p.worldHeight_)
	imgRadio := (imgW / imgH)
	worldRadio := (worldW / worldH)
	// scale image's height to fit world's height
	isScaleHeight := imgRadio > worldRadio
	switch p.mapMode {
	default:
		dstW = worldW
		dstH = worldH
	case mapModeRepeat:
		println("TODO implement mapModeRepeat")
	case mapModeFillCut:
		if isScaleHeight {
			dstW = worldW
			dstH = dstW / imgRadio
		} else {
			dstH = worldH
			dstW = dstH * imgRadio
		}
	case mapModeFillRatio:
		if isScaleHeight {
			dstH = worldH
			dstW = dstH * imgRadio
		} else {
			dstW = worldW
			dstH = dstW / imgRadio
		}
	}
	scaleX := dstW / imgW
	scaleY := dstH / imgH
	p.scale = 1
	checkUpdateCostume(&p.baseObj)
	spriteMgr.SetScale(p.syncSprite.GetId(), mathf.NewVec2(scaleX, scaleY))
}

func (p *Game) endLoad(g reflect.Value, proj *projConfig) (err error) {
	if debugLoad {
		log.Println("==> EndLoad")
	}
	return p.loadIndex(g, proj)
}

func Gopt_Game_Reload(game Gamer, index interface{}) (err error) {
	v := reflect.ValueOf(game).Elem()
	g := instance(v)
	g.reset()
	engine.ReloadScene()
	for i, n := 0, v.NumField(); i < n; i++ {
		name, val := getFieldPtrOrAlloc(g, v, i)
		if fld, ok := val.(Sprite); ok {
			if err := g.loadSprite(fld, name, v); err != nil {
				panic(err)
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
	return
}

// -----------------------------------------------------------------------------

type specsp = map[string]interface{}

func (p *Game) addSpecialShape(g reflect.Value, v specsp, inits []Sprite) []Sprite {
	switch typ := v["type"].(string); typ {
	case "stageMonitor", "monitor":
		if sm, err := newMonitor(g, v); err == nil {
			sm.game = p
			p.addShape(sm)
		}
	case "measure":
		p.addShape(newMeasure(v))
	case "sprites":
		return p.addStageSprites(g, v, inits)
	case "sprite":
		return p.addStageSprite(g, v, inits)
	default:
		panic("addSpecialShape: unknown shape - " + typ)
	}
	return inits
}

func (p *Game) addStageSprite(g reflect.Value, v specsp, inits []Sprite) []Sprite {
	target := v["target"].(string)
	if val := findObjPtr(g, target, 0); val != nil {
		if sp, ok := val.(Sprite); ok {
			dest := spriteOf(sp)
			applySpriteProps(dest, v)
			p.addShape(dest)
			inits = append(inits, sp)
			return inits
		}
	}
	panic("addStageSprite: unexpected - " + target)
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
			isPtr := typItem.Kind() == reflect.Ptr
			if isPtr {
				typItem, typItemPtr = typItem.Elem(), typItem
			} else {
				typItemPtr = reflect.PtrTo(typItem)
			}
			if typItemPtr.Implements(tySprite) {
				spr := p.getSpriteProto(typItem, g)
				items := v["items"].([]interface{})
				n := len(items)
				newSlice := reflect.MakeSlice(typSlice, n, n)
				for i := 0; i < n; i++ {
					newItem := newSlice.Index(i)
					if isPtr {
						newItem.Set(reflect.New(typItem))
						newItem = newItem.Elem()
					}
					dest, sp := applySprite(newItem, spr, items[i].(specsp))
					p.addShape(dest)
					inits = append(inits, sp)
				}
				fldSlice.Set(newSlice)
				return inits
			}
		}
	}
	panic("addStageSprites: unexpected - " + target)
}

var (
	tySprite = reflect.TypeOf((*Sprite)(nil)).Elem()
)

// -----------------------------------------------------------------------------

func (p *Game) runLoop(cfg *Config) (err error) {
	if debugLoad {
		log.Println("==> RunLoop")
	}
	if !cfg.DontRunOnUnfocused {
		platformMgr.SetRunnableOnUnfocused(true)
	}
	p.initEventLoop()
	platformMgr.SetWindowTitle(cfg.Title)
	p.isRunned = true
	return nil
}

func (p *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return p.windowSize_()
}

type clicker interface {
	threadObj
	doWhenClick(this threadObj)
	getProxy() *engine.Sprite
	Visible() bool
}

func (p *Game) doWhenLeftButtonDown(ev *eventLeftButtonDown) {
	// add a global click cooldown
	if !p.inputs.canTriggerClickEvent(inputGlobalClickTimerId) {
		return
	}

	point := ev.Pos
	tempItems := p.getTempShapes()
	count := len(tempItems)

	var target clicker = nil
	for i := 0; i < count; i++ {
		item := tempItems[count-i-1]
		if o, ok := item.(clicker); ok {
			syncSprite := o.getProxy()
			if syncSprite != nil && o.Visible() {
				isClicked := spriteMgr.CheckCollisionWithPoint(syncSprite.GetId(), point, true)
				if isClicked {
					target = o
					break
				}
			}
		}
	}

	if target != nil {
		syncSprite := target.getProxy()
		if p.inputs.canTriggerClickEvent(syncSprite.GetId()) {
			target.doWhenClick(target)
		}
	} else {
		if p.inputs.canTriggerClickEvent(inputStageClickTimerId) {
			p.sinkMgr.doWhenClick(p)
		}
	}
}

func (p *Game) handleEvent(event event) {
	switch ev := event.(type) {

	case *eventLeftButtonDown:
		p.doWhenLeftButtonDown(ev)
	case *eventKeyDown:
		p.sinkMgr.doWhenKeyPressed(ev.Key)
	case *eventStart:
		p.sinkMgr.doWhenStart()
	case *eventTimer:
		p.sinkMgr.doWhenTimer(ev.Time)
	}
}

func (p *Game) fireEvent(ev event) {
	select {
	case p.events <- ev:
	default:
		log.Println("Event buffer is full. Skip event:", ev)
	}
}

func (p *Game) eventLoop(me coroutine.Thread) int {
	for {
		var ev event
		engine.WaitForChan(p.events, &ev)
		p.handleEvent(ev)
	}
}
func (p *Game) logicLoop(me coroutine.Thread) int {
	for {
		tempItems := p.getTempShapes()
		for _, item := range tempItems {
			if result, ok := item.(interface{ onUpdate(float64) }); ok {
				result.onUpdate(gtime.DeltaTime())
			}
		}

		targetTimer := timer.CheckTimerEvent()
		if targetTimer >= 0 {
			p.fireEvent(&eventTimer{Time: targetTimer})
		}

		engine.WaitNextFrame()
		p.showDebugPanel()
	}
}

func (p *Game) inputEventLoop(me coroutine.Thread) int {
	lastLbtnPressed := false
	keyEvents := make([]engine.KeyEvent, 0)
	for {
		curLbtnPressed := inputMgr.GetMouseState(MOUSE_BUTTON_LEFT)
		if curLbtnPressed != lastLbtnPressed {
			if lastLbtnPressed {
				p.fireEvent(&eventLeftButtonUp{Pos: p.mousePos})
			} else {
				p.fireEvent(&eventLeftButtonDown{Pos: p.mousePos})
			}
		}
		lastLbtnPressed = curLbtnPressed

		keyEvents = engine.GetKeyEvents(keyEvents)
		for _, ev := range keyEvents {
			if ev.IsPressed {
				p.fireEvent(&eventKeyDown{Key: Key(ev.Id)})
			} else {
				p.fireEvent(&eventKeyUp{Key: Key(ev.Id)})
			}
		}
		keyEvents = keyEvents[:0]
		engine.WaitNextFrame()
	}
}

func (p *Game) initEventLoop() {
	gco.Create(nil, p.eventLoop)
	gco.Create(nil, p.inputEventLoop)
	gco.Create(nil, p.logicLoop)
}

func init() {
	gco = coroutine.New()
	engine.SetCoroutines(gco)
}

var (
	gco *coroutine.Coroutines
)

type threadObj = coroutine.ThreadObj

func SchedNow() int {
	if isSchedInMain {
		if time.Now().Sub(mainSchedTime) >= time.Second*3 {
			panic("Main execution timed out. Please check if there is an infinite loop in the code.")
		}
	}
	if me := gco.Current(); me != nil {
		gco.Sched(me)
	}
	return 0
}

func Sched() int {
	if isSchedInMain {
		if time.Now().Sub(mainSchedTime) >= time.Second*3 {
			panic("Main execution timed out. Please check if there is an infinite loop in the code.")
		}
	} else {
		if me := gco.Current(); me != nil {
			if me.IsSchedTimeout(3000) {
				log.Println("For loop execution timed out. Please check if there is an infinite loop in the code.\n", debug.GetStackTrace())
				engine.WaitNextFrame()
			}
		}
	}
	return 0
}

func Forever(call func()) {
	if call == nil {
		return
	}
	for {
		call()
		engine.WaitNextFrame()
	}
}

func Repeat(loopCount int, call func()) {
	if call == nil {
		return
	}
	for i := 0; i < loopCount; i++ {
		call()
		engine.WaitNextFrame()
	}
}

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

func WaitUtil(condition func() bool) {
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

func runMain(call func()) {
	isSchedInMain = true
	mainSchedTime = time.Now()
	call()
	isSchedInMain = false
}

// -----------------------------------------------------------------------------
func (p *Game) getWindowSize() mathf.Vec2 {
	x, y := p.windowSize_()
	return mathf.NewVec2(float64(x), float64(y))
}

func (p *Game) windowSize_() (int, int) {
	if p.windowWidth_ == 0 {
		p.doWindowSize()
	}
	return p.windowWidth_, p.windowHeight_
}

func (p *Game) doWindowSize() {
	if p.windowWidth_ == 0 {
		c := p.costumes[p.costumeIndex_]
		p.windowWidth_, p.windowHeight_ = c.getSize()
	}
}

func (p *Game) worldSize_() (int, int) {
	if p.worldWidth_ == 0 {
		p.doWorldSize()
	}
	return p.worldWidth_, p.worldHeight_
}

func (p *Game) doWorldSize() {
	if p.worldWidth_ == 0 {
		c := p.costumes[p.costumeIndex_]
		p.worldWidth_, p.worldHeight_ = c.getSize()
	}
}

func (p *Game) touchingPoint(dst *SpriteImpl, x, y float64) bool {
	return dst.touchPoint(x, y)
}

func (p *Game) touchingSpriteBy(dst *SpriteImpl, name string) *SpriteImpl {
	if dst == nil {
		return nil
	}

	for _, item := range p.items {
		if sp, ok := item.(*SpriteImpl); ok && sp != dst {
			if sp.name == name && (sp.isVisible && !sp.isDying) {
				if sp.touchingSprite(dst) {
					return sp
				}

			}
		}
	}
	return nil
}

func (p *Game) objectPos(obj interface{}) (float64, float64) {
	switch v := obj.(type) {
	case SpriteName:
		if sp := p.findSprite(v); sp != nil {
			return sp.getXY()
		}
		panic("objectPos: sprite not found - " + v)
	case specialObj:
		if v == Mouse {
			return p.getMousePos()
		}
	case Pos:
		if v == Random {
			worldW, worldH := p.worldSize_()
			mx, my := rand.Intn(worldW), rand.Intn(worldH)
			return float64(mx - (worldW >> 1)), float64((worldH >> 1) - my)
		}
	case Sprite:
		return spriteOf(v).getXY()
	}
	panic("objectPos: unexpected input")
}

// -----------------------------------------------------------------------------

func (p *Game) EraseAll() {
	extMgr.DestroyAllPens()
}

// -----------------------------------------------------------------------------

func (p *Game) getItems() []Shape {
	return p.items
}

func (p *Game) addShape(child Shape) {
	p.items = append(p.items, child)
}

func (p *Game) addClonedShape(src, clone Shape) {
	items := p.items
	idx := p.doFindSprite(src)
	if idx < 0 {
		log.Println("addClonedShape: clone a deleted sprite")
		gco.Abort()
	}

	// p.getItems() requires immutable items, so we need copy before modify
	n := len(items)
	newItems := make([]Shape, n+1)
	copy(newItems[:idx], items)
	copy(newItems[idx+2:], items[idx+1:])
	newItems[idx] = clone
	newItems[idx+1] = src
	p.items = newItems
	p.updateRenderLayers()
}

func (p *Game) removeShape(child Shape) {
	items := p.items
	for i, item := range items {
		if item == child {
			// getItems() requires immutable items, so we need copy before modify
			newItems := make([]Shape, len(items)-1)
			copy(newItems, items[:i])
			copy(newItems[i:], items[i+1:])
			p.HasDestroyed = true
			p.destroyItems = append(p.destroyItems, item)
			p.items = newItems
			return
		}
	}
	p.updateRenderLayers()
}

func (p *Game) activateShape(child Shape) {
	items := p.items
	for i, item := range items {
		if item == child {
			if i == 0 {
				return
			}
			// getItems() requires immutable items, so we need copy before modify
			newItems := make([]Shape, len(items))
			copy(newItems, items[:i])
			copy(newItems[i:], items[i+1:])
			newItems[len(items)-1] = child
			p.items = newItems
			return
		}
	}
	p.updateRenderLayers()
}

func (p *Game) gotoFront(spr *SpriteImpl) {
	p.goBackLayers(spr, math.MinInt32)
}

func (p *Game) gotoBack(spr *SpriteImpl) {
	p.goBackLayers(spr, math.MaxInt32)
}

func (p *Game) goBackLayers(spr *SpriteImpl, n int) {
	idx := p.doFindSprite(spr)
	if idx < 0 {
		return
	}
	items := p.items
	// go back
	if n > 0 {
		newIdx := idx
		for newIdx > 0 {
			newIdx--
			item := items[newIdx]
			if _, ok := item.(*SpriteImpl); ok {
				n--
				if n == 0 {
					break
				}
			}
		}
		// should consider that backdrop is always at the bottom
		if newIdx != idx {
			// p.getItems() requires immutable items, so we need copy before modify
			newItems := make([]Shape, len(items))
			copy(newItems, items[:newIdx])
			copy(newItems[newIdx+1:], items[newIdx:idx])
			copy(newItems[idx+1:], items[idx+1:])
			newItems[newIdx] = spr
			p.items = newItems
		}
	} else if n < 0 { // go front
		newIdx := idx
		lastIdx := len(items) - 1
		if newIdx < lastIdx {
			for {
				newIdx++
				if newIdx >= lastIdx {
					break
				}
				item := items[newIdx]
				if _, ok := item.(*SpriteImpl); ok {
					n++
					if n == 0 {
						break
					}
				}
			}
		}
		if newIdx != idx {
			// p.getItems() requires immutable items, so we need copy before modify
			newItems := make([]Shape, len(items))
			copy(newItems, items[:idx])
			copy(newItems[idx:newIdx], items[idx+1:])
			copy(newItems[newIdx+1:], items[newIdx+1:])
			newItems[newIdx] = spr
			p.items = newItems
		}
	}
	p.updateRenderLayers()
}
func (p *Game) updateRenderLayers() {
	layer := 0
	for _, item := range p.items {
		if sp, ok := item.(*SpriteImpl); ok {
			layer++
			sp.setLayer(layer)
		}
	}
}

func (p *Game) doFindSprite(src Shape) int {
	for idx, item := range p.items {
		if item == src {
			return idx
		}
	}
	return -1
}

func (p *Game) findSprite(name SpriteName) *SpriteImpl {
	for _, item := range p.items {
		if sp, ok := item.(*SpriteImpl); ok {
			if !sp.isCloned_ && sp.name == name {
				return sp
			}
		}
	}
	return nil
}

// -----------------------------------------------------------------------------

func (p *Game) BackdropName() string {
	return p.getCostumeName()
}

func (p *Game) BackdropIndex() int {
	return p.getCostumeIndex()
}

type BackdropName = string

// StartBackdrop func:
//
//	StartBackdrop(backdrop) or
//	StartBackdrop(index) or
//	StartBackdrop(spx.Next)
//	StartBackdrop(spx.Prev)
func (p *Game) startBackdrop(backdrop interface{}, wait bool) {
	if p.goSetCostume(backdrop) {
		p.windowWidth_ = 0
		p.setupBackdrop()
		p.doWindowSize()
		p.doWhenBackdropChanged(p.getCostumeName(), wait)
	}
}

func (p *Game) StartBackdrop__0(backdrop BackdropName) {
	p.startBackdrop(backdrop, false)
}

func (p *Game) StartBackdrop__1(backdrop BackdropName, wait bool) {
	p.startBackdrop(backdrop, wait)
}

func (p *Game) StartBackdrop__2(index float64) {
	p.startBackdrop(index, false)
}

func (p *Game) StartBackdrop__3(index float64, wait bool) {
	p.startBackdrop(index, wait)
}

func (p *Game) StartBackdrop__4(index int) {
	p.startBackdrop(index, false)
}

func (p *Game) StartBackdrop__5(index int, wait bool) {
	p.startBackdrop(index, wait)
}

func (p *Game) StartBackdrop__6(action switchAction) {
	p.startBackdrop(action, false)
}

func (p *Game) StartBackdrop__7(action switchAction, wait bool) {
	p.startBackdrop(action, wait)
}

func (p *Game) NextBackdrop__0() {
	p.StartBackdrop__6(Next)
}

func (p *Game) NextBackdrop__1(wait bool) {
	p.StartBackdrop__7(Next, wait)
}

func (p *Game) PrevBackdrop__0() {
	p.StartBackdrop__6(Prev)
}

func (p *Game) PrevBackdrop__1(wait bool) {
	p.StartBackdrop__7(Prev, wait)
}

// -----------------------------------------------------------------------------

func (p *Game) KeyPressed(key Key) bool {
	return inputMgr.GetKey(int64(key))
}

func (p *Game) MouseX() float64 {
	return p.mousePos.X
}

func (p *Game) MouseY() float64 {
	return p.mousePos.Y
}

func (p *Game) MousePressed() bool {
	return inputMgr.MousePressed()
}

func (p *Game) getMousePos() (x, y float64) {
	return p.MouseX(), p.MouseY()
}

func (p *Game) Username() string {
	panic("todo")
}

// -----------------------------------------------------------------------------
func (p *Game) WaitNextFrame() float64 {
	return engine.WaitNextFrame()
}

func (p *Game) Wait(secs float64) {
	engine.Wait(secs)
}

func (p *Game) Timer() float64 {
	return timer.Timer()
}

func (p *Game) ResetTimer() {
	timer.ResetTimer()
}

// -----------------------------------------------------------------------------

func (p *Game) Ask(msgv interface{}) {
	msg, ok := msgv.(string)
	if !ok {
		msg = fmt.Sprint(msgv)
	}
	if msg == "" {
		println("ask: msg should not be empty")
		return
	}
	p.ask(false, msg, func(answer string) {})
}

func (p *Game) Answer() string {
	return p.answerVal
}

func (p *Game) ask(isSprite bool, question string, callback func(string)) {
	if p.askPanel == nil {
		p.askPanel = ui.NewUiAsk()
		p.addShape(p.askPanel)
	}
	hasAnswer := false
	p.askPanel.Show(isSprite, question, func(msg string) {
		p.answerVal = msg
		callback(msg)
		hasAnswer = true
	})
	for {
		if hasAnswer {
			break
		}
		engine.WaitNextFrame()
	}
}

// -----------------------------------------------------------------------------

type EffectKind int

const (
	ColorEffect EffectKind = iota
	FishEyeEffect
	WhirlEffect
	PixelateEffect
	MosaicEffect
	BrightnessEffect
	GhostEffect

	enumNumOfEffect // max index of enum
)

var greffNames = []string{
	ColorEffect:      "color_amount",
	FishEyeEffect:    "fisheye_amount",
	WhirlEffect:      "whirl_amount",
	MosaicEffect:     "uv_amount",
	PixelateEffect:   "pixleate_amount",
	BrightnessEffect: "brightness_amount",
	GhostEffect:      "alpha_amount",
}

func (kind EffectKind) String() string {
	return greffNames[kind]
}

func (p *Game) SetEffect(kind EffectKind, val float64) {
	p.baseObj.setEffect(kind, val)
}

func (p *Game) ChangeEffect(kind EffectKind, delta float64) {
	p.baseObj.changeEffect(kind, delta)
}

func (p *Game) ClearGraphEffects() {
	p.baseObj.clearGraphEffects()
}

// -----------------------------------------------------------------------------

type Sound *soundConfig

type SoundName = string

func hasAsset(path string) bool {
	finalPath := engine.ToAssetPath(path)
	return resMgr.HasFile(finalPath)
}

func (p *Game) canBindSound(name string) bool {
	return hasAsset("sounds/" + name + "/index.json")
}

func (p *Game) loadSound(name SoundName) (media Sound, err error) {
	if media, ok := p.sounds.audios[name]; ok {
		return media, nil
	}

	if debugLoad {
		log.Println("==> LoadSound", name)
	}
	prefix := "sounds/" + name
	media = new(soundConfig)
	if err = loadJson(media, p.fs, prefix+"/index.json"); err != nil {
		println("loadSound failed:", err.Error())
		return
	}
	media.Path = prefix + "/" + media.Path
	p.sounds.audios[name] = media
	return
}

func (p *Game) play(audioId engine.Object, media Sound, opts *PlayOptions) (err error) {
	return p.sounds.play(audioId, media, opts)
}

// Play func:
//
//	Play(sound)
//	Play(video) -- maybe
//	Play(media, wait) -- sync
//	Play(media, opts)

func (p *Game) Play__0(media Sound, action *PlayOptions) {
	if debugInstr {
		log.Println("Play", media.Path)
	}

	p.checkAudioId()
	err := p.play(p.audioId, media, action)
	if err != nil {
		panic(err)
	}
}

func (p *Game) Play__1(media Sound, wait bool) {
	p.Play__0(media, &PlayOptions{Wait: wait})
}

func (p *Game) Play__2(media Sound) {
	if media == nil {
		panic("play media is nil")
	}
	p.Play__0(media, &PlayOptions{})
}

func (p *Game) Play__3(media SoundName) {
	p.Play__5(media, &PlayOptions{})
}

func (p *Game) Play__4(media SoundName, wait bool) {
	p.Play__5(media, &PlayOptions{Wait: wait})
}

func (p *Game) Play__5(media SoundName, action *PlayOptions) {
	m, err := p.loadSound(media)
	if err != nil {
		log.Println(err)
		return
	}
	p.Play__0(m, action)
}

func (p *Game) SetVolume(volume float64) {
	p.checkAudioId()
	p.sounds.setVolume(p.audioId, volume)
}

func (p *Game) ChangeVolume(delta float64) {
	p.checkAudioId()
	p.sounds.changeVolume(p.audioId, delta)
}

func (p *Game) GetSoundEffect(kind SoundEffectKind) float64 {
	p.checkAudioId()
	return p.sounds.getEffect(p.audioId, kind)
}
func (p *Game) SetSoundEffect(kind SoundEffectKind, value float64) {
	p.checkAudioId()
	p.sounds.setEffect(p.audioId, kind, value)
}
func (p *Game) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	p.checkAudioId()
	p.sounds.changeEffect(p.audioId, kind, delta)
}
func (p *Game) checkAudioId() {
	if p.audioId == 0 {
		p.audioId = p.sounds.allocAudio()
	}
}

func (p *Game) ClearSoundEffects() {
	panic("todo")
}

func (p *Game) StopAllSounds() {
	p.sounds.stopAll()
}

func (p *Game) Loudness() float64 {
	if p.aurec == nil {
		p.aurec = audiorecord.Open(gco)
	}
	return p.aurec.Loudness() * 100
}

// -----------------------------------------------------------------------------

func (p *Game) doBroadcast(msg string, data interface{}, wait bool) {
	if debugInstr {
		log.Println("Broadcast", msg, wait)
	}
	p.sinkMgr.doWhenIReceive(msg, data, wait)
}

func (p *Game) Broadcast__0(msg string) {
	p.doBroadcast(msg, nil, false)
}

func (p *Game) Broadcast__1(msg string, wait bool) {
	p.doBroadcast(msg, nil, wait)
}

func (p *Game) Broadcast__2(msg string, data interface{}, wait bool) {
	p.doBroadcast(msg, data, wait)
}

// -----------------------------------------------------------------------------

func (p *Game) setStageMonitor(target string, val string, visible bool) {
	for _, item := range p.items {
		if sp, ok := item.(*Monitor); ok && sp.val == val && sp.target == target {
			sp.setVisible(visible)
			return
		}
	}
}

func (p *Game) HideVar(name string) {
	p.setStageMonitor("", getVarPrefix+name, false)
}

func (p *Game) ShowVar(name string) {
	p.setStageMonitor("", getVarPrefix+name, true)
}

func (p *Game) getAllShapes() []Shape {
	return p.items
}

func (p *Game) getTempShapes() []Shape {
	p.tempItems = getTempShapes(p.tempItems, p.items)
	return p.tempItems
}

func getTempShapes(dst []Shape, src []Shape) []Shape {
	if dst == nil {
		dst = make([]Shape, 50)
	}
	dst = dst[:0]
	if cap(dst) < len(src) {
		dst = make([]Shape, len(src))
	} else {
		dst = dst[:len(src)]
	}
	copy(dst, src)
	return dst
}

// -----------------------------------------------------------------------------
// Widget

type ShapeGetter interface {
	getAllShapes() []Shape
}

// GetWidget_ returns the widget instance with given name. It panics if not found.
// Instead of being used directly, it is meant to be called by `Gopt_Game_Gopx_GetWidget` only.
// We extract `GetWidget_` to keep `Gopt_Game_Gopx_GetWidget` simple, which simplifies work in ispx,
// see details in https://github.com/goplus/builder/issues/765#issuecomment-2313915805.
func GetWidget_(sg ShapeGetter, name WidgetName) Widget {
	items := sg.getAllShapes()
	for _, item := range items {
		widget, ok := item.(Widget)
		if ok && widget.GetName() == name {
			return widget
		}
	}
	panic("GetWidget: widget not found - " + name)
}

// GetWidget returns the widget instance (in given type) with given name. It panics if not found.
func Gopt_Game_Gopx_GetWidget[T any](sg ShapeGetter, name WidgetName) *T {
	widget := GetWidget_(sg, name)
	if result, ok := widget.(interface{}).(*T); ok {
		return result
	} else {
		panic("GetWidget: type mismatch")
	}
}
