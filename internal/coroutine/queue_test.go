package coroutine

import "testing"

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
	if got := q.PopBack(); got != 2 {
		t.Fatalf("PopBack = %d, want 2", got)
	}
}

func TestQueueSupportsZeroValueWithPooling(t *testing.T) {
	var q Queue[int]

	q.PushBack(7)
	if got := q.PopFront(); got != 7 {
		t.Fatalf("PopFront = %d, want 7", got)
	}

	q.PushFront(9)
	if got := q.PopBack(); got != 9 {
		t.Fatalf("PopBack = %d, want 9", got)
	}
}
