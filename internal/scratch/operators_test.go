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

package scratch

import (
	"math"
	"testing"
)

func TestLimitPrecision(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	inside := math.Nextafter(1e-9, 0)
	outside := math.Nextafter(1e-9, math.Inf(1))
	for _, tt := range []struct {
		name        string
		input, want float64
	}{
		{"integer", 42, 42},
		{"negative integer", -42, -42},
		{"zero", 0, 0},
		{"negative zero", negativeZero, negativeZero},
		{"near zero", 5e-10, 0},
		{"near negative zero", -5e-10, negativeZero},
		{"inside threshold", inside, 0},
		{"inside negative threshold", -inside, negativeZero},
		{"at threshold", 1e-9, 1e-9},
		{"at negative threshold", -1e-9, -1e-9},
		{"outside threshold", outside, outside},
		{"outside negative threshold", -outside, -outside},
		{"below integer", 42 - 5e-10, 42},
		{"above integer", 42 + 5e-10, 42},
		{"below negative integer", -42 - 5e-10, -42},
		{"above negative integer", -42 + 5e-10, -42},
		{"outside integer tolerance", 42 + 2e-9, 42 + 2e-9},
		{"outside negative integer tolerance", -42 - 2e-9, -42 - 2e-9},
		{"fraction", 12.25, 12.25},
		{"negative fraction", -12.75, -12.75},
		{"half integer", 0.5, 0.5},
		{"negative half integer", -0.5, -0.5},
		{"large coordinate", math.MaxFloat64, math.MaxFloat64},
		{"negative large coordinate", -math.MaxFloat64, -math.MaxFloat64},
		{"positive infinity", math.Inf(1), math.Inf(1)},
		{"negative infinity", math.Inf(-1), math.Inf(-1)},
		{"NaN", math.NaN(), math.NaN()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := LimitPrecision(tt.input)
			if math.IsNaN(tt.want) && math.IsNaN(got) {
				return
			}
			if got != tt.want || math.Signbit(got) != math.Signbit(tt.want) {
				t.Fatalf("LimitPrecision(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
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
		{name: "zero by negative divisor", a: 0, b: -1, want: negativeZero},
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
		{name: "exact division", dividend: -10, divisor: 5, want: math.Copysign(0, -1)},
		{name: "fractional operands", dividend: -2.5, divisor: 2, want: 1.5},
		{name: "remainder quotient underflows", dividend: -math.SmallestNonzeroFloat64, divisor: 100, want: -math.SmallestNonzeroFloat64},
		{name: "infinite divisor", dividend: -1, divisor: math.Inf(1), want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FloorMod(tt.dividend, tt.divisor); got != tt.want || math.Signbit(got) != math.Signbit(tt.want) {
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

func TestCharAt(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		index      int
		want       string
	}{
		{"first", "hello", 0, "h"},
		{"middle", "hello", 2, "l"},
		{"last", "hello", 4, "o"},
		{"at length", "hello", 5, ""},
		{"past length", "hello", 6, ""},
		{"negative", "hello", -1, ""},
		{"minimum index", "hello", math.MinInt, ""},
		{"maximum index", "hello", math.MaxInt, ""},
		{"empty", "", 0, ""},
		{"single character", "a", 0, "a"},
		{"Chinese first", "你好世界", 0, "你"},
		{"Chinese last", "你好世界", 3, "界"},
		{"Chinese out of range", "你好世界", 4, ""},
		{"emoji", "A😀B", 1, "😀"},
		{"after emoji", "A😀B", 2, "B"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := CharAt(tt.text, tt.index); got != tt.want {
				t.Fatalf("CharAt(%q, %d) = %q, want %q", tt.text, tt.index, got, tt.want)
			}
		})
	}
}

type embeddedValue struct{ Value }

func TestCompareAndEqual(t *testing.T) {
	items := NewList("01", "Apple")
	// Init can retain wrapped elements; At adds one more Value wrapper.
	var nested List
	nested.Init(NewValue(2))
	value := NewValue(2)
	var nilPointer *Value
	twoULPs := math.Nextafter(math.Nextafter(116, math.Inf(1)), math.Inf(1))
	threeULPs := math.Nextafter(twoULPs, math.Inf(1))

	tests := []struct {
		name    string
		v1      any
		v2      any
		compare int
		equal   bool
	}{
		{"case insensitive strings", "p", "P", 0, true},
		{"numeric strings", "01", "1", 0, true},
		{"number and string", 2, "10", -1, false},
		{"non numeric strings less", "apple", "Banana", -1, false},
		{"whitespace uses string compare", "", "0", -1, false},
		{"bool numeric compare", true, 1, 0, true},
		{"wrapped numeric string compares numerically", NewValue("01"), 1, 0, true},
		{"wrapped bool compares numerically", NewValue(true), 1, 0, true},
		{"wrapped text compares case insensitively", NewValue("Apple"), "apple", 0, true},
		{"zero value compares as empty text", Value{}, "", 0, true},
		{"list number", items.At(0), 1, 0, true},
		{"list text", items.At(1), "apple", 0, true},
		{"empty item", items.At(10), "", 0, true},
		{"empty item is not zero", items.At(10), 0, -1, false},
		{"blank is not zero", NewValue(" "), 0, -1, false},
		{"old converter signs", NewValue("--7"), 7, 0, true},
		{"nonnumeric right operand", 10, "2x", -1, false},
		{"single wrapper is numeric", value, 10, -1, false},
		{"nested wrapper stays textual", nested.At(0), 10, 1, false},
		{"embedded value stays textual", embeddedValue{value}, 10, 1, false},
		{"pointer stays textual", &value, 10, 1, false},
		{"nil pointer stays textual", nilPointer, "<nil>", 0, true},
		{"two ULPs", NewValue(twoULPs), 116, 1, true},
		{"three ULPs", NewValue(threeULPs), 116, 1, false},
		{"NaN retains comparison behavior", math.NaN(), math.NaN(), 0, false},
		{"same infinity", math.Inf(1), math.Inf(1), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCompare := Compare(tt.v1, tt.v2)
			if gotCompare != tt.compare {
				t.Fatalf("Compare(%v, %v) = %d, want %d", tt.v1, tt.v2, gotCompare, tt.compare)
			}
			if got := Compare(tt.v2, tt.v1); got != -tt.compare {
				t.Errorf("Compare(%v, %v) = %d, want %d", tt.v2, tt.v1, got, -tt.compare)
			}
			if got := Equal(tt.v2, tt.v1); got != tt.equal {
				t.Errorf("Equal(%v, %v) = %v, want %v", tt.v2, tt.v1, got, tt.equal)
			}
			gotEqual := Equal(tt.v1, tt.v2)
			if gotEqual != tt.equal {
				t.Fatalf("Equal(%v, %v) = %v, want %v", tt.v1, tt.v2, gotEqual, tt.equal)
			}
		})
	}
}

func TestEqualFastPathMatchesPreviousSemantics(t *testing.T) {
	values := []any{
		nil,
		"", "  ", "0", "01", "--1", "Apple", "true", "NaN",
		false, true,
		int(-1), int(0), int(1), int8(1), int16(2), int32(3),
		int64(1<<31 - 1), int64(1 << 31), int64(1 << 53), int64(1<<53 + 1),
		uint(1), uint32(1<<31 - 1), uint32(1 << 31), uint64(1<<53 + 1),
		float32(1), float64(-1), float64(0), float64(1),
		math.Nextafter(1, math.Inf(1)), math.Inf(-1), math.Inf(1), math.NaN(),
		NewValue("01"), NewValue(1), NewValue(true), Value{},
	}

	previousEqual := func(v1, v2 any) bool {
		v1, v2 = UnwrapValue(v1), UnwrapValue(v2)
		if n1, ok1 := toCompareNumber(v1); ok1 {
			if n2, ok2 := toCompareNumber(v2); ok2 {
				return nearlyEqual(n1, n2)
			}
		}
		return compareStrings(v1, v2) == 0
	}

	for _, v1 := range values {
		for _, v2 := range values {
			if got, want := Equal(v1, v2), previousEqual(v1, v2); got != want {
				t.Fatalf("Equal(%T(%v), %T(%v)) = %v, previous behavior %v", v1, v1, v2, v2, got, want)
			}
		}
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

func TestCompareListInsertionStopsAtMissingItem(t *testing.T) {
	tests := []struct {
		name    string
		initial []any
		insert  []float64
		want    []float64
	}{
		{
			name:   "negative value in empty list",
			insert: []float64{-2000},
			want:   []float64{-2000},
		},
		{
			name:    "negative value after last item",
			initial: []any{20, 0, -2},
			insert:  []float64{-2000},
			want:    []float64{20, 0, -2, -2000},
		},
		{
			name:    "repeated negative sentinel",
			initial: []any{-2000, -10000},
			insert:  []float64{-10000, -10000},
			want:    []float64{-2000, -10000, -10000, -10000},
		},
		{
			name:   "mixed sprite depths",
			insert: []float64{-10000, -2000, 175, 0, -180, -10000, -60, -60, 5},
			want:   []float64{175, 5, 0, -60, -60, -180, -2000, -10000, -10000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := NewList(tt.initial...)
			for _, depth := range tt.insert {
				index := 0
				// Scratch insertion loops compare the missing tail as an empty
				// string. Converting it to a float would loop forever for negatives.
				for Compare(depth, list.At(index)) < 0 {
					index++
					if index > list.Len() {
						t.Fatalf("inserting %g passed the missing tail of a %d-item list", depth, list.Len())
					}
				}
				list.Insert(index, depth)
			}
			if got := list.Len(); got != len(tt.want) {
				t.Fatalf("list length = %d, want %d", got, len(tt.want))
			}
			for index, want := range tt.want {
				if got := list.At(index).Float(); got != want {
					t.Errorf("list item %d = %g, want %g", index, got, want)
				}
			}
		})
	}
}

func TestEqualUsesULPToleranceOnlyForNumericEquality(t *testing.T) {
	twoULPsAbove116 := 116.0
	for range 2 {
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
