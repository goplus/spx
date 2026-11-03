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

package scratch

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/goplus/spx/v4/internal/base/numeric"
)

// UnwrapValue unwraps one layer of Value. It uses the concrete value type so
// pointers and other types with a Raw method retain their original behavior.
func UnwrapValue(v any) any {
	if v, ok := v.(Value); ok {
		return v.Raw()
	}
	return v
}

// ToString returns "" for nil and fmt.Sprint(v) otherwise.
func ToString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func normalizeSigns(s string) (string, bool) {
	s = strings.TrimSpace(s)
	negative := false
	i := 0
	for i < len(s) && (s[i] == '+' || s[i] == '-') {
		if s[i] == '-' {
			negative = !negative
		}
		i++
	}
	if i == len(s) {
		return "", false
	}
	if negative {
		return "-" + s[i:], true
	}
	return s[i:], true
}

func parseFloatString(s string) (float64, bool) {
	normalized, ok := normalizeSigns(s)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func parseIntString(s string) (int, bool) {
	normalized, ok := normalizeSigns(s)
	if !ok {
		return 0, false
	}
	if i, err := strconv.Atoi(normalized); err == nil {
		return i, true
	}

	f, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return float64ToInt(f)
}

func float64ToInt(f float64) (int, bool) {
	if f > float64(math.MaxInt) || f < float64(math.MinInt) {
		return 0, false
	}
	return int(f), true
}

// toInt converts values to int, accepting compound signs and preserving integer
// precision. Nil becomes zero; failed conversions return zero and false.
func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case nil:
		return 0, true
	case int:
		return x, true
	case int8:
		return int(x), true
	case int16:
		return int(x), true
	case int32:
		return int(x), true
	case int64:
		if x > int64(math.MaxInt) || x < int64(math.MinInt) {
			return 0, false
		}
		return int(x), true
	case uint:
		if x > uint(math.MaxInt) {
			return 0, false
		}
		return int(x), true
	case uint8:
		return int(x), true
	case uint16:
		return int(x), true
	case uint32:
		if uint64(x) > uint64(math.MaxInt) {
			return 0, false
		}
		return int(x), true
	case uint64:
		if x > uint64(math.MaxInt) {
			return 0, false
		}
		return int(x), true
	case float32:
		if x > float32(math.MaxInt) || x < float32(math.MinInt) {
			return 0, false
		}
		return int(x), true
	case float64:
		return float64ToInt(x)
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case string:
		return parseIntString(x)
	}
	return 0, false
}

// toFloat64 converts values to float64, accepting compound signs. Nil becomes
// zero; failed conversions return zero and false.
func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case nil:
		return 0, true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case string:
		return parseFloatString(x)
	}
	return numeric.Float64(v)
}
