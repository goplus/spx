/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package spx

import (
	"log"
	"sync"

	"github.com/goplus/spx/internal/coroutine"
)

// -------------------------------------------------------------------------------------

type eventSink struct {
	prev  *eventSink
	pthis threadObj
	cond  func(interface{}) bool
	sink  interface{}
}

func (ss *eventSink) doDeleteClone(this interface{}) (ret *eventSink) {
	ret = ss
	pp := &ret
	for {
		p := *pp
		if p == nil {
			return
		}
		if p.pthis == this {
			*pp = p.prev
		} else {
			pp = &p.prev
		}
	}
}

func (ss *eventSink) asyncCall(start bool, wg *sync.WaitGroup, data interface{}, doSth func(*eventSink)) {
	for ss != nil {
		if ss.cond == nil || ss.cond(data) {
			if wg != nil {
				wg.Add(1)
			}
			copy := ss
			createThread(ss.pthis, start, func(coroutine.Thread) int {
				if wg != nil {
					defer wg.Done()
				}
				doSth(copy)
				return 0
			})
		}
		ss = ss.prev
	}
}

// -------------------------------------------------------------------------------------

type eventSinkMgr struct {
	mutex             sync.Mutex
	allWhenStart      *eventSink
	allWhenKeyPressed *eventSink
	allWhenIReceive   *eventSink
	allWhenSceneStart *eventSink
	allWhenCloned     *eventSink
	allWhenClick      *eventSink
	calledStart       bool
}

func (p *eventSinkMgr) doDeleteClone(this interface{}) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.allWhenStart = p.allWhenStart.doDeleteClone(this)
	p.allWhenKeyPressed = p.allWhenKeyPressed.doDeleteClone(this)
	p.allWhenIReceive = p.allWhenIReceive.doDeleteClone(this)
	p.allWhenSceneStart = p.allWhenSceneStart.doDeleteClone(this)
	p.allWhenCloned = p.allWhenCloned.doDeleteClone(this)
	p.allWhenClick = p.allWhenClick.doDeleteClone(this)
}

func (p *eventSinkMgr) doWhenStart() {
	p.mutex.Lock()
	if !p.calledStart {
		p.calledStart = true
		p.allWhenStart.asyncCall(false, nil, nil, func(ev *eventSink) {
			if debugEvent {
				log.Println("==> onStart", nameOf(ev.pthis))
			}
			ev.sink.(func())()
		})
	}
	p.mutex.Unlock()
}

func (p *eventSinkMgr) doWhenKeyPressed(key Key) {
	p.mutex.Lock()
	p.allWhenKeyPressed.asyncCall(false, nil, key, func(ev *eventSink) {
		ev.sink.(func(Key))(key)
	})
	p.mutex.Unlock()
}

func (p *eventSinkMgr) doWhenCloned(this threadObj, data interface{}) {
	p.mutex.Lock()
	p.allWhenCloned.asyncCall(true, nil, this, func(ev *eventSink) {
		if debugEvent {
			log.Println("==> onCloned", nameOf(ev.pthis))
		}
		ev.sink.(func(interface{}))(data)
	})
	p.mutex.Unlock()
}

func (p *eventSinkMgr) doWhenClick(this threadObj) {
	p.mutex.Lock()
	p.allWhenClick.asyncCall(false, nil, this, func(ev *eventSink) {
		if debugEvent {
			log.Println("==> onClick", nameOf(ev.pthis))
		}
		ev.sink.(func())()
	})
	p.mutex.Unlock()
}

func (p *eventSinkMgr) doWhenIReceive(msg string, data interface{}, wait bool) {
	var wg *sync.WaitGroup
	if wait {
		wg = new(sync.WaitGroup)
	}
	p.mutex.Lock()
	p.allWhenIReceive.asyncCall(false, wg, msg, func(ev *eventSink) {
		ev.sink.(func(string, interface{}))(msg, data)
	})
	p.mutex.Unlock()
	if wait {
		waitToDo(wg.Wait)
	}
}

func (p *eventSinkMgr) doWhenSceneStart(name string, wait bool) {
	var wg *sync.WaitGroup
	if wait {
		wg = new(sync.WaitGroup)
	}
	p.mutex.Lock()
	p.allWhenSceneStart.asyncCall(false, wg, name, func(ev *eventSink) {
		ev.sink.(func(string))(name)
	})
	p.mutex.Unlock()
	if wait {
		waitToDo(wg.Wait)
	}
}

// -------------------------------------------------------------------------------------

type eventSinks struct {
	*eventSinkMgr
	pthis threadObj
}

func nameOf(this interface{}) string {
	if spr, ok := this.(*Sprite); ok {
		return spr.name
	}
	if _, ok := this.(*Game); ok {
		return "Game"
	}
	panic("eventSinks: unexpected this object")
}

func (ss *eventSinks) init(mgr *eventSinkMgr, this threadObj) {
	ss.eventSinkMgr = mgr
	ss.pthis = this
}

func (ss *eventSinks) initFrom(src *eventSinks, this threadObj) {
	ss.eventSinkMgr = src.eventSinkMgr
	ss.pthis = this
}

func (ss *eventSinks) doDeleteClone() {
	ss.eventSinkMgr.doDeleteClone(ss.pthis)
}

// -------------------------------------------------------------------------------------

func (ss *eventSinks) OnStart(onStart func()) {
	ss.allWhenStart = &eventSink{
		prev:  ss.allWhenStart,
		pthis: ss.pthis,
		sink:  onStart,
	}
}

func (ss *eventSinks) OnClick(onClick func()) {
	pthis := ss.pthis
	ss.allWhenClick = &eventSink{
		prev:  ss.allWhenClick,
		pthis: pthis,
		sink:  onClick,
		cond: func(data interface{}) bool {
			return data == pthis
		},
	}
}

func (ss *eventSinks) OnCloned__0(onCloned func(data interface{})) {
	pthis := ss.pthis
	ss.allWhenCloned = &eventSink{
		prev:  ss.allWhenCloned,
		pthis: pthis,
		sink:  onCloned,
		cond: func(data interface{}) bool {
			return data == pthis
		},
	}
}

func (ss *eventSinks) OnCloned__1(onCloned func()) {
	ss.OnCloned__0(func(interface{}) {
		onCloned()
	})
}

func (ss *eventSinks) OnAnyKey(onKey func(key Key)) {
	ss.allWhenKeyPressed = &eventSink{
		prev:  ss.allWhenKeyPressed,
		pthis: ss.pthis,
		sink:  onKey,
	}
}

func (ss *eventSinks) OnKey__0(key Key, onKey func()) {
	ss.allWhenKeyPressed = &eventSink{
		prev:  ss.allWhenKeyPressed,
		pthis: ss.pthis,
		sink: func(Key) {
			if debugEvent {
				log.Println("==> onKey", key, nameOf(ss.pthis))
			}
			onKey()
		},
		cond: func(data interface{}) bool {
			return data.(Key) == key
		},
	}
}

func (ss *eventSinks) OnKey__1(keys []Key, onKey func(Key)) {
	ss.allWhenKeyPressed = &eventSink{
		prev:  ss.allWhenKeyPressed,
		pthis: ss.pthis,
		sink: func(key Key) {
			if debugEvent {
				log.Println("==> onKey", keys, nameOf(ss.pthis))
			}
			onKey(key)
		},
		cond: func(data interface{}) bool {
			keyIn := data.(Key)
			for _, key := range keys {
				if key == keyIn {
					return true
				}
			}
			return false
		},
	}
}

func (ss *eventSinks) OnKey__2(keys []Key, onKey func()) {
	ss.OnKey__1(keys, func(Key) {
		onKey()
	})
}

func (ss *eventSinks) OnMsg__0(onMsg func(msg string, data interface{})) {
	ss.allWhenIReceive = &eventSink{
		prev:  ss.allWhenIReceive,
		pthis: ss.pthis,
		sink:  onMsg,
	}
}

func (ss *eventSinks) OnMsg__1(msg string, onMsg func()) {
	ss.allWhenIReceive = &eventSink{
		prev:  ss.allWhenIReceive,
		pthis: ss.pthis,
		sink: func(msg string, data interface{}) {
			if debugEvent {
				log.Println("==> onMsg", msg, nameOf(ss.pthis))
			}
			onMsg()
		},
		cond: func(data interface{}) bool {
			return data.(string) == msg
		},
	}
}

func (ss *eventSinks) OnScene__0(onScene func(name string)) {
	ss.allWhenSceneStart = &eventSink{
		prev:  ss.allWhenSceneStart,
		pthis: ss.pthis,
		sink:  onScene,
	}
}

func (ss *eventSinks) OnScene__1(name string, onScene func()) {
	ss.allWhenSceneStart = &eventSink{
		prev:  ss.allWhenSceneStart,
		pthis: ss.pthis,
		sink: func(name string) {
			if debugEvent {
				log.Println("==> onScene", name, nameOf(ss.pthis))
			}
			onScene()
		},
		cond: func(data interface{}) bool {
			return data.(string) == name
		},
	}
}

// -------------------------------------------------------------------------------------
