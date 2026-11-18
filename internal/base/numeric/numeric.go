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

// Package numeric provides conversions for built-in Go numbers and numeric slices.
package numeric

import "math"

// Float64 converts built-in integer and floating-point values to float64.
// Other types, including defined number types, return zero and false.
func Float64(value any) (float64, bool) {
	switch value := value.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int8:
		return float64(value), true
	case int16:
		return float64(value), true
	case int32:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint:
		return float64(value), true
	case uint8:
		return float64(value), true
	case uint16:
		return float64(value), true
	case uint32:
		return float64(value), true
	case uint64:
		return float64(value), true
	}
	return 0, false
}

// Int converts built-in numbers to int, truncating floats toward zero.
// Non-finite floats, out-of-range results, and other types return zero and false.
func Int(value any) (int, bool) {
	switch value := value.(type) {
	case int:
		return value, true
	case int8:
		return int(value), true
	case int16:
		return int(value), true
	case int32:
		return int(value), true
	case int64:
		if value >= math.MinInt && value <= math.MaxInt {
			return int(value), true
		}
	case uint:
		if value <= math.MaxInt {
			return int(value), true
		}
	case uint8:
		return int(value), true
	case uint16:
		return int(value), true
	case uint32:
		if uint64(value) <= uint64(math.MaxInt) {
			return int(value), true
		}
	case uint64:
		if value <= math.MaxInt {
			return int(value), true
		}
	case float32:
		return floatToInt(float64(value))
	case float64:
		return floatToInt(value)
	}
	return 0, false
}

// ConvertFloatSlice converts floating-point values into a new slice, preserving nil.
func ConvertFloatSlice[To, From ~float32 | ~float64](slice []From) []To {
	if slice == nil {
		return nil
	}
	out := make([]To, len(slice))
	for i, v := range slice {
		out[i] = To(v)
	}
	return out
}

func floatToInt(value float64) (int, bool) {
	value = math.Trunc(value)
	// The exclusive upper bound is exact in float64; this also rejects NaN.
	if !(value >= float64(math.MinInt) && value < -float64(math.MinInt)) {
		return 0, false
	}
	return int(value), true
}
