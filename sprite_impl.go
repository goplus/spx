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
	"log"
	"maps"
	"reflect"
	"slices"

	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
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

	// State flags
	isVisible bool
	isCloned  bool
	isDying   bool
	isDirty   bool // marks if transform or visibility has changed

	// Event flags
	hasOnCloned     bool
	hasOnTouchStart bool
	hasOnTouching   bool
	hasOnTouchEnd   bool

	// Internal state
	gamer               reflect.Value
	defaultCostumeIndex int

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
	return p.isCloned
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
	p.scale = spriteCfg.Size
	p.isVisible = spriteCfg.Visible
}

// initEngineObjects initializes engine-related objects
func (p *SpriteImpl) initEngineObjects() {
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

func (p *SpriteImpl) InitFrom(src *SpriteImpl) {
	p.baseObj.initFrom(&src.baseObj)
	p.eventSinks.initFrom(&src.eventSinks, p)

	p.g, p.name, p.scale = src.g, src.name, src.scale
	p.greffUniforms = maps.Clone(src.greffUniforms)

	p.isVisible = src.isVisible
	p.isCloned = true
	p.isDying = false

	p.hasOnCloned = false
	p.hasOnTouchStart = false
	p.hasOnTouching = false
	p.hasOnTouchEnd = false
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
	engine.WaitMainThread(func() {
		dest.syncCheckInitProxy()
		syncCheckUpdateCostume(&dest.baseObj)
	})
	return dest
}

func applySpriteProps(dest *SpriteImpl, v specsp) {
	transform := dest.transform()
	if x, ok := v["x"]; ok {
		transform.x = x.(float64)
	}
	if y, ok := v["y"]; ok {
		transform.y = y.(float64)
	}
	if heading, ok := v["heading"]; ok {
		transform.direction = heading.(float64)
	}
	if style, ok := v["rotationStyle"]; ok {
		transform.rotationStyle = toRotationStyle(style.(string))
	}
	if visible, ok := v["visible"]; ok {
		dest.isVisible = visible.(bool)
	}
	if size, ok := v["size"]; ok {
		dest.scale = size.(float64)
	}
	if idx, ok := v["costumeIndex"]; ok {
		dest.setCustumeIndex(int(idx.(float64)))
	}
	dest.isCloned = false
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

func (p *SpriteImpl) addTouchStartHandler(onTouchStart func(Sprite)) {
	p.hasOnTouchStart = true
	p.allWhenTouchStart = append(p.allWhenTouchStart, eventSink{
		pthis: p,
		sink:  onTouchStart,
		cond: func(data any) bool {
			return data == p
		},
	})
}

func (p *SpriteImpl) OnTouchStart__0(sprite SpriteName, onTouchStart func(Sprite)) {
	p.physics().addCollisionTarget(sprite)
	p.addTouchStartHandler(func(s Sprite) {
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
		p.physics().addCollisionTarget(sprite)
	}
	p.addTouchStartHandler(func(s Sprite) {
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
	p.Hide()
	p.doDeleteClone()
	p.components.destroyComponents()
	p.g.removeShape(p)
	p.Stop(ThisSprite)
	if p == gco.Current().Obj {
		gco.Abort()
	}
	p.HasDestroyed = true
}

// DeleteThisClone deletes only cloned sprite, no effect on prototype sprite.
// Add this interface to match Scratch.
func (p *SpriteImpl) DeleteThisClone() {
	if !p.isCloned {
		return
	}

	p.Destroy()
}

// ============================================================================
// Touching and Collision Detection Methods
// ============================================================================

func (p *SpriteImpl) TouchingColor(color Color) bool {
	return p.touchingColor(toMathfColor(color))
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

// -----------------------------------------------------------------------------
// Component Accessor Helpers
// -----------------------------------------------------------------------------

func (p *SpriteImpl) transform() *transformComponent {
	return p.components.Transform()
}

func (p *SpriteImpl) animation() *animationComponent {
	return p.components.Animation()
}

func (p *SpriteImpl) physics() *physicsComponent {
	return p.components.Physics()
}

func (p *SpriteImpl) pen() *penComponent {
	return p.components.Pen()
}

func (p *SpriteImpl) sound() *soundComponent {
	return p.components.Sound()
}
