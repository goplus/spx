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
	"log"
	"math"
	"reflect"

	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/time"
	"github.com/goplus/spx/internal/tools"
	"github.com/realdream-ai/mathf"
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

type Sprite interface {
	IEventSinks
	Shape
	Main()
	Animate(name SpriteAnimationName)
	Ask(msg interface{})
	BounceOffEdge()
	Bounds() *mathf.Rect2
	ChangeEffect(kind EffectKind, delta float64)
	ChangeHeading(dir float64)
	ChangePenColor(kind PenColorParam, delta float64)
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
	DeltaTime() float64
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
	Name() string
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
	SetCostume__1(index float64)
	SetCostume__2(index int)
	SetCostume__3(action switchAction)
	SetDying()
	SetEffect(kind EffectKind, val float64)
	SetHeading(dir float64)
	SetPenColor__0(color Color)
	SetPenColor__1(kind PenColorParam, value float64)
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
	Step__1(step float64, animation SpriteAnimationName)
	Step__2(step int)
	Think__0(msg interface{})
	Think__1(msg interface{}, secs float64)
	TimeSinceLevelLoad() float64
	Touching__0(sprite SpriteName) bool
	Touching__1(sprite Sprite) bool
	Touching__2(obj specialObj) bool
	TouchingColor(color Color) bool
	Turn__0(degree float64)
	Turn__1(dir specialDir)
	Turn__2(ti *TurningInfo)
	TurnTo__0(sprite Sprite)
	TurnTo__1(sprite SpriteName)
	TurnTo__2(degree float64)
	TurnTo__3(dir specialDir)
	TurnTo__4(obj specialObj)
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
	direction     float64
	rotationStyle RotationStyle
	pivot         mathf.Vec2

	sayObj           *sayOrThinker
	quoteObj         *quoter
	animations       map[SpriteAnimationName]*aniConfig
	greffUniforms    map[string]interface{} // graphic effects
	animBindings     map[string]string
	defaultAnimation SpriteAnimationName

	penColor mathf.Color
	penWidth float64

	penHue          float64
	penSaturation   float64
	penBrightness   float64
	penTransparency float64

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

	hasShader bool

	gamer               reflect.Value
	curAnimState        *animState
	defaultCostumeIndex int

	triggerMask   int64
	triggerLayer  int64
	triggerType   int64
	triggerCenter mathf.Vec2
	triggerSize   mathf.Vec2
	triggerRadius float64

	collisionMask  int64
	collisionLayer int64
	colliderType   int64
	colliderCenter mathf.Vec2
	colliderSize   mathf.Vec2
	colliderRadius float64

	penObj  *engine.Object
	audioId engine.Object
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
	base string, g *Game, name string, spriteCfg *spriteConfig, gamer reflect.Value, sprite Sprite) {
	if spriteCfg.Costumes != nil {
		p.baseObj.init(base, spriteCfg.Costumes, spriteCfg.getCostumeIndex())
	} else {
		p.baseObj.initWith(base, spriteCfg)
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

	// bind physic config
	p.collisionMask = parseLayerMaskValue(spriteCfg.CollisionMask)
	p.collisionLayer = parseLayerMaskValue(spriteCfg.CollisionLayer)
	// collider is disable by default
	p.colliderType = paserColliderType(spriteCfg.ColliderType, physicColliderNone)
	p.colliderCenter = spriteCfg.ColliderCenter
	p.colliderSize = spriteCfg.ColliderSize
	p.colliderRadius = spriteCfg.ColliderRadius

	p.triggerMask = parseLayerMaskValue(spriteCfg.TriggerMask)
	p.triggerLayer = parseLayerMaskValue(spriteCfg.TriggerLayer)
	p.triggerType = paserColliderType(spriteCfg.TriggerType, physicColliderAuto)
	p.triggerCenter = spriteCfg.TriggerCenter
	p.triggerSize = spriteCfg.TriggerSize
	p.triggerRadius = spriteCfg.TriggerRadius

	// setup animations
	p.defaultAnimation = spriteCfg.DefaultAnimation
	p.animations = make(map[string]*aniConfig)
	anims := spriteCfg.FAnimations
	for key, val := range anims {
		var ani = val
		_, ok := p.animations[key]
		if ok {
			log.Panicf("animation key [%s] is exist", key)
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
		from, to := p.getFromAnToForAniFrames(ani.FrameFrom, ani.FrameTo)
		ani.IFrameFrom, ani.IFrameTo = int(from), int(to)
		ani.Speed = 1
		ani.Duration = (math.Abs(float64(ani.IFrameFrom-ani.IFrameTo)) + 1) / float64(ani.FrameFps)
		p.animations[key] = ani
	}

	// register animations to engine
	for animName, ani := range p.animations {
		registerAnimToEngine(p.name, animName, ani, p.baseObj.costumes, p.isCostumeSet)
	}

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
	p.penHue = src.penHue
	p.penSaturation = src.penSaturation
	p.penBrightness = src.penBrightness
	p.penTransparency = src.penTransparency

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

	p.hasShader = false

	p.collisionMask = src.collisionMask
	p.collisionLayer = src.collisionLayer
	p.triggerMask = src.triggerMask
	p.triggerLayer = src.triggerLayer

	p.colliderType = src.colliderType
	p.colliderCenter = src.colliderCenter
	p.colliderSize = src.colliderSize
	p.colliderRadius = src.colliderRadius

	p.triggerType = src.triggerType
	p.triggerCenter = src.triggerCenter
	p.triggerSize = src.triggerSize
	p.triggerRadius = src.triggerRadius

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
		dest.setCustumeIndex(int(idx.(float64)))
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
	dest.isCostumeDirty = true
	if v != nil { // in loadSprite
		applySpriteProps(dest, v)
	} else { // in sprite.Clone
		dest.OnCloned__1(func() {
			dest.awake()
		})
		runMain(outPtr.Main)
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
	p.syncSprite = nil
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
	p.syncSprite = nil
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
	Obj        *SpriteImpl
}

func (p *MovingInfo) StopMoving() {
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

	p.Hide()
	p.doDeleteClone()
	p.destroyPen()
	p.g.removeShape(p)
	p.Stop(ThisSprite)
	if p == gco.Current().Obj {
		gco.Abort()
	}
	p.HasDestroyed = true

	if p.audioId != 0 {
		p.g.sounds.releaseAudio(p.audioId)
		p.audioId = 0
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

func (p *SpriteImpl) SetCostume__1(index float64) {
	p.setCostume(index)
}

func (p *SpriteImpl) SetCostume__2(index int) {
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

type animState struct {
	AniType  aniTypeEnum
	Name     string
	Duration float64
	From     interface{}
	To       interface{}
	Speed    float64
	IsLoop   bool

	OnStart      *actionConfig
	OnPlay       *actionConfig
	IsCanceled   bool
	IsKeepOnStop bool
}

func (p *SpriteImpl) goAnimate(name SpriteAnimationName, ani *aniConfig) {
	p.goAnimateInternal(name, ani, true)
}
func (p *SpriteImpl) goAnimateInternal(name SpriteAnimationName, ani *aniConfig, isBlocking bool) *animState {
	info := &animState{
		AniType:      ani.AniType,
		Name:         name,
		Duration:     ani.Duration,
		From:         ani.From,
		To:           ani.To,
		Speed:        ani.Speed,
		IsLoop:       ani.IsLoop,
		OnStart:      ani.OnStart,
		OnPlay:       ani.OnPlay,
		IsKeepOnStop: ani.IsKeepOnStop,
		IsCanceled:   false,
	}
	if p.curAnimState != nil {
		p.curAnimState.IsCanceled = true
	}
	p.curAnimState = info
	if isBlocking {
		doAnimation(p, info)
	} else {
		engine.Go(p.pthis, func() {
			doAnimation(p, info)
		})
	}
	return info
}

func doAnimation(p *SpriteImpl, info *animState) {
	animName := info.Name
	for p.syncSprite == nil {
		engine.WaitNextFrame()
	}
	engine.WaitMainThread(func() {
		if info.IsCanceled {
			return
		}
		p.isCostumeDirty = false
		p.syncSprite.PlayAnim(animName, info.Speed, info.IsLoop, false)
	})
	if info.OnStart != nil && info.OnStart.Play != "" {
		p.Play__3(info.OnStart.Play)
	}
	if info.AniType == aniTypeFrame {
		for spriteMgr.IsPlayingAnim(p.syncSprite.GetId()) {
			if info.IsCanceled {
				break
			}
			engine.WaitNextFrame()
		}
	} else {
		duration := info.Duration
		timer := 0.0
		pre_x, pre_y := p.x, p.y
		pre_direction := p.direction
		for timer < duration {
			timer += time.DeltaTime()
			percent := mathf.Clamp01f(timer / duration)
			switch info.AniType {
			case aniTypeMove:
				src, _ := tools.GetFloat(info.From)
				dst, _ := tools.GetFloat(info.To)
				val := mathf.Lerpf(src, dst, percent)
				sin, cos := math.Sincos(toRadian(pre_direction))
				p.doMoveToForAnim(pre_x+val*sin, pre_y+val*cos)
			case aniTypeGlide:
				src, _ := tools.GetVec2(info.From)
				dst, _ := tools.GetVec2(info.To)
				val := src.Lerp(dst, percent)
				p.SetXYpos(val.X, val.Y)
			case aniTypeTurn:
				src, _ := tools.GetFloat(info.From)
				dst, _ := tools.GetFloat(info.To)
				val := mathf.Lerpf(src, dst, percent)
				p.setDirection(val, false)
			}
			if info.IsCanceled {
				break
			}
			engine.WaitNextFrame()
		}
	}
	if !info.IsCanceled {
		isNeedPlayDefault := false
		if animName != p.defaultAnimation && p.isVisible && !info.IsKeepOnStop {
			dieAnimName := p.getStateAnimName(StateDie)
			if animName != dieAnimName {
				isNeedPlayDefault = true
			}
		}
		if isNeedPlayDefault {
			p.playDefaultAnim()
		}
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
	p.doMoveToForAnim(x, y)
}

func (p *SpriteImpl) doMoveToForAnim(x, y float64) {
	x, y = p.fixWorldRange(x, y)
	if p.hasOnMoving {
		mi := &MovingInfo{OldX: p.x, OldY: p.y, NewX: x, NewY: y, Obj: p}
		p.doWhenMoving(p, mi)
	}
	if p.isPenDown {
		p.movePen(x, y)
	}
	p.x, p.y = x, y
	p.updateTransform()
}
func (p *SpriteImpl) updateTransform() {
	p.updateProxyTransform(false)
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
	p.Step__1(step, animName)
}

func (p *SpriteImpl) Step__1(step float64, animation SpriteAnimationName) {
	if debugInstr {
		log.Println("Step", p.name, step)
	}
	if ani, ok := p.animations[animation]; ok {
		anicopy := *ani
		anicopy.From = 0
		anicopy.To = step
		anicopy.AniType = aniTypeMove
		anicopy.Duration = math.Abs(step) * ani.StepDuration
		anicopy.IsLoop = true
		p.goAnimate(animation, &anicopy)
		return
	}
	p.goMoveForward(step)
}

func (p *SpriteImpl) Step__2(step int) {
	p.Step__0(float64(step))
}

func (p *SpriteImpl) playDefaultAnim() {
	animName := p.defaultAnimation
	if p.isVisible {
		isPlayAnim := false
		if ani, ok := p.animations[animName]; ok {
			isPlayAnim = true
			anicopy := *ani
			anicopy.IsLoop = true
			p.goAnimateInternal(animName, &anicopy, false)
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
	from := mathf.NewVec2(x0, y0)
	to := mathf.NewVec2(x, y)
	ani := &aniConfig{
		Duration: secs,
		From:     &from,
		To:       &to,
		AniType:  aniTypeGlide,
	}
	ani.IsLoop = true
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

func (p *SpriteImpl) Name() string {
	return p.name
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
		anicopy.AniType = aniTypeTurn
		anicopy.IsLoop = true
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
		anicopy.IsLoop = true
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

func (p *SpriteImpl) TurnTo__2(degree float64) {
	p.turnTo(degree)
}

func (p *SpriteImpl) TurnTo__3(dir specialDir) {
	p.turnTo(dir)
}

func (p *SpriteImpl) TurnTo__4(obj specialObj) {
	p.turnTo(obj)
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
	p.updateTransform()
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
	p.updateTransform()
}

func (p *SpriteImpl) ChangeSize(delta float64) {
	if debugInstr {
		log.Println("ChangeSize", p.name, delta)
	}
	p.scale += delta
	p.updateTransform()
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
	effs[kind.String()] = float64(val)

	if !p.hasShader {
		p.syncSprite.SetMaterialShader("res://engine/shader/spx_sprite_shader.gdshader")
		p.hasShader = true
	}

	switch kind {
	case ColorEffect:
		p.syncSprite.UpdateColor(val)
	case BrightnessEffect:
		p.syncSprite.UpdateBrightness(val)
	case GhostEffect:
		p.syncSprite.UpdateAlpha(val)
	case MosaicEffect:
		p.syncSprite.UpdateMosaic(val)
	case WhirlEffect:
		p.syncSprite.UpdateWhirl(val)
	case FishEyeEffect:
		p.syncSprite.UpdateFishEye(val)
	case UVEffect:
		p.syncSprite.UpdateUVEffect(val)
	}
}

func (p *SpriteImpl) ChangeEffect(kind EffectKind, delta float64) {
	effs := p.requireGreffUniforms()
	key := kind.String()
	newVal := float64(delta)
	if oldVal, ok := effs[key]; ok {
		newVal += oldVal.(float64)
	}
	effs[key] = newVal
	p.SetEffect(kind, newVal)
}

func (p *SpriteImpl) ClearGraphEffects() {
	p.greffUniforms = nil
	p.syncSprite.ClearGraphEffects()
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) TouchingColor(color Color) bool {
	panic("todo gdspx")
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

func (p *SpriteImpl) touchPoint(x, y float64) bool {
	if p.syncSprite == nil {
		return false
	}
	return spriteMgr.CheckCollisionWithPoint(p.syncSprite.GetId(), mathf.NewVec2(x, y), true)
}

func (p *SpriteImpl) touchingSprite(dst *SpriteImpl) bool {
	if p.syncSprite == nil || dst.syncSprite == nil {
		return false
	}
	return spriteMgr.CheckCollision(p.syncSprite.GetId(), dst.syncSprite.GetId(), true, true)
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
	if p.syncSprite == nil {
		return 0
	}
	value := physicMgr.CheckTouchedCameraBoundary(p.syncSprite.GetId(), int64(where))
	if value {
		return where
	}
	return 0
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
type PenColorParam int

const (
	PenHue PenColorParam = iota
	PenSaturation
	PenBrightness
	PenTransparency
)

func (p *SpriteImpl) PenUp() {
	p.checkOrCreatePen()
	p.isPenDown = false
	extMgr.PenUp(*p.penObj)
}

func (p *SpriteImpl) PenDown() {
	p.checkOrCreatePen()
	p.isPenDown = true
	p.movePen(p.x, p.y)
	extMgr.PenDown(*p.penObj, false)
}

func (p *SpriteImpl) Stamp() {
	p.checkOrCreatePen()
	extMgr.SetPenStampTexture(*p.penObj, p.getCostumePath())
	extMgr.PenStamp(*p.penObj)
}

func (p *SpriteImpl) SetPenColor__0(color Color) {
	p.checkOrCreatePen()
	p.penColor = toMathfColor(color)
	p.applyPenColorProperty()
}

func (p *SpriteImpl) SetPenColor__1(kind PenColorParam, value float64) {
	switch kind {
	case PenHue:
		p.setPenHue(value)
	case PenSaturation:
		p.setPenSaturation(value)
	case PenBrightness:
		p.setPenBrightness(value)
	case PenTransparency:
		p.setPenTransparency(value)
	}
}

func (p *SpriteImpl) ChangePenColor(kind PenColorParam, delta float64) {
	switch kind {
	case PenHue:
		p.changePenHue(delta)
	case PenSaturation:
		p.changePenSaturation(delta)
	case PenBrightness:
		p.changePenBrightness(delta)
	case PenTransparency:
		p.changePenTransparency(delta)
	}
}

func (p *SpriteImpl) setPenHue(value float64) {
	p.checkOrCreatePen()
	p.penHue = mathf.Clamp(value, 0, 100)
	p.applyPenHsvProperty()
}

func (p *SpriteImpl) changePenHue(delta float64) {
	p.setPenHue(p.penHue + delta)
}

func (p *SpriteImpl) setPenSaturation(value float64) {
	p.checkOrCreatePen()
	p.penSaturation = mathf.Clamp(value, 0, 100)
	p.applyPenHsvProperty()
}

func (p *SpriteImpl) changePenSaturation(delta float64) {
	p.setPenSaturation(p.penSaturation + delta)
}

func (p *SpriteImpl) setPenBrightness(value float64) {
	p.checkOrCreatePen()
	p.penBrightness = mathf.Clamp(value, 0, 100)
	p.applyPenHsvProperty()
}

func (p *SpriteImpl) changePenBrightness(delta float64) {
	p.setPenBrightness(p.penBrightness + delta)
}

func (p *SpriteImpl) setPenTransparency(value float64) {
	p.checkOrCreatePen()
	p.penTransparency = mathf.Clamp(value, 0, 100)
	p.applyPenHsvProperty()
}

func (p *SpriteImpl) changePenTransparency(delta float64) {
	p.setPenTransparency(p.penTransparency + delta)
}

func (p *SpriteImpl) SetPenSize(size float64) {
	p.checkOrCreatePen()
	p.penWidth = size
	extMgr.SetPenSizeTo(*p.penObj, size)
}

func (p *SpriteImpl) ChangePenSize(delta float64) {
	p.checkOrCreatePen()
	p.SetPenSize(p.penWidth + delta)
}

func (p *SpriteImpl) checkOrCreatePen() {
	if p.penObj == nil {
		obj := extMgr.CreatePen()
		p.penObj = &obj
		p.penTransparency = p.penColor.A * 100
	}
}

func (p *SpriteImpl) destroyPen() {
	if p.penObj != nil {
		extMgr.DestroyPen(*p.penObj)
		p.penObj = nil
	}
}

func (p *SpriteImpl) movePen(x, y float64) {
	if p.penObj == nil {
		return
	}
	applyRenderOffset(p, &x, &y)
	extMgr.MovePenTo(*p.penObj, mathf.NewVec2(x, -y))
}

func (p *SpriteImpl) applyPenColorProperty() {
	p.checkOrCreatePen()
	h, s, v := p.penColor.ToHSV()
	p.penHue = (h / 360) * 100
	p.penSaturation = s * 100
	p.penBrightness = v * 100
	p.penTransparency = p.penColor.A * 100
	extMgr.SetPenColorTo(*p.penObj, p.penColor)
}

func (p *SpriteImpl) applyPenHsvProperty() {
	color := mathf.NewColorHSV((p.penHue/100)*360, p.penSaturation/100, p.penBrightness/100)
	p.penColor = color
	p.penColor.A = p.penTransparency / 100
	extMgr.SetPenColorTo(*p.penObj, p.penColor)
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
	w, _ := c.getSize()
	return float64(w)
}

// CostumeHeight returns height of sprite current costume.
func (p *SpriteImpl) CostumeHeight() float64 {
	c := p.costumes[p.costumeIndex_]
	_, h := c.getSize()
	return float64(h)
}

func (p *SpriteImpl) Bounds() *mathf.Rect2 {
	if !p.isVisible {
		return nil
	}
	x, y, w, h := 0.0, 0.0, 0.0, 0.0
	c := p.costumes[p.costumeIndex_]
	// calc center
	x, y = p.x, p.y
	applyRenderOffset(p, &x, &y)

	if p.triggerType != physicColliderNone {
		x += p.triggerCenter.X
		y += p.triggerCenter.Y
		w = p.triggerSize.X
		h = p.triggerSize.Y
	} else {
		// calc scale
		wi, hi := c.getSize()
		w, h = float64(wi)*p.scale, float64(hi)*p.scale
	}

	rect := mathf.NewRect2(x-w*0.5, y-h*0.5, w, h)
	return &rect

}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) fixWorldRange(x, y float64) (float64, float64) {
	rect := p.Bounds()
	if rect == nil {
		return x, y
	}
	worldW, worldH := p.g.worldSize_()
	maxW := float64(worldW)/2.0 + float64(rect.Size.X)
	maxH := float64(worldH)/2.0 + float64(rect.Size.Y)
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

// ------------------------ Extra events ----------------------------------------
func (pself *SpriteImpl) onUpdate(delta float64) {
	if pself.quoteObj != nil {
		pself.quoteObj.refresh()
	}
	if pself.sayObj != nil {
		pself.sayObj.refresh()
	}
}

// ------------------------ time ----------------------------------------

func (pself *SpriteImpl) DeltaTime() float64 {
	return time.DeltaTime()
}

func (pself *SpriteImpl) TimeSinceLevelLoad() float64 {
	return time.TimeSinceLevelLoad()
}

// ------------------------ sound ----------------------------------------

type SoundEffectKind int

const (
	SoundPanEffect SoundEffectKind = iota
	SoundPitchEffect
)

// Play func:
//
//	Play(sound)
//	Play(video) -- maybe
//	Play(media, wait) -- sync
//	Play(media, opts)

func (p *SpriteImpl) Play__0(media Sound, action *PlayOptions) {
	if debugInstr {
		log.Println("Play", media.Path)
	}

	p.checkAudioId()
	err := p.g.play(p.audioId, media, action)
	if err != nil {
		panic(err)
	}
}

func (p *SpriteImpl) Play__1(media Sound, wait bool) {
	p.Play__0(media, &PlayOptions{Wait: wait})
}

func (p *SpriteImpl) Play__2(media Sound) {
	if media == nil {
		panic("play media is nil")
	}
	p.Play__0(media, &PlayOptions{})
}

func (p *SpriteImpl) Play__3(media SoundName) {
	p.Play__5(media, &PlayOptions{})
}

func (p *SpriteImpl) Play__4(media SoundName, wait bool) {
	p.Play__5(media, &PlayOptions{Wait: wait})
}

func (p *SpriteImpl) Play__5(media SoundName, action *PlayOptions) {
	m, err := p.g.loadSound(media)
	if err != nil {
		log.Println(err)
		return
	}
	p.Play__0(m, action)
}

func (p *SpriteImpl) Volume() float64 {
	return p.g.sounds.getVolume(p.audioId)
}

func (p *SpriteImpl) SetVolume(volume float64) {
	p.checkAudioId()
	p.g.sounds.setVolume(p.audioId, volume)
}

func (p *SpriteImpl) ChangeVolume(delta float64) {
	p.checkAudioId()
	p.g.sounds.changeVolume(p.audioId, delta)
}

func (p *SpriteImpl) GetSoundEffect(kind SoundEffectKind) float64 {
	p.checkAudioId()
	return p.g.sounds.getEffect(p.audioId, kind)
}
func (p *SpriteImpl) SetSoundEffect(kind SoundEffectKind, value float64) {
	p.checkAudioId()
	p.g.sounds.setEffect(p.audioId, kind, value)
}
func (p *SpriteImpl) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	p.checkAudioId()
	p.g.sounds.changeEffect(p.audioId, kind, delta)
}
func (p *SpriteImpl) checkAudioId() {
	if p.audioId == 0 {
		p.audioId = p.g.sounds.allocAudio()
	}
}
