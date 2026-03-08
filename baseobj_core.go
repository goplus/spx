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

// baseObj provides common functionality for sprites and backdrops.
type baseObj struct {
	costumes     []*costume
	costumeIndex int

	// Rendering state
	syncSprite     *engine.Sprite // !!!All methods (except GetId()) can only be called on main thread
	scale          float64
	HasDestroyed   bool
	isCostumeSet   bool
	isCostumeDirty bool

	// Layer management
	layer        int
	isLayerDirty bool

	// Effects
	greffUniforms map[EffectKind]float64 // graphic effects uniforms
	hasShader     bool

	// Animation state
	isAnimating bool
}

// getSpriteId returns the unique identifier for this sprite.
func (p *baseObj) getSpriteId() engine.Object {
	return p.syncSprite.GetId()
}

// getProxy returns the underlying engine sprite.
func (p *baseObj) getProxy() *engine.Sprite {
	return p.syncSprite
}

// setLayer sets the layer/z-order of the object.
func (p *baseObj) setLayer(layer int) {
	if p.layer != layer {
		p.layer = layer
		p.isLayerDirty = true
	}
}

// setCostumeIndex sets the current costume by index.
func (p *baseObj) setCostumeIndex(value int) {
	p.costumeIndex = value
	p.isCostumeDirty = true
	p.isAnimating = false
}
