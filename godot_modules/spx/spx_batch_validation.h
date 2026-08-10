/**************************************************************************/
/*  spx_batch_validation.h                                                */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
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

#ifndef SPX_BATCH_VALIDATION_H
#define SPX_BATCH_VALIDATION_H

#include <cmath>
#include <limits>

namespace SpxBatchValidation {

// Batch headers are transported as float32 values for compatibility with the
// existing bridge. Validate before converting: converting NaN, infinity, or an
// out-of-range floating-point value to int has undefined behavior in C++.
static inline bool decode_int(float p_value, int &r_value) {
	const double value = static_cast<double>(p_value);
	if (!std::isfinite(value) || std::trunc(value) != value ||
			value < static_cast<double>(std::numeric_limits<int>::min()) ||
			value > static_cast<double>(std::numeric_limits<int>::max())) {
		return false;
	}

	r_value = static_cast<int>(value);
	return true;
}

static inline bool decode_nonnegative_count(float p_value, int &r_value) {
	return decode_int(p_value, r_value) && r_value >= 0;
}

static inline bool all_finite(const float *p_values, int p_count) {
	for (int i = 0; i < p_count; i++) {
		if (!std::isfinite(p_values[i])) {
			return false;
		}
	}
	return true;
}

} // namespace SpxBatchValidation

#endif // SPX_BATCH_VALIDATION_H
