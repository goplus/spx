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
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
)

func (p *Game) initLogicLoop() {
	gco.Create("logicLoop", p.logicLoop)
}

func (p *Game) logicLoop(coroutine.Thread) {
	tempAudios, tempAnimations := []string{}, []*animState{}
	for {
		tempAudios, tempAnimations = p.processLogicFrame(tempAudios, tempAnimations)
		engine.WaitNextFrame()
		p.showDebugPanel()
	}
}

func (p *Game) processLogicFrame(tempAudios []string, tempAnimations []*animState) ([]string, []*animState) {
	items := p.shapeMgr.getTempShapes()
	// Flush all pending audio before completing any animations.
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			tempAudios = sprite.flushPendingAudios(tempAudios)
		}
	}
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			tempAnimations = sprite.flushCompletedAnimations(tempAnimations)
		}
	}
	return tempAudios, tempAnimations
}
