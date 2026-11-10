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

import "github.com/goplus/spx/v4/internal/core/scratch"

// List stores values using 0-based indexes.
//
// Deprecated: Use [github.com/goplus/spx/v4/pkg/scratchcompat.List] instead.
type List = scratch.List

// NewList creates a new List initialized with the given elements.
//
// Deprecated: Use [github.com/goplus/spx/v4/pkg/scratchcompat.NewList] instead.
func NewList(l ...any) List {
	return scratch.NewList(l...)
}

// LimitPrecision rounds a coordinate when it is strictly within 1e-9 of an
// integer; NaN and infinities pass through. Apply it to a sprite's own x/y
// reporters, leaving property queries unchanged.
//
// Deprecated: Use [github.com/goplus/spx/v4/pkg/scratchcompat.LimitPrecision] instead.
func LimitPrecision(coordinate float64) float64 {
	return scratch.LimitPrecision(coordinate)
}

// Div divides a by b, converting a NaN result to zero and preserving infinities.
//
// Deprecated: Use [github.com/goplus/spx/v4/pkg/scratchcompat.Div] instead.
func Div(a, b float64) float64 {
	return scratch.Div(a, b)
}

// FloorMod computes the remainder, adding the divisor when remainder/divisor
// is negative. A zero divisor produces NaN.
//
// Deprecated: Use [github.com/goplus/spx/v4/pkg/scratchcompat.FloorMod] instead.
func FloorMod(dividend, divisor float64) float64 {
	return scratch.FloorMod(dividend, divisor)
}

// Contains reports whether substr occurs in s, ignoring case.
//
// Deprecated: Use [github.com/goplus/spx/v4/pkg/scratchcompat.Contains] instead.
func Contains(s, substr string) bool {
	return scratch.Contains(s, substr)
}

// Compare compares numbers when both operands are numeric, otherwise strings
// ignoring case. It returns -1, 0, or 1 without Equal's numeric tolerance.
//
// Deprecated: Use [github.com/goplus/spx/v4/pkg/scratchcompat.Compare] instead.
func Compare(v1, v2 any) int {
	return scratch.Compare(v1, v2)
}

// Equal compares like Compare, allowing up to two ULPs of numeric rounding error.
// Numeric comparisons involving NaN return false.
//
// Deprecated: Use [github.com/goplus/spx/v4/pkg/scratchcompat.Equal] instead.
func Equal(v1, v2 any) bool {
	return scratch.Equal(v1, v2)
}

// CharAt returns the rune at the 0-based index in s, or "" if out of range.
//
// Deprecated: Use [github.com/goplus/spx/v4/pkg/scratchcompat.CharAt] with a 1-based index instead.
func CharAt(s string, idx int) string {
	return scratch.CharAt(s, idx)
}

// PenColorParamFromString converts a string to its corresponding PenColorParam.
// It accepts "color", "saturation", "brightness", and "transparency" (case-insensitive).
// Returns PenNone if the string is not recognized.
//
// Deprecated: Use [github.com/goplus/spx/v4/pkg/scratchcompat.PenColorParamFromString] instead.
func PenColorParamFromString(s string) PenColorParam {
	return PenColorParam(scratch.PenColorParamFromString(s))
}
