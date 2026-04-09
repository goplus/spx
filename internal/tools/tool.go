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
	"strconv"

	"github.com/goplus/spbase/mathf"
)

func GetVec2(unk any) (mathf.Vec2, bool) {
	if vec, ok := unk.(*mathf.Vec2); ok {
		return *vec, true
	}
	return mathf.Vec2{}, false
}

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

func GetInt(unk any) (int, bool) {
	switch i := unk.(type) {
	case float64:
		return int(i), true
	case float32:
		return int(i), true
	case int64:
		return int(i), true
	case int32:
		return int(i), true
	case int16:
		return int(i), true
	case int8:
		return int(i), true
	case uint64:
		return int(i), true
	case uint32:
		return int(i), true
	case uint16:
		return int(i), true
	case uint8:
		return int(i), true
	case int:
		return int(i), true
	case uint:
		return int(i), true
	case string:
		f, err := strconv.ParseFloat(i, 64)
		if err != nil {
			return 0, false
		}
		return int(f), true
	default:
		return 0, false
	}
}
