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

package event

import (
	"sync"
	"sync/atomic"
	"time"
)

type QueuePolicy int

const (
	QueueDropNewest QueuePolicy = iota
	QueueDropOldest
	QueueBlock
)

const DefaultQueuePolicy = QueueDropNewest

func (p QueuePolicy) String() string {
	switch p {
	case QueueDropOldest:
		return "DropOldest"
	case QueueBlock:
		return "Block"
	default:
		return "DropNewest"
	}
}

func ParsePolicy(policy string) QueuePolicy {
	switch policy {
	case "drop_oldest", "dropoldest", "DropOldest":
		return QueueDropOldest
	case "block", "Block":
		return QueueBlock
	default:
		return DefaultQueuePolicy
	}
}

type QueueStats struct {
	enqueuedTotal  atomic.Uint64
	droppedTotal   atomic.Uint64
	maxQueueLen    atomic.Int64
	lastDropUnixNs atomic.Int64
}

func (s *QueueStats) Reset() {
	s.enqueuedTotal.Store(0)
	s.droppedTotal.Store(0)
	s.maxQueueLen.Store(0)
	s.lastDropUnixNs.Store(0)
}

func (s *QueueStats) OnEnqueue(queueLen int) {
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

func (s *QueueStats) OnDrop() {
	s.droppedTotal.Add(1)
	s.lastDropUnixNs.Store(time.Now().UnixNano())
}

func (s *QueueStats) EnqueuedTotal() uint64 {
	return s.enqueuedTotal.Load()
}

func (s *QueueStats) DroppedTotal() uint64 {
	return s.droppedTotal.Load()
}

func (s *QueueStats) MaxQueueLen() int {
	return int(s.maxQueueLen.Load())
}

func (s *QueueStats) LastDropAt() time.Time {
	if ts := s.lastDropUnixNs.Load(); ts > 0 {
		return time.Unix(0, ts)
	}
	return time.Time{}
}

type QueueSnapshot struct {
	Policy          string
	QueueLen        int
	QueueCap        int
	MaxQueueLenSeen int
	EnqueuedTotal   uint64
	DroppedTotal    uint64
	LastDropAt      time.Time
}

func Snapshot(policy QueuePolicy, stats *QueueStats, queueLen, queueCap int) QueueSnapshot {
	return QueueSnapshot{
		Policy:          policy.String(),
		QueueLen:        queueLen,
		QueueCap:        queueCap,
		MaxQueueLenSeen: stats.MaxQueueLen(),
		EnqueuedTotal:   stats.EnqueuedTotal(),
		DroppedTotal:    stats.DroppedTotal(),
		LastDropAt:      stats.LastDropAt(),
	}
}

func TryEnqueue[T any](ch chan T, item T, stats *QueueStats) bool {
	select {
	case ch <- item:
		stats.OnEnqueue(len(ch))
		return true
	default:
		return false
	}
}

func EnqueueWithPolicy[T any](ch chan T, item T, policy QueuePolicy, stats *QueueStats, dropOldestMu *sync.Mutex) bool {
	if ch == nil {
		return false
	}
	if TryEnqueue(ch, item, stats) {
		return true
	}

	switch policy {
	case QueueDropOldest:
		if dropOldestMu != nil {
			dropOldestMu.Lock()
			defer dropOldestMu.Unlock()
		}
		if TryEnqueue(ch, item, stats) {
			return true
		}
		select {
		case <-ch:
			stats.OnDrop()
		default:
		}
		if TryEnqueue(ch, item, stats) {
			return true
		}
		stats.OnDrop()
		return false
	case QueueBlock:
		ch <- item
		stats.OnEnqueue(len(ch))
		return true
	default:
		stats.OnDrop()
		return false
	}
}
