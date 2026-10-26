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

package tools

import (
	"fmt"
	"math"
	"testing"
)

func TestGetFloat(t *testing.T) {
	for _, value := range []any{float32(12), float64(12), int(12), int8(12), int16(12), int32(12), int64(12), uint(12), uint8(12), uint16(12), uint32(12), uint64(12), "12"} {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) {
			if got, ok := GetFloat(value); !ok || got != 12 {
				t.Errorf("GetFloat = %v, %v", got, ok)
			}
		})
	}
	for _, value := range []any{nil, true, struct{}{}, "", "invalid", "1e9999"} {
		if got, ok := GetFloat(value); ok || got != 0 {
			t.Errorf("GetFloat(%v) = %v, %v", value, got, ok)
		}
	}
	for _, value := range []any{float32(-12.75), float64(-12.75), "-12.75"} {
		if got, ok := GetFloat(value); !ok || got != -12.75 {
			t.Errorf("GetFloat(%v) = %v, %v", value, got, ok)
		}
	}
	if got, ok := GetFloat("NaN"); !ok || !math.IsNaN(got) {
		t.Errorf("NaN = %v, %v", got, ok)
	}
	if got, ok := GetFloat("+Inf"); !ok || !math.IsInf(got, 1) {
		t.Errorf("+Inf = %v, %v", got, ok)
	}
}
