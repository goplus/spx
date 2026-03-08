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
	"github.com/goplus/spx/v2/internal/base/collisionutil"
	"github.com/goplus/spx/v2/internal/base/valueutil"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// physicsComponent encapsulates all physics-related functionality.
type physicsComponent struct {
	componentBase

	triggerInfo   physicConfig
	collisionInfo physicConfig

	physicsMode PhysicsMode
	mass        float64
	friction    float64
	airDrag     float64
	gravity     float64

	collisionTargets map[string]bool
}

// initialize initializes the physics component from config.
func (p *physicsComponent) initialize(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	p.componentBase.initialize(sprite, spriteCfg)
	p.initCollisionConfig(sprite, spriteCfg)
	p.initTriggerConfig(sprite, spriteCfg)

	p.physicsMode = toPhysicsMode(spriteCfg.PhysicsMode)
	p.airDrag = valueutil.OrDefault(spriteCfg.AirDrag, 1)
	p.gravity = valueutil.OrDefault(spriteCfg.Gravity, 1)
	p.friction = valueutil.OrDefault(spriteCfg.Friction, 1)
	p.mass = valueutil.OrDefault(spriteCfg.Mass, 1)
	p.collisionTargets = make(map[string]bool)
}

// initCollisionConfig initializes collision configuration.
func (p *physicsComponent) initCollisionConfig(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	p.collisionInfo.Mask = parseLayerMaskValue(spriteCfg.CollisionMask)
	p.collisionInfo.Layer = parseLayerMaskValue(spriteCfg.CollisionLayer)

	var defaultCollisionType int64 = physicsColliderNone
	if isPhysicsEnabled() {
		defaultCollisionType = physicsColliderAuto
	}

	p.collisionInfo.Type = collisionutil.ParseColliderShapeType(spriteCfg.CollisionShapeType, defaultCollisionType)
	p.collisionInfo.Pivot = spriteCfg.CollisionPivot
	p.collisionInfo.Params = spriteCfg.CollisionShapeParams

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
	p.triggerInfo.Type = collisionutil.ParseColliderShapeType(spriteCfg.TriggerShapeType, physicsColliderAuto)
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

// onDestroy cleanup when component is destroyed.
func (p *physicsComponent) onDestroy() {
	// Nothing to cleanup.
}
