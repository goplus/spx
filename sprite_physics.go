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
	"slices"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/base"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/core/collision"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
)

const minCircleRadius = 0.01

type PhysicsMode = int64

const (
	NoPhysics        PhysicsMode = 0 // No physical response; touch sensing remains available.
	KinematicPhysics PhysicsMode = 1 // Script-controlled movement with collision response.
	DynamicPhysics   PhysicsMode = 2 // Movement driven by forces and gravity.
	StaticPhysics    PhysicsMode = 3 // Fixed body that blocks other bodies.
)

type ColliderShapeType = int64

const (
	RectCollider      ColliderShapeType = collision.ColliderRect
	CircleCollider    ColliderShapeType = collision.ColliderCircle
	CapsuleCollider   ColliderShapeType = collision.ColliderCapsule
	PolygonCollider   ColliderShapeType = collision.ColliderPolygon
	TriggerExtraPixel float64           = 2.0
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

type colliderConfig struct {
	Mask   int64
	Layer  int64
	Type   ColliderShapeType
	Pivot  mathf.Vec2
	Params []float64
}

func (p *SpriteImpl) SetPhysicsMode(mode PhysicsMode) {
	p.physics().setPhysicsMode(mode)
}

func (p *SpriteImpl) PhysicsMode() PhysicsMode {
	return p.physics().getPhysicsMode()
}

func (p *SpriteImpl) SetVelocity(vx, vy float64) {
	p.physics().setVelocity(vx, vy)
}

func (p *SpriteImpl) Velocity() (velocityX, velocityY float64) {
	return p.physics().getVelocity()
}

// AddImpulse applies a one-shot momentum change on the next dynamic physics tick.
// Velocity changes by impulse / mass, independently of the physics tick rate.
// Coordinates use positive X to the right and positive Y upward.
func (p *SpriteImpl) AddImpulse(ix, iy float64) {
	p.physics().addImpulse(ix, iy)
}

func (p *SpriteImpl) IsOnFloor() bool {
	return p.physics().isOnFloor()
}

func (p *SpriteImpl) SetGravity(g float64) {
	p.physics().setGravity(g)
}

func (p *SpriteImpl) Gravity() float64 {
	return p.physics().getGravity()
}

func (p *SpriteImpl) SetCollisionLayer(layer int64) {
	p.physics().setCollisionLayer(layer)
}

func (p *SpriteImpl) CollisionLayer() int64 {
	return p.physics().getCollisionLayer()
}

func (p *SpriteImpl) SetCollisionMask(mask int64) {
	p.physics().setCollisionMask(mask)
}

func (p *SpriteImpl) CollisionMask() int64 {
	return p.physics().getCollisionMask()
}

func (p *SpriteImpl) SetCollisionEnabled(enabled bool) {
	p.physics().setCollisionEnabled(enabled)
}

func (p *SpriteImpl) CollisionEnabled() bool {
	return p.physics().isCollisionEnabled()
}

func (p *SpriteImpl) SetTriggerLayer(layer int64) {
	p.physics().setTriggerLayer(layer)
}

func (p *SpriteImpl) TriggerLayer() int64 {
	return p.physics().getTriggerLayer()
}

func (p *SpriteImpl) SetTriggerMask(mask int64) {
	p.physics().setTriggerMask(mask)
}

func (p *SpriteImpl) TriggerMask() int64 {
	return p.physics().getTriggerMask()
}

func (p *SpriteImpl) SetTriggerEnabled(enabled bool) {
	p.physics().setTriggerEnabled(enabled)
}

func (p *SpriteImpl) TriggerEnabled() bool {
	return p.physics().isTriggerEnabled()
}

func (p *SpriteImpl) SetColliderShape(isTrigger bool, shapeType ColliderShapeType, params []float64) error {
	return p.physics().setColliderShape(isTrigger, shapeType, params)
}

func (p *SpriteImpl) ColliderShape(isTrigger bool) (ColliderShapeType, []float64) {
	return p.physics().getColliderShape(isTrigger)
}

func (p *SpriteImpl) SetColliderPivot(isTrigger bool, offsetX, offsetY float64) {
	p.physics().setColliderPivot(isTrigger, offsetX, offsetY)
}

func (p *SpriteImpl) ColliderPivot(isTrigger bool) (offsetX, offsetY float64) {
	return p.physics().getColliderPivot(isTrigger)
}

func (p *SpriteImpl) markAutoPhysicsShapesDirty() {
	if physics := p.physics(); physics != nil {
		physics.autoShapesDirty = true
	}
}

func (p *SpriteImpl) syncAutoPhysicsShapesAfterCostumeChange() {
	if physics := p.physics(); physics != nil {
		physics.refreshAutoShapes(p.runtimeState.proxy)
	}
}

func (p *SpriteImpl) updatePhysicsShapesScale() {
	if p.runtimeState.proxy == nil {
		return
	}
	physics := p.physics()
	physics.triggerCollider.syncShape(p.runtimeState.proxy, true, p)
	physics.bodyCollider.syncShape(p.runtimeState.proxy, false, p)
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

func (p *physicsComponent) setPhysicsMode(mode PhysicsMode) {
	p.physicsMode = mode
	engine.Managers().SpriteMgr.SetPhysicsMode(p.sprite.proxyID(), mode)
}

func (p *physicsComponent) getPhysicsMode() PhysicsMode {
	return p.physicsMode
}

func (p *physicsComponent) setVelocity(velocityX, velocityY float64) {
	engine.Managers().SpriteMgr.SetVelocity(p.sprite.proxyID(), mathf.NewVec2(velocityX, velocityY))
}

func (p *physicsComponent) getVelocity() (velocityX, velocityY float64) {
	vel := engine.Managers().SpriteMgr.GetVelocity(p.sprite.proxyID())
	return vel.X, vel.Y
}

func (p *physicsComponent) addImpulse(impulseX, impulseY float64) {
	engine.Managers().SpriteMgr.AddImpulse(p.sprite.proxyID(), mathf.NewVec2(impulseX, impulseY))
}

func (p *physicsComponent) isOnFloor() bool {
	return engine.Managers().SpriteMgr.IsOnFloor(p.sprite.proxyID())
}

func (p *physicsComponent) setGravity(gravity float64) {
	engine.Managers().SpriteMgr.SetGravity(p.sprite.proxyID(), gravity)
}

func (p *physicsComponent) getGravity() float64 {
	return engine.Managers().SpriteMgr.GetGravity(p.sprite.proxyID())
}

func (p *physicsComponent) setCollisionLayer(layer int64) {
	p.bodyCollider.Layer = layer
	p.sprite.runtimeState.proxy.SetCollisionLayer(layer)
}

func (p *physicsComponent) getCollisionLayer() int64 {
	return p.sprite.runtimeState.proxy.GetCollisionLayer()
}

func (p *physicsComponent) setCollisionMask(mask int64) {
	p.bodyCollider.Mask = mask
	p.sprite.runtimeState.proxy.SetCollisionMask(mask)
}

func (p *physicsComponent) getCollisionMask() int64 {
	return p.sprite.runtimeState.proxy.GetCollisionMask()
}

func (p *physicsComponent) setCollisionEnabled(enabled bool) {
	p.sprite.runtimeState.proxy.SetCollisionEnabled(enabled)
}

func (p *physicsComponent) isCollisionEnabled() bool {
	return p.sprite.runtimeState.proxy.IsCollisionEnabled()
}

func (p *physicsComponent) setTriggerLayer(layer int64) {
	p.triggerCollider.Layer = layer
	p.sprite.runtimeState.proxy.SetTriggerLayer(layer)
}

func (p *physicsComponent) getTriggerLayer() int64 {
	return p.sprite.runtimeState.proxy.GetTriggerLayer()
}

func (p *physicsComponent) setTriggerMask(mask int64) {
	p.triggerCollider.Mask = mask
	p.sprite.runtimeState.proxy.SetTriggerMask(mask)
}

func (p *physicsComponent) getTriggerMask() int64 {
	return p.sprite.runtimeState.proxy.GetTriggerMask()
}

func (p *physicsComponent) setTriggerEnabled(enabled bool) {
	p.sprite.runtimeState.proxy.SetTriggerEnabled(enabled)
}

func (p *physicsComponent) isTriggerEnabled() bool {
	return p.sprite.runtimeState.proxy.IsTriggerEnabled()
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
	if proxy := p.sprite.runtimeState.proxy; proxy != nil {
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
	if proxy := p.sprite.runtimeState.proxy; proxy != nil {
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

func (cfg colliderConfig) clone() colliderConfig {
	cfg.Params = slices.Clone(cfg.Params)
	return cfg
}

func (cfg *colliderConfig) initializeShape(proxy *engine.Sprite, isTrigger bool, sprite *SpriteImpl) {
	if cfg.Type == collision.ColliderAuto {
		cfg.updateAutoBounds(sprite, isTrigger)
	}
	proxy.SetColliderEnabled(isTrigger, cfg.defaultEnabled(isTrigger, sprite))
	cfg.syncShape(proxy, isTrigger, sprite)
}

func (cfg *colliderConfig) refreshAutoShape(proxy *engine.Sprite, isTrigger bool, sprite *SpriteImpl) {
	if cfg.Type != collision.ColliderAuto || len(cfg.Params) < 2 {
		return
	}
	cfg.updateAutoBounds(sprite, isTrigger)
	cfg.syncShape(proxy, isTrigger, sprite)
}

func (cfg *colliderConfig) syncShape(proxy *engine.Sprite, isTrigger bool, sprite *SpriteImpl) {
	pivot := cfg.shapePivot(sprite)
	scale := sprite.runtimeState.scale

	switch cfg.Type {
	case collision.ColliderNone:
		proxy.ClearColliderShape(isTrigger)
	case collision.ColliderCircle:
		if len(cfg.Params) >= 1 {
			proxy.SetColliderShapeCircle(isTrigger, pivot, math.Max(cfg.Params[0]*scale, minCircleRadius))
		}
	case collision.ColliderRect, collision.ColliderAuto:
		if len(cfg.Params) >= 2 {
			proxy.SetColliderShapeRect(isTrigger, pivot, mathf.NewVec2(cfg.Params[0]*scale, cfg.Params[1]*scale))
		}
	case collision.ColliderCapsule:
		if len(cfg.Params) >= 2 {
			proxy.SetColliderShapeCapsule(isTrigger, pivot, mathf.NewVec2(cfg.Params[0]*scale*2, cfg.Params[1]*scale))
		}
	case collision.ColliderPolygon:
		if len(cfg.Params) >= 6 && len(cfg.Params)%2 == 0 {
			points := make([]float64, len(cfg.Params))
			for i, coordinate := range cfg.Params {
				points[i] = coordinate * scale
			}
			proxy.SetColliderShapePolygon(isTrigger, pivot, points)
		}
	}
}

func (cfg *colliderConfig) defaultEnabled(isTrigger bool, sprite *SpriteImpl) bool {
	// Pixel notifications need no trigger geometry.
	return cfg.Type != collision.ColliderNone || isTrigger && !sprite.g.touchByShape
}

func (cfg *colliderConfig) updateAutoBounds(sprite *SpriteImpl, isTrigger bool) {
	pivot, size := getCostumeBoundByAlpha(sprite)
	if isTrigger {
		size.X += TriggerExtraPixel
		size.Y += TriggerExtraPixel
	}
	cfg.Pivot = pivot
	cfg.Params = []float64{size.X, size.Y}
}

func (cfg *colliderConfig) shapePivot(sprite *SpriteImpl) mathf.Vec2 {
	pivot := cfg.Pivot.Mulf(sprite.runtimeState.scale)
	if cfg.Type == collision.ColliderAuto {
		offsetX, offsetY := getRenderOffset(sprite)
		pivot = pivot.Add(mathf.NewVec2(offsetX, offsetY))
	}
	return pivot
}

func (cfg *colliderConfig) worldBounds(sprite *SpriteImpl) (collision.AABB, bool) {
	local, ok := cfg.localBounds()
	if !ok || !sprite.hasCostume() {
		return collision.AABB{}, false
	}

	rotation, flipX, flipY := renderRotationAndFlip(sprite)
	sin, cos := math.Sincos(-engine.DegToRad(rotation))
	scale := sprite.runtimeState.scale
	pivot := cfg.shapePivot(sprite)
	centerX := (pivot.X + (local.MinX+local.MaxX)*scale/2) * flipX
	centerY := (pivot.Y + (local.MinY+local.MaxY)*scale/2) * flipY
	x, y := sprite.getXY()
	x += centerX*cos - centerY*sin
	y += centerX*sin + centerY*cos
	halfX := math.Abs((local.MaxX-local.MinX)*scale) / 2
	halfY := math.Abs((local.MaxY-local.MinY)*scale) / 2
	extentX := math.Abs(cos)*halfX + math.Abs(sin)*halfY
	extentY := math.Abs(sin)*halfX + math.Abs(cos)*halfY
	if cfg.Type == collision.ColliderCircle {
		extentX = math.Max(cfg.Params[0]*scale, minCircleRadius)
		extentY = extentX
	}
	return collision.AABB{MinX: x - extentX, MinY: y - extentY, MaxX: x + extentX, MaxY: y + extentY}, true
}

func (cfg *colliderConfig) localBounds() (collision.AABB, bool) {
	var minX, minY, maxX, maxY float64
	switch cfg.Type {
	case collision.ColliderAuto, collision.ColliderRect, collision.ColliderCircle, collision.ColliderCapsule:
		width, height := cfg.getDimensions()
		if width <= 0 || height <= 0 {
			return collision.AABB{}, false
		}
		if cfg.Type == collision.ColliderCapsule && height < width {
			height = width
		}
		minX, maxX = -width/2, width/2
		minY, maxY = -height/2, height/2
	case collision.ColliderPolygon:
		if len(cfg.Params) < 6 || len(cfg.Params)%2 != 0 {
			return collision.AABB{}, false
		}
		minX, maxX = cfg.Params[0], cfg.Params[0]
		minY, maxY = cfg.Params[1], cfg.Params[1]
		for i := 2; i < len(cfg.Params); i += 2 {
			minX, maxX = math.Min(minX, cfg.Params[i]), math.Max(maxX, cfg.Params[i])
			minY, maxY = math.Min(minY, cfg.Params[i+1]), math.Max(maxY, cfg.Params[i+1])
		}
	default:
		return collision.AABB{}, false
	}
	return collision.AABB{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}, true
}

func (cfg *colliderConfig) getDimensions() (float64, float64) {
	switch cfg.Type {
	case collision.ColliderCircle:
		if len(cfg.Params) >= 1 {
			radius := math.Max(cfg.Params[0], 0)
			return radius * 2, radius * 2
		}
	case collision.ColliderCapsule:
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

func (cfg *colliderConfig) validateShape() error {
	var expectedLen int
	var typeName string

	switch cfg.Type {
	case collision.ColliderNone, collision.ColliderAuto:
		return nil
	case collision.ColliderRect:
		expectedLen = 2
		typeName = "rect"
	case collision.ColliderCircle:
		expectedLen = 1
		typeName = "circle"
	case collision.ColliderCapsule:
		expectedLen = 2
		typeName = "capsule"
	case collision.ColliderPolygon:
		if len(cfg.Params) < 6 || len(cfg.Params)%2 != 0 {
			return fmt.Errorf("invalid polygon: want at least 3 coordinate pairs, got %d values", len(cfg.Params))
		}
		return nil
	default:
		return fmt.Errorf("unknown collider shape type: %d", cfg.Type)
	}

	if len(cfg.Params) != expectedLen {
		return fmt.Errorf("invalid %s parameter count: got %d, want %d", typeName, len(cfg.Params), expectedLen)
	}
	return nil
}

func toPhysicsMode(mode string) PhysicsMode {
	switch mode {
	case "", "no":
		return NoPhysics
	case "kinematic":
		return KinematicPhysics
	case "dynamic":
		return DynamicPhysics
	case "static":
		return StaticPhysics
	}
	spxlog.Warn("Config error: unknown physics mode %s", mode)
	return NoPhysics
}
