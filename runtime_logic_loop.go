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
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/timer"
)

// processPendingAudios plays any pending audio for sprites.
func (p *Game) processPendingAudios(items []Shape, tempAudios []string) []string {
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			tempAudios = sprite.flushPendingAudios(tempAudios)
		}
	}
	return tempAudios
}

// processAnimationEvents handles completed animation events for sprites.
func (p *Game) processAnimationEvents(items []Shape, tempAnimations []string) []string {
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			tempAnimations = sprite.flushCompletedAnimations(tempAnimations)
		}
	}
	return tempAnimations
}

func (p *Game) logicLoop(me coroutine.Thread) int {
	tempAudios := []string{}
	tempAnimations := []string{}
	for {
		tempItems := p.getTempShapes()
		tempAudios = p.processPendingAudios(tempItems, tempAudios)
		tempAnimations = p.processAnimationEvents(tempItems, tempAnimations)

		if targetTimer, ok := timer.NextTimer(); ok {
			p.fireEvent(&eventTimer{Time: targetTimer})
		}
		engine.WaitNextFrame()
		p.showDebugPanel()
	}
}
