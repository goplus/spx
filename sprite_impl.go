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

	spxlog "github.com/goplus/spx/v4/internal/log"
)

// SpriteImpl is the concrete implementation of the Sprite interface.
type SpriteImpl struct {
	baseObj
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

type spriteRuntimeState struct {
	isVisible           bool
	cloned              bool
	isDying             bool
	dirtyVersion        uint64
	proxySyncVersion    uint64
	visualVersion       uint64
	hasOnCloned         bool
	hasOnTouchStart     bool
	defaultCostumeIndex int
}

// -----------------------------------------------------------------------------
// Public API
// -----------------------------------------------------------------------------
func (p *SpriteImpl) InitFrom(src *SpriteImpl) {
	p.baseObj.initFrom(&src.baseObj)
	p.scriptEventBindings.bind(src.scriptEventRegistry, p)

	p.g, p.name, p.runtimeState.scale = src.g, src.name, src.runtimeState.scale
	p.greffUniforms = maps.Clone(src.greffUniforms)

	p.original = src.originalSprite()
	p.spriteState = spriteRuntimeState{
		isVisible:           src.spriteState.isVisible,
		cloned:              true,
		defaultCostumeIndex: p.spriteState.defaultCostumeIndex,
	}
	p.proxyPublication = nil
	p.preparedVisual = preparedProxyVisual{}
}

func (p *SpriteImpl) Die() {
	p.setDying()
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

func (p *SpriteImpl) Name() string {
	return p.name
}

func (p *SpriteImpl) IsCloned() bool {
	return p.spriteState.cloned
}

// -----------------------------------------------------------------------------
// Internal State
// -----------------------------------------------------------------------------
func (p *SpriteImpl) setDying() {
	p.spriteState.isDying = true
}

func (p *SpriteImpl) markProxyDirty() {
	p.markVisualDirty()
	p.spriteState.dirtyVersion++
}

func (p *SpriteImpl) markVisualDirty() {
	p.requestRedrawIfVisible()
	p.spriteState.visualVersion++
}

// -----------------------------------------------------------------------------
// Components
// -----------------------------------------------------------------------------
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

// -----------------------------------------------------------------------------
// Lifecycle Helpers
// -----------------------------------------------------------------------------

func (p *SpriteImpl) playStateAnimationAndWait(stateName string) {
	animName := p.animation().getStateAnimName(stateName)
	if animName == "" || !p.animation().hasAnim(animName) {
		return
	}
	p.AnimateAndWait(animName)
}

// destroy releases resources without stopping scripts.
func (p *SpriteImpl) destroy() {
	if p.isDestroyed() {
		return
	}
	p.setVisible(false)
	p.clearHandlers()
	p.components.destroyComponents()
	p.g.shapeMgr.removeShape(p)
	if syncSprite := p.runtimeState.syncSprite; syncSprite != nil {
		p.g.inputMgr.clickGate.Remove(syncSprite.GetId())
	}
	p.markDestroyed()
}

func (p *SpriteImpl) stopIfCurrentCoroutine() {
	if gco.IsInCoroutine() {
		current := gco.Current()
		if current != nil && p == current.Owner {
			gco.StopCurrent()
		}
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------
// originalSprite returns the original instance, including for clones of clones.
// A configured stage instance starts its own family even if built from a template.
func (p *SpriteImpl) originalSprite() *SpriteImpl {
	if p.IsCloned() && p.original != nil {
		return p.original
	}
	return p
}
