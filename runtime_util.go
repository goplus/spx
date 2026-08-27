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

	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

// Rand__0 returns a random integer between from and to (inclusive).
func Rand__0(from, to int) float64 {
	if to < from {
		from, to = to, from
	}
	return float64(from + randomIntn(to-from+1))
}

// Rand__1 returns a random float64 between from and to.
func Rand__1(from, to float64) float64 {
	if to < from {
		from, to = to, from
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

// Contains reports whether substr is within s.
func Contains(s, substr string) bool {
	return strings.Contains(s, substr)
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
	switch {
	case s1 < s2:
		return -1
	case s1 > s2:
		return 1
	default:
		return 0
	}
}

// Equal reports whether two values match Scratch's = operator semantics.
func Equal(v1, v2 any) bool {
	return Compare(v1, v2) == 0
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
