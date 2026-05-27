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
)

type Sink struct {
	Owner   any
	Cond    func(any) bool
	Handler any
}

type Bucket int

const (
	BucketStart Bucket = iota
	BucketAwake
	BucketKeyPressed
	BucketSwipe
	BucketIReceive
	BucketBackdropChanged
	BucketCloned
	BucketTouchStart
	BucketTouching
	BucketTouchEnd
	BucketClick
	BucketTimer

	bucketCount
)

// Manager groups event sinks by Bucket and tracks the one-time BucketStart lifecycle.
// DispatchBucketAsync, DispatchBucketSync, DispatchBucket, and DispatchStartOnce
// treat a nil receiver as a no-op.
type Manager struct {
	mu      sync.RWMutex
	buckets [bucketCount][]Sink

	startFired bool
}

func appendSinkCopy(sinks []Sink, sink Sink) []Sink {
	out := make([]Sink, len(sinks)+1)
	copy(out, sinks)
	out[len(sinks)] = sink
	return out
}

func readOnlySnapshot(sinks []Sink) []Sink {
	if len(sinks) == 0 {
		return sinks
	}
	return sinks[:len(sinks):len(sinks)]
}

func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.buckets {
		m.buckets[i] = nil
	}
	m.startFired = false
}

func (m *Manager) DeleteOwner(owner any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.buckets {
		m.buckets[i] = deleteOwnerCopy(m.buckets[i], owner)
	}
}

func deleteOwnerCopy(sinks []Sink, owner any) []Sink {
	if len(sinks) == 0 {
		return nil
	}

	firstMatch := -1
	for i, sink := range sinks {
		if sink.Owner == owner {
			firstMatch = i
			break
		}
	}
	if firstMatch < 0 {
		return sinks
	}

	out := make([]Sink, 0, len(sinks)-1)
	out = append(out, sinks[:firstMatch]...)
	for _, sink := range sinks[firstMatch+1:] {
		if sink.Owner != owner {
			out = append(out, sink)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func HasOwner(sinks []Sink, owner any) bool {
	for _, sink := range sinks {
		if sink.Owner == owner {
			return true
		}
	}
	return false
}

func (m *Manager) Add(bucket Bucket, sink Sink) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buckets[bucket] = appendSinkCopy(m.buckets[bucket], sink)
}

func (m *Manager) AddStart(sink Sink) {
	m.Add(BucketStart, sink)
}

func (m *Manager) TryAddStart(sink Sink) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startFired {
		return false
	}
	m.buckets[BucketStart] = appendSinkCopy(m.buckets[BucketStart], sink)
	return true
}

func (m *Manager) AddAwake(sink Sink) {
	m.Add(BucketAwake, sink)
}

func (m *Manager) AddKeyPressed(sink Sink) {
	m.Add(BucketKeyPressed, sink)
}

func (m *Manager) AddSwipe(sink Sink) {
	m.Add(BucketSwipe, sink)
}

func (m *Manager) AddIReceive(sink Sink) {
	m.Add(BucketIReceive, sink)
}

func (m *Manager) AddBackdropChanged(sink Sink) {
	m.Add(BucketBackdropChanged, sink)
}

func (m *Manager) AddCloned(sink Sink) {
	m.Add(BucketCloned, sink)
}

func (m *Manager) AddTouchStart(sink Sink) {
	m.Add(BucketTouchStart, sink)
}

func (m *Manager) AddTouching(sink Sink) {
	m.Add(BucketTouching, sink)
}

func (m *Manager) AddTouchEnd(sink Sink) {
	m.Add(BucketTouchEnd, sink)
}

func (m *Manager) AddClick(sink Sink) {
	m.Add(BucketClick, sink)
}

func (m *Manager) AddTimer(sink Sink) {
	m.Add(BucketTimer, sink)
}

func (m *Manager) Snapshot(bucket Bucket) []Sink {
	m.mu.RLock()
	out := readOnlySnapshot(m.buckets[bucket])
	m.mu.RUnlock()
	return out
}

func (m *Manager) SnapshotStart() []Sink {
	return m.Snapshot(BucketStart)
}

func (m *Manager) SnapshotStartOnce() []Sink {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startFired {
		return nil
	}
	m.startFired = true
	return readOnlySnapshot(m.buckets[BucketStart])
}

func (m *Manager) SnapshotAwake() []Sink {
	return m.Snapshot(BucketAwake)
}

func (m *Manager) SnapshotKeyPressed() []Sink {
	return m.Snapshot(BucketKeyPressed)
}

func (m *Manager) SnapshotSwipe() []Sink {
	return m.Snapshot(BucketSwipe)
}

func (m *Manager) SnapshotIReceive() []Sink {
	return m.Snapshot(BucketIReceive)
}

func (m *Manager) SnapshotBackdropChanged() []Sink {
	return m.Snapshot(BucketBackdropChanged)
}

func (m *Manager) SnapshotCloned() []Sink {
	return m.Snapshot(BucketCloned)
}

func (m *Manager) SnapshotTouchStart() []Sink {
	return m.Snapshot(BucketTouchStart)
}

func (m *Manager) SnapshotTouching() []Sink {
	return m.Snapshot(BucketTouching)
}

func (m *Manager) SnapshotTouchEnd() []Sink {
	return m.Snapshot(BucketTouchEnd)
}

func (m *Manager) SnapshotClick() []Sink {
	return m.Snapshot(BucketClick)
}

func (m *Manager) SnapshotTimer() []Sink {
	return m.Snapshot(BucketTimer)
}
