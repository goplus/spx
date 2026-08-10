/**************************************************************************/
/*  test_spx_object_mgr.h                                                 */
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

#ifndef TEST_SPX_OBJECT_MGR_H
#define TEST_SPX_OBJECT_MGR_H

#include "../spx_object_mgr.h"
#include "tests/test_macros.h"

namespace TestSpxObjectMgr {

struct ManagedValue {
	static inline int destroy_count = 0;
	int update_count = 0;

	void on_update(float) { update_count++; }
	void on_destroy() { destroy_count++; }
	void on_reset(int) {}
};

class ObjectMgrProbe : public SpxObjectMgr<ManagedValue> {
public:
	void add(GdObj p_id, ManagedValue *p_value) { id_objects[p_id] = p_value; }
	void update_all() { _update_all(0.0f); }
};

struct WorkerProbe {
	ObjectMgrProbe *manager = nullptr;
	GdObj id = 0;
	ManagedValue *lookup_result = reinterpret_cast<ManagedValue *>(uintptr_t(1));
	bool with_object_result = true;

	static void run(void *p_data) {
		WorkerProbe *probe = static_cast<WorkerProbe *>(p_data);
		probe->lookup_result = probe->manager->get_object(probe->id);
		probe->with_object_result = probe->manager->with_object(probe->id, [](ManagedValue *p_value) {
			p_value->update_count++;
		});
		probe->manager->update_all();
		probe->manager->destroy_object(probe->id);
	}
};

#ifdef THREADS_ENABLED
TEST_CASE("[SPX] Object manager rejects access outside the engine main thread") {
	ManagedValue::destroy_count = 0;
	ObjectMgrProbe manager;
	ManagedValue *value = memnew(ManagedValue);
	constexpr GdObj id = 7;
	manager.add(id, value);

	CHECK_EQ(manager.get_object(id), value);
	CHECK_EQ(manager.get_object_count(), 1);

	WorkerProbe probe;
	probe.manager = &manager;
	probe.id = id;
	Thread worker;
	ERR_PRINT_OFF
	const Thread::ID worker_id = worker.start(&WorkerProbe::run, &probe);
	if (worker.is_started()) {
		worker.wait_to_finish();
	}
	ERR_PRINT_ON

	REQUIRE_NE(worker_id, Thread::UNASSIGNED_ID);
	CHECK_EQ(probe.lookup_result, nullptr);
	CHECK_FALSE(probe.with_object_result);
	CHECK_EQ(value->update_count, 0);
	CHECK_EQ(ManagedValue::destroy_count, 0);
	CHECK_EQ(manager.get_object_count(), 1);

	manager.update_all();
	CHECK_EQ(value->update_count, 1);
	manager.destroy_object(id);
	CHECK_EQ(ManagedValue::destroy_count, 1);
	CHECK_EQ(manager.get_object_count(), 0);
}
#endif // THREADS_ENABLED

} // namespace TestSpxObjectMgr

#endif // TEST_SPX_OBJECT_MGR_H
