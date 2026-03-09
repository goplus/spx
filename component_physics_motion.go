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

import "github.com/goplus/spbase/mathf"

// SetPhysicsMode sets the physics mode for the sprite.
func (p *physicsComponent) SetPhysicsMode(mode PhysicsMode) {
	p.physicsMode = mode
	p.engine().SpriteMgr.SetPhysicsMode(p.sprite.getSpriteId(), int64(mode))
}

// GetPhysicsMode returns the current physics mode.
func (p *physicsComponent) GetPhysicsMode() PhysicsMode {
	return p.physicsMode
}

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

// GetGravity returns the current gravity scale.
func (p *physicsComponent) GetGravity() float64 {
	return p.engine().SpriteMgr.GetGravity(p.sprite.getSpriteId())
}

// SetGravity sets the gravity scale for the sprite.
func (p *physicsComponent) SetGravity(gravity float64) {
	p.engine().SpriteMgr.SetGravity(p.sprite.getSpriteId(), gravity)
}
