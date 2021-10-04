/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package spx

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"log"
	"math/rand"
	"os"
	"reflect"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/goplus/spx/internal/coroutine"
	"github.com/goplus/spx/internal/gdi"
	"github.com/hajimehoshi/ebiten/v2"

	spxfs "github.com/goplus/spx/fs"
	_ "github.com/goplus/spx/fs/local"
	_ "github.com/goplus/spx/fs/zip"
)

const (
	GopPackage = true
	Gop_sched  = "Sched,SchedNow"
)

const (
	DbgFlagLoad = 1 << iota
	DbgFlagInstr
	DbgFlagEvent
	DbgFlagAll = DbgFlagLoad | DbgFlagInstr | DbgFlagEvent
)

var (
	debugInstr bool
	debugLoad  bool
	debugEvent bool
)

func SetDebug(flags int) {
	debugLoad = (flags & DbgFlagLoad) != 0
	debugInstr = (flags & DbgFlagInstr) != 0
	debugEvent = (flags & DbgFlagEvent) != 0
}

// -------------------------------------------------------------------------------------

type Game struct {
	baseObj
	eventSinks
	fs spxfs.Dir

	sinkMgr eventSinkMgr

	sounds soundMgr
	turtle turtleCanvas
	shapes map[string]Spriter
	items  []Shape

	input  inputMgr
	events chan event

	width  int
	height int

	gMouseX, gMouseY int64
}

type Spriter = Shape

type Gamer interface {
	runLoop(cfg *Config) error
}

func (p *Game) Initialize() {
	p.eventSinks.init(&p.sinkMgr, p)
}

func Gopt_Game_Run(game Gamer, resource string, gameConf ...*Config) {
	var conf Config
	if gameConf != nil {
		conf = *gameConf[0]
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
	vPtr := reflect.ValueOf(game)
	g := instance(vPtr)
	if err := g.startLoad(resource, &conf); err != nil {
		panic(err)
	}
	tPtr, v := vPtr.Type(), vPtr.Elem()
	for i, n := 0, v.NumField(); i < n; i++ {
		name, val := getFieldPtr(v, i)
		switch fld := val.(type) {
		case *Sound:
			media, err := g.loadSound(name)
			if err != nil {
				panic(err)
			}
			*fld = media
		case Shape:
			if err := g.loadSprite(fld, name, vPtr); err != nil {
				panic(err)
			}
			vFld := v.Field(i)
			if vFld.NumField() > 1 {
				if fldSub := vFld.Field(1); fldSub.Type() == tPtr {
					*(*uintptr)(unsafe.Pointer(fldSub.Addr().Pointer())) = vPtr.Pointer()
				}
			}
		}
	}
	if err := g.endLoad(v); err != nil {
		panic(err)
	}
	if err := g.runLoop(&conf); err != nil {
		panic(err)
	}
}

func lookupSound(gamer reflect.Value, name string) (Sound, bool) {
	v := gamer.Elem()
	for i, n := 0, v.NumField(); i < n; i++ {
		nameFld, val := getFieldPtr(v, i)
		if nameFld == name {
			if m, ok := val.(*Sound); ok {
				return *m, true
			}
			break
		}
	}
	return nil, false
}

func instance(gamer reflect.Value) *Game {
	fld := gamer.Elem().FieldByName("Game")
	if !fld.IsValid() {
		log.Panicf("type %v doesn't has field spx.Game", gamer.Type())
	}
	return fld.Addr().Interface().(*Game)
}

func getFieldPtr(v reflect.Value, i int) (name string, val interface{}) {
	tFld := v.Type().Field(i)
	word := unsafe.Pointer(v.Field(i).Addr().Pointer())
	return tFld.Name, makeEmptyInterface(reflect.PtrTo(tFld.Type), word)
}

// emptyInterface is the header for an interface{} value.
type emptyInterface struct {
	typ  unsafe.Pointer
	word unsafe.Pointer
}

func makeEmptyInterface(typ reflect.Type, word unsafe.Pointer) (i interface{}) {
	e := (*emptyInterface)(unsafe.Pointer(&i))
	etyp := (*emptyInterface)(unsafe.Pointer(&typ))
	e.typ, e.word = etyp.word, word
	return
}

type costumeSetItem struct {
	NamePrefix string `json:"namePrefix"`
	N          int    `json:"n"`
}

type costumeSet struct {
	Path             string           `json:"path"`
	Nx               int              `json:"nx"`
	BitmapResolution int              `json:"bitmapResolution"`
	Items            []costumeSetItem `json:"items"`
}

type costumeConfig struct {
	Name             string  `json:"name"`
	Path             string  `json:"path"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	BitmapResolution int     `json:"bitmapResolution"`
}

type aniConfig struct {
	Play string      `json:"play"`
	Wait float64     `json:"wait"`
	Move float64     `json:"move"`
	From interface{} `json:"from"`
	N    int         `json:"n"`
	Step int         `json:"step"`
	Die  bool        `json:"die"`
}

type spriteConfig struct {
	Heading             float64               `json:"heading"`
	X                   float64               `json:"x"`
	Y                   float64               `json:"y"`
	Size                float64               `json:"size"`
	RotationStyle       string                `json:"rotationStyle"`
	Costumes            []*costumeConfig      `json:"costumes"`
	CostumeSet          *costumeSet           `json:"costumeSet"`
	CurrentCostumeIndex int                   `json:"currentCostumeIndex"`
	Animations          map[string]*aniConfig `json:"animations"`
	Visible             bool                  `json:"visible"`
	IsDraggable         bool                  `json:"isDraggable"`
}

func (p *Game) startLoad(resource string, cfg *Config) error {
	if debugLoad {
		log.Println("==> StartLoad", resource)
	}
	fs, err := spxfs.Open(resource)
	if err != nil {
		return err
	}
	var keyDuration int
	if cfg != nil {
		keyDuration = cfg.KeyDuration
	}
	p.Initialize()
	p.input.init(p, keyDuration)
	p.sounds.init()
	p.shapes = make(map[string]Spriter)
	p.events = make(chan event, 16)
	p.fs = fs
	return nil
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
	base := spriteOf(sprite)
	base.init(baseDir, p, name, &conf, gamer)
	p.shapes[name] = sprite
	return nil
}

func spriteOf(sprite Spriter) *Sprite {
	fld := reflect.ValueOf(sprite).Elem().FieldByName("Sprite")
	if !fld.IsValid() {
		log.Panicf("type %v doesn't has field spx.Sprite", reflect.TypeOf(sprite))
	}
	return fld.Addr().Interface().(*Sprite)
}

func loadJson(ret interface{}, fs spxfs.Dir, file string) (err error) {
	f, err := fs.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(ret)
}

type projConfig struct {
	Zorder              []interface{}    `json:"zorder"`
	Costumes            []*costumeConfig `json:"costumes"`
	CurrentCostumeIndex int              `json:"currentCostumeIndex"`
}

type initer interface {
	Main()
}

func (p *Game) endLoad(g reflect.Value) (err error) {
	if debugLoad {
		log.Println("==> EndLoad")
	}
	var proj projConfig
	err = loadJson(&proj, p.fs, "index.json")
	if err != nil {
		return
	}
	p.baseObj.init("", proj.Costumes, proj.CurrentCostumeIndex)
	inits := make([]initer, 0, len(proj.Zorder))
	for _, v := range proj.Zorder {
		if name, ok := v.(string); !ok { // not a sprite
			p.addSpecialShape(g, v.(specsp))
		} else if sp, ok := p.shapes[name]; ok {
			p.addShape(spriteOf(sp))
			if ini, ok := sp.(initer); ok {
				inits = append(inits, ini)
			}
		} else {
			return fmt.Errorf("sprite %s is not found", name)
		}
	}
	for _, ini := range inits {
		ini.Main()
	}
	return
}

// -----------------------------------------------------------------------------

type specsp = map[string]interface{}

func (p *Game) addSpecialShape(g reflect.Value, v specsp) {
	switch typ := v["type"].(string); typ {
	case "stageMonitor":
		if sm, err := newStageMonitor(g, v); err == nil {
			p.addShape(sm)
		}
	default:
		panic("addSpecialShape: unknown shape - " + typ)
	}
}

// -----------------------------------------------------------------------------

type Config struct {
	Title              string
	Scale              float64
	KeyDuration        int
	FullScreen         bool
	DontRunOnUnfocused bool
	DontParseFlags     bool
}

func (p *Game) runLoop(cfg *Config) (err error) {
	if debugLoad {
		log.Println("==> RunLoop")
	}
	if cfg == nil {
		cfg = &Config{}
	}
	if !cfg.DontRunOnUnfocused {
		ebiten.SetRunnableOnUnfocused(true)
	}
	if cfg.FullScreen {
		ebiten.SetFullscreen(true)
	}
	p.initEventLoop()
	/*
		scale := 1.0
		if cfg.Scale != 0 {
			scale = cfg.Scale
		}
		w, h := p.size()
		ebiten.SetWindowSize(int(float64(w)*scale), int(float64(h)*scale))
	*/
	w, h := p.size()
	ebiten.SetWindowSize(w, h)
	title := cfg.Title
	if title == "" {
		title = "Game powered by Go+"
	}
	ebiten.SetWindowTitle(title)
	return ebiten.RunGame(p)
}

func (p *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return p.size()
}

func (p *Game) Update() error {
	p.updateMousePos()
	p.input.update()
	p.sounds.update()
	return nil
}

func (p *Game) Draw(screen *ebiten.Image) {
	dc := drawContext{Image: screen}
	p.onDraw(dc)
}

type clicker interface {
	doWhenClick()
}

func (p *Game) doWhenLeftButtonDown(ev *eventLeftButtonDown) {
	hc := hitContext{Pos: image.Pt(ev.X, ev.Y)}
	if hr, ok := p.onHit(hc); ok {
		if o, ok := hr.Target.(clicker); ok {
			o.doWhenClick()
		}
	}
}

func (p *Game) doFireEvent(event event) {
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
		p.doFireEvent(ev)
	}
}

func (p *Game) initEventLoop() {
	gco.Create(nil, p.eventLoop)
}

func init() {
	gco = coroutine.New()
}

var (
	gco            *coroutine.Coroutines
	errAbortThread = errors.New("abort thread")
)

func createThread(start bool, f func(coroutine.Thread) int) {
	var thMain coroutine.Thread
	if start {
		thMain = gco.Current()
	}
	fn := func(th coroutine.Thread) int {
		defer func() {
			if e := recover(); e != nil {
				if e != errAbortThread {
					panic(e)
				}
			}
		}()
		return f(th)
	}
	gco.CreateAndStart(nil, fn, thMain)
}

func abortThread() {
	panic(errAbortThread)
}

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

func sleep(t time.Duration) {
	me := gco.Current()
	go func() {
		time.Sleep(t)
		gco.Resume(me)
	}()
	gco.Yield(me)
}

func SchedNow() int {
	me := gco.Current()
	gco.Sched(me)
	return 0
}

func Sched() int {
	now := time.Now()
	if now.Sub(lastSched) >= 3e7 {
		me := gco.Current()
		gco.Sched(me)
		lastSched = now
	}
	return 0
}

var lastSched time.Time

// -----------------------------------------------------------------------------

func (p *Game) getWidth() int {
	if p.width == 0 {
		p.doSize()
	}
	return p.width
}

func (p *Game) size() (int, int) {
	if p.width == 0 {
		p.doSize()
	}
	return p.width, p.height
}

func (p *Game) doSize() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.width == 0 {
		c := p.costumes[p.currentCostumeIndex]
		img, _, _ := c.needImage(p.fs)
		w, h := img.Size()
		p.width, p.height = w/c.bitmapResolution, h/c.bitmapResolution
	}
}

func (p *Game) getGdiPos(x, y float64) (int, int) {
	screenW, screenH := p.size()
	return int(x) + (screenW >> 1), (screenH >> 1) - int(y)
}

func (p *Game) touchingPoint(dst *Sprite, x, y float64) bool {
	sp, pt := dst.getGdiSprite()
	sx, sy := p.getGdiPos(x, y)
	return gdi.TouchingPoint(sp, pt, sx, sy)
}

func (p *Game) touchingSpriteBy(dst *Sprite, name string) *Sprite {
	sp1, pt1 := dst.getGdiSprite()

	p.mutex.Lock()
	defer p.mutex.Unlock()
	for _, item := range p.items {
		if sp, ok := item.(*Sprite); ok && sp != dst {
			if sp.name == name {
				sp2, pt2 := sp.getGdiSprite()
				if gdi.Touching(sp1, pt1, sp2, pt2) {
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
	case *Sprite:
		return v.getXY()
	case specialObj:
		if v == Mouse {
			return p.getMousePos()
		} else if v == Random {
			screenW, screenH := p.size()
			mx, my := rand.Intn(screenW), rand.Intn(screenH)
			return float64(mx - (screenW >> 1)), float64((screenH >> 1) - my)
		}
	}
	panic("objectPos: unexpected input")
}

// -----------------------------------------------------------------------------

func (p *Game) getTurtle() turtleCanvas {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.turtle
}

func (p *Game) Clear() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.turtle.clear()
}

func (p *Game) stampCostume(di *spriteDrawInfo) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.turtle.stampCostume(di)
}

func (p *Game) movePen(sp *Sprite, x, y float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	screenW, screenH := p.size()
	p.turtle.penLine(&penLine{
		x1:    (screenW >> 1) + int(sp.x),
		y1:    (screenH >> 1) - int(sp.y),
		x2:    (screenW >> 1) + int(x),
		y2:    (screenH >> 1) - int(y),
		clr:   sp.penColor,
		width: int(sp.penWidth),
	})
}

// -----------------------------------------------------------------------------

func (p *Game) getItems() []Shape {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.items
}

func (p *Game) addShape(child Shape) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.items = append(p.items, child)
}

func (p *Game) addClonedShape(src, clone Shape) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	items := p.items
	idx := p.doFindSprite(src)
	if idx < 0 {
		log.Println("addClonedShape: clone a deleted sprite")
		abortThread()
		return
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
	p.mutex.Lock()
	defer p.mutex.Unlock()

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
	p.mutex.Lock()
	defer p.mutex.Unlock()

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
	p.mutex.Lock()
	defer p.mutex.Unlock()

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
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, item := range p.items {
		if sp, ok := item.(*Sprite); ok {
			if !sp.isCloned && sp.name == name {
				return sp
			}
		}
	}
	return nil
}

// -----------------------------------------------------------------------------

func (p *Game) drawBackground(dc drawContext) {
	c := p.costumes[p.currentCostumeIndex]
	img, _, _ := c.needImage(p.fs)

	var options *ebiten.DrawImageOptions
	if c.bitmapResolution > 1 {
		scale := 1.0 / float64(c.bitmapResolution)
		options = new(ebiten.DrawImageOptions)
		options.GeoM.Scale(scale, scale)
	}
	dc.DrawImage(img, options)
}

func (p *Game) onDraw(dc drawContext) {
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

func (p *Game) SceneName() string {
	return p.getCostumeName()
}

func (p *Game) SceneIndex() int {
	return p.getCostumeIndex()
}

// StartScene func:
//   StartScene(sceneName) or
//   StartScene(sceneIndex) or
//   StartScene(spx.Next)
//   StartScene(spx.Prev)
func (p *Game) StartScene(scene interface{}, wait ...bool) {
	if p.goSetCostume(scene) {
		p.width = 0
		p.doWhenSceneStart(p.getCostumeName(), wait != nil && wait[0])
	}
}

func (p *Game) NextScene(wait ...bool) {
	p.StartScene(Next, wait...)
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
	return isMousePressed()
}

func (p *Game) getMousePos() (x, y float64) {
	return p.MouseX(), p.MouseY()
}

func (p *Game) updateMousePos() {
	x, y := ebiten.CursorPosition()
	screenW, screenH := p.size()
	mx, my := x-(screenW>>1), (screenH>>1)-y
	atomic.StoreInt64(&p.gMouseX, int64(mx))
	atomic.StoreInt64(&p.gMouseY, int64(my))
}

func (p *Game) Username() string {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Game) Wait(secs float64) {
	sleep(time.Duration(secs * 1e9))
}

func (p *Game) Timer() float64 {
	panic("todo")
}

func (p *Game) ResetTimer() {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Game) Ask(ask string) {
	panic("todo")
}

func (p *Game) Answer() Value {
	panic("todo")
}

// -----------------------------------------------------------------------------

type EffectKind int

func (p *Game) SetEffect(kind EffectKind, val float64) {
	panic("todo")
}

func (p *Game) ChangeEffect(kind EffectKind, delta float64) {
	panic("todo")
}

func (p *Game) ClearEffects() {
	panic("todo")
}

// -----------------------------------------------------------------------------

type soundConfig struct {
	Path        string `json:"path"`
	Rate        int    `json:"rate"`
	SampleCount int    `json:"sampleCount"`
}

type Sound *soundConfig

func (p *Game) loadSound(name string) (media Sound, err error) {
	if debugLoad {
		log.Println("==> LoadSound", name)
	}
	prefix := "sounds/" + name
	media = new(soundConfig)
	if err = loadJson(media, p.fs, prefix+"/index.json"); err != nil {
		return
	}
	media.Path = prefix + "/" + media.Path
	return
}

// Play func:
//   Play(sound)
//   Play(video) -- maybe
func (p *Game) Play__0(media Sound, wait ...bool) {
	if debugInstr {
		log.Println("Play", media.Path, wait)
	}
	f, err := p.fs.Open(media.Path)
	if err != nil {
		panic(err)
	}
	err = p.sounds.play(f, wait...)
	if err != nil {
		panic(err)
	}
}

func (p *Game) StopAllSounds() {
	p.sounds.stopAll()
}

func (p *Game) Volume() float64 {
	panic("todo")
}

func (p *Game) SetVolume(volume float64) {
	panic("todo")
}

func (p *Game) ChangeVolume(delta float64) {
	panic("todo")
}

func (p *Game) Stop(what string) {
	if what == "all" {
		os.Exit(0)
	}
	panic("todo")
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
	p.mutex.Lock()
	defer p.mutex.Unlock()

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
