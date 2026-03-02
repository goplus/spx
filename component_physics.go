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
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// ============================================================================
// Physics Component
// ============================================================================
// This component encapsulates all physics-related functionality

type physicsComponent struct {
	componentBase

	// Physics configuration
	triggerInfo   physicConfig
	collisionInfo physicConfig

	// Physics properties
	physicsMode PhysicsMode
	mass        float64
	friction    float64
	airDrag     float64
	gravity     float64

	// Collision tracking
	collisionTargets map[string]bool
}

// initialize initializes the physics component from config.
func (p *physicsComponent) initialize(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	p.componentBase.initialize(sprite, spriteCfg)
	// Always initialize from config
	p.initCollisionConfig(sprite, spriteCfg)
	p.initTriggerConfig(sprite, spriteCfg)

	// Physics properties
	p.physicsMode = toPhysicsMode(spriteCfg.PhysicsMode)
	p.airDrag = parseDefaultValue(spriteCfg.AirDrag, 1)
	p.gravity = parseDefaultValue(spriteCfg.Gravity, 1)
	p.friction = parseDefaultValue(spriteCfg.Friction, 1)
	p.mass = parseDefaultValue(spriteCfg.Mass, 1)

	// Initialize collision targets map
	p.collisionTargets = make(map[string]bool)
}

// initCollisionConfig initializes collision configuration.
func (p *physicsComponent) initCollisionConfig(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	p.collisionInfo.Mask = parseLayerMaskValue(spriteCfg.CollisionMask)
	p.collisionInfo.Layer = parseLayerMaskValue(spriteCfg.CollisionLayer)

	// collider is disable by default
	var defaultCollisionType int64 = physicsColliderNone
	if isPhysicsEnabled() {
		defaultCollisionType = physicsColliderAuto
	}

	p.collisionInfo.Type = parseColliderShapeType(spriteCfg.CollisionShapeType, defaultCollisionType)
	p.collisionInfo.Pivot = spriteCfg.CollisionPivot
	p.collisionInfo.Params = spriteCfg.CollisionShapeParams

	// Validate colliderShapeType and colliderShape length matching
	if !p.collisionInfo.validateShape() {
		spxlog.Warn("Invalid collider configuration for sprite %s, using default values", sprite.name)
		p.collisionInfo.Type = physicsColliderNone
		p.collisionInfo.Params = nil
	}
}

// initTriggerConfig initializes trigger configuration.
func (p *physicsComponent) initTriggerConfig(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	p.triggerInfo.Mask = parseLayerMaskValue(spriteCfg.TriggerMask)
	p.triggerInfo.Layer = parseLayerMaskValue(spriteCfg.TriggerLayer)
	p.triggerInfo.Type = parseColliderShapeType(spriteCfg.TriggerShapeType, physicsColliderAuto)
	p.triggerInfo.Pivot = spriteCfg.TriggerPivot
	p.triggerInfo.Params = spriteCfg.TriggerShapeParams

	// Validate triggerType and triggerShape length matching
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

// OnDestroy cleanup when component is destroyed
func (p *physicsComponent) onDestroy() {
	// Nothing to cleanup
}

// ============================================================================
// Physics Mode Control
// ============================================================================

// SetPhysicsMode sets the physics mode for the sprite.
func (p *physicsComponent) SetPhysicsMode(mode PhysicsMode) {
	p.physicsMode = mode
	p.engine().SpriteMgr.SetPhysicsMode(p.sprite.getSpriteId(), int64(mode))
}

// GetPhysicsMode returns the current physics mode.
func (p *physicsComponent) GetPhysicsMode() PhysicsMode {
	return p.physicsMode
}

// ============================================================================
// Velocity and Movement
// ============================================================================

// GetVelocity returns the current velocity in X and Y directions.
func (p *physicsComponent) GetVelocity() (velocityX, velocityY float64) {
	vel := p.engine().SpriteMgr.GetVelocity(p.sprite.getSpriteId())
	return vel.X, vel.Y
}

// SetVelocity sets the velocity in X and Y directions.
func (p *physicsComponent) SetVelocity(velocityX, velocityY float64) {
	p.engine().SpriteMgr.SetVelocity(p.sprite.getSpriteId(), mathf.NewVec2(velocityX, velocityY))
}

// AddImpulse applies an impulse force to the sprite.
func (p *physicsComponent) AddImpulse(impulseX, impulseY float64) {
	p.engine().SpriteMgr.AddImpulse(p.sprite.getSpriteId(), mathf.NewVec2(impulseX, impulseY))
}

// IsOnFloor checks if the sprite is on the floor.
func (p *physicsComponent) IsOnFloor() bool {
	return p.engine().SpriteMgr.IsOnFloor(p.sprite.getSpriteId())
}

// ============================================================================
// Gravity Control
// ============================================================================

// GetGravity returns the current gravity scale.
func (p *physicsComponent) GetGravity() float64 {
	return p.engine().SpriteMgr.GetGravity(p.sprite.getSpriteId())
}

// SetGravity sets the gravity scale for the sprite.
func (p *physicsComponent) SetGravity(gravity float64) {
	p.engine().SpriteMgr.SetGravity(p.sprite.getSpriteId(), gravity)
}

// ============================================================================
// Collider Shape Control
// ============================================================================

// SetColliderShape sets the collider shape type and parameters.
func (p *physicsComponent) SetColliderShape(isTrigger bool, ctype ColliderShapeType, params []float64) error {
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

func (p *physicsComponent) GetColliderShape(isTrigger bool) (ColliderShapeType, []float64) {
	config := p.getPhysicConfig(isTrigger)
	params := make([]float64, len(config.Params))
	copy(params, config.Params)
	return config.Type, params
}

func (p *physicsComponent) SetColliderPivot(isTrigger bool, offsetX, offsetY float64) {
	config := p.getPhysicConfig(isTrigger)
	config.Pivot = mathf.NewVec2(offsetX, offsetY)

	// Re-apply current shape with new pivot if needed
	if p.sprite.syncSprite != nil {
		p.applyPhysicShape(isTrigger)
	}
}

func (p *physicsComponent) GetColliderPivot(isTrigger bool) (offsetX, offsetY float64) {
	config := p.getPhysicConfig(isTrigger)
	return config.Pivot.X, config.Pivot.Y
}

// ============================================================================
// Collision Layer and Mask Control
// ============================================================================

func (p *physicsComponent) SetCollisionLayer(layer int64) {
	p.sprite.syncSprite.SetCollisionLayer(layer)
}

func (p *physicsComponent) SetCollisionMask(mask int64) {
	p.sprite.syncSprite.SetCollisionMask(mask)
}

func (p *physicsComponent) SetCollisionEnabled(enabled bool) {
	p.sprite.syncSprite.SetCollisionEnabled(enabled)
}

func (p *physicsComponent) GetCollisionLayer() int64 {
	return p.sprite.syncSprite.GetCollisionLayer()
}

func (p *physicsComponent) GetCollisionMask() int64 {
	return p.sprite.syncSprite.GetCollisionMask()
}

func (p *physicsComponent) IsCollisionEnabled() bool {
	return p.sprite.syncSprite.IsCollisionEnabled()
}

// ============================================================================
// Trigger Layer and Mask Control
// ============================================================================

func (p *physicsComponent) SetTriggerEnabled(trigger bool) {
	p.sprite.syncSprite.SetTriggerEnabled(trigger)
}

func (p *physicsComponent) SetTriggerLayer(layer int64) {
	p.sprite.syncSprite.SetTriggerLayer(layer)
}

func (p *physicsComponent) SetTriggerMask(mask int64) {
	p.sprite.syncSprite.SetTriggerMask(mask)
}

func (p *physicsComponent) GetTriggerLayer() int64 {
	return p.sprite.syncSprite.GetTriggerLayer()
}

func (p *physicsComponent) GetTriggerMask() int64 {
	return p.sprite.syncSprite.GetTriggerMask()
}

func (p *physicsComponent) IsTriggerEnabled() bool {
	return p.sprite.syncSprite.IsTriggerEnabled()
}

// ============================================================================
// Sync Methods
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

// syncInitPhysicInfo synchronizes physics information to the engine sprite proxy
func (p *physicsComponent) syncInitPhysicInfo(syncProxy *engine.Sprite) {
	p.initCollisionParams()
	p.collisionInfo.syncToProxy(syncProxy, false, p.sprite)
	p.triggerInfo.syncToProxy(syncProxy, true, p.sprite)
	syncProxy.SetGravityScale(p.gravity)
	syncProxy.SetPhysicsMode(p.physicsMode)
}

// ============================================================================
// Collision Targets Management
// ============================================================================

func (p *physicsComponent) getCollisionTargets() map[string]bool {
	return p.collisionTargets
}

func (p *physicsComponent) addCollisionTarget(target string) {
	p.collisionTargets[target] = true
}

// ============================================================================
// Accessor Methods
// ============================================================================

func (p *physicsComponent) getTriggerInfo() *physicConfig {
	return &p.triggerInfo
}

func (p *physicsComponent) getCollisionInfo() *physicConfig {
	return &p.collisionInfo
}

// ============================================================================
// Private Helper Methods
// ============================================================================

func (p *physicsComponent) getPhysicConfig(isTrigger bool) *physicConfig {
	if isTrigger {
		return &p.triggerInfo
	}
	return &p.collisionInfo
}

func (p *physicsComponent) applyPhysicShape(isTrigger bool) {
	config := p.getPhysicConfig(isTrigger)
	ctype := config.Type
	params := config.Params

	if p.sprite.syncSprite != nil {
		switch ctype {
		case RectCollider:
			if len(params) >= 2 {
				p.sprite.syncSprite.SetColliderShapeRect(isTrigger, config.Pivot, mathf.NewVec2(params[0], params[1]))
			}
		case CircleCollider:
			if len(params) >= 1 {
				p.sprite.syncSprite.SetColliderShapeCircle(isTrigger, config.Pivot, params[0])
			}
		case CapsuleCollider:
			if len(params) >= 2 {
				p.sprite.syncSprite.SetColliderShapeCapsule(isTrigger, config.Pivot, mathf.NewVec2(params[0]*2, params[1]))
			}
		case PolygonCollider:
			if len(params) >= 6 {
				// Polygon requires at least 3 points (6 floats: x1,y1,x2,y2,x3,y3)
				p.sprite.syncSprite.SetColliderShapePolygon(isTrigger, config.Pivot, params)
			}
		}
	}
}
