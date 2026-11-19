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

import "testing"

func TestParseBackdropIndex(t *testing.T) {
	for _, test := range []struct {
		name     string
		backdrop string
		count    int
		want     int
	}{
		{name: "zero wraps", backdrop: "0", count: 2, want: 1},
		{name: "negative wraps", backdrop: "-2", count: 2, want: 1},
		{name: "fraction rounds", backdrop: "1.5", count: 2, want: 1},
		{name: "negative fraction rounds toward positive infinity", backdrop: "-1.5", count: 2, want: 0},
		{name: "large fraction wraps", backdrop: "2.5", count: 2, want: 0},
		{name: "invalid", backdrop: "missing", count: 2, want: -1},
		{name: "non-finite", backdrop: "NaN", count: 2, want: -1},
		{name: "infinite", backdrop: "Inf", count: 2, want: -1},
		{name: "empty backdrop list", backdrop: "3", count: 0, want: 2},
		{name: "empty backdrop list overflow", backdrop: "1e100", count: 0, want: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseBackdropIndex(test.backdrop, test.count); got != test.want {
				t.Fatalf("ParseBackdropIndex(%q, %d) = %d, want %d", test.backdrop, test.count, got, test.want)
			}
		})
	}
}
