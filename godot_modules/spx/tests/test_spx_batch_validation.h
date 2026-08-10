/**************************************************************************/
/*  test_spx_batch_validation.h                                           */
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

#ifndef TEST_SPX_BATCH_VALIDATION_H
#define TEST_SPX_BATCH_VALIDATION_H

#include "../spx_batch_validation.h"
#include "tests/test_macros.h"

#include <limits>

namespace TestSpxBatchValidation {

TEST_CASE("[SPX] Batch count decoding rejects unsafe float-to-int conversions") {
	int decoded = -1;
	CHECK(SpxBatchValidation::decode_nonnegative_count(0.0f, decoded));
	CHECK_EQ(decoded, 0);
	CHECK(SpxBatchValidation::decode_nonnegative_count(42.0f, decoded));
	CHECK_EQ(decoded, 42);

	CHECK_FALSE(SpxBatchValidation::decode_nonnegative_count(-1.0f, decoded));
	CHECK_FALSE(SpxBatchValidation::decode_nonnegative_count(1.5f, decoded));
	CHECK_FALSE(SpxBatchValidation::decode_nonnegative_count(std::numeric_limits<float>::infinity(), decoded));
	CHECK_FALSE(SpxBatchValidation::decode_nonnegative_count(std::numeric_limits<float>::quiet_NaN(), decoded));
	// INT_MAX rounds up to 2^31 as float32 and must be rejected before cast.
	CHECK_FALSE(SpxBatchValidation::decode_nonnegative_count(static_cast<float>(std::numeric_limits<int>::max()), decoded));
}

TEST_CASE("[SPX] Batch integer decoding accepts signed values and rejects fractions") {
	int decoded = 0;
	CHECK(SpxBatchValidation::decode_int(-4096.0f, decoded));
	CHECK_EQ(decoded, -4096);
	CHECK(SpxBatchValidation::decode_int(4096.0f, decoded));
	CHECK_EQ(decoded, 4096);
	CHECK_FALSE(SpxBatchValidation::decode_int(-0.25f, decoded));
	CHECK_FALSE(SpxBatchValidation::decode_int(std::numeric_limits<float>::quiet_NaN(), decoded));
}

TEST_CASE("[SPX] Batch finite preflight checks every selected lane") {
	const float finite_values[] = { 0.0f, -1.0f, 3.5f };
	CHECK(SpxBatchValidation::all_finite(finite_values, 3));

	const float non_finite_values[] = { 0.0f, std::numeric_limits<float>::infinity(), 3.5f };
	CHECK_FALSE(SpxBatchValidation::all_finite(non_finite_values, 3));
}

} // namespace TestSpxBatchValidation

#endif // TEST_SPX_BATCH_VALIDATION_H
