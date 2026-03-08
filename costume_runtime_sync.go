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
	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
)

// checkUpdateCostume schedules a costume update on the main thread.
func checkUpdateCostume(p *baseObj) {
	engine.WaitMainThread(func() {
		syncCheckUpdateCostume(p)
	})
}

// syncCheckUpdateCostume updates sprite costume and layer if they are dirty.
func syncCheckUpdateCostume(p *baseObj) {
	syncSprite := p.SyncSprite
	if p.IsLayerDirty {
		if !engine.HasLayerSortMethod() {
			syncSprite.SetZIndex(int64(p.Layer))
		}
		p.IsLayerDirty = false
	}
	if !p.IsCostumeDirty {
		return
	}
	p.IsCostumeDirty = false
	path := p.getCostumePath()
	renderScale := p.getCostumeRenderScale()
	if p.isCostumeAtlas() {
		rect := p.getCostumeAtlasRegion()
		syncSprite.UpdateTextureAtlas(path, rect, renderScale, !p.IsAnimating)
		syncOnAtlasChanged(p)
		return
	}
	syncSprite.UpdateTexture(path, renderScale, !p.IsAnimating)
}

func syncOnAtlasChanged(p *baseObj) {
	uvRemap := p.getCostumeAtlasUvRemap()
	val := mathf.NewVec4(uvRemap.Position.X, uvRemap.Position.Y, uvRemap.Size.X, uvRemap.Size.Y)
	p.setMaterialParamsVec4("atlas_uv_rect2", val, true)
}
