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
	"strings"

	"github.com/goplus/spx/v4/internal/engine"
	itime "github.com/goplus/spx/v4/internal/time"
)

const compareMaxULPs = 2

// Rand__0 returns a random integer between from and to (inclusive).
func Rand__0(from, to int) float64 {
	if to < from {
		from, to = to, from
	}
	return float64(from + randomIntn(to-from+1))
}

// Rand__1 returns a random number between from and to. Like Scratch, integral
// bounds select an integer from the inclusive range, even when stored as floats.
func Rand__1(from, to float64) float64 {
	if to < from {
		from, to = to, from
	}
	if from == to {
		return from
	}
	if math.Mod(from, 1) == 0 && math.Mod(to, 1) == 0 {
		return from + math.Floor(randomFloat64()*(to-from+1))
	}
	return randomFloat64()*(to-from) + from
}

// Iround returns an integer value, while math.Round returns a float value.
func Iround(v float64) int {
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}

// LimitPrecision snaps a coordinate to the nearest integer when the distance is
// less than 1e-9, matching Scratch's own x/y position reporters. Other values,
// including NaN and infinities, are returned unchanged.
// In SPX scripts, use limitPrecision(xpos) or limitPrecision(ypos) for the
// sprite's own coordinate blocks; read other sprites' coordinates directly.
func LimitPrecision(coordinate float64) float64 {
	rounded := math.Round(coordinate)
	if math.Abs(coordinate-rounded) < 1e-9 {
		return rounded
	}
	return coordinate
}

// Div returns a divided by b, converting a NaN result to zero while preserving
// infinities. For example, Div(0, 0) returns 0 and Div(1, 0) returns +Inf.
// In SPX scripts, it can be called as div(a, b).
func Div(a, b float64) float64 {
	result := a / b
	if math.IsNaN(result) {
		return 0
	}
	return result
}

// FloorMod returns the remainder of dividend divided by divisor using
// floored-division semantics. Unlike math.Mod, a non-zero result has the same
// sign as divisor. This matches Scratch's mod operator:
// https://github.com/scratchfoundation/scratch-vm/blob/3070e30241c391254d94f3a98129f9e96b902631/src/blocks/scratch3_operators.js#L118-L125
func FloorMod(dividend, divisor float64) float64 {
	result := math.Mod(dividend, divisor)
	if result/divisor < 0 {
		result += divisor
	}
	return result
}

// Contains reports whether substr is within s using Scratch's
// case-insensitive string matching semantics.
func Contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// Compare compares values using the same rules as Scratch's =, <, and >
// operator blocks.
//
// Values are compared numerically when both sides can be interpreted as numbers;
// otherwise they are compared as case-insensitive strings.
func Compare(v1, v2 any) int {
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

	s1 := strings.ToLower(toString(v1))
	s2 := strings.ToLower(toString(v2))
	return strings.Compare(s1, s2)
}

// Equal reports whether two values match Scratch's = operator semantics.
// Numeric values tolerate a small number of adjacent floating-point values to
// absorb representation-level rounding errors. Compare remains exact for
// ordering.
func Equal(v1, v2 any) bool {
	return equalValues(normalizeEqualValue(v1), normalizeEqualValue(v2))
}

// equalValue keeps the normalized forms needed by repeated equality checks.
// Lists use it to normalize the searched value once instead of once per item.
type equalValue struct {
	raw         any
	number      float64
	integer     int64
	numeric     bool
	fastInteger bool
}

func normalizeEqualValue(v any) equalValue {
	v = fromObj(v)
	if i, ok := equalityFastInteger(v); ok {
		return equalValue{
			raw:         v,
			number:      float64(i),
			integer:     i,
			numeric:     true,
			fastInteger: true,
		}
	}
	n, ok := toCompareNumber(v)
	return equalValue{raw: v, number: n, numeric: ok}
}

func equalValues(v1, v2 equalValue) bool {
	if v1.fastInteger && v2.fastInteger {
		return v1.integer == v2.integer
	}
	if v1.numeric && v2.numeric {
		return nearlyEqual(v1.number, v2.number)
	}
	// Empty strings are deliberately non-numeric in Scratch. A primitive
	// numeric value can never have an empty textual representation.
	if (v1.numeric && isBlankString(v2.raw)) || (v2.numeric && isBlankString(v1.raw)) {
		return false
	}
	return strings.ToLower(toString(v1.raw)) == strings.ToLower(toString(v2.raw))
}

func isBlankString(v any) bool {
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) == ""
}

// equalityFastInteger covers the range used by generated block values while
// staying far below the point where adjacent integers are within the ULP
// tolerance used by Equal.
func equalityFastInteger(v any) (int64, bool) {
	const (
		min = int64(-1 << 31)
		max = int64(1<<31 - 1)
	)
	var i int64
	switch v := v.(type) {
	case int:
		i = int64(v)
	case int8:
		i = int64(v)
	case int16:
		i = int64(v)
	case int32:
		i = int64(v)
	case int64:
		i = v
	case uint:
		if uint64(v) > uint64(max) {
			return 0, false
		}
		i = int64(v)
	case uint8:
		i = int64(v)
	case uint16:
		i = int64(v)
	case uint32:
		if uint64(v) > uint64(max) {
			return 0, false
		}
		i = int64(v)
	case uint64:
		if v > uint64(max) {
			return 0, false
		}
		i = int64(v)
	default:
		return 0, false
	}
	return i, i >= min && i <= max
}

// DeltaTime returns the time elapsed since the previous frame.
func DeltaTime() Seconds {
	return itime.DeltaTime()
}

// TimeSinceLevelLoad returns the time elapsed since the current level started.
func TimeSinceLevelLoad() Seconds {
	return itime.TimeSinceLevelLoad()
}

// CharAt returns the character at the given 0-based index in the string s.
// It is similar to Scratch's operator_letter_of block, but uses Go-style
// 0-based indexing. It returns an empty string if idx is out of range for the
// rune sequence.
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

// Exit__0 exits the program with exit code 0.
func Exit__0() {
	engine.RequestExit(0)
}

// Exit__1 exits the program with the specified exit code.
func Exit__1(code int) {
	engine.RequestExit(int64(code))
}

// PenColorParamFromString converts a string to its corresponding PenColorParam.
// It accepts "color", "saturation", "brightness", and "transparency" (case-insensitive).
// Returns PenNone if the string is not recognized.
func PenColorParamFromString(s string) PenColorParam {
	switch strings.ToLower(s) {
	case "color":
		return PenHue
	case "saturation":
		return PenSaturation
	case "brightness":
		return PenBrightness
	case "transparency":
		return PenTransparency
	default:
		return PenNone
	}
}

func nearlyEqual(a, b float64) bool {
	if a == b {
		return true
	}
	if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return false
	}
	for range compareMaxULPs {
		a = math.Nextafter(a, b)
		if a == b {
			return true
		}
	}
	return false
}

func toCompareNumber(v any) (float64, bool) {
	v = fromObj(v)
	if v == nil {
		return 0, false
	}
	if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
		return 0, false
	}
	return toFloat64Any(v)
}
