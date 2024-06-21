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
	"image/color"
	"log"
	"math"
	"reflect"
	"sync"

	"github.com/goplus/spx/internal/anim"
	"github.com/goplus/spx/internal/gdi/clrutil"
	"github.com/goplus/spx/internal/math32"
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
const (
	StateDie   string = "die"
	StateTurn  string = "turn"
	StateGlide string = "glide"
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
	rRect         *math32.RotatedRect
	pivot         math32.Vector2

	sayObj           *sayOrThinker
	quoteObj         *quoter
	animations       map[string]*aniConfig
	greffUniforms    map[string]interface{} // graphic effects
	animBindings     map[string]string
	defaultAnimation string

	penColor color.RGBA
	penShade float64
	penHue   float64
	penWidth float64

	isVisible bool
	isCloned_ bool
	isPenDown bool
	isDying   bool

	hasOnTurning bool
	hasOnMoving  bool
	hasOnCloned  bool
	hasOnTouched bool

	gamer    reflect.Value
	lastAnim *anim.Anim
}

func (p *Sprite) SetDying() { // dying: visible but can't be touched
	p.isDying = true
}

func (p *Sprite) Parent() *Game {
	return p.g
}

func (p *Sprite) init(
	base string, g *Game, name string, sprite *spriteConfig, gamer reflect.Value, shared *sharedImages) {
	if sprite.Costumes != nil {
		p.baseObj.init(base, sprite.Costumes, sprite.getCostumeIndex())
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
	p.pivot = sprite.Pivot

	p.animBindings = make(map[string]string)
	for key, val := range sprite.AnimBindings {
		p.animBindings[key] = val
	}

	p.defaultAnimation = sprite.DefaultAnimation
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
func (p *Sprite) Awake() {
	if p.defaultAnimation != "" {
		if p.isVisible {
			p.Animate(p.defaultAnimation)
		}
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
	p.greffUniforms = cloneMap(src.greffUniforms)

	p.penColor = src.penColor
	p.penShade = src.penShade
	p.penHue = src.penHue
	p.penWidth = src.penWidth

	p.isVisible = src.isVisible
	p.isCloned_ = true
	p.isPenDown = src.isPenDown
	p.isDying = false

	p.hasOnTurning = false
	p.hasOnMoving = false
	p.hasOnCloned = false
	p.hasOnTouched = false
}

func cloneMap(v map[string]interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	ret := make(map[string]interface{}, len(v))
	for k, v := range v {
		ret[k] = v
	}
	return ret
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
	if _, ok := v["currentCostumeIndex"]; ok {
		// TODO(xsw): to be removed
		panic("please change `currentCostumeIndex` => `costumeIndex` in index.json")
	}
	if idx, ok := v["costumeIndex"]; ok {
		dest.costumeIndex_ = int(idx.(float64))
	}
	dest.isCloned_ = false
}

func applySprite(out reflect.Value, sprite Spriter, v specsp) (*Sprite, Spriter) {
	in := reflect.ValueOf(sprite).Elem()
	outPtr := out.Addr().Interface().(Spriter)
	return cloneSprite(out, outPtr, in, v), outPtr
}

func cloneSprite(out reflect.Value, outPtr Spriter, in reflect.Value, v specsp) *Sprite {
	dest := spriteOf(outPtr)
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
	if v != nil { // in loadSprite
		applySpriteProps(dest, v)
	} else { // in sprite.Clone
		dest.OnCloned__1(func() {
			dest.Awake()
		})
		outPtr.Main()
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
	out, outPtr := v.Elem(), v.Interface().(Spriter)
	dest := cloneSprite(out, outPtr, in, nil)
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

func (p *Sprite) fireTouched(obj *Sprite) {
	if p.hasOnTouched {
		p.doWhenTouched(p, obj)
	}
}

func (p *Sprite) OnTouched__0(onTouched func(obj *Sprite)) {
	p.hasOnTouched = true
	p.allWhenTouched = &eventSink{
		prev:  p.allWhenTouched,
		pthis: p,
		sink:  onTouched,
		cond: func(data interface{}) bool {
			return data == p
		},
	}
}

func (p *Sprite) OnTouched__1(onTouched func()) {
	p.OnTouched__0(func(*Sprite) {
		onTouched()
	})
}

func (p *Sprite) OnTouched__2(name string, onTouched func(obj *Sprite)) {
	p.OnTouched__0(func(obj *Sprite) {
		if obj.name == name {
			onTouched(obj)
		}
	})
}

func (p *Sprite) OnTouched__3(name string, onTouched func()) {
	p.OnTouched__2(name, func(*Sprite) {
		onTouched()
	})
}

func (p *Sprite) OnTouched__4(names []string, onTouched func(obj *Sprite)) {
	p.OnTouched__0(func(obj *Sprite) {
		name := obj.name
		for _, v := range names {
			if v == name {
				onTouched(obj)
				return
			}
		}
	})
}

func (p *Sprite) OnTouched__5(names []string, onTouched func()) {
	p.OnTouched__4(names, func(*Sprite) {
		onTouched()
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

func (p *Sprite) Die() { // prototype sprite can't be destroyed, but can die
	aniName := p.getStateAnimName(StateDie)
	p.SetDying()
	if ani, ok := p.animations[aniName]; ok {
		p.goAnimate(aniName, ani)
	}
	if p.isCloned_ {
		p.doDestroy()
	} else {
		p.Hide()
	}
}

func (p *Sprite) Destroy() { // delete this clone
	if p.isCloned_ {
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
	p.Stop(ThisSprite)
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

func (p *Sprite) IsCloned() bool {
	return p.isCloned_
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

func (p *Sprite) getFromAnToForAni(anitype aniTypeEnum, from interface{}, to interface{}) (interface{}, interface{}) {

	if anitype == aniTypeFrame {
		fromval := 0.0
		toval := 0.0
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

		return fromval, toval
	}

	return from, to

}

func (p *Sprite) getStateAnimName(stateName string) string {
	if bindingName, ok := p.animBindings[stateName]; ok {
		return bindingName
	}
	return stateName
}

func (p *Sprite) goAnimate(name string, ani *aniConfig) {
	if p.lastAnim != nil {
		p.lastAnim.Stop()
	}
	var animwg sync.WaitGroup
	animwg.Add(1)

	if ani.OnStart != nil && ani.OnStart.Play != "" {
		p.g.Play__3(ani.OnStart.Play)
	}

	//anim frame
	fromval, toval := p.getFromAnToForAni(ani.AniType, ani.From, ani.To)
	animtype := anim.AnimValTypeFloat
	if ani.AniType == aniTypeFrame {
		animtype = anim.AnimValTypeInt
		p.goSetCostume(ani.From)
		if ani.Fps == 0 { //compute fps
			tovalf, _ := toval.(float64)
			fromvalf, _ := fromval.(float64)
			ani.Fps = math.Abs(tovalf-fromvalf) / ani.Duration
		}
	}
	if ani.AniType == aniTypeGlide {
		animtype = anim.AnimValTypeVector2
	}

	framenum := int(ani.Duration * ani.Fps)
	fps := ani.Fps

	//frame
	//pre_index := p.getCostumeIndex()
	//xy pos
	pre_x := p.x
	pre_y := p.y
	pre_direction := p.direction //turn p.direction

	an := anim.NewAnim(name, animtype, fps, framenum).AddKeyFrame(0, fromval).AddKeyFrame(framenum, toval).SetLoop(ani.IsLoop)
	p.lastAnim = an
	if debugInstr {
		log.Printf("New anim [name %s id %d] from:%v to:%v framenum:%d fps:%f", an.Name, an.Id, fromval, toval, framenum, fps)
	}
	an.SetOnPlayingListener(func(currframe int, isReplay bool, currval interface{}) {
		if debugInstr {
			log.Printf("playing anim [name %s id %d]  currframe %d, val %v", an.Name, an.Id, currframe, currval)
		}
		if isReplay && ani.IsLoop {
			if ani.OnStart != nil && ani.OnStart.Play != "" {
				p.g.Play__3(ani.OnStart.Play)
			}
		}

		switch ani.AniType {
		case aniTypeFrame:
			val, _ := tools.GetFloat(currval)
			p.setCostumeByIndex(int(val))
		case aniTypeMove:
			val, _ := tools.GetFloat(currval)
			sin, cos := math.Sincos(toRadian(pre_direction))
			p.doMoveToForAnim(pre_x+val*sin, pre_y+val*cos, an)
		case aniTypeTurn:
			val, _ := tools.GetFloat(currval)
			p.setDirection(val, false)
		case aniTypeGlide:
			val, ok := currval.(*math32.Vector2)
			if ok {
				p.SetXYpos(val.X, val.Y)
			}

		}

		playaction := ani.OnPlay
		if playaction != nil {
			if ani.AniType != aniTypeFrame && playaction.Costumes != nil {
				costumes := playaction.Costumes
				costumesFrom, costumesTo := p.getFromAnToForAni(aniTypeFrame, costumes.From, costumes.To)
				costumesFromf, _ := costumesFrom.(float64)
				costumesTof, _ := costumesTo.(float64)
				costumeval := ((int)(costumesTof-costumesFromf) + currframe) % (int)(costumesTof)
				p.setCostumeByIndex(costumeval)
			}
		}
	})
	an.SetOnStopingListener(func() {
		if debugInstr {
			log.Printf("stop anim [name %s id %d]  ", an.Name, an.Id)
		}
		animwg.Done()
		p.lastAnim = nil
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

func (p *Sprite) Ask(msg interface{}) {
	panic("todo")
}

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

func (p *Sprite) Quote__0(message string) {
	if message == "" {
		p.doStopQuote()
		return
	}
	p.Quote__2(message, "")
}

func (p *Sprite) Quote__1(message string, secs float64) {
	p.Quote__2(message, "", secs)
}

func (p *Sprite) Quote__2(message, description string, secs ...float64) {
	if debugInstr {
		log.Println("Quote", p.name, message, description, secs)
	}
	p.quote_(message, description)
	if secs != nil {
		p.waitStopQuote(secs[0])
	}
}

// -----------------------------------------------------------------------------

func (p *Sprite) getXY() (x, y float64) {
	return p.x, p.y
}

// DistanceTo func:
//
//	DistanceTo(sprite)
//	DistanceTo(spriteName)
//	DistanceTo(spx.Mouse)
//	DistanceTo(spx.Random)
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
	x, y = p.fixWorldRange(x, y)
	if p.hasOnMoving {
		mi := &MovingInfo{OldX: p.x, OldY: p.y, NewX: x, NewY: y, Obj: p, ani: ani}
		p.doWhenMoving(p, mi)
	}
	if p.isPenDown {
		p.g.movePen(p, x, y)
	}
	p.x, p.y = x, y
	p.getDrawInfo().updateMatrix()
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

	p.Step__2(step, "step")
}

func (p *Sprite) Step__1(step int) {
	p.Step__0(float64(step))
}

func (p *Sprite) Step__2(step float64, animname string) {
	if debugInstr {
		log.Println("Step", p.name, step)
	}
	if ani, ok := p.animations[animname]; ok {
		anicopy := *ani
		anicopy.From = 0
		anicopy.To = step
		anicopy.Duration = math.Abs(step) * ani.Duration
		p.goAnimate(animname, &anicopy)
		return
	}
	p.goMoveForward(step)
}

// Goto func:
//
//	Goto(sprite)
//	Goto(spriteName)
//	Goto(spx.Mouse)
//	Goto(spx.Random)
func (p *Sprite) Goto(obj interface{}) {
	if debugInstr {
		log.Println("Goto", p.name, obj)
	}
	x, y := p.g.objectPos(obj)
	p.SetXYpos(x, y)
}

func (p *Sprite) Glide__0(x, y float64, secs float64) {
	if debugInstr {
		log.Println("Glide", p.name, x, y, secs)
	}
	x0, y0 := p.getXY()
	ani := &aniConfig{
		Duration: secs,
		Fps:      24.0,
		From:     math32.NewVector2(x0, y0),
		To:       math32.NewVector2(x, y),
		AniType:  aniTypeGlide,
	}
	animName := p.getStateAnimName(StateGlide)
	p.goAnimate(animName, ani)
}

func (p *Sprite) Glide__1(obj interface{}, secs float64) {
	if debugInstr {
		log.Println("Glide", obj, secs)
	}
	x, y := p.g.objectPos(obj)
	p.Glide__0(x, y, secs)
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
	None RotationStyle = iota
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
//
//	Turn(degree)
//	Turn(spx.Left)
//	Turn(spx.Right)
//	Turn(ti *spx.TurningInfo)
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
	animName := p.getStateAnimName(StateTurn)
	if ani, ok := p.animations[animName]; ok {
		anicopy := *ani
		anicopy.From = p.direction
		anicopy.To = p.direction + delta
		anicopy.Duration = ani.Duration / 360.0 * math.Abs(delta)
		p.goAnimate(animName, &anicopy)
		return
	}
	if p.setDirection(delta, true) && debugInstr {
		log.Println("Turn", p.name, val)
	}
}

// TurnTo func:
//
//	TurnTo(sprite)
//	TurnTo(spriteName)
//	TurnTo(spx.Mouse)
//	TurnTo(degree)
//	TurnTo(spx.Left)
//	TurnTo(spx.Right)
//	TurnTo(spx.Up)
//	TurnTo(spx.Down)
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

	animName := p.getStateAnimName(StateTurn)
	if ani, ok := p.animations[animName]; ok {
		fromangle := math.Mod(p.direction+360.0, 360.0)
		toangle := math.Mod(angle+360.0, 360.0)
		if toangle-fromangle > 180.0 {
			fromangle = fromangle + 360.0
		}
		if fromangle-toangle > 180.0 {
			toangle = toangle + 360.0
		}
		delta := math.Abs(fromangle - toangle)
		anicopy := *ani
		anicopy.From = fromangle
		anicopy.To = toangle
		anicopy.Duration = ani.Duration / 360.0 * math.Abs(delta)
		p.goAnimate(animName, &anicopy)
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
	p.getDrawInfo().updateMatrix()
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
	p.getDrawInfo().updateMatrix()
}

func (p *Sprite) ChangeSize(delta float64) {
	if debugInstr {
		log.Println("ChangeSize", p.name, delta)
	}
	p.scale += delta
	p.getDrawInfo().updateMatrix()
}

// -----------------------------------------------------------------------------

func (p *Sprite) requireGreffUniforms() map[string]interface{} {
	effs := p.greffUniforms
	if effs == nil {
		effs = make(map[string]interface{})
		p.greffUniforms = effs
	}
	return effs
}

func (p *Sprite) SetEffect(kind EffectKind, val float64) {
	effs := p.requireGreffUniforms()
	effs[kind.String()] = float32(val)
}

func (p *Sprite) ChangeEffect(kind EffectKind, delta float64) {
	effs := p.requireGreffUniforms()
	key := kind.String()
	newVal := float32(delta)
	if oldVal, ok := effs[key]; ok {
		newVal += oldVal.(float32)
	}
	effs[key] = newVal
}

func (p *Sprite) ClearGraphEffects() {
	p.greffUniforms = nil
}

// -----------------------------------------------------------------------------

type Color = color.RGBA

func (p *Sprite) TouchingColor(color Color) bool {
	for _, item := range p.g.items {
		if sp, ok := item.(*Sprite); ok && sp != p {
			ret := p.touchedColor_(sp, color)
			if ret {
				return true
			}
		}
	}
	return false
}

// Touching func:
//
//	Touching(spriteName)
//	Touching(sprite)
//	Touching(spx.Mouse)
//	Touching(spx.Edge)
//	Touching(spx.EdgeLeft)
//	Touching(spx.EdgeTop)
//	Touching(spx.EdgeRight)
//	Touching(spx.EdgeBottom)
func (p *Sprite) Touching(obj interface{}) bool {
	if !p.isVisible || p.isDying {
		return false
	}
	switch v := obj.(type) {
	case string:
		if o := p.g.touchingSpriteBy(p, v); o != nil {
			o.fireTouched(p)
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

func touchingSprite(dst, src *Sprite) bool {
	if !src.isVisible || src.isDying {
		return false
	}
	ret := src.touchingSprite(dst)
	return ret
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
	rect := p.getRotatedRect()
	if rect == nil {
		return
	}
	plist := rect.Points()
	w, h := p.g.worldSize_()

	edge := 2.0

	for _, val := range plist {
		if val.X < float64(-w/2) && (where&touchingScreenLeft) != 0 {
			//w/2-edge,-h/2   edge,h
			rect := math32.NewRotatedRect1(math32.NewRect(float64(-w/2)-edge, float64(-h/2), edge, float64(h)))
			if p.touchRotatedRect(rect) {
				return touchingScreenLeft
			}
		}
		if val.Y > float64(h/2) && (where&touchingScreenTop) != 0 {
			//w/2,h/2+edge   w,edge
			rect := math32.NewRotatedRect1(math32.NewRect(float64(-w/2), float64(h/2)+edge, float64(w), edge))
			if p.touchRotatedRect(rect) {
				return touchingScreenTop
			}
		}
		if val.X > float64(w/2) && (where&touchingScreenRight) != 0 {
			//w/2,-h/2   edge,h
			rect := math32.NewRotatedRect1(math32.NewRect(float64(w/2), float64(-h/2), edge, float64(h)))
			if p.touchRotatedRect(rect) {
				return touchingScreenRight
			}
		}
		if val.Y < float64(-h/2) && (where&touchingScreenBottom) != 0 {
			//w/2,-h/2  w, edge
			rect := math32.NewRotatedRect1(math32.NewRect(float64(-w/2), float64(-h/2), float64(w), edge))
			if p.touchRotatedRect(rect) {
				return touchingScreenBottom
			}
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

func (p *Sprite) GotoBack() {
	p.g.goBackByLayers(p, 1e8)
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

// CostumeWidth returns width of sprite current costume.
func (p *Sprite) CostumeWidth() float64 {
	c := p.costumes[p.costumeIndex_]
	img, _, _ := c.needImage(p.g.fs)
	w, _ := img.Size()
	return float64(w / c.bitmapResolution)
}

// CostumeHeight returns height of sprite current costume.
func (p *Sprite) CostumeHeight() float64 {
	c := p.costumes[p.costumeIndex_]
	img, _, _ := c.needImage(p.g.fs)
	_, h := img.Size()
	return float64(h / c.bitmapResolution)
}

func (p *Sprite) Bounds() *math32.RotatedRect {
	return p.getRotatedRect()
}

/*
func (p *Sprite) Pixel(x, y float64) color.Color {
	c2 := p.costumes[p.costumeIndex_]
	img, cx, cy := c2.needImage(p.g.fs)
	geo := p.getDrawInfo().getPixelGeo(cx, cy)
	color1, p1 := p.getDrawInfo().getPixel(math32.NewVector2(x, y), img, geo)
	if debugInstr {
		log.Printf("<<<< getPixel x, y(%f,%F) p1(%v) color1(%v) geo(%v)  ", x, y, p1, color1, geo)
	}
	return color1
}
*/

// -----------------------------------------------------------------------------

func (p *Sprite) fixWorldRange(x, y float64) (float64, float64) {
	rect := p.getDrawInfo().getUpdateRotateRect(x, y)
	if rect == nil {
		return x, y
	}
	plist := rect.Points()
	for _, val := range plist {
		if p.g.isWorldRange(val) {
			return x, y
		}
	}

	worldW, worldH := p.g.worldSize_()
	maxW := float64(worldW)/2.0 + float64(rect.Size.Width)
	maxH := float64(worldH)/2.0 + float64(rect.Size.Height)

	if x < -maxW {
		x = -maxW
	}
	if x > maxW {
		x = maxW
	}
	if y < -maxH {
		y = -maxH
	}
	if y > maxH {
		y = maxH
	}

	return x, y
}

// -----------------------------------------------------------------------------
