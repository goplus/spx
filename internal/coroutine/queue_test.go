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

package coroutine

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"
)

func TestQueueSortStable(t *testing.T) {
	type entry struct{ key int }
	a, b, c := &entry{2}, &entry{1}, &entry{2}
	var q Queue[*entry]
	compare := func(a, b *entry) int { return cmp.Compare(a.key, b.key) }
	q.SortStable(compare)
	if _, ok := q.PeekFront(); ok {
		t.Fatal("empty queue has a front value")
	}

	for range 2 {
		for _, value := range []*entry{a, b, c} {
			q.PushBack(value)
		}
		q.SortStable(compare)
		if first, ok := q.PeekFront(); !ok || first != b || q.Count() != 3 {
			t.Fatal("PeekFront changed the sorted queue")
		}
		if q.PopFront() != b || q.PopFront() != a || q.PopFront() != c {
			t.Fatal("sorting lost equal-value order")
		}
		for _, value := range q.sortBuffer {
			if value != nil {
				t.Fatal("sort buffer retains a queued value")
			}
		}
	}
}

func TestQueuePushPopPreservesOrder(t *testing.T) {
	q := NewQueue[int]()
	q.PushBack(1)
	q.PushBack(2)
	q.PushFront(0)

	if got := q.PopFront(); got != 0 {
		t.Fatalf("first PopFront = %d, want 0", got)
	}
	if got := q.PopFront(); got != 1 {
		t.Fatalf("second PopFront = %d, want 1", got)
	}
	if got := q.PopFront(); got != 2 {
		t.Fatalf("third PopFront = %d, want 2", got)
	}
}

func TestQueueSupportsZeroValueWithPooling(t *testing.T) {
	var q Queue[int]

	q.PushBack(7)
	if got := q.PopFront(); got != 7 {
		t.Fatalf("PopFront = %d, want 7", got)
	}

	q.PushFront(9)
	if got := q.PopFront(); got != 9 {
		t.Fatalf("PopFront = %d, want 9", got)
	}
}

func TestQueueMoveToSelfIsNoOp(t *testing.T) {
	q := NewQueue[int]()
	q.PushBack(1)
	q.PushBack(2)

	q.Move(q)

	if got := q.Count(); got != 2 {
		t.Fatalf("Count after self move = %d, want 2", got)
	}
	if got := q.PopFront(); got != 1 {
		t.Fatalf("first PopFront after self move = %d, want 1", got)
	}
	if got := q.PopFront(); got != 2 {
		t.Fatalf("second PopFront after self move = %d, want 2", got)
	}
}

func TestQueueAnyDoesNotChangeOrder(t *testing.T) {
	q := NewQueue[int]()
	q.PushBack(1)
	q.PushBack(2)

	if !q.Any(func(value int) bool { return value%2 == 0 }) {
		t.Fatal("Any did not find a matching value")
	}
	if q.Any(func(value int) bool { return value > 2 }) {
		t.Fatal("Any found a value that is not queued")
	}
	if first, second := q.PopFront(), q.PopFront(); first != 1 || second != 2 {
		t.Fatalf("queue order after Any = %d, %d", first, second)
	}
}

func TestQueueRandomOperations(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	var queues [2]Queue[int]
	var expected [2][]int
	for step := range 2000 {
		i := random.IntN(len(queues))
		q := &queues[i]
		value := random.IntN(100)
		switch random.IntN(7) {
		case 0:
			q.PushBack(value)
			expected[i] = append(expected[i], value)
		case 1:
			q.PushFront(value)
			expected[i] = slices.Insert(expected[i], 0, value)
		case 2:
			if len(expected[i]) > 0 {
				if got := q.PopFront(); got != expected[i][0] {
					t.Fatalf("step %d: PopFront = %d, want %d", step, got, expected[i][0])
				}
				expected[i] = expected[i][1:]
			}
		case 3:
			other := 1 - i
			q.Move(&queues[other])
			expected[i] = append(expected[i], expected[other]...)
			expected[other] = nil
		case 4:
			q.SortStable(cmp.Compare[int])
			slices.Sort(expected[i])
		case 5:
			if got := q.Any(func(v int) bool { return v == value }); got != slices.Contains(expected[i], value) {
				t.Fatalf("step %d: Any(%d) = %v", step, value, got)
			}
		case 6:
			q.Move(q)
		}
		for j := range queues {
			if got := queues[j].Count(); got != len(expected[j]) {
				t.Fatalf("step %d: queue %d Count = %d, want %d", step, j, got, len(expected[j]))
			}
			if first, ok := queues[j].PeekFront(); ok != (len(expected[j]) > 0) || ok && first != expected[j][0] {
				t.Fatalf("step %d: queue %d PeekFront = %d, %v, values = %v", step, j, first, ok, expected[j])
			}
		}
	}
	for i := range queues {
		for _, want := range expected[i] {
			if got := queues[i].PopFront(); got != want {
				t.Fatalf("queue %d final PopFront = %d, want %d", i, got, want)
			}
		}
	}
}

func TestQueueMoveWithConcurrentProducers(t *testing.T) {
	var destination, source Queue[int]
	var producers sync.WaitGroup
	const valuesPerProducer = 100
	for id, q := range []*Queue[int]{&destination, &source, &destination, &source} {
		producers.Go(func() {
			for value := range valuesPerProducer {
				q.PushBack(id*valuesPerProducer + value)
			}
		})
	}
	for range valuesPerProducer {
		destination.Move(&source)
	}
	producers.Wait()
	destination.Move(&source)

	if source.Count() != 0 || destination.Count() != 4*valuesPerProducer {
		t.Fatalf("counts after Move = %d, %d", destination.Count(), source.Count())
	}
	var next [4]int
	for range 4 * valuesPerProducer {
		value := destination.PopFront()
		id, index := value/valuesPerProducer, value%valuesPerProducer
		if id < 0 || id >= len(next) || index != next[id] {
			t.Fatalf("concurrent Move lost producer order or duplicated a value: %d, next = %v", value, next)
		}
		next[id]++
	}
}
