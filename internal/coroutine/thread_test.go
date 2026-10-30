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
	"fmt"
	"testing"
	"testing/synctest"
	"time"
)

type namedOwner struct{}

type invalidNamedOwner struct{}

func TestThreadIdentity(t *testing.T) {
	co := New(nil)
	var wantID int64
	for _, test := range []struct {
		name  string
		owner any
		want  string
	}{
		{"nil owner", nil, ""},
		{"string owner", "script", "script"},
		{"named owner", &namedOwner{}, "named-thread"},
		{"invalid Name signature", &invalidNamedOwner{}, "*invalidNamedOwner"},
		{"value owner", struct{}{}, ""},
		{"anonymous pointer", &struct{}{}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			wantID++
			thread := co.newThread(test.owner)
			t.Cleanup(thread.Cancel)
			if got := thread.ID(); got != wantID {
				t.Errorf("ID = %d, want %d", got, wantID)
			}
			if got := thread.Name(); got != test.want {
				t.Errorf("Name = %q, want %q", got, test.want)
			}
			want := fmt.Sprintf("id=%d name=%s ", wantID, test.want)
			if got := thread.String(); got != want {
				t.Errorf("String = %q, want %q", got, want)
			}
		})
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
		t.Cleanup(thread.Cancel)
		if !thread.ShouldYield(time.Hour) {
			t.Fatal("ordinary loop did not yield")
		}
		if got := thread.warpStart.Load(); got != 0 {
			t.Fatalf("ordinary loop started a warp budget at %d", got)
		}
	})
	for _, budget := range []time.Duration{-time.Nanosecond, 0, time.Millisecond, 500 * time.Millisecond} {
		t.Run(budget.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				thread := New(nil).newThread("warp")
				t.Cleanup(thread.Cancel)
				thread.SetWarp(true)
				if thread.ShouldYield(budget) {
					t.Fatal("first edge must start the budget")
				}
				startedAt := thread.warpStart.Load()
				if startedAt == 0 {
					t.Fatal("first edge did not start the budget")
				}
				if budget >= 0 {
					if thread.ShouldYield(budget) {
						t.Fatal("warp loop yielded before its budget")
					}
					time.Sleep(budget)
					if thread.ShouldYield(budget) {
						t.Fatal("edge exactly at budget must not yield")
					}
					if thread.warpStart.Load() != startedAt {
						t.Fatal("loop edge restarted an unexpired budget")
					}
					time.Sleep(time.Nanosecond)
				}
				if !thread.ShouldYield(budget) {
					t.Fatal("edge after budget must yield")
				}
				if thread.warpStart.Load() != 0 {
					t.Fatal("exhausted warp budget was not reset")
				}
				if thread.ShouldYield(budget) || thread.warpStart.Load() == 0 {
					t.Fatal("yield must reset the next execution window")
				}
			})
		})
	}
}

func TestIsSchedTimeoutUsesFrameTimestampAndThreshold(t *testing.T) {
	thread := New(nil).newThread("scheduler")
	if thread.IsSchedTimeout(float64(time.Hour / time.Millisecond)) {
		t.Fatal("new scheduler frame was already timed out")
	}
	if !thread.IsSchedTimeout(-1) {
		t.Fatal("negative scheduler timeout did not expire immediately")
	}
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
	t.Cleanup(func() { co.RunAfterStop(nil) })
	co.JoinFirstSlice(thread)
	thread.Cancel()
	waitForThreadSignal(t, thread.done, "Cancel did not wake the suspended script")
	if !thread.Stopped() || thread.Context().Err() == nil {
		t.Fatal("Cancel did not publish a stop request")
	}
}

func TestIsInCoroutineTracksCaller(t *testing.T) {
	co := New(nil)
	if co.IsInCoroutine() {
		t.Fatal("external caller was reported as a coroutine")
	}

	managed := make(chan bool, 1)
	co.Create("managed", func(Thread) {
		managed <- co.IsInCoroutine()
	})
	select {
	case got := <-managed:
		if !got {
			t.Fatal("managed caller was not reported as a coroutine")
		}
	case <-time.After(time.Second):
		t.Fatal("managed caller did not run")
	}
	co.RunAfterStop(nil)
}

func TestExternalCallsPreserveActiveThread(t *testing.T) {
	co := New(nil)
	started, release := make(chan struct{}), make(chan struct{})
	thread := co.Create("active", func(Thread) {
		close(started)
		<-release
	})
	t.Cleanup(func() {
		close(release)
		co.RunAfterStop(nil)
	})
	waitForThreadSignal(t, started, "coroutine did not start")
	if co.Current() != thread {
		t.Fatalf("Current() = %v, want running coroutine %v", co.Current(), thread)
	}

	called := false
	for _, test := range []struct {
		name      string
		call      func(Thread)
		wantPanic any
		wantCall  bool
	}{
		{"worker", func(Thread) { co.WaitToDo(func() { called = true }) }, nil, true},
		{"yield", co.yield, ErrNotCurrent, false},
		{"wait yield", co.WaitYield, ErrNotCurrent, false},
		{"frame", co.WaitNextFrameFor, ErrNotCurrent, false},
		{"loop", co.YieldLoopFor, ErrNotCurrent, false},
		{"round", co.YieldToNextRoundFor, ErrNotCurrent, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			called = false
			if recovered := catchPanic(func() { test.call(thread) }); recovered != test.wantPanic {
				t.Fatalf("panic = %v, want %v", recovered, test.wantPanic)
			}
			if called != test.wantCall {
				t.Fatalf("callback called = %v, want %v", called, test.wantCall)
			}
			if co.Current() != thread || co.callerThread() != nil {
				t.Fatal("external call changed script ownership or caller identity")
			}
			co.schedulerMu.Lock()
			defer co.schedulerMu.Unlock()
			_, pending := co.pendingExecution[thread]
			if !pending || co.currentJobs.len() != 0 {
				t.Fatal("external call changed the running script or queued work")
			}
		})
	}
}

func (*namedOwner) Name() string {
	return "named-thread"
}

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

func BenchmarkShouldYield(b *testing.B) {
	for _, warp := range []bool{false, true} {
		b.Run(fmt.Sprintf("warp=%t", warp), func(b *testing.B) {
			thread := New(nil).newThread("loop")
			thread.SetWarp(warp)
			b.Cleanup(thread.Cancel)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				thread.ShouldYield(time.Hour)
			}
		})
	}
}
