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

// WrapCostumeNumber converts a 1-based costume number to a zero-based index.
// With no costumes, it returns the unwrapped index.
func WrapCostumeNumber(number, count int) int {
	if count > 0 {
		// Reduce before subtracting one to avoid overflow for math.MinInt.
		number %= count
		if number <= 0 {
			number += count
		}
	}
	return number - 1
}
