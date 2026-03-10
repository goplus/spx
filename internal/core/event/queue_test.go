package event

import (
	"sync"
	"testing"
	"time"
)

func TestParsePolicy(t *testing.T) {
	if got := ParsePolicy("dropoldest"); got != QueueDropOldest {
		t.Fatalf("ParsePolicy(dropoldest) = %v", got)
	}
	if got := ParsePolicy("Block"); got != QueueBlock {
		t.Fatalf("ParsePolicy(Block) = %v", got)
	}
	if got := ParsePolicy("unknown"); got != DefaultQueuePolicy {
		t.Fatalf("ParsePolicy(unknown) = %v", got)
	}
}

func TestEnqueueWithPolicyDropNewest(t *testing.T) {
	ch := make(chan int, 1)
	var stats QueueStats
	stats.Reset()

	if !EnqueueWithPolicy(ch, 1, QueueDropNewest, &stats, nil) {
		t.Fatal("expected first enqueue to succeed")
	}
	if EnqueueWithPolicy(ch, 2, QueueDropNewest, &stats, nil) {
		t.Fatal("expected second enqueue to be dropped")
	}
	if got := <-ch; got != 1 {
		t.Fatalf("queue retained %d, want 1", got)
	}
	if stats.DroppedTotal() != 1 {
		t.Fatalf("DroppedTotal = %d, want 1", stats.DroppedTotal())
	}
}

func TestEnqueueWithPolicyDropOldest(t *testing.T) {
	ch := make(chan int, 1)
	var mu sync.Mutex
	var stats QueueStats
	stats.Reset()

	if !EnqueueWithPolicy(ch, 1, QueueDropOldest, &stats, &mu) {
		t.Fatal("expected first enqueue to succeed")
	}
	if !EnqueueWithPolicy(ch, 2, QueueDropOldest, &stats, &mu) {
		t.Fatal("expected second enqueue to succeed")
	}
	if got := <-ch; got != 2 {
		t.Fatalf("queue retained %d, want 2", got)
	}
	if stats.DroppedTotal() != 1 {
		t.Fatalf("DroppedTotal = %d, want 1", stats.DroppedTotal())
	}
}

func TestEnqueueWithPolicyBlock(t *testing.T) {
	ch := make(chan int, 1)
	var stats QueueStats
	stats.Reset()

	if !EnqueueWithPolicy(ch, 1, QueueBlock, &stats, nil) {
		t.Fatal("expected first enqueue to succeed")
	}

	done := make(chan struct{})
	go func() {
		EnqueueWithPolicy(ch, 2, QueueBlock, &stats, nil)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("expected enqueue to block")
	case <-time.After(50 * time.Millisecond):
	}

	<-ch

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("blocked enqueue did not resume")
	}

	if got := <-ch; got != 2 {
		t.Fatalf("queue retained %d, want 2", got)
	}
}

func TestSnapshot(t *testing.T) {
	var stats QueueStats
	stats.Reset()
	stats.OnEnqueue(3)
	stats.OnDrop()

	snap := Snapshot(QueueDropNewest, &stats, 2, 8)
	if snap.Policy != "DropNewest" || snap.QueueLen != 2 || snap.QueueCap != 8 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	if snap.EnqueuedTotal != 1 || snap.DroppedTotal != 1 {
		t.Fatalf("unexpected totals: %+v", snap)
	}
	if snap.LastDropAt.IsZero() {
		t.Fatal("expected LastDropAt to be set")
	}
}
