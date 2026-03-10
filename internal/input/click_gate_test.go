package input

import (
	"testing"
	"time"
)

func TestClickGateAllowHonorsIntervalPerObject(t *testing.T) {
	now := time.Unix(0, 0)
	gate := ClickGate{
		now: func() time.Time { return now },
	}
	gate.Init(50 * time.Millisecond)

	if !gate.Allow(1) {
		t.Fatal("expected first click to be allowed")
	}
	if !gate.Allow(2) {
		t.Fatal("expected independent object click to be allowed")
	}

	now = now.Add(25 * time.Millisecond)
	if gate.Allow(1) {
		t.Fatal("expected click inside interval to be blocked")
	}

	now = now.Add(25 * time.Millisecond)
	if !gate.Allow(1) {
		t.Fatal("expected click at interval boundary to be allowed")
	}
}

func TestClickGatePrunesExpiredEntriesAndRemove(t *testing.T) {
	now := time.Unix(0, 0)
	gate := ClickGate{
		now: func() time.Time { return now },
	}
	gate.Init(50 * time.Millisecond)

	if !gate.Allow(1) {
		t.Fatal("expected first click to be allowed")
	}
	now = now.Add(60 * time.Millisecond)
	if !gate.Allow(2) {
		t.Fatal("expected second click to be allowed")
	}
	if len(gate.lastClickMs) != 1 {
		t.Fatalf("lastClickMs len = %d, want 1 after pruning", len(gate.lastClickMs))
	}

	gate.Remove(2)
	if len(gate.lastClickMs) != 0 {
		t.Fatalf("lastClickMs len = %d, want 0 after remove", len(gate.lastClickMs))
	}
}
