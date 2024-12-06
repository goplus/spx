package coroutine

import "sync"

type Queue[T any] struct {
	mu sync.Mutex
	// TODO Use a linked list to avoid moving a whole
	// block of memory when dequeuing
	tasks []T
}

func NewQueue[T any]() *Queue[T] {
	return &Queue[T]{
		tasks: make([]T, 0),
	}
}

// Move all tasks from the src queue to the current queue.
// Afterward, the src queue will be empty.
func (s *Queue[T]) Move(src *Queue[T]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src.mu.Lock()
	defer src.mu.Unlock()
	s.tasks = append(s.tasks, src.tasks...)
	src.tasks = src.tasks[:0]
}

func (s *Queue[T]) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.tasks)
}

func (s *Queue[T]) PushBack(value T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks = append(s.tasks, value)
}

func (s *Queue[T]) PushFront(value T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks = append([]T{value}, s.tasks...)
}

func (s *Queue[T]) PopFront() T {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.tasks) == 0 {
		panic("queue is empty")
	}
	value := s.tasks[0]
	s.tasks = s.tasks[1:]
	return value
}

func (s *Queue[T]) PopBack() T {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.tasks) == 0 {
		panic("queue is empty")
	}
	lastIdx := len(s.tasks) - 1
	value := s.tasks[lastIdx]
	s.tasks = s.tasks[:lastIdx]
	return value
}
