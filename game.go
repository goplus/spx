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
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/goplus/spx/internal/audiorecord"
	"github.com/goplus/spx/internal/coroutine"
	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/ui"

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

func SetDebug(flags dbgFlags) {
	debugLoad = (flags & DbgFlagLoad) != 0
	debugInstr = (flags & DbgFlagInstr) != 0
	debugEvent = (flags & DbgFlagEvent) != 0
	debugPerf = (flags & DbgFlagPerf) != 0
}

// -------------------------------------------------------------------------------------

type Game struct {
	baseObj
	eventSinks
	Camera

	fs spxfs.Dir

	sounds       soundMgr
	turtle       turtleCanvas
	typs         map[string]reflect.Type // map: name => sprite type, for all sprites
	sprs         map[string]Sprite       // map: name => sprite prototype, for loaded sprites
	items        []Shape                 // shapes on stage (in Zorder), not only sprites
	destroyItems []Shape                 // shapes on stage (in Zorder), not only sprites

	tickMgr   tickMgr
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

	gMouseX, gMouseY int64

	sinkMgr  eventSinkMgr
	isLoaded bool
	isRunned bool
	gamer_   Gamer
}

type Gamer interface {
	engine.Gamer
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
	p.sinkMgr.reset()
	p.startFlag = sync.Once{}
	p.Stop(AllOtherScripts)
	p.items = nil
	p.destroyItems = nil
	p.isLoaded = false
	p.sprs = make(map[string]Sprite)
}

func (p *Game) getGame() *Game {
	return p
}

func (p *Game) initGame(sprites []Sprite) *Game {
	p.tickMgr.init()
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
	engine.GdspxMain(game)
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
		projectPath := f.Bool("path", false, "gdspx project path")
		editorMode := f.Bool("e", false, "editor mode")
		flag.Parse()
		if *projectPath || *editorMode {
			println("======== gdspx debug mode ========")
		}
		if *help {
			fmt.Fprintf(os.Stderr, "Usage: %v [-v -f -h]\n", os.Args[0])
			flag.PrintDefaults()
			return
		}
		if *verbose {
			SetDebug(DbgFlagAll)
		}
		conf.FullScreen = *fullscreen
	}
	if conf.Title == "" {
		dir, _ := os.Getwd()
		appName := filepath.Base(dir)
		conf.Title = appName + " (by Go+ Builder)"
	}

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
	p.events = make(chan event, 16)
	p.fs = fs
	p.windowWidth_ = cfg.Width
	p.windowHeight_ = cfg.Height
}

func (p *Game) canBindSprite(name string) bool {
	// auto bind the sprite, if assets/sprites/{name}/index.json exists.
	var baseDir = "sprites/" + name + "/"
	f, err := p.fs.Open(baseDir + "index.json")
	if err != nil {
		return false
	}
	defer f.Close()
	return true
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
	engine.SyncSetDebugMode(proj.Debug)
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
	// setup proxy's property
	p.proxy = engine.SyncNewBackdropProxy(p, p.getCostumePath())

	p.doWindowSize() // set window size

	ui.WinX = float64(p.windowWidth_)
	ui.WinY = float64(p.windowHeight_)

	inits := make([]Sprite, 0, len(proj.Zorder))
	for _, v := range proj.Zorder {
		if name, ok := v.(string); ok {
			sp := p.getSpriteProtoByName(name, g)
			p.addShape(spriteOf(sp))
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
		ini.Main()
	}

	if debugLoad {
		log.Println("==> SetWindowSize", p.windowWidth_, p.windowHeight_)
	}
	engine.SyncPlatformSetWindowSize(int64(p.windowWidth_), int64(p.windowHeight_))
	if p.windowWidth_ > p.worldWidth_ {
		p.windowWidth_ = p.worldWidth_
	}
	if p.windowHeight_ > p.worldHeight_ {
		p.windowHeight_ = p.worldHeight_
	}
	p.Camera.init(p, float64(p.windowWidth_), float64(p.windowHeight_), float64(p.worldWidth_), float64(p.worldHeight_))

	if proj.Camera != nil && proj.Camera.On != "" {
		p.Camera.On(proj.Camera.On)
	}
	if loader, ok := g.Addr().Interface().(interface{ OnLoaded() }); ok {
		loader.OnLoaded()
	}
	// game load success
	p.isLoaded = true
	return
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
	return g.loadIndex(v, &proj)
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
		engine.SyncSetRunnableOnUnfocused(true)
	}
	if cfg.FullScreen {
		engine.SyncPlatformSetWindowFullscreen(true)
	}
	p.initEventLoop()
	engine.SyncPlatformSetWindowTitle(cfg.Title)
	p.isRunned = true
	return nil
}

func (p *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return p.windowSize_()
}

// startTick creates tickHandler to handle `onTick` event.
// You can call tickHandler.Stop to stop listening `onTick` event.
func (p *Game) startTick(duration int64, onTick func(tick int64)) *tickHandler {
	return p.tickMgr.start(duration, onTick)
}

// currentTPS returns the current TPS (ticks per second),
// that represents how many update function is called in a second.
func (p *Game) currentTPS() float64 {
	return p.tickMgr.getCurrentTPS()
}

type clicker interface {
	threadObj
	doWhenClick(this threadObj)
	getProxy() *engine.ProxySprite
}

func (p *Game) doWhenLeftButtonDown(ev *eventLeftButtonDown) {
	point := engine.NewVec2(float64(ev.X), float64(ev.Y))
	// TOOD(tanjp) avoid new a array every frame
	newItems := make([]Shape, len(p.items))
	copy(newItems, p.items)
	for _, item := range newItems {
		if o, ok := item.(clicker); ok {
			proxy := o.getProxy()
			if proxy != nil {
				isClicked := engine.SyncSpriteCheckCollisionWithPoint(proxy.GetId(), point, true)
				if isClicked {
					o.doWhenClick(o)
				}
			}
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
		go func() {
			ev = <-p.events
			gco.Resume(me)
		}()
		gco.Yield(me)
		p.handleEvent(ev)
	}
}

func (p *Game) logicLoop(me coroutine.Thread) int {
	lastLbtnPressed := false
	for {
		p.Wait(0.01)
		curLbtnPressed := engine.SyncInputGetMouseState(MOUSE_BUTTON_LEFT)
		if curLbtnPressed != lastLbtnPressed {
			if lastLbtnPressed {
				p.fireEvent(&eventLeftButtonUp{X: int(p.gMouseX), Y: int(p.gMouseY)})
			} else {
				p.fireEvent(&eventLeftButtonDown{X: int(p.gMouseX), Y: int(p.gMouseY)})
			}
		}
		lastLbtnPressed = curLbtnPressed
	}
}

func (p *Game) initEventLoop() {
	gco.Create(nil, p.eventLoop)
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

func waitToDo(fn func()) {
	me := gco.Current()
	go func() {
		fn()
		gco.Resume(me)
	}()
	gco.Yield(me)
}

func waitForChan(done chan bool) {
	me := gco.Current()
	go func() {
		<-done
		gco.Resume(me)
	}()
	gco.Yield(me)
}

func SchedNow() int {
	if me := gco.Current(); me != nil {
		gco.Sched(me)
	}
	return 0
}

func Sched() int {
	now := time.Now()
	if now.Sub(lastSched) >= 3e7 {
		if me := gco.Current(); me != nil {
			gco.Sched(me)
		}
		lastSched = now
	}
	return 0
}

var lastSched time.Time

// -----------------------------------------------------------------------------

func (p *Game) getWidth() int {
	if p.windowWidth_ == 0 {
		p.doWindowSize()
	}
	return p.windowWidth_
}

// convert pos from win space(0,0 is top left) to game space(0,0 is center)
func (p *Game) convertWinSpace2GameSpace(x, y float64) (float64, float64) {
	winW, winH := p.getWindowSize()
	x += float64(winW) / 2
	y = float64(winH)/2 - y
	return x, y
}

func (p *Game) getWindowSize() (int, int) {
	return p.windowSize_()
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
	case string:
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

func (p *Game) getTurtle() turtleCanvas {
	return p.turtle
}

func (p *Game) stampCostume(di *SpriteImpl) {
	p.turtle.stampCostume(di)
}

func (p *Game) movePen(sp *SpriteImpl, x, y float64) {
	worldW, worldH := p.worldSize_()
	p.turtle.penLine(&penLine{
		x1:    (worldW >> 1) + int(sp.x),
		y1:    (worldH >> 1) - int(sp.y),
		x2:    (worldW >> 1) + int(x),
		y2:    (worldH >> 1) - int(y),
		clr:   sp.penColor,
		width: int(sp.penWidth),
	})
}

func (p *Game) EraseAll() {
	p.turtle.eraseAll()
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
}

func (p *Game) goBackByLayers(spr *SpriteImpl, n int) {
	idx := p.doFindSprite(spr)
	if idx < 0 {
		return
	}
	items := p.items
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
		if newIdx != idx {
			// p.getItems() requires immutable items, so we need copy before modify
			newItems := make([]Shape, len(items))
			copy(newItems, items[:newIdx])
			copy(newItems[newIdx+1:], items[newIdx:idx])
			copy(newItems[idx+1:], items[idx+1:])
			newItems[newIdx] = spr
			p.items = newItems
		}
	} else if n < 0 {
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
}

func (p *Game) doFindSprite(src Shape) int {
	for idx, item := range p.items {
		if item == src {
			return idx
		}
	}
	return -1
}

func (p *Game) findSprite(name string) *SpriteImpl {
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

// StartBackdrop func:
//
//	StartBackdrop(backdropName) or
//	StartBackdrop(backdropIndex) or
//	StartBackdrop(spx.Next)
//	StartBackdrop(spx.Prev)
func (p *Game) StartBackdrop(backdrop interface{}, wait ...bool) {
	if p.goSetCostume(backdrop) {
		p.windowWidth_ = 0
		p.doWindowSize()
		p.doWhenBackdropChanged(p.getCostumeName(), wait != nil && wait[0])
	}
}

func (p *Game) NextBackdrop(wait ...bool) {
	p.StartBackdrop(Next, wait...)
}

func (p *Game) PrevBackdrop(wait ...bool) {
	p.StartBackdrop(Prev, wait...)
}

// -----------------------------------------------------------------------------

func (p *Game) KeyPressed(key Key) bool {
	return engine.SyncInputGetKey(key)
}

func (p *Game) MouseX() float64 {
	return float64(atomic.LoadInt64(&p.gMouseX))
}

func (p *Game) MouseY() float64 {
	return float64(atomic.LoadInt64(&p.gMouseY))
}

func (p *Game) MousePressed() bool {
	return engine.SyncInputMousePressed()
}

func (p *Game) getMousePos() (x, y float64) {
	return p.MouseX(), p.MouseY()
}

func (p *Game) Username() string {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Game) Wait(secs float64) {
	gco.Sleep(time.Duration(secs * 1e9))
}

func (p *Game) Timer() float64 {
	panic("todo")
}

func (p *Game) ResetTimer() {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Game) Ask(msg interface{}) {
	panic("todo")
}

func (p *Game) Answer() Value {
	panic("todo")
}

// -----------------------------------------------------------------------------

type EffectKind int

const (
	ColorEffect EffectKind = iota
	BrightnessEffect
	GhostEffect
)

var greffNames = []string{
	ColorEffect:      "Color",
	BrightnessEffect: "Brightness",
	GhostEffect:      "Ghost",
}

func (kind EffectKind) String() string {
	return greffNames[kind]
}

func (p *Game) SetEffect(kind EffectKind, val float64) {
	panic("todo")
}

func (p *Game) ChangeEffect(kind EffectKind, delta float64) {
	panic("todo")
}

func (p *Game) ClearSoundEffects() {
	panic("todo")
}

// -----------------------------------------------------------------------------

type Sound *soundConfig

func (p *Game) canBindSound(name string) bool {
	// auto bind the sound, if assets/sounds/{name}/index.json exists.
	prefix := "sounds/" + name
	f, err := p.fs.Open(prefix + "/index.json")
	if err != nil {
		return false
	}
	defer f.Close()
	return true
}

func (p *Game) loadSound(name string) (media Sound, err error) {
	if media, ok := p.sounds.audios[name]; ok {
		return media, nil
	}

	if debugLoad {
		log.Println("==> LoadSound", name)
	}
	prefix := "sounds/" + name
	media = new(soundConfig)
	if err = loadJson(media, p.fs, prefix+"/index.json"); err != nil {
		return
	}
	media.Path = prefix + "/" + media.Path
	p.sounds.audios[name] = media
	return
}

// Play func:
//
//	Play(sound)
//	Play(video) -- maybe
//	Play(media, wait) -- sync
//	Play(media, opts)
//	Play(mediaName)
//	Play(mediaName, wait) -- sync
//	Play(mediaName, opts)
func (p *Game) Play__0(media Sound) {
	p.Play__2(media, &PlayOptions{})
}

func (p *Game) Play__1(media Sound, wait bool) {
	p.Play__2(media, &PlayOptions{Wait: wait})
}

func (p *Game) Play__2(media Sound, action *PlayOptions) {
	if debugInstr {
		log.Println("Play", media.Path)
	}

	err := p.sounds.play(media, action)
	if err != nil {
		panic(err)
	}
}
func (p *Game) Play__3(mediaName string) {
	p.Play__5(mediaName, &PlayOptions{})
}

func (p *Game) Play__4(mediaName string, wait bool) {
	p.Play__5(mediaName, &PlayOptions{Wait: wait})
}

func (p *Game) Play__5(mediaName string, action *PlayOptions) {
	media, err := p.loadSound(mediaName)
	if err != nil {
		log.Println(err)
		return
	}
	p.Play__2(media, action)
}

func (p *Game) StopAllSounds() {
	p.sounds.stopAll()
}

func (p *Game) Volume() float64 {
	return p.sounds.getVolume()
}

func (p *Game) SetVolume(volume float64) {
	p.sounds.setVolume(volume)
}

func (p *Game) ChangeVolume(delta float64) {
	p.sounds.changeVolume(delta)
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

// -----------------------------------------------------------------------------
// Widget

type ShapeGetter interface {
	getAllShapes() []Shape
}

// GetWidget_ returns the widget instance with given name. It panics if not found.
// Instead of being used directly, it is meant to be called by `Gopt_Game_Gopx_GetWidget` only.
// We extract `GetWidget_` to keep `Gopt_Game_Gopx_GetWidget` simple, which simplifies work in ispx,
// see details in https://github.com/goplus/builder/issues/765#issuecomment-2313915805.
func GetWidget_(sg ShapeGetter, name string) Widget {
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
func Gopt_Game_Gopx_GetWidget[T any](sg ShapeGetter, name string) *T {
	widget := GetWidget_(sg, name)
	if result, ok := widget.(interface{}).(*T); ok {
		return result
	} else {
		panic("GetWidget: type mismatch")
	}
}
