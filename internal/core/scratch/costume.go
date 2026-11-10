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

import "strconv"

// ParseCostumeIndex parses a 1-based integer string after costume-name lookup
// has failed. Count must be nonnegative. It preserves SPX's zero and negative
// indices, signed remainder, and empty-costume behavior; invalid input returns -1.
func ParseCostumeIndex(costume string, count int) int {
	idx, err := strconv.Atoi(costume)
	if err != nil {
		return -1
	}
	if count == 0 || idx >= 0 && idx <= count {
		return idx - 1
	}
	return (idx - 1) % count
}
