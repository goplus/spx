/**************************************************************************/
/*  test_spx_task_utils.h                                                 */
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
/* "Software"), to deal in the Software without restriction, including   */
/* without limitation the rights to use, copy, modify, merge, publish,    */
/* distribute, sublicense, and/or sell copies of the Software, and to     */
/* permit persons to whom the Software is furnished to do so, subject to   */
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

#ifndef TEST_SPX_TASK_UTILS_H
#define TEST_SPX_TASK_UTILS_H

#include "../spx_task_utils.h"
#include "core/object/worker_thread_pool.h"
#include "tests/test_macros.h"

#include <array>
#include <atomic>

namespace TestSpxTaskUtils {

TEST_CASE("[SPX] Group task falls back to serial work below the threshold") {
	std::array<std::atomic<int>, 3> visits;
	for (std::atomic<int> &visit : visits) {
		visit.store(0);
	}
	int serial_calls = 0;

	const SpxTaskUtils::IndexedTask parallel_task = [&](uint32_t p_index) {
		visits[p_index].fetch_add(1, std::memory_order_relaxed);
	};
	const std::function<void()> serial_task = [&]() {
		++serial_calls;
		for (uint32_t index = 0; index < visits.size(); ++index) {
			parallel_task(index);
		}
	};

	REQUIRE(SpxTaskUtils::execute_group_task(3, parallel_task, serial_task, 4, String("SPXTaskUtilsTestSerial")));
	CHECK(serial_calls == 1);
	for (const std::atomic<int> &visit : visits) {
		CHECK(visit.load(std::memory_order_relaxed) == 1);
	}
}

TEST_CASE("[SPX] Group task falls back when the worker pool has no threads") {
	WorkerThreadPool *worker_pool = WorkerThreadPool::get_singleton();
	REQUIRE(worker_pool != nullptr);
	const int original_thread_count = worker_pool->get_thread_count();
	worker_pool->finish();

	int parallel_calls = 0;
	int serial_calls = 0;
	const bool result = SpxTaskUtils::execute_group_task(
			1,
			[&](uint32_t) { ++parallel_calls; },
			[&]() { ++serial_calls; },
			1,
			String("SPXTaskUtilsTestNoThreads"));

	if (original_thread_count > 0) {
		worker_pool->init(original_thread_count);
	}

	CHECK(result);
	CHECK(parallel_calls == 0);
	CHECK(serial_calls == 1);
}

TEST_CASE("[SPX] Group task normalizes non-positive parallel thresholds") {
	for (const int threshold : { 0, -1 }) {
		std::atomic<int> visits = 0;
		std::atomic<bool> invalid_index = false;
		int serial_calls = 0;
		const SpxTaskUtils::IndexedTask parallel_task = [&](uint32_t p_index) {
			if (p_index != 0) {
				invalid_index.store(true, std::memory_order_relaxed);
				return;
			}
			visits.fetch_add(1, std::memory_order_relaxed);
		};
		const std::function<void()> serial_task = [&]() {
			++serial_calls;
			parallel_task(0);
		};

		REQUIRE(SpxTaskUtils::execute_group_task(1, parallel_task, serial_task, threshold, String("SPXTaskUtilsTestThreshold")));
		WorkerThreadPool *worker_pool = WorkerThreadPool::get_singleton();
		const bool serial_fallback_expected = worker_pool == nullptr || worker_pool->get_thread_count() <= 1;
		CHECK_FALSE(invalid_index.load(std::memory_order_relaxed));
		CHECK(visits.load(std::memory_order_relaxed) == 1);
		CHECK(serial_calls == (serial_fallback_expected ? 1 : 0));
	}
}

TEST_CASE("[SPX] Group task executes every index exactly once in the available mode") {
	constexpr uint32_t TASK_COUNT = 32;
	std::array<std::atomic<int>, TASK_COUNT> visits;
	for (std::atomic<int> &visit : visits) {
		visit.store(0);
	}
	int serial_calls = 0;
	std::atomic<bool> invalid_index = false;

	const SpxTaskUtils::IndexedTask parallel_task = [&](uint32_t p_index) {
		if (p_index >= TASK_COUNT) {
			invalid_index.store(true, std::memory_order_relaxed);
			return;
		}
		visits[p_index].fetch_add(1, std::memory_order_relaxed);
	};
	const std::function<void()> serial_task = [&]() {
		++serial_calls;
		for (uint32_t index = 0; index < TASK_COUNT; ++index) {
			parallel_task(index);
		}
	};

	// Threshold 1 exercises the pool on threaded builds and the fallback on Web.
	WorkerThreadPool *worker_pool = WorkerThreadPool::get_singleton();
	const bool serial_fallback_expected = worker_pool == nullptr || worker_pool->get_thread_count() <= 1;
	REQUIRE(SpxTaskUtils::execute_group_task(TASK_COUNT, parallel_task, serial_task, 1, String("SPXTaskUtilsTestAll")));
	CHECK(serial_calls == (serial_fallback_expected ? 1 : 0));
	CHECK_FALSE(invalid_index.load(std::memory_order_relaxed));
	for (const std::atomic<int> &visit : visits) {
		CHECK(visit.load(std::memory_order_relaxed) == 1);
	}
}

TEST_CASE("[SPX] Group task handles empty and invalid input without running callbacks") {
	int callback_calls = 0;
	const SpxTaskUtils::IndexedTask parallel_task = [&](uint32_t) {
		++callback_calls;
	};
	const std::function<void()> serial_task = [&]() {
		++callback_calls;
	};

	CHECK(SpxTaskUtils::execute_group_task(0, parallel_task, serial_task, 1, String("SPXTaskUtilsTestEmpty")));
	CHECK(callback_calls == 0);
	CHECK_FALSE(SpxTaskUtils::execute_group_task(-1, parallel_task, serial_task, 1, String("SPXTaskUtilsTestNegative")));
	CHECK_FALSE(SpxTaskUtils::execute_group_task(1, SpxTaskUtils::IndexedTask(), serial_task, 1, String("SPXTaskUtilsTestMissingParallel")));
	CHECK_FALSE(SpxTaskUtils::execute_group_task(1, parallel_task, std::function<void()>(), 1, String("SPXTaskUtilsTestMissingSerial")));
	CHECK(callback_calls == 0);
}

} // namespace TestSpxTaskUtils

#endif // TEST_SPX_TASK_UTILS_H
