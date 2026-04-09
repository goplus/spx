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

type Queue[T any] struct {
	mu    sync.Mutex
	head  *node[T]
	tail  *node[T]
	count int
	pool  sync.Pool
}

func NewQueue[T any]() *Queue[T] {
	q := &Queue[T]{}
	q.pool.New = func() any {
		return new(node[T])
	}
	return q
}

func (s *Queue[T]) ensurePool() {
	if s.pool.New == nil {
		s.pool.New = func() any {
			return new(node[T])
		}
	}
}

func (s *Queue[T]) acquireNode(value T) *node[T] {
	s.ensurePool()
	n := s.pool.Get().(*node[T])
	n.value = value
	n.prev = nil
	n.next = nil
	return n
}

func (s *Queue[T]) releaseNode(n *node[T]) {
	var zero T
	n.value = zero
	n.prev = nil
	n.next = nil
	s.ensurePool()
	s.pool.Put(n)
}

// Move all tasks from the src queue to the current queue.
// Afterward, the src queue will be empty.
func (s *Queue[T]) Move(src *Queue[T]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src.mu.Lock()
	defer src.mu.Unlock()

	if src.count == 0 {
		return
	}

	if s.count == 0 {
		s.head = src.head
		s.tail = src.tail
	} else {
		s.tail.next = src.head
		src.head.prev = s.tail
		s.tail = src.tail
	}
	s.count += src.count

	// Clear source queue
	src.head = nil
	src.tail = nil
	src.count = 0
}

func (s *Queue[T]) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

func (s *Queue[T]) PushBack(value T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newNode := s.acquireNode(value)
	if s.count == 0 {
		s.head = newNode
		s.tail = newNode
	} else {
		newNode.prev = s.tail
		s.tail.next = newNode
		s.tail = newNode
	}
	s.count++
}

func (s *Queue[T]) PushFront(value T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newNode := s.acquireNode(value)
	if s.count == 0 {
		s.head = newNode
		s.tail = newNode
	} else {
		newNode.next = s.head
		s.head.prev = newNode
		s.head = newNode
	}
	s.count++
}

func (s *Queue[T]) PopFront() T {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.count == 0 {
		panic("queue is empty")
	}

	n := s.head
	value := n.value
	s.head = n.next
	s.count--

	if s.count == 0 {
		s.tail = nil
	} else {
		s.head.prev = nil
	}

	s.releaseNode(n)
	return value
}

func (s *Queue[T]) PopBack() T {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.count == 0 {
		panic("queue is empty")
	}

	n := s.tail
	value := n.value
	s.tail = n.prev
	s.count--

	if s.count == 0 {
		s.head = nil
	} else {
		s.tail.next = nil
	}

	s.releaseNode(n)
	return value
}
