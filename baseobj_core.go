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
	corestate "github.com/goplus/spx/v2/internal/core/state"
	"github.com/goplus/spx/v2/internal/engine"
)

// baseObj provides common functionality for sprites and backdrops.
type baseObj struct {
	corestate.BaseObjRuntimeState

	costumes     []*costume
	costumeIndex int

	// Effects
	greffUniforms map[EffectKind]float64 // graphic effects uniforms
}

// getSpriteId returns the unique identifier for this sprite.
func (p *baseObj) getSpriteId() engine.Object {
	return p.SyncSprite.GetId()
}

// getProxy returns the underlying engine sprite.
func (p *baseObj) getProxy() *engine.Sprite {
	return p.SyncSprite
}

// setLayer sets the layer/z-order of the object.
func (p *baseObj) setLayer(layer int) {
	if p.Layer != layer {
		p.Layer = layer
		p.IsLayerDirty = true
	}
}

// setCostumeIndex sets the current costume by index.
func (p *baseObj) setCostumeIndex(value int) {
	p.costumeIndex = value
	p.IsCostumeDirty = true
	p.IsAnimating = false
}
