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

import "strconv"

func GetFloat(unk any) (float64, bool) {
	switch i := unk.(type) {
	case float32:
		return float64(i), true
	case float64:
		return float64(i), true
	case int64:
		return float64(i), true
	case int32:
		return float64(i), true
	case int16:
		return float64(i), true
	case int8:
		return float64(i), true
	case uint64:
		return float64(i), true
	case uint32:
		return float64(i), true
	case uint16:
		return float64(i), true
	case uint8:
		return float64(i), true
	case int:
		return float64(i), true
	case uint:
		return float64(i), true
	case string:
		f, err := strconv.ParseFloat(i, 64)
		if err != nil {
			return 0, false
		}
		return float64(f), true
	default:
		return 0, false
	}
}
