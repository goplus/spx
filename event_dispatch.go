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
	"sync"

	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

func (p *eventSinkMgr) doWhenStart() {
	sinks := p.snapshotWhenStartOnce()
	if len(sinks) == 0 {
		return
	}
	asyncCall(sinks, false, nil, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("==> onStart: %s", nameOf(ev.pthis))
		}
		ev.sink.(func())()
	})
}

func (p *eventSinkMgr) doWhenAwake(this threadObj) {
	sinks := p.snapshotWhenAwake()
	syncCall(sinks, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("==> onAwake: %s", nameOf(ev.pthis))
		}
		ev.sink.(func())()
	})
}

func (p *eventSinkMgr) doWhenTimer(time float64) {
	sinks := p.snapshotWhenTimer()
	asyncCall(sinks, false, time, func(ev *eventSink) {
		ev.sink.(func(float64))(time)
	})
}

func (p *eventSinkMgr) doWhenKeyPressed(key Key) {
	sinks := p.snapshotWhenKeyPressed()
	asyncCall(sinks, false, key, func(ev *eventSink) {
		ev.sink.(func(Key))(key)
	})
}

func (p *eventSinkMgr) doWhenSwipe(direction Direction, this threadObj) {
	sinks := p.snapshotWhenSwipe()
	asyncCall(sinks, false, direction, func(ev *eventSink) {
		if ev.pthis == this {
			ev.sink.(func(Direction))(direction)
		}
	})
}

func (p *eventSinkMgr) doWhenClick(this threadObj) {
	sinks := p.snapshotWhenClick()
	asyncCall(sinks, false, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("==> onClick: %s", nameOf(this))
		}
		ev.sink.(func())()
	})
}

func (p *eventSinkMgr) doWhenTouchStart(this threadObj, obj *SpriteImpl) {
	sinks := p.snapshotWhenTouchStart()
	asyncCall(sinks, false, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("===> onTouchStart: %s, %s", nameOf(this), obj.name)
		}
		ev.sink.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenTouching(this threadObj, obj *SpriteImpl) {
	sinks := p.snapshotWhenTouching()
	asyncCall(sinks, false, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("==> onTouching: %s, %s", nameOf(this), obj.name)
		}
		ev.sink.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenTouchEnd(this threadObj, obj *SpriteImpl) {
	sinks := p.snapshotWhenTouchEnd()
	asyncCall(sinks, false, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("===> onTouchEnd: %s, %s", nameOf(this), obj.name)
		}
		ev.sink.(func(Sprite))(obj.sprite)
	})
}

func (p *eventSinkMgr) doWhenCloned(this threadObj, data any) {
	sinks := p.snapshotWhenCloned()
	asyncCall(sinks, true, this, func(ev *eventSink) {
		if isDebugEventEnabled() {
			spxlog.Debug("==> onCloned: %s", nameOf(this))
		}
		ev.sink.(func(any))(data)
	})
}

func (p *eventSinkMgr) doWhenIReceive(msg string, data any, wait bool) {
	sinks := p.snapshotWhenIReceive()
	call(sinks, wait, msg, func(ev *eventSink) {
		ev.sink.(func(string, any))(msg, data)
	})
}

func (p *eventSinkMgr) doWhenBackdropChanged(name BackdropName, wait bool) {
	sinks := p.snapshotWhenBackdropChanged()
	call(sinks, wait, name, func(ev *eventSink) {
		ev.sink.(func(BackdropName))(name)
	})
}

func asyncCall(sinks []eventSink, start bool, data any, doSth func(*eventSink)) {
	for _, ev := range sinks {
		ev := ev
		if ev.cond == nil || ev.cond(data) {
			gco.CreateAndStart(start, ev.pthis, func(coroutine.Thread) int {
				doSth(&ev)
				return 0
			})
		}
	}
}

func syncCall(sinks []eventSink, data any, doSth func(*eventSink)) {
	var wg sync.WaitGroup
	for _, ev := range sinks {
		ev := ev
		if ev.cond == nil || ev.cond(data) {
			wg.Add(1)
			gco.CreateAndStart(false, ev.pthis, func(coroutine.Thread) int {
				defer wg.Done()
				doSth(&ev)
				return 0
			})
		}
	}
	engine.WaitToDo(wg.Wait)
}

func call(sinks []eventSink, wait bool, data any, doSth func(*eventSink)) {
	if wait {
		syncCall(sinks, data, doSth)
	} else {
		asyncCall(sinks, false, data, doSth)
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
