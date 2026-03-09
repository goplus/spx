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
	"github.com/goplus/spx/v2/internal/coroutine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/timer"
)

type IEventSinks interface {
	OnAnyKey(onKey func(key Key))
	OnBackdrop__0(onBackdrop func(name BackdropName))
	OnBackdrop__1(name BackdropName, onBackdrop func())
	OnClick(onClick func())
	OnKey__0(key Key, onKey func())
	OnKey__1(keys []Key, onKey func(Key))
	OnKey__2(keys []Key, onKey func())
	OnMsg__0(onMsg func(msg string, data any))
	OnMsg__1(msg string, onMsg func())
	OnStart(onStart func())
	OnSwipe__0(direction Direction, onSwipe func())
	OnTimer(time float64, onTimer func())
	Stop(kind StopKind)
}

type StopKind = coreevent.StopKind

const (
	AllStop              StopKind = coreevent.AllStop              // -3: stop all scripts of stage/sprites and abort this script
	AllOtherScripts      StopKind = coreevent.AllOtherScripts      // stop all other scripts
	AllSprites           StopKind = coreevent.AllSprites           // stop all scripts of sprites
	ThisSprite           StopKind = coreevent.ThisSprite           // stop all scripts of this sprite
	ThisScript           StopKind = coreevent.ThisScript           // abort this script
	OtherScriptsInSprite StopKind = coreevent.OtherScriptsInSprite // stop other scripts of this sprite
)

func (p *eventSinks) OnStart(onStart func()) {
	p.eventSinkMgr.AddStart(coreevent.NewSink(p.pthis, onStart))
}

func (p *eventSinks) OnClick(onClick func()) {
	pthis := p.pthis
	p.eventSinkMgr.AddClick(coreevent.NewSink(pthis, onClick, coreevent.MatchOwner(pthis)))
}

func (p *eventSinks) OnAnyKey(onKey func(key Key)) {
	p.eventSinkMgr.AddKeyPressed(coreevent.NewSink(p.pthis, onKey))
}

func (p *eventSinks) OnTimer(time float64, call func()) {
	timer.RegisterTimer(time)
	p.eventSinkMgr.AddTimer(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(call, coreevent.If1(isDebugEventEnabled, func(float64) {
			spxlog.Debug("==> onTimer: %s", nameOf(p.pthis))
		})),
		coreevent.MatchApproxFloat(time, 0.001),
	))
}

func (p *eventSinks) OnKey__0(key Key, onKey func()) {
	p.eventSinkMgr.AddKeyPressed(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onKey, coreevent.If1(isDebugEventEnabled, func(Key) {
			spxlog.Debug("==> onKey: %v, %s", key, nameOf(p.pthis))
		})),
		coreevent.MatchValue(key),
	))
}

func (p *eventSinks) OnSwipe__0(direction Direction, onSwipe func()) {
	p.eventSinkMgr.AddSwipe(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onSwipe, coreevent.If1(isDebugEventEnabled, func(Direction) {
			spxlog.Debug("==> onSwipe: %v, %s", direction, nameOf(p.pthis))
		})),
		coreevent.MatchValue(direction),
	))
}

func (p *eventSinks) OnKey__1(keys []Key, onKey func(Key)) {
	p.eventSinkMgr.AddKeyPressed(coreevent.NewSink(
		p.pthis,
		coreevent.Tap1(onKey, coreevent.If1(isDebugEventEnabled, func(key Key) {
			spxlog.Debug("==> onKey: %v, %s", keys, nameOf(p.pthis))
		})),
		coreevent.MatchAnyOf(keys),
	))
}

func (p *eventSinks) OnKey__2(keys []Key, onKey func()) {
	p.OnKey__1(keys, coreevent.Ignore1[Key](onKey))
}

func (p *eventSinks) OnMsg__0(onMsg func(msg string, data any)) {
	p.eventSinkMgr.AddIReceive(coreevent.NewSink(p.pthis, onMsg))
}

func (p *eventSinks) OnMsg__1(msg string, onMsg func()) {
	p.eventSinkMgr.AddIReceive(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid2(onMsg, coreevent.If2(isDebugEventEnabled, func(msg string, data any) {
			spxlog.Debug("==> onMsg: %s, %s", msg, nameOf(p.pthis))
		})),
		coreevent.MatchValue(msg),
	))
}

func (p *eventSinks) OnBackdrop__0(onBackdrop func(name BackdropName)) {
	p.eventSinkMgr.AddBackdropChanged(coreevent.NewSink(p.pthis, onBackdrop))
}

func (p *eventSinks) OnBackdrop__1(name BackdropName, onBackdrop func()) {
	p.eventSinkMgr.AddBackdropChanged(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onBackdrop, coreevent.If1(isDebugEventEnabled, func(name BackdropName) {
			spxlog.Debug("==> onBackdrop: %s, %s", name, nameOf(p.pthis))
		})),
		coreevent.MatchValue(name),
	))
}

func (p *eventSinks) Stop(kind StopKind) {
	current := gco.Current()
	filter, abort := coreevent.ResolveStop(
		kind,
		p.pthis,
		func(obj any) bool { return isSprite(obj) },
		func(obj any) bool { return isGame(obj) },
	)
	if filter != nil {
		gco.StopIf(func(th coroutine.Thread) bool {
			return filter(th.Obj, th == current)
		})
	}
	if abort {
		gco.Abort()
	}
}
