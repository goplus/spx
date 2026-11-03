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

package numeric

import (
	"math"
	"testing"
)

type definedInt int

func TestFloat64ConvertsBuiltinNumbersOnly(t *testing.T) {
	for _, test := range []struct {
		value any
		want  float64
	}{
		{int64(1<<53 + 1), 1 << 53},
		{uint64(7), 7},
		{float32(0.25), 0.25},
		{math.Inf(1), math.Inf(1)},
	} {
		if got, ok := Float64(test.value); !ok || got != test.want {
			t.Errorf("Float64(%T) = %v, %v; want %v, true", test.value, got, ok, test.want)
		}
	}
	for _, value := range []any{nil, true, "12", definedInt(12)} {
		if got, ok := Float64(value); ok || got != 0 {
			t.Errorf("Float64(%T) = %v, %v; want 0, false", value, got, ok)
		}
	}
}
