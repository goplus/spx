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
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

func (p *eventSinkMgr) doWhenStart() {
	sinks := p.SnapshotStartOnce()
	if len(sinks) == 0 {
		return
	}
	coreevent.DispatchAsync(sinks, false, nil, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onStart: %s", nameOf(ev.Owner))
		})()
		ev.Handler.(func())()
	})
}

func (p *eventSinkMgr) doWhenAwake(this threadObj) {
	sinks := p.SnapshotAwake()
	coreevent.DispatchSync(sinks, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onAwake: %s", nameOf(ev.Owner))
		})()
		ev.Handler.(func())()
	})
}

func (p *eventSinkMgr) doWhenTimer(time float64) {
	sinks := p.SnapshotTimer()
	coreevent.DispatchAsync(sinks, false, time, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(float64))(time)
	})
}

func (p *eventSinkMgr) doWhenKeyPressed(key Key) {
	sinks := p.SnapshotKeyPressed()
	coreevent.DispatchAsync(sinks, false, key, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(Key))(key)
	})
}

func (p *eventSinkMgr) doWhenSwipe(direction Direction, this threadObj) {
	sinks := p.SnapshotSwipe()
	coreevent.DispatchAsync(sinks, false, direction, eventDispatchHooks(), func(ev *eventSink) {
		if ev.Owner == this {
			ev.Handler.(func(Direction))(direction)
		}
	})
}

func (p *eventSinkMgr) doWhenClick(this threadObj) {
	sinks := p.SnapshotClick()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onClick: %s", nameOf(this))
		})()
		ev.Handler.(func())()
	})
}

func (p *eventSinkMgr) doWhenTouchStart(this threadObj, obj *SpriteImpl) {
	sinks := p.SnapshotTouchStart()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("===> onTouchStart: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenTouching(this threadObj, obj *SpriteImpl) {
	sinks := p.SnapshotTouching()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onTouching: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenTouchEnd(this threadObj, obj *SpriteImpl) {
	sinks := p.SnapshotTouchEnd()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("===> onTouchEnd: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenCloned(this threadObj, data any) {
	sinks := p.SnapshotCloned()
	coreevent.DispatchAsync(sinks, true, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onCloned: %s", nameOf(this))
		})()
		ev.Handler.(func(any))(data)
	})
}

func (p *eventSinkMgr) doWhenIReceive(msg string, data any, wait bool) {
	sinks := p.SnapshotIReceive()
	coreevent.Dispatch(sinks, wait, msg, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(string, any))(msg, data)
	})
}

func (p *eventSinkMgr) doWhenBackdropChanged(name BackdropName, wait bool) {
	sinks := p.SnapshotBackdropChanged()
	coreevent.Dispatch(sinks, wait, name, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(BackdropName))(name)
	})
}

func eventDispatchHooks() coreevent.DispatchHooks {
	return coreevent.DispatchHooks{
		Spawn: func(start bool, owner any, call func()) {
			gco.CreateAndStart(start, owner, func(coroutine.Thread) int {
				call()
				return 0
			})
		},
		Wait: engine.WaitToDo,
	}
}

func isGame(obj threadObj) bool {
	_, ok := obj.(*Game)
	return ok
}

func isSprite(obj threadObj) bool {
	_, ok := obj.(*SpriteImpl)
	return ok
}
