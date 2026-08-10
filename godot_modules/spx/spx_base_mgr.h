/**************************************************************************/
/*  spx_base_mgr.h                                                        */
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

#ifndef SPX_BASE_MGR_H
#define SPX_BASE_MGR_H

#include "gdextension_spx_ext.h"
#include "scene/2d/node_2d.h"
#include "spx_mgr_access.h"
#include "spx_utils.h"
#include "svg_mgr.h"

#define SPXCLASS(m_class, m_inherits)        \
public:                                      \
	String get_class_name() const override { \
		return #m_class;                     \
	}

#define SpxStr(str) (String::utf8((const char *)str))
#define SpxReturnStr(str) (SpxBaseMgr::to_return_cstr(str))

#ifndef SPX_API
#define SPX_API
#endif

#ifndef SPX_BIND
#define SPX_BIND SPX_API
#endif

#define NULL_OBJECT_ID 0

class Window;
class SceneTree;
class SpxBaseMgr {
private:
	static void *_get_array(GdArray array, int64_t index, int type_size);

public:
	static GdString to_return_cstr(const String &ret_val);
	static void free_return_cstr(GdString ret_val);
	static GdArray create_array(int32_t type, int32_t size);
	static void free_array(GdArray array);

	template <typename T>
	static void set_array(GdArray array, int64_t index, T value);
	template <typename T>
	static T *get_array(GdArray array, int64_t index);

protected:
	Node *owner;
	virtual Node *create_owner_node();

protected:
	virtual GdInt get_unique_id();
	virtual SceneTree *get_tree();
	virtual Window *get_root();
	virtual Node *get_spx_root();

public:
	virtual String get_class_name() const { return "SpxBaseMgr"; }
	virtual void on_awake();
	virtual void on_start();
	virtual void on_update(float delta);
	virtual void on_fixed_update(float delta);
	virtual void on_destroy();
	virtual void on_reset(int reset_code);
	virtual void on_exit(int exit_code);
	virtual void on_pause();
	virtual void on_resume();
	virtual ~SpxBaseMgr() = default; // Added virtual destructor to fix -Werror=non-virtual-dtor
};

template <typename T>
T *SpxBaseMgr::get_array(GdArray array, int64_t index) {
	return static_cast<T *>(_get_array(array, index, sizeof(T)));
}
template <typename T>
void SpxBaseMgr::set_array(GdArray array, int64_t index, T value) {
	auto ptr = get_array<T>(array, index);
	if (ptr == nullptr) {
		return;
	}
	*ptr = value;
}

#endif // SPX_BASE_MGR_H
