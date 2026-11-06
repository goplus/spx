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
	"slices"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine"
	spxlog "github.com/goplus/spx/v4/internal/log"
)

func (p *Game) Broadcast__0(msg MsgName) {
	p.doBroadcast(msg, nil, false)
}

func (p *Game) Broadcast__1(msg MsgName, data any) {
	p.doBroadcast(msg, data, false)
}

func (p *Game) BroadcastAndWait__0(msg MsgName) {
	p.doBroadcast(msg, nil, true)
}

func (p *Game) BroadcastAndWait__1(msg MsgName, data any) {
	p.doBroadcast(msg, data, true)
}

func (p *Game) bindScriptEvents() {
	p.scriptEvents.game = p
	p.scriptEventBindings.bind(&p.scriptEvents, p)
}

func (p *Game) doBroadcast(msg MsgName, data any, wait bool) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Broadcast: msg=%s, wait=%v", msg, wait)
	}
	p.doWhenIReceive(msg, data, wait)
}

func (p *Game) handleEvent(ev event) {
	switch e := ev.(type) {
	case *eventLeftButtonUp:
		p.inputMgr.onLeftButtonUp(e.Pos)
	case *eventLeftButtonDown:
		p.inputMgr.onLeftButtonDown(e.Pos)
	case *eventKeyDown:
		// Note: key-up callbacks are not part of the current event sink API.
		p.doWhenKeyPressed(e.Key)
	case *eventStart:
		p.doWhenStart(e.generation)
	case *eventTimer:
		p.doWhenTimer(e.Timestamp)
	}
}

// Click Dispatch
type clicker interface {
	doWhenClick()
	getProxy() *engine.Sprite
	Visible() bool
}

func (p *Game) findClickTarget(point mathf.Vec2) clicker {
	for _, item := range slices.Backward(p.shapeMgr.getTempShapes()) {
		if target, ok := item.(clicker); ok && p.pointHitsClickTarget(target, point) {
			return target
		}
	}
	return nil
}

func (p *Game) pointHitsClickTarget(target clicker, point mathf.Vec2) bool {
	syncSprite := target.getProxy()
	if syncSprite == nil || !target.Visible() {
		return false
	}

	sprite, ok := target.(*SpriteImpl)
	if ok {
		if sprite.isDestroyed() {
			return false
		}
		sprite.syncProxyForQuery()
		if sprite.isFullyGhosted() {
			return false
		}
	}

	return engine.Managers().SpriteMgr.CheckCollisionWithPoint(syncSprite.GetId(), point, true)
}
