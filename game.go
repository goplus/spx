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
	"image"
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/goplus/spx/internal/audiorecord"
	"github.com/goplus/spx/internal/coroutine"
	"github.com/goplus/spx/internal/gdi"
	"github.com/goplus/spx/internal/math32"
	"github.com/hajimehoshi/ebiten/v2"

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
	DbgFlagAll = DbgFlagLoad | DbgFlagInstr | DbgFlagEvent
)

var (
	debugInstr bool
	debugLoad  bool
	debugEvent bool
)

func SetDebug(flags dbgFlags) {
	debugLoad = (flags & DbgFlagLoad) != 0
	debugInstr = (flags & DbgFlagInstr) != 0
	debugEvent = (flags & DbgFlagEvent) != 0
}

// -------------------------------------------------------------------------------------

type Game struct {
	baseObj
	eventSinks
	Camera

	fs     spxfs.Dir
	shared *sharedImages

	sounds soundMgr
	turtle turtleCanvas
	typs   map[string]reflect.Type // map: name => sprite type, for all sprites
	sprs   map[string]Spriter      // map: name => sprite prototype, for loaded sprites
	items  []Shape                 // shapes on stage (in Zorder), not only sprites

	tickMgr tickMgr
	input   inputMgr
	events  chan event
	aurec   *audiorecord.Recorder

	// map world
	worldWidth_  int
	worldHeight_ int
	mapMode      int
	world        *ebiten.Image

	// window
	windowWidth_  int
	windowHeight_ int

	gMouseX, gMouseY int64

	sinkMgr  eventSinkMgr
	isLoaded bool
	isRunned bool
}

type Spriter interface {
	Shape
	Main()
}

type Gamer interface {
	initGame(sprites []Spriter) *Game
}

func (p *Game) IsRunned() bool {
	return p.isRunned
}

func (p *Game) getSharedImgs() *sharedImages {
	if p.shared == nil {
		p.shared = &sharedImages{imgs: make(map[string]gdi.Image)}
	}
	return p.shared
}

func (p *Game) newSpriteAndLoad(name string, tySpr reflect.Type, g reflect.Value) Spriter {
	spr := reflect.New(tySpr).Interface().(Spriter)
	if err := p.loadSprite(spr, name, g); err != nil {
		panic(err)
	}
	// p.sprs[name] = spr (has been set by loadSprite)
	return spr
}

func (p *Game) getSpriteProto(tySpr reflect.Type, g reflect.Value) Spriter {
	name := tySpr.Name()
	spr, ok := p.sprs[name]
	if !ok {
		spr = p.newSpriteAndLoad(name, tySpr, g)
	}
	return spr
}

func (p *Game) getSpriteProtoByName(name string, g reflect.Value) Spriter {
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
	p.input.reset()
	p.Stop(AllOtherScripts)
	p.items = nil
	p.isLoaded = false
	p.sprs = make(map[string]Spriter)
}

func (p *Game) initGame(sprites []Spriter) *Game {
	p.tickMgr.init()
	p.eventSinks.init(&p.sinkMgr, p)
	p.sprs = make(map[string]Spriter)
	p.typs = make(map[string]reflect.Type)
	for _, spr := range sprites {
		tySpr := reflect.TypeOf(spr).Elem()
		p.typs[tySpr.Name()] = tySpr
	}
	return p
}

// Gopt_Game_Main is required by Go+ compiler as the entry of a .gmx project.
func Gopt_Game_Main(game Gamer, sprites ...Spriter) {
	g := game.initGame(sprites)
	if me, ok := game.(interface{ MainEntry() }); ok {
		me.MainEntry()
	}
	if !g.isRunned {
		Gopt_Game_Run(game, "assets")
	}
}

// Gopt_Game_Run runs the game.
// resource can be a string or fs.Dir object.
func Gopt_Game_Run(game Gamer, resource interface{}, gameConf ...*Config) {
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
		flag.Parse()
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
		err := os.Setenv("EBITEN_SCREENSHOT_KEY", key)
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
		name, val := getFieldPtrOrAlloc(v, i)
		switch fld := val.(type) {
		case *Sound:
			media, err := g.loadSound(name)
			if err != nil {
				panic(err)
			}
			*fld = media
		case Spriter:
			if err := g.loadSprite(fld, name, v); err != nil {
				panic(err)
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
func (p *Game) MouseHitItem() (target *Sprite, ok bool) {
	x, y := p.input.mouseXY()
	hc := hitContext{Pos: image.Pt(x, y)}
	item, ok := p.onHit(hc)
	if ok {
		target, ok = item.Target.(*Sprite)
	}
	return
}

func instance(gamer reflect.Value) *Game {
	fld := gamer.FieldByName("Game")
	if !fld.IsValid() {
		log.Panicf("type %v doesn't has field spx.Game", gamer.Type())
	}
	return fld.Addr().Interface().(*Game)
}

func getFieldPtrOrAlloc(v reflect.Value, i int) (name string, val interface{}) {
	tFld := v.Type().Field(i)
	vFld := v.Field(i)
	typ := tFld.Type
	word := unsafe.Pointer(vFld.Addr().Pointer())
	ret := reflect.NewAt(typ, word).Interface()
	if vFld.Kind() == reflect.Ptr && typ.Implements(tySpriter) {
		obj := reflect.New(typ.Elem())
		reflect.ValueOf(ret).Elem().Set(obj)
		ret = obj.Interface()
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
			word := unsafe.Pointer(vFld.Addr().Pointer())
			return reflect.NewAt(typ, word).Interface()
		}
	}
	return nil
}

func (p *Game) startLoad(fs spxfs.Dir, cfg *Config) {
	var keyDuration int
	if cfg != nil {
		keyDuration = cfg.KeyDuration
	}
	p.input.init(p, keyDuration)
	p.sounds.init(p)
	p.events = make(chan event, 16)
	p.fs = fs
	p.windowWidth_ = cfg.Width
	p.windowHeight_ = cfg.Height
}

func (p *Game) loadSprite(sprite Spriter, name string, gamer reflect.Value) error {
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
	base := vSpr.Field(0).Addr().Interface().(*Sprite)
	base.init(baseDir, p, name, &conf, gamer, p.getSharedImgs())
	p.sprs[name] = sprite
	//
	// init gamer pointer (field 1)
	*(*uintptr)(unsafe.Pointer(vSpr.Field(1).Addr().Pointer())) = gamer.Addr().Pointer()
	return nil
}

func spriteOf(sprite Spriter) *Sprite {
	vSpr := reflect.ValueOf(sprite).Elem()
	return vSpr.Field(0).Addr().Interface().(*Sprite)
}

func (p *Game) loadIndex(g reflect.Value, proj *projConfig) (err error) {
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
	p.world = ebiten.NewImage(p.worldWidth_, p.worldHeight_)
	p.mapMode = toMapMode(proj.Map.Mode)

	inits := make([]Spriter, 0, len(proj.Zorder))
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
		ini.Main()
	}

	p.doWindowSize() // set window size
	if debugLoad {
		log.Println("==> SetWindowSize", p.windowWidth_, p.windowHeight_)
	}
	ebiten.SetWindowSize(p.windowWidth_, p.windowHeight_)
	if p.windowWidth_ > p.worldWidth_ {
		p.windowWidth_ = p.worldWidth_
	}
	if p.windowHeight_ > p.worldHeight_ {
		p.windowHeight_ = p.worldHeight_
	}
	p.Camera.init(p, float64(p.windowWidth_), float64(p.windowHeight_), float64(p.worldWidth_), float64(p.worldHeight_))

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeOnlyFullscreenEnabled)
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
		name, val := getFieldPtrOrAlloc(v, i)
		if fld, ok := val.(Spriter); ok {
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

func (p *Game) addSpecialShape(g reflect.Value, v specsp, inits []Spriter) []Spriter {
	switch typ := v["type"].(string); typ {
	case "stageMonitor":
		if sm, err := newStageMonitor(g, v); err == nil {
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

func (p *Game) addStageSprite(g reflect.Value, v specsp, inits []Spriter) []Spriter {
	target := v["target"].(string)
	if val := findObjPtr(g, target, 0); val != nil {
		if sp, ok := val.(Spriter); ok {
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
func (p *Game) addStageSprites(g reflect.Value, v specsp, inits []Spriter) []Spriter {
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
			if typItemPtr.Implements(tySpriter) {
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
	tySpriter = reflect.TypeOf((*Spriter)(nil)).Elem()
)

// -----------------------------------------------------------------------------

func (p *Game) runLoop(cfg *Config) (err error) {
	if debugLoad {
		log.Println("==> RunLoop")
	}
	if !cfg.DontRunOnUnfocused {
		ebiten.SetRunnableOnUnfocused(true)
	}
	if cfg.FullScreen {
		ebiten.SetFullscreen(true)
	}
	p.isRunned = true
	p.initEventLoop()
	ebiten.SetWindowTitle(cfg.Title)
	return ebiten.RunGame(p)
}

func (p *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return p.windowSize_()
}

func (p *Game) Update() error {
	if !p.isLoaded {
		return nil
	}
	p.input.update()
	p.updateMousePos()
	p.sounds.update()
	p.tickMgr.update()
	return nil
}

// startTick creates tickHandler to handle `onTick` event.
// You can call tickHandler.Stop to stop listening `onTick` event.
func (p *Game) startTick(duration int64, onTick func(tick int64)) *tickHandler {
	return p.tickMgr.start(duration, onTick)
}

// currentTPS returns the current TPS (ticks per second),
// that represents how many update function is called in a second.
func (p *Game) currentTPS() float64 {
	return p.tickMgr.currentTPS
}

func (p *Game) Draw(screen *ebiten.Image) {
	dc := drawContext{Image: p.world}
	p.onDraw(dc)
	p.Camera.render(dc.Image, screen)
}

type clicker interface {
	threadObj
	doWhenClick(this threadObj)
}

func (p *Game) doWhenLeftButtonDown(ev *eventLeftButtonDown) {
	hc := hitContext{Pos: image.Pt(ev.X, ev.Y)}
	if hr, ok := p.onHit(hc); ok {
		if o, ok := hr.Target.(clicker); ok {
			o.doWhenClick(o)
		}
	}
}

func (p *Game) handleEvent(event event) {
	switch ev := event.(type) {

	case *eventLeftButtonDown:
		p.updateMousePos()
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

func (p *Game) initEventLoop() {
	gco.Create(nil, p.eventLoop)
}

func init() {
	gco = coroutine.New()
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

func (p *Game) windowSize_() (int, int) {
	if p.windowWidth_ == 0 {
		p.doWindowSize()
	}
	return p.windowWidth_, p.windowHeight_
}

func (p *Game) doWindowSize() {
	if p.windowWidth_ == 0 {
		c := p.costumes[p.costumeIndex_]
		img, _, _ := c.needImage(p.fs)
		w, h := img.Size()
		p.windowWidth_, p.windowHeight_ = w/c.bitmapResolution, h/c.bitmapResolution
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
		img, _, _ := c.needImage(p.fs)
		w, h := img.Size()
		p.worldWidth_, p.worldHeight_ = w/c.bitmapResolution, h/c.bitmapResolution
	}
}

func (p *Game) touchingPoint(dst *Sprite, x, y float64) bool {
	return dst.touchPoint(x, y)
}

func (p *Game) touchingSpriteBy(dst *Sprite, name string) *Sprite {
	if dst == nil {
		return nil
	}

	for _, item := range p.items {
		if sp, ok := item.(*Sprite); ok && sp != dst {
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
	case Spriter:
		return spriteOf(v).getXY()
	}
	panic("objectPos: unexpected input")
}

// -----------------------------------------------------------------------------

func (p *Game) getTurtle() turtleCanvas {
	return p.turtle
}

func (p *Game) stampCostume(di *spriteDrawInfo) {
	p.turtle.stampCostume(di)
}

func (p *Game) movePen(sp *Sprite, x, y float64) {
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

func (p *Game) goBackByLayers(spr *Sprite, n int) {
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
			if _, ok := item.(*Sprite); ok {
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
				if _, ok := item.(*Sprite); ok {
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

func (p *Game) findSprite(name string) *Sprite {
	for _, item := range p.items {
		if sp, ok := item.(*Sprite); ok {
			if !sp.isCloned_ && sp.name == name {
				return sp
			}
		}
	}
	return nil
}

// -----------------------------------------------------------------------------

func (p *Game) drawBackground(dc drawContext) {
	c := p.costumes[p.costumeIndex_]
	img, _, _ := c.needImage(p.fs)
	options := new(ebiten.DrawTrianglesOptions)
	options.Filter = ebiten.FilterLinear

	if p.mapMode == mapModeRepeat {
		bgImage := img.Ebiten()
		imgW := float64(img.Bounds().Dx())
		imgH := float64(img.Bounds().Dy())
		winW := float64(p.windowWidth_)
		winH := float64(p.windowHeight_)
		numW := int(math.Ceil(winW/imgW/2 - 0.5))
		numH := int(math.Ceil(winH/imgH/2 - 0.5))
		rawOffsetW := float64(p.worldWidth_-p.windowWidth_) / 2.0
		rawOffsetH := float64(p.worldHeight_-p.windowHeight_) / 2.0
		offsetW := rawOffsetW + winW*0.5 - imgW*0.5 // draw from center
		offsetH := rawOffsetH + winH*0.5 - imgH*0.5
		for w := -numW; w <= numW; w++ {
			for h := -numH; h <= numH; h++ {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(imgW*float64(w)+offsetW, imgH*float64(h)+offsetH)
				dc.DrawImage(bgImage, op)
			}
		}

	} else {
		var imgW, imgH, dstW, dstH float32
		imgW = float32(img.Bounds().Dx())
		imgH = float32(img.Bounds().Dy())
		worldW := float32(p.worldWidth_)
		worldH := float32(p.worldHeight_)

		options.Address = ebiten.AddressClampToZero
		imgRadio := (imgW / imgH)
		worldRadio := (worldW / worldH)
		// scale image's height to fit world's height
		isScaleHeight := imgRadio > worldRadio
		switch p.mapMode {
		default:
			dstW = worldW
			dstH = worldH
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

		var cx, cy float32
		cx = (worldW - dstW) / 2.0
		cy = (worldH - dstH) / 2.0
		vs := []ebiten.Vertex{
			{
				DstX: cx, DstY: cy, SrcX: 0, SrcY: 0,
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
			},
			{
				DstX: dstW + cx, DstY: cy, SrcX: imgW, SrcY: 0,
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
			},
			{
				DstX: cx, DstY: dstH + cy, SrcX: 0, SrcY: imgH,
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
			},
			{
				DstX: dstW + cx, DstY: dstH + cy, SrcX: imgW, SrcY: imgH,
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
			},
		}
		dc.DrawTriangles(vs, []uint16{0, 1, 2, 1, 2, 3}, img.Ebiten(), options)
	}
}

func (p *Game) onDraw(dc drawContext) {
	dc.Fill(color.White)
	p.drawBackground(dc)
	p.getTurtle().draw(dc, p.fs)

	items := p.getItems()
	for _, item := range items {
		item.draw(dc)
	}
}

func (p *Game) onHit(hc hitContext) (hr hitResult, ok bool) {
	items := p.getItems()
	i := len(items)
	for i > 0 {
		i--
		if hr, ok = items[i].hit(hc); ok {
			return
		}
	}
	return hitResult{Target: p}, true
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
	return isKeyPressed(key)
}

func (p *Game) MouseX() float64 {
	return float64(atomic.LoadInt64(&p.gMouseX))
}

func (p *Game) MouseY() float64 {
	return float64(atomic.LoadInt64(&p.gMouseY))
}

func (p *Game) MousePressed() bool {
	return p.input.isMousePressed()
}

func (p *Game) getMousePos() (x, y float64) {
	return p.MouseX(), p.MouseY()
}

func (p *Game) updateMousePos() {
	x, y := p.input.mouseXY()
	pos := p.Camera.screenToWorld(math32.NewVector2(float64(x), float64(y)))

	worldW, worldH := p.worldSize_()
	mx, my := int(pos.X)-(worldW>>1), (worldH>>1)-int(pos.Y)
	atomic.StoreInt64(&p.gMouseX, int64(mx))
	atomic.StoreInt64(&p.gMouseY, int64(my))
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

	err := p.sounds.playAction(media, action)
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
	return p.sounds.volume()
}

func (p *Game) SetVolume(volume float64) {
	p.sounds.SetVolume(volume)
}

func (p *Game) ChangeVolume(delta float64) {
	p.sounds.ChangeVolume(delta)
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
		if sp, ok := item.(*stageMonitor); ok && sp.val == val && sp.target == target {
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

// -----------------------------------------------------------------------------
