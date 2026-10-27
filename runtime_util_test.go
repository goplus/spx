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

	"github.com/goplus/spx/v4/internal/coroutine"
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
		if got != from {
			sawNonUpperBound = true
		}
	}

	if !sawNonUpperBound {
		t.Fatalf("Rand__0(%d, %d) only returned the upper bound; reversed bounds should produce values across the range", from, to)
	}
}

func TestDiv(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	tests := []struct {
		name string
		a, b float64
		want float64
	}{
		{name: "positive operands", a: 7, b: 2, want: 3.5},
		{name: "negative dividend", a: -7, b: 2, want: -3.5},
		{name: "negative divisor", a: 7, b: -2, want: -3.5},
		{name: "negative operands", a: -7, b: -2, want: 3.5},
		{name: "fractional operands", a: 0.5, b: 0.25, want: 2},
		{name: "zero dividend", a: 0, b: 2, want: 0},
		{name: "negative zero dividend", a: negativeZero, b: 2, want: negativeZero},
		{name: "zero by zero", a: 0, b: 0, want: 0},
		{name: "zero by negative zero", a: 0, b: negativeZero, want: 0},
		{name: "positive by zero", a: 1, b: 0, want: math.Inf(1)},
		{name: "negative by zero", a: -1, b: 0, want: math.Inf(-1)},
		{name: "positive by negative zero", a: 1, b: negativeZero, want: math.Inf(-1)},
		{name: "negative by negative zero", a: -1, b: negativeZero, want: math.Inf(1)},
		{name: "NaN dividend", a: math.NaN(), b: 1, want: 0},
		{name: "NaN divisor", a: 1, b: math.NaN(), want: 0},
		{name: "infinite dividend", a: math.Inf(1), b: -2, want: math.Inf(-1)},
		{name: "infinite divisor", a: 1, b: math.Inf(1), want: 0},
		{name: "negative infinite divisor", a: 1, b: math.Inf(-1), want: negativeZero},
		{name: "infinity by infinity", a: math.Inf(1), b: math.Inf(1), want: 0},
		{name: "opposite infinities", a: math.Inf(1), b: math.Inf(-1), want: 0},
		{name: "overflow", a: math.MaxFloat64, b: 0.5, want: math.Inf(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Div(tt.a, tt.b); got != tt.want || math.Signbit(got) != math.Signbit(tt.want) {
				t.Fatalf("Div(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestFloorMod(t *testing.T) {
	tests := []struct {
		name              string
		dividend, divisor float64
		want              float64
	}{
		{name: "positive operands", dividend: 7, divisor: 5, want: 2},
		{name: "negative dividend", dividend: -7, divisor: 5, want: 3},
		{name: "negative divisor", dividend: 7, divisor: -5, want: -3},
		{name: "negative operands", dividend: -7, divisor: -5, want: -2},
		{name: "exact division", dividend: -10, divisor: 5, want: 0},
		{name: "fractional operands", dividend: -2.5, divisor: 2, want: 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FloorMod(tt.dividend, tt.divisor); got != tt.want {
				t.Fatalf("FloorMod(%v, %v) = %v, want %v", tt.dividend, tt.divisor, got, tt.want)
			}
		})
	}

	if got := FloorMod(1, 0); !math.IsNaN(got) {
		t.Fatalf("FloorMod(1, 0) = %v, want NaN", got)
	}
}

func TestContainsUsesCaseInsensitiveSemantics(t *testing.T) {
	tests := []struct {
		name      string
		s         string
		substr    string
		contained bool
	}{
		{name: "exact match", s: "Hello world", substr: "world", contained: true},
		{name: "different case", s: "Hello World", substr: "hello world", contained: true},
		{name: "mixed case substring", s: "Scratch", substr: "RaTc", contained: true},
		{name: "missing substring", s: "Scratch", substr: "sprite", contained: false},
		{name: "empty substring", s: "Scratch", substr: "", contained: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.s, tt.substr); got != tt.contained {
				t.Fatalf("Contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.contained)
			}
		})
	}
}

func TestRandomSeedMakesScriptRandomDeterministic(t *testing.T) {
	SetRandomSeed(123)
	gotA := []any{Rand__0(1, 10), Rand__1(0, 1), Rand__0(1, 10)}
	SetRandomSeed(123)
	gotB := []any{Rand__0(1, 10), Rand__1(0, 1), Rand__0(1, 10)}
	defer ResetRandomSeed()

	for i := range gotA {
		if gotA[i] != gotB[i] {
			t.Fatalf("seeded random mismatch at %d: %v vs %v", i, gotA, gotB)
		}
	}
}

func TestCharAt(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		idx      int
		expected string
	}{
		{
			name:     "first character",
			s:        "hello",
			idx:      0,
			expected: "h",
		},
		{
			name:     "last character",
			s:        "hello",
			idx:      4,
			expected: "o",
		},
		{
			name:     "middle character",
			s:        "hello",
			idx:      2,
			expected: "l",
		},
		{
			name:     "index equals length returns empty string",
			s:        "hello",
			idx:      5,
			expected: "",
		},
		{
			name:     "negative index returns empty string",
			s:        "hello",
			idx:      -1,
			expected: "",
		},
		{
			name:     "index exceeds length returns empty string",
			s:        "hello",
			idx:      6,
			expected: "",
		},
		{
			name:     "empty string returns empty string",
			s:        "",
			idx:      0,
			expected: "",
		},
		{
			name:     "unicode multibyte character",
			s:        "你好世界",
			idx:      0,
			expected: "你",
		},
		{
			name:     "unicode last character",
			s:        "你好世界",
			idx:      3,
			expected: "界",
		},
		{
			name:     "unicode index out of range",
			s:        "你好世界",
			idx:      4,
			expected: "",
		},
		{
			name:     "single character string",
			s:        "a",
			idx:      0,
			expected: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CharAt(tt.s, tt.idx)
			if got != tt.expected {
				t.Errorf("CharAt(%q, %d) = %q, want %q", tt.s, tt.idx, got, tt.expected)
			}
		})
	}
}

func TestCompareAndEqual(t *testing.T) {
	tests := []struct {
		name    string
		v1      any
		v2      any
		compare int
		equal   bool
	}{
		{name: "case insensitive strings", v1: "p", v2: "P", compare: 0, equal: true},
		{name: "numeric strings", v1: "01", v2: "1", compare: 0, equal: true},
		{name: "number and string", v1: 2, v2: "10", compare: -1, equal: false},
		{name: "non numeric strings less", v1: "apple", v2: "Banana", compare: -1, equal: false},
		{name: "whitespace uses string compare", v1: "", v2: "0", compare: -1, equal: false},
		{name: "bool numeric compare", v1: true, v2: 1, compare: 0, equal: true},
		{name: "wrapped numeric string compares numerically", v1: NewValue("01"), v2: 1, compare: 0, equal: true},
		{name: "wrapped numeric string on right compares numerically", v1: 1, v2: NewValue("01"), compare: 0, equal: true},
		{name: "wrapped bool compares numerically", v1: NewValue(true), v2: 1, compare: 0, equal: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCompare := Compare(tt.v1, tt.v2)
			if gotCompare != tt.compare {
				t.Fatalf("Compare(%v, %v) = %d, want %d", tt.v1, tt.v2, gotCompare, tt.compare)
			}
			gotEqual := Equal(tt.v1, tt.v2)
			if gotEqual != tt.equal {
				t.Fatalf("Equal(%v, %v) = %v, want %v", tt.v1, tt.v2, gotEqual, tt.equal)
			}
		})
	}
}

func TestOutOfRangeListItemDoesNotEqualZero(t *testing.T) {
	list := NewList(1)
	item := list.At(17)
	if got := item.String(); got != "" {
		t.Fatalf("out-of-range item string = %q, want empty", got)
	}
	if Equal(item, 0) {
		t.Fatal("out-of-range item unexpectedly equals zero")
	}
}

func TestEqualUsesULPToleranceOnlyForNumericEquality(t *testing.T) {
	twoULPsAbove116 := 116.0
	for range compareMaxULPs {
		twoULPsAbove116 = math.Nextafter(twoULPsAbove116, math.Inf(1))
	}
	threeULPsAbove116 := math.Nextafter(twoULPsAbove116, math.Inf(1))

	tests := []struct {
		name  string
		v1    any
		v2    any
		equal bool
	}{
		{name: "rounding error", v1: 116.00000000000001, v2: 116.0, equal: true},
		{name: "numeric string rounding error", v1: "116.00000000000001", v2: 116.0, equal: true},
		{name: "two ULPs", v1: twoULPsAbove116, v2: 116.0, equal: true},
		{name: "three ULPs", v1: threeULPsAbove116, v2: 116.0, equal: false},
		{name: "distinct close distances", v1: 372.15588144669624, v2: 372.1558811904396, equal: false},
		{name: "different integers", v1: 116.0, v2: 117.0, equal: false},
		{name: "meaningful fractional difference", v1: 116.0, v2: 116.5, equal: false},
		{name: "same positive infinity", v1: math.Inf(1), v2: math.Inf(1), equal: true},
		{name: "opposite infinities", v1: math.Inf(1), v2: math.Inf(-1), equal: false},
		{name: "finite and infinity", v1: 1.0, v2: math.Inf(1), equal: false},
		{name: "NaN", v1: math.NaN(), v2: math.NaN(), equal: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Equal(tt.v1, tt.v2); got != tt.equal {
				t.Fatalf("Equal(%v, %v) = %v, want %v", tt.v1, tt.v2, got, tt.equal)
			}
		})
	}

	accumulated := 1.0
	for range 8 {
		accumulated += 0.02
	}
	if got := accumulated * 100; !Equal(got, 116) {
		t.Fatalf("Equal(%v, 116) = false after repeated floating-point addition", got)
	}

	if got := Compare(116.00000000000001, 116.0); got != 1 {
		t.Fatalf("Compare should remain exact: got %d, want 1", got)
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
		if got := Rand__1(5, -2); got < -2 || got >= 5 {
			t.Fatalf("reversed float bounds: %v", got)
		}
	}
	if got := Compare("z", "a"); got != 1 {
		t.Fatalf("lexical comparison = %d, want 1", got)
	}
	itime.Start(nil)
	itime.Update(0.25, 30)
	if DeltaTime() != 0.25 || TimeSinceLevelLoad() != 0.25 {
		t.Fatalf("logical clock = %v/%v", DeltaTime(), TimeSinceLevelLoad())
	}
}

func TestDeterministicRandomIsolatedPerCoroutine(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	co := setupRuntimeScheduler(t)

	runScenario := func(extraDraws int) []float64 {
		t.Helper()
		setDeterministicRandomSeed(123)

		doneA := make(chan struct{}, 1)
		co.Create("extra-draws", func(me coroutine.Thread) {
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
		co.Create("captured-sequence", func(me coroutine.Thread) {
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
	co := setupRuntimeScheduler(t)

	runScenario := func(drawOutsideCoroutine bool) []float64 {
		t.Helper()
		setDeterministicRandomSeed(123)

		done := make(chan []float64, 1)
		co.Create("wait-to-do-random", func(me coroutine.Thread) {
			co.WaitToDo(func() {
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
	previous := gco
	gco = nil
	t.Cleanup(func() { gco = previous; ResetRandomSeed() })
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
