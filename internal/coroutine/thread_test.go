/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package coroutine

import (
	"testing"
	"time"
)

func TestThreadNameUsesNameMethodWithoutReflectionCall(t *testing.T) {
	if got := threadName(&namedOwner{}); got != "named-thread" {
		t.Fatalf("expected name from interface method, got %q", got)
	}
}

func TestThreadNameFallsBackForInvalidNameSignature(t *testing.T) {
	if got := threadName(&invalidNamedOwner{}); got != "*invalidNamedOwner" {
		t.Fatalf("expected fallback type name, got %q", got)
	}
}

func TestThreadString(t *testing.T) {
	thread := New(nil).newThread(&namedOwner{})
	if got := thread.String(); got != "id=1 name=named-thread " {
		t.Fatalf("thread diagnostic = %q, want ID and resolved owner name", got)
	}
}

func TestSetWarp(t *testing.T) {
	for _, tt := range []struct {
		name     string
		previous bool
		enabled  bool
	}{
		{"stay disabled", false, false},
		{"enable", false, true},
		{"disable", true, false},
		{"stay enabled", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			thread := New(nil).newThread("warp")
			thread.SetWarp(tt.previous)
			const startedAt = int64(123)
			thread.warpStart.Store(startedAt)

			if previous := thread.SetWarp(tt.enabled); previous != tt.previous {
				t.Fatalf("SetWarp returned %v, want previous mode %v", previous, tt.previous)
			}
			if got := thread.Warp(); got != tt.enabled {
				t.Fatalf("Warp = %v, want %v", got, tt.enabled)
			}
			wantStart := startedAt
			if tt.enabled != tt.previous {
				wantStart = 0
			}
			if got := thread.warpStart.Load(); got != wantStart {
				t.Fatalf("warp budget started at %d, want %d", got, wantStart)
			}
		})
	}
}

func TestShouldYield(t *testing.T) {
	t.Run("without warp", func(t *testing.T) {
		thread := New(nil).newThread("loop")
		if !thread.ShouldYield(time.Hour) {
			t.Fatal("ordinary loop did not yield")
		}
		if got := thread.warpStart.Load(); got != 0 {
			t.Fatalf("ordinary loop started a warp budget at %d", got)
		}
	})

	t.Run("start budget", func(t *testing.T) {
		thread := New(nil).newThread("warp")
		thread.SetWarp(true)
		if thread.ShouldYield(time.Hour) {
			t.Fatal("first warp loop edge yielded before starting its budget")
		}
		if thread.warpStart.Load() == 0 {
			t.Fatal("first warp loop edge did not start its budget")
		}
	})

	t.Run("within budget", func(t *testing.T) {
		thread := New(nil).newThread("warp")
		thread.SetWarp(true)
		startedAt := time.Now().Add(-time.Minute).UnixNano()
		thread.warpStart.Store(startedAt)
		if thread.ShouldYield(time.Hour) {
			t.Fatal("warp loop yielded before exhausting its budget")
		}
		if got := thread.warpStart.Load(); got != startedAt {
			t.Fatalf("warp loop changed its budget start to %d, want %d", got, startedAt)
		}
	})

	t.Run("exhausted budget", func(t *testing.T) {
		thread := New(nil).newThread("warp")
		thread.SetWarp(true)
		thread.warpStart.Store(time.Now().Add(-time.Hour).UnixNano())
		if !thread.ShouldYield(time.Minute) {
			t.Fatal("warp loop did not yield after exhausting its budget")
		}
		if got := thread.warpStart.Load(); got != 0 {
			t.Fatalf("exhausted warp budget was not reset: %d", got)
		}
		if thread.ShouldYield(time.Minute) || thread.warpStart.Load() == 0 {
			t.Fatal("warp loop did not start a fresh budget after yielding")
		}
	})
}

func TestBeginMainRestoresEnclosingStart(t *testing.T) {
	thread := New(nil).newThread("main")
	outerStartedAt := time.Unix(10, 0)
	innerStartedAt := time.Unix(20, 0)

	endOuter := thread.BeginMain(outerStartedAt)
	endInner := thread.BeginMain(innerStartedAt)
	if got := thread.MainStartedAt(); !got.Equal(innerStartedAt) {
		t.Fatalf("inner Main started at %v, want %v", got, innerStartedAt)
	}
	thread.DisableMainTimeout()
	if got := thread.MainStartedAt(); !got.IsZero() {
		t.Fatalf("disabled Main timeout started at %v, want zero", got)
	}

	endInner()
	if got := thread.MainStartedAt(); !got.Equal(outerStartedAt) {
		t.Fatalf("restored outer Main started at %v, want %v", got, outerStartedAt)
	}
	endOuter()
	if got := thread.MainStartedAt(); !got.IsZero() {
		t.Fatalf("finished Main started at %v, want zero", got)
	}
}

func TestCancelStopsSuspendedThread(t *testing.T) {
	co := New(nil)
	thread := co.Create("suspended", func(me Thread) {
		co.WaitYield(me)
		t.Error("canceled script continued")
	})
	t.Cleanup(func() { co.RunAfterStop(drainTestContext(t), nil) })
	co.JoinFirstSlice(thread)
	thread.Cancel()
	waitForThreadSignal(t, thread.done, "Cancel did not wake the suspended script")
	if !thread.Stopped() || thread.Context().Err() == nil {
		t.Fatal("Cancel did not publish a stop request")
	}
}

type namedOwner struct{}

func (*namedOwner) Name() string {
	return "named-thread"
}

type invalidNamedOwner struct{}

func (*invalidNamedOwner) Name(_ string) string {
	return "invalid"
}

func waitForThreadSignal(t *testing.T, signal <-chan struct{}, failure string) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-signal:
	case <-timer.C:
		t.Fatal(failure)
	}
}
