package spx

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"math/rand"
	"os"
	"reflect"
	"sync/atomic"
	"syscall"
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

const (
	mapModeFill = iota
	mapModeRepeat
	mapModeFillRatio
	mapModeFillCut
)

type Game struct {
	baseObj
	eventSinks
	Camera

	fs     spxfs.Dir
	shared *sharedImages

	sounds soundMgr
	turtle turtleCanvas
	shapes map[string]Spriter // sprite prototypes
	items  []Shape            // sprites on stage

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
}

type Spriter = Shape

type Gamer interface {
	initGame()
}

func (p *Game) getSharedImgs() *sharedImages {
	if p.shared == nil {
		p.shared = &sharedImages{imgs: make(map[string]gdi.Image)}
	}
	return p.shared
}

func (p *Game) reset() {
	p.sinkMgr.reset()
	p.input.reset()
	p.Stop(AllOtherScripts)
	p.items = nil
	p.isLoaded = false
	p.shapes = make(map[string]Spriter)
}

func (p *Game) initGame() {
	p.tickMgr.init()
	p.eventSinks.init(&p.sinkMgr, p)
}

// Gopt_Game_Main is required by Go+ compiler as the entry of a .gmx project.
func Gopt_Game_Main(game Gamer) {
	game.initGame()
	game.(interface{ MainEntry() }).MainEntry()
}

// Gopt_Game_Run runs the game.
// resource can be a string or fs.Dir object.
func Gopt_Game_Run(game Gamer, resource interface{}, gameConf ...*Config) {
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
	if err := g.startLoad(resource, &conf); err != nil {
		panic(err)
	}
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
		}
	}
	if err := g.endLoad(v, conf.Index); err != nil {
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

func lookupSound(gamer reflect.Value, name string) (Sound, bool) {
	if val := findFieldPtr(gamer, name, 0); val != nil {
		if m, ok := val.(*Sound); ok {
			return *m, true
		}
	}
	return nil, false
}

func getFieldPtrOrAlloc(v reflect.Value, i int) (name string, val interface{}) {
	tFld := v.Type().Field(i)
	vFld := v.Field(i)
	typ := tFld.Type
	word := unsafe.Pointer(vFld.Addr().Pointer())
	ret := makeEmptyInterface(reflect.PtrTo(typ), word)
	if vFld.Kind() == reflect.Ptr && typ.Implements(tyShape) {
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
			return makeEmptyInterface(reflect.PtrTo(tFld.Type), word)
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
				return makeEmptyInterface(typ, word)
			}
			word := unsafe.Pointer(vFld.Addr().Pointer())
			return makeEmptyInterface(reflect.PtrTo(typ), word)
		}
	}
	return nil
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

type costumeSetRect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type costumeSetItem struct {
	NamePrefix string `json:"namePrefix"`
	N          int    `json:"n"`
}

type costumeSet struct {
	Path             string           `json:"path"`
	FaceRight        float64          `json:"faceRight"` // turn face to right
	BitmapResolution int              `json:"bitmapResolution"`
	Nx               int              `json:"nx"`
	Rect             *costumeSetRect  `json:"rect"`
	Items            []costumeSetItem `json:"items"`
}

type costumeSetPart struct {
	Nx    int              `json:"nx"`
	Rect  costumeSetRect   `json:"rect"`
	Items []costumeSetItem `json:"items"`
}

type costumeMPSet struct {
	Path             string           `json:"path"`
	FaceRight        float64          `json:"faceRight"` // turn face to right
	BitmapResolution int              `json:"bitmapResolution"`
	Parts            []costumeSetPart `json:"parts"`
}

type costumeConfig struct {
	Name             string  `json:"name"`
	Path             string  `json:"path"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	FaceRight        float64 `json:"faceRight"` // turn face to right
	BitmapResolution int     `json:"bitmapResolution"`
}

type cameraConfig struct {
	On string `json:"on"`
}

type mapConfig struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Mode   string `json:"mode"`
}

func toMapMode(mode string) int {
	switch mode {
	case "repeat":
		return mapModeRepeat
	case "fillCut":
		return mapModeFillCut
	case "fillRatio":
		return mapModeFillRatio
	}
	return mapModeFill
}

//frame aniConfig
type aniTypeEnum int8

const (
	aniTypeFrame aniTypeEnum = iota
	aniTypeMove
	aniTypeTurn
	aniTypeGlide
)

type costumesConfig struct {
	From interface{} `json:"from"`
	To   interface{} `json:"to"`
}

type actionConfig struct {
	Play     string          `json:"play"`     //play sound
	Costumes *costumesConfig `json:"costumes"` //play frame
}

type aniConfig struct {
	Duration float64       `json:"duration"`
	Fps      float64       `json:"fps"`
	From     interface{}   `json:"from"`
	To       interface{}   `json:"to"`
	AniType  aniTypeEnum   `json:"anitype"`
	OnStart  *actionConfig `json:"onStart"` //start
	OnPlay   *actionConfig `json:"onPlay"`  //play
	//OnEnd *actionConfig  `json:"onEnd"`   //stop
}

type spriteConfig struct {
	Heading             float64               `json:"heading"`
	X                   float64               `json:"x"`
	Y                   float64               `json:"y"`
	Size                float64               `json:"size"`
	RotationStyle       string                `json:"rotationStyle"`
	Costumes            []*costumeConfig      `json:"costumes"`
	CostumeSet          *costumeSet           `json:"costumeSet"`
	CostumeMPSet        *costumeMPSet         `json:"costumeMPSet"`
	CurrentCostumeIndex *int                  `json:"currentCostumeIndex"`
	CostumeIndex        int                   `json:"costumeIndex"`
	FAnimations         map[string]*aniConfig `json:"fAnimations"`
	MAnimations         map[string]*aniConfig `json:"mAnimations"`
	TAnimations         map[string]*aniConfig `json:"tAnimations"`
	Visible             bool                  `json:"visible"`
	IsDraggable         bool                  `json:"isDraggable"`
}

func (p *spriteConfig) getCostumeIndex() int {
	if p.CurrentCostumeIndex != nil { // for backward compatibility
		return *p.CurrentCostumeIndex
	}
	return p.CostumeIndex
}

func (p *Game) startLoad(resource interface{}, cfg *Config) (err error) {
	if debugLoad {
		log.Println("==> StartLoad", resource)
	}
	fs, ok := resource.(spxfs.Dir)
	if !ok {
		fs, err = spxfs.Open(resource.(string))
		if err != nil {
			return err
		}
	}
	var keyDuration int
	if cfg != nil {
		keyDuration = cfg.KeyDuration
	}
	p.initGame()
	p.input.init(p, keyDuration)
	p.sounds.init(p)
	p.shapes = make(map[string]Spriter)
	p.events = make(chan event, 16)
	p.fs = fs
	p.windowWidth_ = cfg.Width
	p.windowHeight_ = cfg.Height
	return
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
	p.shapes[name] = sprite
	//
	// init gamer pointer (field 1)
	*(*uintptr)(unsafe.Pointer(vSpr.Field(1).Addr().Pointer())) = gamer.Addr().Pointer()
	return nil
}

func spriteOf(sprite Spriter) *Sprite {
	vSpr := reflect.ValueOf(sprite).Elem()
	return vSpr.Field(0).Addr().Interface().(*Sprite)
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
	Scenes              []*costumeConfig `json:"scenes"`
	Costumes            []*costumeConfig `json:"costumes"`
	CurrentCostumeIndex *int             `json:"currentCostumeIndex"`
	SceneIndex          int              `json:"sceneIndex"`

	Map    mapConfig     `json:"map"`
	Camera *cameraConfig `json:"camera"`
}

func (p *projConfig) getScenes() []*costumeConfig {
	if p.Scenes != nil {
		return p.Scenes
	}
	return p.Costumes
}

func (p *projConfig) getSceneIndex() int {
	if p.CurrentCostumeIndex != nil {
		return *p.CurrentCostumeIndex
	}
	return p.SceneIndex
}

type initer interface {
	Main()
}

func (p *Game) loadIndex(g reflect.Value, index interface{}) (err error) {
	var proj projConfig
	switch v := index.(type) {
	case io.Reader:
		err = json.NewDecoder(v).Decode(&proj)
	case string:
		err = loadJson(&proj, p.fs, v)
	case nil:
		err = loadJson(&proj, p.fs, "index.json")
	default:
		return syscall.EINVAL
	}
	if err != nil {
		return
	}

	if scenes := proj.getScenes(); len(scenes) > 0 {
		p.baseObj.init("", scenes, proj.getSceneIndex())
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

	inits := make([]initer, 0, len(proj.Zorder))
	for _, v := range proj.Zorder {
		if name, ok := v.(string); !ok { // not a prototype sprite
			inits = p.addSpecialShape(g, v.(specsp), inits)
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

	ebiten.SetWindowResizable(true)
	if proj.Camera != nil && proj.Camera.On != "" {
		p.Camera.On(proj.Camera.On)
	}
	if loader, ok := g.Addr().Interface().(interface{ OnLoaded() }); ok {
		loader.OnLoaded()
	}
	//game load success
	p.isLoaded = true
	return
}

func (p *Game) endLoad(g reflect.Value, index interface{}) (err error) {
	if debugLoad {
		log.Println("==> EndLoad")
	}
	return p.loadIndex(g, index)
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
	return g.loadIndex(v, index)
}

// -----------------------------------------------------------------------------

type specsp = map[string]interface{}

func (p *Game) addSpecialShape(g reflect.Value, v specsp, inits []initer) []initer {
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

func (p *Game) addStageSprite(g reflect.Value, v specsp, inits []initer) []initer {
	target := v["target"].(string)
	if val := findObjPtr(g, target, 0); val != nil {
		if sp, ok := val.(Shape); ok {
			dest := spriteOf(sp)
			applySpriteProps(dest, v)
			p.addShape(dest)
			if ini, ok := val.(initer); ok {
				inits = append(inits, ini)
			}
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
func (p *Game) addStageSprites(g reflect.Value, v specsp, inits []initer) []initer {
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
			if typItemPtr.Implements(tyShape) {
				name := typItem.Name()
				spr, ok := p.shapes[name]
				if !ok {
					spr = reflect.New(typItem).Interface().(Spriter)
					if err := p.loadSprite(spr, name, g); err != nil {
						panic(err)
					}
					p.shapes[name] = spr
				}
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
					if ini, ok := sp.(initer); ok {
						inits = append(inits, ini)
					}
				}
				fldSlice.Set(newSlice)
				return inits
			}
		}
	}
	panic("addStageSprites: unexpected - " + target)
}

var (
	tyShape = reflect.TypeOf((*Shape)(nil)).Elem()
)

// -----------------------------------------------------------------------------

type Config struct {
	Title              string
	Index              interface{} // where is index, can be file (string) or io.Reader
	KeyDuration        int
	FullScreen         bool
	DontRunOnUnfocused bool
	DontParseFlags     bool
	Width              int
	Height             int
	ScreenshotKey      string // screenshot image capture key
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
	title := cfg.Title
	if title == "" {
		title = "Game powered by Go+"
	}
	ebiten.SetWindowTitle(title)
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
	case int:
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

	var srcWidth, srcHeight, dstWidth, dstHeight float32
	if p.mapMode == mapModeRepeat {
		srcWidth = float32(p.worldWidth_)
		srcHeight = float32(p.worldHeight_)
		dstWidth = float32(p.worldWidth_)
		dstHeight = float32(p.worldHeight_)
		options.Address = ebiten.AddressRepeat
	} else {
		srcWidth = float32(img.Bounds().Dx())
		srcHeight = float32(img.Bounds().Dy())
		options.Address = ebiten.AddressClampToZero
		switch p.mapMode {
		default:
			dstWidth = float32(p.worldWidth_)
			dstHeight = float32(p.worldHeight_)
		case mapModeFillCut:
			if srcWidth > srcHeight {
				dstHeight = float32(p.worldHeight_)
				dstWidth = float32(p.worldWidth_) * srcWidth / srcHeight
			} else {
				dstWidth = float32(p.worldWidth_)
				dstHeight = float32(p.worldHeight_) * srcWidth / srcHeight
			}
		case mapModeFillRatio:
			if srcWidth > srcHeight {
				dstHeight = float32(p.worldHeight_)
				dstWidth = float32(p.worldWidth_) * srcHeight / srcWidth
			} else {
				dstWidth = float32(p.worldWidth_)
				dstHeight = float32(p.worldHeight_) * srcHeight / srcWidth
			}
		}
	}

	var cx, cy float32
	cx = (float32(p.worldWidth_) - dstWidth) / 2.0
	cy = (float32(p.worldHeight_) - dstHeight) / 2.0
	vs := []ebiten.Vertex{
		{
			DstX: cx, DstY: cy, SrcX: 0, SrcY: 0,
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
		},
		{
			DstX: dstWidth + cx, DstY: cy, SrcX: srcWidth, SrcY: 0,
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
		},
		{
			DstX: cx, DstY: dstHeight + cy, SrcX: 0, SrcY: srcHeight,
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
		},
		{
			DstX: dstWidth + cx, DstY: dstHeight + cy, SrcX: srcWidth, SrcY: srcHeight,
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
		},
	}
	dc.DrawTriangles(vs, []uint16{0, 1, 2, 1, 2, 3}, img.Ebiten(), options)
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
		p.windowWidth_ = 0 // TODO: need review
		p.doWhenSceneStart(p.getCostumeName(), wait != nil && wait[0])
	}
}

func (p *Game) NextScene(wait ...bool) {
	p.StartScene(Next, wait...)
}

func (p *Game) PrevScene(wait ...bool) {
	p.StartScene(Prev, wait...)
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
//   Play(media, wait) -- sync
//   Play(media, opts)
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
