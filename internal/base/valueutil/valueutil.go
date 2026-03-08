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

package valueutil

// ClampFloat64 constrains a float64 value to the specified range.
func ClampFloat64(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// OrDefault returns defaultValue when pval is nil.
func OrDefault[T any](pval *T, defaultValue T) T {
	if pval == nil {
		return defaultValue
	}
	return *pval
}

// SetDefaultIfZero sets the pointed value when it is currently the zero value.
func SetDefaultIfZero[T comparable](pval *T, defaultValue T) {
	var zero T
	if *pval == zero {
		*pval = defaultValue
	}
}
