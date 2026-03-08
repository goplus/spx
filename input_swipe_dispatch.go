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
	inputstate "github.com/goplus/spx/v2/internal/input"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

func (p *inputManager) beginSwipeTracking(startPos mathf.Vec2, targetSprite *SpriteImpl) {
	p.swipeTarget = targetSprite
	p.swipeRecognizer.StartTracking(startPos)
}

func (p *inputManager) finishSwipeTracking(point mathf.Vec2) {
	swiper := &p.swipeRecognizer
	if !swiper.IsTracking() {
		return
	}
	targetSprite := p.swipeTarget
	p.swipeTarget = nil
	result, ok := swiper.Finish(point)
	if !ok {
		return
	}
	p.dispatchSwipeResult(result, targetSprite)
}

func (p *inputManager) onMouseMove(pos mathf.Vec2) {
	if !p.swipeRecognizer.IsTracking() {
		return
	}
	result, ok := p.swipeRecognizer.OnMouseMove(pos)
	if !ok {
		if !p.swipeRecognizer.IsTracking() {
			p.swipeTarget = nil
		}
		return
	}
	targetSprite := p.swipeTarget
	p.swipeTarget = nil
	p.dispatchSwipeResult(result, targetSprite)
}

func (p *inputManager) dispatchSwipeResult(result inputstate.SwipeResult, targetSprite *SpriteImpl) {
	targetName := "stage"
	if targetSprite != nil {
		targetName = targetSprite.name
	}

	if isDebugEventEnabled() {
		spxlog.Debug("Swipe detected: direction=%v, velocity=%.2f, distance=%.2f, target=%s",
			Direction(result.Direction), result.Velocity, result.Distance, targetName)
	}

	if targetSprite != nil {
		targetSprite.doWhenSwipe(Direction(result.Direction), targetSprite)
		return
	}
	p.g.sinkMgr.doWhenSwipe(Direction(result.Direction), p.g)
}
