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
