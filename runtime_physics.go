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
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// syncUpdatePhysic processes physics trigger events and fires collision callbacks.
func (p *Game) syncUpdatePhysic() {
	triggers := make([]engine.TriggerEvent, 0)
	triggers = engine.GetTriggerEvents(triggers)

	for _, pair := range triggers {
		p.processTriggerPair(pair)
	}
}

// processTriggerPair processes a single physics trigger pair.
func (p *Game) processTriggerPair(pair engine.TriggerEvent) {
	srcSprite, ok1 := pair.Src.Target.(*SpriteImpl)
	dstSprite, ok2 := pair.Dst.Target.(*SpriteImpl)
	if !ok1 || !ok2 {
		spxlog.Info("Physics error: unexpected trigger pair - invalid sprite types")
		return
	}
	if !isSpriteTouchable(srcSprite) || !isSpriteTouchable(dstSprite) {
		return
	}
	srcSprite.hasOnTouchStart = true
	srcSprite.fireTouchStart(dstSprite)
}

func isSpriteTouchable(sprite *SpriteImpl) bool {
	return sprite.isVisible && !sprite.isDying
}
