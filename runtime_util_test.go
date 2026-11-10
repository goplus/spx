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

package spx

import (
	"math"
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/engine/coroutine"
	itime "github.com/goplus/spx/v4/internal/time"
)

func TestRand0SwapsReversedBounds(t *testing.T) {
	const (
		from = 93
		to   = -220
	)

	sawNonUpperBound := false
	for i := 0; i < 100; i++ {
		got := Rand__0(from, to)
		if got < to || got > from {
			t.Fatalf("Rand__0(%d, %d) = %v, want value in [%d, %d]", from, to, got, to, from)
		}
		if got != float64(from) {
			sawNonUpperBound = true
		}
	}

	if !sawNonUpperBound {
		t.Fatalf("Rand__0(%d, %d) only returned the upper bound; reversed bounds should produce values across the range", from, to)
	}
}

func TestRand1AlwaysSamplesContinuousRange(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	for _, bounds := range [][2]float64{{1, 12}, {12, 1}, {-3, -1}, {-1, 1}, {0, 1}, {1.5, 12}, {1, 12.5}, {12.5, 1}, {-3, -1.5}} {
		SetRandomSeed(1042497735)
		low, high := math.Min(bounds[0], bounds[1]), math.Max(bounds[0], bounds[1])
		seenFraction := false
		for range 64 {
			got := Rand__1(bounds[0], bounds[1])
			if got < low || got >= high {
				t.Fatalf("Rand__1(%v, %v) = %v, outside continuous range", bounds[0], bounds[1], got)
			}
			seenFraction = seenFraction || math.Trunc(got) != got
		}
		if !seenFraction {
			t.Fatalf("Rand__1(%v, %v) only returned integers", bounds[0], bounds[1])
		}
	}
}

func TestRand1EqualBoundsStillConsumeRandomSource(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	SetRandomSeed(17)
	_ = Rand__1(0, 1)
	wantNext := Rand__1(0, 1)
	for _, bound := range []float64{7, 0.5, 1e20} {
		SetRandomSeed(17)
		if got := Rand__1(bound, bound); got != bound {
			t.Fatalf("Rand__1(%v, %v) = %v, want %v", bound, bound, got, bound)
		}
		if got := Rand__1(0, 1); got != wantNext {
			t.Fatalf("Rand__1(%v, %v) did not consume exactly one draw: next = %v, want %v", bound, bound, got, wantNext)
		}
	}
}

// Keep the legacy entry points wired to the core rules. Boundary matrices live
// with their implementation in internal/scratch.
func TestLegacyOperatorDelegation(t *testing.T) {
	if got := LimitPrecision(5e-10); got != 0 {
		t.Errorf("LimitPrecision = %v", got)
	}
	if got := Div(0, 0); got != 0 {
		t.Errorf("Div = %v", got)
	}
	if got := FloorMod(-7, 5); got != 3 {
		t.Errorf("FloorMod = %v", got)
	}
	if !Contains("Scratch", "RaTc") {
		t.Error("Contains lost case-insensitive matching")
	}
	value := NewValue("01")
	if got := Compare(value, 2); got != -1 {
		t.Errorf("Compare = %v", got)
	}
	if !Equal(value, 1) {
		t.Error("Equal lost wrapped numeric comparison")
	}
	if got := CharAt("A😀B", 0); got != "A" {
		t.Errorf("CharAt lost zero-based indexing: %q", got)
	}
}

func TestRandomSeedMakesScriptRandomDeterministic(t *testing.T) {
	SetRandomSeed(123)
	gotA := []float64{Rand__0(1, 10), Rand__1(0, 1), Rand__0(1, 10)}
	SetRandomSeed(123)
	gotB := []float64{Rand__0(1, 10), Rand__1(0, 1), Rand__0(1, 10)}
	defer ResetRandomSeed()

	for i := range gotA {
		if gotA[i] != gotB[i] {
			t.Fatalf("seeded random mismatch at %d: %v vs %v", i, gotA, gotB)
		}
	}
}

func TestNumericAndClockUtilities(t *testing.T) {
	for _, tt := range []struct {
		value float64
		want  int
	}{{0, 0}, {1.49, 1}, {1.5, 2}, {-1.49, -1}, {-1.5, -2}} {
		if got := Iround(tt.value); got != tt.want {
			t.Errorf("Iround(%v) = %d, want %d", tt.value, got, tt.want)
		}
	}
	SetRandomSeed(7)
	t.Cleanup(ResetRandomSeed)
	for range 20 {
		if got := Rand__1(5.5, -2); got < -2 || got >= 5.5 {
			t.Fatalf("reversed float bounds: %v", got)
		}
	}
	if got := Compare("z", "a"); got != 1 {
		t.Fatalf("lexical comparison = %d, want 1", got)
	}
	itime.Start()
	itime.Update(0.25, 30)
	if DeltaTime() != 0.25 || TimeSinceLevelLoad() != 0.25 {
		t.Fatalf("logical clock = %v/%v", DeltaTime(), TimeSinceLevelLoad())
	}
}

func TestDeterministicRandomIsolatedPerCoroutine(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	currentScheduler := setupRuntimeScheduler(t)

	runScenario := func(extraDraws int) []float64 {
		t.Helper()
		setDeterministicRandomSeed(123)

		doneA := make(chan struct{}, 1)
		currentScheduler.Create("extra-draws", func(me coroutine.Thread) {
			for range extraDraws {
				_ = Rand__1(0, 1)
			}
			doneA <- struct{}{}
		})
		select {
		case <-doneA:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for extra-draw coroutine")
		}

		doneB := make(chan []float64, 1)
		currentScheduler.Create("captured-sequence", func(me coroutine.Thread) {
			doneB <- []float64{Rand__1(0, 1), Rand__1(0, 1)}
		})
		select {
		case values := <-doneB:
			return values
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for captured-sequence coroutine")
			return nil
		}
	}

	gotA := runScenario(1)
	gotB := runScenario(5)
	if len(gotA) != len(gotB) {
		t.Fatalf("sequence length mismatch: %v vs %v", gotA, gotB)
	}
	for i := range gotA {
		if gotA[i] != gotB[i] {
			t.Fatalf("coroutine-local deterministic random mismatch at %d: %v vs %v", i, gotA, gotB)
		}
	}
}

func TestDeterministicRandomIgnoresWaitToDoGoroutineDraws(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	currentScheduler := setupRuntimeScheduler(t)

	runScenario := func(drawOutsideCoroutine bool) []float64 {
		t.Helper()
		setDeterministicRandomSeed(123)

		done := make(chan []float64, 1)
		currentScheduler.Create("wait-to-do-random", func(me coroutine.Thread) {
			if got := currentCoroutineID(); got != me.ID() {
				t.Errorf("managed random stream ID = %d, want %d", got, me.ID())
			}
			currentScheduler.WaitToDo(func() {
				if got := currentCoroutineID(); got != 0 {
					t.Errorf("native worker random stream ID = %d, want 0", got)
				}
				if drawOutsideCoroutine {
					_ = Rand__1(0, 1)
				}
			})
			done <- []float64{Rand__1(0, 1), Rand__1(0, 1)}
		})

		select {
		case values := <-done:
			return values
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for wait-to-do-random coroutine")
			return nil
		}
	}

	gotA := runScenario(false)
	gotB := runScenario(true)
	for i := range gotA {
		if gotA[i] != gotB[i] {
			t.Fatalf("wait-to-do goroutine shifted coroutine random stream at %d: %v vs %v", i, gotA, gotB)
		}
	}
}

func TestRandomWithoutSchedulerAndInt31Sequence(t *testing.T) {
	previous := scheduler
	scheduler = nil
	t.Cleanup(func() { scheduler = previous; ResetRandomSeed() })
	if currentCoroutineID() != 0 || lastAllocatedCoroutineID() != 0 {
		t.Fatal("scheduler-free random has a coroutine identity")
	}
	SetRandomSeed(91)
	first := randomInt31n(1000)
	SetRandomSeed(91)
	if second := randomInt31n(1000); second != first || second < 0 || second >= 1000 {
		t.Fatalf("int31 sequence changed: %d/%d", first, second)
	}
}
