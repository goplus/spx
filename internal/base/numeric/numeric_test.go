/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package numeric

import (
	"math"
	"slices"
	"strconv"
	"testing"
)

type definedInt int

func TestFloat64ConvertsBuiltinNumbersOnly(t *testing.T) {
	for _, test := range []struct {
		value any
		want  float64
	}{
		{int64(1<<53 + 1), 1 << 53},
		{uint64(7), 7},
		{float32(0.25), 0.25},
		{math.Inf(1), math.Inf(1)},
	} {
		if got, ok := Float64(test.value); !ok || got != test.want {
			t.Errorf("Float64(%T) = %v, %v; want %v, true", test.value, got, ok, test.want)
		}
	}
	for _, value := range []any{nil, true, "12", definedInt(12)} {
		if got, ok := Float64(value); ok || got != 0 {
			t.Errorf("Float64(%T) = %v, %v; want 0, false", value, got, ok)
		}
	}
}

func TestIntConvertsBuiltinNumbersOnly(t *testing.T) {
	for _, value := range []any{
		int(7), int8(7), int16(7), int32(7), int64(7),
		uint(7), uint8(7), uint16(7), uint32(7), uint64(7),
		float32(7), float64(7),
	} {
		if got, ok := Int(value); !ok || got != 7 {
			t.Errorf("Int(%T(%v)) = %d, %v; want 7, true", value, value, got, ok)
		}
	}
	type definedFloat float64
	for _, value := range []any{nil, true, "7", definedInt(7), definedFloat(7), uintptr(7)} {
		if got, ok := Int(value); ok || got != 0 {
			t.Errorf("Int(%T(%v)) = %d, %v; want 0, false", value, value, got, ok)
		}
	}
}

func TestIntBoundsAndPrecision(t *testing.T) {
	limit := math.Ldexp(1, strconv.IntSize-1)
	below64, below32 := int64(1<<31-1), int64(1<<31-128)
	if strconv.IntSize == 64 {
		below64, below32 = 1<<63-1024, 1<<63-1<<39
	}
	for _, test := range []struct {
		name  string
		value any
		want  int
	}{
		{"max int", math.MaxInt, math.MaxInt},
		{"min int", math.MinInt, math.MinInt},
		{"max signed", int64(math.MaxInt), math.MaxInt},
		{"min signed", int64(math.MinInt), math.MinInt},
		{"max unsigned", uint64(math.MaxInt), math.MaxInt},
		{"float64 below upper bound", math.Nextafter(limit, 0), int(below64)},
		{"float32 below upper bound", math.Nextafter32(float32(limit), 0), int(below32)},
		{"float64 lower bound", -limit, math.MinInt},
		{"float32 lower bound", float32(-limit), math.MinInt},
		{"positive fraction", 7.9, 7},
		{"negative fraction", -7.9, -7},
		{"float32 fraction", float32(-7.9), -7},
		{"positive zero", 0.0, 0},
		{"negative zero", math.Copysign(0, -1), 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := Int(test.value); !ok || got != test.want {
				t.Errorf("Int(%v) = %d, %v; want %d, true", test.value, got, ok, test.want)
			}
		})
	}
	invalid := []any{
		uint(math.MaxInt) + 1, uint64(math.MaxInt) + 1,
		limit, float32(limit), -limit - 2048, math.Nextafter32(float32(-limit), float32(math.Inf(-1))),
		math.NaN(), math.Inf(1), math.Inf(-1),
		float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1)),
	}
	if strconv.IntSize == 32 {
		invalid = append(invalid, int64(1<<31), int64(-1<<31-1), uint32(1<<31))
	} else {
		invalid = append(invalid, float64(math.MaxInt), math.Nextafter(-limit, math.Inf(-1)))
	}
	for _, value := range invalid {
		if got, ok := Int(value); ok || got != 0 {
			t.Errorf("Int(%T(%v)) = %d, %v; want 0, false", value, value, got, ok)
		}
	}
	if strconv.IntSize == 64 {
		// Integer inputs must never round through float64.
		want := int64(1<<53 + 1)
		for _, value := range []any{want, uint64(want)} {
			if got, ok := Int(value); !ok || int64(got) != want {
				t.Errorf("Int(%T(%v)) = %d, %v; want %d, true", value, value, got, ok, want)
			}
		}
	} else {
		// These fractions are representable on 32-bit hosts and truncate in range.
		for _, test := range []struct {
			value float64
			want  int
		}{
			{float64(math.MaxInt) + 0.5, math.MaxInt},
			{float64(math.MinInt) - 0.5, math.MinInt},
			{math.Nextafter(-limit, math.Inf(-1)), math.MinInt},
		} {
			if got, ok := Int(test.value); !ok || got != test.want {
				t.Errorf("Int(%v) = %d, %v; want %d, true", test.value, got, ok, test.want)
			}
		}
	}
}

func TestFloatSliceConversions(t *testing.T) {
	t.Run("nil and empty", func(t *testing.T) {
		if ConvertFloatSlice[float32, float64](nil) != nil || ConvertFloatSlice[float64, float32](nil) != nil {
			t.Fatal("conversion did not preserve nil")
		}
		if got := ConvertFloatSlice[float32]([]float64{}); got == nil || len(got) != 0 {
			t.Fatalf("ConvertFloatSlice[float32](empty) = %#v, want non-nil empty slice", got)
		}
		if got := ConvertFloatSlice[float64]([]float32{}); got == nil || len(got) != 0 {
			t.Fatalf("ConvertFloatSlice[float64](empty) = %#v, want non-nil empty slice", got)
		}
	})
	t.Run("values and ownership", func(t *testing.T) {
		input := []float64{-2.5, 0, 1.25}
		narrowed := ConvertFloatSlice[float32](input)
		if !slices.Equal(narrowed, []float32{-2.5, 0, 1.25}) {
			t.Fatalf("ConvertFloatSlice[float32] = %v", narrowed)
		}
		widened := ConvertFloatSlice[float64](narrowed)
		if !slices.Equal(widened, input) {
			t.Fatalf("ConvertFloatSlice[float64] = %v, want %v", widened, input)
		}
		narrowed[0] = 7
		widened[1] = 8
		if input[0] != -2.5 || input[1] != 0 || widened[0] != -2.5 || narrowed[1] != 0 {
			t.Fatal("conversion shared mutable storage")
		}
	})
	t.Run("IEEE values", func(t *testing.T) {
		input := []float64{math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN(), math.MaxFloat64, -math.MaxFloat64}
		got := ConvertFloatSlice[float64](ConvertFloatSlice[float32](input))
		if len(got) != len(input) {
			t.Fatalf("converted length = %d, want %d", len(got), len(input))
		}
		if got[0] != 0 || !math.Signbit(got[0]) || !math.IsInf(got[1], 1) || !math.IsInf(got[2], -1) || !math.IsNaN(got[3]) {
			t.Fatalf("special values were not preserved: %v", got[:4])
		}
		if !math.IsInf(got[4], 1) || !math.IsInf(got[5], -1) {
			t.Fatalf("overflow did not preserve sign: %v", got[4:])
		}
	})
}
