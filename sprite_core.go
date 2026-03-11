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
	"github.com/goplus/spx/v2/internal/engine"
)

// SpriteImpl is the concrete implementation of the Sprite interface.
type SpriteImpl struct {
	baseObj
	spriteState corestate.SpriteRuntimeState
	scriptEventBindings
	g      *Game
	sprite Sprite
	name   string

	gamer reflect.Value

	components spriteComponents
}

func (p *SpriteImpl) Name() string {
	return p.name
}

func (p *SpriteImpl) IsCloned() bool {
	return p.spriteState.Cloned
}

func (p *SpriteImpl) setDying() {
	p.spriteState.IsDying = true
}

func (p *SpriteImpl) getAllShapes() []Shape {
	return p.g.getAllShapes()
}

func (p *SpriteImpl) init(
	base string, g *Game, name string, spriteCfg *coreproject.SpriteConfig, gamer reflect.Value, sprite Sprite) {
	p.initBaseObjects(base, spriteCfg, g)
	p.initBasicProperties(g, name, sprite, gamer, spriteCfg)
	p.initComponents(spriteCfg)
	p.initEngineObjects()
}

func (p *SpriteImpl) initBaseObjects(base string, spriteCfg *coreproject.SpriteConfig, g *Game) {
	if spriteCfg.Costumes != nil {
		p.baseObj.init(base, spriteCfg.Costumes, spriteCfg.GetCostumeIndex())
	} else {
		p.baseObj.initWith(base, spriteCfg)
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

func (p *SpriteImpl) initEngineObjects() {
	p.runtimeState.SyncSprite = nil
	engine.WaitMainThread(func() {
		p.syncCheckInitProxy()
	})
}

func (p *SpriteImpl) initComponents(spriteCfg *coreproject.SpriteConfig) {
	p.components.initComponents(p, spriteCfg)
}

func (p *SpriteImpl) awake() {
	p.playDefaultAnim()
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
