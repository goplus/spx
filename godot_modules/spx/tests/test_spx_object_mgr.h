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

#include "../spx_object_access.h"
#include "../spx_object_mgr.h"
#include "spx_test_fixtures.h"
#include "tests/test_macros.h"

#include <functional>

namespace TestSpxObjectMgr {

struct ManagedValue {
	static inline int destroy_count = 0;
	static inline int reset_count = 0;
	int update_count = 0;
	GdObj id = 0;
	Node2D *root = nullptr;
	std::function<void()> update;
	std::function<void()> destroy;

	void on_create(GdObj p_id, Node2D *p_root) {
		id = p_id;
		root = p_root;
	}
	void on_update(float) {
		update_count++;
		if (update) {
			update();
		}
	}
	void on_destroy() {
		destroy_count++;
		if (destroy) {
			destroy();
		}
	}
	void on_reset(int p_code) { reset_count += p_code; }
};

class ObjectMgrProbe : public SpxObjectMgr<ManagedValue> {
public:
	void add(GdObj p_id, ManagedValue *p_value) { id_objects[p_id] = p_value; }
	void update_all() { _update_all(0.0f); }
	void create_root() { _create_root("managed_values"); }
	GdObj create_object() { return _create_object(); }
	void reset_objects(int p_code) { _reset_objects(p_code); }
	void clear() { _destroy_objects_and_root(); }
};

struct WorkerProbe {
	ObjectMgrProbe *manager = nullptr;
	GdObj id = 0;
	ManagedValue *lookup_result = reinterpret_cast<ManagedValue *>(uintptr_t(1));
	bool with_object_result = true;
	GdObj created = -1;

	static void run(void *p_data) {
		WorkerProbe *probe = static_cast<WorkerProbe *>(p_data);
		probe->lookup_result = probe->manager->get_object(probe->id);
		probe->with_object_result = probe->manager->with_object(probe->id, [](ManagedValue *p_value) {
			p_value->update_count++;
		});
		probe->manager->update_all();
		probe->manager->destroy_object(probe->id);
		probe->created = probe->manager->create_object();
		probe->manager->create_root();
		probe->manager->reset_objects(1);
		probe->manager->clear();
	}
};

struct CheckedLookupProbe {
	ObjectMgrProbe *manager = nullptr;
	GdObj id = 0;
	int getter_calls = 0;
	ManagedValue *result = nullptr;

	static ManagedValue *resolve(CheckedLookupProbe *p_probe, GdObj p_id) {
		p_probe->getter_calls++;
		return p_probe->manager->get_object(p_id);
	}

	static void run(void *p_data) {
		auto *probe = static_cast<CheckedLookupProbe *>(p_data);
		probe->result = spx_checked_lookup<ManagedValue>(probe->id, __func__, probe, &resolve);
	}
};

TEST_CASE("[SPX] Checked lookup validates the manager and thread before invoking the getter") {
	ObjectMgrProbe manager;
	CheckedLookupProbe probe;
	probe.manager = &manager;
	probe.id = 8;
	CHECK(spx_checked_lookup<ManagedValue>(probe.id, __func__, static_cast<CheckedLookupProbe *>(nullptr), &CheckedLookupProbe::resolve) == nullptr);
	CHECK(probe.getter_calls == 0);
	ERR_PRINT_OFF;
	CheckedLookupProbe::run(&probe);
	ERR_PRINT_ON;
	CHECK(probe.result == nullptr);
	CHECK(probe.getter_calls == 1);
	ManagedValue *value = memnew(ManagedValue);
	manager.add(probe.id, value);
	CheckedLookupProbe::run(&probe);
	CHECK(probe.result == value);
	CHECK(probe.getter_calls == 2);
#ifdef THREADS_ENABLED
	Thread worker;
	ERR_PRINT_OFF;
	const Thread::ID worker_id = worker.start(&CheckedLookupProbe::run, &probe);
	if (worker.is_started()) {
		worker.wait_to_finish();
	}
	ERR_PRINT_ON;
	CHECK_NE(worker_id, Thread::UNASSIGNED_ID);
	CHECK(probe.result == nullptr);
	CHECK(probe.getter_calls == 2);
#endif
	CHECK(manager.get_object(probe.id) == value);
	manager.destroy_object(probe.id);
}

#ifdef THREADS_ENABLED
TEST_CASE("[SPX] Object manager rejects access outside the engine main thread") {
	ManagedValue::destroy_count = 0;
	ObjectMgrProbe manager;
	ManagedValue *value = memnew(ManagedValue);
	constexpr GdObj id = 7;
	manager.add(id, value);

	CHECK_EQ(manager.get_object(id), value);

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
	CHECK(probe.created == NULL_OBJECT_ID);
	CHECK_EQ(value->update_count, 0);
	CHECK_EQ(ManagedValue::destroy_count, 0);
	CHECK_EQ(manager.get_object(id), value);

	manager.update_all();
	CHECK_EQ(value->update_count, 1);
	manager.destroy_object(id);
	CHECK_EQ(ManagedValue::destroy_count, 1);
	CHECK_EQ(manager.get_object(id), nullptr);
}
#endif // THREADS_ENABLED

TEST_CASE("[SceneTree][SPX] Object manager handles missing objects and an empty root") {
	TestSpx::EngineScene scene;
	ManagedValue::destroy_count = 0;
	ManagedValue::reset_count = 0;
	ObjectMgrProbe manager;
	CHECK(manager.get_object(99) == nullptr);
	manager.update_all();
	manager.reset_objects(3);
	manager.destroy_object(99);
	manager.clear();
	CHECK(ManagedValue::destroy_count == 0);
	CHECK(ManagedValue::reset_count == 0);

	const GdObj id = manager.create_object();
	REQUIRE(id != NULL_OBJECT_ID);
	CHECK(manager.get_object(id)->root == nullptr);
	manager.update_all();
	CHECK(manager.get_object(id)->update_count == 1);
	manager.reset_objects(3);
	CHECK(ManagedValue::reset_count == 3);
	CHECK(ManagedValue::destroy_count == 1);
	CHECK(manager.get_object(id) == nullptr);
	manager.create_root();
	manager.clear();
	CHECK(ManagedValue::destroy_count == 1);
}

TEST_CASE("[SceneTree][SPX] Object manager callbacks observe cleared ownership and preserve later objects") {
	TestSpx::EngineScene scene;
	ObjectMgrProbe manager;
	manager.create_root();
	const GdObj first = manager.create_object();
	const GdObj second = manager.create_object();
	REQUIRE(first != second);
	const ObjectMgrProbe &view = manager;
	ManagedValue *value = manager.get_object(first);
	REQUIRE(value != nullptr);
	CHECK(view.get_object(first) == value);
	CHECK(value->id == first);
	Node2D *root = value->root;
	REQUIRE(root != nullptr);
	CHECK(root->get_parent() == scene.root);
	int callback_count = 0;
	auto update = [&](ManagedValue *p_value) { callback_count++; p_value->update_count = 4; };
	auto read = [&](ManagedValue *p_value) { callback_count++; return p_value->update_count; };
	CHECK(manager.with_object(first, update));
	CHECK(manager.with_object_ret(first, -1, read) == 4);
	CHECK_FALSE(manager.with_object(0, update));
	CHECK(manager.with_object_ret(0, -1, read) == -1);
	CHECK(callback_count == 2);
	int updates = 0;
	value->update = [&]() { updates++; manager.destroy_object(second); };
	manager.get_object(second)->update = [&]() { updates++; manager.destroy_object(first); };
	ManagedValue::destroy_count = 0;
	manager.update_all();
	CHECK(updates == 1);
	CHECK(ManagedValue::destroy_count == 1);
	const GdObj survivor = manager.get_object(first) != nullptr ? first : second;
	GdObj created_during_reset = 0;
	manager.get_object(survivor)->destroy = [&]() {
		CHECK(manager.get_object(first) == nullptr);
		CHECK(manager.get_object(second) == nullptr);
		created_during_reset = manager.create_object();
	};
	ManagedValue::reset_count = 0;
	manager.reset_objects(7);
	CHECK(ManagedValue::reset_count == 7);
	CHECK(ManagedValue::destroy_count == 2);
	CHECK(manager.get_object(created_during_reset) != nullptr);
	CHECK_FALSE(root->is_queued_for_deletion());
	manager.clear();
	CHECK(ManagedValue::destroy_count == 3);
	CHECK(manager.get_object(created_during_reset) == nullptr);
	CHECK(root->is_queued_for_deletion());
	manager.clear();
	CHECK(ManagedValue::destroy_count == 3);
}

} // namespace TestSpxObjectMgr

#endif // TEST_SPX_OBJECT_MGR_H
