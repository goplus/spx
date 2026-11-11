/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"maps"
	"math"
	"slices"

	"github.com/goplus/spx/v4/internal/engine"
)

func (p *SpriteImpl) isSpineMode() bool {
	animation := p.animation()
	return animation != nil && animation.shared != nil && animation.shared.spine != nil
}

// initSpineProxy runs before physics initialization so automatic colliders use
// the setup pose, while explicit shapes keep their configured dimensions.
func (p *SpriteImpl) initSpineProxy() {
	if !p.isSpineMode() {
		return
	}
	if !engine.Managers().SpriteMgr.IsSpineSupported() {
		panic("Spine support is disabled in this runtime; rebuild with SPX_SPINE=1 using an isolated GOPATH")
	}
	cfg := p.animation().shared.spine
	proxy := p.runtimeState.syncSprite
	if !proxy.SetSpineSkeleton(engine.ToAssetPath(cfg.Atlas), engine.ToAssetPath(cfg.Skeleton), cfg.DefaultMix) {
		panic(fmt.Errorf("load Spine skeleton for sprite %q: %s", p.name, cfg.Skeleton))
	}
	// Reject invalid aliases before a default loop starts. Otherwise a failed
	// PlayAnim could leave that loop running and AnimateAndWait would never end.
	for _, name := range slices.Sorted(maps.Keys(p.animation().shared.animations)) {
		nativeName := p.animation().playbackName(name)
		if !proxy.HasSpineAnimation(nativeName) {
			panic(fmt.Errorf("sprite %q animation %q references missing Spine animation %q", p.name, name, nativeName))
		}
	}
	p.animation().spineBounds = proxy.GetSpineBounds()
	size := p.animation().spineBounds.Size
	// Clones share their original costumes. Replace this slice rather than
	// changing a shared descriptor when a new native skeleton is initialized.
	p.costumes = []*costume{newCostumeWithSize(int(math.Ceil(size.X)), int(math.Ceil(size.Y)))}
	p.costumes[0].frame.ImageSize = size
	proxy.SetRenderScale(engine.UniformVec2(p.runtimeState.scale))
}
