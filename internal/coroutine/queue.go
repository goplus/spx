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

import "sync"

type node[T any] struct {
	value T
	prev  *node[T]
	next  *node[T]
}

// Queue is a thread-safe double-ended queue. Its zero value is ready for use.
type Queue[T any] struct {
	mu    sync.Mutex
	head  *node[T]
	tail  *node[T]
	count int
	pool  sync.Pool
}

// NewQueue creates an empty queue.
func NewQueue[T any]() *Queue[T] {
	q := &Queue[T]{}
	q.pool.New = func() any {
		return new(node[T])
	}
	return q
}

// Move appends every value from src to the receiving queue and leaves src
// empty. Source and receiver must be different queues; concurrent moves between
// the same two queues must use a consistent direction.
func (q *Queue[T]) Move(src *Queue[T]) {
	q.mu.Lock()
	defer q.mu.Unlock()
	src.mu.Lock()
	defer src.mu.Unlock()

	if src.count == 0 {
		return
	}

	if q.count == 0 {
		q.head = src.head
		q.tail = src.tail
	} else {
		q.tail.next = src.head
		src.head.prev = q.tail
		q.tail = src.tail
	}
	q.count += src.count

	src.head = nil
	src.tail = nil
	src.count = 0
}

// Count returns the number of values in the queue.
func (q *Queue[T]) Count() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.count
}

// PushBack adds value to the back of the queue.
func (q *Queue[T]) PushBack(value T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	newNode := q.acquireNode(value)
	if q.count == 0 {
		q.head = newNode
		q.tail = newNode
	} else {
		newNode.prev = q.tail
		q.tail.next = newNode
		q.tail = newNode
	}
	q.count++
}

// PushFront adds value to the front of the queue.
func (q *Queue[T]) PushFront(value T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	newNode := q.acquireNode(value)
	if q.count == 0 {
		q.head = newNode
		q.tail = newNode
	} else {
		newNode.next = q.head
		q.head.prev = newNode
		q.head = newNode
	}
	q.count++
}

// PopFront removes and returns the front value. It panics if the queue is empty.
func (q *Queue[T]) PopFront() T {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.count == 0 {
		panic("queue is empty")
	}

	n := q.head
	value := n.value
	q.head = n.next
	q.count--

	if q.count == 0 {
		q.tail = nil
	} else {
		q.head.prev = nil
	}

	q.releaseNode(n)
	return value
}

// PopBack removes and returns the back value. It panics if the queue is empty.
func (q *Queue[T]) PopBack() T {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.count == 0 {
		panic("queue is empty")
	}

	n := q.tail
	value := n.value
	q.tail = n.prev
	q.count--

	if q.count == 0 {
		q.head = nil
	} else {
		q.tail.next = nil
	}

	q.releaseNode(n)
	return value
}

func (q *Queue[T]) ensurePool() {
	if q.pool.New == nil {
		q.pool.New = func() any {
			return new(node[T])
		}
	}
}

func (q *Queue[T]) acquireNode(value T) *node[T] {
	q.ensurePool()
	n := q.pool.Get().(*node[T])
	n.value = value
	n.prev = nil
	n.next = nil
	return n
}

func (q *Queue[T]) releaseNode(n *node[T]) {
	var zero T
	n.value = zero
	n.prev = nil
	n.next = nil
	q.ensurePool()
	q.pool.Put(n)
}
