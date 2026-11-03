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

package scratch

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestRandomScratchBranchesAndRounding(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	lastUnit := math.Nextafter(1, 0)
	for _, tt := range []struct {
		name       string
		from, to   any
		unit, want float64
		calls      int
	}{
		{"integer lower endpoint", 1, 12, 0, 1, 1},
		{"integer upper endpoint", 1, 12, lastUnit, 12, 1},
		{"negative integer lower endpoint", -3, -1, 0, -3, 1},
		{"negative integer upper endpoint", -3, -1, lastUnit, -1, 1},
		{"cross-zero lower endpoint", -1, 1, 0, -1, 1},
		{"cross-zero upper endpoint", -1, 1, lastUnit, 1, 1},
		{"binary lower endpoint", 0, 1, 0, 0, 1},
		{"binary upper endpoint", 0, 1, lastUnit, 1, 1},
		{"integers", 1, 6, 0.75, 5, 1},
		{"reversed integers", 6, 1, 0.75, 5, 1},
		{"fractional lower bound", 1.5, 12, 0.5, 6.75, 1},
		{"fractional upper bound", 1, 12.5, 0.5, 6.75, 1},
		{"reversed fractional bounds", 12.5, 1, 0.5, 6.75, 1},
		{"negative fractional bounds", -3, -1.5, 0.5, -2.25, 1},
		{"continuous lower endpoint", 0, 0.5, 0, 0, 1},
		{"continuous near upper endpoint", 0, 0.5, lastUnit, math.Nextafter(0.5, 0), 1},
		{"decimal strings", "1.0", "6.0", 0.5, 3.5, 1},
		{"reversed decimal strings", "6", "1.0", 0.5, 3.5, 1},
		{"decimal exponent strings", "1.0e-2", "1", 0.5, 0.505, 1},
		{"exponent strings use integer branch", "1e-1", "2e-1", 0.95, 1.1, 1},
		{"boolean bounds", false, true, 0.75, 1, 1},
		{"null is not an integer", nil, 6, 0.75, 4.5, 1},
		{"unsupported Go type", struct{}{}, 6, 0.75, 4.5, 1},
		{"invalid string with decimal point", "foo.bar", 6, 0.75, 4.5, 1},
		{"NaN casts to zero", math.NaN(), 0, 0.5, 0, 0},
		{"NaN selects integer branch", math.NaN(), 6, 0.75, 5, 1},
		{"equal values", "4.0", 4, 0.5, 4, 0},
		{"equal integer bounds", 7, 7, 0.5, 7, 0},
		{"equal fractional bounds", 0.5, 0.5, 0.5, 0.5, 0},
		{"equal large bounds", 1e20, 1e20, 0.5, 1e20, 0},
		{"negative zero first", negativeZero, 0, 0.5, negativeZero, 0},
		{"positive zero first", 0, negativeZero, 0.5, 0, 0},
		{"equal infinities", math.Inf(1), math.Inf(1), 0.5, math.Inf(1), 0},
		{"infinite upper bound", 0, math.Inf(1), 0.5, math.Inf(1), 1},
		{"zero draw with infinite width", 0, math.Inf(1), 0, math.NaN(), 1},
		{"opposite infinities", math.Inf(-1), math.Inf(1), 0.5, math.NaN(), 1},
		{"round upper bound before subtraction", float64(1<<53) - 1, float64(1 << 53), 0.75, float64(1<<53) - 1, 1},
		{"scientific numbers select continuous branch", 1e21, 1e21 + 262144, 0.2500001, 1e21 + 131072, 1},
		{"multiply rounds before add", -1.0, math.Ldexp(1, -27), 1 - math.Ldexp(1, -27), 0, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			got := Random(tt.from, tt.to, func() float64 {
				calls++
				return tt.unit
			})
			if !sameRandomNumber(got, tt.want) || calls != tt.calls {
				t.Fatalf("Random(%v, %v) = %v, %d draws; want %v, %d draws", tt.from, tt.to, got, calls, tt.want, tt.calls)
			}
		})
	}
}

func TestRandomStringNumberConversion(t *testing.T) {
	for _, tt := range []struct {
		input   string
		want    float64
		integer bool
	}{
		{"", 0, true},
		{"\ufeff\u00a0\u2000\u2028 12\u2029\u3000", 12, true},
		{"\u008512", 0, true},
		{"\u180e12", 0, true},
		{" 1.0 ", 1, false},
		{"1e-1", 0.1, true},
		{"+12", 12, true},
		{"-0", math.Copysign(0, -1), true},
		{"-.0", math.Copysign(0, -1), false},
		{"1.e2", 100, false},
		{".5", 0.5, false},
		{"01", 1, true},
		{"1e309", math.Inf(1), true},
		{"-1.0e309", math.Inf(-1), false},
		{"1e-999", 0, true},
		{"-1e-999", math.Copysign(0, -1), true},
		{"Infinity", math.Inf(1), true},
		{"+Infinity", math.Inf(1), true},
		{"-Infinity", math.Inf(-1), true},
		{"0x10", 16, true},
		{"0XFF", 255, true},
		{"0b101", 5, true},
		{"0B101", 5, true},
		{"0o17", 15, true},
		{"0O17", 15, true},
		{"0x100000000000000000000", math.Ldexp(1, 80), true},
		{"0x20000000000001", 9007199254740992, true},
		{"0x20000000000003", 9007199254740996, true},
		{"9007199254740993", 9007199254740992, true},
		{"0x1" + strings.Repeat("0", 256), math.Inf(1), true},
		{"0.1000000000000000055511151231257827021181583404541015625", 0.1, false},
		{"NaN", 0, true},
		{"Inf", 0, true},
		{"infinity", 0, true},
		{"1_0", 0, true},
		{"0x1p2", 0, true},
		{"+0x10", 0, true},
		{"-0x10", 0, true},
		{"0x-10", 0, true},
		{"0x+10", 0, true},
		{"0b1_0", 0, true},
		{"0o8", 0, true},
		{"0x", 0, true},
		{"--5", 0, true},
		{"1 2", 0, true},
		{"1e", 0, true},
		{"0x1.2", 0, false},
		{".", 0, false},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got, integer := randomNumber(tt.input)
			if !sameRandomNumber(got, tt.want) || integer != tt.integer {
				t.Fatalf("randomNumber(%q) = %v, %v; want %v, %v", tt.input, got, integer, tt.want, tt.integer)
			}
		})
	}
}

func TestRandomNumericInputsUseJSNumbers(t *testing.T) {
	for _, value := range []any{int(7), int8(7), int16(7), int32(7), int64(7), uint(7), uint8(7), uint16(7), uint32(7), uint64(7), float32(7), float64(7)} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) {
			if got, integer := randomNumber(value); got != 7 || !integer {
				t.Fatalf("randomNumber(%T) = %v, %v; want 7, true", value, got, integer)
			}
		})
	}
	for _, tt := range []struct {
		input   any
		want    float64
		integer bool
	}{
		{int64(1<<53 + 1), 1 << 53, true},
		{uint64(math.MaxUint64), 1 << 64, true},
		{float32(0.1), float64(float32(0.1)), false},
		{math.Nextafter(1e21, 0), math.Nextafter(1e21, 0), true},
		{1e21, 1e21, false},
		{-1e21, -1e21, false},
		{math.NaN(), 0, true},
		{math.Inf(1), math.Inf(1), false},
		{math.Inf(-1), math.Inf(-1), false},
		{math.Copysign(0, -1), math.Copysign(0, -1), true},
		{true, 1, true},
		{false, 0, true},
		{nil, 0, false},
		{[]any{2}, 0, false},
	} {
		if got, integer := randomNumber(tt.input); !sameRandomNumber(got, tt.want) || integer != tt.integer {
			t.Errorf("randomNumber(%v) = %v, %v; want %v, %v", tt.input, got, integer, tt.want, tt.integer)
		}
	}
}

func sameRandomNumber(got, want float64) bool {
	return math.Float64bits(got) == math.Float64bits(want) || math.IsNaN(got) && math.IsNaN(want)
}
