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

package engine

import (
	"math"
	"testing"
)

func TestFixedPointConversions(t *testing.T) {
	for _, value := range []float64{-2.5, 0, 3.25} {
		if ConvertToFloat64(ConvertToInt64(value)) != value {
			t.Fatalf("fixed-point round trip=%v", value)
		}
	}
}

func TestAnglesAndVectors(t *testing.T) {
	for _, degree := range []float64{-360, -180, -90, 0, 90, 180, 360} {
		rad := DegToRad(degree)
		if math.Abs(rad-degree*math.Pi/180) > 1e-12 {
			t.Errorf("angle conversion failed for %v degrees", degree)
		}
	}
	for _, tc := range []struct{ rad, sin, cos float64 }{
		{0, 0, 1}, {math.Pi / 2, 1, 0}, {math.Pi, 0, -1}, {-math.Pi / 2, -1, 0},
	} {
		got := Sincos(tc.rad)
		if math.Abs(got.X-tc.sin) > 1e-12 || math.Abs(got.Y-tc.cos) > 1e-12 {
			t.Errorf("Sincos(%v) = %v, want (%v, %v)", tc.rad, got, tc.sin, tc.cos)
		}
	}
	for _, value := range []float64{-2.5, 0, 3.25} {
		if got := UniformVec2(value); got.X != value || got.Y != value {
			t.Errorf("UniformVec2(%v) = %v", value, got)
		}
	}
}

func FuzzSplitInt64BatchBits(f *testing.F) {
	// The final two seeds carry infinity and NaN bit patterns in both halves.
	for _, value := range []int64{0, 1, -1, math.MinInt64, math.MaxInt64, 1 << 32, 0x7f8000007f800000, 0x7fc000007fc00000} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value int64) {
		low, high := splitInt64BitsToFloat32(value)
		got := int64(uint64(math.Float32bits(high))<<32 | uint64(math.Float32bits(low)))
		if got != value {
			t.Fatalf("batch ID round trip = %#016x, want %#016x", got, value)
		}
	})
}

func TestBatchBufferReuseAndGrowth(t *testing.T) {
	for _, tc := range []struct {
		name   string
		buffer []float32
		size   int
	}{
		{"nil empty", nil, 0},
		{"allocate minimum", nil, 1},
		{"allocate larger", nil, 17},
		{"shrink", make([]float32, 4, 8), 2},
		{"reuse capacity", make([]float32, 2, 8), 8},
		{"grow capacity", make([]float32, 2, 8), 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.buffer) != 0 {
				tc.buffer[0] = 42
			}
			got := ensureFloat32BufferSize(tc.buffer, tc.size)
			if len(got) != tc.size || cap(got) < tc.size {
				t.Fatalf("length/capacity = %d/%d, requested %d", len(got), cap(got), tc.size)
			}
			if tc.size > cap(tc.buffer) && cap(got) <= tc.size {
				t.Fatalf("growth capacity = %d, want headroom beyond %d", cap(got), tc.size)
			}
			if tc.buffer == nil && tc.size == 0 && got != nil {
				t.Fatal("zero-sized nil buffer was allocated")
			}
			if tc.size == 0 || len(tc.buffer) == 0 {
				return
			}
			if tc.size <= cap(tc.buffer) {
				if &got[0] != &tc.buffer[0] || got[0] != 42 {
					t.Fatal("available capacity was not reused with its contents intact")
				}
			} else {
				got[0] = 99
				if tc.buffer[0] != 42 {
					t.Fatal("growing the buffer overwrote the original storage")
				}
			}
		})
	}
	if boolArg(false) != 0 || boolArg(true) != 1 {
		t.Fatal("boolean batch argument encoding changed")
	}
}
