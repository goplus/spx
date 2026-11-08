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

type (
	Uint8T  C.uint8_t
	Uint32T C.uint32_t
	Uint64T C.uint64_t
	Char    C.char
	WcharT  C.wchar_t
	Char16T C.char16_t
	Char32T C.char32_t
)

type (
	GdBool   C.GdBool
	GdInt    C.GdInt
	GdObj    C.GdObj
	GdFloat  C.GdFloat
	GdString C.GdString
	GdVec2   C.GdVec2
	GdVec3   C.GdVec3
	GdVec4   C.GdVec4
	GdColor  C.GdColor
	GdRect2  C.GdRect2
	GdArray  C.GdArray
)
