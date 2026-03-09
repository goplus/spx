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

func (p *physicsComponent) getCollisionTargets() map[string]bool {
	return p.collisionTargets
}

func (p *physicsComponent) addCollisionTarget(target string) {
	p.collisionTargets[target] = true
}
