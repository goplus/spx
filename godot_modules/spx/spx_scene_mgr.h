/**************************************************************************/
/*  spx_scene_mgr.h                                                       */
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

#ifndef SPX_SCENE_MGR_H
#define SPX_SCENE_MGR_H

#include "gdextension_spx_ext.h"
#include "spx_base_mgr.h"

class ISortableSprite;
class TileMapLayer;
class SubViewport;

class SpxSceneMgr : public SpxBaseMgr {
	SPXCLASS(SpxSceneMgr, SpxBaseMgr)

private:
	const String DEFAULT_SAVE_PATH = "user://exported_scene.png";
	bool export_pending = false;
	double elapsed = 0.0;
	SubViewport *viewport_to_export = nullptr;
	Vector2 cached_cell_size{ 16, 16 };

	void _request_export(SubViewport *viewport);
	void _export_vp_png(SubViewport *viewport);

public:
	// Pure sprite management (kept in SpxExtMgr)
	Node *pure_sprite_root;
	RBMap<GdObj, ISortableSprite *> id_pure_sprites;

	void on_awake() override;
	void on_update(float delta) override;
	void on_destroy() override;
	void on_reset(int reset_code) override;

	void collect_sortable_sprites(Vector<ISortableSprite *> &out);

	_FORCE_INLINE_ void set_cached_cell_size(Vector2 cell_size) {
		cached_cell_size = cell_size;
	}
	Rect2 get_scene_bounds(Node *node);
	Rect2 get_tilemap_bounds(TileMapLayer *layer);

	void export_scene_as_png(Node *root);

public:
	virtual ~SpxSceneMgr() = default; // Added virtual destructor to fix -Werror=non-virtual-dtor

	SPX_API void change_scene_to_file(GdString path);
	SPX_API void destroy_all_sprites();
	SPX_API GdInt reload_current_scene();
	SPX_API void unload_current_scene();

	// create sprites
	SPX_API void clear_pure_sprites();
	SPX_API void create_pure_sprite(GdString texture_path, GdVec2 pos, GdInt zindex);
	SPX_API void destroy_pure_sprite(GdObj id);

	SPX_API GdObj create_render_sprite(GdString texture_path, GdVec2 pos, GdFloat degree, GdVec2 scale, GdInt zindex, GdVec2 pivot);
	SPX_API GdObj create_static_sprite(GdString texture_path, GdVec2 pos, GdFloat degree, GdVec2 scale, GdInt zindex, GdVec2 pivot, GdInt collider_type, GdVec2 collider_pivot, GdArray collider_params);
};

#endif // SPX_SCENE_MGR_H
