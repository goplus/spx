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

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

type eventTestHarness struct {
	subscribe      func(func([2]int)) int
	unsubscribe    func(int)
	unsubscribeAll func()
	trigger        func(int)
	arguments      func(int) [2]int
}

func eventTestCases() map[string]func() eventTestHarness {
	return map[string]func() eventTestHarness{
		"Event0": func() eventTestHarness {
			e := NewEvent0()
			return eventTestHarness{
				subscribe:   func(fn func([2]int)) int { return e.Subscribe(func() { fn([2]int{}) }) },
				unsubscribe: e.Unsubscribe, unsubscribeAll: e.UnsubscribeAll,
				trigger: func(int) { e.Trigger() }, arguments: func(int) [2]int { return [2]int{} },
			}
		},
		"Event1": func() eventTestHarness {
			e := NewEvent1[int]()
			return eventTestHarness{
				subscribe:   func(fn func([2]int)) int { return e.Subscribe(func(a int) { fn([2]int{a}) }) },
				unsubscribe: e.Unsubscribe, unsubscribeAll: e.UnsubscribeAll,
				trigger: e.Trigger, arguments: func(a int) [2]int { return [2]int{a} },
			}
		},
		"Event2": func() eventTestHarness {
			e := NewEvent2[int, int]()
			return eventTestHarness{
				subscribe:   func(fn func([2]int)) int { return e.Subscribe(func(a, b int) { fn([2]int{a, b}) }) },
				unsubscribe: e.Unsubscribe, unsubscribeAll: e.UnsubscribeAll,
				trigger: func(a int) { e.Trigger(a, -a) }, arguments: func(a int) [2]int { return [2]int{a, -a} },
			}
		},
	}
}

func TestEventTriggerReentryPreservesOuterSnapshot(t *testing.T) {
	for name, create := range eventTestCases() {
		t.Run(name, func(t *testing.T) {
			e := create()
			type call struct {
				listener int
				args     [2]int
			}
			var got []call
			nested := false
			e.subscribe(func(args [2]int) {
				got = append(got, call{1, args})
				if !nested {
					nested = true
					e.trigger(20)
					nested = false
				}
			})
			e.subscribe(func(args [2]int) { got = append(got, call{2, args}) })
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("Trigger panicked: %v; calls=%v", p, got)
				}
			}()
			e.trigger(10)
			want := []call{{1, e.arguments(10)}, {1, e.arguments(20)}, {2, e.arguments(20)}, {2, e.arguments(10)}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("calls=%v, want %v", got, want)
			}
		})
	}
}

func TestEventTriggerKeepsSubscriptionSnapshot(t *testing.T) {
	for name, create := range eventTestCases() {
		t.Run(name, func(t *testing.T) {
			e := create()
			var got []int
			var second int
			e.subscribe(func([2]int) {
				got = append(got, 1)
				e.unsubscribe(second)
				e.unsubscribeAll()
				e.subscribe(func([2]int) { got = append(got, 3) })
			})
			second = e.subscribe(func([2]int) { got = append(got, 2) })
			e.trigger(1)
			e.trigger(2)
			if want := []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
				t.Fatalf("calls=%v, want %v", got, want)
			}
		})
	}
}

func TestEventTriggerPanicDoesNotPoisonNextDispatch(t *testing.T) {
	for name, create := range eventTestCases() {
		t.Run(name, func(t *testing.T) {
			e := create()
			id := e.subscribe(func([2]int) { panic("listener panic") })
			func() {
				defer func() {
					if got := recover(); got != "listener panic" {
						t.Fatalf("panic=%v", got)
					}
				}()
				e.trigger(1)
			}()
			e.unsubscribe(id)
			calls := 0
			e.subscribe(func([2]int) { calls++ })
			e.trigger(2)
			if calls != 1 {
				t.Fatalf("calls=%d, want 1", calls)
			}
		})
	}
}

func TestEventConcurrentTriggersOwnTheirSnapshots(t *testing.T) {
	for name, create := range eventTestCases() {
		t.Run(name, func(t *testing.T) {
			e := create()
			const workers = 8
			var first, second atomic.Int32
			entered := make(chan struct{}, workers)
			release := make(chan struct{})
			e.subscribe(func([2]int) {
				if first.Add(1) <= workers {
					entered <- struct{}{}
				}
				<-release
			})
			e.subscribe(func([2]int) { second.Add(1) })
			var done sync.WaitGroup
			for i := range workers {
				done.Go(func() { e.trigger(i) })
			}
			for range workers {
				<-entered
			}
			close(release)
			done.Wait()
			if first.Load() != workers || second.Load() != workers {
				t.Fatalf("listener calls=%d,%d, want %d each", first.Load(), second.Load(), workers)
			}
		})
	}
}
