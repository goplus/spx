/**************************************************************************/
/*  spx_input_mgr.h                                                       */
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

#ifndef SPX_INPUT_MGR_H
#define SPX_INPUT_MGR_H

#include "core/string/string_name.h"
#include "core/templates/hash_map.h"
#include "core/templates/vector.h"
#include "gdextension_spx_ext.h"
#include "spx_base_mgr.h"
#include "spx_input_proxy.h"

class SpxInputMgr : public SpxBaseMgr {
	SPXCLASS(SpxInputMgr, SpxBaseMgr)
public:
	virtual ~SpxInputMgr() = default; // Added virtual destructor to fix -Werror=non-virtual-dtor
	virtual void on_start() override;
	void on_reset(int reset_code) override;

protected:
	SpxInputProxy *input_proxy = nullptr;
	Vector<StringName> action_names;
	HashMap<StringName, GdInt> action_ids;

public:
	SPX_API GdVec2 get_global_mouse_pos();
	SPX_API GdBool get_key(GdInt key);
	SPX_API GdBool get_mouse_state(GdInt mouse_id);
	SPX_API GdInt get_key_state(GdInt key);
	SPX_API GdFloat get_axis(GdString neg_action, GdString pos_action);
	SPX_API GdBool is_action_pressed(GdString action);
	SPX_API GdBool is_action_just_pressed(GdString action);
	SPX_API GdBool is_action_just_released(GdString action);
	SPX_API GdInt register_action(GdString action);
	SPX_API GdFloat get_axis_id(GdInt neg_action_id, GdInt pos_action_id);
	SPX_API GdBool is_action_pressed_id(GdInt action_id);
	SPX_API GdBool is_action_just_pressed_id(GdInt action_id);
	SPX_API GdBool is_action_just_released_id(GdInt action_id);
	SPX_API void write_snapshot(float *out, int len);

private:
	static constexpr GdInt KEY_ANY = -1;
	const StringName *get_registered_action(GdInt action_id) const;
};

#endif // SPX_INPUT_MGR_H
