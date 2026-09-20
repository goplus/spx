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
	BucketAnyKeyPressed
	BucketSwipe
	BucketIReceive
	BucketBackdropChanged
	BucketCloned
	BucketTouchStart
	BucketTouching
	BucketTouchEnd
	BucketClick
	BucketTimer
	BucketCondition

	bucketCount
)

// Manager groups sinks by Bucket and tracks the one-time start lifecycle.
// Published slice elements are never changed: registration appends after the
// existing prefix, deletion copies survivors, and Reset drops slice references.
type Manager struct {
	mu      sync.RWMutex
	buckets [bucketCount][]Sink

	startFired bool
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

func (m *Manager) Add(bucket Bucket, sink Sink) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buckets[bucket] = append(m.buckets[bucket], sink)
}

func (m *Manager) TryAddStart(sink Sink) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startFired {
		return false
	}
	m.buckets[BucketStart] = append(m.buckets[BucketStart], sink)
	return true
}

// Snapshot returns a shallow, read-only view of the current registrations.
// Callers must not assign its elements. The view remains stable across manager
// writes, and its capacity is limited so appending cannot change the bucket.
// Objects referenced by a Sink, including its Handler, are not made immutable.
func (m *Manager) Snapshot(bucket Bucket) []Sink {
	m.mu.RLock()
	out := readOnlySnapshot(m.buckets[bucket])
	m.mu.RUnlock()
	return out
}

// SnapshotStartOnce closes start registration and returns its first snapshot.
// The returned slice has the same read-only, shallow-view contract as Snapshot.
func (m *Manager) SnapshotStartOnce() []Sink {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startFired {
		return nil
	}
	m.startFired = true
	return readOnlySnapshot(m.buckets[BucketStart])
}

func readOnlySnapshot(sinks []Sink) []Sink {
	return sinks[:len(sinks):len(sinks)]
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
