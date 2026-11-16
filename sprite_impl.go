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
	"maps"

	spxlog "github.com/goplus/spx/v4/internal/base/log"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
)

// SpriteImpl is the concrete implementation of the Sprite interface.
type SpriteImpl struct {
	visualObject
	scriptEventBindings

	sprite      Sprite
	original    *SpriteImpl
	spriteState spriteRuntimeState
	// proxyPublication points to fresh clone-local state. The pointer itself
	// remains stable after clone construction so reflective cloning never copies
	// publication state while it is in use.
	proxyPublication *cloneProxyPublication
	preparedVisual   preparedProxyVisual
	name             string
	components       spriteComponents

	g *Game
}

// spriteComponents holds all components attached to a sprite.
type spriteComponents struct {
	transform *transformComponent
	animation *animationComponent
	physics   *physicsComponent
	pen       *penComponent
	sound     *soundComponent
	bubble    *bubbleComponent // Optional: only allocated when Say/Think/Quote is used
}

type spriteRuntimeState struct {
	isVisible           bool
	isDraggable         bool
	cloned              bool
	isDying             bool
	dirtyVersion        uint64
	proxySyncVersion    uint64
	visualVersion       uint64
	hasOnCloned         bool
	hasOnTouchStart     bool
	defaultCostumeIndex int
}

// Public API.
func (p *SpriteImpl) InitFrom(src *SpriteImpl) {
	p.visualObject.initFrom(&src.visualObject)
	p.scriptEventBindings.bind(src.scriptEventRegistry, p)

	p.g, p.name, p.runtimeState.scale = src.g, src.name, src.runtimeState.scale
	p.graphicEffects = maps.Clone(src.graphicEffects)

	p.original = src.originalSprite()
	p.spriteState = spriteRuntimeState{
		isVisible:           src.spriteState.isVisible,
		isDraggable:         src.spriteState.isDraggable,
		cloned:              true,
		defaultCostumeIndex: src.spriteState.defaultCostumeIndex,
	}
	p.proxyPublication = nil
	p.preparedVisual = preparedProxyVisual{}
}

func (p *SpriteImpl) Die() {
	p.spriteState.isDying = true
	p.Stop(OtherScriptsInSprite)
	p.playStateAnimationAndWait(StateDie)
	p.Destroy()
}

func (p *SpriteImpl) Destroy() {
	if isDebugInstrEnabled() {
		spxlog.Debug("Destroy: %s", p.name)
	}
	p.destroy()
	p.Stop(ThisSprite)
	p.stopIfCurrentCoroutine()
}

func (p *SpriteImpl) DeleteThisClone() {
	if !p.spriteState.cloned {
		return
	}
	p.Destroy()
}

// SetDragMode enables left-button dragging for Draggable.
// Any other value disables dragging and cancels the current gesture.
func (p *SpriteImpl) SetDragMode(mode DragMode) {
	p.spriteState.isDraggable = mode == Draggable
	if !p.spriteState.isDraggable && p.g != nil {
		p.g.inputMgr.cancelDrag(p)
	}
}

func (p *SpriteImpl) Name() string {
	return p.name
}

func (p *SpriteImpl) IsCloned() bool {
	return p.spriteState.cloned
}

// destroy releases resources without stopping scripts.
func (p *SpriteImpl) destroy() {
	if p.isDestroyed() {
		return
	}
	p.setVisible(false)
	p.clearHandlers()
	p.components.destroyComponents()
	p.g.scene.removeShape(p)
	if proxy := p.runtimeState.proxy; proxy != nil {
		p.g.inputMgr.clickGate.Remove(proxy.GetId())
	}
	p.markDestroyed()
}

func (p *SpriteImpl) isDragging() bool {
	return p.g != nil && p.g.inputMgr.drag.target == p && p.g.inputMgr.drag.active
}

func (p *SpriteImpl) markProxyDirty() {
	p.markVisualDirty()
	p.spriteState.dirtyVersion++
	if p.g != nil {
		p.g.scene.noteProxyDirty(p)
	}
}

func (p *SpriteImpl) markVisualDirty() {
	p.requestRedrawIfVisible()
	p.spriteState.visualVersion++
}

func (p *SpriteImpl) setLayer(layer int) {
	previous := p.runtimeState.layer
	p.visualObject.setLayer(layer)
	if previous != layer && p.g != nil {
		// Layer changes bypass transform versions but affect scene color compositing.
		p.g.scene.noteProxyDirty(p)
	}
}

// Components.
func (p *SpriteImpl) transform() *transformComponent {
	return p.components.transform
}

func (p *SpriteImpl) animation() *animationComponent {
	return p.components.animation
}

func (p *SpriteImpl) physics() *physicsComponent {
	return p.components.physics
}

func (p *SpriteImpl) pen() *penComponent {
	return p.components.pen
}

func (p *SpriteImpl) sound() *soundComponent {
	return p.components.sound
}

func (p *SpriteImpl) bubble() *bubbleComponent {
	if p.components.bubble == nil {
		p.components.bubble = &bubbleComponent{sprite: p}
	}
	return p.components.bubble
}

func (p *SpriteImpl) playStateAnimationAndWait(stateName string) {
	animName := p.animation().getStateAnimName(stateName)
	if animName == "" || !p.animation().hasAnim(animName) {
		return
	}
	p.AnimateAndWait(animName)
}

func (p *SpriteImpl) stopIfCurrentCoroutine() {
	if scheduler.IsInCoroutine() {
		current := scheduler.Current()
		if current != nil && p == current.Owner {
			scheduler.StopCurrent()
		}
	}
}

// Helpers.
// originalSprite returns the original instance, including for clones of clones.
// A configured stage instance starts its own family even if built from a template.
func (p *SpriteImpl) originalSprite() *SpriteImpl {
	if p.IsCloned() && p.original != nil {
		return p.original
	}
	return p
}

func (sc *spriteComponents) initComponents(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	sc.transform = &transformComponent{}
	sc.transform.initialize(sprite, spriteCfg)

	sc.animation = &animationComponent{}
	sc.animation.initialize(sprite, spriteCfg)

	sc.physics = &physicsComponent{}
	sc.physics.initialize(sprite, spriteCfg)

	sc.pen = &penComponent{}
	sc.pen.initialize(sprite)

	sc.sound = &soundComponent{sprite: sprite}
}

func (sc *spriteComponents) destroyComponents() {
	if sc.animation != nil {
		sc.animation.onDestroy()
	}
	if sc.pen != nil {
		sc.pen.onDestroy()
	}
	if sc.sound != nil {
		sc.sound.onDestroy()
	}
	if sc.bubble != nil {
		sc.bubble.onDestroy()
	}
}

// cloneComponents creates independent eager components for a cloned sprite.
// Bubble state is intentionally omitted and remains lazy.
func cloneComponents(source *SpriteImpl, newSprite *SpriteImpl) spriteComponents {
	// Pen and sound inherit the original instance's current state across generations.
	original := source.originalSprite()
	return spriteComponents{
		transform: source.components.transform.cloneFor(newSprite),
		animation: source.components.animation.cloneFor(newSprite),
		physics:   source.components.physics.cloneFor(newSprite),
		pen:       original.components.pen.cloneFor(newSprite),
		sound:     original.components.sound.cloneFor(newSprite),
	}
}
