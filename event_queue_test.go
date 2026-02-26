package spx

import (
	"testing"
	"time"
)

func newEventQueueTestGame(queueCap int) *Game {
	g := &Game{}
	g.initRuntimeState()
	g.events = make(chan event, queueCap)
	return g
}

func TestEventQueueDropNewest(t *testing.T) {
	resetStateForTest()
	t.Cleanup(resetStateForTest)

	g := newEventQueueTestGame(1)
	g.setEventQueuePolicy(eventQueueDropNewest)

	g.fireEvent(&eventTimer{Time: 1})
	g.fireEvent(&eventTimer{Time: 2})

	snap := g.eventQueueSnapshot()
	if snap.DroppedTotal != 1 {
		t.Fatalf("expected droppedTotal=1, got %d", snap.DroppedTotal)
	}
	if snap.EnqueuedTotal != 1 {
		t.Fatalf("expected enqueuedTotal=1, got %d", snap.EnqueuedTotal)
	}
	if snap.QueueLen != 1 || snap.QueueCap != 1 {
		t.Fatalf("unexpected queue size len/cap=%d/%d", snap.QueueLen, snap.QueueCap)
	}

	ev := (<-g.events).(*eventTimer)
	if ev.Time != 1 {
		t.Fatalf("expected oldest event to remain, got %.0f", ev.Time)
	}
}

func TestEventQueueDropOldest(t *testing.T) {
	resetStateForTest()
	t.Cleanup(resetStateForTest)

	g := newEventQueueTestGame(1)
	g.setEventQueuePolicy(eventQueueDropOldest)

	g.fireEvent(&eventTimer{Time: 1})
	g.fireEvent(&eventTimer{Time: 2})

	snap := g.eventQueueSnapshot()
	if snap.DroppedTotal != 0 {
		t.Fatalf("expected droppedTotal=0, got %d", snap.DroppedTotal)
	}
	if snap.EnqueuedTotal != 2 {
		t.Fatalf("expected enqueuedTotal=2, got %d", snap.EnqueuedTotal)
	}
	if snap.QueueLen != 1 {
		t.Fatalf("expected queue len=1, got %d", snap.QueueLen)
	}

	ev := (<-g.events).(*eventTimer)
	if ev.Time != 2 {
		t.Fatalf("expected newest event to remain, got %.0f", ev.Time)
	}
}

func TestEventQueueBlock(t *testing.T) {
	resetStateForTest()
	t.Cleanup(resetStateForTest)

	g := newEventQueueTestGame(1)
	g.setEventQueuePolicy(eventQueueBlock)

	g.fireEvent(&eventTimer{Time: 1})

	done := make(chan struct{})
	go func() {
		g.fireEvent(&eventTimer{Time: 2})
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("expected fireEvent to block while queue is full")
	case <-time.After(50 * time.Millisecond):
	}

	_ = <-g.events

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("fireEvent did not unblock after queue had space")
	}

	ev := (<-g.events).(*eventTimer)
	if ev.Time != 2 {
		t.Fatalf("expected blocked event to be enqueued after unblock, got %.0f", ev.Time)
	}

	snap := g.eventQueueSnapshot()
	if snap.DroppedTotal != 0 {
		t.Fatalf("expected droppedTotal=0, got %d", snap.DroppedTotal)
	}
	if snap.EnqueuedTotal != 2 {
		t.Fatalf("expected enqueuedTotal=2, got %d", snap.EnqueuedTotal)
	}
}
