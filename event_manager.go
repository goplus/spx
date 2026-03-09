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

	"github.com/goplus/spx/v2/internal/engine"
)

type eventSink struct {
	pthis threadObj
	cond  func(any) bool
	sink  any
}

type eventSinks struct {
	*eventSinkMgr
	pthis threadObj
}

type eventSinkMgr struct {
	mu sync.RWMutex

	allWhenStart           []eventSink
	allWhenAwake           []eventSink
	allWhenKeyPressed      []eventSink
	allWhenSwipe           []eventSink
	allWhenIReceive        []eventSink
	allWhenBackdropChanged []eventSink
	allWhenCloned          []eventSink
	allWhenTouchStart      []eventSink
	allWhenTouching        []eventSink
	allWhenTouchEnd        []eventSink
	allWhenClick           []eventSink
	allWhenTimer           []eventSink
	calledStart            bool

	// cache of pointers returned by sinkBuckets()
	sinkBucketsCache []*[]eventSink
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
	p.eventSinkMgr.doDeleteClone(p.pthis)
}

func (p *eventSinks) doWhenSwipe(direction Direction, target threadObj) {
	p.eventSinkMgr.doWhenSwipe(direction, target)
}

func (p *eventSinks) onAwake(onAwake func()) {
	pthis := p.pthis
	p.eventSinkMgr.addWhenAwake(eventSink{
		pthis: p.pthis,
		sink:  onAwake,
		cond: func(data any) bool {
			return data == nil || data == pthis
		},
	})
}

func (p *eventSinkMgr) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, bucket := range p.sinkBuckets() {
		*bucket = nil
	}
	p.calledStart = false
}

func (p *eventSinkMgr) doDeleteClone(this any) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, bucket := range p.sinkBuckets() {
		*bucket = doDeleteClone(*bucket, this)
	}
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

func doDeleteClone(sinks []eventSink, this any) []eventSink {
	n := 0
	for _, sink := range sinks {
		if sink.pthis != this {
			sinks[n] = sink
			n++
		}
	}
	clear(sinks[n:])
	return sinks[:n]
}

func copyEventSinks(sinks []eventSink) []eventSink {
	if len(sinks) == 0 {
		return nil
	}
	out := make([]eventSink, len(sinks))
	copy(out, sinks)
	return out
}

// sinkBuckets returns pointers to all allWhen* slice fields.
// IMPORTANT: Every allWhen* field in eventSinkMgr must be listed here.
// Missing a field will cause reset() and doDeleteClone() to silently skip it.
func (p *eventSinkMgr) sinkBuckets() []*[]eventSink {
	if p.sinkBucketsCache != nil {
		return p.sinkBucketsCache
	}

	p.sinkBucketsCache = []*[]eventSink{
		&p.allWhenStart,
		&p.allWhenAwake,
		&p.allWhenKeyPressed,
		&p.allWhenSwipe,
		&p.allWhenIReceive,
		&p.allWhenBackdropChanged,
		&p.allWhenCloned,
		&p.allWhenTouchStart,
		&p.allWhenTouching,
		&p.allWhenTouchEnd,
		&p.allWhenClick,
		&p.allWhenTimer,
	}
	return p.sinkBucketsCache
}

func (p *eventSinkMgr) addSink(bucket *[]eventSink, sink eventSink) {
	p.mu.Lock()
	*bucket = append(*bucket, sink)
	p.mu.Unlock()
}

func (p *eventSinkMgr) snapshotSinks(sinks []eventSink) []eventSink {
	p.mu.RLock()
	out := copyEventSinks(sinks)
	p.mu.RUnlock()
	return out
}

func (p *eventSinkMgr) addWhenStart(sink eventSink) {
	p.addSink(&p.allWhenStart, sink)
}

func (p *eventSinkMgr) addWhenAwake(sink eventSink) {
	p.addSink(&p.allWhenAwake, sink)
}

func (p *eventSinkMgr) addWhenKeyPressed(sink eventSink) {
	p.addSink(&p.allWhenKeyPressed, sink)
}

func (p *eventSinkMgr) addWhenSwipe(sink eventSink) {
	p.addSink(&p.allWhenSwipe, sink)
}

func (p *eventSinkMgr) addWhenIReceive(sink eventSink) {
	p.addSink(&p.allWhenIReceive, sink)
}

func (p *eventSinkMgr) addWhenBackdropChanged(sink eventSink) {
	p.addSink(&p.allWhenBackdropChanged, sink)
}

func (p *eventSinkMgr) addWhenCloned(sink eventSink) {
	p.addSink(&p.allWhenCloned, sink)
}

func (p *eventSinkMgr) addWhenTouchStart(sink eventSink) {
	p.addSink(&p.allWhenTouchStart, sink)
}

func (p *eventSinkMgr) addWhenTouching(sink eventSink) {
	p.addSink(&p.allWhenTouching, sink)
}

func (p *eventSinkMgr) addWhenTouchEnd(sink eventSink) {
	p.addSink(&p.allWhenTouchEnd, sink)
}

func (p *eventSinkMgr) addWhenClick(sink eventSink) {
	p.addSink(&p.allWhenClick, sink)
}

func (p *eventSinkMgr) addWhenTimer(sink eventSink) {
	p.addSink(&p.allWhenTimer, sink)
}

func (p *eventSinkMgr) snapshotWhenStartOnce() []eventSink {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calledStart {
		return nil
	}
	p.calledStart = true
	return copyEventSinks(p.allWhenStart)
}

func (p *eventSinkMgr) snapshotWhenAwake() []eventSink {
	return p.snapshotSinks(p.allWhenAwake)
}

func (p *eventSinkMgr) snapshotWhenKeyPressed() []eventSink {
	return p.snapshotSinks(p.allWhenKeyPressed)
}

func (p *eventSinkMgr) snapshotWhenSwipe() []eventSink {
	return p.snapshotSinks(p.allWhenSwipe)
}

func (p *eventSinkMgr) snapshotWhenIReceive() []eventSink {
	return p.snapshotSinks(p.allWhenIReceive)
}

func (p *eventSinkMgr) snapshotWhenBackdropChanged() []eventSink {
	return p.snapshotSinks(p.allWhenBackdropChanged)
}

func (p *eventSinkMgr) snapshotWhenCloned() []eventSink {
	return p.snapshotSinks(p.allWhenCloned)
}

func (p *eventSinkMgr) snapshotWhenTouchStart() []eventSink {
	return p.snapshotSinks(p.allWhenTouchStart)
}

func (p *eventSinkMgr) snapshotWhenTouching() []eventSink {
	return p.snapshotSinks(p.allWhenTouching)
}

func (p *eventSinkMgr) snapshotWhenTouchEnd() []eventSink {
	return p.snapshotSinks(p.allWhenTouchEnd)
}

func (p *eventSinkMgr) snapshotWhenClick() []eventSink {
	return p.snapshotSinks(p.allWhenClick)
}

func (p *eventSinkMgr) snapshotWhenTimer() []eventSink {
	return p.snapshotSinks(p.allWhenTimer)
}
