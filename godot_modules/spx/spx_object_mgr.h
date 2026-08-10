/**************************************************************************/
/*  spx_object_mgr.h                                                      */
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
#include "gdextension_spx_ext.h"
#include "spx_base_mgr.h"

/**
 * @brief Generic object manager template for SPX objects
 *
 * This template provides common functionality for managing SPX objects:
 * - Main-thread-owned object storage and retrieval
 * - Automatic lifecycle management (create, destroy, update)
 * - Common helper macros for null checks
 *
 * Threading:
 * - Managed values own Godot Node/Object state and may only be accessed on the
 *   engine main thread.
 * - Cross-thread producers must enqueue immutable data before entering a
 *   manager. A lock around this map cannot make a returned raw Node pointer
 *   safe after the lock is released.
 *
 * @tparam T The type of object being managed (e.g., SpxAudio, SpxPen)
 */
template <typename T>
class SpxObjectMgr : public SpxBaseMgr {
protected:
	HashMap<GdObj, T *> id_objects;
	Node2D *root = nullptr;

	bool _validate_main_thread(const char *p_operation) const {
		if (likely(Thread::is_main_thread())) {
			return true;
		}
		ERR_PRINT(vformat("%s::%s may only access managed objects on the engine main thread.", get_class_name(), p_operation));
		return false;
	}

	/**
	 * @brief Create and register a new object
	 * Can be called directly from derived classes
	 * Main-thread only
	 */
	GdObj _create_object() {
		if (unlikely(!_validate_main_thread(__func__))) {
			return NULL_OBJECT_ID;
		}
		auto id = get_unique_id();
		T *node = memnew(T);
		node->on_create(id, root);
		id_objects[id] = node;
		return id;
	}

	/**
	 * @brief Get object by ID after main-thread validation
	 */
	T *_get_object_unsafe(GdObj obj) const {
		if (id_objects.has(obj)) {
			return id_objects[obj];
		}
		return nullptr;
	}

	/**
	 * @brief Create root node for this manager's objects
	 */
	void _create_root(const String &name) {
		if (unlikely(!_validate_main_thread(__func__))) {
			return;
		}
		root = memnew(Node2D);
		root->set_name(name);
		get_spx_root()->add_child(root);
	}

	/**
	 * @brief Destroy all managed objects and root node
	 */
	void _destroy_all();

	/**
	 * @brief Update all managed objects
	 */
	void _update_all(float delta);

	/**
	 * @brief Reset all managed objects
	 */
	void _reset_all(int reset_code);

public:
	/**
	 * @brief Get object by ID on the main thread
	 * @warning The returned pointer must not escape the current main-thread task.
	 * @param obj Object ID
	 * @return Pointer to object, or nullptr if not found
	 */
	T *get_object(GdObj obj) {
		if (unlikely(!_validate_main_thread(__func__))) {
			return nullptr;
		}
		return _get_object_unsafe(obj);
	}

	/**
	 * @brief Get object by ID (const version, main-thread only)
	 */
	const T *get_object(GdObj obj) const {
		if (unlikely(!_validate_main_thread(__func__))) {
			return nullptr;
		}
		return _get_object_unsafe(obj);
	}

	template <typename Func>
	bool with_object(GdObj obj, Func &&func) {
		if (unlikely(!_validate_main_thread(__func__))) {
			return false;
		}
		T *object = _get_object_unsafe(obj);
		if (object == nullptr) {
			return false;
		}
		func(object);
		return true;
	}

	template <typename Ret, typename Func>
	Ret with_object_ret(GdObj obj, Ret default_value, Func &&func) {
		if (unlikely(!_validate_main_thread(__func__))) {
			return default_value;
		}
		T *object = _get_object_unsafe(obj);
		if (object == nullptr) {
			return default_value;
		}
		Ret result = func(object);
		return result;
	}

	/**
	 * @brief Destroy a managed object by ID (main-thread only)
	 */
	void destroy_object(GdObj obj);

	/**
	 * @brief Get the number of managed objects (main-thread only)
	 */
	int get_object_count() const {
		if (unlikely(!_validate_main_thread(__func__))) {
			return 0;
		}
		return id_objects.size();
	}

	virtual ~SpxObjectMgr() = default;
};

template <typename T>
void SpxObjectMgr<T>::_destroy_all() {
	if (unlikely(!_validate_main_thread(__func__))) {
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
	if (unlikely(!_validate_main_thread(__func__))) {
		return;
	}
	Vector<GdObj> object_ids;
	for (const KeyValue<GdObj, T *> &E : id_objects) {
		object_ids.push_back(E.key);
	}

	// Re-resolve each ID so a main-thread callback that destroys a later object
	// cannot leave a dangling pointer in this update pass.
	for (GdObj id : object_ids) {
		T *object = _get_object_unsafe(id);
		if (object != nullptr) {
			object->on_update(delta);
		}
	}
}

template <typename T>
void SpxObjectMgr<T>::_reset_all(int reset_code) {
	if (unlikely(!_validate_main_thread(__func__))) {
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
	if (unlikely(!_validate_main_thread(__func__))) {
		return;
	}
	T *object = _get_object_unsafe(obj);
	if (object != nullptr) {
		id_objects.erase(obj);
		object->on_destroy();
		memdelete(object);
	}
}

// Common macros for checking and getting objects with error handling
#define SPX_CHECK_AND_GET_OBJECT_V(obj, getter, obj_type)       \
	auto obj = getter;                                          \
	if (obj == nullptr) {                                       \
		print_error("try to access null " #obj_type " object"); \
		return;                                                 \
	}

#define SPX_CHECK_AND_GET_OBJECT_R(obj, getter, obj_type, ret_value) \
	auto obj = getter;                                               \
	if (obj == nullptr) {                                            \
		print_error("try to access null " #obj_type " object");      \
		return ret_value;                                            \
	}

#endif // SPX_OBJECT_MGR_H
