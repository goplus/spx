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
	"fmt"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/base/collision"
	"github.com/goplus/spx/v3/internal/base/defaults"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

// ============================================================================
// Physics Component
// ============================================================================
// This component manages sprite physics state, collision, and trigger shapes.

// physicsComponent encapsulates all physics-related functionality.
type physicsComponent struct {
	componentBase

	triggerInfo   physicConfig
	collisionInfo physicConfig

	physicsMode     PhysicsMode
	mass            float64
	friction        float64
	airDrag         float64
	gravity         float64
	autoShapesDirty bool

	collisionTargets map[string]bool
}

// ============================================================================
// Lifecycle
// ============================================================================

// initialize initializes the physics component from config.
func (p *physicsComponent) initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	p.componentBase.initialize(sprite, spriteCfg)
	p.initCollisionConfig(sprite, spriteCfg)
	p.initTriggerConfig(sprite, spriteCfg)

	p.physicsMode = toPhysicsMode(spriteCfg.PhysicsMode)
	p.airDrag = defaults.OrDefault(spriteCfg.AirDrag, 1)
	p.gravity = defaults.OrDefault(spriteCfg.Gravity, 1)
	p.friction = defaults.OrDefault(spriteCfg.Friction, 1)
	p.mass = defaults.OrDefault(spriteCfg.Mass, 1)
	p.collisionTargets = make(map[string]bool)
}

// initCollisionConfig initializes collision configuration.
func (p *physicsComponent) initCollisionConfig(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	p.collisionInfo.Mask = parseLayerMaskValue(spriteCfg.CollisionMask)
	p.collisionInfo.Layer = parseLayerMaskValue(spriteCfg.CollisionLayer)

	var defaultCollisionType int64 = physicsColliderNone
	if isPhysicsEnabled() {
		defaultCollisionType = physicsColliderAuto
	}

	p.collisionInfo.Type = collision.ParseColliderShapeType(spriteCfg.CollisionShapeType, defaultCollisionType)
	p.collisionInfo.Pivot = spriteCfg.CollisionPivot
	p.collisionInfo.Params = spriteCfg.CollisionShapeParams

	if !p.collisionInfo.validateShape() {
		spxlog.Warn("Invalid collider configuration for sprite %s, using default values", sprite.name)
		p.collisionInfo.Type = physicsColliderNone
		p.collisionInfo.Params = nil
	}
}

// initTriggerConfig initializes trigger configuration.
func (p *physicsComponent) initTriggerConfig(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	p.triggerInfo.Mask = parseLayerMaskValue(spriteCfg.TriggerMask)
	p.triggerInfo.Layer = parseLayerMaskValue(spriteCfg.TriggerLayer)
	p.triggerInfo.Type = collision.ParseColliderShapeType(spriteCfg.TriggerShapeType, physicsColliderAuto)
	p.triggerInfo.Pivot = spriteCfg.TriggerPivot
	p.triggerInfo.Params = spriteCfg.TriggerShapeParams

	if !p.triggerInfo.validateShape() {
		spxlog.Warn("Invalid trigger configuration for sprite %s, using default values", sprite.name)
		p.triggerInfo.Type = physicsColliderAuto
		p.triggerInfo.Params = nil
	}
}

// cloneFrom creates a new physics component by cloning from source.
func (p *physicsComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	srcPhysics := src.(*physicsComponent)
	newPhys := &physicsComponent{
		componentBase:    componentBase{sprite: newSprite},
		physicsMode:      srcPhysics.physicsMode,
		mass:             srcPhysics.mass,
		friction:         srcPhysics.friction,
		airDrag:          srcPhysics.airDrag,
		gravity:          srcPhysics.gravity,
		collisionTargets: make(map[string]bool),
	}
	newPhys.collisionInfo.copyFrom(&srcPhysics.collisionInfo)
	newPhys.triggerInfo.copyFrom(&srcPhysics.triggerInfo)
	return newPhys
}

// onDestroy cleans up when the component is destroyed.
func (p *physicsComponent) onDestroy() {
}

// ============================================================================
// Physics Control
// ============================================================================

func (p *physicsComponent) setPhysicsMode(mode PhysicsMode) {
	p.physicsMode = mode
	p.engine().SpriteMgr.SetPhysicsMode(p.sprite.getSpriteId(), int64(mode))
}

func (p *physicsComponent) getPhysicsMode() PhysicsMode {
	return p.physicsMode
}

func (p *physicsComponent) getVelocity() (velocityX, velocityY float64) {
	vel := p.engine().SpriteMgr.GetVelocity(p.sprite.getSpriteId())
	return vel.X, vel.Y
}

func (p *physicsComponent) setVelocity(velocityX, velocityY float64) {
	p.engine().SpriteMgr.SetVelocity(p.sprite.getSpriteId(), mathf.NewVec2(velocityX, velocityY))
}

func (p *physicsComponent) addImpulse(impulseX, impulseY float64) {
	p.engine().SpriteMgr.AddImpulse(p.sprite.getSpriteId(), mathf.NewVec2(impulseX, impulseY))
}

func (p *physicsComponent) isOnFloor() bool {
	return p.engine().SpriteMgr.IsOnFloor(p.sprite.getSpriteId())
}

func (p *physicsComponent) getGravity() float64 {
	return p.engine().SpriteMgr.GetGravity(p.sprite.getSpriteId())
}

func (p *physicsComponent) setGravity(gravity float64) {
	p.engine().SpriteMgr.SetGravity(p.sprite.getSpriteId(), gravity)
}

// ============================================================================
// Collision and Trigger Control
// ============================================================================

func (p *physicsComponent) setCollisionLayer(layer int64) {
	p.sprite.runtimeState.SyncSprite.SetCollisionLayer(layer)
}

func (p *physicsComponent) setCollisionMask(mask int64) {
	p.sprite.runtimeState.SyncSprite.SetCollisionMask(mask)
}

func (p *physicsComponent) setCollisionEnabled(enabled bool) {
	p.sprite.runtimeState.SyncSprite.SetCollisionEnabled(enabled)
}

func (p *physicsComponent) getCollisionLayer() int64 {
	return p.sprite.runtimeState.SyncSprite.GetCollisionLayer()
}

func (p *physicsComponent) getCollisionMask() int64 {
	return p.sprite.runtimeState.SyncSprite.GetCollisionMask()
}

func (p *physicsComponent) isCollisionEnabled() bool {
	return p.sprite.runtimeState.SyncSprite.IsCollisionEnabled()
}

func (p *physicsComponent) setTriggerEnabled(trigger bool) {
	p.sprite.runtimeState.SyncSprite.SetTriggerEnabled(trigger)
}

func (p *physicsComponent) setTriggerLayer(layer int64) {
	p.sprite.runtimeState.SyncSprite.SetTriggerLayer(layer)
}

func (p *physicsComponent) setTriggerMask(mask int64) {
	p.sprite.runtimeState.SyncSprite.SetTriggerMask(mask)
}

func (p *physicsComponent) getTriggerLayer() int64 {
	return p.sprite.runtimeState.SyncSprite.GetTriggerLayer()
}

func (p *physicsComponent) getTriggerMask() int64 {
	return p.sprite.runtimeState.SyncSprite.GetTriggerMask()
}

func (p *physicsComponent) isTriggerEnabled() bool {
	return p.sprite.runtimeState.SyncSprite.IsTriggerEnabled()
}

// ============================================================================
// Collider Configuration
// ============================================================================

func (p *physicsComponent) setColliderShape(isTrigger bool, ctype ColliderShapeType, params []float64) error {
	config := p.getPhysicConfig(isTrigger)
	originalType := config.Type
	originalParams := make([]float64, len(config.Params))
	copy(originalParams, config.Params)

	config.Type = ctype
	config.Params = make([]float64, len(params))
	copy(config.Params, params)

	if !config.validateShape() {
		config.Type = originalType
		config.Params = originalParams
		return fmt.Errorf("invalid shape parameters for type %d", ctype)
	}

	p.applyPhysicShape(isTrigger)
	return nil
}

func (p *physicsComponent) getColliderShape(isTrigger bool) (ColliderShapeType, []float64) {
	config := p.getPhysicConfig(isTrigger)
	params := make([]float64, len(config.Params))
	copy(params, config.Params)
	return config.Type, params
}

func (p *physicsComponent) setColliderPivot(isTrigger bool, offsetX, offsetY float64) {
	config := p.getPhysicConfig(isTrigger)
	config.Pivot = mathf.NewVec2(offsetX, offsetY)
	if p.sprite.runtimeState.SyncSprite != nil {
		p.applyPhysicShape(isTrigger)
	}
}

func (p *physicsComponent) getColliderPivot(isTrigger bool) (offsetX, offsetY float64) {
	config := p.getPhysicConfig(isTrigger)
	return config.Pivot.X, config.Pivot.Y
}

func (p *physicsComponent) getTriggerInfo() *physicConfig {
	return &p.triggerInfo
}

func (p *physicsComponent) getCollisionInfo() *physicConfig {
	return &p.collisionInfo
}

func (p *physicsComponent) getPhysicConfig(isTrigger bool) *physicConfig {
	if isTrigger {
		return &p.triggerInfo
	}
	return &p.collisionInfo
}

func (p *physicsComponent) applyPhysicShape(isTrigger bool) {
	config := p.getPhysicConfig(isTrigger)
	if p.sprite.runtimeState.SyncSprite == nil {
		return
	}
	config.applyShape(p.sprite.runtimeState.SyncSprite, isTrigger, p.sprite)
}

// ============================================================================
// Proxy Sync
// ============================================================================

// initCollisionParams initializes collision parameters based on game settings.
func (p *physicsComponent) initCollisionParams() {
	if p.sprite.g.isAutoSetCollisionLayer {
		info := p.sprite.g.getSpriteCollisionInfo(p.sprite.name)
		p.collisionInfo.Layer = 0
		p.collisionInfo.Mask = 0
		p.triggerInfo.Layer = int64(info.Layer)
		p.triggerInfo.Mask = int64(info.Mask)
		if isPhysicsEnabled() {
			p.collisionInfo.Layer = int64(info.Layer)
			p.collisionInfo.Mask = int64(info.Mask)
		}
	}
}

// applyPhysicsProxyConfig applies initial physics configuration to the engine sprite proxy.
func (p *physicsComponent) applyPhysicsProxyConfig(syncProxy *engine.Sprite) {
	p.initCollisionParams()
	p.collisionInfo.syncToProxy(syncProxy, false, p.sprite)
	p.triggerInfo.syncToProxy(syncProxy, true, p.sprite)
	syncProxy.SetGravityScale(p.gravity)
	syncProxy.SetPhysicsMode(p.physicsMode)
	p.autoShapesDirty = false
}

// ============================================================================
// Collision Targets
// ============================================================================

func (p *physicsComponent) getCollisionTargets() map[string]bool {
	return p.collisionTargets
}

func (p *physicsComponent) addCollisionTarget(target string) {
	if p.collisionTargets[target] {
		return
	}
	p.collisionTargets[target] = true
	if p.sprite != nil && !p.sprite.IsCloned() && p.sprite.g != nil {
		p.sprite.g.refreshCollisionLayers()
	}
}

// ============================================================================
// Physics Helpers
// ============================================================================

func parseLayerMaskValue(pval *int64) int64 {
	return defaults.OrDefault(pval, 1)
}
