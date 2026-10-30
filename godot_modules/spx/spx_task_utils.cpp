/**************************************************************************/
/*  spx_task_utils.cpp                                                    */
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

#include "spx_task_utils.h"

#include "core/object/worker_thread_pool.h"

namespace SpxTaskUtils {

struct IndexedTaskContext {
	const IndexedTask *task = nullptr;
};

// Adapt std::function to WorkerThreadPool's native callback signature.
static void indexed_task_trampoline(void *p_userdata, uint32_t p_index) {
	IndexedTaskContext *context = static_cast<IndexedTaskContext *>(p_userdata);
	(*context->task)(p_index);
}

bool execute_group_task(
		int p_element_count,
		const IndexedTask &p_parallel_task,
		const std::function<void()> &p_serial_task,
		int p_parallel_threshold,
		const String &p_description) {
	if (!p_parallel_task || !p_serial_task || p_element_count < 0) {
		return false;
	}
	if (p_element_count == 0) {
		return true;
	}

	const int parallel_threshold = p_parallel_threshold > 0 ? p_parallel_threshold : 1;
	WorkerThreadPool *worker_pool = WorkerThreadPool::get_singleton();
	// Web without threads and single-threaded desktop builds use the same fallback.
	if (p_element_count < parallel_threshold || worker_pool->get_thread_count() <= 1) {
		p_serial_task();
		return true;
	}

	IndexedTaskContext context;
	context.task = &p_parallel_task;
	const WorkerThreadPool::GroupID group = worker_pool->add_native_group_task(
			indexed_task_trampoline,
			&context,
			p_element_count,
			-1,
			false,
			p_description);
	// The context and callbacks may be stack-owned, so wait before returning.
	worker_pool->wait_for_group_task_completion(group);
	return true;
}

} // namespace SpxTaskUtils
