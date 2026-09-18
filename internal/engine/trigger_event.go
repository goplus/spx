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

package engine

import "sync"

type TriggerEvent struct {
	Src *Sprite
	Dst *Sprite
}

type triggerEventQueue struct {
	mu      sync.Mutex
	pending []TriggerEvent
	ready   []TriggerEvent
}

var triggerEvents triggerEventQueue

func GetTriggerEvents(dst []TriggerEvent) []TriggerEvent {
	return triggerEvents.drain(dst)
}

func resetTriggerEvents() {
	triggerEvents.reset()
}

func cacheTriggerEvents() {
	triggerEvents.cache()
}

func enqueueTriggerEvent(src, dst *Sprite) {
	triggerEvents.mu.Lock()
	triggerEvents.pending = append(triggerEvents.pending, TriggerEvent{Src: src, Dst: dst})
	triggerEvents.mu.Unlock()
}

func (q *triggerEventQueue) drain(dst []TriggerEvent) []TriggerEvent {
	q.mu.Lock()
	dst = append(dst, q.ready...)
	clear(q.ready)
	q.ready = q.ready[:0]
	q.mu.Unlock()
	return dst
}

func (q *triggerEventQueue) reset() {
	q.mu.Lock()
	clear(q.pending)
	clear(q.ready)
	q.pending = nil
	q.ready = nil
	q.mu.Unlock()
}

func (q *triggerEventQueue) cache() {
	q.mu.Lock()
	q.ready = append(q.ready, q.pending...)
	clear(q.pending)
	q.pending = q.pending[:0]
	q.mu.Unlock()
}
