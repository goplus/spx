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
	"fmt"
	"math"
	"strconv"
	"testing"
)

func TestToFloat64String(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  float64
		ok    bool
	}{
		{"double minus becomes positive", "--5.5", 5.5, true},
		{"plus minus becomes negative", "+-12", -12, true},
		{"generated negation pattern on negative value", "-" + fmt.Sprintf("%v", -18.25), 18.25, true},
		{"empty after trimming", " ", 0, false},
		{"overflow", "1e309", 0, false},
		{"NaN", "NaN", math.NaN(), true},
		{"infinity", "+Inf", math.Inf(1), true},
		{"negative zero", "-0", math.Copysign(0, -1), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ToFloat64(test.input)
			equal := math.Float64bits(got) == math.Float64bits(test.want) || math.IsNaN(got) && math.IsNaN(test.want)
			if ok != test.ok || !equal {
				t.Errorf("ToFloat64(%q) = %g, %v; want %g, %v", test.input, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestNormalizeSigns(t *testing.T) {
	for _, test := range []struct {
		name, input, want string
		ok                bool
	}{
		{"plain number unchanged", "12.5", "12.5", true},
		{"double minus becomes positive", "--7", "7", true},
		{"plus minus becomes negative", "+-12", "-12", true},
		{"trim surrounding spaces", "  --18.25  ", "18.25", true},
		{"signs only invalid", "--", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := normalizeSigns(test.input); ok != test.ok || got != test.want {
				t.Fatalf("normalizeSigns(%q) = %q, %v; want %q, %v", test.input, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestToIntString(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  int
		ok    bool
	}{
		{"compound sign integer", "--7", 7, true},
		{"generated negation pattern on negative integer", "-" + fmt.Sprintf("%v", -24), 24, true},
		{"float string truncates like int conversion", "--7.9", 7, true},
		{"negative fraction truncates toward zero", "-7.9", -7, true},
		{"max int preserved", strconv.Itoa(math.MaxInt), math.MaxInt, true},
		{"min int preserved", strconv.Itoa(math.MinInt), math.MinInt, true},
		{"signs only invalid", "--", 0, false},
		{"empty", "", 0, false},
		{"empty after trimming", "  ", 0, false},
		{"plus only invalid", "+", 0, false},
		{"positive overflow", "1e30", 0, false},
		{"negative overflow", "-1e30", 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := ToInt(test.input); ok != test.ok || got != test.want {
				t.Fatalf("ToInt(%q) = %d, %v; want %d, %v", test.input, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestToIntPreservesIntegerPrecision(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("requires 64-bit int")
	}
	want := int64(1<<53) + 1
	for _, input := range []any{want, uint64(want), strconv.FormatInt(want, 10)} {
		t.Run(fmt.Sprintf("%T", input), func(t *testing.T) {
			if got, ok := ToInt(input); !ok || int64(got) != want {
				t.Fatalf("ToInt(%v) = %d, %v; want %d, true", input, got, ok, want)
			}
		})
	}
}

func TestToIntOutOfRange(t *testing.T) {
	inputs := []any{^uint(0), ^uint64(0), float32(1e30), float64(1e30), float32(-1e30), float64(-1e30)}
	if strconv.IntSize == 32 {
		inputs = append(inputs, int64(1<<40), int64(-1<<31)-1, uint32(1<<31))
	}
	for _, input := range inputs {
		t.Run(fmt.Sprintf("%T(%v)", input, input), func(t *testing.T) {
			if got, ok := ToInt(input); ok || got != 0 {
				t.Fatalf("ToInt() = %d, %v; want 0, false", got, ok)
			}
		})
	}
}

func TestValueEqualUsesCompatibilitySemantics(t *testing.T) {
	if !NewValue(1).Equal(1.0) {
		t.Fatal("Value.Equal should treat integer and float values equally")
	}
	if !NewValue("Apple").Equal("apple") {
		t.Fatal("Value.Equal should compare strings case-insensitively")
	}
}

func TestValueNumericConversions(t *testing.T) {
	for _, test := range []struct {
		input any
		want  int
	}{
		{true, 1},
		{false, 0},
		{int(7), 7},
		{int8(7), 7},
		{int16(7), 7},
		{int32(7), 7},
		{int64(7), 7},
		{uint(7), 7},
		{uint8(7), 7},
		{uint16(7), 7},
		{uint32(7), 7},
		{uint64(7), 7},
		{float32(7), 7},
		{float64(7), 7},
	} {
		t.Run(fmt.Sprintf("%T(%v)", test.input, test.input), func(t *testing.T) {
			value := NewValue(test.input)
			if got := value.Raw(); got != test.input {
				t.Errorf("Value.Raw() = %T(%v), want %T(%v)", got, got, test.input, test.input)
			}
			if got := value.Int(); got != test.want {
				t.Errorf("Value.Int() = %d, want %d", got, test.want)
			}
			if got := value.Float(); got != float64(test.want) {
				t.Errorf("Value.Float() = %g, want %d", got, test.want)
			}
			if got, ok := ToInt(test.input); !ok || got != test.want {
				t.Errorf("ToInt() = %d, %v; want %d, true", got, ok, test.want)
			}
			if got, ok := ToFloat64(test.input); !ok || got != float64(test.want) {
				t.Errorf("ToFloat64() = %g, %v; want %d, true", got, ok, test.want)
			}
		})
	}
}

func TestValueConversionFailureReturnsZero(t *testing.T) {
	tests := []struct {
		name  string
		value Value
	}{
		{name: "invalid string", value: NewValue("not a number")},
		{name: "unsupported type", value: NewValue([]int{1})},
		{name: "zero value", value: Value{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.value.Int(); got != 0 {
				t.Errorf("Value.Int() = %d, want 0", got)
			}
			if got := tt.value.Float(); got != 0 {
				t.Errorf("Value.Float() = %f, want 0", got)
			}
		})
	}
}

func TestValueSet(t *testing.T) {
	var absent *Value
	absent.Set("ignored")
	value := NewValue("old")
	value.Set(NewValue("new"))
	if value.String() != "new" {
		t.Fatalf("Set = %v", value)
	}
	if got := value.Raw(); got != "new" {
		t.Fatalf("Set retained a wrapped value: %T(%v)", got, got)
	}
}

func TestValueWrapping(t *testing.T) {
	inner := NewValue("value")
	wrapped := WrapValue(inner)
	if got := wrapped.Raw(); got != inner {
		t.Fatalf("WrapValue = %#v, want unchanged Value %#v", got, inner)
	}
	if got := NewValue(wrapped).Raw(); got != inner {
		t.Fatalf("NewValue = %#v, want exactly one wrapper removed", got)
	}
}
