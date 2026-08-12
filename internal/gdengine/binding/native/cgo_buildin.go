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

import (
	"unsafe"
)

/*
#include "gdextension_spx_interface.h"

void cgo_callfn_GDExtensionPtrConstructor(const GDExtensionPtrConstructor fn, GDExtensionUninitializedTypePtr p_base, const GDExtensionConstTypePtr *  p_args) {
    fn(p_base, p_args);
}
void cgo_callfn_GDExtensionPtrDestructor(const GDExtensionPtrDestructor fn, GDExtensionTypePtr p_base) {
    fn(p_base);
}
void cgo_callfn_GDExtensionSpxGlobalRegisterCallbacks(const GDExtensionSpxGlobalRegisterCallbacks fn, GDExtensionSpxCallbackInfoPtr callback_ptr) {
	fn(callback_ptr);
}
void cgo_callfn_GDExtensionInterfaceStringNewWithLatin1Chars(const GDExtensionInterfaceStringNewWithLatin1Chars fn, GDExtensionUninitializedStringPtr r_dest, const char *  p_contents) {
	 fn(r_dest, p_contents);
}
void cgo_callfn_GDExtensionInterfaceStringNewWithUtf8Chars(const GDExtensionInterfaceStringNewWithUtf8Chars fn, GDExtensionUninitializedStringPtr r_dest, const char *  p_contents) {
	 fn(r_dest, p_contents);
}
GdInt cgo_callfn_GDExtensionInterfaceStringToLatin1Chars(const GDExtensionInterfaceStringToLatin1Chars fn, GDExtensionConstStringPtr p_self, char *  r_text, GdInt p_max_write_length) {
	return fn(p_self, r_text, p_max_write_length);
}
GdInt cgo_callfn_GDExtensionInterfaceStringToUtf8Chars(const GDExtensionInterfaceStringToUtf8Chars fn, GDExtensionConstStringPtr p_self, char *  r_text, GdInt p_max_write_length) {
	return fn(p_self, r_text, p_max_write_length);
}
GDExtensionPtrConstructor cgo_callfn_GDExtensionInterfaceVariantGetPtrConstructor(const GDExtensionInterfaceVariantGetPtrConstructor fn, GDExtensionVariantType p_type, int32_t p_constructor) {
	return fn(p_type, p_constructor);
}
GDExtensionPtrDestructor cgo_callfn_GDExtensionInterfaceVariantGetPtrDestructor(const GDExtensionInterfaceVariantGetPtrDestructor fn, GDExtensionVariantType p_type) {
	return fn(p_type);
}
*/
import "C"

type GDExtensionInterfaceStringNewWithLatin1Chars C.GDExtensionInterfaceStringNewWithLatin1Chars
type GDExtensionInterfaceStringNewWithUtf8Chars C.GDExtensionInterfaceStringNewWithUtf8Chars
type GDExtensionInterfaceStringToLatin1Chars C.GDExtensionInterfaceStringToLatin1Chars
type GDExtensionInterfaceStringToUtf8Chars C.GDExtensionInterfaceStringToUtf8Chars
type GDExtensionInterfaceVariantGetPtrConstructor C.GDExtensionInterfaceVariantGetPtrConstructor
type GDExtensionInterfaceVariantGetPtrDestructor C.GDExtensionInterfaceVariantGetPtrDestructor

var (
	builtinAPI GDExtensionBuiltinInterface
)

type GDExtensionBuiltinInterface struct {
	SpxGlobalRegisterCallbacks GDExtensionSpxGlobalRegisterCallbacks
	StringNewWithLatin1Chars   GDExtensionInterfaceStringNewWithLatin1Chars
	StringNewWithUtf8Chars     GDExtensionInterfaceStringNewWithUtf8Chars
	StringToLatin1Chars        GDExtensionInterfaceStringToLatin1Chars
	StringToUtf8Chars          GDExtensionInterfaceStringToUtf8Chars
	VariantGetPtrConstructor   GDExtensionInterfaceVariantGetPtrConstructor
	VariantGetPtrDestructor    GDExtensionInterfaceVariantGetPtrDestructor
}

func (x *GDExtensionBuiltinInterface) resolveAPIFunctions() {
	x.SpxGlobalRegisterCallbacks = (GDExtensionSpxGlobalRegisterCallbacks)(resolveCFunc("spx_global_register_callbacks"))
	x.StringNewWithLatin1Chars = (GDExtensionInterfaceStringNewWithLatin1Chars)(resolveCFunc("string_new_with_latin1_chars"))
	x.StringNewWithUtf8Chars = (GDExtensionInterfaceStringNewWithUtf8Chars)(resolveCFunc("string_new_with_utf8_chars"))
	x.StringToLatin1Chars = (GDExtensionInterfaceStringToLatin1Chars)(resolveCFunc("string_to_latin1_chars"))
	x.StringToUtf8Chars = (GDExtensionInterfaceStringToUtf8Chars)(resolveCFunc("string_to_utf8_chars"))
	x.VariantGetPtrConstructor = (GDExtensionInterfaceVariantGetPtrConstructor)(resolveCFunc("variant_get_ptr_constructor"))
	x.VariantGetPtrDestructor = (GDExtensionInterfaceVariantGetPtrDestructor)(resolveCFunc("variant_get_ptr_destructor"))
}

type stringMethodBindings struct {
	constructor GDExtensionPtrConstructor
	destructor  GDExtensionPtrDestructor
}

var (
	globalStringMethodBindings stringMethodBindings
	nullptr                    = unsafe.Pointer(nil)
)

func stringInitConstructorBindings() {
	globalStringMethodBindings.constructor = CallVariantGetPtrConstructor(GDEXTENSION_VARIANT_TYPE_STRING, 0)
	globalStringMethodBindings.destructor = CallVariantGetPtrDestructor(GDEXTENSION_VARIANT_TYPE_STRING)
}

func CallBuiltinConstructor(constructor GDExtensionPtrConstructor, base GDExtensionUninitializedTypePtr, args ...GDExtensionConstTypePtr) {
	a := (GDExtensionPtrConstructor)(constructor)
	b := (GDExtensionUninitializedTypePtr)(base)
	if a == nil {
		panic("constructor is null")
	}
	c := (*GDExtensionConstTypePtr)(unsafe.SliceData(args))
	CallPtrConstructor(a, b, c)
}

func CallPtrConstructor(
	fn GDExtensionPtrConstructor,
	p_base GDExtensionUninitializedTypePtr,
	p_args *GDExtensionConstTypePtr,
) {
	arg0 := (C.GDExtensionPtrConstructor)(fn)
	arg1 := (C.GDExtensionUninitializedTypePtr)(p_base)
	arg2 := (*C.GDExtensionConstTypePtr)(p_args)
	C.cgo_callfn_GDExtensionPtrConstructor(arg0, arg1, arg2)
}

func CallPtrDestructor(
	fn GDExtensionPtrDestructor,
	p_base GDExtensionTypePtr,
) {
	arg0 := (C.GDExtensionPtrDestructor)(fn)
	arg1 := (C.GDExtensionTypePtr)(p_base)
	C.cgo_callfn_GDExtensionPtrDestructor(arg0, arg1)
}

func CallStringNewWithLatin1Chars(
	r_dest GDExtensionUninitializedStringPtr,
	p_contents string,
) {
	arg0 := (C.GDExtensionInterfaceStringNewWithLatin1Chars)(builtinAPI.StringNewWithLatin1Chars)
	arg1 := (C.GDExtensionUninitializedStringPtr)(r_dest)
	arg2 := C.CString(p_contents)
	C.cgo_callfn_GDExtensionInterfaceStringNewWithLatin1Chars(arg0, arg1, arg2)
	C.free(unsafe.Pointer(arg2))

}
func CallStringNewWithUtf8Chars(
	r_dest GDExtensionUninitializedStringPtr,
	p_contents string,
) {
	arg0 := (C.GDExtensionInterfaceStringNewWithUtf8Chars)(builtinAPI.StringNewWithUtf8Chars)
	arg1 := (C.GDExtensionUninitializedStringPtr)(r_dest)
	arg2 := C.CString(p_contents)
	C.cgo_callfn_GDExtensionInterfaceStringNewWithUtf8Chars(arg0, arg1, arg2)
	C.free(unsafe.Pointer(arg2))
}
func CallStringToLatin1Chars(
	p_self GDExtensionConstStringPtr,
	r_text *Char,
	p_max_write_length GdInt,
) GdInt {
	arg0 := (C.GDExtensionInterfaceStringToLatin1Chars)(builtinAPI.StringToLatin1Chars)
	arg1 := (C.GDExtensionConstStringPtr)(p_self)
	arg2 := (*C.char)(r_text)
	arg3 := (C.GdInt)(p_max_write_length)
	ret := C.cgo_callfn_GDExtensionInterfaceStringToLatin1Chars(arg0, arg1, arg2, arg3)
	return (GdInt)(ret)
}
func CallStringToUtf8Chars(
	p_self GDExtensionConstStringPtr,
	r_text *Char,
	p_max_write_length GdInt,
) GdInt {
	arg0 := (C.GDExtensionInterfaceStringToUtf8Chars)(builtinAPI.StringToUtf8Chars)
	arg1 := (C.GDExtensionConstStringPtr)(p_self)
	arg2 := (*C.char)(r_text)
	arg3 := (C.GdInt)(p_max_write_length)
	ret := C.cgo_callfn_GDExtensionInterfaceStringToUtf8Chars(arg0, arg1, arg2, arg3)
	return (GdInt)(ret)
}
func CallVariantGetPtrConstructor(
	p_type GDExtensionVariantType,
	p_constructor int32,
) GDExtensionPtrConstructor {
	arg0 := (C.GDExtensionInterfaceVariantGetPtrConstructor)(builtinAPI.VariantGetPtrConstructor)
	arg1 := (C.GDExtensionVariantType)(p_type)
	arg2 := (C.int32_t)(p_constructor)
	ret := C.cgo_callfn_GDExtensionInterfaceVariantGetPtrConstructor(arg0, arg1, arg2)
	return (GDExtensionPtrConstructor)(ret)
}
func CallVariantGetPtrDestructor(
	p_type GDExtensionVariantType,
) GDExtensionPtrDestructor {
	arg0 := (C.GDExtensionInterfaceVariantGetPtrDestructor)(builtinAPI.VariantGetPtrDestructor)
	arg1 := (C.GDExtensionVariantType)(p_type)
	ret := C.cgo_callfn_GDExtensionInterfaceVariantGetPtrDestructor(arg0, arg1)
	return (GDExtensionPtrDestructor)(ret)
}

func CallGlobalRegisterCallbacks(
	callback_ptr GDExtensionSpxCallbackInfoPtr,
) {
	arg0 := (C.GDExtensionSpxGlobalRegisterCallbacks)(builtinAPI.SpxGlobalRegisterCallbacks)
	arg1 := (C.GDExtensionSpxCallbackInfoPtr)(callback_ptr)

	C.cgo_callfn_GDExtensionSpxGlobalRegisterCallbacks(arg0, arg1)
}
