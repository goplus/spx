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

import spxlog "github.com/goplus/spx/v2/internal/log"

func (p *SpriteImpl) Die() {
	aniName := p.getStateAnimName(StateDie)
	p.setDying()

	p.Stop(OtherScriptsInSprite)
	if p.hasAnim(aniName) {
		p.AnimateAndWait(aniName)
	}
	p.Destroy()
}

// Destroy destroys sprite, whether prototype or cloned.
func (p *SpriteImpl) Destroy() {
	if isDebugInstrEnabled() {
		spxlog.Debug("Destroy: %s", p.name)
	}
	p.Hide()
	p.doDeleteClone()
	if p.runtimeState.SyncSprite != nil {
		p.g.inputMgr.removeClickTarget(p.runtimeState.SyncSprite.GetId())
	}
	p.components.destroyComponents()
	p.g.removeShape(p)
	p.Stop(ThisSprite)
	if p == gco.Current().Obj {
		gco.Abort()
	}
	p.markDestroyed()
}

// DeleteThisClone deletes only cloned sprite, no effect on prototype sprite.
// Add this interface to match Scratch.
func (p *SpriteImpl) DeleteThisClone() {
	if !p.spriteState.Cloned {
		return
	}
	p.Destroy()
}

func (p *SpriteImpl) TouchingColor(color Color) bool {
	return p.touchingColor(toMathfColor(color))
}

func (p *SpriteImpl) Touching__0(sprite SpriteName) bool {
	return p.touching(sprite)
}

func (p *SpriteImpl) Touching__1(sprite Sprite) bool {
	return p.touching(sprite)
}

func (p *SpriteImpl) Touching__2(obj specialObj) bool {
	return p.touching(obj)
}
