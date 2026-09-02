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

func TestContainsUsesScratchCaseInsensitiveSemantics(t *testing.T) {
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

func TestEqualUsesToleranceOnlyForNumericEquality(t *testing.T) {
	tests := []struct {
		name  string
		v1    any
		v2    any
		equal bool
	}{
		{name: "rounding error", v1: 116.00000000000001, v2: 116.0, equal: true},
		{name: "numeric string rounding error", v1: "116.00000000000001", v2: 116.0, equal: true},
		{name: "near zero", v1: 0.0, v2: 1e-12, equal: true},
		{name: "relative tolerance at large magnitude", v1: 1e12, v2: 1e12 + 100, equal: true},
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

func TestPenColorParamFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected PenColorParam
	}{
		{name: "color lowercase", input: "color", expected: PenHue},
		{name: "color uppercase", input: "COLOR", expected: PenHue},
		{name: "color mixed case", input: "Color", expected: PenHue},
		{name: "saturation", input: "saturation", expected: PenSaturation},
		{name: "saturation uppercase", input: "SATURATION", expected: PenSaturation},
		{name: "brightness", input: "brightness", expected: PenBrightness},
		{name: "brightness uppercase", input: "BRIGHTNESS", expected: PenBrightness},
		{name: "transparency", input: "transparency", expected: PenTransparency},
		{name: "transparency uppercase", input: "TRANSPARENCY", expected: PenTransparency},
		{name: "unknown string falls back to PenNone", input: "unknown", expected: PenNone},
		{name: "empty string falls back to PenNone", input: "", expected: PenNone},
		{name: "numeric string falls back to PenNone", input: "123", expected: PenNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PenColorParamFromString(tt.input)
			if got != tt.expected {
				t.Errorf("PenColorParamFromString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
