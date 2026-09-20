/**************************************************************************/
/*  spx_abi.h                                                             */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/*                        https://godotengine.org                         */
/**************************************************************************/
/* Copyright (c) 2014-present Godot Engine contributors (see AUTHORS.md). */
/* Copyright (c) 2007-2014 Juan Linietsky, Ariel Manzur.                  */
/*                                                                        */
/* Permission is hereby granted, free of charge, to any person obtaining  */
/* a copy of this software and associated documentation files (the        */
/* "Software"), to deal in the Software without restriction, including    */
/* without limitation the rights to use, copy, modify, merge, publish,    */
/* distribute, sublicense, and/or sell copies of the Software, and to     */
/* permit persons to whom the Software is furnished to do so, subject to  */
/* the following conditions:                                              */
/*                                                                        */
/* The above copyright notice and this permission notice shall be         */
/* included in all copies or substantial portions of the Software.        */
/*                                                                        */
/* THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,        */
/* EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF     */
/* MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. */
/* IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY   */
/* CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,   */
/* TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE      */
/* SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.                 */
/**************************************************************************/

#ifndef SPX_ABI_H
#define SPX_ABI_H

#include "gdextension_spx_ext.h"
#include <type_traits>

// Exported cross-language method. C++ static declarations need no manager.
#define SPX_BIND

// Output-only native array: completely filled on success, never read on entry.
// GdBool methods return false before modifying any writable array on failure.
#define SPX_OUT

// ABI allocations belong to their caller, independently of engine lifetime.
namespace SpxAbi {

void *_get_array(GdArray array, int64_t index, int type_size,
		int32_t expected_type);

template <typename T>
constexpr int32_t _array_type_for() {
	using U = std::remove_cv_t<std::remove_reference_t<T>>;
	if constexpr (std::is_same_v<U, GdString> ||
			(std::is_pointer_v<U> &&
					std::is_same_v<std::remove_cv_t<std::remove_pointer_t<U>>,
							char>)) {
		return GD_ARRAY_TYPE_STRING;
	} else if constexpr (std::is_floating_point_v<U>) {
		return GD_ARRAY_TYPE_FLOAT;
	} else if constexpr (std::is_same_v<U, bool> ||
			(std::is_integral_v<U> &&
					sizeof(U) == sizeof(uint8_t))) {
		// GdBool and byte are both one-byte ABI values. The underlying C
		// aliases can be indistinguishable, so _get_array accepts either
		// one-byte wire type for this category.
		return GD_ARRAY_TYPE_BOOL;
	} else if constexpr (std::is_integral_v<U> && sizeof(U) == sizeof(int64_t)) {
		// GdInt and GdObj intentionally share the 64-bit ABI.
		return GD_ARRAY_TYPE_INT64;
	} else {
		return GD_ARRAY_TYPE_UNKNOWN;
	}
}

GdString to_return_cstr(const String &ret_val);
void free_return_cstr(GdString ret_val);
GdArray create_array(int32_t type, int32_t size);
void free_array(GdArray array);

template <typename T>
void set_array(GdArray array, int64_t index, T value);
template <typename T>
T *get_array(GdArray array, int64_t index);

template <typename T>
T *get_array(GdArray array, int64_t index) {
	return static_cast<T *>(
			_get_array(array, index, sizeof(T), _array_type_for<T>()));
}
template <typename T>
void set_array(GdArray array, int64_t index, T value) {
	auto ptr = get_array<T>(array, index);
	if (ptr == nullptr) {
		return;
	}
	*ptr = value;
}

} // namespace SpxAbi

#endif // SPX_ABI_H
