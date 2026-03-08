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

import "github.com/goplus/spx/v2/internal/engine"

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

// syncInitPhysicInfo synchronizes physics information to the engine sprite proxy.
func (p *physicsComponent) syncInitPhysicInfo(syncProxy *engine.Sprite) {
	p.initCollisionParams()
	p.collisionInfo.syncToProxy(syncProxy, false, p.sprite)
	p.triggerInfo.syncToProxy(syncProxy, true, p.sprite)
	syncProxy.SetGravityScale(p.gravity)
	syncProxy.SetPhysicsMode(p.physicsMode)
}
