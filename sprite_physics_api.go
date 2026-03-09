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
