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

// Package scratch implements the shared compatibility rules used by SPX.
package scratch

import (
	"math"
	"strings"
)

const maxEqualityULPs = 2

// LimitPrecision rounds a coordinate when it is strictly within 1e-9 of an
// integer. NaN and infinities pass through.
func LimitPrecision(coordinate float64) float64 {
	rounded := math.Round(coordinate)
	if math.Abs(coordinate-rounded) < 1e-9 {
		return rounded
	}
	return coordinate
}

// Div divides a by b, converting a NaN result to zero and preserving infinities.
func Div(a, b float64) float64 {
	result := a / b
	if math.IsNaN(result) {
		return 0
	}
	return result
}

// FloorMod implements Scratch's remainder adjustment: it adds divisor when
// remainder/divisor is negative, preserving floating-point underflow behavior.
// A zero divisor produces NaN. This follows Scratch's mod operator:
// https://github.com/scratchfoundation/scratch-vm/blob/3070e30241c391254d94f3a98129f9e96b902631/src/blocks/scratch3_operators.js#L118-L125
func FloorMod(dividend, divisor float64) float64 {
	result := math.Mod(dividend, divisor)
	if result/divisor < 0 {
		result += divisor
	}
	return result
}

// Contains reports whether substr occurs in s, ignoring case.
func Contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// CharAt returns the rune at the 0-based index in s, or "" if out of range.
func CharAt(s string, idx int) string {
	if idx < 0 {
		return ""
	}
	for _, r := range s {
		if idx == 0 {
			return string(r)
		}
		idx--
	}
	return ""
}

// Compare compares numbers when both operands are numeric, otherwise strings
// ignoring case. It returns -1, 0, or 1 without Equal's numeric tolerance.
// Value operands are unwrapped once.
func Compare(v1, v2 any) int {
	v1, v2 = UnwrapValue(v1), UnwrapValue(v2)
	if n1, ok1 := toCompareNumber(v1); ok1 {
		if n2, ok2 := toCompareNumber(v2); ok2 {
			switch {
			case n1 < n2:
				return -1
			case n1 > n2:
				return 1
			default:
				return 0
			}
		}
	}

	return compareStrings(v1, v2)
}

// Equal compares like Compare, allowing up to two ULPs of numeric rounding error.
// Numeric comparisons involving NaN return false.
// Value operands are unwrapped once.
func Equal(v1, v2 any) bool {
	v1, v2 = UnwrapValue(v1), UnwrapValue(v2)
	if n1, ok1 := toCompareNumber(v1); ok1 {
		if n2, ok2 := toCompareNumber(v2); ok2 {
			return nearlyEqual(n1, n2)
		}
	}
	return compareStrings(v1, v2) == 0
}

func compareStrings(v1, v2 any) int {
	s1 := strings.ToLower(ToString(v1))
	s2 := strings.ToLower(ToString(v2))
	return strings.Compare(s1, s2)
}

func nearlyEqual(a, b float64) bool {
	if a == b {
		return true
	}
	if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return false
	}
	for range maxEqualityULPs {
		a = math.Nextafter(a, b)
		if a == b {
			return true
		}
	}
	return false
}

func toCompareNumber(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	return ToFloat64(v)
}
