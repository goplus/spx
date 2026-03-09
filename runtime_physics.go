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
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// syncUpdatePhysic processes physics trigger events and fires collision callbacks.
func (p *Game) syncUpdatePhysic() {
	triggers := make([]engine.TriggerEvent, 0)
	triggers = engine.GetTriggerEvents(triggers)
	coreruntime.ProcessTriggerPairs(
		triggers,
		func(target any) (*SpriteImpl, bool) {
			sprite, ok := target.(*SpriteImpl)
			return sprite, ok
		},
		isSpriteTouchable,
		func(srcSprite, dstSprite *SpriteImpl) {
			srcSprite.HasOnTouchStart = true
			srcSprite.fireTouchStart(dstSprite)
		},
		func() {
			spxlog.Info("Physics error: unexpected trigger pair - invalid sprite types")
		},
	)
}

func isSpriteTouchable(sprite *SpriteImpl) bool {
	return sprite.IsVisible && !sprite.IsDying
}
