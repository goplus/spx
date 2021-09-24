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
	pthis interface{}
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

func (ss *eventSink) doClone(src, copy interface{}) *eventSink {
	for p := ss; p != nil; p = p.prev {
		if p.pthis == src {
			ss = &eventSink{prev: ss, pthis: copy, sink: p.sink}
		}
	}
	return ss
}

func (ss *eventSink) asyncCall(wg *sync.WaitGroup, doSth func(*eventSink)) {
	for ss != nil {
		if wg != nil {
			wg.Add(1)
		}
		copy := ss
		createThread(false, func(coroutine.Thread) int {
			if wg != nil {
				defer wg.Done()
			}
			doSth(copy)
			return 0
		})
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
	calledStart       bool
}

func (p *eventSinkMgr) doClone(src, copy interface{}) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.allWhenStart = p.allWhenStart.doClone(src, copy)
	p.allWhenKeyPressed = p.allWhenKeyPressed.doClone(src, copy)
	p.allWhenIReceive = p.allWhenIReceive.doClone(src, copy)
	p.allWhenSceneStart = p.allWhenSceneStart.doClone(src, copy)
}

func (p *eventSinkMgr) doDeleteClone(this interface{}) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.allWhenStart = p.allWhenStart.doDeleteClone(this)
	p.allWhenKeyPressed = p.allWhenKeyPressed.doDeleteClone(this)
	p.allWhenIReceive = p.allWhenIReceive.doDeleteClone(this)
	p.allWhenSceneStart = p.allWhenSceneStart.doDeleteClone(this)
}

func (p *eventSinkMgr) doWhenStart() {
	p.mutex.Lock()
	if !p.calledStart {
		p.calledStart = true
		p.allWhenStart.asyncCall(nil, func(ev *eventSink) {
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
	p.allWhenKeyPressed.asyncCall(nil, func(ev *eventSink) {
		ev.sink.(func(Key))(key)
	})
	p.mutex.Unlock()
}

func (p *eventSinkMgr) doWhenIReceive(msg string, data interface{}, wait bool) {
	var wg *sync.WaitGroup
	if wait {
		wg = new(sync.WaitGroup)
	}
	p.mutex.Lock()
	p.allWhenIReceive.asyncCall(wg, func(ev *eventSink) {
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
	p.allWhenSceneStart.asyncCall(wg, func(ev *eventSink) {
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
	pthis         interface{}
	allWhenCloned func(data interface{})
	allWhenClick  func()
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

func (ss *eventSinks) init(mgr *eventSinkMgr, this interface{}) {
	ss.eventSinkMgr = mgr
	ss.pthis = this
}

func (ss *eventSinks) initFrom(src *eventSinks, this interface{}) {
	ss.eventSinkMgr = src.eventSinkMgr
	ss.pthis = this
	ss.allWhenCloned = src.allWhenCloned
	ss.allWhenClick = src.allWhenClick
	src.eventSinkMgr.doClone(src.pthis, this)
}

func (ss *eventSinks) doDeleteClone() {
	ss.eventSinkMgr.doDeleteClone(ss.pthis)
}

func (ss *eventSinks) doWhenCloned(data interface{}) {
	if sink := ss.allWhenCloned; sink != nil {
		createThread(true, func(coroutine.Thread) int {
			if debugEvent {
				log.Println("==> onCloned", nameOf(ss.pthis))
			}
			sink(data)
			return 0
		})
	}
}

func (ss *eventSinks) doWhenClick() {
	if sink := ss.allWhenClick; sink != nil {
		createThread(false, func(coroutine.Thread) int {
			if debugEvent {
				log.Println("==> onClick", nameOf(ss.pthis))
			}
			sink()
			return 0
		})
	}
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
	if ss.allWhenClick != nil {
		panic("Can't support multi onClick events")
	}
	ss.allWhenClick = onClick
}

func (ss *eventSinks) OnCloned__0(onCloned func(data interface{})) {
	if ss.allWhenCloned != nil {
		panic("Can't support multi onCloned events")
	}
	ss.allWhenCloned = onCloned
}

func (ss *eventSinks) OnCloned__1(onCloned func()) {
	ss.OnCloned__0(func(interface{}) {
		onCloned()
	})
}

func (ss *eventSinks) OnKey__0(onKey func(key Key)) {
	ss.allWhenKeyPressed = &eventSink{
		prev:  ss.allWhenKeyPressed,
		pthis: ss.pthis,
		sink:  onKey,
	}
}

func (ss *eventSinks) OnKey__1(key Key, onKey func()) {
	ss.OnKey__0(func(keyIn Key) {
		if keyIn == key {
			if debugEvent {
				log.Println("==> onKey", key, nameOf(ss.pthis))
			}
			onKey()
		}
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
	ss.OnMsg__0(func(msgIn string, data interface{}) {
		if msgIn == msg {
			if debugEvent {
				log.Println("==> onMsg", msg, nameOf(ss.pthis))
			}
			onMsg()
		}
	})
}

func (ss *eventSinks) OnScene__0(onScene func(name string)) {
	ss.allWhenSceneStart = &eventSink{
		prev:  ss.allWhenSceneStart,
		pthis: ss.pthis,
		sink:  onScene,
	}
}

func (ss *eventSinks) OnScene__1(name string, onScene func()) {
	ss.OnScene__0(func(nameIn string) {
		if nameIn == name {
			if debugEvent {
				log.Println("==> onScene", name, nameOf(ss.pthis))
			}
			onScene()
		}
	})
}

/*
	func onStart()
	func onClick()
	func onKey(key Key)
	func onMsg(msg string, data interface{})
	func onScene(name string)
	func onCloned(data interface{})
//
func (ss *eventSinks) Sink(obj interface{}) {
	if o, ok := obj.(interface{ OnStart() }); ok {
		ss.OnStart(o.OnStart)
	}
	if o, ok := obj.(interface{ OnClick() }); ok {
		ss.OnClick(o.OnClick)
	}
	if o, ok := obj.(interface{ OnKey(Key) }); ok {
		ss.OnKey__0(o.OnKey)
	}
	if o, ok := obj.(interface{ OnMsg(string, interface{}) }); ok {
		ss.OnMsg__0(o.OnMsg)
	}
	if o, ok := obj.(interface{ OnScene(string) }); ok {
		ss.OnScene__0(o.OnScene)
	}
	if o, ok := obj.(interface{ OnCloned(interface{}) }); ok {
		ss.OnCloned__0(o.OnCloned)
	}
}
*/

// -------------------------------------------------------------------------------------
