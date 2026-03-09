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
	"math"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
)

type PhysicsMode = int64

const (
	NoPhysics        PhysicsMode = 0 // Pure visual, no collision, best performance (current default) eg: decorators
	KinematicPhysics PhysicsMode = 1 // Code-controlled movement with collision detection eg: player
	DynamicPhysics   PhysicsMode = 2 // Affected by physics, automatic gravity and collision eg: items
	StaticPhysics    PhysicsMode = 3 // Static immovable, but has collision, affects other objects : eg: walls
)

type ColliderShapeType = int64

const (
	RectCollider      ColliderShapeType = ColliderShapeType(physicsColliderRect)
	CircleCollider    ColliderShapeType = ColliderShapeType(physicsColliderCircle)
	CapsuleCollider   ColliderShapeType = ColliderShapeType(physicsColliderCapsule)
	PolygonCollider   ColliderShapeType = ColliderShapeType(physicsColliderPolygon)
	TriggerExtraPixel float64           = 2.0
)

// toPhysicsMode converts string to PhysicsMode.
func toPhysicsMode(mode string) PhysicsMode {
	if mode == "" {
		return NoPhysics
	}
	switch mode {
	case "kinematic":
		return KinematicPhysics
	case "dynamic":
		return DynamicPhysics
	case "static":
		return StaticPhysics
	case "no":
		return NoPhysics
	}
	println("config error: unknown physics mode ", mode)
	return NoPhysics
}

// physicConfig common structure for physics configuration.
type physicConfig struct {
	Mask        int64             // collision/trigger mask
	Layer       int64             // collision/trigger layer
	Type        ColliderShapeType // collider/trigger type
	Pivot       mathf.Vec2        // pivot position
	Params      []float64         // shape parameters
	PivotOffset mathf.Vec2        // pivot offset for render offset adjustment
}

func (cfg *physicConfig) String() string {
	return fmt.Sprintf("Mask: %d, Layer: %d, Type: %d, Pivot: %v, Params: %v", cfg.Mask, cfg.Layer, cfg.Type, cfg.Pivot, cfg.Params)
}

func (cfg *physicConfig) copyFrom(src *physicConfig) {
	cfg.Mask = src.Mask
	cfg.Layer = src.Layer
	cfg.Type = src.Type
	cfg.Pivot = src.Pivot
	cfg.PivotOffset = src.PivotOffset
	cfg.Params = make([]float64, len(src.Params))
	copy(cfg.Params, src.Params)
}

// validateShape validates if shape parameters match the type.
func (cfg *physicConfig) validateShape() bool {
	if cfg.Type == physicsColliderNone || cfg.Type == physicsColliderAuto {
		return true
	}

	var expectedLen int
	var typeName string

	switch cfg.Type {
	case physicsColliderRect:
		expectedLen = 2
		typeName = "RectTrigger"
	case physicsColliderCircle:
		expectedLen = 1
		typeName = "CircleTrigger"
	case physicsColliderCapsule:
		expectedLen = 2
		typeName = "CapsuleTrigger"
	case physicsColliderPolygon:
		if len(cfg.Params) < 6 || len(cfg.Params)%2 != 0 {
			fmt.Printf("Shape validation error: PolygonTrigger requires at least 6 parameters (3 vertices) and even count, got %d\n", len(cfg.Params))
			return false
		}
		return true
	default:
		fmt.Printf("Shape validation error: Unknown trigger type: %d\n", cfg.Type)
		return false
	}

	if len(cfg.Params) != expectedLen {
		fmt.Printf("Shape validation error: %s requires exactly %d parameters, got %d\n", typeName, expectedLen, len(cfg.Params))
		return false
	}
	return true
}

// getDimensions calculates width and height based on type and shape parameters.
func (cfg *physicConfig) getDimensions() (float64, float64) {
	switch cfg.Type {
	case physicsColliderRect:
		if len(cfg.Params) >= 2 {
			return math.Max(cfg.Params[0], 0), math.Max(cfg.Params[1], 0)
		}
	case physicsColliderCircle:
		if len(cfg.Params) >= 1 {
			radius := math.Max(cfg.Params[0], 0)
			return radius * 2, radius * 2
		}
	case physicsColliderCapsule:
		if len(cfg.Params) >= 2 {
			radius := math.Max(cfg.Params[0], 0)
			height := math.Max(cfg.Params[1], 0)
			return radius * 2, height
		}
	default:
		if len(cfg.Params) >= 2 {
			return math.Max(cfg.Params[0], 0), math.Max(cfg.Params[1], 0)
		}
	}
	return 0, 0
}

// syncToProxy synchronizes physics configuration to engine proxy.
func (cfg *physicConfig) syncToProxy(syncProxy *engine.Sprite, isTrigger bool, sprite *SpriteImpl) {
	if isTrigger {
		syncProxy.SetTriggerLayer(cfg.Layer)
		syncProxy.SetTriggerMask(cfg.Mask)
		cfg.syncShape(syncProxy, true, sprite)
	} else {
		syncProxy.SetCollisionLayer(cfg.Layer)
		syncProxy.SetCollisionMask(cfg.Mask)
		cfg.syncShape(syncProxy, false, sprite)
	}
}

// syncShape synchronizes shape to engine proxy.
func (cfg *physicConfig) syncShape(syncProxy *engine.Sprite, isTrigger bool, sprite *SpriteImpl) {
	scale := sprite.Scale
	if cfg.Type != physicsColliderNone && cfg.Type != physicsColliderAuto {
		center := mathf.NewVec2(0, 0)
		applyRenderOffset(sprite, &center.X, &center.Y)
		cfg.PivotOffset = center.Divf(scale)
	}
	if cfg.Type == physicsColliderAuto {
		pivot, autoSize := syncGetCostumeBoundByAlpha(sprite)
		if isTrigger {
			autoSize.X += TriggerExtraPixel
			autoSize.Y += TriggerExtraPixel
		}
		cfg.Pivot = pivot
		cfg.Params = []float64{autoSize.X, autoSize.Y}
	}
	cfg.applyShape(syncProxy, isTrigger, scale)
}

func (cfg *physicConfig) applyShape(syncProxy *engine.Sprite, isTrigger bool, scale float64) {
	pivot := cfg.Pivot.Sub(cfg.PivotOffset)
	pivot = pivot.Mulf(scale)
	switch cfg.Type {
	case physicsColliderCircle:
		syncProxy.SetColliderEnabled(isTrigger, true)
		if len(cfg.Params) >= 1 {
			syncProxy.SetColliderShapeCircle(isTrigger, pivot, math.Max(cfg.Params[0]*scale, 0.01))
		}
	case physicsColliderRect:
		syncProxy.SetColliderEnabled(isTrigger, true)
		if len(cfg.Params) >= 2 {
			syncProxy.SetColliderShapeRect(isTrigger, pivot, mathf.NewVec2(cfg.Params[0]*scale, cfg.Params[1]*scale))
		}
	case physicsColliderCapsule:
		syncProxy.SetColliderEnabled(isTrigger, true)
		if len(cfg.Params) >= 2 {
			syncProxy.SetColliderShapeCapsule(isTrigger, pivot, mathf.NewVec2(cfg.Params[0]*scale*2, cfg.Params[1]*scale))
		}
	case physicsColliderAuto:
		syncProxy.SetColliderEnabled(isTrigger, true)
		if len(cfg.Params) >= 2 {
			syncProxy.SetColliderShapeRect(isTrigger, pivot, mathf.NewVec2(cfg.Params[0]*scale, cfg.Params[1]*scale))
		}
	case physicsColliderNone:
		syncProxy.SetColliderEnabled(isTrigger, false)
	}
}

// updatePhysicsShapesScale updates collision and trigger shapes when sprite scale changes.
func (p *SpriteImpl) updatePhysicsShapesScale() {
	physics := p.physics()
	physics.getTriggerInfo().applyShape(p.SyncSprite, true, p.Scale)
	physics.getCollisionInfo().applyShape(p.SyncSprite, false, p.Scale)
}

func (p *SpriteImpl) SetPhysicsMode(mode PhysicsMode) {
	p.physics().SetPhysicsMode(mode)
}

func (p *SpriteImpl) PhysicsMode() PhysicsMode {
	return p.physics().GetPhysicsMode()
}

func (p *SpriteImpl) Velocity() (velocityX, velocityY float64) {
	return p.physics().GetVelocity()
}

func (p *SpriteImpl) SetVelocity(velocityX, velocityY float64) {
	p.physics().SetVelocity(velocityX, velocityY)
}

func (p *SpriteImpl) AddImpulse(impulseX, impulseY float64) {
	p.physics().AddImpulse(impulseX, impulseY)
}

func (p *SpriteImpl) IsOnFloor() bool {
	return p.physics().IsOnFloor()
}

func (p *SpriteImpl) Gravity() float64 {
	return p.physics().GetGravity()
}

func (p *SpriteImpl) SetGravity(gravity float64) {
	p.physics().SetGravity(gravity)
}

func (p *SpriteImpl) SetColliderShape(isTrigger bool, ctype ColliderShapeType, params []float64) error {
	return p.physics().SetColliderShape(isTrigger, ctype, params)
}

func (p *SpriteImpl) ColliderShape(isTrigger bool) (ColliderShapeType, []float64) {
	return p.physics().GetColliderShape(isTrigger)
}

func (p *SpriteImpl) SetColliderPivot(isTrigger bool, offsetX, offsetY float64) {
	p.physics().SetColliderPivot(isTrigger, offsetX, offsetY)
}

func (p *SpriteImpl) ColliderPivot(isTrigger bool) (offsetX, offsetY float64) {
	return p.physics().GetColliderPivot(isTrigger)
}

func (p *SpriteImpl) SetCollisionLayer(layer int64) {
	p.physics().SetCollisionLayer(layer)
}

func (p *SpriteImpl) SetCollisionMask(mask int64) {
	p.physics().SetCollisionMask(mask)
}

func (p *SpriteImpl) SetCollisionEnabled(enabled bool) {
	p.physics().SetCollisionEnabled(enabled)
}

func (p *SpriteImpl) CollisionLayer() int64 {
	return p.physics().GetCollisionLayer()
}

func (p *SpriteImpl) CollisionMask() int64 {
	return p.physics().GetCollisionMask()
}

func (p *SpriteImpl) CollisionEnabled() bool {
	return p.physics().IsCollisionEnabled()
}

func (p *SpriteImpl) SetTriggerEnabled(trigger bool) {
	p.physics().SetTriggerEnabled(trigger)
}

func (p *SpriteImpl) SetTriggerLayer(layer int64) {
	p.physics().SetTriggerLayer(layer)
}

func (p *SpriteImpl) SetTriggerMask(mask int64) {
	p.physics().SetTriggerMask(mask)
}

func (p *SpriteImpl) TriggerLayer() int64 {
	return p.physics().GetTriggerLayer()
}

func (p *SpriteImpl) TriggerMask() int64 {
	return p.physics().GetTriggerMask()
}

func (p *SpriteImpl) TriggerEnabled() bool {
	return p.physics().IsTriggerEnabled()
}
