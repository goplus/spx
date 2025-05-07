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
void cgo_callfn_GDExtensionSpxStringNewWithLatin1Chars(const GDExtensionSpxStringNewWithLatin1Chars fn, GDExtensionUninitializedStringPtr r_dest, const char *  p_contents) {
	 fn(r_dest, p_contents);
}
void cgo_callfn_GDExtensionSpxStringNewWithUtf8Chars(const GDExtensionSpxStringNewWithUtf8Chars fn, GDExtensionUninitializedStringPtr r_dest, const char *  p_contents) {
	 fn(r_dest, p_contents);
}
void cgo_callfn_GDExtensionSpxStringNewWithLatin1CharsAndLen(const GDExtensionSpxStringNewWithLatin1CharsAndLen fn, GDExtensionUninitializedStringPtr r_dest, const char *  p_contents, GdInt p_size) {
	 fn(r_dest, p_contents, p_size);
}
void cgo_callfn_GDExtensionSpxStringNewWithUtf8CharsAndLen(const GDExtensionSpxStringNewWithUtf8CharsAndLen fn, GDExtensionUninitializedStringPtr r_dest, const char *  p_contents, GdInt p_size) {
	 fn(r_dest, p_contents, p_size);
}
GdInt cgo_callfn_GDExtensionSpxStringToLatin1Chars(const GDExtensionSpxStringToLatin1Chars fn, GDExtensionConstStringPtr p_self, char *  r_text, GdInt p_max_write_length) {
	return fn(p_self, r_text, p_max_write_length);
}
GdInt cgo_callfn_GDExtensionSpxStringToUtf8Chars(const GDExtensionSpxStringToUtf8Chars fn, GDExtensionConstStringPtr p_self, char *  r_text, GdInt p_max_write_length) {
	return fn(p_self, r_text, p_max_write_length);
}
GDExtensionPtrConstructor cgo_callfn_GDExtensionSpxVariantGetPtrConstructor(const GDExtensionSpxVariantGetPtrConstructor fn, GDExtensionVariantType p_type, int32_t p_constructor) {
	return fn(p_type, p_constructor);
}
GDExtensionPtrDestructor cgo_callfn_GDExtensionSpxVariantGetPtrDestructor(const GDExtensionSpxVariantGetPtrDestructor fn, GDExtensionVariantType p_type) {
	return fn(p_type);
}
*/
import "C"

var (
	builtinAPI GDExtensionBuiltinInterface
)

type GDExtensionBuiltinInterface struct {
	SpxGlobalRegisterCallbacks        GDExtensionSpxGlobalRegisterCallbacks
	SpxStringNewWithLatin1Chars       GDExtensionSpxStringNewWithLatin1Chars
	SpxStringNewWithUtf8Chars         GDExtensionSpxStringNewWithUtf8Chars
	SpxStringNewWithLatin1CharsAndLen GDExtensionSpxStringNewWithLatin1CharsAndLen
	SpxStringNewWithUtf8CharsAndLen   GDExtensionSpxStringNewWithUtf8CharsAndLen
	SpxStringToLatin1Chars            GDExtensionSpxStringToLatin1Chars
	SpxStringToUtf8Chars              GDExtensionSpxStringToUtf8Chars
	SpxVariantGetPtrConstructor       GDExtensionSpxVariantGetPtrConstructor
	SpxVariantGetPtrDestructor        GDExtensionSpxVariantGetPtrDestructor
}

func (x *GDExtensionBuiltinInterface) loadProcAddresses() {
	x.SpxGlobalRegisterCallbacks = (GDExtensionSpxGlobalRegisterCallbacks)(dlsymGD("spx_global_register_callbacks"))
	x.SpxStringNewWithLatin1Chars = (GDExtensionSpxStringNewWithLatin1Chars)(dlsymGD("spx_string_new_with_latin1_chars"))
	x.SpxStringNewWithUtf8Chars = (GDExtensionSpxStringNewWithUtf8Chars)(dlsymGD("spx_string_new_with_utf8_chars"))
	x.SpxStringNewWithLatin1CharsAndLen = (GDExtensionSpxStringNewWithLatin1CharsAndLen)(dlsymGD("spx_string_new_with_latin1_chars_and_len"))
	x.SpxStringNewWithUtf8CharsAndLen = (GDExtensionSpxStringNewWithUtf8CharsAndLen)(dlsymGD("spx_string_new_with_utf8_chars_and_len"))
	x.SpxStringToLatin1Chars = (GDExtensionSpxStringToLatin1Chars)(dlsymGD("spx_string_to_latin1_chars"))
	x.SpxStringToUtf8Chars = (GDExtensionSpxStringToUtf8Chars)(dlsymGD("spx_string_to_utf8_chars"))
	x.SpxVariantGetPtrConstructor = (GDExtensionSpxVariantGetPtrConstructor)(dlsymGD("spx_variant_get_ptr_constructor"))
	x.SpxVariantGetPtrDestructor = (GDExtensionSpxVariantGetPtrDestructor)(dlsymGD("spx_variant_get_ptr_destructor"))
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
	arg0 := (C.GDExtensionSpxStringNewWithLatin1Chars)(builtinAPI.SpxStringNewWithLatin1Chars)
	arg1 := (C.GDExtensionUninitializedStringPtr)(r_dest)
	arg2 := C.CString(p_contents)
	C.cgo_callfn_GDExtensionSpxStringNewWithLatin1Chars(arg0, arg1, arg2)
	C.free(unsafe.Pointer(arg2))

}
func CallStringNewWithUtf8Chars(
	r_dest GDExtensionUninitializedStringPtr,
	p_contents string,
) {
	arg0 := (C.GDExtensionSpxStringNewWithUtf8Chars)(builtinAPI.SpxStringNewWithUtf8Chars)
	arg1 := (C.GDExtensionUninitializedStringPtr)(r_dest)
	arg2 := C.CString(p_contents)
	C.cgo_callfn_GDExtensionSpxStringNewWithUtf8Chars(arg0, arg1, arg2)
	C.free(unsafe.Pointer(arg2))
}
func CallStringToLatin1Chars(
	p_self GDExtensionConstStringPtr,
	r_text *Char,
	p_max_write_length GdInt,
) GdInt {
	arg0 := (C.GDExtensionSpxStringToLatin1Chars)(builtinAPI.SpxStringToLatin1Chars)
	arg1 := (C.GDExtensionConstStringPtr)(p_self)
	arg2 := (*C.char)(r_text)
	arg3 := (C.GdInt)(p_max_write_length)
	ret := C.cgo_callfn_GDExtensionSpxStringToLatin1Chars(arg0, arg1, arg2, arg3)
	return (GdInt)(ret)
}
func CallStringToUtf8Chars(
	p_self GDExtensionConstStringPtr,
	r_text *Char,
	p_max_write_length GdInt,
) GdInt {
	arg0 := (C.GDExtensionSpxStringToUtf8Chars)(builtinAPI.SpxStringToUtf8Chars)
	arg1 := (C.GDExtensionConstStringPtr)(p_self)
	arg2 := (*C.char)(r_text)
	arg3 := (C.GdInt)(p_max_write_length)
	ret := C.cgo_callfn_GDExtensionSpxStringToUtf8Chars(arg0, arg1, arg2, arg3)
	return (GdInt)(ret)
}
func CallVariantGetPtrConstructor(
	p_type GDExtensionVariantType,
	p_constructor int32,
) GDExtensionPtrConstructor {
	arg0 := (C.GDExtensionSpxVariantGetPtrConstructor)(builtinAPI.SpxVariantGetPtrConstructor)
	arg1 := (C.GDExtensionVariantType)(p_type)
	arg2 := (C.int32_t)(p_constructor)
	ret := C.cgo_callfn_GDExtensionSpxVariantGetPtrConstructor(arg0, arg1, arg2)
	return (GDExtensionPtrConstructor)(ret)
}
func CallVariantGetPtrDestructor(
	p_type GDExtensionVariantType,
) GDExtensionPtrDestructor {
	arg0 := (C.GDExtensionSpxVariantGetPtrDestructor)(builtinAPI.SpxVariantGetPtrDestructor)
	arg1 := (C.GDExtensionVariantType)(p_type)
	ret := C.cgo_callfn_GDExtensionSpxVariantGetPtrDestructor(arg0, arg1)
	return (GDExtensionPtrDestructor)(ret)
}

func CallGlobalRegisterCallbacks(
	callback_ptr GDExtensionSpxCallbackInfoPtr,
) {
	arg0 := (C.GDExtensionSpxGlobalRegisterCallbacks)(builtinAPI.SpxGlobalRegisterCallbacks)
	arg1 := (C.GDExtensionSpxCallbackInfoPtr)(callback_ptr)

	C.cgo_callfn_GDExtensionSpxGlobalRegisterCallbacks(arg0, arg1)
}
