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
	"slices"
	"sync"
)

type node[T any] struct {
	value T
	next  *node[T]
}

// queue is a deque whose caller owns synchronization. Its zero value is ready for use.
type queue[T any] struct {
	head  *node[T]
	tail  *node[T]
	count int
	pool  *sync.Pool

	sortBuffer []T
}

func newQueue[T any]() *queue[T] {
	q := &queue[T]{}
	q.ensurePool()
	return q
}

func (q *queue[T]) len() int {
	return q.count
}

func (q *queue[T]) peekFront() (value T, ok bool) {
	if q.head == nil {
		return value, false
	}
	return q.head.value, true
}

func (q *queue[T]) pushBack(value T) {
	newNode := q.acquireNode(value)
	if q.count == 0 {
		q.head = newNode
	} else {
		q.tail.next = newNode
	}
	q.tail = newNode
	q.count++
}

func (q *queue[T]) pushFront(value T) {
	newNode := q.acquireNode(value)
	newNode.next = q.head
	q.head = newNode
	if q.count == 0 {
		q.tail = newNode
	}
	q.count++
}

// popFront removes and returns the front value. It panics if the queue is empty.
func (q *queue[T]) popFront() T {
	if q.count == 0 {
		panic("queue is empty")
	}

	n := q.head
	value := n.value
	q.head = n.next
	q.count--

	if q.count == 0 {
		q.tail = nil
	}

	q.releaseNode(n)
	return value
}

// appendFrom appends every value from src to the receiving queue and empties src.
func (q *queue[T]) appendFrom(src *queue[T]) {
	if q == src {
		return
	}
	if src.count == 0 {
		return
	}

	if q.count == 0 {
		q.head = src.head
	} else {
		q.tail.next = src.head
	}
	q.tail = src.tail
	q.count += src.count

	src.head = nil
	src.tail = nil
	src.count = 0
}

func (q *queue[T]) any(match func(T) bool) bool {
	for node := q.head; node != nil; node = node.next {
		if match(node.value) {
			return true
		}
	}
	return false
}

func (q *queue[T]) sortStable(compare func(T, T) int) {
	if q.count < 2 {
		return
	}
	values := slices.Grow(q.sortBuffer[:0], q.count)
	for n := q.head; n != nil; n = n.next {
		values = append(values, n.value)
	}
	q.sortBuffer = values
	defer clear(values)
	slices.SortStableFunc(values, compare)
	n := q.head
	for _, value := range values {
		n.value = value
		n = n.next
	}
}

func (q *queue[T]) ensurePool() {
	if q.pool == nil {
		q.pool = &sync.Pool{New: func() any {
			return new(node[T])
		}}
	}
}

func (q *queue[T]) acquireNode(value T) *node[T] {
	q.ensurePool()
	n := q.pool.Get().(*node[T])
	n.value = value
	n.next = nil
	return n
}

func (q *queue[T]) releaseNode(n *node[T]) {
	var zero T
	n.value = zero
	n.next = nil
	q.ensurePool()
	q.pool.Put(n)
}
