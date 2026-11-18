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
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

const (
	monitorDecimalScale = int64(1_000_000)
	monitorLargeValue   = 1e21
)

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

	units := roundMonitorUnits(value)
	if units.Sign() == 0 {
		return "0"
	}
	return formatMonitorUnits(units)
}

func roundMonitorUnits(value float64) *big.Int {
	ratio := new(big.Rat).SetFloat64(value)
	ratio.Mul(ratio, big.NewRat(monitorDecimalScale, 1))

	quotient := new(big.Int)
	remainder := new(big.Int)
	quotient.QuoRem(ratio.Num(), ratio.Denom(), remainder)

	if remainder.Sign() != 0 {
		twiceRemainder := new(big.Int).Lsh(new(big.Int).Abs(remainder), 1)
		if twiceRemainder.Cmp(ratio.Denom()) >= 0 {
			if ratio.Num().Sign() < 0 {
				quotient.Sub(quotient, big.NewInt(1))
			} else {
				quotient.Add(quotient, big.NewInt(1))
			}
		}
	}
	return quotient
}

func formatMonitorUnits(units *big.Int) string {
	negative := units.Sign() < 0
	absolute := new(big.Int).Abs(units)
	scale := big.NewInt(monitorDecimalScale)
	integer := new(big.Int)
	fraction := new(big.Int)
	integer.QuoRem(absolute, scale, fraction)

	text := integer.String()
	if fraction.Sign() != 0 {
		fractionText := fmt.Sprintf("%06d", fraction.Int64())
		text += "." + strings.TrimRight(fractionText, "0")
	}
	if negative {
		return "-" + text
	}
	return text
}
