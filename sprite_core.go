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
	"reflect"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	corestate "github.com/goplus/spx/v3/internal/core/state"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

// SpriteImpl is the concrete implementation of the Sprite interface.
type SpriteImpl struct {
	baseObj
	scriptEventBindings

	sprite      Sprite
	spriteState corestate.SpriteRuntimeState
	name        string
	components  spriteComponents

	g     *Game
	gamer reflect.Value
}

// -----------------------------------------------------------------------------
// Basic State
// -----------------------------------------------------------------------------
func (p *SpriteImpl) Name() string {
	return p.name
}

func (p *SpriteImpl) IsCloned() bool {
	return p.spriteState.Cloned
}

func (p *SpriteImpl) setDying() {
	p.spriteState.IsDying = true
}

func (p *SpriteImpl) markProxyDirty() {
	p.spriteState.DirtyVersion++
	p.spriteState.IsDirty = true
}

// -----------------------------------------------------------------------------
// Initialization
// -----------------------------------------------------------------------------
func (p *SpriteImpl) init(
	g *Game, name string, spriteCfg *coreproject.SpriteConfig, gamer reflect.Value, sprite Sprite) {
	p.initBaseObjects(spriteCfg, g)
	p.initBasicProperties(g, name, sprite, gamer, spriteCfg)
	p.initComponents(spriteCfg)
	p.initRuntimeProxy()
}

func (p *SpriteImpl) initBaseObjects(spriteCfg *coreproject.SpriteConfig, g *Game) {
	if spriteCfg.Costumes != nil {
		p.baseObj.init(spriteCfg.Costumes, spriteCfg.GetCostumeIndex())
	} else {
		p.baseObj.initWith(spriteCfg)
	}
	p.spriteState.DefaultCostumeIndex = p.baseObj.costumeIndex
	p.scriptEventBindings.init(&g.scriptEvents, p)
}

func (p *SpriteImpl) initBasicProperties(g *Game, name string, sprite Sprite, gamer reflect.Value, spriteCfg *coreproject.SpriteConfig) {
	p.gamer = gamer
	p.g, p.name, p.sprite = g, name, sprite
	p.runtimeState.Scale = spriteCfg.Size
	p.spriteState.IsVisible = spriteCfg.Visible
	p.spriteState.IsAwakened = false
}

func (p *SpriteImpl) initComponents(spriteCfg *coreproject.SpriteConfig) {
	p.components.initComponents(p, spriteCfg)
}

func (p *SpriteImpl) InitFrom(src *SpriteImpl) {
	p.baseObj.initFrom(&src.baseObj)
	p.scriptEventBindings.initFrom(&src.scriptEventBindings, p)

	p.g, p.name, p.runtimeState.Scale = src.g, src.name, src.runtimeState.Scale
	p.greffUniforms = maps.Clone(src.greffUniforms)

	p.spriteState.IsVisible = src.spriteState.IsVisible
	p.spriteState.Cloned = true
	p.spriteState.IsDying = false
	p.spriteState.IsAwakened = false

	p.spriteState.DirtyVersion = 0
	p.spriteState.ProxySyncVersion = 0
	p.spriteState.HasOnCloned = false
	p.spriteState.HasOnTouchStart = false
	p.spriteState.HasOnTouching = false
	p.spriteState.HasOnTouchEnd = false
}

// -----------------------------------------------------------------------------
// Components
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

// -----------------------------------------------------------------------------
// Lifecycle
// -----------------------------------------------------------------------------
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
	p.teardown()
	p.Stop(ThisSprite)
	p.markDestroyed()
	p.abortIfCurrentCoroutine()
}

func (p *SpriteImpl) DeleteThisClone() {
	if !p.spriteState.Cloned {
		return
	}
	p.Destroy()
}

func (p *SpriteImpl) playStateAnimationAndWait(stateName string) {
	animName := p.getStateAnimName(stateName)
	if animName == "" || !p.hasAnim(animName) {
		return
	}
	p.AnimateAndWait(animName)
}

func (p *SpriteImpl) teardown() {
	if bubble := p.components.bubble; bubble != nil {
		bubble.stopAll()
	}
	p.setVisible(false)
	p.doDeleteClone()
	p.components.destroyComponents()
	p.g.removeShape(p)
	if syncSprite := p.runtimeState.SyncSprite; syncSprite != nil {
		p.g.inputMgr.removeClickTarget(syncSprite.GetId())
	}
}

func (p *SpriteImpl) abortIfCurrentCoroutine() {
	if current := gco.Current(); current != nil && p == current.Obj {
		gco.Abort()
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------
func spriteOf(sprite Sprite) *SpriteImpl {
	vSpr := reflect.ValueOf(sprite)
	if vSpr.Kind() == reflect.Pointer {
		vSpr = vSpr.Elem()
	}
	if vSpr.Kind() != reflect.Struct {
		return nil
	}
	for i, n := 0, vSpr.NumField(); i < n; i++ {
		fld := vSpr.Field(i)
		if fld.Kind() == reflect.Struct && fld.Type() == reflect.TypeOf(SpriteImpl{}) {
			return fld.Addr().Interface().(*SpriteImpl)
		}
	}
	return nil
}
