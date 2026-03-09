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
	coreevent "github.com/goplus/spx/v2/internal/core/event"
	"github.com/goplus/spx/v2/internal/engine"
)

type eventSink = coreevent.Sink

type eventSinks struct {
	*eventSinkMgr
	pthis threadObj
}

type eventSinkMgr struct {
	coreevent.Manager
}

func (p *eventSinks) init(mgr *eventSinkMgr, this threadObj) {
	p.eventSinkMgr = mgr
	p.pthis = this
}

func (p *eventSinks) initFrom(src *eventSinks, this threadObj) {
	p.eventSinkMgr = src.eventSinkMgr
	p.pthis = this
}

func (p *eventSinks) doDeleteClone() {
	p.eventSinkMgr.DeleteOwner(p.pthis)
}

func (p *eventSinks) doWhenSwipe(direction Direction, target threadObj) {
	p.eventSinkMgr.doWhenSwipe(direction, target)
}

func (p *eventSinks) onAwake(onAwake func()) {
	pthis := p.pthis
	p.eventSinkMgr.AddAwake(coreevent.NewSink(p.pthis, onAwake, coreevent.MatchOwnerOrNil(pthis)))
}

func nameOf(this any) string {
	if spr, ok := this.(*SpriteImpl); ok {
		return spr.name
	}
	if _, ok := this.(*Game); ok {
		return "Game"
	}
	engine.Panic("eventSinks: unexpected this object")
	return ""
}
