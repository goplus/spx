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
	StateStep  string = "step"
)
const (
	AnimChannelFrame string = "@frame"
	AnimChannelTurn  string = "@turn"
	AnimChannelGlide string = "@glide"
	AnimChannelMove  string = "@move"
)

type Collider struct {
	sprite  *SpriteImpl
	others  map[*SpriteImpl]bool
	othersM sync.Mutex
}

func (c *Collider) SetTouching(other *SpriteImpl, on bool) {
	c.othersM.Lock()
	defer c.othersM.Unlock()

	if other == nil || c.sprite == nil {
		return
	}

	if c.others == nil {
		c.others = make(map[*SpriteImpl]bool)
	}

	_, exist := c.others[other]
	if on {
		if !exist {
			c.others[other] = true
			c.sprite.fireTouchStart(other)
		} else {
			c.sprite.fireTouching(other)
		}
	} else {
		if exist {
			delete(c.others, other)
			c.sprite.fireTouchEnd(other)
		}
	}
}

func (c *Collider) Reset() {
	copy := make(map[*SpriteImpl]bool, len(c.others))
	for k, v := range c.others {
		copy[k] = v
	}
	for other := range copy {
		c.SetTouching(other, false)
		other.collider.SetTouching(c.sprite, false)
	}
}

type Sprite interface {
	IEventSinks
	Shape
	Main()

	Animate(name SpriteAnimationName)
	Ask(msg interface{})
	BounceOffEdge()
	Bounds() *math32.RotatedRect
	ChangeEffect(kind EffectKind, delta float64)
	ChangeHeading(dir float64)
	ChangePenColor(delta float64)
	ChangePenHue(delta float64)
	ChangePenShade(delta float64)
	ChangePenSize(delta float64)
	ChangeSize(delta float64)
	ChangeXpos(dx float64)
	ChangeXYpos(dx, dy float64)
	ChangeYpos(dy float64)
	ClearGraphEffects()
	CostumeHeight() float64
	CostumeIndex() int
	CostumeName() SpriteCostumeName
	CostumeWidth() float64
	DeleteThisClone()
	Destroy()
	Die()
	DistanceTo__0(sprite Sprite) float64
	DistanceTo__1(sprite SpriteName) float64
	DistanceTo__2(obj specialObj) float64
	DistanceTo__3(pos Pos) float64
	Glide__0(x, y float64, secs float64)
	Glide__1(sprite Sprite, secs float64)
	Glide__2(sprite SpriteName, secs float64)
	Glide__3(obj specialObj, secs float64)
	Glide__4(pos Pos, secs float64)
	GoBackLayers(n int)
	Goto__0(sprite Sprite)
	Goto__1(sprite SpriteName)
	Goto__2(obj specialObj)
	GotoBack()
	GotoFront()
	Heading() float64
	Hide()
	HideVar(name string)
	IsCloned() bool
	Move__0(step float64)
	Move__1(step int)
	NextCostume()
	OnCloned__0(onCloned func(data interface{}))
	OnCloned__1(onCloned func())
	OnMoving__0(onMoving func(mi *MovingInfo))
	OnMoving__1(onMoving func())
	OnTouchStart__0(onTouchStart func(Sprite))
	OnTouchStart__1(onTouchStart func())
	OnTouchStart__2(sprite SpriteName, onTouchStart func(Sprite))
	OnTouchStart__3(sprite SpriteName, onTouchStart func())
	OnTurning__0(onTurning func(ti *TurningInfo))
	OnTurning__1(onTurning func())
	Parent() *Game
	PenDown()
	PenUp()
	PrevCostume()
	Quote__0(message string)
	Quote__1(message string, secs float64)
	Quote__2(message, description string)
	Quote__3(message, description string, secs float64)
	Say__0(msg interface{})
	Say__1(msg interface{}, secs float64)
	SetCostume__0(costume SpriteCostumeName)
	SetCostume__1(index int)
	SetCostume__2(index float64)
	SetCostume__3(action switchAction)
	SetDying()
	SetEffect(kind EffectKind, val float64)
	SetHeading(dir float64)
	SetPenColor(color Color)
	SetPenHue(hue float64)
	SetPenShade(shade float64)
	SetPenSize(size float64)
	SetRotationStyle(style RotationStyle)
	SetSize(size float64)
	SetXpos(x float64)
	SetXYpos(x, y float64)
	SetYpos(y float64)
	Show()
	ShowVar(name string)
	Size() float64
	Stamp()
	Step__0(step float64)
	Step__1(step int)
	Step__2(step float64, animation SpriteAnimationName)
	Think__0(msg interface{})
	Think__1(msg interface{}, secs float64)
	Touching__0(sprite SpriteName) bool
	Touching__1(sprite Sprite) bool
	Touching__2(obj specialObj) bool
	TouchingColor(color Color) bool
	Turn__0(degree float64)
	Turn__1(dir specialDir)
	Turn__2(ti *TurningInfo)
	TurnTo__0(sprite Sprite)
	TurnTo__1(sprite SpriteName)
	TurnTo__2(obj specialObj)
	TurnTo__3(degree float64)
	TurnTo__4(dir specialDir)
	Visible() bool
	Xpos() float64
	Ypos() float64
}

type SpriteName = string

type SpriteCostumeName = string

type SpriteAnimationName = string

type SpriteImpl struct {
	baseObj
	eventSinks
	g      *Game
	sprite Sprite
	name   string

	x, y          float64
	scale         float64
	direction     float64
	rotationStyle RotationStyle
	rRect         *math32.RotatedRect
	pivot         math32.Vector2

	sayObj           *sayOrThinker
	quoteObj         *quoter
	animations       map[SpriteAnimationName]*aniConfig
	greffUniforms    map[string]interface{} // graphic effects
	animBindings     map[string]string
	defaultAnimation SpriteAnimationName

	penColor color.RGBA
	penShade float64
	penHue   float64
	penWidth float64

	isVisible bool
	isCloned_ bool
	isPenDown bool
	isDying   bool

	hasOnTurning    bool
	hasOnMoving     bool
	hasOnCloned     bool
	hasOnTouchStart bool
	hasOnTouching   bool
	hasOnTouchEnd   bool

	gamer               reflect.Value
	lastAnim            *anim.Anim
	isWaitingStopAnim   bool
	defaultCostumeIndex int

	collider Collider
}

func (p *SpriteImpl) SetDying() { // dying: visible but can't be touched
	p.isDying = true
}

func (p *SpriteImpl) Parent() *Game {
	return p.g
}

func (p *SpriteImpl) getAllShapes() []Shape {
	return p.g.getAllShapes()
}

func (p *SpriteImpl) init(
	base string, g *Game, name string, spriteCfg *spriteConfig, gamer reflect.Value, shared *sharedImages, sprite Sprite) {
	if spriteCfg.Costumes != nil {
		p.baseObj.init(base, spriteCfg.Costumes, spriteCfg.getCostumeIndex())
	} else {
		p.baseObj.initWith(base, spriteCfg, shared)
	}
	p.defaultCostumeIndex = p.baseObj.costumeIndex_
	p.eventSinks.init(&g.sinkMgr, p)

	p.gamer = gamer
	p.g, p.name, p.sprite = g, name, sprite
	p.x, p.y = spriteCfg.X, spriteCfg.Y
	p.scale = spriteCfg.Size
	p.direction = spriteCfg.Heading
	p.rotationStyle = toRotationStyle(spriteCfg.RotationStyle)
	p.isVisible = spriteCfg.Visible
	p.pivot = spriteCfg.Pivot

	p.animBindings = make(map[string]string)
	for key, val := range spriteCfg.AnimBindings {
		p.animBindings[key] = val
	}

	p.defaultAnimation = spriteCfg.DefaultAnimation
	p.animations = make(map[string]*aniConfig)
	for key, val := range spriteCfg.FAnimations {
		var ani = val
		_, ok := p.animations[key]
		if ok {
			log.Panicf("animation key [%s] is exist", key)
		}
		oldFps := ani.Fps
		oldFrameFps := ani.FrameFps
		if oldFps == 0 {
			ani.Fps = 25
		}
		if ani.FrameFps == 0 {
			ani.FrameFps = 25
		}
		if ani.TurnToDuration == 0 {
			ani.TurnToDuration = 1
		}
		if ani.StepDuration == 0 {
			ani.StepDuration = 0.01
		}
		switch ani.AniType {
		case aniTypeFrame:
			if ani.From == nil {
				if ani.FrameFrom != "" {
					ani.From = ani.FrameFrom
				} else {
					log.Panicf("animation key [%s] missing FrameFrom ", key)
				}
				if ani.FrameTo != "" {
					ani.To = ani.FrameTo
				} else {
					log.Panicf("animation key [%s] missing FrameTo ", key)
				}
			} else {
				if str, ok := ani.From.(string); ok && str != "" {
					ani.FrameFrom = ani.From.(string)
					ani.FrameTo = ani.To.(string)
				}
				ani.From, ani.To = p.getFromAnToForAniFrames(ani.From, ani.To)
			}
			if oldFps == 0 && oldFrameFps != 0 {
				ani.Fps = float64(oldFrameFps)
				ani.FrameFps = oldFrameFps
			} else {
				ani.Fps = oldFps
				ani.FrameFps = int(oldFps)
			}
			from, to := p.getFromAnToForAniFrames(ani.From, ani.To)
			ani.Duration = math.Abs(to-from) / ani.Fps
		case aniTypeMove:
		case aniTypeTurn:
		case aniTypeGlide:
		default:
			log.Panicf("unknown animation type [%s] is exist[%d]", key, ani.AniType)
		}
		p.animations[key] = ani
	}

	for key, val := range spriteCfg.MAnimations {
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

	for key, val := range spriteCfg.TAnimations {
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

	p.collider.others = make(map[*SpriteImpl]bool)
	p.collider.sprite = p
}
func (p *SpriteImpl) awake() {
	p.playDefaultAnim()
}

func (p *SpriteImpl) InitFrom(src *SpriteImpl) {
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
	p.hasOnTouchStart = false
	p.hasOnTouching = false
	p.hasOnTouchEnd = false

	p.collider.others = make(map[*SpriteImpl]bool)
	p.collider.sprite = p
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

func applySpriteProps(dest *SpriteImpl, v specsp) {
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

func applySprite(out reflect.Value, sprite Sprite, v specsp) (*SpriteImpl, Sprite) {
	in := reflect.ValueOf(sprite).Elem()
	outPtr := out.Addr().Interface().(Sprite)
	return cloneSprite(out, outPtr, in, v), outPtr
}

func cloneSprite(out reflect.Value, outPtr Sprite, in reflect.Value, v specsp) *SpriteImpl {
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
	dest.sprite = outPtr
	if v != nil { // in loadSprite
		applySpriteProps(dest, v)
	} else { // in sprite.Clone
		dest.OnCloned__1(func() {
			dest.awake()
		})
		outPtr.Main()
	}
	return dest
}

func Gopt_SpriteImpl_Clone__0(sprite Sprite) {
	Gopt_SpriteImpl_Clone__1(sprite, nil)
}

func Gopt_SpriteImpl_Clone__1(sprite Sprite, data interface{}) {
	src := spriteOf(sprite)
	if debugInstr {
		log.Println("Clone", src.name)
	}
	in := reflect.ValueOf(sprite).Elem()
	v := reflect.New(in.Type())
	out, outPtr := v.Elem(), v.Interface().(Sprite)
	dest := cloneSprite(out, outPtr, in, nil)
	src.g.addClonedShape(src, dest)
	if dest.hasOnCloned {
		dest.doWhenCloned(dest, data)
	}
}

func (p *SpriteImpl) OnCloned__0(onCloned func(data interface{})) {
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

func (p *SpriteImpl) OnCloned__1(onCloned func()) {
	p.OnCloned__0(func(interface{}) {
		onCloned()
	})
}

func (p *SpriteImpl) fireTouchStart(obj *SpriteImpl) {
	if p.hasOnTouchStart {
		p.doWhenTouchStart(p, obj)
	}
}

func (p *SpriteImpl) fireTouching(obj *SpriteImpl) {
	if p.hasOnTouching {
		p.doWhenTouching(p, obj)
	}
}

func (p *SpriteImpl) fireTouchEnd(obj *SpriteImpl) {
	if p.hasOnTouchEnd {
		p.doWhenTouchEnd(p, obj)
	}
}

func (p *SpriteImpl) OnTouchStart__0(onTouchStart func(Sprite)) {
	p.hasOnTouchStart = true
	p.allWhenTouchStart = &eventSink{
		prev:  p.allWhenTouchStart,
		pthis: p,
		sink:  onTouchStart,
		cond: func(data interface{}) bool {
			return data == p
		},
	}
}

func (p *SpriteImpl) OnTouchStart__1(onTouchStart func()) {
	p.OnTouchStart__0(func(Sprite) {
		onTouchStart()
	})
}

func (p *SpriteImpl) OnTouchStart__2(sprite SpriteName, onTouchStart func(Sprite)) {
	p.OnTouchStart__0(func(s Sprite) {
		impl := spriteOf(s)
		if impl != nil && impl.name == sprite {
			onTouchStart(s)
		}
	})
}

func (p *SpriteImpl) OnTouchStart__3(sprite SpriteName, onTouchStart func()) {
	p.OnTouchStart__2(sprite, func(Sprite) {
		onTouchStart()
	})
}

type MovingInfo struct {
	OldX, OldY float64
	NewX, NewY float64
	ani        *anim.Anim
	Obj        *SpriteImpl
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

func (p *SpriteImpl) OnMoving__0(onMoving func(mi *MovingInfo)) {
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

func (p *SpriteImpl) OnMoving__1(onMoving func()) {
	p.OnMoving__0(func(mi *MovingInfo) {
		onMoving()
	})
}

type TurningInfo struct {
	OldDir float64
	NewDir float64
	Obj    *SpriteImpl
}

func (p *TurningInfo) Dir() float64 {
	return p.NewDir - p.OldDir
}

func (p *SpriteImpl) OnTurning__0(onTurning func(ti *TurningInfo)) {
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

func (p *SpriteImpl) OnTurning__1(onTurning func()) {
	p.OnTurning__0(func(*TurningInfo) {
		onTurning()
	})
}

func (p *SpriteImpl) Die() {
	aniName := p.getStateAnimName(StateDie)
	p.SetDying()

	p.Stop(OtherScriptsInSprite)
	if ani, ok := p.animations[aniName]; ok {
		p.goAnimate(aniName, ani)
	}

	p.Destroy()
}

func (p *SpriteImpl) Destroy() { // destroy sprite, whether prototype or cloned
	if debugInstr {
		log.Println("Destroy", p.name)
	}

	p.collider.Reset()
	p.Hide()
	p.doDeleteClone()
	p.g.removeShape(p)
	p.Stop(ThisSprite)
	if p == gco.Current().Obj {
		gco.Abort()
	}
}

// delete only cloned sprite, no effect on prototype sprite.
// Add this interface, to match Scratch.
func (p *SpriteImpl) DeleteThisClone() {
	if !p.isCloned_ {
		return
	}

	p.Destroy()
}

func (p *SpriteImpl) Hide() {
	if debugInstr {
		log.Println("Hide", p.name)
	}

	p.collider.Reset()
	p.doStopSay()
	p.isVisible = false
}

func (p *SpriteImpl) Show() {
	if debugInstr {
		log.Println("Show", p.name)
	}
	p.isVisible = true
}

func (p *SpriteImpl) Visible() bool {
	return p.isVisible
}

func (p *SpriteImpl) IsCloned() bool {
	return p.isCloned_
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) CostumeName() SpriteCostumeName {
	return p.getCostumeName()
}

func (p *SpriteImpl) CostumeIndex() int {
	return p.getCostumeIndex()
}

// SetCostume func:
//
//	SetCostume(costume) or
//	SetCostume(index) or
//	SetCostume(spx.Next) or
//	SetCostume(spx.Prev)
func (p *SpriteImpl) setCostume(costume interface{}) {
	if debugInstr {
		log.Println("SetCostume", p.name, costume)
	}
	p.goSetCostume(costume)
	p.defaultCostumeIndex = p.costumeIndex_
}

func (p *SpriteImpl) SetCostume__0(costume SpriteCostumeName) {
	p.setCostume(costume)
}

func (p *SpriteImpl) SetCostume__1(index int) {
	p.setCostume(index)
}

func (p *SpriteImpl) SetCostume__2(index float64) {
	p.setCostume(index)
}

func (p *SpriteImpl) SetCostume__3(action switchAction) {
	p.setCostume(action)
}

func (p *SpriteImpl) NextCostume() {
	if debugInstr {
		log.Println("NextCostume", p.name)
	}
	p.goNextCostume()
	p.defaultCostumeIndex = p.costumeIndex_
}

func (p *SpriteImpl) PrevCostume() {
	if debugInstr {
		log.Println("PrevCostume", p.name)
	}
	p.goPrevCostume()
	p.defaultCostumeIndex = p.costumeIndex_
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) getFromAnToForAni(anitype aniTypeEnum, from interface{}, to interface{}) (interface{}, interface{}) {

	if anitype == aniTypeFrame {
		return p.getFromAnToForAniFrames(from, to)
	}

	return from, to

}

func (p *SpriteImpl) getFromAnToForAniFrames(from interface{}, to interface{}) (float64, float64) {
	fromval := 0.0
	toval := 0.0
	switch v := from.(type) {
	case SpriteCostumeName:
		fromval = float64(p.findCostume(v))
		if fromval < 0 {
			log.Panicf("findCostume %s failed", v)
		}
	default:
		fromval, _ = tools.GetFloat(from)
	}

	switch v := to.(type) {
	case SpriteCostumeName:
		toval = float64(p.findCostume(v))
		if toval < 0 {
			log.Panicf("findCostume %s failed", v)
		}
	default:
		toval, _ = tools.GetFloat(to)
	}

	return fromval, toval
}

func (p *SpriteImpl) getStateAnimName(stateName string) string {
	if bindingName, ok := p.animBindings[stateName]; ok {
		return bindingName
	}
	return stateName
}

func lerp(a float64, b float64, progress float64) float64 {
	return a + (b-a)*progress
}
func (p *SpriteImpl) goAnimate(name SpriteAnimationName, ani *aniConfig) {
	p.goAnimateInternal(name, ani, true)
}
func (p *SpriteImpl) goAnimateInternal(name SpriteAnimationName, ani *aniConfig, isBlocking bool) {
	if p.lastAnim != nil {
		p.isWaitingStopAnim = true
		p.lastAnim.Stop()
		p.isWaitingStopAnim = false
	}

	var animwg sync.WaitGroup
	if isBlocking {
		animwg.Add(1)
	}

	if ani.OnStart != nil && ani.OnStart.Play != "" {
		p.g.Play__3(ani.OnStart.Play)
	}

	//anim frame
	fromval, toval := p.getFromAnToForAni(ani.AniType, ani.From, ani.To)
	frameFrom, frameTo := 0.0, 0.0
	hasExtraChannel := ani.FrameFrom != "" && ani.FrameTo != ""
	if hasExtraChannel {
		frameFrom, frameTo = p.getFromAnToForAniFrames(ani.FrameFrom, ani.FrameTo)
	}
	fromvalf, tovalf := 0.0, 0.0
	if hasExtraChannel {
		fromvalf = frameFrom
		tovalf = frameTo
	} else {
		if ani.AniType != aniTypeGlide {
			// glide animation, the type of value is vector2, not float
			fromvalf = fromval.(float64)
			tovalf = toval.(float64)
		}
	}

	if ani.AniType == aniTypeFrame {
		p.goSetCostume(ani.From)
		if ani.Fps == 0 { //compute fps
			ani.Fps = math.Abs(tovalf-fromvalf) / ani.Duration
		} else {
			ani.Duration = math.Abs(tovalf-fromvalf) / ani.Fps
		}
	}

	framenum := int(ani.Duration * ani.Fps)
	fps := ani.Fps

	pre_x := p.x
	pre_y := p.y
	pre_direction := p.direction //turn p.direction

	an := anim.NewAnim(name, fps, framenum, ani.IsLoop)
	// create channels
	defaultChannel := []*anim.AnimationKeyFrame{{Frame: 0, Value: fromval}, {Frame: framenum, Value: toval}}
	switch ani.AniType {
	case aniTypeFrame:
		an.AddChannel(AnimChannelFrame, anim.AnimValTypeInt, defaultChannel)
	case aniTypeMove:
		an.AddChannel(AnimChannelMove, anim.AnimValTypeFloat, defaultChannel)
	case aniTypeTurn:
		an.AddChannel(AnimChannelTurn, anim.AnimValTypeFloat, defaultChannel)
	case aniTypeGlide:
		an.AddChannel(AnimChannelGlide, anim.AnimValTypeVector2, defaultChannel)
	}
	if hasExtraChannel && ani.AniType != aniTypeFrame {
		iFrameFrom := int(math.Round(frameFrom))
		iFrameTo := int(math.Round(frameTo))
		frameCount := iFrameTo - iFrameFrom + 1
		framePerIter := int(float64(frameCount) * ani.Fps / float64(ani.FrameFps))
		iterCount := int(framenum / framePerIter)
		is_need_ext := framenum != iterCount*int(ani.FrameFps)
		arySize := iterCount * 2
		if is_need_ext {
			arySize += 2
		}
		keyFrames := make([]*anim.AnimationKeyFrame, arySize)
		i := 0
		for ; i < iterCount; i++ {
			offset := framePerIter * i
			keyFrames[i*2+0] = &anim.AnimationKeyFrame{Frame: offset + 0, Value: iFrameFrom}
			keyFrames[i*2+1] = &anim.AnimationKeyFrame{Frame: offset + framePerIter - 1, Value: iFrameTo}
		}
		if is_need_ext {
			offset := framePerIter * i
			finalFrame := framenum - offset
			lastDuration := float64(finalFrame) / float64(framePerIter)
			finalIFrame := int(lastDuration * float64(frameCount))
			keyFrames[i*2+0] = &anim.AnimationKeyFrame{Frame: offset + 0, Value: iFrameFrom}
			keyFrames[i*2+1] = &anim.AnimationKeyFrame{Frame: offset + finalFrame - 1, Value: iFrameFrom + finalIFrame}
		}
		an.AddChannel(AnimChannelFrame, anim.AnimValTypeInt, keyFrames)
	}

	p.lastAnim = an
	if debugInstr {
		log.Printf("New anim [name %s id %d] from:%v to:%v framenum:%d fps:%f", an.Name, an.Id, fromval, toval, framenum, fps)
	}
	an.SetOnPlayingListener(func(currframe int, isReplay bool, progress float64) {
		if debugInstr {
			log.Printf("playing anim [name %s id %d]  currframe %d", an.Name, an.Id, currframe)
		}
		if isReplay && ani.IsLoop {
			if ani.OnStart != nil && ani.OnStart.Play != "" {
				p.g.Play__3(ani.OnStart.Play)
			}
		}
		frameValue := an.SampleChannel(AnimChannelFrame)
		if frameValue != nil {
			val, _ := tools.GetFloat(frameValue)
			p.setCostumeByIndex(int(val))
		}
		moveValue := an.SampleChannel(AnimChannelMove)
		if moveValue != nil {
			val, _ := tools.GetFloat(moveValue)
			sin, cos := math.Sincos(toRadian(pre_direction))
			p.doMoveToForAnim(pre_x+val*sin, pre_y+val*cos, an)
		}
		turnValue := an.SampleChannel(AnimChannelTurn)
		if turnValue != nil {
			val, _ := tools.GetFloat(turnValue)
			p.setDirection(val, false)
		}
		glideValue := an.SampleChannel(AnimChannelGlide)
		if glideValue != nil {
			val, ok := glideValue.(*math32.Vector2)
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
	isNeedPlayDefault := false
	an.SetOnStopingListener(func() {
		if debugInstr {
			log.Printf("stop anim [name %s id %d]  ", an.Name, an.Id)
		}
		if isBlocking {
			animwg.Done()
		}
		p.lastAnim = nil
		if !p.isWaitingStopAnim && name != p.defaultAnimation && p.isVisible && !ani.IsKeepOnStop {
			dieAnimName := p.getStateAnimName(StateDie)
			if name != dieAnimName {
				isNeedPlayDefault = true
			}
		}
	})

	var h *tickHandler
	h = p.g.startTick(-1, func(tick int64) {
		runing := an.Update(1000.0 / p.g.currentTPS() * float64(tick))
		if !runing {
			h.Stop()
		}
	})
	if isBlocking {
		waitToDo(animwg.Wait)
	}
	if isNeedPlayDefault {
		p.playDefaultAnim()
	}
}

func (p *SpriteImpl) Animate(name SpriteAnimationName) {
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

func (p *SpriteImpl) Ask(msg interface{}) {
	panic("todo")
}

func (p *SpriteImpl) Say__0(msg interface{}) {
	p.Say__1(msg, 0)
}

func (p *SpriteImpl) Say__1(msg interface{}, secs float64) {
	if debugInstr {
		log.Println("Say", p.name, msg, secs)
	}
	p.sayOrThink(msg, styleSay)
	if secs > 0 {
		p.waitStopSay(secs)
	}
}

func (p *SpriteImpl) Think__0(msg interface{}) {
	p.Think__1(msg, 0)
}

func (p *SpriteImpl) Think__1(msg interface{}, secs float64) {
	if debugInstr {
		log.Println("Think", p.name, msg, secs)
	}
	p.sayOrThink(msg, styleThink)
	if secs > 0 {
		p.waitStopSay(secs)
	}
}

func (p *SpriteImpl) Quote__0(message string) {
	if message == "" {
		p.doStopQuote()
		return
	}
	p.Quote__2(message, "")
}

func (p *SpriteImpl) Quote__1(message string, secs float64) {
	p.Quote__3(message, "", secs)
}

func (p *SpriteImpl) Quote__2(message, description string) {
	p.Quote__3(message, description, 0)
}

func (p *SpriteImpl) Quote__3(message, description string, secs float64) {
	if debugInstr {
		log.Println("Quote", p.name, message, description, secs)
	}
	p.quote_(message, description)
	if secs > 0 {
		p.waitStopQuote(secs)
	}
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) getXY() (x, y float64) {
	return p.x, p.y
}

// DistanceTo func:
//
//	DistanceTo(sprite)
//	DistanceTo(spx.Mouse)
//	DistanceTo(spx.Random)
func (p *SpriteImpl) distanceTo(obj interface{}) float64 {
	x, y := p.x, p.y
	x2, y2 := p.g.objectPos(obj)
	x -= x2
	y -= y2
	return math.Sqrt(x*x + y*y)
}

func (p *SpriteImpl) DistanceTo__0(sprite Sprite) float64 {
	return p.distanceTo(sprite)
}

func (p *SpriteImpl) DistanceTo__1(sprite SpriteName) float64 {
	return p.distanceTo(sprite)
}

func (p *SpriteImpl) DistanceTo__2(obj specialObj) float64 {
	return p.distanceTo(obj)
}

func (p *SpriteImpl) DistanceTo__3(pos Pos) float64 {
	return p.distanceTo(pos)
}

func (p *SpriteImpl) doMoveTo(x, y float64) {
	p.doMoveToForAnim(x, y, nil)
}

func (p *SpriteImpl) doMoveToForAnim(x, y float64, ani *anim.Anim) {
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

func (p *SpriteImpl) goMoveForward(step float64) {
	sin, cos := math.Sincos(toRadian(p.direction))
	p.doMoveTo(p.x+step*sin, p.y+step*cos)
}

func (p *SpriteImpl) Move__0(step float64) {
	if debugInstr {
		log.Println("Move", p.name, step)
	}
	p.goMoveForward(step)
}

func (p *SpriteImpl) Move__1(step int) {
	p.Move__0(float64(step))
}

func (p *SpriteImpl) Step__0(step float64) {
	animName := p.getStateAnimName(StateStep)
	p.Step__2(step, animName)
}

func (p *SpriteImpl) Step__1(step int) {
	p.Step__0(float64(step))
}

func (p *SpriteImpl) Step__2(step float64, animation SpriteAnimationName) {
	if debugInstr {
		log.Println("Step", p.name, step)
	}
	if ani, ok := p.animations[animation]; ok {
		anicopy := *ani
		anicopy.From = 0
		anicopy.To = step
		anicopy.AniType = aniTypeMove
		anicopy.Duration = math.Abs(step) * ani.StepDuration

		p.goAnimate(animation, &anicopy)
		return
	}
	p.goMoveForward(step)
}

func (p *SpriteImpl) playDefaultAnim() {
	animName := p.defaultAnimation
	if p.isVisible {
		isPlayAnim := false
		if animName != "" {
			if ani, ok := p.animations[animName]; ok {
				isPlayAnim = true
				anicopy := *ani
				anicopy.IsLoop = true
				p.goAnimateInternal(animName, &anicopy, false)
			}
		}
		if !isPlayAnim {
			p.goSetCostume(p.defaultCostumeIndex)
		}
	}

}

// Goto func:
//
//	Goto(sprite)
//	Goto(spx.Mouse)
//	Goto(spx.Random)
func (p *SpriteImpl) goGoto(obj interface{}) {
	if debugInstr {
		log.Println("Goto", p.name, obj)
	}
	x, y := p.g.objectPos(obj)
	p.SetXYpos(x, y)
}

func (p *SpriteImpl) Goto__0(sprite Sprite) {
	p.goGoto(sprite)
}

func (p *SpriteImpl) Goto__1(sprite SpriteName) {
	p.goGoto(sprite)
}

func (p *SpriteImpl) Goto__2(obj specialObj) {
	p.goGoto(obj)
}

func (p *SpriteImpl) Glide__0(x, y float64, secs float64) {
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

func (p *SpriteImpl) goGlide(obj interface{}, secs float64) {
	if debugInstr {
		log.Println("Glide", obj, secs)
	}
	x, y := p.g.objectPos(obj)
	p.Glide__0(x, y, secs)
}

func (p *SpriteImpl) Glide__1(sprite Sprite, secs float64) {
	p.goGlide(sprite, secs)
}

func (p *SpriteImpl) Glide__2(sprite SpriteName, secs float64) {
	p.goGlide(sprite, secs)
}

func (p *SpriteImpl) Glide__3(obj specialObj, secs float64) {
	p.goGlide(obj, secs)
}

func (p *SpriteImpl) Glide__4(pos Pos, secs float64) {
	p.goGlide(pos, secs)
}

func (p *SpriteImpl) SetXYpos(x, y float64) {
	p.doMoveTo(x, y)
}

func (p *SpriteImpl) ChangeXYpos(dx, dy float64) {
	p.doMoveTo(p.x+dx, p.y+dy)
}

func (p *SpriteImpl) Xpos() float64 {
	return p.x
}

func (p *SpriteImpl) SetXpos(x float64) {
	p.doMoveTo(x, p.y)
}

func (p *SpriteImpl) ChangeXpos(dx float64) {
	p.doMoveTo(p.x+dx, p.y)
}

func (p *SpriteImpl) Ypos() float64 {
	return p.y
}

func (p *SpriteImpl) SetYpos(y float64) {
	p.doMoveTo(p.x, y)
}

func (p *SpriteImpl) ChangeYpos(dy float64) {
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

func (p *SpriteImpl) SetRotationStyle(style RotationStyle) {
	if debugInstr {
		log.Println("SetRotationStyle", p.name, style)
	}
	p.rotationStyle = style
}

func (p *SpriteImpl) Heading() float64 {
	return p.direction
}

// Turn func:
//
//	Turn(degree)
//	Turn(spx.Left)
//	Turn(spx.Right)
//	Turn(ti *spx.TurningInfo)
func (p *SpriteImpl) turn(val interface{}) {
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
		anicopy.Duration = ani.TurnToDuration / 360.0 * math.Abs(delta)
		p.goAnimate(animName, &anicopy)
		return
	}
	if p.setDirection(delta, true) && debugInstr {
		log.Println("Turn", p.name, val)
	}
}

func (p *SpriteImpl) Turn__0(degree float64) {
	p.turn(degree)
}

func (p *SpriteImpl) Turn__1(dir specialDir) {
	p.turn(dir)
}

func (p *SpriteImpl) Turn__2(ti *TurningInfo) {
	p.turn(ti)
}

// TurnTo func:
//
//	TurnTo(sprite)
//	TurnTo(spx.Mouse)
//	TurnTo(degree)
//	TurnTo(spx.Left)
//	TurnTo(spx.Right)
//	TurnTo(spx.Up)
//	TurnTo(spx.Down)
func (p *SpriteImpl) turnTo(obj interface{}) {
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
		anicopy.Duration = ani.TurnToDuration / 360.0 * math.Abs(delta)
		anicopy.AniType = aniTypeTurn
		p.goAnimate(animName, &anicopy)
		return
	}
	if p.setDirection(angle, false) && debugInstr {
		log.Println("TurnTo", p.name, obj)
	}
}

func (p *SpriteImpl) TurnTo__0(sprite Sprite) {
	p.turnTo(sprite)
}

func (p *SpriteImpl) TurnTo__1(sprite SpriteName) {
	p.turnTo(sprite)
}

func (p *SpriteImpl) TurnTo__2(obj specialObj) {
	p.turnTo(obj)
}

func (p *SpriteImpl) TurnTo__3(degree float64) {
	p.turnTo(degree)
}

func (p *SpriteImpl) TurnTo__4(dir specialDir) {
	p.turnTo(dir)
}

func (p *SpriteImpl) SetHeading(dir float64) {
	p.setDirection(dir, false)
}

func (p *SpriteImpl) ChangeHeading(dir float64) {
	p.setDirection(dir, true)
}

func (p *SpriteImpl) setDirection(dir float64, change bool) bool {
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

func (p *SpriteImpl) doTurnTogether(ti *TurningInfo) {
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

func (p *SpriteImpl) Size() float64 {
	v := p.scale
	return v
}

func (p *SpriteImpl) SetSize(size float64) {
	if debugInstr {
		log.Println("SetSize", p.name, size)
	}
	p.scale = size
	p.getDrawInfo().updateMatrix()
}

func (p *SpriteImpl) ChangeSize(delta float64) {
	if debugInstr {
		log.Println("ChangeSize", p.name, delta)
	}
	p.scale += delta
	p.getDrawInfo().updateMatrix()
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) requireGreffUniforms() map[string]interface{} {
	effs := p.greffUniforms
	if effs == nil {
		effs = make(map[string]interface{})
		p.greffUniforms = effs
	}
	return effs
}

func (p *SpriteImpl) SetEffect(kind EffectKind, val float64) {
	effs := p.requireGreffUniforms()
	effs[kind.String()] = float32(val)
}

func (p *SpriteImpl) ChangeEffect(kind EffectKind, delta float64) {
	effs := p.requireGreffUniforms()
	key := kind.String()
	newVal := float32(delta)
	if oldVal, ok := effs[key]; ok {
		newVal += oldVal.(float32)
	}
	effs[key] = newVal
}

func (p *SpriteImpl) ClearGraphEffects() {
	p.greffUniforms = nil
}

// -----------------------------------------------------------------------------

type Color = color.RGBA

func (p *SpriteImpl) TouchingColor(color Color) bool {
	for _, item := range p.g.items {
		if sp, ok := item.(*SpriteImpl); ok && sp != p {
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
//	Touching(sprite)
//	Touching(spx.Mouse)
//	Touching(spx.Edge)
//	Touching(spx.EdgeLeft)
//	Touching(spx.EdgeTop)
//	Touching(spx.EdgeRight)
//	Touching(spx.EdgeBottom)
func (p *SpriteImpl) touching(obj interface{}) bool {
	if !p.isVisible || p.isDying {
		return false
	}
	switch v := obj.(type) {
	case SpriteName:
		if o := p.g.touchingSpriteBy(p, v); o != nil {
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
	case Sprite:
		return touchingSprite(p, spriteOf(v))
	}
	panic("Touching: unexpected input")
}

func (p *SpriteImpl) Touching__0(sprite SpriteName) bool {
	return p.touching(sprite)
}

func (p *SpriteImpl) Touching__1(sprite Sprite) bool {
	return p.touching(sprite)
}

func (p *SpriteImpl) Touching__2(obj specialObj) bool {
	return p.touching(obj)
}

func touchingSprite(dst, src *SpriteImpl) bool {
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

func (p *SpriteImpl) BounceOffEdge() {
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

func (p *SpriteImpl) checkTouchingScreen(where int) (touching int) {
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

func (p *SpriteImpl) GoBackLayers(n int) {
	p.g.goBackByLayers(p, n)
}

func (p *SpriteImpl) GotoFront() {
	p.g.goBackByLayers(p, -1e8)
}

func (p *SpriteImpl) GotoBack() {
	p.g.goBackByLayers(p, 1e8)
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) Stamp() {
	p.g.stampCostume(p.getDrawInfo())
}

func (p *SpriteImpl) PenUp() {
	p.isPenDown = false
}

func (p *SpriteImpl) PenDown() {
	p.isPenDown = true
}

func (p *SpriteImpl) SetPenColor(color Color) {
	h, _, v := clrutil.RGB2HSV(color.R, color.G, color.B)
	p.penHue = (200 * h) / 360
	p.penShade = 50 * v
	p.penColor = color
}

func (p *SpriteImpl) ChangePenColor(delta float64) {
	panic("todo")
}

func (p *SpriteImpl) SetPenShade(shade float64) {
	p.setPenShade(shade, false)
}

func (p *SpriteImpl) ChangePenShade(delta float64) {
	p.setPenShade(delta, true)
}

func (p *SpriteImpl) SetPenHue(hue float64) {
	p.setPenHue(hue, false)
}

func (p *SpriteImpl) ChangePenHue(delta float64) {
	p.setPenHue(delta, true)
}

func (p *SpriteImpl) setPenHue(v float64, change bool) {
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

func (p *SpriteImpl) setPenShade(v float64, change bool) {
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

func (p *SpriteImpl) doUpdatePenColor() {
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

func (p *SpriteImpl) SetPenSize(size float64) {
	p.setPenWidth(size, true)
}

func (p *SpriteImpl) ChangePenSize(delta float64) {
	p.setPenWidth(delta, true)
}

func (p *SpriteImpl) setPenWidth(w float64, change bool) {
	if change {
		w += p.penWidth
	}
	p.penWidth = w
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) HideVar(name string) {
	p.g.setStageMonitor(p.name, getVarPrefix+name, false)
}

func (p *SpriteImpl) ShowVar(name string) {
	p.g.setStageMonitor(p.name, getVarPrefix+name, true)
}

// -----------------------------------------------------------------------------

// CostumeWidth returns width of sprite current costume.
func (p *SpriteImpl) CostumeWidth() float64 {
	c := p.costumes[p.costumeIndex_]
	img, _, _ := c.needImage(p.g.fs)
	w, _ := img.Size()
	return float64(w / c.bitmapResolution)
}

// CostumeHeight returns height of sprite current costume.
func (p *SpriteImpl) CostumeHeight() float64 {
	c := p.costumes[p.costumeIndex_]
	img, _, _ := c.needImage(p.g.fs)
	_, h := img.Size()
	return float64(h / c.bitmapResolution)
}

func (p *SpriteImpl) Bounds() *math32.RotatedRect {
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

func (p *SpriteImpl) fixWorldRange(x, y float64) (float64, float64) {
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
