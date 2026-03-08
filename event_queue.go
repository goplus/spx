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
)

type eventQueuePolicy = coreevent.QueuePolicy

const (
	eventQueueDropNewest eventQueuePolicy = coreevent.QueueDropNewest
	eventQueueDropOldest eventQueuePolicy = coreevent.QueueDropOldest
	eventQueueBlock      eventQueuePolicy = coreevent.QueueBlock
)

const defaultEventQueuePolicy = coreevent.DefaultQueuePolicy

func parseEventQueuePolicy(policy string) eventQueuePolicy {
	return coreevent.ParsePolicy(policy)
}

type eventQueueStats = coreevent.QueueStats
type eventQueueSnapshot = coreevent.QueueSnapshot

func (p *Game) initEventQueueState() {
	p.EventQueuePolicy = defaultEventQueuePolicy
	p.EventQueueStats.Reset()
}

func (p *Game) resetEventQueueStats() {
	p.EventQueueStats.Reset()
}

func (p *Game) setEventQueuePolicy(policy eventQueuePolicy) {
	p.EventQueuePolicy = policy
}

func (p *Game) eventQueueSnapshot() eventQueueSnapshot {
	queueLen, queueCap := 0, 0
	if p.events != nil {
		queueLen = len(p.events)
		queueCap = cap(p.events)
	}
	return coreevent.Snapshot(p.EventQueuePolicy, &p.EventQueueStats, queueLen, queueCap)
}

func (p *Game) tryEnqueueEvent(ev event) bool {
	return coreevent.TryEnqueue(p.events, ev, &p.EventQueueStats)
}

func (p *Game) queueEventWithPolicy(ev event) bool {
	return coreevent.EnqueueWithPolicy(p.events, ev, p.EventQueuePolicy, &p.EventQueueStats)
}
