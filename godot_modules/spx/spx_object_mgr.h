/**************************************************************************/
/*  spx_object_mgr.h                                                        */
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

#ifndef SPX_OBJECT_MGR_H
#define SPX_OBJECT_MGR_H

#include "core/os/thread.h"
#include "core/templates/hash_map.h"
#include "scene/2d/node_2d.h"
#include "spx_manager.h"

// Owns non-Node objects and their shared scene root on the engine main thread.
// Returned pointers must not escape the current main-thread task; callers from
// other threads must enqueue work before entering the manager.
template <typename T>
class SpxObjectMgr : public SpxManager {
protected:
	HashMap<GdObj, T *> id_objects;
	Node2D *root = nullptr;

	bool _require_main_thread(const char *p_operation) const {
		if (likely(Thread::is_main_thread())) {
			return true;
		}
		ERR_PRINT(vformat("Object manager operation %s must run on the engine main thread.", p_operation));
		return false;
	}

	GdObj _create_object() {
		if (unlikely(!_require_main_thread(__func__))) {
			return NULL_OBJECT_ID;
		}
		auto id = get_unique_id();
		T *object = memnew(T);
		object->on_create(id, root);
		id_objects[id] = object;
		return id;
	}

	T *_find_object(GdObj obj) const {
		T *const *object = id_objects.getptr(obj);
		return object != nullptr ? *object : nullptr;
	}

	T *_find_object_checked(GdObj obj, const char *p_operation) const {
		if (unlikely(!_require_main_thread(p_operation))) {
			return nullptr;
		}
		return _find_object(obj);
	}

	void _create_root(const String &name) {
		if (unlikely(!_require_main_thread(__func__))) {
			return;
		}
		root = memnew(Node2D);
		root->set_name(name);
		get_spx_root()->add_child(root);
	}

	void _destroy_objects_and_root();

	void _update_all(float delta);

	void _reset_objects(int reset_code);

public:
	T *get_object(GdObj obj) {
		return _find_object_checked(obj, __func__);
	}

	const T *get_object(GdObj obj) const {
		return _find_object_checked(obj, __func__);
	}

	template <typename Func>
	bool with_object(GdObj obj, Func &&func) {
		T *object = _find_object_checked(obj, __func__);
		if (object == nullptr) {
			return false;
		}
		func(object);
		return true;
	}

	template <typename Ret, typename Func>
	Ret with_object_ret(GdObj obj, Ret default_value, Func &&func) {
		T *object = _find_object_checked(obj, __func__);
		if (object == nullptr) {
			return default_value;
		}
		return func(object);
	}

	void destroy_object(GdObj obj);
};

template <typename T>
void SpxObjectMgr<T>::_destroy_objects_and_root() {
	if (unlikely(!_require_main_thread(__func__))) {
		return;
	}
	Vector<T *> objects;
	for (const KeyValue<GdObj, T *> &E : id_objects) {
		objects.push_back(E.value);
	}
	id_objects.clear();

	for (T *object : objects) {
		object->on_destroy();
		memdelete(object);
	}

	if (root) {
		root->queue_free();
		root = nullptr;
	}
}

template <typename T>
void SpxObjectMgr<T>::_update_all(float delta) {
	if (unlikely(!_require_main_thread(__func__))) {
		return;
	}
	Vector<GdObj> object_ids;
	for (const KeyValue<GdObj, T *> &E : id_objects) {
		object_ids.push_back(E.key);
	}

	// Re-resolve each ID so a main-thread callback that destroys a later object
	// cannot leave a dangling pointer in this update pass.
	for (GdObj id : object_ids) {
		T *object = _find_object(id);
		if (object != nullptr) {
			object->on_update(delta);
		}
	}
}

template <typename T>
void SpxObjectMgr<T>::_reset_objects(int reset_code) {
	if (unlikely(!_require_main_thread(__func__))) {
		return;
	}
	Vector<T *> objects;
	for (const KeyValue<GdObj, T *> &E : id_objects) {
		objects.push_back(E.value);
	}
	id_objects.clear();

	for (T *object : objects) {
		object->on_reset(reset_code);
		object->on_destroy();
		memdelete(object);
	}
}

template <typename T>
void SpxObjectMgr<T>::destroy_object(GdObj obj) {
	T *object = _find_object_checked(obj, __func__);
	if (object != nullptr) {
		id_objects.erase(obj);
		object->on_destroy();
		memdelete(object);
	}
}

#endif // SPX_OBJECT_MGR_H
