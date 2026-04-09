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

import "sync"

type DispatchHooks struct {
	Spawn func(start bool, owner any, call func())
	Wait  func(func())
}

// DispatchBucketAsync snapshots the requested bucket and dispatches matching sinks asynchronously.
// A nil Manager is treated as a no-op.
func (m *Manager) DispatchBucketAsync(bucket Bucket, start bool, data any, hooks DispatchHooks, do func(*Sink)) {
	if m == nil {
		return
	}
	DispatchAsync(m.Snapshot(bucket), start, data, hooks, do)
}

// DispatchBucketSync snapshots the requested bucket and dispatches matching sinks synchronously.
// A nil Manager is treated as a no-op.
func (m *Manager) DispatchBucketSync(bucket Bucket, data any, hooks DispatchHooks, do func(*Sink)) {
	if m == nil {
		return
	}
	DispatchSync(m.Snapshot(bucket), data, hooks, do)
}

// DispatchBucket snapshots the requested bucket and dispatches matching sinks.
// When wait is true it waits for synchronous completion; otherwise it dispatches asynchronously.
// A nil Manager is treated as a no-op.
func (m *Manager) DispatchBucket(bucket Bucket, wait bool, data any, hooks DispatchHooks, do func(*Sink)) {
	if m == nil {
		return
	}
	Dispatch(m.Snapshot(bucket), wait, data, hooks, do)
}

// DispatchStartOnce dispatches BucketStart sinks at most once per Manager lifetime.
// After the first call, subsequent calls are no-ops.
// A nil Manager is treated as a no-op.
func (m *Manager) DispatchStartOnce(data any, hooks DispatchHooks, do func(*Sink)) {
	if m == nil {
		return
	}
	DispatchAsync(m.SnapshotStartOnce(), false, data, hooks, do)
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
