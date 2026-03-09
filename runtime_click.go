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

const (
	clickTimerGlobal = -1 // Global click cooldown
	clickTimerStage  = 0  // Stage click cooldown
)

type clicker interface {
	threadObj
	doWhenClick(this threadObj)
	getProxy() *engine.Sprite
	Visible() bool
}

func (p *Game) findClickTarget(point mathf.Vec2) (clicker, *SpriteImpl) {
	tempItems := p.getTempShapes()
	count := len(tempItems)
	for i := range count {
		item := tempItems[count-i-1]
		o, ok := item.(clicker)
		if !ok {
			continue
		}
		syncSprite := o.getProxy()
		if syncSprite == nil || !o.Visible() {
			continue
		}
		if !p.engine().SpriteMgr.CheckCollisionWithPoint(syncSprite.GetId(), point, true) {
			continue
		}
		if sprite, ok := o.(*SpriteImpl); ok {
			return o, sprite
		}
		return o, nil
	}
	return nil, nil
}

func (p *Game) dispatchClickTarget(target clicker) {
	if target != nil {
		syncSprite := target.getProxy()
		if syncSprite != nil && p.inputMgr.canTriggerClickEvent(syncSprite.GetId()) {
			target.doWhenClick(target)
		}
		return
	}
	if p.inputMgr.canTriggerClickEvent(clickTimerStage) {
		p.sinkMgr.doWhenClick(p)
	}
}

func (p *Game) doWhenLeftButtonUp(ev *eventLeftButtonUp) {
	p.inputMgr.finishSwipeTracking(ev.Pos)
}

func (p *Game) doWhenLeftButtonDown(ev *eventLeftButtonDown) {
	point := ev.Pos
	target, targetSprite := p.findClickTarget(point)
	p.inputMgr.beginSwipeTracking(point, targetSprite)
	if !p.inputMgr.canTriggerClickEvent(clickTimerGlobal) {
		return
	}
	p.dispatchClickTarget(target)
}
