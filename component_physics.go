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

func parseLayerMaskValue(pval *int64) int64 {
	return defaults.OrDefault(pval, 1)
}

// physicsComponent encapsulates all physics-related functionality.
type physicsComponent struct {
	componentBase

	triggerInfo   physicConfig
	collisionInfo physicConfig

	physicsMode            PhysicsMode
	collisionEnabled       bool
	triggerEnabled         bool
	pendingVelocity        mathf.Vec2
	pendingPhysicsCommands []pendingPhysicsCommand
	mass                   float64
	friction               float64
	airDrag                float64
	gravity                float64
	autoShapesDirty        bool

	collisionTargets map[string]bool
}

type pendingPhysicsCommandKind uint8

const (
	pendingPhysicsSetMode pendingPhysicsCommandKind = iota
	pendingPhysicsSetVelocity
	pendingPhysicsAddImpulse
)

type pendingPhysicsCommand struct {
	kind pendingPhysicsCommandKind
	mode PhysicsMode
	vec  mathf.Vec2
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
	p.collisionEnabled = p.collisionInfo.Type != physicsColliderNone
	p.triggerEnabled = p.triggerInfo.Type != physicsColliderNone
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
		collisionEnabled: srcPhysics.collisionEnabled,
		triggerEnabled:   srcPhysics.triggerEnabled,
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
	// Nothing to cleanup.
}

// ============================================================================
// Physics Control
// ============================================================================

// SetPhysicsMode sets the physics mode for the sprite.
func (p *physicsComponent) SetPhysicsMode(mode PhysicsMode) {
	p.physicsMode = mode
	if p.sprite.spriteState.IsProxyPublicationPending {
		p.pendingPhysicsCommands = append(p.pendingPhysicsCommands, pendingPhysicsCommand{
			kind: pendingPhysicsSetMode,
			mode: mode,
		})
		// Godot synchronously clears velocity whenever NoPhysics is selected.
		if mode == NoPhysics {
			p.pendingVelocity = mathf.Vec2{}
		}
		return
	}
	if syncProxy := p.sprite.runtimeState.SyncSprite; syncProxy != nil {
		syncProxy.SetPhysicsMode(mode)
	}
}

// GetPhysicsMode returns the current physics mode.
func (p *physicsComponent) GetPhysicsMode() PhysicsMode {
	return p.physicsMode
}

// GetVelocity returns the current velocity in X and Y directions.
func (p *physicsComponent) GetVelocity() (velocityX, velocityY float64) {
	if p.sprite.spriteState.IsProxyPublicationPending {
		return p.pendingVelocity.X, p.pendingVelocity.Y
	}
	vel := p.engine().SpriteMgr.GetVelocity(p.sprite.getSpriteId())
	return vel.X, vel.Y
}

// SetVelocity sets the velocity in X and Y directions.
func (p *physicsComponent) SetVelocity(velocityX, velocityY float64) {
	velocity := mathf.NewVec2(velocityX, velocityY)
	if p.sprite.spriteState.IsProxyPublicationPending {
		p.pendingVelocity = velocity
		p.pendingPhysicsCommands = append(p.pendingPhysicsCommands, pendingPhysicsCommand{
			kind: pendingPhysicsSetVelocity,
			vec:  velocity,
		})
		return
	}
	p.engine().SpriteMgr.SetVelocity(p.sprite.getSpriteId(), velocity)
}

// AddImpulse applies an impulse force to the sprite.
func (p *physicsComponent) AddImpulse(impulseX, impulseY float64) {
	impulse := mathf.NewVec2(impulseX, impulseY)
	if p.sprite.spriteState.IsProxyPublicationPending {
		p.pendingPhysicsCommands = append(p.pendingPhysicsCommands, pendingPhysicsCommand{
			kind: pendingPhysicsAddImpulse,
			vec:  impulse,
		})
		return
	}
	p.engine().SpriteMgr.AddImpulse(p.sprite.getSpriteId(), impulse)
}

// IsOnFloor checks if the sprite is on the floor.
func (p *physicsComponent) IsOnFloor() bool {
	return p.engine().SpriteMgr.IsOnFloor(p.sprite.getSpriteId())
}

// GetGravity returns the current gravity scale.
func (p *physicsComponent) GetGravity() float64 {
	return p.engine().SpriteMgr.GetGravity(p.sprite.getSpriteId())
}

// SetGravity sets the gravity scale for the sprite.
func (p *physicsComponent) SetGravity(gravity float64) {
	p.engine().SpriteMgr.SetGravity(p.sprite.getSpriteId(), gravity)
}

func (p *physicsComponent) SetCollisionLayer(layer int64) {
	p.sprite.runtimeState.SyncSprite.SetCollisionLayer(layer)
}

func (p *physicsComponent) SetCollisionMask(mask int64) {
	p.sprite.runtimeState.SyncSprite.SetCollisionMask(mask)
}

func (p *physicsComponent) SetCollisionEnabled(enabled bool) {
	p.collisionEnabled = enabled
	if syncProxy := p.sprite.runtimeState.SyncSprite; syncProxy != nil {
		syncProxy.SetCollisionEnabled(p.effectiveColliderEnabled(false))
	}
}

func (p *physicsComponent) GetCollisionLayer() int64 {
	return p.sprite.runtimeState.SyncSprite.GetCollisionLayer()
}

func (p *physicsComponent) GetCollisionMask() int64 {
	return p.sprite.runtimeState.SyncSprite.GetCollisionMask()
}

func (p *physicsComponent) IsCollisionEnabled() bool {
	return p.desiredColliderEnabled(false)
}

func (p *physicsComponent) SetTriggerEnabled(trigger bool) {
	p.triggerEnabled = trigger
	if syncProxy := p.sprite.runtimeState.SyncSprite; syncProxy != nil {
		syncProxy.SetTriggerEnabled(p.effectiveColliderEnabled(true))
	}
}

func (p *physicsComponent) SetTriggerLayer(layer int64) {
	p.sprite.runtimeState.SyncSprite.SetTriggerLayer(layer)
}

func (p *physicsComponent) SetTriggerMask(mask int64) {
	p.sprite.runtimeState.SyncSprite.SetTriggerMask(mask)
}

func (p *physicsComponent) GetTriggerLayer() int64 {
	return p.sprite.runtimeState.SyncSprite.GetTriggerLayer()
}

func (p *physicsComponent) GetTriggerMask() int64 {
	return p.sprite.runtimeState.SyncSprite.GetTriggerMask()
}

func (p *physicsComponent) IsTriggerEnabled() bool {
	return p.desiredColliderEnabled(true)
}

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
// Collider Configuration
// ============================================================================

// SetColliderShape sets the collider shape type and parameters.
func (p *physicsComponent) SetColliderShape(isTrigger bool, ctype ColliderShapeType, params []float64) error {
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

	p.setColliderEnabledState(isTrigger, ctype != physicsColliderNone)
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
	if p.sprite.runtimeState.SyncSprite != nil {
		p.applyPhysicShape(isTrigger)
	}
}

func (p *physicsComponent) GetColliderPivot(isTrigger bool) (offsetX, offsetY float64) {
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
	config.applyShape(
		p.sprite.runtimeState.SyncSprite,
		isTrigger,
		p.sprite,
		p.effectiveColliderEnabled(isTrigger),
	)
}

func (p *physicsComponent) setColliderEnabledState(isTrigger, enabled bool) {
	if isTrigger {
		p.triggerEnabled = enabled
		return
	}
	p.collisionEnabled = enabled
}

func (p *physicsComponent) desiredColliderEnabled(isTrigger bool) bool {
	if isTrigger {
		return p.triggerEnabled
	}
	return p.collisionEnabled
}

func (p *physicsComponent) activatableColliderEnabled(isTrigger bool) bool {
	return p.getPhysicConfig(isTrigger).Type != physicsColliderNone &&
		p.desiredColliderEnabled(isTrigger)
}

func (p *physicsComponent) effectiveColliderEnabled(isTrigger bool) bool {
	return !p.sprite.spriteState.IsProxyPublicationPending &&
		p.activatableColliderEnabled(isTrigger)
}

func (p *physicsComponent) effectivePhysicsMode() PhysicsMode {
	if p.sprite.spriteState.IsProxyPublicationPending {
		return NoPhysics
	}
	return p.physicsMode
}

// beginPendingProxyPhysics starts a logical command stream for a fresh clone.
// Replaying it at commit preserves native call ordering without allowing the
// unpublished proxy to move or collide in the meantime.
func (p *physicsComponent) beginPendingProxyPhysics() {
	p.pendingVelocity = mathf.Vec2{}
	p.pendingPhysicsCommands = append(
		p.pendingPhysicsCommands[:0],
		pendingPhysicsCommand{kind: pendingPhysicsSetMode, mode: p.physicsMode},
	)
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
	p.collisionInfo.syncToProxy(
		syncProxy, false, p.sprite, p.effectiveColliderEnabled(false),
	)
	p.triggerInfo.syncToProxy(
		syncProxy, true, p.sprite, p.effectiveColliderEnabled(true),
	)
	syncProxy.SetGravityScale(p.gravity)
	syncProxy.SetPhysicsMode(p.effectivePhysicsMode())
	p.autoShapesDirty = false
}

// publishPendingProxyPhysics activates the final physics configuration. The
// caller owns the clone publication barrier and must compensate this work if
// the surrounding transaction does not commit.
func (p *physicsComponent) publishPendingProxyPhysics(syncProxy *engine.Sprite) {
	commands := p.pendingPhysicsCommands
	p.pendingVelocity = mathf.Vec2{}
	p.pendingPhysicsCommands = nil

	syncProxy.SetCollisionEnabled(p.activatableColliderEnabled(false))
	syncProxy.SetTriggerEnabled(p.activatableColliderEnabled(true))
	// Keep the body out of the physics world until both shapes are ready, then
	// replay the exact mode/velocity/impulse order observed by clone user code.
	for _, command := range commands {
		switch command.kind {
		case pendingPhysicsSetMode:
			syncProxy.SetPhysicsMode(command.mode)
		case pendingPhysicsSetVelocity:
			syncProxy.SetVelocity(command.vec)
		case pendingPhysicsAddImpulse:
			syncProxy.AddImpulse(command.vec)
		}
	}
}
