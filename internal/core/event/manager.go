package event

import (
	"slices"
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

func deleteOwnerCopy(sinks []Sink, owner any) []Sink {
	if len(sinks) == 0 {
		return nil
	}

	out := make([]Sink, 0, len(sinks))
	for _, sink := range sinks {
		if sink.Owner != owner {
			out = append(out, sink)
		}
	}
	if len(out) == len(sinks) {
		return sinks
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (m *Manager) Add(bucket Bucket, sink Sink) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buckets[bucket] = append(m.buckets[bucket], sink)
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
	m.buckets[BucketStart] = append(m.buckets[BucketStart], sink)
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
	out := slices.Clone(m.buckets[bucket])
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
	return slices.Clone(m.buckets[BucketStart])
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
