package spx

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"reflect"
	"sync"
	"time"

	"github.com/goplus/spx/internal/anim"
	"github.com/goplus/spx/internal/gdi"
	"github.com/goplus/spx/internal/gdi/clrutil"
	"github.com/goplus/spx/internal/tools"
)

type specialDir = int
type specialObj int

const (
	Right specialDir = 90
	Left  specialDir = -90
	Up    specialDir = 0
	Down  specialDir = 180
)

const (
	Mouse      specialObj = -5
	Edge       specialObj = touchingAllEdges
	EdgeLeft   specialObj = touchingScreenLeft
	EdgeTop    specialObj = touchingScreenTop
	EdgeRight  specialObj = touchingScreenRight
	EdgeBottom specialObj = touchingScreenBottom
)

type Sprite struct {
	baseObj
	eventSinks
	g *Game

	name string

	x, y          float64
	scale         float64
	direction     float64
	rotationStyle RotationStyle

	sayObj     *sayOrThinker
	animations map[string]*aniConfig

	penColor color.RGBA
	penShade float64
	penHue   float64
	penWidth float64

	isVisible bool
	isCloned  bool
	isPenDown bool
	//isDraggable bool
	hasOnCloned bool

	isStopped bool
	isDying   bool

	hasOnTurning bool
	hasOnMoving  bool

	gamer reflect.Value
}

func (p *Sprite) SetDying() { // dying: visible but can't be touched
	p.isDying = true
}

func (p *Sprite) Stopped() bool {
	return p.isStopped
}

func (p *Sprite) Parent() *Game {
	return p.g
}

func (p *Sprite) init(
	base string, g *Game, name string, sprite *spriteConfig, gamer reflect.Value, shared *sharedImages) {
	if sprite.Costumes != nil {
		p.baseObj.init(base, sprite.Costumes, sprite.CurrentCostumeIndex)
	} else {
		p.baseObj.initWith(base, sprite, shared)
	}
	p.eventSinks.init(&g.sinkMgr, p)

	p.gamer = gamer
	p.g, p.name = g, name
	p.x, p.y = sprite.X, sprite.Y
	p.scale = sprite.Size
	p.direction = sprite.Heading
	p.rotationStyle = toRotationStyle(sprite.RotationStyle)

	p.isVisible = sprite.Visible

	p.animations = make(map[string]*aniConfig)

	for key, val := range sprite.FAnimations {
		var ani = val
		ani.AniType = aniTypeFrame

		p.animations[key] = ani
	}

	for key, val := range sprite.MAnimations {
		_, ok := p.animations[key]
		if ok {
			log.Panicf("animation key [%s] is exist", key)
		}
		var ani = val
		ani.AniType = aniTypeMove
		if ani.Fps == 0 {
			ani.Fps = 25
		}
		p.animations[key] = ani
	}

	for key, val := range sprite.TAnimations {
		_, ok := p.animations[key]
		if ok {
			log.Panicf("animation key [%s] is exist", key)
		}
		var ani = val
		ani.AniType = aniTypeTurn
		if ani.Fps == 0 {
			ani.Fps = 25
		}
		p.animations[key] = ani
	}
}

func (p *Sprite) InitFrom(src *Sprite) {
	p.baseObj.initFrom(&src.baseObj)
	p.eventSinks.initFrom(&src.eventSinks, p)

	p.g, p.name = src.g, src.name
	p.x, p.y = src.x, src.y
	p.scale = src.scale
	p.direction = src.direction
	p.rotationStyle = src.rotationStyle
	p.sayObj = nil
	p.animations = src.animations

	p.penColor = src.penColor
	p.penShade = src.penShade
	p.penHue = src.penHue
	p.penWidth = src.penWidth

	p.isVisible = src.isVisible
	p.isCloned = true
	p.isPenDown = src.isPenDown
	//p.isDraggable = src.isDraggable

	p.isStopped = false
	p.isDying = false

	p.hasOnTurning = false
	p.hasOnMoving = false
	p.hasOnCloned = false
}

func applyFloat64(out *float64, in interface{}) {
	if in != nil {
		*out = in.(float64)
	}
}

func applySpriteProps(dest *Sprite, v specsp) {
	applyFloat64(&dest.x, v["x"])
	applyFloat64(&dest.y, v["y"])
	applyFloat64(&dest.scale, v["size"])
	applyFloat64(&dest.direction, v["heading"])
	if visible, ok := v["visible"]; ok {
		dest.isVisible = visible.(bool)
	}
	if style, ok := v["rotationStyle"]; ok {
		dest.rotationStyle = toRotationStyle(style.(string))
	}
	if idx, ok := v["currentCostumeIndex"]; ok {
		dest.currentCostumeIndex = int(idx.(float64))
	}
	dest.isCloned = false
}

func applySprite(out reflect.Value, sprite Spriter, v specsp) (*Sprite, interface{}) {
	src := spriteOf(sprite)
	in := reflect.ValueOf(sprite).Elem()
	outPtr := out.Addr().Interface()
	return cloneSprite(out, outPtr, in, src, v), outPtr
}

func cloneSprite(out reflect.Value, outPtr interface{}, in reflect.Value, src *Sprite, v specsp) *Sprite {
	dest := spriteOf(outPtr.(Shape))
	func() {
		out.Set(in)
		for i, n := 0, out.NumField(); i < n; i++ {
			fld := out.Field(i).Addr()
			if ini := fld.MethodByName("InitFrom"); ini.IsValid() {
				args := []reflect.Value{in.Field(i).Addr()}
				ini.Call(args)
			}
		}
	}()
	if v != nil {
		applySpriteProps(dest, v)
	} else if ini, ok := outPtr.(initer); ok {
		ini.Main()
	}
	return dest
}

func Gopt_Sprite_Clone__0(sprite Spriter) {
	Gopt_Sprite_Clone__1(sprite, nil)
}

func Gopt_Sprite_Clone__1(sprite Spriter, data interface{}) {
	src := spriteOf(sprite)
	if debugInstr {
		log.Println("Clone", src.name)
	}
	in := reflect.ValueOf(sprite).Elem()
	v := reflect.New(in.Type())
	out, outPtr := v.Elem(), v.Interface()
	dest := cloneSprite(out, outPtr, in, src, nil)
	src.g.addClonedShape(src, dest)
	if dest.hasOnCloned {
		dest.doWhenCloned(dest, data)
	}
}

func (p *Sprite) OnCloned__0(onCloned func(data interface{})) {
	p.hasOnCloned = true
	p.allWhenCloned = &eventSink{
		prev:  p.allWhenCloned,
		pthis: p,
		sink:  onCloned,
		cond: func(data interface{}) bool {
			return data == p
		},
	}
}

func (p *Sprite) OnCloned__1(onCloned func()) {
	p.OnCloned__0(func(interface{}) {
		onCloned()
	})
}

type MovingInfo struct {
	OldX, OldY float64
	NewX, NewY float64
	ani        *anim.Anim
	Obj        *Sprite
}

func (p *MovingInfo) StopMoving() {
	if p.ani != nil {
		p.ani.Stop()
	}
}

func (p *MovingInfo) Dx() float64 {
	return p.NewX - p.OldX
}

func (p *MovingInfo) Dy() float64 {
	return p.NewY - p.OldY
}

func (p *Sprite) OnMoving__0(onMoving func(mi *MovingInfo)) {
	p.hasOnMoving = true
	p.allWhenMoving = &eventSink{
		prev:  p.allWhenMoving,
		pthis: p,
		sink:  onMoving,
		cond: func(data interface{}) bool {
			return data == p
		},
	}
}

func (p *Sprite) OnMoving__1(onMoving func()) {
	p.OnMoving__0(func(mi *MovingInfo) {
		onMoving()
	})
}

type TurningInfo struct {
	OldDir float64
	NewDir float64
	Obj    *Sprite
}

func (p *TurningInfo) Dir() float64 {
	return p.NewDir - p.OldDir
}

func (p *Sprite) OnTurning__0(onTurning func(ti *TurningInfo)) {
	p.hasOnTurning = true
	p.allWhenTurning = &eventSink{
		prev:  p.allWhenTurning,
		pthis: p,
		sink:  onTurning,
		cond: func(data interface{}) bool {
			return data == p
		},
	}
}

func (p *Sprite) OnTurning__1(onTurning func()) {
	p.OnTurning__0(func(*TurningInfo) {
		onTurning()
	})
}

func (p *Sprite) Die() { // prototype sprite can't be destoryed, but can die
	const aniName = "die"
	p.SetDying()
	if ani, ok := p.animations[aniName]; ok {
		p.goAnimate(aniName, ani)
	}
	if p.isCloned {
		p.doDestroy()
	} else {
		p.Hide()
	}
}

func (p *Sprite) Destroy() { // delete this clone
	if p.isCloned {
		p.doDestroy()
	}
}

func (p *Sprite) doDestroy() {
	if debugInstr {
		log.Println("Destroy", p.name)
	}
	p.doStopSay()
	p.doDeleteClone()
	p.g.removeShape(p)
	p.isStopped = true
	if p == gco.Current().Obj {
		gco.Abort()
	}
}

func (p *Sprite) Hide() {
	if debugInstr {
		log.Println("Hide", p.name)
	}
	p.doStopSay()
	p.isVisible = false
}

func (p *Sprite) Show() {
	if debugInstr {
		log.Println("Show", p.name)
	}
	p.isVisible = true
}

func (p *Sprite) Visible() bool {
	return p.isVisible
}

func (p *Sprite) Cloned() bool {
	return p.isCloned
}

// -----------------------------------------------------------------------------

func (p *Sprite) CostumeName() string {
	return p.getCostumeName()
}

func (p *Sprite) CostumeIndex() int {
	return p.getCostumeIndex()
}

func (p *Sprite) SetCostume(costume interface{}) {
	if debugInstr {
		log.Println("SetCostume", p.name, costume)
	}
	p.goSetCostume(costume)
}

func (p *Sprite) NextCostume() {
	if debugInstr {
		log.Println("NextCostume", p.name)
	}
	p.goNextCostume()
}

func (p *Sprite) PrevCostume() {
	if debugInstr {
		log.Println("PrevCostume", p.name)
	}
	p.goPrevCostume()
}

// -----------------------------------------------------------------------------

func (p *Sprite) getFromAnToForAni(anitype aniTypeEnum, from interface{}, to interface{}) (float64, float64) {
	fromval := 0.0
	toval := 0.0
	if anitype == aniTypeFrame {
		switch v := from.(type) {
		case string:
			fromval = float64(p.findCostume(v))
			if fromval < 0 {
				log.Panicf("findCostume %s failed", v)
			}
		default:
			fromval, _ = tools.GetFloat(from)
		}

		switch v := to.(type) {
		case string:
			toval = float64(p.findCostume(v))
			if toval < 0 {
				log.Panicf("findCostume %s failed", v)
			}
		default:
			toval, _ = tools.GetFloat(to)
		}
	} else {
		fromval, _ = tools.GetFloat(from)
		toval, _ = tools.GetFloat(to)
	}
	return fromval, toval
}

func (p *Sprite) goAnimate(name string, ani *aniConfig) {
	var animwg sync.WaitGroup
	animwg.Add(1)

	if ani.OnStart != nil {
		if ani.OnStart.Play != "" {
			media, playSound := lookupSound(p.gamer, ani.OnStart.Play)
			if !playSound {
				panic("lookupSound: media not found")
			}
			p.g.Play__0(media)
		}
	}

	//anim frame
	fromval, toval := p.getFromAnToForAni(ani.AniType, ani.From, ani.To)
	animtype := anim.AnimValTypeFloat
	if ani.AniType == aniTypeFrame {
		animtype = anim.AnimValTypeInt
		p.goSetCostume(ani.From)
		if ani.Fps == 0 { //compute fps
			ani.Fps = math.Abs(toval-fromval) / ani.Duration
		}
	}

	framenum := int(ani.Duration * ani.Fps)
	fps := ani.Fps

	//frame
	//pre_index := p.getCostumeIndex()
	//xy pos
	pre_x := p.x
	pre_y := p.y
	pre_direction := p.direction //turn p.direction

	an := anim.NewAnim(name, animtype, fps, framenum).AddKeyFrame(0, fromval).AddKeyFrame(framenum, toval).SetLoop(false)
	if debugInstr {
		log.Printf("New anim [name %s id %d] from:%v to:%v framenum:%d fps:%f", an.Name, an.Id, fromval, toval, framenum, fps)
	}
	an.SetOnPlayingListener(func(currframe int, currval interface{}) {
		if debugInstr {
			log.Printf("playing anim [name %s id %d]  currframe %d, val %v", an.Name, an.Id, currframe, currval)
		}
		val, _ := tools.GetFloat(currval)
		switch ani.AniType {
		case aniTypeFrame:
			p.setCostumeByIndex(int(val))
		case aniTypeMove:
			sin, cos := math.Sincos(toRadian(pre_direction))
			p.doMoveToForAnim(pre_x+val*sin, pre_y+val*cos, an)
		case aniTypeTurn:
			p.setDirection(val, false)
		}

		playaction := ani.OnPlay
		if playaction != nil {
			if ani.AniType != aniTypeFrame && playaction.Costumes != nil {
				costumes := playaction.Costumes
				costumesFrom, costumesTo := p.getFromAnToForAni(aniTypeFrame, costumes.From, costumes.To)

				costumeval := ((int)(costumesTo-costumesFrom) + currframe) % (int)(costumesTo)
				p.setCostumeByIndex(costumeval)
			}
		}
	})
	an.SetOnStopingListener(func() {
		if debugInstr {
			log.Printf("stop anim [name %s id %d]  ", an.Name, an.Id)
		}
		animwg.Done()
	})

	var h *tickHandler
	h = p.g.startTick(-1, func(tick int64) {
		runing := an.Update(1000.0 / p.g.currentTPS() * float64(tick))
		if !runing {
			h.Stop()
		}
	})
	waitToDo(animwg.Wait)
}

func (p *Sprite) Animate(name string) {
	if debugInstr {
		log.Println("==> Animation", name)
	}
	if ani, ok := p.animations[name]; ok {
		p.goAnimate(name, ani)
	} else {
		log.Println("Animation not found:", name)
	}
}

// -----------------------------------------------------------------------------

func (p *Sprite) Say(msg interface{}, secs ...float64) {
	if debugInstr {
		log.Println("Say", p.name, msg, secs)
	}
	p.sayOrThink(msg, styleSay)
	if secs != nil {
		p.waitStopSay(secs[0])
	}
}

func (p *Sprite) Think(msg interface{}, secs ...float64) {
	if debugInstr {
		log.Println("Think", p.name, msg, secs)
	}
	p.sayOrThink(msg, styleThink)
	if secs != nil {
		p.waitStopSay(secs[0])
	}
}

func (p *Sprite) sayOrThink(msgv interface{}, style int) {
	msg, ok := msgv.(string)
	if !ok {
		msg = fmt.Sprint(msgv)
	}

	if msg == "" {
		p.doStopSay()
		return
	}

	old := p.sayObj
	if old == nil {
		p.sayObj = &sayOrThinker{sp: p, msg: msg, style: style}
		p.g.addShape(p.sayObj)
	} else {
		old.msg, old.style = msg, style
		p.g.activateShape(old)
	}
}

func (p *Sprite) waitStopSay(secs float64) {
	p.g.Wait(secs)

	p.doStopSay()
}

func (p *Sprite) doStopSay() {
	if p.sayObj != nil {
		p.g.removeShape(p.sayObj)
		p.sayObj = nil
	}
}

// -----------------------------------------------------------------------------

func (p *Sprite) getXY() (x, y float64) {
	return p.x, p.y
}

// DistanceTo func:
//   DistanceTo(sprite)
//   DistanceTo(spriteName)
//   DistanceTo(spx.Mouse)
//   DistanceTo(spx.Random)
func (p *Sprite) DistanceTo(obj interface{}) float64 {
	x, y := p.x, p.y
	x2, y2 := p.g.objectPos(obj)
	x -= x2
	y -= y2
	return math.Sqrt(x*x + y*y)
}

func (p *Sprite) doMoveTo(x, y float64) {
	p.doMoveToForAnim(x, y, nil)
}

func (p *Sprite) doMoveToForAnim(x, y float64, ani *anim.Anim) {
	if p.hasOnMoving {
		mi := &MovingInfo{OldX: p.x, OldY: p.y, NewX: x, NewY: y, Obj: p, ani: ani}
		p.doWhenMoving(p, mi)
	}
	if p.isPenDown {
		p.g.movePen(p, x, y)
	}
	p.x, p.y = x, y
}

func (p *Sprite) goMoveForward(step float64) {
	sin, cos := math.Sincos(toRadian(p.direction))
	p.doMoveTo(p.x+step*sin, p.y+step*cos)
}

func (p *Sprite) Move__0(step float64) {
	if debugInstr {
		log.Println("Move", p.name, step)
	}
	p.goMoveForward(step)
}

func (p *Sprite) Move__1(step int) {
	p.Move__0(float64(step))
}

func (p *Sprite) Step__0(step float64) {
	if debugInstr {
		log.Println("Step", p.name, step)
	}
	if ani, ok := p.animations["step"]; ok {
		anicopy := *ani
		anicopy.From = 0
		anicopy.To = step * anicopy.Unit
		anicopy.Duration = math.Abs(step) * ani.Duration
		p.goAnimate("step", &anicopy)
		return
	}
	p.goMoveForward(step)
}

func (p *Sprite) Step__1(step int) {
	p.Step__0(float64(step))
}

// Goto func:
//   Goto(sprite)
//   Goto(spriteName)
//   Goto(spx.Mouse)
//   Goto(spx.Random)
func (p *Sprite) Goto(obj interface{}) {
	if debugInstr {
		log.Println("Goto", p.name, obj)
	}
	x, y := p.g.objectPos(obj)
	p.SetXYpos(x, y)
}

const (
	glideTick = 1e8
)

func (p *Sprite) Glide(x, y float64, secs float64) {
	if debugInstr {
		log.Println("Glide", p.name, x, y, secs)
	}
	inDur := time.Duration(secs * 1e9)
	n := int(inDur / glideTick)
	if n > 0 {
		x0, y0 := p.getXY()
		dx := (x - x0) / float64(n)
		dy := (y - y0) / float64(n)
		for i := 1; i < n; i++ {
			sleep(glideTick)
			inDur -= glideTick
			x0 += dx
			y0 += dy
			p.SetXYpos(x0, y0)
		}
	}
	sleep(inDur)
	p.SetXYpos(x, y)
}

func (p *Sprite) SetXYpos(x, y float64) {
	p.doMoveTo(x, y)
}

func (p *Sprite) ChangeXYpos(dx, dy float64) {
	p.doMoveTo(p.x+dx, p.y+dy)
}

func (p *Sprite) Xpos() float64 {
	return p.x
}

func (p *Sprite) SetXpos(x float64) {
	p.doMoveTo(x, p.y)
}

func (p *Sprite) ChangeXpos(dx float64) {
	p.doMoveTo(p.x+dx, p.y)
}

func (p *Sprite) Ypos() float64 {
	return p.y
}

func (p *Sprite) SetYpos(y float64) {
	p.doMoveTo(p.x, y)
}

func (p *Sprite) ChangeYpos(dy float64) {
	p.doMoveTo(p.x, p.y+dy)
}

// -----------------------------------------------------------------------------

type RotationStyle int

const (
	None = iota
	Normal
	LeftRight
)

func toRotationStyle(style string) RotationStyle {
	switch style {
	case "left-right":
		return LeftRight
	case "none":
		return None
	}
	return Normal
}

func (p *Sprite) SetRotationStyle(style RotationStyle) {
	if debugInstr {
		log.Println("SetRotationStyle", p.name, style)
	}
	p.rotationStyle = style
}

func (p *Sprite) Heading() float64 {
	return p.direction
}

// Turn func:
//   Turn(degree)
//   Turn(spx.Left)
//   Turn(spx.Right)
//   Turn(ti *spx.TurningInfo)
func (p *Sprite) Turn(val interface{}) {
	var delta float64
	switch v := val.(type) {
	//case specialDir:
	//	delta = float64(v)
	case int:
		delta = float64(v)
	case float64:
		delta = v
	case *TurningInfo:
		p.doTurnTogether(v) // don't animate
		return
	default:
		panic("Turn: unexpected input")
	}

	if ani, ok := p.animations["turn"]; ok {
		anicopy := *ani
		anicopy.From = p.direction
		anicopy.To = p.direction + delta
		anicopy.Duration = ani.Duration / 360.0 * math.Abs(delta)
		p.goAnimate("turn", &anicopy)
		return
	}
	if p.setDirection(delta, true) && debugInstr {
		log.Println("Turn", p.name, val)
	}
}

// TurnTo func:
//   TurnTo(sprite)
//   TurnTo(spriteName)
//   TurnTo(spx.Mouse)
//   TurnTo(degree)
//   TurnTo(spx.Left)
//   TurnTo(spx.Right)
//   TurnTo(spx.Up)
//   TurnTo(spx.Down)
func (p *Sprite) TurnTo(obj interface{}) {
	var angle float64
	switch v := obj.(type) {
	//case specialDir:
	//	angle = float64(v)
	case int:
		angle = float64(v)
	case float64:
		angle = v
	default:
		x, y := p.g.objectPos(obj)
		dx := x - p.x
		dy := y - p.y
		angle = 90 - math.Atan2(dy, dx)*180/math.Pi
	}

	if ani, ok := p.animations["turn"]; ok {
		delta := p.direction - angle
		anicopy := *ani
		anicopy.From = p.direction
		anicopy.To = angle
		anicopy.Duration = ani.Duration / 360.0 * math.Abs(delta)
		p.goAnimate("turn", &anicopy)
		return
	}
	if p.setDirection(angle, false) && debugInstr {
		log.Println("TurnTo", p.name, obj)
	}
}

func (p *Sprite) SetHeading(dir float64) {
	p.setDirection(dir, false)
}

func (p *Sprite) ChangeHeading(dir float64) {
	p.setDirection(dir, true)
}

func (p *Sprite) setDirection(dir float64, change bool) bool {
	if change {
		dir += p.direction
	}
	dir = normalizeDirection(dir)
	if p.direction == dir {
		return false
	}
	if p.hasOnTurning {
		p.doWhenTurning(p, &TurningInfo{NewDir: dir, OldDir: p.direction, Obj: p})
	}
	p.direction = dir
	return true
}

func (p *Sprite) doTurnTogether(ti *TurningInfo) {
	/*
		x’ = x0 + cos * (x-x0) + sin * (y-y0)
		y’ = y0 - sin * (x-x0) + cos * (y-y0)
	*/
	x0, y0 := ti.Obj.x, ti.Obj.y
	dir := ti.Dir()
	sin, cos := math.Sincos(dir * (math.Pi / 180))
	p.x, p.y = x0+cos*(p.x-x0)+sin*(p.y-y0), y0-sin*(p.x-x0)+cos*(p.y-y0)
	p.direction = normalizeDirection(p.direction + dir)
}

// -----------------------------------------------------------------------------

func (p *Sprite) Size() float64 {
	v := p.scale
	return v
}

func (p *Sprite) SetSize(size float64) {
	if debugInstr {
		log.Println("SetSize", p.name, size)
	}
	p.scale = size
}

func (p *Sprite) ChangeSize(delta float64) {
	if debugInstr {
		log.Println("ChangeSize", p.name, delta)
	}
	p.scale += delta
}

// -----------------------------------------------------------------------------

type Color = color.RGBA

func (p *Sprite) TouchingColor(color Color) bool {
	panic("todo")
}

// Touching func:
//   Touching(spriteName[, animation])
//   Touching(sprite)
//   Touching(spx.Mouse)
//   Touching(spx.Edge)
//   Touching(spx.EdgeLeft)
//   Touching(spx.EdgeTop)
//   Touching(spx.EdgeRight)
//   Touching(spx.EdgeBottom)
func (p *Sprite) Touching(obj interface{}, ani ...string) bool {
	if !p.isVisible || p.isDying {
		return false
	}
	switch v := obj.(type) {
	case string:
		if o := p.g.touchingSpriteBy(p, v); o != nil {
			if ani != nil {
				o.execTouchingAni(ani[0])
			}
			return true
		}
		return false
	case specialObj:
		if v > 0 {
			return p.checkTouchingScreen(int(v)) != 0
		} else if v == Mouse {
			x, y := p.g.getMousePos()
			return p.g.touchingPoint(p, x, y)
		}
	case Spriter:
		return touchingSprite(p, spriteOf(v))
	}
	panic("Touching: unexpected input")
}

func (p *Sprite) execTouchingAni(ani string) {
	if ani == "die" {
		p.Die()
	} else {
		p.Animate(ani)
	}
}

func touchingSprite(dst, src *Sprite) bool {
	if !src.isVisible || src.isDying {
		return false
	}
	sp1, pt1 := dst.getGdiSprite()
	sp2, pt2 := src.getGdiSprite()
	return gdi.Touching(sp1, pt1, sp2, pt2)
}

const (
	touchingScreenLeft   = 1
	touchingScreenTop    = 2
	touchingScreenRight  = 4
	touchingScreenBottom = 8
	touchingAllEdges     = 15
)

func (p *Sprite) BounceOffEdge() {
	if debugInstr {
		log.Println("BounceOffEdge", p.name)
	}
	dir := p.Heading()
	where := checkTouchingDirection(dir)
	touching := p.checkTouchingScreen(where)
	if touching == 0 {
		return
	}
	if (touching & (touchingScreenLeft | touchingScreenRight)) != 0 {
		dir = -dir
	} else {
		dir = 180 - dir
	}

	p.direction = normalizeDirection(dir)
}

func checkTouchingDirection(dir float64) int {
	if dir > 0 {
		if dir < 90 {
			return touchingScreenRight | touchingScreenTop
		}
		if dir > 90 {
			if dir == 180 {
				return touchingScreenBottom
			}
			return touchingScreenRight | touchingScreenBottom
		}
		return touchingScreenRight
	}
	if dir < 0 {
		if dir > -90 {
			return touchingScreenLeft | touchingScreenTop
		}
		if dir < -90 {
			return touchingScreenLeft | touchingScreenBottom
		}
		return touchingScreenLeft
	}
	return touchingScreenTop
}

func (p *Sprite) checkTouchingScreen(where int) (touching int) {
	spr, pt := p.getGdiSprite()
	if spr == nil {
		return
	}

	if (where & touchingScreenLeft) != 0 {
		if gdi.TouchingRect(spr, pt, -1e8, -1e8, 0, 1e8) {
			return touchingScreenLeft
		}
	}
	if (where & touchingScreenTop) != 0 {
		if gdi.TouchingRect(spr, pt, -1e8, -1e8, 1e8, 0) {
			return touchingScreenTop
		}
	}
	w, h := p.g.size_()
	if (where & touchingScreenRight) != 0 {
		if gdi.TouchingRect(spr, pt, w, -1e8, 1e8, 1e8) {
			return touchingScreenRight
		}
	}
	if (where & touchingScreenBottom) != 0 {
		if gdi.TouchingRect(spr, pt, -1e8, h, 1e8, 1e8) {
			return touchingScreenBottom
		}
	}
	return
}

// -----------------------------------------------------------------------------

func (p *Sprite) GoBackLayers(n int) {
	p.g.goBackByLayers(p, n)
}

func (p *Sprite) GotoFront() {
	p.g.goBackByLayers(p, -1e8)
}

// -----------------------------------------------------------------------------

func (p *Sprite) Stamp() {
	p.g.stampCostume(p.getDrawInfo())
}

func (p *Sprite) PenUp() {
	p.isPenDown = false
}

func (p *Sprite) PenDown() {
	p.isPenDown = true
}

func (p *Sprite) SetPenColor(color Color) {
	h, _, v := clrutil.RGB2HSV(color.R, color.G, color.B)
	p.penHue = (200 * h) / 360
	p.penShade = 50 * v
	p.penColor = color
}

func (p *Sprite) ChangePenColor(delta float64) {
	panic("todo")
}

func (p *Sprite) SetPenShade(shade float64) {
	p.setPenShade(shade, false)
}

func (p *Sprite) ChangePenShade(delta float64) {
	p.setPenShade(delta, true)
}

func (p *Sprite) SetPenHue(hue float64) {
	p.setPenHue(hue, false)
}

func (p *Sprite) ChangePenHue(delta float64) {
	p.setPenHue(delta, true)
}

func (p *Sprite) setPenHue(v float64, change bool) {
	if change {
		v += p.penHue
	}
	v = math.Mod(v, 200)
	if v < 0 {
		v += 200
	}
	p.penHue = v
	p.doUpdatePenColor()
}

func (p *Sprite) setPenShade(v float64, change bool) {
	if change {
		v += p.penShade
	}
	v = math.Mod(v, 200)
	if v < 0 {
		v += 200
	}
	p.penShade = v
	p.doUpdatePenColor()
}

func (p *Sprite) doUpdatePenColor() {
	r, g, b := clrutil.HSV2RGB((p.penHue*180)/100, 1, 1)
	shade := p.penShade
	if shade > 100 { // range 0..100
		shade = 200 - shade
	}
	if shade < 50 {
		r, g, b = clrutil.MixRGB(0, 0, 0, r, g, b, (10+shade)/60)
	} else {
		r, g, b = clrutil.MixRGB(r, g, b, 255, 255, 255, (shade-50)/60)
	}
	p.penColor = color.RGBA{R: r, G: g, B: b, A: p.penColor.A}
}

func (p *Sprite) SetPenSize(size float64) {
	p.setPenWidth(size, true)
}

func (p *Sprite) ChangePenSize(delta float64) {
	p.setPenWidth(delta, true)
}

func (p *Sprite) setPenWidth(w float64, change bool) {
	if change {
		w += p.penWidth
	}
	p.penWidth = w
}

// -----------------------------------------------------------------------------

func (p *Sprite) HideVar(name string) {
	p.g.setStageMonitor(p.name, getVarPrefix+name, false)
}

func (p *Sprite) ShowVar(name string) {
	p.g.setStageMonitor(p.name, getVarPrefix+name, true)
}

// -----------------------------------------------------------------------------

// Width returns sprite width
func (p *Sprite) Width() float64 {
	c := p.costumes[p.currentCostumeIndex]
	img, _, _ := c.needImage(p.g.fs)
	w, _ := img.Size()
	return float64(w / c.bitmapResolution)
}

// Height returns sprite height
func (p *Sprite) Height() float64 {
	c := p.costumes[p.currentCostumeIndex]
	img, _, _ := c.needImage(p.g.fs)
	_, h := img.Size()
	return float64(h / c.bitmapResolution)
}

// -----------------------------------------------------------------------------
