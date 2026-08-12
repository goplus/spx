/**************************************************************************/
/*  spx_ui_mgr.h                                                          */
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

#ifndef SPX_UI_MGR_H
#define SPX_UI_MGR_H

#include "gdextension_spx_ext.h"
#include "spx_engine.h"
#include "spx_ui.h"
#include "spx_ui_binding.h"
class Node;
class SpxUiMgr : public SpxBaseMgr, private SpxUiBindingListener {
	SPXCLASS(SpxUiMgr, SpxBaseMgr)

public:
	virtual ~SpxUiMgr() = default; // Added virtual destructor to fix -Werror=non-virtual-dtor

private:
	RBMap<GdObj, SpxUi *> id_objects;

	Control *create_control(GdString path);
	void _clear_nodes(bool emit_destroyed, bool queue_controls);
	Node *create_owner_node() override;
	void on_spx_ui_clicked(GdObj p_gid, ObjectID p_binding_id) override;
	void on_spx_ui_destroyed(GdObj p_gid, ObjectID p_binding_id) override;

public:
	void on_awake() override;
	void on_destroy() override;
	void on_reset(int reset_code) override;

	SpxUi *on_create_node(Control *control, GdInt type, bool is_attach = true);
	SpxUi *get_node(GdObj obj);

	static ESpxUiType get_node_type(Node *obj);

public:
	SPX_API GdObj bind_node(GdObj obj, GdString rel_path);

	SPX_API GdObj create_node(GdString path);
	SPX_API GdObj create_button(GdString path, GdString text);
	SPX_API GdObj create_label(GdString path, GdString text);
	SPX_API GdObj create_image(GdString path);
	SPX_API GdObj create_toggle(GdString path, GdBool value);
	SPX_API GdObj create_slider(GdString path, GdFloat value);
	SPX_API GdObj create_input(GdString path, GdString text);
	SPX_API GdBool destroy_node(GdObj obj);

	SPX_API GdInt get_type(GdObj obj);
	SPX_API void set_text(GdObj obj, GdString text);
	SPX_API GdString get_text(GdObj obj);
	SPX_API void set_texture(GdObj obj, GdString path);
	SPX_API GdString get_texture(GdObj obj);
	SPX_API void set_color(GdObj obj, GdColor color);
	SPX_API GdColor get_color(GdObj obj);
	SPX_API void set_font_size(GdObj obj, GdInt size);
	SPX_API GdInt get_font_size(GdObj obj);
	SPX_API void set_visible(GdObj obj, GdBool visible);
	SPX_API GdBool get_visible(GdObj obj);
	SPX_API void set_interactable(GdObj obj, GdBool interactable);
	SPX_API GdBool get_interactable(GdObj obj);
	SPX_API void set_rect(GdObj obj, GdRect2 rect);
	SPX_API GdRect2 get_rect(GdObj obj);

	SPX_API GdInt get_layout_direction(GdObj obj);
	SPX_API void set_layout_direction(GdObj obj, GdInt value);
	SPX_API GdInt get_layout_mode(GdObj obj);
	SPX_API void set_layout_mode(GdObj obj, GdInt value);
	SPX_API GdInt get_anchors_preset(GdObj obj);
	SPX_API void set_anchors_preset(GdObj obj, GdInt value);
	SPX_API GdVec2 get_scale(GdObj obj);
	SPX_API void set_scale(GdObj obj, GdVec2 value);
	SPX_API GdVec2 get_position(GdObj obj);
	SPX_API void set_position(GdObj obj, GdVec2 value);
	SPX_API GdVec2 get_size(GdObj obj);
	SPX_API void set_size(GdObj obj, GdVec2 value);
	SPX_API GdVec2 get_global_position(GdObj obj);
	SPX_API void set_global_position(GdObj obj, GdVec2 value);
	SPX_API GdFloat get_rotation(GdObj obj);
	SPX_API void set_rotation(GdObj obj, GdFloat value);

	SPX_API GdBool get_flip(GdObj obj, GdBool horizontal);
	SPX_API void set_flip(GdObj obj, GdBool horizontal, GdBool is_flip);
};

#endif // SPX_UI_MGR_H
