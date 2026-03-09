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

type StopKind int

const (
	AllStop              StopKind = All  // -3: stop all scripts of stage/sprites and abort this script
	AllOtherScripts      StopKind = -100 // stop all other scripts
	AllSprites           StopKind = -101 // stop all scripts of sprites
	ThisSprite           StopKind = -102 // stop all scripts of this sprite
	ThisScript           StopKind = -103 // abort this script
	OtherScriptsInSprite StopKind = -104 // stop other scripts of this sprite
)

func (p *eventSinks) OnStart(onStart func()) {
	p.eventSinkMgr.addWhenStart(eventSink{
		pthis: p.pthis,
		sink:  onStart,
	})
}

func (p *eventSinks) OnClick(onClick func()) {
	pthis := p.pthis
	p.eventSinkMgr.addWhenClick(eventSink{
		pthis: pthis,
		sink:  onClick,
		cond: func(data any) bool {
			return data == pthis
		},
	})
}

func (p *eventSinks) OnAnyKey(onKey func(key Key)) {
	p.eventSinkMgr.addWhenKeyPressed(eventSink{
		pthis: p.pthis,
		sink:  onKey,
	})
}

func (p *eventSinks) OnTimer(time float64, call func()) {
	timer.RegisterTimer(time)
	p.eventSinkMgr.addWhenTimer(eventSink{
		pthis: p.pthis,
		sink: func(float64) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onTimer: %s", nameOf(p.pthis))
			}
			call()
		},
		cond: func(data any) bool {
			return mathf.Absf(data.(float64)-time) < 0.001
		},
	})
}

func (p *eventSinks) OnKey__0(key Key, onKey func()) {
	p.eventSinkMgr.addWhenKeyPressed(eventSink{
		pthis: p.pthis,
		sink: func(Key) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onKey: %v, %s", key, nameOf(p.pthis))
			}
			onKey()
		},
		cond: func(data any) bool {
			return data.(Key) == key
		},
	})
}

func (p *eventSinks) OnSwipe__0(direction Direction, onSwipe func()) {
	p.eventSinkMgr.addWhenSwipe(eventSink{
		pthis: p.pthis,
		sink: func(Direction) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onSwipe: %v, %s", direction, nameOf(p.pthis))
			}
			onSwipe()
		},
		cond: func(data any) bool {
			return data.(Direction) == direction
		},
	})
}

func (p *eventSinks) OnKey__1(keys []Key, onKey func(Key)) {
	p.eventSinkMgr.addWhenKeyPressed(eventSink{
		pthis: p.pthis,
		sink: func(key Key) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onKey: %v, %s", keys, nameOf(p.pthis))
			}
			onKey(key)
		},
		cond: func(data any) bool {
			keyIn := data.(Key)
			for _, key := range keys {
				if key == keyIn {
					return true
				}
			}
			return false
		},
	})
}

func (p *eventSinks) OnKey__2(keys []Key, onKey func()) {
	p.OnKey__1(keys, func(Key) {
		onKey()
	})
}

func (p *eventSinks) OnMsg__0(onMsg func(msg string, data any)) {
	p.eventSinkMgr.addWhenIReceive(eventSink{
		pthis: p.pthis,
		sink:  onMsg,
	})
}

func (p *eventSinks) OnMsg__1(msg string, onMsg func()) {
	p.eventSinkMgr.addWhenIReceive(eventSink{
		pthis: p.pthis,
		sink: func(msg string, data any) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onMsg: %s, %s", msg, nameOf(p.pthis))
			}
			onMsg()
		},
		cond: func(data any) bool {
			return data.(string) == msg
		},
	})
}

func (p *eventSinks) OnBackdrop__0(onBackdrop func(name BackdropName)) {
	p.eventSinkMgr.addWhenBackdropChanged(eventSink{
		pthis: p.pthis,
		sink:  onBackdrop,
	})
}

func (p *eventSinks) OnBackdrop__1(name BackdropName, onBackdrop func()) {
	p.eventSinkMgr.addWhenBackdropChanged(eventSink{
		pthis: p.pthis,
		sink: func(name BackdropName) {
			if isDebugEventEnabled() {
				spxlog.Debug("==> onBackdrop: %s, %s", name, nameOf(p.pthis))
			}
			onBackdrop()
		},
		cond: func(data any) bool {
			return data.(BackdropName) == name
		},
	})
}

func (p *eventSinks) Stop(kind StopKind) {
	var filter func(th coroutine.Thread) bool
	switch kind {
	case AllSprites:
		filter = func(th coroutine.Thread) bool {
			return isSprite(th.Obj)
		}
	case ThisSprite:
		this := p.pthis
		filter = func(th coroutine.Thread) bool {
			return th.Obj == this
		}
	case OtherScriptsInSprite:
		this := p.pthis
		filter = func(th coroutine.Thread) bool {
			return th.Obj == this && th != gco.Current()
		}
	case AllOtherScripts:
		filter = func(th coroutine.Thread) bool {
			return (isSprite(th.Obj) || isGame(th.Obj)) && th != gco.Current()
		}
	case AllStop:
		gco.StopIf(func(th coroutine.Thread) bool {
			return isSprite(th.Obj) || isGame(th.Obj)
		})
		fallthrough
	case ThisScript:
		gco.Abort()
	}
	if filter != nil {
		gco.StopIf(filter)
	}
}
