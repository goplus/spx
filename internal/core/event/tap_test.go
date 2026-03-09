package event

import "testing"

func TestIfHelpers(t *testing.T) {
	var calls []string
	enabled := false

	If0(func() bool { return enabled }, func() {
		calls = append(calls, "if0")
	})()
	If1(func() bool { return enabled }, func(v string) {
		calls = append(calls, "if1:"+v)
	})("x")
	If2(func() bool { return enabled }, func(a, b string) {
		calls = append(calls, "if2:"+a+":"+b)
	})("a", "b")

	if len(calls) != 0 {
		t.Fatalf("calls = %v, want none", calls)
	}

	enabled = true
	If0(func() bool { return enabled }, func() {
		calls = append(calls, "if0")
	})()
	If1(func() bool { return enabled }, func(v string) {
		calls = append(calls, "if1:"+v)
	})("x")
	If2(func() bool { return enabled }, func(a, b string) {
		calls = append(calls, "if2:"+a+":"+b)
	})("a", "b")

	want := []string{"if0", "if1:x", "if2:a:b"}
	if len(calls) != len(want) {
		t.Fatalf("calls len = %d, want %d (%v)", len(calls), len(want), calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q", i, calls[i], want[i])
		}
	}
}

func TestIgnoreHelpers(t *testing.T) {
	var calls []string

	Ignore1[string](func() {
		calls = append(calls, "ignore1")
	})("x")
	Ignore2[string, string](func() {
		calls = append(calls, "ignore2")
	})("a", "b")

	want := []string{"ignore1", "ignore2"}
	if len(calls) != len(want) {
		t.Fatalf("calls len = %d, want %d (%v)", len(calls), len(want), calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q", i, calls[i], want[i])
		}
	}
}

func TestTapHelpers(t *testing.T) {
	var calls []string

	Tap0(
		func() { calls = append(calls, "call0") },
		func() { calls = append(calls, "tap0") },
	)()

	Tap1(
		func(v string) { calls = append(calls, "call1:"+v) },
		func(v string) { calls = append(calls, "tap1:"+v) },
	)("x")

	Tap2(
		func(a, b string) { calls = append(calls, "call2:"+a+":"+b) },
		func(a, b string) { calls = append(calls, "tap2:"+a+":"+b) },
	)("a", "b")

	TapVoid1(
		func() { calls = append(calls, "callv1") },
		func(v string) { calls = append(calls, "tapv1:"+v) },
	)("y")

	TapVoid2(
		func() { calls = append(calls, "callv2") },
		func(a, b string) { calls = append(calls, "tapv2:"+a+":"+b) },
	)("m", "n")

	want := []string{
		"tap0", "call0",
		"tap1:x", "call1:x",
		"tap2:a:b", "call2:a:b",
		"tapv1:y", "callv1",
		"tapv2:m:n", "callv2",
	}
	if len(calls) != len(want) {
		t.Fatalf("calls len = %d, want %d (%v)", len(calls), len(want), calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q", i, calls[i], want[i])
		}
	}
}
