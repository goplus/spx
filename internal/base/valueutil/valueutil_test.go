package valueutil

import "testing"

func TestClampFloat64(t *testing.T) {
	if got := ClampFloat64(5, 0, 3); got != 3 {
		t.Fatalf("ClampFloat64 upper bound = %v, want 3", got)
	}
	if got := ClampFloat64(-1, 0, 3); got != 0 {
		t.Fatalf("ClampFloat64 lower bound = %v, want 0", got)
	}
	if got := ClampFloat64(2, 0, 3); got != 2 {
		t.Fatalf("ClampFloat64 in-range = %v, want 2", got)
	}
}

func TestOrDefault(t *testing.T) {
	if got := OrDefault[int](nil, 42); got != 42 {
		t.Fatalf("OrDefault(nil) = %v, want 42", got)
	}
	val := 7
	if got := OrDefault(&val, 42); got != 7 {
		t.Fatalf("OrDefault(value) = %v, want 7", got)
	}
}

func TestSetDefaultIfZero(t *testing.T) {
	v := 0
	SetDefaultIfZero(&v, 10)
	if v != 10 {
		t.Fatalf("SetDefaultIfZero zero = %v, want 10", v)
	}
	v = 5
	SetDefaultIfZero(&v, 10)
	if v != 5 {
		t.Fatalf("SetDefaultIfZero non-zero = %v, want 5", v)
	}
}
