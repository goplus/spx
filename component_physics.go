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
}

// initialize initializes the physics component from config
func (pc *physicsComponent) initialize(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	pc.componentBase.initialize(sprite, spriteCfg)
	// Always initialize from config
	pc.initCollisionConfig(sprite, spriteCfg)
	pc.initTriggerConfig(sprite, spriteCfg)

	// Physics properties
	pc.physicsMode = toPhysicsMode(spriteCfg.PhysicsMode)
	pc.airDrag = parseDefaultFloatValue(spriteCfg.AirDrag, 1)
	pc.gravity = parseDefaultFloatValue(spriteCfg.Gravity, 1)
	pc.friction = parseDefaultFloatValue(spriteCfg.Friction, 1)
	pc.mass = parseDefaultFloatValue(spriteCfg.Mass, 1)
}

// initCollisionConfig initializes collision configuration
func (pc *physicsComponent) initCollisionConfig(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	pc.collisionInfo.Mask = parseLayerMaskValue(spriteCfg.CollisionMask)
	pc.collisionInfo.Layer = parseLayerMaskValue(spriteCfg.CollisionLayer)

	// collider is disable by default
	var defaultCollisionType int64 = physicsColliderNone
	if enabledPhysics {
		defaultCollisionType = physicsColliderAuto
	}

	pc.collisionInfo.Type = parseColliderShapeType(spriteCfg.CollisionShapeType, defaultCollisionType)
	pc.collisionInfo.Pivot = spriteCfg.CollisionPivot
	pc.collisionInfo.Params = spriteCfg.CollisionShapeParams

	// Validate colliderShapeType and colliderShape length matching
	if !pc.collisionInfo.validateShape() {
		spxlog.Warn("Invalid collider configuration for sprite %s, using default values", sprite.name)
		pc.collisionInfo.Type = physicsColliderNone
		pc.collisionInfo.Params = nil
	}
}

// initTriggerConfig initializes trigger configuration
func (pc *physicsComponent) initTriggerConfig(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	pc.triggerInfo.Mask = parseLayerMaskValue(spriteCfg.TriggerMask)
	pc.triggerInfo.Layer = parseLayerMaskValue(spriteCfg.TriggerLayer)
	pc.triggerInfo.Type = parseColliderShapeType(spriteCfg.TriggerShapeType, physicsColliderAuto)
	pc.triggerInfo.Pivot = spriteCfg.TriggerPivot
	pc.triggerInfo.Params = spriteCfg.TriggerShapeParams

	// Validate triggerType and triggerShape length matching
	if !pc.triggerInfo.validateShape() {
		spxlog.Warn("Invalid trigger configuration for sprite %s, using default values", sprite.name)
		pc.triggerInfo.Type = physicsColliderAuto
		pc.triggerInfo.Params = nil
	}
}

// cloneFrom creates a new physics component by cloning from source
func (pc *physicsComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	srcPhysics := src.(*physicsComponent)
	newPhys := &physicsComponent{
		componentBase: componentBase{sprite: newSprite},
		physicsMode:   srcPhysics.physicsMode,
		mass:          srcPhysics.mass,
		friction:      srcPhysics.friction,
		airDrag:       srcPhysics.airDrag,
		gravity:       srcPhysics.gravity,
	}
	newPhys.collisionInfo.copyFrom(&srcPhysics.collisionInfo)
	newPhys.triggerInfo.copyFrom(&srcPhysics.triggerInfo)
	return newPhys
}

// cloneFrom creates a new physics component by cloning from source
func (pc *physicsComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	newPhys := &physicsComponent{
		componentBase: componentBase{sprite: newSprite},
		physicsMode:   newSprite.physicsMode,
		mass:          newSprite.mass,
		friction:      newSprite.friction,
		airDrag:       newSprite.airDrag,
		gravity:       newSprite.gravity,
	}
	newPhys.collisionInfo.copyFrom(&newSprite.collisionInfo)
	newPhys.triggerInfo.copyFrom(&newSprite.triggerInfo)
	return newPhys
}

// OnDestroy cleanup when component is destroyed
func (pc *physicsComponent) onDestroy() {
	// Nothing to cleanup
}

// ============================================================================
// Physics Mode Control
// ============================================================================

func (pc *physicsComponent) SetPhysicsMode(mode PhysicsMode) {
	pc.physicsMode = mode
	spriteMgr.SetPhysicsMode(pc.sprite.getSpriteId(), int64(mode))
}

func (pc *physicsComponent) GetPhysicsMode() PhysicsMode {
	return pc.physicsMode
}

// ============================================================================
// Velocity and Movement
// ============================================================================

func (pc *physicsComponent) GetVelocity() (velocityX, velocityY float64) {
	vel := spriteMgr.GetVelocity(pc.sprite.getSpriteId())
	return vel.X, vel.Y
}

func (pc *physicsComponent) SetVelocity(velocityX, velocityY float64) {
	spriteMgr.SetVelocity(pc.sprite.getSpriteId(), mathf.NewVec2(velocityX, velocityY))
}

func (pc *physicsComponent) AddImpulse(impulseX, impulseY float64) {
	spriteMgr.AddImpulse(pc.sprite.getSpriteId(), mathf.NewVec2(impulseX, impulseY))
}

func (pc *physicsComponent) IsOnFloor() bool {
	return spriteMgr.IsOnFloor(pc.sprite.getSpriteId())
}

// ============================================================================
// Gravity Control
// ============================================================================

func (pc *physicsComponent) GetGravity() float64 {
	return spriteMgr.GetGravity(pc.sprite.getSpriteId())
}

func (pc *physicsComponent) SetGravity(gravity float64) {
	spriteMgr.SetGravity(pc.sprite.getSpriteId(), gravity)
}

// ============================================================================
// Collider Shape Control
// ============================================================================

func (pc *physicsComponent) SetColliderShape(isTrigger bool, ctype ColliderShapeType, params []float64) error {
	config := pc.getPhysicConfig(isTrigger)

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
	pc.applyPhysicShape(isTrigger)
	return nil
}

func (pc *physicsComponent) GetColliderShape(isTrigger bool) (ColliderShapeType, []float64) {
	config := pc.getPhysicConfig(isTrigger)
	params := make([]float64, len(config.Params))
	copy(params, config.Params)
	return config.Type, params
}

func (pc *physicsComponent) SetColliderPivot(isTrigger bool, offsetX, offsetY float64) {
	config := pc.getPhysicConfig(isTrigger)
	config.Pivot = mathf.NewVec2(offsetX, offsetY)

	// Re-apply current shape with new pivot if needed
	if pc.sprite.syncSprite != nil {
		pc.applyPhysicShape(isTrigger)
	}
}

func (pc *physicsComponent) GetColliderPivot(isTrigger bool) (offsetX, offsetY float64) {
	config := pc.getPhysicConfig(isTrigger)
	return config.Pivot.X, config.Pivot.Y
}

// ============================================================================
// Collision Layer and Mask Control
// ============================================================================

func (pc *physicsComponent) SetCollisionLayer(layer int64) {
	pc.sprite.syncSprite.SetCollisionLayer(layer)
}

func (pc *physicsComponent) SetCollisionMask(mask int64) {
	pc.sprite.syncSprite.SetCollisionMask(mask)
}

func (pc *physicsComponent) SetCollisionEnabled(enabled bool) {
	pc.sprite.syncSprite.SetCollisionEnabled(enabled)
}

func (pc *physicsComponent) GetCollisionLayer() int64 {
	return pc.sprite.syncSprite.GetCollisionLayer()
}

func (pc *physicsComponent) GetCollisionMask() int64 {
	return pc.sprite.syncSprite.GetCollisionMask()
}

func (pc *physicsComponent) IsCollisionEnabled() bool {
	return pc.sprite.syncSprite.IsCollisionEnabled()
}

// ============================================================================
// Trigger Layer and Mask Control
// ============================================================================

func (pc *physicsComponent) SetTriggerEnabled(trigger bool) {
	pc.sprite.syncSprite.SetTriggerEnabled(trigger)
}

func (pc *physicsComponent) SetTriggerLayer(layer int64) {
	pc.sprite.syncSprite.SetTriggerLayer(layer)
}

func (pc *physicsComponent) SetTriggerMask(mask int64) {
	pc.sprite.syncSprite.SetTriggerMask(mask)
}

func (pc *physicsComponent) GetTriggerLayer() int64 {
	return pc.sprite.syncSprite.GetTriggerLayer()
}

func (pc *physicsComponent) GetTriggerMask() int64 {
	return pc.sprite.syncSprite.GetTriggerMask()
}

func (pc *physicsComponent) IsTriggerEnabled() bool {
	return pc.sprite.syncSprite.IsTriggerEnabled()
}

// ============================================================================
// Sync Methods
// ============================================================================

// initCollisionParams initializes collision parameters based on game settings
func (pc *physicsComponent) initCollisionParams() {
	if pc.sprite.g.isAutoSetCollisionLayer {
		info := pc.sprite.g.getSpriteCollisionInfo(pc.sprite.name)
		pc.collisionInfo.Layer = 0
		pc.collisionInfo.Mask = 0
		pc.triggerInfo.Layer = int64(info.Layer)
		pc.triggerInfo.Mask = int64(info.Mask)
		if enabledPhysics {
			pc.collisionInfo.Layer = int64(info.Layer)
			pc.collisionInfo.Mask = int64(info.Mask)
		}
	}
}

// syncInitPhysicInfo synchronizes physics information to the engine sprite proxy
func (pc *physicsComponent) syncInitPhysicInfo(syncProxy *engine.Sprite) {
	pc.initCollisionParams()
	pc.collisionInfo.syncToProxy(syncProxy, false, pc.sprite)
	pc.triggerInfo.syncToProxy(syncProxy, true, pc.sprite)
	syncProxy.SetGravityScale(pc.gravity)
	syncProxy.SetPhysicsMode(pc.physicsMode)
}

// ============================================================================
// Accessor Methods
// ============================================================================

func (pc *physicsComponent) getTriggerInfo() *physicConfig {
	return &pc.triggerInfo
}

func (pc *physicsComponent) getCollisionInfo() *physicConfig {
	return &pc.collisionInfo
}

// ============================================================================
// Private Helper Methods
// ============================================================================

func (pc *physicsComponent) getPhysicConfig(isTrigger bool) *physicConfig {
	if isTrigger {
		return &pc.triggerInfo
	}
	return &pc.collisionInfo
}

func (pc *physicsComponent) applyPhysicShape(isTrigger bool) {
	config := pc.getPhysicConfig(isTrigger)
	ctype := config.Type
	params := config.Params

	if pc.sprite.syncSprite != nil {
		switch ctype {
		case RectCollider:
			if len(params) >= 2 {
				pc.sprite.syncSprite.SetColliderShapeRect(isTrigger, config.Pivot, mathf.NewVec2(params[0], params[1]))
			}
		case CircleCollider:
			if len(params) >= 1 {
				pc.sprite.syncSprite.SetColliderShapeCircle(isTrigger, config.Pivot, params[0])
			}
		case CapsuleCollider:
			if len(params) >= 2 {
				pc.sprite.syncSprite.SetColliderShapeCapsule(isTrigger, config.Pivot, mathf.NewVec2(params[0]*2, params[1]))
			}
		case PolygonCollider:
			// TODO: Implement polygon shape setting when available
		}
	}
}
