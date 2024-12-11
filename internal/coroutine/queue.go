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
}

func NewQueue[T any]() *Queue[T] {
	return &Queue[T]{}
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

	newNode := &node[T]{value: value}
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

	newNode := &node[T]{value: value}
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

	value := s.head.value
	s.head = s.head.next
	s.count--

	if s.count == 0 {
		s.tail = nil
	} else {
		s.head.prev = nil
	}

	return value
}

func (s *Queue[T]) PopBack() T {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.count == 0 {
		panic("queue is empty")
	}

	value := s.tail.value
	s.tail = s.tail.prev
	s.count--

	if s.count == 0 {
		s.head = nil
	} else {
		s.tail.next = nil
	}

	return value
}
