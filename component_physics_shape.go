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
)

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
	if p.sprite.SyncSprite != nil {
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
	ctype := config.Type
	params := config.Params

	if p.sprite.SyncSprite == nil {
		return
	}

	switch ctype {
	case RectCollider:
		if len(params) >= 2 {
			p.sprite.SyncSprite.SetColliderShapeRect(isTrigger, config.Pivot, mathf.NewVec2(params[0], params[1]))
		}
	case CircleCollider:
		if len(params) >= 1 {
			p.sprite.SyncSprite.SetColliderShapeCircle(isTrigger, config.Pivot, params[0])
		}
	case CapsuleCollider:
		if len(params) >= 2 {
			p.sprite.SyncSprite.SetColliderShapeCapsule(isTrigger, config.Pivot, mathf.NewVec2(params[0]*2, params[1]))
		}
	case PolygonCollider:
		if len(params) >= 6 {
			p.sprite.SyncSprite.SetColliderShapePolygon(isTrigger, config.Pivot, params)
		}
	}
}
