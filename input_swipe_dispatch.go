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
	coreevent "github.com/goplus/spx/v2/internal/core/event"
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

func (p *inputManager) beginSwipeTracking(startPos mathf.Vec2, targetSprite *SpriteImpl) {
	p.swipe.Begin(startPos, targetSprite)
}

func (p *inputManager) finishSwipeTracking(point mathf.Vec2) {
	p.swipe.Finish(point, p.swipeHooks())
}

func (p *inputManager) onMouseMove(pos mathf.Vec2) {
	p.swipe.OnMouseMove(pos, p.swipeHooks())
}

func (p *inputManager) swipeHooks() coreruntime.SwipeHooks[*SpriteImpl] {
	return coreruntime.SwipeHooks[*SpriteImpl]{
		Debug: coreevent.If1(isDebugEventEnabled, func(ev coreruntime.SwipeEvent[*SpriteImpl]) {
			targetName := "stage"
			if ev.Target != nil {
				targetName = ev.Target.name
			}
			spxlog.Debug("Swipe detected: direction=%v, velocity=%.2f, distance=%.2f, target=%s",
				Direction(ev.Direction), ev.Velocity, ev.Distance, targetName)
		}),
		DispatchTarget: func(direction float64, targetSprite *SpriteImpl) {
			targetSprite.doWhenSwipe(Direction(direction), targetSprite)
		},
		DispatchStage: func(direction float64) {
			p.g.sinkMgr.doWhenSwipe(Direction(direction), p.g)
		},
	}
}
