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
	gtime "github.com/goplus/spx/v2/internal/time"
)

// flushActivate updates all pending shapes in the active list.
func (sm *spriteManager) flushActivate() {
	if len(sm.tempItems) == 0 {
		return
	}

	delta := gtime.DeltaTime()
	for _, item := range sm.tempItems {
		if _, ok := item.(*SpriteImpl); ok {
			continue
		}

		switch v := item.(type) {
		case *quoterBubble:
			v.onUpdate(delta)
		case *textBubble:
			v.onUpdate(delta)
		case *Monitor:
			v.onUpdate(delta)
		default:
			if updater, ok := item.(interface{ onUpdate(float64) }); ok {
				updater.onUpdate(delta)
			}
		}
	}
}

func (sm *spriteManager) syncProxyStates(buffer *engine.SpriteSyncBuffer) {
	for _, item := range sm.getTempShapes() {
		if sprite, ok := item.(*SpriteImpl); ok {
			sprite.syncProxyState(buffer)
		}
	}
}

// flushDestroy performs cleanup for shapes that have been scheduled for destruction.
func (sm *spriteManager) flushDestroy(buffer *engine.SpriteSyncBuffer) {
	if len(sm.destroyItems) == 0 {
		return
	}

	for _, item := range sm.destroyItems {
		if sprite, ok := item.(*SpriteImpl); ok && sprite.syncSprite != nil {
			buffer.AddDelete(int64(sprite.syncSprite.Id))
			sprite.syncSprite = nil
		}
	}

	sm.destroyItems = sm.destroyItems[:0]
}
