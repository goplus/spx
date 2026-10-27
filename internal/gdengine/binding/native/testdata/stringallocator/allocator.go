//go:build cgo

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

// Package stringallocator supplies bounded C fixtures for native ABI tests.
// Go does not support importing C directly from a _test.go file.
package stringallocator

/*
#include "../../gdextension_spx_interface.h"
#include <string.h>

static char *owned_string;
static int release_count;

static void release_string(GdString value) {
	release_count++;
	if (value == owned_string && owned_string != NULL) {
		// Overwrite before release to expose copies that still alias native storage.
		volatile char *bytes = owned_string;
		for (size_t i = 0; bytes[i] != '\0'; i++) {
			bytes[i] = '?';
		}
		free(owned_string);
		owned_string = NULL;
	}
}

static void *track_string(char *value) {
	owned_string = value;
	return value;
}

static GDExtensionSpxGlobalFreeString free_callback(void) {
	return release_string;
}

static int count_releases(void) {
	return release_count;
}

static int has_outstanding_allocation(void) {
	return owned_string != NULL;
}

static void reset_allocator(void) {
	free(owned_string);
	owned_string = NULL;
	release_count = 0;
}

// These callbacks exercise the ABI only; they do not implement Godot objects.
static void default_constructor(GDExtensionUninitializedTypePtr base, const GDExtensionConstTypePtr *args) {
	*(int64_t *)base = args == NULL ? 71 : -71;
}

static void argument_constructor(GDExtensionUninitializedTypePtr base, const GDExtensionConstTypePtr *args) {
	*(int64_t *)base = 10 * *(const int64_t *)args[0] + *(const int64_t *)args[1];
}

static void destructor(GDExtensionTypePtr base) {
	*(int64_t *)base = -99;
}

static GDExtensionPtrConstructor get_constructor(GDExtensionVariantType type, int32_t index) {
	if (type != GDEXTENSION_VARIANT_TYPE_STRING) return NULL;
	if (index == 0) return default_constructor;
	if (index == 1) return argument_constructor;
	return NULL;
}

static GDExtensionPtrDestructor get_destructor(GDExtensionVariantType type) {
	return type == GDEXTENSION_VARIANT_TYPE_STRING ? destructor : NULL;
}

// A tagged byte buffer makes swapped Latin-1/UTF-8 bindings observable.
// Tests supply enough destination storage for their small input strings.
static void new_latin1(GDExtensionUninitializedStringPtr dest, const char *contents) {
	((char *)dest)[0] = 'L';
	strcpy((char *)dest + 1, contents);
}

static void new_utf8(GDExtensionUninitializedStringPtr dest, const char *contents) {
	((char *)dest)[0] = 'U';
	strcpy((char *)dest + 1, contents);
}

static GDExtensionInt read_string(GDExtensionConstStringPtr self, char *text, GDExtensionInt max_length, char tag) {
	if (((const char *)self)[0] != tag) return -1;
	const char *contents = (const char *)self + 1;
	GDExtensionInt length = strlen(contents);
	if (text != NULL && max_length > 0) {
		memcpy(text, contents, max_length < length ? max_length : length);
	}
	return length;
}

static GDExtensionInt to_latin1(GDExtensionConstStringPtr self, char *text, GDExtensionInt max_length) {
	return read_string(self, text, max_length, 'L');
}

static GDExtensionInt to_utf8(GDExtensionConstStringPtr self, char *text, GDExtensionInt max_length) {
	return read_string(self, text, max_length, 'U');
}

static SpxCallbackInfo registered_callbacks;
static int registration_count;
static int registration_nonnull;

static void register_callbacks(GDExtensionSpxCallbackInfoPtr callbacks) {
	registration_count++;
	registration_nonnull = callbacks != NULL;
	// The generated registration helper supplies a stack-local table: copy it.
	if (callbacks != NULL) registered_callbacks = *(const SpxCallbackInfo *)callbacks;
	else memset(&registered_callbacks, 0, sizeof(registered_callbacks));
}

static void reset_callbacks(void) {
	memset(&registered_callbacks, 0, sizeof(registered_callbacks));
	registration_count = 0;
	registration_nonnull = 0;
}

static int count_registrations(void) { return registration_count; }
static int has_registration(void) { return registration_nonnull; }

static int invoke_callbacks(GdObj object, GdInt other, GdFloat delta, GdBool flag, GdString text) {
	int calls = 0;
#define INVOKE(field, ...) if (registered_callbacks.field != NULL) { registered_callbacks.field(__VA_ARGS__); calls++; }
	INVOKE(func_on_engine_start);
	INVOKE(func_on_engine_update, delta);
	INVOKE(func_on_engine_pause, flag);
	INVOKE(func_on_sprite_ready, object);
	INVOKE(func_on_key_pressed, other);
	INVOKE(func_on_action_pressed, text);
	INVOKE(func_on_axis_changed, text, delta);
	INVOKE(func_on_collision_enter, object, other);
	INVOKE(func_on_scene_sprite_instantiated, object, text);
	INVOKE(func_on_ui_toggle, object, flag);
	INVOKE(func_on_ui_text_changed, object, text);
#undef INVOKE
	return calls;
}

static GDExtensionInterfaceFunctionPtr resolve_builtin(const char *name) {
#define RESOLVE(symbol, fn) if (strcmp(name, symbol) == 0) return (GDExtensionInterfaceFunctionPtr)fn
	RESOLVE("spx_global_register_callbacks", register_callbacks);
	RESOLVE("spx_global_free_string", release_string);
	RESOLVE("string_new_with_latin1_chars", new_latin1);
	RESOLVE("string_new_with_utf8_chars", new_utf8);
	RESOLVE("string_to_latin1_chars", to_latin1);
	RESOLVE("string_to_utf8_chars", to_utf8);
	RESOLVE("variant_get_ptr_constructor", get_constructor);
	RESOLVE("variant_get_ptr_destructor", get_destructor);
#undef RESOLVE
	return NULL;
}

static GDExtensionInterfaceGetProcAddress builtin_resolver(void) { return resolve_builtin; }
*/
import "C"

import "unsafe"

func Allocate(value string) unsafe.Pointer {
	return C.track_string(C.CString(value))
}

func FreeCallback() unsafe.Pointer {
	return unsafe.Pointer(C.free_callback())
}

func ReleaseCount() int {
	return int(C.count_releases())
}

func HasOutstandingAllocation() bool {
	return C.has_outstanding_allocation() != 0
}

func Reset() {
	C.reset_allocator()
	C.reset_callbacks()
}

func Resolver() unsafe.Pointer {
	return unsafe.Pointer(C.builtin_resolver())
}

func RegistrationCount() int {
	return int(C.count_registrations())
}

func HasRegistration() bool {
	return C.has_registration() != 0
}

func InvokeCallbacks(object, other int64, delta float64, flag uint8, text unsafe.Pointer) int {
	return int(C.invoke_callbacks(C.GdObj(object), C.GdInt(other), C.GdFloat(delta), C.GdBool(flag), C.GdString(text)))
}
