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

	coreproject "github.com/goplus/spx/v2/internal/core/project"
	corestate "github.com/goplus/spx/v2/internal/core/state"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/time"
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

// -----------------------------------------------------------------------------
// Initialization
// -----------------------------------------------------------------------------
func (p *SpriteImpl) init(
	g *Game, name string, spriteCfg *coreproject.SpriteConfig, gamer reflect.Value, sprite Sprite) {
	p.initBaseObjects(spriteCfg, g)
	p.initBasicProperties(g, name, sprite, gamer, spriteCfg)
	p.initComponents(spriteCfg)
	p.initEngineObjects()
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
	aniName := p.getStateAnimName(StateDie)
	p.setDying()

	p.Stop(OtherScriptsInSprite)
	if p.hasAnim(aniName) {
		p.AnimateAndWait(aniName)
	}
	p.Destroy()
}

func (p *SpriteImpl) Destroy() {
	if isDebugInstrEnabled() {
		spxlog.Debug("Destroy: %s", p.name)
	}
	p.Hide()
	p.doDeleteClone()
	if p.runtimeState.SyncSprite != nil {
		p.g.inputMgr.removeClickTarget(p.runtimeState.SyncSprite.GetId())
	}
	p.components.destroyComponents()
	p.g.removeShape(p)
	p.Stop(ThisSprite)
	p.markDestroyed()
	if p == gco.Current().Obj {
		gco.Abort()
	}
}

func (p *SpriteImpl) DeleteThisClone() {
	if !p.spriteState.Cloned {
		return
	}
	p.Destroy()
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

// -----------------------------------------------------------------------------
// Timing
// -----------------------------------------------------------------------------
func (p *SpriteImpl) DeltaTime() float64 {
	return time.DeltaTime()
}

func (p *SpriteImpl) TimeSinceLevelLoad() float64 {
	return time.TimeSinceLevelLoad()
}
