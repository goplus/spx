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
	"fmt"
	"image/color"
	"log"
	"math"
	"reflect"
	"time"

	"github.com/goplus/spx/internal/gdi"
	"github.com/goplus/spx/internal/gdi/clrutil"
)

const (
	Right = 90
	Left  = -90
	Up    = 0
	Down  = 180
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
	sayObj        *sayOrThinker
	anis          map[string]func(*Sprite)

	penColor color.RGBA
	penShade float64
	penHue   float64
	penWidth float64

	visible     bool
	isDraggable bool
	isCloned    bool
	isPenDown   bool
}

func (p *Sprite) Game() *Game {
	return p.g
}

func (p *Sprite) init(base string, g *Game, name string, sprite *spriteConfig) {
	if sprite.CostumeSet != nil {
		p.baseObj.initWith(base, sprite.CostumeSet, sprite.CurrentCostumeIndex)
	} else {
		p.baseObj.init(base, sprite.Costumes, sprite.CurrentCostumeIndex)
	}
	p.eventSinks.init(&g.sinkMgr, p)

	p.g, p.name = g, name
	p.x, p.y = sprite.X, sprite.Y
	p.scale = sprite.Size
	p.direction = sprite.Heading
	p.rotationStyle = toRotationStyle(sprite.RotationStyle)

	p.visible = sprite.Visible
	p.isDraggable = sprite.IsDraggable
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
	p.anis = src.anis

	p.penColor = src.penColor
	p.penShade = src.penShade
	p.penHue = src.penHue
	p.penWidth = src.penWidth

	p.visible = src.visible
	p.isDraggable = src.isDraggable
	p.isCloned = true
	p.isPenDown = src.isPenDown
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
	dest := spriteOf(outPtr.(Shape))

	func() {
		src.mutex.Lock()
		defer src.mutex.Unlock()

		out.Set(in)
		for i, n := 0, out.NumField(); i < n; i++ {
			fld := out.Field(i).Addr()
			if ini := fld.MethodByName("InitFrom"); ini.IsValid() {
				args := []reflect.Value{in.Field(i).Addr()}
				ini.Call(args)
			}
		}
	}()
	if ini, ok := outPtr.(initer); ok {
		ini.Main()
	}
	src.g.addClonedShape(src, dest)
	dest.doWhenCloned(data)
}

func (p *Sprite) Destroy() { // delete this clone
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.isCloned {
		if debugInstr {
			log.Println("Destroy", p.name)
		}
		p.doStopSay()
		p.doDeleteClone()
		p.g.removeShape(p)
		abortThread()
	}
}

// -----------------------------------------------------------------------------

func (p *Sprite) Hide() {
	if debugInstr {
		log.Println("Hide", p.name)
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.visible = false
}

func (p *Sprite) Show() {
	if debugInstr {
		log.Println("Show", p.name)
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.visible = true
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

func (p *Sprite) goAnimate(secs float64, costume interface{}, n, step int) {
	p.goSetCostume(costume)
	for i := 0; i < n; i++ {
		p.g.Wait(secs)
		p.goCostume(step)
	}
}

func (p *Sprite) Animate__0(secs float64, costume interface{}, n, step int) {
	if debugInstr {
		log.Println("Animation", secs, costume, n, step)
	}
	p.goAnimate(secs, costume, n, step)
}

func (p *Sprite) Animate__1(secs float64, costume interface{}, n int) {
	if debugInstr {
		log.Println("Animation", secs, costume, n)
	}
	p.goAnimate(secs, costume, n, 1)
}

func (p *Sprite) Animate__2(name string) {
	if debugInstr {
		log.Println("==> Animation", name)
	}
	if ani := p.getAni(name); ani != nil {
		ani(p)
	}
	if debugInstr {
		log.Println("==> End Animation", name)
	}
}

func (p *Sprite) SetAnimation(name string, ani func(*Sprite)) {
	// animations are shared.
	// don't need SetAnimation to cloned sprites.
	if p.isCloned {
		return
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.anis == nil {
		p.anis = make(map[string]func(*Sprite))
	}
	p.anis[name] = ani
}

func (p *Sprite) getAni(name string) func(*Sprite) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.anis != nil {
		return p.anis[name]
	}
	return nil
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

	p.mutex.Lock()
	defer p.mutex.Unlock()

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

	p.mutex.Lock()
	defer p.mutex.Unlock()
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
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.x, p.y
}

// DistanceTo func:
//   DistanceTo(sprite)
//   DistanceTo(spriteName)
//   DistanceTo(gox.Mouse)
//   DistanceTo(gox.Random)
func (p *Sprite) DistanceTo(obj interface{}) float64 {
	p.mutex.Lock()
	x, y := p.x, p.y
	p.mutex.Unlock()

	x2, y2 := p.g.objectPos(obj)
	x -= x2
	y -= y2
	return math.Sqrt(x*x + y*y)
}

func (p *Sprite) doMoveTo(x, y float64) {
	if p.isPenDown {
		p.g.movePen(p, x, y)
	}
	p.x, p.y = x, y
}

func (p *Sprite) goMoveForward(step float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	sin, cos := math.Sincos(toRadian(p.direction))
	p.doMoveTo(p.x+step*sin, p.y+step*cos)
}

func (p *Sprite) Move(step float64) {
	if debugInstr {
		log.Println("Move", p.name, step)
	}
	p.goMoveForward(step)
}

func (p *Sprite) Step(step float64) {
	if debugInstr {
		log.Println("Step", p.name, step)
	}
	if p.anis == nil {
		p.goMoveForward(step)
		return
	}
	var backward = step < 0
	var name string
	if backward {
		name = "backward"
	} else {
		name = "forward"
	}
	ani := p.getAni(name)
	if ani == nil {
		p.goMoveForward(step)
		return
	}
	var n int
	if backward {
		n = int(-step + 0.5)
	} else {
		n = int(step + 0.5)
	}
	for ; n > 0; n-- {
		ani(p)
	}
}

// Goto func:
//   Goto(sprite)
//   Goto(spriteName)
//   Goto(gox.Mouse)
//   Goto(gox.Random)
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
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.doMoveTo(x, y)
}

func (p *Sprite) Xpos() float64 {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.x
}

func (p *Sprite) SetXpos(x float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.doMoveTo(x, p.y)
}

func (p *Sprite) ChangeXpos(dx float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.doMoveTo(p.x+dx, p.y)
}

func (p *Sprite) Ypos() float64 {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.y
}

func (p *Sprite) SetYpos(y float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.doMoveTo(p.x, y)
}

func (p *Sprite) ChangeYpos(dy float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

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
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.rotationStyle = style
}

func (p *Sprite) Heading() float64 {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.direction
}

// Turn func:
//   Turn(degree)
//   Turn(gox.Left)
//   Turn(gox.Right)
func (p *Sprite) Turn(delta float64) {
	if debugInstr {
		log.Println("Turn", p.name, delta)
	}
	p.setDirection(delta, true)
}

// TurnTo func:
//   TurnTo(sprite)
//   TurnTo(spriteName)
//   TurnTo(gox.Mouse)
//   TurnTo(gox.Random)
//   TurnTo(degree)
//   TurnTo(gox.Left)
//   TurnTo(gox.Right)
//   TurnTo(gox.Up)
//   TurnTo(gox.Down)
func (p *Sprite) TurnTo(obj interface{}) {
	if debugInstr {
		log.Println("TurnTo", p.name, obj)
	}
	switch v := obj.(type) {
	case int:
		p.setDirection(float64(v), false)
	case float64:
		p.setDirection(v, false)
	default:
		x, y := p.g.objectPos(obj)
		dx := x - p.x
		dy := y - p.y
		angle := 90 - math.Atan2(dy, dx)*180/math.Pi
		p.setDirection(angle, false)
	}
}

func (p *Sprite) setDirection(dir float64, change bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if change {
		dir += p.direction
	}
	p.direction = normalizeDirection(dir)
}

// -----------------------------------------------------------------------------

func (p *Sprite) Size() float64 {
	p.mutex.Lock()
	v := p.scale
	p.mutex.Unlock()

	return v
}

func (p *Sprite) SetSize(size float64) {
	if debugInstr {
		log.Println("SetSize", p.name, size)
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.scale = size
}

func (p *Sprite) ChangeSize(delta float64) {
	if debugInstr {
		log.Println("ChangeSize", p.name, delta)
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.scale += delta
}

// -----------------------------------------------------------------------------

type Color = color.RGBA

func (p *Sprite) TouchingColor(color Color) bool {
	panic("todo")
}

// Touching func:
//   Touching(spriteName)
//   Touching(sprite)
//   Touching(spx.Mouse)
//   Touching(spx.Edge)
func (p *Sprite) Touching(obj interface{}) bool {
	switch v := obj.(type) {
	case string:
		return p.g.touchingSpriteBy(p, v)
	case *Sprite:
		return touchingSprite(p, v)
	case specialObj:
		if v == Edge {
			return p.checkTouchingScreen(touchingAllEdges) != 0
		} else if v == Mouse {
			x, y := p.g.getMousePos()
			return p.g.touchingPoint(p, x, y)
		}
	}
	panic("Touching: unexpected input")
}

func touchingSprite(dst, src *Sprite) bool {
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

	p.mutex.Lock()
	defer p.mutex.Unlock()
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
	w, h := p.g.size()
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
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.isPenDown = false
}

func (p *Sprite) PenDown() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.isPenDown = true
}

func (p *Sprite) SetPenColor(color Color) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

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
	p.mutex.Lock()
	defer p.mutex.Unlock()

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
	p.mutex.Lock()
	defer p.mutex.Unlock()

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
	p.mutex.Lock()
	defer p.mutex.Unlock()

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
