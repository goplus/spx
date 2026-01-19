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
	"slices"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/time"
)

// ============================================================================
// SpriteImpl Structure
// ============================================================================

// SpriteImpl is the concrete implementation of the Sprite interface
type SpriteImpl struct {
	baseObj
	eventSinks
	g      *Game
	sprite Sprite
	name   string

	// TODO(refactor): The following fields duplicate data in components.
	// These should be removed in the future, with all access going through components.
	// Currently kept for backward compatibility and initialization flow.
	// See: https://github.com/goplus/spx/issues/1157 for migration plan

	// Position and orientation
	// TODO(refactor): Duplicated in transformComponent - migrate to use components.Transform()
	x, y          float64
	direction     float64
	rotationStyle RotationStyle
	pivot         mathf.Vec2

	// Visual components
	sayObj   *sayOrThinker
	quoteObj *quoter

	// Animation configuration
	// TODO(refactor): Duplicated in animationComponent - migrate to use components.Animation()
	animations        map[SpriteAnimationName]*aniConfig
	animBindings      map[string]string
	defaultAnimation  SpriteAnimationName
	animationWrappers map[SpriteAnimationName]*animationWrapper // lazy load

	// Pen properties
	// TODO(refactor): Duplicated in penComponent - migrate to use components.Pen()
	penColor        mathf.Color
	penWidth        float64
	penHue          float64
	penSaturation   float64
	penBrightness   float64
	penTransparency float64

	// State flags
	isVisible bool
	isCloned_ bool
	isDying   bool
	isDirty   bool // marks if transform or visibility has changed

	// Event flags
	hasOnCloned     bool
	hasOnTouchStart bool
	hasOnTouching   bool
	hasOnTouchEnd   bool

	// Internal state
	gamer               reflect.Value
	curAnimState        *animState
	curTweenState       *animState
	defaultCostumeIndex int

	// Physics configuration
	// TODO(refactor): Duplicated in physicsComponent - migrate to use components.Physics()
	triggerInfo   physicConfig
	collisionInfo physicConfig

	// Engine objects
	// TODO(refactor): penObj is duplicated in penComponent
	penObj   *engine.Object
	soundObj engine.Object

	// Runtime data
	collisionTargets map[string]bool
	pendingAudios    []string
	donedAnimations  []string

	// Physics properties
	// TODO(refactor): Duplicated in physicsComponent - migrate to use components.Physics()
	physicsMode PhysicsMode
	mass        float64
	friction    float64
	airDrag     float64
	gravity     float64

	// Component system
	components spriteComponents
}

// ============================================================================
// Core Methods
// ============================================================================

func (p *SpriteImpl) Name() string {
	return p.name
}

func (p *SpriteImpl) IsCloned() bool {
	return p.isCloned_
}

func (p *SpriteImpl) setDying() { // dying: visible but can't be touched
	p.isDying = true
}

func (p *SpriteImpl) getAllShapes() []Shape {
	return p.g.getAllShapes()
}

// ============================================================================
// Initialization Methods
// ============================================================================

func (p *SpriteImpl) init(
	base string, g *Game, name string, spriteCfg *spriteConfig, gamer reflect.Value, sprite Sprite) {
	p.initBaseObjects(base, spriteCfg, g)
	p.initBasicProperties(g, name, sprite, gamer, spriteCfg)
	p.initPhysicsConfig(spriteCfg)
	p.initPhysicsProperties(spriteCfg)
	p.initComponents(spriteCfg) // Components will copy from sprite fields and init from config
	p.initEngineObjects()
}

// initBaseObjects initializes the base object and event sinks
func (p *SpriteImpl) initBaseObjects(base string, spriteCfg *spriteConfig, g *Game) {
	if spriteCfg.Costumes != nil {
		p.baseObj.init(base, spriteCfg.Costumes, spriteCfg.getCostumeIndex())
	} else {
		p.baseObj.initWith(base, spriteCfg)
	}
	p.defaultCostumeIndex = p.baseObj.costumeIndex_
	p.eventSinks.init(&g.sinkMgr, p)
}

// initBasicProperties initializes basic sprite properties like position, direction, visibility
func (p *SpriteImpl) initBasicProperties(g *Game, name string, sprite Sprite, gamer reflect.Value, spriteCfg *spriteConfig) {
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
}

// initPhysicsConfig initializes collision and trigger configurations
func (p *SpriteImpl) initPhysicsConfig(spriteCfg *spriteConfig) {
	p.initCollisionConfig(spriteCfg)
	p.initTriggerConfig(spriteCfg)
}

// initCollisionConfig initializes collision configuration
func (p *SpriteImpl) initCollisionConfig(spriteCfg *spriteConfig) {
	p.collisionInfo.Mask = parseLayerMaskValue(spriteCfg.CollisionMask)
	p.collisionInfo.Layer = parseLayerMaskValue(spriteCfg.CollisionLayer)

	// collider is disable by default
	var defaultCollisionType int64 = physicsColliderNone
	if enabledPhysics {
		defaultCollisionType = physicsColliderAuto
	}

	p.collisionInfo.Type = parseColliderShapeType(spriteCfg.CollisionShapeType, defaultCollisionType)
	p.collisionInfo.Pivot = spriteCfg.CollisionPivot
	p.collisionInfo.Params = spriteCfg.CollisionShapeParams

	// Validate colliderShapeType and colliderShape length matching
	if !p.collisionInfo.validateShape() {
		spxlog.Warn("Invalid collider configuration for sprite %s, using default values", p.name)
		p.collisionInfo.Type = physicsColliderNone
		p.collisionInfo.Params = nil
	}
}

// initTriggerConfig initializes trigger configuration
func (p *SpriteImpl) initTriggerConfig(spriteCfg *spriteConfig) {
	p.triggerInfo.Mask = parseLayerMaskValue(spriteCfg.TriggerMask)
	p.triggerInfo.Layer = parseLayerMaskValue(spriteCfg.TriggerLayer)
	p.triggerInfo.Type = parseColliderShapeType(spriteCfg.TriggerShapeType, physicsColliderAuto)
	p.triggerInfo.Pivot = spriteCfg.TriggerPivot
	p.triggerInfo.Params = spriteCfg.TriggerShapeParams

	// Validate triggerType and triggerShape length matching
	if !p.triggerInfo.validateShape() {
		spxlog.Warn("Invalid trigger configuration for sprite %s, using default values", p.name)
		p.triggerInfo.Type = physicsColliderAuto
		p.triggerInfo.Params = nil
	}
}

// initPhysicsProperties initializes physics properties like mass, friction, gravity
func (p *SpriteImpl) initPhysicsProperties(spriteCfg *spriteConfig) {
	p.physicsMode = toPhysicsMode(spriteCfg.PhysicsMode)
	p.airDrag = parseDefaultFloatValue(spriteCfg.AirDrag, 1)
	p.gravity = parseDefaultFloatValue(spriteCfg.Gravity, 1)
	p.friction = parseDefaultFloatValue(spriteCfg.Friction, 1)
	p.mass = parseDefaultFloatValue(spriteCfg.Mass, 1)
}

// initAnimations initializes sprite animations and animation wrappers
func (p *SpriteImpl) initAnimations(spriteCfg *spriteConfig) {
	p.defaultAnimation = spriteCfg.DefaultAnimation
	p.animations = make(map[string]*aniConfig)
	anims := spriteCfg.FAnimations

	for key, val := range anims {
		var ani = val
		_, ok := p.animations[key]
		if ok {
			log.Panicf("animation key [%s] is exist", key)
		}

		// Set default values
		if ani.FrameFps == 0 {
			ani.FrameFps = 25
		}
		if ani.TurnToDuration == 0 {
			ani.TurnToDuration = 1
		}
		if ani.StepDuration == 0 {
			ani.StepDuration = 0.01
		}

		// Calculate frame ranges and duration
		from, to := p.getFromAnToForAniFrames(ani.FrameFrom, ani.FrameTo)
		ani.IFrameFrom, ani.IFrameTo = int(from), int(to)
		ani.Speed = 1
		ani.Duration = (math.Abs(float64(ani.IFrameFrom-ani.IFrameTo)) + 1) / float64(ani.FrameFps)
		p.animations[key] = ani
	}

	// Lazy register animations to engine
	p.animationWrappers = make(map[SpriteAnimationName]*animationWrapper)
	for animName, ani := range p.animations {
		p.animationWrappers[animName] = &animationWrapper{spr: p, ani: ani}
	}
}

// initEngineObjects initializes engine-related objects
func (p *SpriteImpl) initEngineObjects() {
	p.pendingAudios = make([]string, 0)
	p.syncSprite = nil
	engine.WaitMainThread(func() {
		p.syncCheckInitProxy()
	})
}

// initComponents initializes all sprite components
func (p *SpriteImpl) initComponents(spriteCfg *spriteConfig) {
	p.components.initComponents(p, spriteCfg)
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

	// NOTE: Transform-related fields (x, y, direction, rotationStyle, scale, pivot)
	// are now handled by components.cloneFrom(), not here

	p.sayObj = nil

	// NOTE: Animation-related fields (animations, animationWrappers, defaultAnimation)
	// are now handled by components.cloneFrom(), not here

	// clone effect params
	p.greffUniforms = maps.Clone(src.greffUniforms)

	// NOTE: Pen-related fields (penColor, penHue, penWidth, isPenDown, etc.)
	// are now handled by components.cloneFrom(), not here

	p.isVisible = src.isVisible
	p.isCloned_ = true
	p.isDying = false

	p.hasOnCloned = false
	p.hasOnTouchStart = false
	p.hasOnTouching = false
	p.hasOnTouchEnd = false

	// NOTE: Physics-related fields (collisionInfo, triggerInfo, physicsMode, etc.)
	// are now handled by components.cloneFrom(), not here

	p.pendingAudios = make([]string, 0)
}

// ============================================================================
// Clone Methods
// ============================================================================

func Gopt_SpriteImpl_Clone__0(sprite Sprite) {
	Gopt_SpriteImpl_Clone__1(sprite, nil)
}

func Gopt_SpriteImpl_Clone__1(sprite Sprite, data any) {
	doClone(sprite, data, false, nil)
}

func doClone(sprite Sprite, data any, isAsync bool, onCloned func(sprite *SpriteImpl)) {
	if sprite == nil {
		log.Panicln("doClone, sprite is nil")
	}
	src := spriteOf(sprite)
	if debugInstr {
		spxlog.Debug("Clone: %s", src.name)
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

	// Clone components from source sprite
	src := spriteOf(in.Addr().Interface().(Sprite))
	dest.components.cloneFrom(&src.components, dest)

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

// ============================================================================
// Event Handler Methods
// ============================================================================

func (p *SpriteImpl) OnCloned__0(onCloned func(data any)) {
	p.hasOnCloned = true
	p.allWhenCloned = append(p.allWhenCloned, eventSink{
		pthis: p,
		sink:  onCloned,
		cond: func(data any) bool {
			return data == p
		},
	})
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
	p.allWhenTouchStart = append(p.allWhenTouchStart, eventSink{
		pthis: p,
		sink:  onTouchStart,
		cond: func(data any) bool {
			return data == p
		},
	})
}

func (p *SpriteImpl) onTouchStart__0(onTouchStart func(Sprite)) {
	for name := range p.g.sprs {
		p.collisionTargets[name] = true
	}
	p._onTouchStart(onTouchStart)
}

func (p *SpriteImpl) onTouchStart__1(onTouchStart func()) {
	p.onTouchStart__0(func(Sprite) {
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
		if impl != nil && slices.Contains(sprites, impl.name) {
			onTouchStart(s)
		}
	})
}

func (p *SpriteImpl) OnTouchStart__3(sprites []SpriteName, onTouchStart func()) {
	p.OnTouchStart__2(sprites, func(Sprite) {
		onTouchStart()
	})
}

// ============================================================================
// Lifecycle Methods
// ============================================================================

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
		spxlog.Debug("Destroy: %s", p.name)
	}

	p.syncSprite.UnRegisterOnAnimationFinished()

	p.Hide()
	p.doDeleteClone()
	p.components.destroyComponents()
	p.g.removeShape(p)
	p.Stop(ThisSprite)
	if p == gco.Current().Obj {
		gco.Abort()
	}
	p.HasDestroyed = true

	if p.soundObj != 0 {
		p.g.sounds.releaseSound(p.soundObj)
		p.soundObj = 0
	}
}

// DeleteThisClone deletes only cloned sprite, no effect on prototype sprite.
// Add this interface to match Scratch.
func (p *SpriteImpl) DeleteThisClone() {
	if !p.isCloned_ {
		return
	}

	p.Destroy()
}

// ============================================================================
// Visibility Methods
// ============================================================================

func (p *SpriteImpl) Hide() {
	if debugInstr {
		spxlog.Debug("Hide: %s", p.name)
	}

	p.doStopSay()
	if p.isVisible {
		p.isVisible = false
		p.isDirty = true
	}
}

func (p *SpriteImpl) Show() {
	if debugInstr {
		spxlog.Debug("Show: %s", p.name)
	}
	if !p.isVisible {
		p.isVisible = true
		p.isDirty = true
	}
}

func (p *SpriteImpl) Visible() bool {
	return p.isVisible
}

// ============================================================================
// Costume Methods
// ============================================================================

func (p *SpriteImpl) CostumeName() SpriteCostumeName {
	return p.getCostumeName()
}

func (p *SpriteImpl) CostumeIndex() int {
	return p.getCostumeIndex()
}

// SetCostume sets the costume by:
//   - costume name
//   - index (float64 or int)
//   - spx.Next or spx.Prev
func (p *SpriteImpl) setCostume(costume any) {
	if debugInstr {
		spxlog.Debug("SetCostume: sprite=%s, costume=%v", p.name, costume)
	}
	p.goSetCostume(costume)
	p.defaultCostumeIndex = p.costumeIndex_
	p.isDirty = true
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

// ============================================================================
// Communication Methods
// ============================================================================

func (p *SpriteImpl) Ask(msg any) {
	if debugInstr {
		spxlog.Debug("Ask: sprite=%s, msg=%v", p.name, msg)
	}
	msgStr, ok := msg.(string)
	if !ok {
		msgStr = fmt.Sprint(msg)
	}
	if msgStr == "" {
		spxlog.Warn("ask: msg should not be empty")
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
		spxlog.Debug("Say: sprite=%s, msg=%v, secs=%v", p.name, msg, secs)
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
		spxlog.Debug("Think: sprite=%s, msg=%v, secs=%v", p.name, msg, secs)
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
		spxlog.Debug("Quote: sprite=%s, message=%s, description=%s, secs=%v", p.name, message, description, secs)
	}
	p.quote_(message, description)
	if secs > 0 {
		p.waitStopQuote(secs)
	}
}

// ============================================================================
// Graphic Effects Methods
// ============================================================================

func (p *SpriteImpl) SetGraphicEffect(kind EffectKind, val float64) {
	p.baseObj.setGraphicEffect(kind, val)
}

func (p *SpriteImpl) ChangeGraphicEffect(kind EffectKind, delta float64) {
	p.baseObj.changeGraphicEffect(kind, delta)
}

func (p *SpriteImpl) ClearGraphicEffects() {
	p.baseObj.clearGraphicEffects()
}

// ============================================================================
// Touching and Collision Detection Methods
// ============================================================================

func (p *SpriteImpl) TouchingColor(color Color) bool {
	return p.touchingColor(toMathfColor(color))
}

// Touching checks if sprite is touching:
//   - another sprite (by name or Sprite object)
//   - spx.Mouse
//   - spx.Edge, spx.EdgeLeft, spx.EdgeTop, spx.EdgeRight, spx.EdgeBottom
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

func (p *SpriteImpl) BounceOffEdge() {
	if debugInstr {
		spxlog.Debug("BounceOffEdge: %s", p.name)
	}

	nearestEdge := p.checkNearestTouchedBoundary()

	if nearestEdge == 0 {
		return
	}

	// prevents sprites from getting stuck at boundaries
	const minBounceComponent = 0.2
	radians := toRadian(90 - p.direction)
	dx := math.Cos(radians)
	dy := -math.Sin(radians)

	switch nearestEdge {
	case touchingScreenLeft:
		dx = math.Max(minBounceComponent, math.Abs(dx))
	case touchingScreenTop:
		dy = math.Max(minBounceComponent, math.Abs(dy))
	case touchingScreenRight:
		dx = -math.Max(minBounceComponent, math.Abs(dx))
	case touchingScreenBottom:
		dy = -math.Max(minBounceComponent, math.Abs(dy))
	}

	newDirection := engine.RadToDeg(math.Atan2(dy, dx)) + 90
	p.direction = normalizeDirection(newDirection)
}

// ============================================================================
// Layer Methods
// ============================================================================

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

// ============================================================================
// Monitor and Variable Display Methods
// ============================================================================

func (p *SpriteImpl) HideVar(name string) {
	p.g.setStageMonitor(p.name, getVarPrefix+name, false)
}

func (p *SpriteImpl) ShowVar(name string) {
	p.g.setStageMonitor(p.name, getVarPrefix+name, true)
}

// ============================================================================
// Update Methods
// ============================================================================

func (pself *SpriteImpl) onUpdate(delta float64) {
	if pself.quoteObj != nil {
		pself.quoteObj.refresh()
	}
	if pself.sayObj != nil {
		pself.sayObj.refresh()
	}
}

// ============================================================================
// Time Methods
// ============================================================================

func (pself *SpriteImpl) DeltaTime() float64 {
	return time.DeltaTime()
}

func (pself *SpriteImpl) TimeSinceLevelLoad() float64 {
	return time.TimeSinceLevelLoad()
}
