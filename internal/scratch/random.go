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
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

// Random implements Scratch's random operator for scalar inputs. Unit must
// supply a number in [0, 1); equal bounds do not call it. Nil represents null;
// unsupported Go values convert to zero without being classified as integers.
//
// https://github.com/scratchfoundation/scratch-editor/blob/50081d95d53db1186af38a05491efb98c0f36c96/packages/scratch-vm/src/blocks/scratch3_operators.js
func Random(from, to any, unit func() float64) float64 {
	low, fromInt := randomNumber(from)
	high, toInt := randomNumber(to)
	if low > high {
		low, high = high, low
	}
	if low == high {
		return low
	}
	if fromInt && toInt {
		width := float64(high+1) - low
		product := float64(unit() * width)
		return low + math.Floor(product)
	}
	// The conversion forces multiplication to round before addition, as in JS.
	product := float64(unit() * (high - low))
	return product + low
}

// randomNumber keeps Cast.toNumber and Cast.isInt separate: strings containing
// a decimal point select continuous draws even when their numeric value is whole.
// https://github.com/scratchfoundation/scratch-editor/blob/50081d95d53db1186af38a05491efb98c0f36c96/packages/scratch-vm/src/util/cast.js
func randomNumber(value any) (float64, bool) {
	switch value := value.(type) {
	case nil:
		return 0, false
	case string:
		return randomStringNumber(value), !strings.ContainsRune(value, '.')
	}
	number, ok := ToFloat64(value)
	if !ok {
		return 0, false
	}
	if math.IsNaN(number) {
		return 0, true
	}
	// JS parseInt first stringifies numbers; magnitudes >= 1e21 use exponents.
	return number, math.Trunc(number) == number && math.Abs(number) < 1e21
}

func randomStringNumber(value string) float64 {
	value = strings.TrimFunc(value, func(r rune) bool {
		return unicode.Is(unicode.Zs, r) || r >= '\t' && r <= '\r' ||
			r == '\u2028' || r == '\u2029' || r == '\ufeff'
	})
	switch value {
	case "":
		return 0
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	if len(value) > 2 && value[0] == '0' {
		base := 0
		switch value[1] {
		case 'x', 'X':
			base = 16
		case 'b', 'B':
			base = 2
		case 'o', 'O':
			base = 8
		}
		if base != 0 {
			if value[2] == '+' || value[2] == '-' {
				return 0
			}
			integer, ok := new(big.Int).SetString(value[2:], base)
			if !ok {
				return 0
			}
			number, _ := integer.Float64()
			return number
		}
	}
	// ParseFloat supplies decimal rounding, but also accepts Go-only syntax.
	// Restrict its alphabet to JS decimal syntax before checking the grammar.
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.' || c == 'e' || c == 'E') {
			return 0
		}
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0
	}
	return number
}
