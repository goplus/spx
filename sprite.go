/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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
	"context"
	"fmt"
	"log"
	"maps"
	"math"
	"reflect"
	"sync"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/time"
	"github.com/goplus/spx/v2/internal/tools"
)

type Direction = float64

type specialObj int

const (
	Right Direction = 90
	Left  Direction = -90
	Up    Direction = 0
	Down  Direction = 180
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
	Animate__0(name SpriteAnimationName)
	Animate__1(name SpriteAnimationName, loop bool)
	AnimateAndWait(name SpriteAnimationName)
	StopAnimation(name SpriteAnimationName)
	Ask(msg any)
	BounceOffEdge()
	ChangeGraphicEffect(kind EffectKind, delta float64)
	ChangeHeading(dir Direction)
	ChangePenColor(kind PenColorParam, delta float64)
	ChangePenSize(delta float64)
	ChangeSize(delta float64)
	ChangeXpos(dx float64)
	ChangeXYpos(dx, dy float64)
	ChangeYpos(dy float64)
	ClearGraphicEffects()
	CostumeIndex() int
	CostumeName() SpriteCostumeName
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
	SetLayer__0(layer layerAction)
	SetLayer__1(dir dirAction, delta int)
	Heading() Direction
	Hide()
	HideVar(name string)
	IsCloned() bool
	Name() string
	OnCloned__0(onCloned func(data any))
	OnCloned__1(onCloned func())
	OnTouchStart__0(sprite SpriteName, onTouchStart func(Sprite))
	OnTouchStart__1(sprite SpriteName, onTouchStart func())
	OnTouchStart__2(sprites []SpriteName, onTouchStart func(Sprite))
	OnTouchStart__3(sprites []SpriteName, onTouchStart func())
	PenDown()
	PenUp()
	Quote__0(message string)
	Quote__1(message string, secs float64)
	Quote__2(message, description string)
	Quote__3(message, description string, secs float64)
	Say__0(msg any)
	Say__1(msg any, secs float64)
	SetCostume__0(costume SpriteCostumeName)
	SetCostume__1(index float64)
	SetCostume__2(index int)
	SetCostume__3(action switchAction)
	SetGraphicEffect(kind EffectKind, val float64)
	SetHeading(dir Direction)
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
	Step__1(step float64, speed float64)
	Step__2(step float64, speed float64, animation SpriteAnimationName)
	StepTo__0(sprite Sprite)
	StepTo__1(sprite SpriteName)
	StepTo__2(x, y float64)
	StepTo__3(obj specialObj)
	StepTo__4(sprite Sprite, speed float64)
	StepTo__5(sprite SpriteName, speed float64)
	StepTo__6(x, y, speed float64)
	StepTo__7(obj specialObj, speed float64)
	StepTo__8(sprite Sprite, speed float64, animation SpriteAnimationName)
	StepTo__9(sprite SpriteName, speed float64, animation SpriteAnimationName)
	StepTo__a(x, y, speed float64, animation SpriteAnimationName)
	StepTo__b(obj specialObj, speed float64, animation SpriteAnimationName)
	Think__0(msg any)
	Think__1(msg any, secs float64)
	TimeSinceLevelLoad() float64
	Touching__0(sprite SpriteName) bool
	Touching__1(sprite Sprite) bool
	Touching__2(obj specialObj) bool
	TouchingColor(color Color) bool
	Turn__0(dir Direction)
	Turn__1(dir Direction, speed float64)
	Turn__2(dir Direction, speed float64, animation SpriteAnimationName)
	TurnTo__0(target Sprite)
	TurnTo__1(target SpriteName)
	TurnTo__2(dir Direction)
	TurnTo__3(target specialObj)
	TurnTo__4(target Sprite, speed float64)
	TurnTo__5(target SpriteName, speed float64)
	TurnTo__6(dir Direction, speed float64)
	TurnTo__7(target specialObj, speed float64)
	TurnTo__8(target Sprite, speed float64, animation SpriteAnimationName)
	TurnTo__9(target SpriteName, speed float64, animation SpriteAnimationName)
	TurnTo__a(dir Direction, speed float64, animation SpriteAnimationName)
	TurnTo__b(target specialObj, speed float64, animation SpriteAnimationName)
	Visible() bool
	Xpos() float64
	Ypos() float64

	Volume() float64
	SetVolume(volume float64)
	ChangeVolume(delta float64)
	GetSoundEffect(kind SoundEffectKind) float64
	SetSoundEffect(kind SoundEffectKind, value float64)
	ChangeSoundEffect(kind SoundEffectKind, delta float64)
	Play__0(name SoundName, loop bool)
	Play__1(name SoundName)
	PlayAndWait(name SoundName)
	PausePlaying(name SoundName)
	ResumePlaying(name SoundName)
	StopPlaying(name SoundName)

	// physic
	SetPhysicsMode(mode PhysicsMode)
	PhysicsMode() PhysicsMode
	SetVelocity(velocityX, velocityY float64)
	Velocity() (velocityX, velocityY float64)
	SetGravity(gravity float64)
	Gravity() float64
	AddImpulse(impulseX, impulseY float64)
	IsOnFloor() bool
	SetColliderShape(isTrigger bool, ctype ColliderShapeType, params []float64) error
	ColliderShape(isTrigger bool) (ColliderShapeType, []float64)
	SetColliderPivot(isTrigger bool, offsetX, offsetY float64)
	ColliderPivot(isTrigger bool) (offsetX, offsetY float64)

	SetCollisionLayer(layer int64)
	SetCollisionMask(mask int64)
	SetCollisionEnabled(enabled bool)
	CollisionLayer() int64
	CollisionMask() int64
	CollisionEnabled() bool

	SetTriggerEnabled(trigger bool)
	SetTriggerLayer(layer int64)
	SetTriggerMask(mask int64)
	TriggerLayer() int64
	TriggerMask() int64
	TriggerEnabled() bool
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

	sayObj            *sayOrThinker
	quoteObj          *quoter
	animations        map[SpriteAnimationName]*aniConfig
	animBindings      map[string]string
	defaultAnimation  SpriteAnimationName
	animationWrappers map[SpriteAnimationName]*animationWrapper // lazy load

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

	hasOnCloned     bool
	hasOnTouchStart bool
	hasOnTouching   bool
	hasOnTouchEnd   bool

	gamer               reflect.Value
	curAnimState        *animState
	curTweenState       *animState
	defaultCostumeIndex int

	triggerInfo   physicConfig
	collisionInfo physicConfig

	penObj  *engine.Object
	audioId engine.Object

	collisionTargets map[string]bool
	pendingAudios    []string
	donedAnimations  []string

	physicsMode PhysicsMode
	mass        float64
	friction    float64
	airDrag     float64
	gravity     float64
}

type animationWrapper struct {
	spr      *SpriteImpl
	ani      *aniConfig
	loaded   bool
	loadOnce sync.Once
}

func (aw *animationWrapper) ensureRegistered(pName string) {
	aw.loadOnce.Do(func() {
		createAnimation(aw.spr.name, pName, aw.ani, aw.spr.costumes, aw.spr.isCostumeSet)
		aw.loaded = true
	})

	aw.spr.adaptAnimBitmapResolution(aw.ani)
}

func (p *SpriteImpl) adaptAnimBitmapResolution(ani *aniConfig) {
	renderScale := p.getAnimRenderScale(ani.AdaptAnimBitmapResolution)
	p.syncSprite.SetRenderScale(mathf.NewVec2(renderScale, renderScale))
}

func (p *SpriteImpl) setDying() { // dying: visible but can't be touched
	p.isDying = true
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
	maps.Copy(p.animBindings, spriteCfg.AnimBindings)

	p.collisionTargets = make(map[string]bool)

	// bind physic config
	p.collisionInfo.Mask = parseLayerMaskValue(spriteCfg.CollisionMask)
	p.collisionInfo.Layer = parseLayerMaskValue(spriteCfg.CollisionLayer)
	// collider is disable by default
	var defaultCollisionType int64 = physicsColliderNone
	if enabledPhysics {
		defaultCollisionType = physicsColliderAuto
	}
	p.collisionInfo.Type = paserColliderShapeType(spriteCfg.CollisionShapeType, defaultCollisionType)
	p.collisionInfo.Pivot = spriteCfg.CollisionPivot
	p.collisionInfo.Params = spriteCfg.CollisionShapeParams
	// Validate colliderShapeType and colliderShape length matching
	if !p.collisionInfo.validateShape() {
		fmt.Printf("Warning: Invalid collider configuration for sprite %s, using default values\n", p.name)
		p.collisionInfo.Type = physicsColliderNone
		p.collisionInfo.Params = nil
	}

	p.triggerInfo.Mask = parseLayerMaskValue(spriteCfg.TriggerMask)
	p.triggerInfo.Layer = parseLayerMaskValue(spriteCfg.TriggerLayer)
	p.triggerInfo.Type = paserColliderShapeType(spriteCfg.TriggerShapeType, physicsColliderAuto)
	p.triggerInfo.Pivot = spriteCfg.TriggerPivot
	p.triggerInfo.Params = spriteCfg.TriggerShapeParams
	// Validate triggerType and triggerShape length matching
	if !p.triggerInfo.validateShape() {
		fmt.Printf("Warning: Invalid trigger configuration for sprite %s, using default values\n", p.name)
		p.triggerInfo.Type = physicsColliderAuto
		p.triggerInfo.Params = nil
	}

	p.physicsMode = toPhysicsMode(spriteCfg.PhysicsMode)
	p.airDrag = parseDefaultFloatValue(spriteCfg.AirDrag, 1)
	p.gravity = parseDefaultFloatValue(spriteCfg.Gravity, 1)
	p.friction = parseDefaultFloatValue(spriteCfg.Friction, 1)
	p.mass = parseDefaultFloatValue(spriteCfg.Mass, 1)

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

	// lazy register animations to engine
	p.animationWrappers = make(map[SpriteAnimationName]*animationWrapper)
	for animName, ani := range p.animations {
		p.animationWrappers[animName] = &animationWrapper{spr: p, ani: ani}
	}

	p.pendingAudios = make([]string, 0)
	// create engine object
	p.syncSprite = nil
	engine.WaitMainThread(func() {
		p.syncCheckInitProxy()
	})
}

func (p *SpriteImpl) awake() {
	p.playDefaultAnim()
}

func (p *SpriteImpl) initCollisionParams() {
	if p.g.isAutoSetCollisionLayer {
		info := p.g.getSpriteCollisionInfo(p.name)
		p.collisionInfo.Layer = 0
		p.collisionInfo.Mask = 0
		p.triggerInfo.Layer = int64(info.Layer)
		p.triggerInfo.Mask = int64(info.Mask)
		if enabledPhysics {
			p.collisionInfo.Layer = int64(info.Layer)
			p.collisionInfo.Mask = int64(info.Mask)
		}
	}
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
	p.animationWrappers = make(map[SpriteAnimationName]*animationWrapper)
	for animName, ani := range p.animations {
		p.animationWrappers[animName] = &animationWrapper{spr: p, ani: ani}
	}
	// clone effect params
	p.greffUniforms = maps.Clone(src.greffUniforms)

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

	p.hasOnCloned = false
	p.hasOnTouchStart = false
	p.hasOnTouching = false
	p.hasOnTouchEnd = false

	p.collisionInfo.copyFrom(&src.collisionInfo)
	p.triggerInfo.copyFrom(&src.triggerInfo)

	p.pendingAudios = make([]string, 0)
}

func cloneMap(v map[string]any) map[string]any {
	if v == nil {
		return nil
	}
	ret := make(map[string]any, len(v))
	for k, v := range v {
		ret[k] = v
	}
	return ret
}

func applyFloat64(out *float64, in any) {
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
		dest.onAwake(func() {
			dest.awake()
		})
		runMain(outPtr.Main)
	}
	dest.syncSprite = nil
	dest.curAnimState = nil
	dest.curTweenState = nil
	engine.WaitMainThread(func() {
		dest.syncCheckInitProxy()
		syncCheckUpdateCostume(&dest.baseObj)
	})
	return dest
}

func Gopt_SpriteImpl_Clone__0(sprite Sprite) {
	Gopt_SpriteImpl_Clone__1(sprite, nil)
}

func Gopt_SpriteImpl_Clone__1(sprite Sprite, data any) {
	doClone(sprite, data, false, nil)
}

func doClone(sprite Sprite, data any, isAsync bool, onCloned func(sprite *SpriteImpl)) {
	src := spriteOf(sprite)
	if debugInstr {
		log.Println("Clone", src.name)
	}
	in := reflect.ValueOf(sprite).Elem()
	v := reflect.New(in.Type())
	out, outPtr := v.Elem(), v.Interface().(Sprite)
	dest := cloneSprite(out, outPtr, in, nil)
	src.g.addClonedShape(src, dest)
	if onCloned != nil {
		onCloned(dest)
	}
	if dest.hasOnCloned {
		if isAsync {
			engine.Go(dest.pthis, func(ctx context.Context) {
				dest.doWhenAwake(dest)
				dest.doWhenCloned(dest, data)
			})
		} else {
			dest.doWhenAwake(dest)
			dest.doWhenCloned(dest, data)
		}
	}
}
func (p *SpriteImpl) OnCloned__0(onCloned func(data any)) {
	p.hasOnCloned = true
	p.allWhenCloned = &eventSink{
		prev:  p.allWhenCloned,
		pthis: p,
		sink:  onCloned,
		cond: func(data any) bool {
			return data == p
		},
	}
}

func (p *SpriteImpl) OnCloned__1(onCloned func()) {
	p.OnCloned__0(func(any) {
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

func (p *SpriteImpl) _onTouchStart(onTouchStart func(Sprite)) {
	p.hasOnTouchStart = true
	p.allWhenTouchStart = &eventSink{
		prev:  p.allWhenTouchStart,
		pthis: p,
		sink:  onTouchStart,
		cond: func(data any) bool {
			return data == p
		},
	}
}

func (p *SpriteImpl) onTouchStart__0(onTouchStart func(Sprite)) {
	// collision with other sprites by default
	for name, _ := range p.g.sprs {
		p.collisionTargets[name] = true
	}
	p._onTouchStart(onTouchStart)
}

func (p *SpriteImpl) onTouchStart__1(onTouchStart func()) {
	// collision with all other sprites by default
	for name, _ := range p.g.sprs {
		p.collisionTargets[name] = true
	}
	p._onTouchStart(func(Sprite) {
		onTouchStart()
	})
}

func (p *SpriteImpl) OnTouchStart__0(sprite SpriteName, onTouchStart func(Sprite)) {
	p.collisionTargets[sprite] = true
	p._onTouchStart(func(s Sprite) {
		impl := spriteOf(s)
		if impl != nil && impl.name == sprite {
			onTouchStart(s)
		}
	})
}

func (p *SpriteImpl) OnTouchStart__1(sprite SpriteName, onTouchStart func()) {
	p.collisionTargets[sprite] = true
	p.OnTouchStart__0(sprite, func(Sprite) {
		onTouchStart()
	})
}

func (p *SpriteImpl) OnTouchStart__2(sprites []SpriteName, onTouchStart func(Sprite)) {
	for _, sprite := range sprites {
		p.collisionTargets[sprite] = true
	}
	p._onTouchStart(func(s Sprite) {
		impl := spriteOf(s)
		if impl != nil {
			for _, sprite := range sprites {
				if impl.name == sprite {
					onTouchStart(s)
					return
				}
			}
		}
	})
}

func (p *SpriteImpl) OnTouchStart__3(sprites []SpriteName, onTouchStart func()) {
	for _, sprite := range sprites {
		p.collisionTargets[sprite] = true
	}
	p.OnTouchStart__2(sprites, func(Sprite) {
		onTouchStart()
	})
}

func (p *SpriteImpl) Die() {
	aniName := p.getStateAnimName(StateDie)
	p.setDying()

	p.Stop(OtherScriptsInSprite)
	if p.hasAnim(aniName) {
		p.AnimateAndWait(aniName)
	}
	p.Destroy()
}

func (p *SpriteImpl) Destroy() { // destroy sprite, whether prototype or cloned
	if debugInstr {
		log.Println("Destroy", p.name)
	}

	p.syncSprite.UnRegisterOnAnimationFinished()

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
func (p *SpriteImpl) setCostume(costume any) {
	if debugInstr {
		log.Println("SetCostume", p.name, costume)
	}
	p.goSetCostume(costume)
	p.defaultCostumeIndex = p.costumeIndex_
	p.updateTransform()
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

// -----------------------------------------------------------------------------

func (p *SpriteImpl) getFromAnToForAni(anitype aniTypeEnum, from any, to any) (any, any) {

	if anitype == aniTypeFrame {
		return p.getFromAnToForAniFrames(from, to)
	}

	return from, to

}

func (p *SpriteImpl) getFromAnToForAniFrames(from any, to any) (float64, float64) {
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

func (p *SpriteImpl) hasAnim(animName string) bool {
	if _, ok := p.animations[animName]; ok {
		return true
	}
	return false
}

type animState struct {
	AniType    aniTypeEnum
	Name       string
	IsCanceled bool
	Speed      float64
	AudioName  string
	AudioId    soundId
}

func (p *SpriteImpl) onAnimationDone(animName string) {
	if p.curAnimState != nil && p.curAnimState.Name == animName {
		p.playDefaultAnim()
	}
}

func (p *SpriteImpl) doAnimation(animName SpriteAnimationName, ani *aniConfig, loop bool, speed float64, isBlocking bool, playAudio bool) {
	p.stopAnimState(p.curAnimState)
	p.curAnimState = &animState{
		AniType:    aniTypeFrame,
		IsCanceled: false,
		Name:       animName,
		Speed:      speed,
	}
	info := p.curAnimState
	if playAudio {
		p.playAnimAudio(ani, info)
	}

	syncCheckUpdateCostume(&p.baseObj)
	p.animationWrappers[animName].ensureRegistered(animName)

	spriteMgr.PlayAnim(p.syncSprite.GetId(), animName, speed, loop, false)
	if isBlocking {
		p.isAnimating = true
		for spriteMgr.IsPlayingAnim(p.syncSprite.GetId()) {
			if info.IsCanceled {
				break
			}
			engine.WaitNextFrame()
		}
		p.isAnimating = false
		p.stopAnimState(info)
	}
}

func (p *SpriteImpl) doTween(name SpriteAnimationName, ani *aniConfig) {
	info := &animState{
		AniType:    ani.AniType,
		Name:       name,
		Speed:      ani.Speed,
		IsCanceled: false,
	}
	p.stopAnimState(p.curTweenState)
	p.curTweenState = info
	animName := info.Name
	if p.hasAnim(animName) {
		p.doAnimation(animName, ani, ani.IsLoop, ani.Speed, false, false)
		p.playAnimAudio(ani, info)
	}
	duration := ani.Duration
	timer := 0.0
	prePercent := 0.0
	for timer < duration {
		if info.IsCanceled {
			return
		}
		timer += time.DeltaTime()
		percent := mathf.Clamp01f(timer / duration)
		deltaPercent := percent - prePercent
		prePercent = percent
		switch ani.AniType {
		case aniTypeMove:
			src, _ := tools.GetVec2(ani.From)
			dst, _ := tools.GetVec2(ani.To)
			diff := dst.Sub(src)
			if enabledPhysics && p.physicsMode != NoPhysics && p.physicsMode != StaticPhysics {
				speed := diff.Length() / duration
				dir := diff.Normalize()
				vel := dir.Mulf(speed)
				p.SetVelocity(vel.X, vel.Y)
			} else {
				val := diff.Mulf(deltaPercent)
				p.ChangeXYpos(val.X, val.Y)
			}
		case aniTypeGlide:
			src, _ := tools.GetVec2(ani.From)
			dst, _ := tools.GetVec2(ani.To)
			diff := dst.Sub(src)
			val := diff.Mulf(deltaPercent)
			p.ChangeXYpos(val.X, val.Y)
		case aniTypeTurn:
			src, _ := tools.GetFloat(ani.From)
			dst, _ := tools.GetFloat(ani.To)
			diff := dst - src
			val := diff * deltaPercent
			p.ChangeHeading(val)
		}
		engine.WaitNextFrame()
	}
	switch ani.AniType {
	case aniTypeMove:
		if enabledPhysics && p.physicsMode != NoPhysics && p.physicsMode != StaticPhysics {
			p.SetVelocity(0, 0)
		}
	}
	p.stopAnimState(info)
	p.curTweenState = nil
	if animName != p.defaultAnimation && !ani.IsKeepOnStop {
		p.playDefaultAnim()
	}
}

func (p *SpriteImpl) stopAnimState(state *animState) {
	if state == nil {
		return
	}
	state.IsCanceled = true
	// don't need to stop audio when anim is canceled
	//p.stopAnimAudio(state)
}

func (p *SpriteImpl) stopAnimAudio(state *animState) {
	if state != nil {
		if state.AudioName != "" {
			p.g.stopSoundInstance(state.AudioId)
		}
	}
}
func (p *SpriteImpl) playAnimAudio(ani *aniConfig, info *animState) {
	if ani.OnStart != nil && ani.OnStart.Play != "" {
		info.AudioName = ani.OnStart.Play
		info.AudioId = p.playAudio(info.AudioName, false)
	}
}

func (p *SpriteImpl) Animate__0(name SpriteAnimationName) {
	p.Animate__1(name, false)
}

func (p *SpriteImpl) Animate__1(name SpriteAnimationName, loop bool) {
	if debugInstr {
		log.Println("==> Animation", name)
	}
	if ani, ok := p.animations[name]; ok {
		p.doAnimation(name, ani, loop, 1, false, true)
	} else {
		log.Println("Animation not found:", name)
	}
}

func (p *SpriteImpl) AnimateAndWait(name SpriteAnimationName) {
	if debugInstr {
		log.Println("==> AnimateAndWait", name)
	}
	if ani, ok := p.animations[name]; ok {
		p.doAnimation(name, ani, false, 1, true, true)
	} else {
		log.Println("Animation not found:", name)
	}
}

func (p *SpriteImpl) StopAnimation(name SpriteAnimationName) {
	if name == "" {
		return
	}
	if !p.hasAnim(name) {
		return
	}
	if p.curAnimState == nil || p.curAnimState.Name != name {
		return
	}
	p.syncSprite.PauseAnim()
	p.playDefaultAnim()
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) Ask(msg any) {
	if debugInstr {
		log.Println("Ask", p.name, msg)
	}
	msgStr, ok := msg.(string)
	if !ok {
		msgStr = fmt.Sprint(msg)
	}
	if msgStr == "" {
		println("ask: msg should not be empty")
		return
	}
	p.Say__0(msgStr)
	p.g.ask(true, msgStr, func(answer string) {
		p.doStopSay()
	})
}

func (p *SpriteImpl) Say__0(msg any) {
	p.Say__1(msg, 0)
}

func (p *SpriteImpl) Say__1(msg any, secs float64) {
	if debugInstr {
		log.Println("Say", p.name, msg, secs)
	}
	p.sayOrThink(msg, styleSay)
	if secs > 0 {
		p.waitStopSay(secs)
	}
}

func (p *SpriteImpl) Think__0(msg any) {
	p.Think__1(msg, 0)
}

func (p *SpriteImpl) Think__1(msg any, secs float64) {
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
func (p *SpriteImpl) distanceTo(obj any) float64 {
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
	if p.isPenDown {
		p.movePen(x, y)
	}
	p.x, p.y = x, y
	p.updateTransform()
}
func (p *SpriteImpl) updateTransform() {
	p.updateProxyTransform(false)
}

func (p *SpriteImpl) updateScale() {
	p.triggerInfo.applyShape(p.syncSprite, true, p.scale)
	p.collisionInfo.applyShape(p.syncSprite, false, p.scale)
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
	p.doStep(step, 1, "")
}

func (p *SpriteImpl) Step__1(step float64, speed float64) {
	p.doStep(step, speed, "")
}

func (p *SpriteImpl) Step__2(step float64, speed float64, animation SpriteAnimationName) {
	p.doStep(step, speed, animation)
}

func (p *SpriteImpl) doStepToPos(x, y, speed float64, animation SpriteAnimationName) {
	if animation == "" {
		animation = p.getStateAnimName(StateStep)
	}
	// if no animation, goto target immediately
	if !p.hasAnim(animation) {
		p.SetXYpos(x, y)
	} else {
		speed = math.Max(speed, 0.001)
		from := mathf.NewVec2(p.x, p.y)
		to := mathf.NewVec2(x, y)
		distance := from.DistanceTo(to)
		if ani, ok := p.animations[animation]; ok {
			anicopy := *ani
			anicopy.From = &from
			anicopy.To = &to
			anicopy.AniType = aniTypeMove
			anicopy.Duration = math.Abs(distance) * ani.StepDuration / speed
			anicopy.IsLoop = true
			anicopy.Speed = speed
			p.doTween(animation, &anicopy)
			return
		}
	}
}
func (p *SpriteImpl) doStepTo(obj any, speed float64, animation SpriteAnimationName) {
	if debugInstr {
		log.Println("Goto", p.name, obj)
	}
	x, y := p.g.objectPos(obj)
	p.doStepToPos(x, y, speed, animation)
}
func (p *SpriteImpl) doStep(step float64, speed float64, animation SpriteAnimationName) {
	dirSin, dirCos := math.Sincos(toRadian(p.direction))
	diff := mathf.NewVec2(step*dirSin, step*dirCos)
	to := mathf.NewVec2(p.x, p.y).Add(diff)
	p.doStepToPos(to.X, to.Y, speed, animation)
}
func (p *SpriteImpl) playDefaultAnim() {
	animName := ""
	if !p.isVisible || p.isDying {
		return
	}
	speed := 1.0
	if p.curTweenState == nil {
		animName = p.defaultAnimation
	} else {
		switch p.curTweenState.AniType {
		case aniTypeMove:
			animName = p.getStateAnimName(StateStep)
			break
		case aniTypeTurn:
			animName = p.getStateAnimName(StateTurn)
			break
		case aniTypeGlide:
			animName = p.getStateAnimName(StateGlide)
			break
		}
		speed = p.curTweenState.Speed
	}

	if animName == "" {
		animName = p.defaultAnimation
	}

	if _, ok := p.animations[animName]; ok {
		p.animationWrappers[animName].ensureRegistered(animName)
		spriteMgr.PlayAnim(p.syncSprite.GetId(), animName, speed, true, false)
	} else {
		p.goSetCostume(p.defaultCostumeIndex)
	}
}

func (p *SpriteImpl) StepTo__0(sprite Sprite) {
	p.doStepTo(sprite, 1, "")
}

func (p *SpriteImpl) StepTo__1(sprite SpriteName) {
	p.doStepTo(sprite, 1, "")
}

func (p *SpriteImpl) StepTo__2(x, y float64) {
	p.doStepToPos(x, y, 1, "")
}

func (p *SpriteImpl) StepTo__3(obj specialObj) {
	p.doStepTo(obj, 1, "")
}

func (p *SpriteImpl) StepTo__4(sprite Sprite, speed float64) {
	p.doStepTo(sprite, speed, "")
}

func (p *SpriteImpl) StepTo__5(sprite SpriteName, speed float64) {
	p.doStepTo(sprite, speed, "")
}

func (p *SpriteImpl) StepTo__6(x, y, speed float64) {
	p.doStepToPos(x, y, speed, "")
}

func (p *SpriteImpl) StepTo__7(obj specialObj, speed float64) {
	p.doStepTo(obj, speed, "")
}

func (p *SpriteImpl) StepTo__8(sprite Sprite, speed float64, animation SpriteAnimationName) {
	p.doStepTo(sprite, speed, animation)
}

func (p *SpriteImpl) StepTo__9(sprite SpriteName, speed float64, animation SpriteAnimationName) {
	p.doStepTo(sprite, speed, animation)
}

func (p *SpriteImpl) StepTo__a(x, y, speed float64, animation SpriteAnimationName) {
	p.doStepToPos(x, y, speed, animation)
}

func (p *SpriteImpl) StepTo__b(obj specialObj, speed float64, animation SpriteAnimationName) {
	p.doStepTo(obj, speed, animation)
}

func (p *SpriteImpl) doGlideTo(obj any, secs float64) {
	if debugInstr {
		log.Println("Glide", obj, secs)
	}
	x, y := p.g.objectPos(obj)
	p.doGlide(x, y, secs)
}

func (p *SpriteImpl) doGlide(x, y float64, secs float64) {
	if debugInstr {
		log.Println("Glide", p.name, x, y, secs)
	}
	x0, y0 := p.getXY()
	from := mathf.NewVec2(x0, y0)
	to := mathf.NewVec2(x, y)
	anicopy := aniConfig{
		Duration: secs,
		From:     &from,
		To:       &to,
		AniType:  aniTypeGlide,
	}
	anicopy.IsLoop = true
	animName := p.getStateAnimName(StateGlide)
	p.doTween(animName, &anicopy)
}
func (p *SpriteImpl) Glide__0(x, y float64, secs float64) {
	p.doGlide(x, y, secs)
}

func (p *SpriteImpl) Glide__1(sprite Sprite, secs float64) {
	p.doGlideTo(sprite, secs)
}

func (p *SpriteImpl) Glide__2(sprite SpriteName, secs float64) {
	p.doGlideTo(sprite, secs)
}

func (p *SpriteImpl) Glide__3(obj specialObj, secs float64) {
	p.doGlideTo(obj, secs)
}

func (p *SpriteImpl) Glide__4(pos Pos, secs float64) {
	p.doGlideTo(pos, secs)
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

func (p *SpriteImpl) Heading() Direction {
	return p.direction
}

func (p *SpriteImpl) Name() string {
	return p.name
}

func (p *SpriteImpl) Turn__0(dir Direction) {
	p.doTurn(dir, 1, "")

}
func (p *SpriteImpl) Turn__1(dir Direction, speed float64) {
	p.doTurn(dir, speed, "")

}
func (p *SpriteImpl) Turn__2(dir Direction, speed float64, animation SpriteAnimationName) {
	p.doTurn(dir, speed, animation)
}
func (p *SpriteImpl) TurnTo__0(target Sprite) {
	p.doTurnTo(target, 1, "")
}
func (p *SpriteImpl) TurnTo__1(target SpriteName) {
	p.doTurnTo(target, 1, "")
}
func (p *SpriteImpl) TurnTo__2(dir Direction) {
	p.doTurnTo(dir, 1, "")
}
func (p *SpriteImpl) TurnTo__3(target specialObj) {
	p.doTurnTo(target, 1, "")
}
func (p *SpriteImpl) TurnTo__4(target Sprite, speed float64) {
	p.doTurnTo(target, speed, "")
}
func (p *SpriteImpl) TurnTo__5(target SpriteName, speed float64) {
	p.doTurnTo(target, speed, "")
}
func (p *SpriteImpl) TurnTo__6(dir Direction, speed float64) {
	p.doTurnTo(dir, speed, "")
}
func (p *SpriteImpl) TurnTo__7(target specialObj, speed float64) {
	p.doTurnTo(target, speed, "")
}
func (p *SpriteImpl) TurnTo__8(target Sprite, speed float64, animation SpriteAnimationName) {
	p.doTurnTo(target, speed, animation)
}
func (p *SpriteImpl) TurnTo__9(target SpriteName, speed float64, animation SpriteAnimationName) {
	p.doTurnTo(target, speed, animation)
}
func (p *SpriteImpl) TurnTo__a(dir Direction, speed float64, animation SpriteAnimationName) {
	p.doTurnTo(dir, speed, animation)
}
func (p *SpriteImpl) TurnTo__b(target specialObj, speed float64, animation SpriteAnimationName) {
	p.doTurnTo(target, speed, animation)
}

func (p *SpriteImpl) doTurn(val Direction, speed float64, animation SpriteAnimationName) {
	delta := val
	if animation == "" {
		animation = p.getStateAnimName(StateTurn)
	}
	if ani, ok := p.animations[animation]; ok {
		anicopy := *ani
		anicopy.From = p.direction
		anicopy.To = p.direction + delta
		anicopy.Duration = ani.TurnToDuration / 360.0 * math.Abs(delta) / speed
		anicopy.AniType = aniTypeTurn
		anicopy.IsLoop = true
		anicopy.Speed = speed
		p.doTween(animation, &anicopy)
		return
	}
	p.setDirection(delta, true)
	if debugInstr {
		log.Println("Turn", p.name, val)
	}
}

func (p *SpriteImpl) doTurnTo(obj any, speed float64, animation SpriteAnimationName) {
	var angle float64
	switch v := obj.(type) {
	case Direction:
		angle = v
	default:
		x, y := p.g.objectPos(obj)
		dx := x - p.x
		dy := y - p.y
		angle = 90 - math.Atan2(dy, dx)*180/math.Pi
	}

	if animation == "" {
		animation = p.getStateAnimName(StateTurn)
	}
	if ani, ok := p.animations[animation]; ok {
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
		anicopy.Duration = ani.TurnToDuration / 360.0 * math.Abs(delta) / speed
		anicopy.AniType = aniTypeTurn
		anicopy.IsLoop = true
		anicopy.Speed = speed
		p.doTween(animation, &anicopy)
		return
	}
	if p.setDirection(angle, false) && debugInstr {
		log.Println("TurnTo", p.name, obj)
	}
}
func (p *SpriteImpl) SetHeading(dir Direction) {
	p.setDirection(dir, false)
}

func (p *SpriteImpl) ChangeHeading(dir Direction) {
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
	p.direction = dir
	p.updateTransform()
	return true
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
	p.isCostumeDirty = true
	p.updateScale()
}

func (p *SpriteImpl) ChangeSize(delta float64) {
	if debugInstr {
		log.Println("ChangeSize", p.name, delta)
	}
	p.SetSize(p.scale + delta)
}

// -----------------------------------------------------------------------------
func (p *SpriteImpl) SetGraphicEffect(kind EffectKind, val float64) {
	p.baseObj.setGraphicEffect(kind, val)
}

func (p *SpriteImpl) ChangeGraphicEffect(kind EffectKind, delta float64) {
	p.baseObj.changeGraphicEffect(kind, delta)
}

func (p *SpriteImpl) ClearGraphicEffects() {
	p.baseObj.clearGraphicEffects()
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) TouchingColor(color Color) bool {
	return p.touchingColor(toMathfColor(color))
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
func (p *SpriteImpl) touching(obj any) bool {
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

func (p *SpriteImpl) touchingColor(color mathf.Color) bool {
	if p.syncSprite == nil {
		return false
	}
	return spriteMgr.CheckCollisionByColor(p.syncSprite.GetId(), color, 0.05, 0.1)
}

func (p *SpriteImpl) touchingSprite(dst *SpriteImpl) bool {
	if p.syncSprite == nil || dst.syncSprite == nil {
		return false
	}
	return spriteMgr.CheckCollisionWithSpriteByAlpha(p.syncSprite.GetId(), dst.syncSprite.GetId(), 0.05)
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
	touching = (int)(physicMgr.CheckTouchedCameraBoundaries(p.syncSprite.GetId()))
	return touching & where
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) SetLayer__0(layer layerAction) {
	switch layer {
	case Front:
		p.g.gotoFront(p)
	case Back:
		p.g.gotoBack(p)
	}

}
func (p *SpriteImpl) SetLayer__1(dir dirAction, delta int) {
	switch dir {
	case Forward:
		p.g.goBackLayers(p, -delta)
	case Backward:
		p.g.goBackLayers(p, delta)
	}
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
	penMgr.PenUp(*p.penObj)
}

func (p *SpriteImpl) PenDown() {
	p.checkOrCreatePen()
	p.isPenDown = true
	p.movePen(p.x, p.y)
	penMgr.PenDown(*p.penObj, false)
}

func (p *SpriteImpl) Stamp() {
	p.checkOrCreatePen()
	penMgr.SetPenStampTexture(*p.penObj, p.getCostumePath())
	penMgr.PenStamp(*p.penObj)
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
	penMgr.SetPenSizeTo(*p.penObj, size)
}

func (p *SpriteImpl) ChangePenSize(delta float64) {
	p.checkOrCreatePen()
	p.SetPenSize(p.penWidth + delta)
}

func (p *SpriteImpl) checkOrCreatePen() {
	if p.penObj == nil {
		obj := penMgr.CreatePen()
		p.penObj = &obj
		p.penTransparency = p.penColor.A * 100
	}
}

func (p *SpriteImpl) destroyPen() {
	if p.penObj != nil {
		penMgr.DestroyPen(*p.penObj)
		p.penObj = nil
	}
}

func (p *SpriteImpl) movePen(x, y float64) {
	if p.penObj == nil {
		return
	}
	applyRenderOffset(p, &x, &y)
	penMgr.MovePenTo(*p.penObj, mathf.NewVec2(x, -y))
}

func (p *SpriteImpl) applyPenColorProperty() {
	p.checkOrCreatePen()
	h, s, v := p.penColor.ToHSV()
	p.penHue = (h / 360) * 100
	p.penSaturation = s * 100
	p.penBrightness = v * 100
	p.penTransparency = p.penColor.A * 100
	penMgr.SetPenColorTo(*p.penObj, p.penColor)
}

func (p *SpriteImpl) applyPenHsvProperty() {
	color := mathf.NewColorHSV((p.penHue/100)*360, p.penSaturation/100, p.penBrightness/100)
	p.penColor = color
	p.penColor.A = p.penTransparency / 100
	penMgr.SetPenColorTo(*p.penObj, p.penColor)
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) HideVar(name string) {
	p.g.setStageMonitor(p.name, getVarPrefix+name, false)
}

func (p *SpriteImpl) ShowVar(name string) {
	p.g.setStageMonitor(p.name, getVarPrefix+name, true)
}

// -----------------------------------------------------------------------------

func (p *SpriteImpl) bounds() *mathf.Rect2 {
	if !p.isVisible {
		return nil
	}
	x, y, w, h := 0.0, 0.0, 0.0, 0.0
	c := p.costumes[p.costumeIndex_]
	// calc center
	x, y = p.x, p.y
	applyRenderOffset(p, &x, &y)

	if p.triggerInfo.Type != physicsColliderNone {
		if p.triggerInfo.Type == physicsColliderAuto && p.syncSprite == nil {
			// if sprite's proxy is not created, use the sync version to get the bound
			center, size := getCostumeBoundByAlpha(p, p.scale, false)
			// Update sprite state atomically to prevent race conditions
			p.triggerInfo.Pivot = center
			p.triggerInfo.Params = []float64{size.X, size.Y}
		}
		x += p.triggerInfo.Pivot.X
		y += p.triggerInfo.Pivot.Y
		// Calculate dimensions from triggerShape based on type
		w, h = p.triggerInfo.getDimensions()
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
	rect := p.bounds()
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

func (p *SpriteImpl) playAudio(name SoundName, loop bool) soundId {
	p.checkAudioId()
	return p.g.playSound(p.syncSprite, p.audioId, name, loop, p.g.audioAttenuation, p.g.audioMaxDistance)
}

func (p *SpriteImpl) Play__0(name SoundName, loop bool) {
	p.checkAudioId()
	p.g.playSound(p.syncSprite, p.audioId, name, loop, p.g.audioAttenuation, p.g.audioMaxDistance)
}

func (p *SpriteImpl) Play__1(name SoundName) {
	p.Play__0(name, false)
}

func (p *SpriteImpl) PlayAndWait(name SoundName) {
	p.checkAudioId()
	p.g.playSoundAndWait(p.syncSprite, p.audioId, name, p.g.audioAttenuation, p.g.audioMaxDistance)
}

func (p *SpriteImpl) PausePlaying(name SoundName) {
	p.checkAudioId()
	p.g.pauseSound(p.audioId, name)
}
func (p *SpriteImpl) ResumePlaying(name SoundName) {
	p.checkAudioId()
	p.g.resumeSound(p.audioId, name)
}
func (p *SpriteImpl) StopPlaying(name SoundName) {
	p.checkAudioId()
	p.g.stopSound(p.audioId, name)
}

func (p *SpriteImpl) Volume() float64 {
	p.checkAudioId()
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

// ------------------------ physic ----------------------------------------
type PhysicsMode = int64

const (
	NoPhysics        PhysicsMode = 0 // Pure visual, no collision, best performance (current default) eg: decorators
	KinematicPhysics PhysicsMode = 1 // Code-controlled movement with collision detection eg: player
	DynamicPhysics   PhysicsMode = 2 // Affected by physics, automatic gravity and collision eg: items
	StaticPhysics    PhysicsMode = 3 // Static immovable, but has collision, affects other objects : eg: walls
)

type ColliderShapeType = int64

const (
	RectCollider      ColliderShapeType = ColliderShapeType(physicsColliderRect)
	CircleCollider    ColliderShapeType = ColliderShapeType(physicsColliderCircle)
	CapsuleCollider   ColliderShapeType = ColliderShapeType(physicsColliderCapsule)
	PolygonCollider   ColliderShapeType = ColliderShapeType(physicsColliderPolygon)
	TriggerExtraPixel float64           = 2.0
)

// physicConfig common structure for physics configuration
type physicConfig struct {
	Mask        int64             // collision/trigger mask
	Layer       int64             // collision/trigger layer
	Type        ColliderShapeType // collider/trigger type
	Pivot       mathf.Vec2        // pivot position
	Params      []float64         // shape parameters
	PivotOffset mathf.Vec2        // pivot offset for render offset adjustment
}

func (cfg *physicConfig) String() string {
	return fmt.Sprintf("Mask: %d, Layer: %d, Type: %d, Pivot: %v, Params: %v", cfg.Mask, cfg.Layer, cfg.Type, cfg.Pivot, cfg.Params)
}

func (cfg *physicConfig) copyFrom(src *physicConfig) {
	cfg.Mask = src.Mask
	cfg.Layer = src.Layer
	cfg.Type = src.Type
	cfg.Pivot = src.Pivot
	cfg.PivotOffset = src.PivotOffset
	cfg.Params = make([]float64, len(src.Params))
	copy(cfg.Params, src.Params)
}

// validateShape validates if shape parameters match the type
func (cfg *physicConfig) validateShape() bool {
	if cfg.Type == physicsColliderNone || cfg.Type == physicsColliderAuto {
		return true
	}

	var expectedLen int
	var typeName string

	switch cfg.Type {
	case physicsColliderRect:
		expectedLen = 2
		typeName = "RectTrigger"
	case physicsColliderCircle:
		expectedLen = 1
		typeName = "CircleTrigger"
	case physicsColliderCapsule:
		expectedLen = 2
		typeName = "CapsuleTrigger"
	case physicsColliderPolygon:
		if len(cfg.Params) < 6 || len(cfg.Params)%2 != 0 {
			fmt.Printf("Shape validation error: PolygonTrigger requires at least 6 parameters (3 vertices) and even count, got %d\n", len(cfg.Params))
			return false
		}
		return true
	default:
		fmt.Printf("Shape validation error: Unknown trigger type: %d\n", cfg.Type)
		return false
	}

	if len(cfg.Params) != expectedLen {
		fmt.Printf("Shape validation error: %s requires exactly %d parameters, got %d\n", typeName, expectedLen, len(cfg.Params))
		return false
	}
	return true
}

// getDimensions calculates width and height based on type and shape parameters
func (cfg *physicConfig) getDimensions() (float64, float64) {
	switch cfg.Type {
	case physicsColliderRect:
		if len(cfg.Params) >= 2 {
			return math.Max(cfg.Params[0], 0), math.Max(cfg.Params[1], 0)
		}
	case physicsColliderCircle:
		if len(cfg.Params) >= 1 {
			radius := math.Max(cfg.Params[0], 0)
			return radius * 2, radius * 2
		}
	case physicsColliderCapsule:
		if len(cfg.Params) >= 2 {
			radius := math.Max(cfg.Params[0], 0)
			height := math.Max(cfg.Params[1], 0)
			return radius * 2, height
		}
	default:
		if len(cfg.Params) >= 2 {
			return math.Max(cfg.Params[0], 0), math.Max(cfg.Params[1], 0)
		}
	}
	return 0, 0
}

// syncToProxy synchronizes physics configuration to engine proxy
func (cfg *physicConfig) syncToProxy(syncProxy *engine.Sprite, isTrigger bool, sprite *SpriteImpl) {
	if isTrigger {
		syncProxy.SetTriggerLayer(cfg.Layer)
		syncProxy.SetTriggerMask(cfg.Mask)
		cfg.syncShape(syncProxy, true, sprite)
	} else {
		syncProxy.SetCollisionLayer(cfg.Layer)
		syncProxy.SetCollisionMask(cfg.Mask)
		cfg.syncShape(syncProxy, false, sprite)
	}
}

// syncShape synchronizes shape to engine proxy
func (cfg *physicConfig) syncShape(syncProxy *engine.Sprite, isTrigger bool, sprite *SpriteImpl) {
	scale := sprite.scale
	if cfg.Type != physicsColliderNone && cfg.Type != physicsColliderAuto {
		center := mathf.NewVec2(0, 0)
		applyRenderOffset(sprite, &center.X, &center.Y)
		cfg.PivotOffset = center.Divf(scale)
	}
	if cfg.Type == physicsColliderAuto {
		pivot, autoSize := syncGetCostumeBoundByAlpha(sprite, 1.0)
		if isTrigger {
			autoSize.X += TriggerExtraPixel
			autoSize.Y += TriggerExtraPixel
		}
		cfg.Pivot = pivot
		cfg.Params = []float64{autoSize.X, autoSize.Y}
	}
	cfg.applyShape(syncProxy, isTrigger, scale)
}

func (cfg *physicConfig) applyShape(syncProxy *engine.Sprite, isTrigger bool, scale float64) {
	pivot := cfg.Pivot.Sub(cfg.PivotOffset)
	pivot = pivot.Mulf(scale)
	switch cfg.Type {
	case physicsColliderCircle:
		syncProxy.SetColliderEnabled(isTrigger, true)
		if len(cfg.Params) >= 1 {
			syncProxy.SetColliderShapeCircle(isTrigger, pivot, math.Max(cfg.Params[0]*scale, 0.01))
		}
	case physicsColliderRect:
		syncProxy.SetColliderEnabled(isTrigger, true)
		if len(cfg.Params) >= 2 {
			syncProxy.SetColliderShapeRect(isTrigger, pivot, mathf.NewVec2(cfg.Params[0]*scale, cfg.Params[1]*scale))
		}
	case physicsColliderCapsule:
		syncProxy.SetColliderEnabled(isTrigger, true)
		if len(cfg.Params) >= 2 {
			syncProxy.SetColliderShapeCapsule(isTrigger, pivot, mathf.NewVec2(cfg.Params[0]*scale*2, cfg.Params[1]*scale))
		}
	case physicsColliderAuto:
		syncProxy.SetColliderEnabled(isTrigger, true)
		if len(cfg.Params) >= 2 {
			syncProxy.SetColliderShapeRect(isTrigger, pivot, mathf.NewVec2(cfg.Params[0]*scale, cfg.Params[1]*scale))
		}
	case physicsColliderNone:
		syncProxy.SetColliderEnabled(isTrigger, false)
	}
}

func toPhysicsMode(mode string) PhysicsMode {
	if mode == "" {
		return NoPhysics
	}
	switch mode {
	case "kinematic":
		return KinematicPhysics
	case "dynamic":
		return DynamicPhysics
	case "static":
		return StaticPhysics
	case "no":
		return NoPhysics
	}
	println("config error: unknown physic mode ", mode)
	return NoPhysics
}

func (p *SpriteImpl) AddImpulse(impulseX, impulseY float64) {
	spriteMgr.AddImpulse(p.getSpriteId(), mathf.NewVec2(impulseX, impulseY))
}

func (p *SpriteImpl) IsOnFloor() bool {
	return spriteMgr.IsOnFloor(p.getSpriteId())
}

func (p *SpriteImpl) SetPhysicsMode(mode PhysicsMode) {
	p.physicsMode = mode
	spriteMgr.SetPhysicsMode(p.getSpriteId(), int64(mode))
}

func (p *SpriteImpl) PhysicsMode() PhysicsMode {
	return p.physicsMode
}
func (p *SpriteImpl) Velocity() (velocityX, velocityY float64) {
	vel := spriteMgr.GetVelocity(p.getSpriteId())
	return vel.X, vel.Y
}

func (p *SpriteImpl) SetVelocity(velocityX, velocityY float64) {
	spriteMgr.SetVelocity(p.getSpriteId(), mathf.NewVec2(velocityX, velocityY))
}

func (p *SpriteImpl) Gravity() float64 {
	return spriteMgr.GetGravity(p.getSpriteId())
}

func (p *SpriteImpl) SetGravity(gravity float64) {
	spriteMgr.SetGravity(p.getSpriteId(), gravity)
}

// ========== Unified Physics Implementation (Private Methods) ==========

// getPhysicConfig returns the appropriate physicConfig based on trigger flag
func (p *SpriteImpl) getPhysicConfig(isTrigger bool) *physicConfig {
	if isTrigger {
		return &p.triggerInfo
	}
	return &p.collisionInfo
}

// setPhysicShape sets physics shape with validation and synchronization
func (p *SpriteImpl) setPhysicShape(isTrigger bool, ctype ColliderShapeType, params []float64) error {
	config := p.getPhysicConfig(isTrigger)

	// Store original values for rollback if validation fails
	originalType := config.Type
	originalParams := make([]float64, len(config.Params))
	copy(originalParams, config.Params)

	// Temporarily set new values for validation
	config.Type = ctype
	config.Params = make([]float64, len(params))
	copy(config.Params, params)

	// Validate parameters before applying
	if !config.validateShape() {
		// Rollback to original values if validation fails
		config.Type = originalType
		config.Params = originalParams
		return fmt.Errorf("invalid shape parameters for type %d", ctype)
	}

	// Apply shape-specific settings
	p.applyPhysicShape(isTrigger)
	return nil
}

// getPhysicShape returns the current shape type and parameters
func (p *SpriteImpl) getPhysicShape(isTrigger bool) (ColliderShapeType, []float64) {
	config := p.getPhysicConfig(isTrigger)
	params := make([]float64, len(config.Params))
	copy(params, config.Params)
	return config.Type, params
}

// setPhysicPivot sets the pivot point for physics config
func (p *SpriteImpl) setPhysicPivot(isTrigger bool, offsetX, offsetY float64) {
	config := p.getPhysicConfig(isTrigger)
	config.Pivot = mathf.NewVec2(offsetX, offsetY)

	// Re-apply current shape with new pivot if needed
	if p.syncSprite != nil {
		p.applyPhysicShape(isTrigger)
	}
}

// getPhysicPivot returns the current pivot offset
func (p *SpriteImpl) getPhysicPivot(isTrigger bool) (offsetX, offsetY float64) {
	config := p.getPhysicConfig(isTrigger)
	return config.Pivot.X, config.Pivot.Y
}

// applyPhysicShape applies the shape settings to the engine
func (p *SpriteImpl) applyPhysicShape(isTrigger bool) {
	config := p.getPhysicConfig(isTrigger)
	ctype := config.Type
	params := config.Params

	if p.syncSprite != nil {
		switch ctype {
		case RectCollider:
			if len(params) >= 2 {
				p.syncSprite.SetColliderShapeRect(isTrigger, config.Pivot, mathf.NewVec2(params[0], params[1]))
			}
		case CircleCollider:
			if len(params) >= 1 {
				p.syncSprite.SetColliderShapeCircle(isTrigger, config.Pivot, params[0])
			}
		case CapsuleCollider:
			if len(params) >= 2 {
				p.syncSprite.SetColliderShapeCapsule(isTrigger, config.Pivot, mathf.NewVec2(params[0]*2, params[1]))
			}
		case PolygonCollider:
			// TODO: Implement polygon shape setting when available
		}
	}
}

func (p *SpriteImpl) SetColliderShape(isTrigger bool, ctype ColliderShapeType, params []float64) error {
	return p.setPhysicShape(isTrigger, ctype, params)
}

func (p *SpriteImpl) ColliderShape(isTrigger bool) (ColliderShapeType, []float64) {
	return p.getPhysicShape(isTrigger)
}

func (p *SpriteImpl) SetColliderPivot(isTrigger bool, offsetX, offsetY float64) {
	p.setPhysicPivot(isTrigger, offsetX, offsetY)
}

func (p *SpriteImpl) ColliderPivot(isTrigger bool) (offsetX, offsetY float64) {
	return p.getPhysicPivot(isTrigger)
}

func (p *SpriteImpl) SetCollisionLayer(layer int64) {
	p.syncSprite.SetCollisionLayer(layer)
}

func (p *SpriteImpl) SetCollisionMask(mask int64) {
	p.syncSprite.SetCollisionMask(mask)
}

func (p *SpriteImpl) SetCollisionEnabled(enabled bool) {
	p.syncSprite.SetCollisionEnabled(enabled)
}

func (p *SpriteImpl) SetTriggerEnabled(trigger bool) {
	p.syncSprite.SetTriggerEnabled(trigger)
}

func (p *SpriteImpl) SetTriggerLayer(layer int64) {
	p.syncSprite.SetTriggerLayer(layer)
}

func (p *SpriteImpl) SetTriggerMask(mask int64) {
	p.syncSprite.SetTriggerMask(mask)
}

func (p *SpriteImpl) TriggerLayer() int64 {
	return p.syncSprite.GetTriggerLayer()
}
func (p *SpriteImpl) TriggerMask() int64 {
	return p.syncSprite.GetTriggerMask()
}
func (p *SpriteImpl) TriggerEnabled() bool {
	return p.syncSprite.IsTriggerEnabled()
}

func (p *SpriteImpl) CollisionLayer() int64 {
	return p.syncSprite.GetCollisionLayer()
}
func (p *SpriteImpl) CollisionMask() int64 {
	return p.syncSprite.GetCollisionMask()
}

func (p *SpriteImpl) CollisionEnabled() bool {
	return p.syncSprite.IsCollisionEnabled()
}
