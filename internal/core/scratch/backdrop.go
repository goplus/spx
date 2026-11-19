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

import "math"

// ParseBackdropIndex parses a Scratch backdrop selector after name lookup has
// failed. Numeric selectors are rounded like JavaScript Math.round, converted
// from 1-based to zero-based, and wrapped over the available backdrops.
// Invalid and non-finite values return -1.
func ParseBackdropIndex(backdrop string, count int) int {
	value, ok := toFloat64(backdrop)
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return -1
	}

	// JavaScript Math.round rounds half values toward positive infinity.
	index := math.Floor(value+0.5) - 1
	if count == 0 {
		if index < float64(math.MinInt) || index >= -float64(math.MinInt) {
			return -1
		}
		return int(index)
	}
	return int(FloorMod(index, float64(count)))
}
