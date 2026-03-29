package event

import "sync"

type DispatchHooks struct {
	Spawn func(start bool, owner any, call func())
	Wait  func(func())
}

func (m *Manager) DispatchBucketAsync(bucket Bucket, start bool, data any, hooks DispatchHooks, do func(*Sink)) {
	if m == nil {
		return
	}
	DispatchAsync(m.Snapshot(bucket), start, data, hooks, do)
}

func (m *Manager) DispatchBucketSync(bucket Bucket, data any, hooks DispatchHooks, do func(*Sink)) {
	if m == nil {
		return
	}
	DispatchSync(m.Snapshot(bucket), data, hooks, do)
}

func (m *Manager) DispatchBucket(bucket Bucket, wait bool, data any, hooks DispatchHooks, do func(*Sink)) {
	if m == nil {
		return
	}
	Dispatch(m.Snapshot(bucket), wait, data, hooks, do)
}

func (m *Manager) DispatchStartOnce(start bool, data any, hooks DispatchHooks, do func(*Sink)) {
	if m == nil {
		return
	}
	DispatchAsync(m.SnapshotStartOnce(), start, data, hooks, do)
}

func DispatchAsync(sinks []Sink, start bool, data any, hooks DispatchHooks, do func(*Sink)) {
	for _, sink := range sinks {
		sink := sink
		if sink.Cond != nil && !sink.Cond(data) {
			continue
		}
		dispatchSink(hooks, start, sink, do)
	}
}

func DispatchSync(sinks []Sink, data any, hooks DispatchHooks, do func(*Sink)) {
	var wg sync.WaitGroup
	for _, sink := range sinks {
		sink := sink
		if sink.Cond != nil && !sink.Cond(data) {
			continue
		}
		wg.Add(1)
		dispatchSink(hooks, false, sink, func(sink *Sink) {
			defer wg.Done()
			do(sink)
		})
	}
	if hooks.Wait != nil {
		hooks.Wait(wg.Wait)
		return
	}
	wg.Wait()
}

func Dispatch(sinks []Sink, wait bool, data any, hooks DispatchHooks, do func(*Sink)) {
	if wait {
		DispatchSync(sinks, data, hooks, do)
		return
	}
	DispatchAsync(sinks, false, data, hooks, do)
}

func dispatchSink(hooks DispatchHooks, start bool, sink Sink, do func(*Sink)) {
	if hooks.Spawn != nil {
		hooks.Spawn(start, sink.Owner, func() {
			do(&sink)
		})
		return
	}
	do(&sink)
}
