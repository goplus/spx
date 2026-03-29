package event

import (
	"reflect"
	"testing"
)

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
	if want := []string{"a", "b"}; !reflect.DeepEqual(owners, want) {
		t.Fatalf("owners = %+v, want %+v", owners, want)
	}
	if want := []string{"A", "B"}; !reflect.DeepEqual(calls, want) {
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
	if want := []string{"A", "B"}; !reflect.DeepEqual(calls, want) {
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
	if want := []string{"a", "b"}; !reflect.DeepEqual(owners, want) {
		t.Fatalf("owners = %+v, want %+v", owners, want)
	}
	if want := []string{"A", "B"}; !reflect.DeepEqual(calls, want) {
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
	if want := []string{"A"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %+v, want %+v", calls, want)
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
				t.Fatal("DispatchStartOnce should always dispatch with start=false")
			}
			call()
		},
	}

	mgr.DispatchStartOnce(nil, hooks, func(sink *Sink) {
		calls = append(calls, sink.Handler.(string))
	})
	mgr.DispatchStartOnce(nil, hooks, func(sink *Sink) {
		calls = append(calls, sink.Handler.(string))
	})

	if want := []string{"start"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %+v, want %+v", calls, want)
	}
}

func TestManagerDispatchBucket(t *testing.T) {
	t.Run("async", func(t *testing.T) {
		waited := false
		var calls []string

		var mgr Manager
		mgr.Add(BucketIReceive, Sink{
			Owner:   "msg",
			Handler: "A",
			Cond: func(data any) bool {
				return data == "ok"
			},
		})

		mgr.DispatchBucket(BucketIReceive, false, "ok", DispatchHooks{
			Spawn: func(start bool, owner any, call func()) {
				if start {
					t.Fatal("DispatchBucket async path should not force start=true")
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

		if waited {
			t.Fatal("wait hook should not be called for async DispatchBucket")
		}
		if want := []string{"A"}; !reflect.DeepEqual(calls, want) {
			t.Fatalf("calls = %+v, want %+v", calls, want)
		}
	})

	t.Run("sync", func(t *testing.T) {
		waited := false
		var calls []string

		var mgr Manager
		mgr.Add(BucketBackdropChanged, Sink{Owner: "backdrop", Handler: "B"})

		mgr.DispatchBucket(BucketBackdropChanged, true, "stage", DispatchHooks{
			Spawn: func(start bool, owner any, call func()) {
				if start {
					t.Fatal("DispatchBucket sync path should not force start=true")
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
			t.Fatal("expected wait hook to be called for sync DispatchBucket")
		}
		if want := []string{"B"}; !reflect.DeepEqual(calls, want) {
			t.Fatalf("calls = %+v, want %+v", calls, want)
		}
	})
}
