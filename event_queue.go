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
	"sync/atomic"
	"time"
)

type eventQueuePolicy int

const (
	eventQueueDropNewest eventQueuePolicy = iota
	eventQueueDropOldest
	eventQueueBlock
)

const defaultEventQueuePolicy = eventQueueDropNewest

func (p eventQueuePolicy) String() string {
	switch p {
	case eventQueueDropOldest:
		return "DropOldest"
	case eventQueueBlock:
		return "Block"
	default:
		return "DropNewest"
	}
}

func parseEventQueuePolicy(policy string) eventQueuePolicy {
	switch policy {
	case "drop_oldest", "dropoldest", "DropOldest":
		return eventQueueDropOldest
	case "block", "Block":
		return eventQueueBlock
	default:
		return defaultEventQueuePolicy
	}
}

type eventQueueStats struct {
	enqueuedTotal  atomic.Uint64
	droppedTotal   atomic.Uint64
	maxQueueLen    atomic.Int64
	lastDropUnixNs atomic.Int64
}

func (s *eventQueueStats) reset() {
	s.enqueuedTotal.Store(0)
	s.droppedTotal.Store(0)
	s.maxQueueLen.Store(0)
	s.lastDropUnixNs.Store(0)
}

func (s *eventQueueStats) onEnqueue(queueLen int) {
	s.enqueuedTotal.Add(1)
	cur := int64(queueLen)
	for {
		prev := s.maxQueueLen.Load()
		if cur <= prev {
			return
		}
		if s.maxQueueLen.CompareAndSwap(prev, cur) {
			return
		}
	}
}

func (s *eventQueueStats) onDrop() {
	s.droppedTotal.Add(1)
	s.lastDropUnixNs.Store(time.Now().UnixNano())
}

type eventQueueSnapshot struct {
	Policy          string
	QueueLen        int
	QueueCap        int
	MaxQueueLenSeen int
	EnqueuedTotal   uint64
	DroppedTotal    uint64
	LastDropAt      time.Time
}

func (p *Game) initEventQueueState() {
	p.eventQueuePolicy = defaultEventQueuePolicy
	p.eventQueueStats.reset()
}

func (p *Game) resetEventQueueStats() {
	p.eventQueueStats.reset()
}

func (p *Game) setEventQueuePolicy(policy eventQueuePolicy) {
	p.eventQueuePolicy = policy
}

func (p *Game) eventQueueSnapshot() eventQueueSnapshot {
	s := eventQueueSnapshot{
		Policy:          p.eventQueuePolicy.String(),
		EnqueuedTotal:   p.eventQueueStats.enqueuedTotal.Load(),
		DroppedTotal:    p.eventQueueStats.droppedTotal.Load(),
		MaxQueueLenSeen: int(p.eventQueueStats.maxQueueLen.Load()),
	}
	if p.events != nil {
		s.QueueLen = len(p.events)
		s.QueueCap = cap(p.events)
	}
	if ts := p.eventQueueStats.lastDropUnixNs.Load(); ts > 0 {
		s.LastDropAt = time.Unix(0, ts)
	}
	return s
}

func (p *Game) tryEnqueueEvent(ev event) bool {
	select {
	case p.events <- ev:
		p.eventQueueStats.onEnqueue(len(p.events))
		return true
	default:
		return false
	}
}

func (p *Game) queueEventWithPolicy(ev event) bool {
	if p.events == nil {
		return false
	}
	if p.tryEnqueueEvent(ev) {
		return true
	}

	switch p.eventQueuePolicy {
	case eventQueueDropOldest:
		select {
		case <-p.events:
		default:
		}
		if p.tryEnqueueEvent(ev) {
			return true
		}
		p.eventQueueStats.onDrop()
		return false
	case eventQueueBlock:
		p.events <- ev
		p.eventQueueStats.onEnqueue(len(p.events))
		return true
	default:
		p.eventQueueStats.onDrop()
		return false
	}
}
