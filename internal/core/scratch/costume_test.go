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

import (
	"math"
	"testing"
)

func TestWrapCostumeNumberIntegerLimits(t *testing.T) {
	for _, tt := range []struct {
		number, count, want int
	}{
		{math.MinInt, 3, 0},
		{math.MaxInt, 3, 0},
		{math.MaxInt - 1, math.MaxInt, math.MaxInt - 2},
	} {
		if got := WrapCostumeNumber(tt.number, tt.count); got != tt.want {
			t.Errorf("WrapCostumeNumber(%d, %d) = %d, want %d", tt.number, tt.count, got, tt.want)
		}
	}
}
