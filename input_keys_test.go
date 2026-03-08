package spx

import "testing"

func TestKeyFromStringHandlesSentinels(t *testing.T) {
	if got := KeyFromString("Any"); got != KeyAny {
		t.Fatalf("unexpected Any mapping: got %v want %v", got, KeyAny)
	}
	if got := KeyFromString("NotAKey"); got != KeyMax {
		t.Fatalf("unexpected unknown mapping: got %v want %v", got, KeyMax)
	}
}
