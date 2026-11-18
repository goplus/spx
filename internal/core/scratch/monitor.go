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
	"math/big"
	"reflect"
	"strconv"
)

const monitorLargeValue = 1e21

// FormatMonitorValue formats a scalar using Scratch's monitor display rules.
// Integer values and strings retain their source representation. Floating-point
// values are rounded to six decimal places before they are displayed.
func FormatMonitorValue(value any) string {
	value = UnwrapValue(value)
	if value == nil {
		return ToString(value)
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Float32, reflect.Float64:
		return formatMonitorFloat(rv.Float())
	default:
		return ToString(value)
	}
}

func formatMonitorFloat(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return ToString(value)
	}
	if math.Abs(value) >= monitorLargeValue {
		return strconv.FormatFloat(value, 'e', -1, 64)
	}

	roundedText := new(big.Rat).SetFloat64(value).FloatString(6)
	rounded, err := strconv.ParseFloat(roundedText, 64)
	if err != nil || rounded == 0 {
		return "0"
	}
	if math.Abs(rounded) >= monitorLargeValue {
		return strconv.FormatFloat(rounded, 'e', -1, 64)
	}
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}
