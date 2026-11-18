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

func TestFormatMonitorValue(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		want  string
	}{
		{"floating point noise", 4.5600000000000005, "4.56"},
		{"rounds positive tie away from zero", 0.0078125, "0.007813"},
		{"rounds negative tie away from zero", -0.0078125, "-0.007813"},
		{"small value rounds to zero", 0.0000005, "0"},
		{"large integer stays exact", int64(9007199254740993), "9007199254740993"},
		{"numeric string stays textual", "00123", "00123"},
		{"formatted string stays textual", "1.2300", "1.2300"},
		{"float32 is rounded", float32(1.2345678), "1.234568"},
		{"negative zero is normalized", math.Copysign(0, -1), "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := FormatMonitorValue(test.value); got != test.want {
				t.Fatalf("FormatMonitorValue(%#v) = %q, want %q", test.value, got, test.want)
			}
		})
	}

	if got := FormatMonitorValue(NewValue("00123")); got != "00123" {
		t.Fatalf("wrapped string = %q, want 00123", got)
	}
}
