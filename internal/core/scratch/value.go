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

// toInt converts values to int, accepting compound signs and preserving integer
// precision. Nil becomes zero; failed conversions return zero and false.
func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case nil:
		return 0, true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case string:
		return parseIntString(x)
	}
	return numeric.Int(v)
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
		return numeric.ParseFloat64(normalizeSigns(x))
	}
	return numeric.Float64(v)
}

func parseIntString(s string) (int, bool) {
	normalized := normalizeSigns(s)
	if i, err := strconv.Atoi(normalized); err == nil {
		return i, true
	}

	f, ok := numeric.ParseFloat64(normalized)
	if !ok {
		return 0, false
	}
	return numeric.Int(f)
}

func normalizeSigns(s string) string {
	s = strings.TrimSpace(s)
	negative := false
	i := 0
	for i < len(s) && (s[i] == '+' || s[i] == '-') {
		if s[i] == '-' {
			negative = !negative
		}
		i++
	}
	rest := s[i:]
	if negative && rest != "" {
		return "-" + rest
	}
	return rest
}
