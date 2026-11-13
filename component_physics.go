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
	"slices"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/base"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/core/collision"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
)

type physicsComponent struct {
	sprite *SpriteImpl

	bodyCollider    colliderConfig
	triggerCollider colliderConfig

	physicsMode     PhysicsMode
	gravity         float64
	autoShapesDirty bool

	touchTargets map[string]bool
}

func (p *physicsComponent) initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	p.sprite = sprite
	p.initCollisionConfig(spriteCfg)
	p.initTriggerConfig(spriteCfg)

	p.physicsMode = toPhysicsMode(spriteCfg.PhysicsMode)
	p.gravity = base.OrDefault(spriteCfg.Gravity, 1)
	p.touchTargets = make(map[string]bool)
}

func (p *physicsComponent) cloneFor(newSprite *SpriteImpl) *physicsComponent {
	return &physicsComponent{
		sprite:          newSprite,
		bodyCollider:    p.bodyCollider.clone(),
		triggerCollider: p.triggerCollider.clone(),
		physicsMode:     p.physicsMode,
		gravity:         p.gravity,
		touchTargets:    make(map[string]bool),
	}
}

func (p *physicsComponent) setPhysicsMode(mode PhysicsMode) {
	p.physicsMode = mode
	engine.Managers().SpriteMgr.SetPhysicsMode(p.sprite.getSpriteId(), mode)
}

func (p *physicsComponent) getPhysicsMode() PhysicsMode {
	return p.physicsMode
}

func (p *physicsComponent) setVelocity(velocityX, velocityY float64) {
	engine.Managers().SpriteMgr.SetVelocity(p.sprite.getSpriteId(), mathf.NewVec2(velocityX, velocityY))
}

func (p *physicsComponent) getVelocity() (velocityX, velocityY float64) {
	vel := engine.Managers().SpriteMgr.GetVelocity(p.sprite.getSpriteId())
	return vel.X, vel.Y
}

func (p *physicsComponent) addImpulse(impulseX, impulseY float64) {
	engine.Managers().SpriteMgr.AddImpulse(p.sprite.getSpriteId(), mathf.NewVec2(impulseX, impulseY))
}

func (p *physicsComponent) isOnFloor() bool {
	return engine.Managers().SpriteMgr.IsOnFloor(p.sprite.getSpriteId())
}

func (p *physicsComponent) setGravity(gravity float64) {
	engine.Managers().SpriteMgr.SetGravity(p.sprite.getSpriteId(), gravity)
}

func (p *physicsComponent) getGravity() float64 {
	return engine.Managers().SpriteMgr.GetGravity(p.sprite.getSpriteId())
}

func (p *physicsComponent) setCollisionLayer(layer int64) {
	p.bodyCollider.Layer = layer
	p.sprite.runtimeState.syncSprite.SetCollisionLayer(layer)
}

func (p *physicsComponent) getCollisionLayer() int64 {
	return p.sprite.runtimeState.syncSprite.GetCollisionLayer()
}

func (p *physicsComponent) setCollisionMask(mask int64) {
	p.bodyCollider.Mask = mask
	p.sprite.runtimeState.syncSprite.SetCollisionMask(mask)
}

func (p *physicsComponent) getCollisionMask() int64 {
	return p.sprite.runtimeState.syncSprite.GetCollisionMask()
}

func (p *physicsComponent) setCollisionEnabled(enabled bool) {
	p.sprite.runtimeState.syncSprite.SetCollisionEnabled(enabled)
}

func (p *physicsComponent) isCollisionEnabled() bool {
	return p.sprite.runtimeState.syncSprite.IsCollisionEnabled()
}

func (p *physicsComponent) setTriggerLayer(layer int64) {
	p.triggerCollider.Layer = layer
	p.sprite.runtimeState.syncSprite.SetTriggerLayer(layer)
}

func (p *physicsComponent) getTriggerLayer() int64 {
	return p.sprite.runtimeState.syncSprite.GetTriggerLayer()
}

func (p *physicsComponent) setTriggerMask(mask int64) {
	p.triggerCollider.Mask = mask
	p.sprite.runtimeState.syncSprite.SetTriggerMask(mask)
}

func (p *physicsComponent) getTriggerMask() int64 {
	return p.sprite.runtimeState.syncSprite.GetTriggerMask()
}

func (p *physicsComponent) setTriggerEnabled(enabled bool) {
	p.sprite.runtimeState.syncSprite.SetTriggerEnabled(enabled)
}

func (p *physicsComponent) isTriggerEnabled() bool {
	return p.sprite.runtimeState.syncSprite.IsTriggerEnabled()
}

func (p *physicsComponent) setColliderShape(isTrigger bool, shapeType ColliderShapeType, params []float64) error {
	config := p.colliderConfig(isTrigger)
	candidate := *config
	candidate.Type = shapeType
	candidate.Params = params

	if err := candidate.validateShape(); err != nil {
		return err
	}

	candidate.Params = slices.Clone(params)
	*config = candidate
	if proxy := p.sprite.runtimeState.syncSprite; proxy != nil {
		proxy.SetColliderEnabled(isTrigger, config.defaultEnabled(isTrigger, p.sprite))
		config.syncShape(proxy, isTrigger, p.sprite)
	}
	return nil
}

func (p *physicsComponent) getColliderShape(isTrigger bool) (ColliderShapeType, []float64) {
	config := p.colliderConfig(isTrigger)
	params := make([]float64, len(config.Params))
	copy(params, config.Params)
	return config.Type, params
}

func (p *physicsComponent) setColliderPivot(isTrigger bool, offsetX, offsetY float64) {
	config := p.colliderConfig(isTrigger)
	config.Pivot = mathf.NewVec2(offsetX, offsetY)
	if proxy := p.sprite.runtimeState.syncSprite; proxy != nil {
		config.syncShape(proxy, isTrigger, p.sprite)
	}
}

func (p *physicsComponent) getColliderPivot(isTrigger bool) (offsetX, offsetY float64) {
	config := p.colliderConfig(isTrigger)
	return config.Pivot.X, config.Pivot.Y
}

func (p *physicsComponent) addTouchTargets(targets ...string) {
	changed := false
	for _, target := range targets {
		if !p.touchTargets[target] {
			p.touchTargets[target] = true
			changed = true
		}
	}
	if changed && p.sprite != nil && !p.sprite.IsCloned() && p.sprite.g != nil {
		p.sprite.g.refreshTouchFilters()
	}
}

func (p *physicsComponent) initializeProxy(proxy *engine.Sprite) {
	if p.sprite.g.autoTouchLayers {
		p.bodyCollider.Layer = 0
		p.bodyCollider.Mask = 0
	}
	proxy.SetCollisionLayer(p.bodyCollider.Layer)
	proxy.SetCollisionMask(p.bodyCollider.Mask)
	p.syncTriggerFilters(proxy)
	p.bodyCollider.initializeShape(proxy, false, p.sprite)
	p.triggerCollider.initializeShape(proxy, true, p.sprite)
	proxy.SetGravityScale(p.gravity)
	proxy.SetPhysicsMode(p.physicsMode)
	p.autoShapesDirty = false
}

func (p *physicsComponent) syncTriggerFilters(proxy *engine.Sprite) {
	if p.sprite.g.autoTouchLayers {
		filter := p.sprite.g.getTouchFilter(p.sprite.name)
		p.triggerCollider.Layer = filter.Layer
		p.triggerCollider.Mask = filter.Mask
	}
	proxy.SetTriggerLayer(p.triggerCollider.Layer)
	proxy.SetTriggerMask(p.triggerCollider.Mask)
}

func (p *physicsComponent) refreshAutoShapes(proxy *engine.Sprite) {
	if !p.autoShapesDirty || proxy == nil {
		return
	}
	p.triggerCollider.refreshAutoShape(proxy, true, p.sprite)
	p.bodyCollider.refreshAutoShape(proxy, false, p.sprite)
	p.autoShapesDirty = false
}

func (p *physicsComponent) initCollisionConfig(spriteCfg *coreproject.SpriteConfig) {
	p.bodyCollider.Mask = base.OrDefault(spriteCfg.CollisionMask, 1)
	p.bodyCollider.Layer = base.OrDefault(spriteCfg.CollisionLayer, 1)

	defaultType := collision.ColliderNone
	if p.sprite.g.physicsEnabled {
		defaultType = collision.ColliderAuto
	}

	p.bodyCollider.Type = collision.ParseColliderShapeType(spriteCfg.CollisionShapeType, defaultType)
	p.bodyCollider.Pivot = spriteCfg.CollisionPivot
	p.bodyCollider.Params = slices.Clone(spriteCfg.CollisionShapeParams)

	if err := p.bodyCollider.validateShape(); err != nil {
		spxlog.Warn("Invalid collider for sprite %s: %v; using none", p.sprite.name, err)
		p.bodyCollider.Type = collision.ColliderNone
		p.bodyCollider.Params = nil
	}
}

func (p *physicsComponent) initTriggerConfig(spriteCfg *coreproject.SpriteConfig) {
	p.triggerCollider.Mask = base.OrDefault(spriteCfg.TriggerMask, 1)
	p.triggerCollider.Layer = base.OrDefault(spriteCfg.TriggerLayer, 1)
	p.triggerCollider.Type = collision.ParseColliderShapeType(spriteCfg.TriggerShapeType, collision.ColliderAuto)
	p.triggerCollider.Pivot = spriteCfg.TriggerPivot
	p.triggerCollider.Params = slices.Clone(spriteCfg.TriggerShapeParams)

	if err := p.triggerCollider.validateShape(); err != nil {
		spxlog.Warn("Invalid trigger for sprite %s: %v; using auto", p.sprite.name, err)
		p.triggerCollider.Type = collision.ColliderAuto
		p.triggerCollider.Params = nil
	}
}

func (p *physicsComponent) colliderConfig(isTrigger bool) *colliderConfig {
	if isTrigger {
		return &p.triggerCollider
	}
	return &p.bodyCollider
}
