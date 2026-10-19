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
	"testing"
)

func TestQueueSortStable(t *testing.T) {
	type entry struct{ key int }
	a, b, c := &entry{2}, &entry{1}, &entry{2}
	var q queue[*entry]
	compare := func(a, b *entry) int { return cmp.Compare(a.key, b.key) }
	q.sortStable(compare)
	if _, ok := q.peekFront(); ok {
		t.Fatal("empty queue has a front value")
	}

	for range 2 {
		for _, value := range []*entry{a, b, c} {
			q.pushBack(value)
		}
		q.sortStable(compare)
		if first, ok := q.peekFront(); !ok || first != b || q.len() != 3 {
			t.Fatal("PeekFront changed the sorted queue")
		}
		if q.popFront() != b || q.popFront() != a || q.popFront() != c {
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
	q := newQueue[int]()
	q.pushBack(1)
	q.pushBack(2)
	q.pushFront(0)

	if got := q.popFront(); got != 0 {
		t.Fatalf("first PopFront = %d, want 0", got)
	}
	if got := q.popFront(); got != 1 {
		t.Fatalf("second PopFront = %d, want 1", got)
	}
	if got := q.popFront(); got != 2 {
		t.Fatalf("third PopFront = %d, want 2", got)
	}
}

func TestQueuePopFrontPanicsWhenEmpty(t *testing.T) {
	var q queue[int]
	defer func() {
		if recovered := recover(); recovered != "queue is empty" {
			t.Fatalf("empty PopFront panic = %v, want queue is empty", recovered)
		}
	}()
	q.popFront()
}

func TestQueueSupportsZeroValueWithPooling(t *testing.T) {
	var q queue[int]

	q.pushBack(7)
	if got := q.popFront(); got != 7 {
		t.Fatalf("PopFront = %d, want 7", got)
	}

	q.pushFront(9)
	if got := q.popFront(); got != 9 {
		t.Fatalf("PopFront = %d, want 9", got)
	}
}

func TestQueueMoveToSelfIsNoOp(t *testing.T) {
	q := newQueue[int]()
	q.pushBack(1)
	q.pushBack(2)

	q.appendFrom(q)

	if got := q.len(); got != 2 {
		t.Fatalf("Count after self move = %d, want 2", got)
	}
	if got := q.popFront(); got != 1 {
		t.Fatalf("first PopFront after self move = %d, want 1", got)
	}
	if got := q.popFront(); got != 2 {
		t.Fatalf("second PopFront after self move = %d, want 2", got)
	}
}

func TestQueueAnyDoesNotChangeOrder(t *testing.T) {
	q := newQueue[int]()
	q.pushBack(1)
	q.pushBack(2)

	if !q.any(func(value int) bool { return value%2 == 0 }) {
		t.Fatal("Any did not find a matching value")
	}
	if q.any(func(value int) bool { return value > 2 }) {
		t.Fatal("Any found a value that is not queued")
	}
	if first, second := q.popFront(), q.popFront(); first != 1 || second != 2 {
		t.Fatalf("queue order after Any = %d, %d", first, second)
	}
}

func TestQueueRandomOperations(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	var queues [2]queue[int]
	var expected [2][]int
	for step := range 2000 {
		i := random.IntN(len(queues))
		q := &queues[i]
		value := random.IntN(100)
		switch random.IntN(7) {
		case 0:
			q.pushBack(value)
			expected[i] = append(expected[i], value)
		case 1:
			q.pushFront(value)
			expected[i] = slices.Insert(expected[i], 0, value)
		case 2:
			if len(expected[i]) > 0 {
				if got := q.popFront(); got != expected[i][0] {
					t.Fatalf("step %d: PopFront = %d, want %d", step, got, expected[i][0])
				}
				expected[i] = expected[i][1:]
			}
		case 3:
			other := 1 - i
			q.appendFrom(&queues[other])
			expected[i] = append(expected[i], expected[other]...)
			expected[other] = nil
		case 4:
			q.sortStable(cmp.Compare[int])
			slices.Sort(expected[i])
		case 5:
			if got := q.any(func(v int) bool { return v == value }); got != slices.Contains(expected[i], value) {
				t.Fatalf("step %d: Any(%d) = %v", step, value, got)
			}
		case 6:
			q.appendFrom(q)
		}
		for j := range queues {
			if got := queues[j].len(); got != len(expected[j]) {
				t.Fatalf("step %d: queue %d Count = %d, want %d", step, j, got, len(expected[j]))
			}
			if first, ok := queues[j].peekFront(); ok != (len(expected[j]) > 0) || ok && first != expected[j][0] {
				t.Fatalf("step %d: queue %d PeekFront = %d, %v, values = %v", step, j, first, ok, expected[j])
			}
		}
	}
	for i := range queues {
		for _, want := range expected[i] {
			if got := queues[i].popFront(); got != want {
				t.Fatalf("queue %d final PopFront = %d, want %d", i, got, want)
			}
		}
	}
}
