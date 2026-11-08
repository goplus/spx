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

package ffi

/*
#include "gdextension_spx_interface.h"
*/
import "C"

import (
	"unsafe"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func ToGdBool(val bool) GdBool {
	if val {
		return GdBool(1)
	}
	return GdBool(0)
}

func ToBool(val GdBool) bool {
	return val != 0
}

func ToGdInt(val int64) GdInt {
	return GdInt(val)
}

func ToInt64(val GdInt) int64 {
	return int64(val)
}

func ToGdObj(val engine.Object) GdObj {
	return GdObj(val)
}

func ToObject(val GdObj) engine.Object {
	return engine.Object(val)
}

func ToGdFloat(val float64) GdFloat {
	return GdFloat(val)
}

func ToFloat64(val GdFloat) float64 {
	return float64(val)
}

// ToString copies an owned ABI return and releases it through the runtime allocator.
func ToString(val GdString) string {
	if val == nil {
		return ""
	}
	str := copyString(val)
	freeReturnedString(val)
	return str
}

// copyString copies a borrowed string without releasing the caller's memory.
func copyString(val GdString) string {
	if val == nil {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(val)))
}

func ToGdVec2(val mathf.Vec2) GdVec2 {
	return GdVec2{C.GdFloat(val.X), C.GdFloat(val.Y)}
}

func ToVec2(val GdVec2) mathf.Vec2 {
	return mathf.NewVec2(float64(val.X), float64(val.Y))
}

func ToGdVec4(val mathf.Vec4) GdVec4 {
	return GdVec4{C.GdFloat(val.X), C.GdFloat(val.Y), C.GdFloat(val.Z), C.GdFloat(val.W)}
}

func ToVec4(val GdVec4) mathf.Vec4 {
	return mathf.NewVec4(float64(val.X), float64(val.Y), float64(val.Z), float64(val.W))
}

func ToGdColor(val mathf.Color) GdColor {
	return GdColor{C.float(val.R), C.float(val.G), C.float(val.B), C.float(val.A)}
}

func ToColor(val GdColor) mathf.Color {
	return mathf.NewColor(float64(val.R), float64(val.G), float64(val.B), float64(val.A))
}

func ToGdRect2(val mathf.Rect2) GdRect2 {
	return GdRect2{
		Position: C.GdVec2(ToGdVec2(val.Position)),
		Size:     C.GdVec2(ToGdVec2(val.Size)),
	}
}

func ToRect2(val GdRect2) mathf.Rect2 {
	return mathf.Rect2{
		Position: ToVec2(GdVec2(val.Position)),
		Size:     ToVec2(GdVec2(val.Size)),
	}
}
