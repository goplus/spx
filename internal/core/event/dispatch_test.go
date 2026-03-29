package event

import "testing"

func TestDispatchAsync(t *testing.T) {
	var (
		starts []bool
		owners []string
		calls  []string
	)
	DispatchAsync(
		[]Sink{
			{
				Owner:   "a",
				Handler: "A",
			},
			{
				Owner:   "b",
				Handler: "B",
				Cond: func(data any) bool {
					return data == "ok"
				},
			},
			{
				Owner:   "c",
				Handler: "C",
				Cond: func(data any) bool {
					return false
				},
			},
		},
		true,
		"ok",
		DispatchHooks{
			Spawn: func(start bool, owner any, call func()) {
				starts = append(starts, start)
				owners = append(owners, owner.(string))
				call()
			},
		},
		func(sink *Sink) {
			calls = append(calls, sink.Handler.(string))
		},
	)

	if len(starts) != 2 || !starts[0] || !starts[1] {
		t.Fatalf("starts = %+v, want [true true]", starts)
	}
	if want := []string{"a", "b"}; len(owners) != len(want) || owners[0] != want[0] || owners[1] != want[1] {
		t.Fatalf("owners = %+v, want %+v", owners, want)
	}
	if want := []string{"A", "B"}; len(calls) != len(want) || calls[0] != want[0] || calls[1] != want[1] {
		t.Fatalf("calls = %+v, want %+v", calls, want)
	}
}

func TestDispatchSync(t *testing.T) {
	waited := false
	var calls []string
	DispatchSync(
		[]Sink{
			{Owner: "a", Handler: "A"},
			{
				Owner:   "b",
				Handler: "B",
				Cond: func(data any) bool {
					return data == "ok"
				},
			},
		},
		"ok",
		DispatchHooks{
			Spawn: func(start bool, owner any, call func()) {
				if start {
					t.Fatal("DispatchSync should not start threads in start mode")
				}
				call()
			},
			Wait: func(wait func()) {
				waited = true
				wait()
			},
		},
		func(sink *Sink) {
			calls = append(calls, sink.Handler.(string))
		},
	)

	if !waited {
		t.Fatal("expected wait hook to be called")
	}
	if want := []string{"A", "B"}; len(calls) != len(want) || calls[0] != want[0] || calls[1] != want[1] {
		t.Fatalf("calls = %+v, want %+v", calls, want)
	}
}

func TestDispatchFallbackHooks(t *testing.T) {
	var calls []string
	Dispatch(
		[]Sink{{Owner: "a", Handler: "A"}},
		false,
		nil,
		DispatchHooks{},
		func(sink *Sink) {
			calls = append(calls, sink.Handler.(string))
		},
	)
	if len(calls) != 1 || calls[0] != "A" {
		t.Fatalf("calls = %+v, want [A]", calls)
	}
}

func TestManagerDispatchBucketAsync(t *testing.T) {
	var (
		starts []bool
		owners []string
		calls  []string
	)

	var mgr Manager
	mgr.Add(BucketClick, Sink{Owner: "a", Handler: "A"})
	mgr.Add(BucketClick, Sink{
		Owner:   "b",
		Handler: "B",
		Cond: func(data any) bool {
			return data == "ok"
		},
	})

	mgr.DispatchBucketAsync(BucketClick, true, "ok", DispatchHooks{
		Spawn: func(start bool, owner any, call func()) {
			starts = append(starts, start)
			owners = append(owners, owner.(string))
			call()
		},
	}, func(sink *Sink) {
		calls = append(calls, sink.Handler.(string))
	})

	if len(starts) != 2 || !starts[0] || !starts[1] {
		t.Fatalf("starts = %+v, want [true true]", starts)
	}
	if want := []string{"a", "b"}; len(owners) != len(want) || owners[0] != want[0] || owners[1] != want[1] {
		t.Fatalf("owners = %+v, want %+v", owners, want)
	}
	if want := []string{"A", "B"}; len(calls) != len(want) || calls[0] != want[0] || calls[1] != want[1] {
		t.Fatalf("calls = %+v, want %+v", calls, want)
	}
}

func TestManagerDispatchBucketSync(t *testing.T) {
	waited := false
	var calls []string

	var mgr Manager
	mgr.Add(BucketTimer, Sink{Owner: "timer", Handler: "A"})

	mgr.DispatchBucketSync(BucketTimer, 1.0, DispatchHooks{
		Spawn: func(start bool, owner any, call func()) {
			if start {
				t.Fatal("DispatchBucketSync should not mark start=true")
			}
			call()
		},
		Wait: func(wait func()) {
			waited = true
			wait()
		},
	}, func(sink *Sink) {
		calls = append(calls, sink.Handler.(string))
	})

	if !waited {
		t.Fatal("expected wait hook to be called")
	}
	if len(calls) != 1 || calls[0] != "A" {
		t.Fatalf("calls = %+v, want [A]", calls)
	}
}

func TestManagerDispatchStartOnce(t *testing.T) {
	var calls []string

	var mgr Manager
	if !mgr.TryAddStart(Sink{Owner: "game", Handler: "start"}) {
		t.Fatal("expected start sink to be added")
	}

	hooks := DispatchHooks{
		Spawn: func(start bool, owner any, call func()) {
			if start {
				t.Fatal("DispatchStartOnce should not require start=true in this test")
			}
			call()
		},
	}

	mgr.DispatchStartOnce(false, nil, hooks, func(sink *Sink) {
		calls = append(calls, sink.Handler.(string))
	})
	mgr.DispatchStartOnce(false, nil, hooks, func(sink *Sink) {
		calls = append(calls, sink.Handler.(string))
	})

	if len(calls) != 1 || calls[0] != "start" {
		t.Fatalf("calls = %+v, want [start]", calls)
	}
}
