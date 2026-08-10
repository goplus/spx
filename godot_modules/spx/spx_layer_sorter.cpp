/**************************************************************************/
/*  spx_layer_sorter.cpp                                                  */
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

#include "spx_layer_sorter.h"

#include "scene/theme/theme_db.h"

#include "spx.h"
#include "spx_camera_mgr.h"
#include "spx_engine.h"

void SpxLayerSorter::set_mode(LayerSortMode mode) {
	sort_mode = mode;
	_create_drawer();
}

void SpxLayerSorter::update(const Vector<ISortableSprite *> &sortables) {
	if (sort_mode == LayerSortMode::NONE) {
		return;
	}

	auto camera_mgr = SpxEngine::get_singleton()->get_camera();
	set_screen_rect(camera_mgr->get_global_camera_rect());

	// Uncomment the line below to enable screen visibility callbacks
	//_update_visibility(sortables);
	_collect_sprites(sortables);

	if (dynamic_dirty.empty()) {
		return;
	}

	float ratio = float(dynamic_dirty.size()) / float(dynamic_sorted.size() + 1);
	if (ratio > full_sort_ratio) {
		_full_sort_dynamic();
	} else {
		_incremental_sort_dynamic();
	}

	_apply_z_index_merged();

	if (Spx::debug_mode && !drawer) {
		_create_drawer();
	}

	if (drawer) {
		drawer->update();
	}
}

void SpxLayerSorter::add_static_sprite(ISortableSprite *sp) {
	if (!sp || !sp->is_node_valid()) {
		return;
	}

	GdObj id = sp->get_sort_id();
	auto it = std::find_if(static_sorted.begin(), static_sorted.end(),
			[id](const SortInfo &s) { return s.id == id; });

	if (it != static_sorted.end()) {
		return;
	}

	SortInfo info = { id, sp->get_sort_position(), sp };
	auto insert_pos = std::lower_bound(static_sorted.begin(), static_sorted.end(), info, sprite_cmp);
	static_sorted.insert(insert_pos, info);
}

void SpxLayerSorter::remove_static_sprite(ISortableSprite *sp) {
	if (!sp) {
		return;
	}

	GdObj id = sp->get_sort_id();
	static_sorted.erase(
			std::remove_if(static_sorted.begin(), static_sorted.end(),
					[id](const SortInfo &s) { return s.id == id; }),
			static_sorted.end());
}

void SpxLayerSorter::_mark_dirty(ISortableSprite *sp) {
	if (!sp) {
		return;
	}
	GdObj id = sp->get_sort_id();
	if (dynamic_dirty_ids.find(id) != dynamic_dirty_ids.end()) {
		return;
	}

	dynamic_dirty.push_back({ id, sp->get_sort_position(), sp });
	dynamic_dirty_ids.insert(id);
}

void SpxLayerSorter::_update_visibility(const Vector<ISortableSprite *> &sortables) {
	std::unordered_set<GdObj> new_visible;

	auto in_screen = [&](ISortableSprite *sp) -> bool {
		if (!sp) {
			return false;
		}
		return screen_rect.has_point(sp->get_sort_position());
	};

	for (auto sp : sortables) {
		if (!sp || !sp->is_node_valid()) {
			continue;
		}

		if (in_screen(sp)) {
			new_visible.insert(sp->get_sort_id());
			if (visible_ids.find(sp->get_sort_id()) == visible_ids.end()) {
				if (visibility_callback) {
					visibility_callback(sp, true);
				}
			}
		} else {
			if (visible_ids.find(sp->get_sort_id()) != visible_ids.end()) {
				if (visibility_callback) {
					visibility_callback(sp, false);
				}
			}
		}
	}

	visible_ids.swap(new_visible);
}

void SpxLayerSorter::_collect_sprites(const Vector<ISortableSprite *> &sortables) {
	auto remove_invalid = [&](std::vector<SortInfo> &arr, bool remove_offscreen) {
		arr.erase(
				std::remove_if(arr.begin(), arr.end(),
						[&](const SortInfo &s) {
							if (!s.sortable || !s.sortable->is_node_valid()) {
								return true;
							}
							if (remove_offscreen && !screen_rect.has_point(s.sortable->get_sort_position())) {
								return true;
							}

							return false;
						}),
				arr.end());
	};

	remove_invalid(static_sorted, false);
	remove_invalid(dynamic_sorted, true);

	for (auto sp : sortables) {
		if (!sp || !sp->is_node_valid()) {
			continue;
		}

		Point2 pos = sp->get_sort_position();
		if (!screen_rect.has_point(pos)) {
			continue;
		}

		GdObj id = sp->get_sort_id();
		bool is_static = sp->is_sort_static();

		std::vector<SortInfo> &target = is_static ? static_sorted : dynamic_sorted;
		auto found = std::find_if(target.begin(), target.end(),
				[id](const SortInfo &s) { return s.id == id; });

		if (found != target.end()) {
			if (found->pos != pos) {
				found->pos = pos;
				found->sortable = sp;
				if (!is_static) {
					_mark_dirty(sp);
				}
			}
		} else {
			target.push_back({ id, pos, sp });
			if (!is_static) {
				_mark_dirty(sp);
			} else {
				static_initialized = false;
			}
		}
	}

	if (!static_sorted.empty() && !static_initialized) {
		std::sort(static_sorted.begin(), static_sorted.end(), sprite_cmp);
		static_initialized = true;
	}
}

void SpxLayerSorter::_incremental_sort_dynamic() {
	dynamic_sorted.erase(
			std::remove_if(dynamic_sorted.begin(), dynamic_sorted.end(),
					[&](const SortInfo &s) {
						return dynamic_dirty_ids.find(s.id) != dynamic_dirty_ids.end();
					}),
			dynamic_sorted.end());

	for (auto &d : dynamic_dirty) {
		auto pos = std::lower_bound(dynamic_sorted.begin(), dynamic_sorted.end(), d, sprite_cmp);
		dynamic_sorted.insert(pos, d);
	}

	dynamic_dirty.clear();
	dynamic_dirty_ids.clear();
}

void SpxLayerSorter::_full_sort_dynamic() {
	std::sort(dynamic_sorted.begin(), dynamic_sorted.end(), sprite_cmp);
	dynamic_dirty.clear();
	dynamic_dirty_ids.clear();
}

void SpxLayerSorter::_apply_z_index_merged() {
	auto it_static = static_sorted.begin();
	auto it_dynamic = dynamic_sorted.begin();

	int z = 1;
	while (it_static != static_sorted.end() || it_dynamic != dynamic_sorted.end()) {
		bool use_dynamic = false;
		if (it_static == static_sorted.end()) {
			use_dynamic = true;
		} else if (it_dynamic == dynamic_sorted.end()) {
			use_dynamic = false;
		} else {
			use_dynamic = sprite_cmp(*it_dynamic, *it_static);
		}

		SortInfo &s = use_dynamic ? *it_dynamic++ : *it_static++;

		if (s.sortable && s.sortable->is_node_valid()) {
			if (s.sortable->get_sort_z_index() != z) {
				s.sortable->set_sort_z_index(z);
			}
		}
		++z;
	}
}

void SpxLayerSorter::_create_drawer() {
	if (Spx::debug_mode && !drawer) {
		Node *root = nullptr;
		if (SpxEngine::get_singleton()) {
			root = SpxEngine::get_singleton()->get_spx_root();
		}

		if (!root) {
			root = SceneTree::get_singleton()->get_current_scene();
		}

		drawer = memnew(LayerSorterDebugDrawer(this));
		root->add_child(drawer);
	}
}

void SpxLayerSorter::_clear_drawer() {
	if (drawer) {
		drawer->queue_free();
		drawer = nullptr;
	}
}

SpxLayerSorter::~SpxLayerSorter() {
	_clear_drawer();
}

void LayerSorterDebugDrawer::_bind_methods() {
}

void LayerSorterDebugDrawer::_notification(int p_what) {
	if (p_what == NOTIFICATION_READY) {
		_ready();
	}

	if (p_what == NOTIFICATION_DRAW) {
		_draw();
	}

	if (p_what == NOTIFICATION_EXIT_TREE) {
		_exit_tree();
	}
}

void LayerSorterDebugDrawer::_ready() {
	set_process_input(true);
	set_z_index(1000);
	set_z_as_relative(false);
	font = ThemeDB::get_singleton()->get_default_theme()->get_default_font();
}

void LayerSorterDebugDrawer::_draw() {
	if (!sorter) {
		return;
	}

	if (!font.is_valid()) {
		print_error("Font is not valid!!!");
		return;
	}

	auto draw_sort_group = [&](const std::vector<SortInfo> &arr,
								   const Color &dot_color,
								   const Color &text_color,
								   const String &label_prefix) {
		for (size_t i = 0; i < arr.size(); ++i) {
			const auto &s = arr[i];
			if (!s.sortable || !s.sortable->is_node_valid()) {
				continue;
			}

			draw_circle(s.pos, 8, dot_color);
			draw_string(font, s.pos + Vector2(6, -2),
					vformat("%s:%d", label_prefix, (int64_t)i),
					HORIZONTAL_ALIGNMENT_LEFT, -1, 24, text_color);

			//if (i > 0) draw_line(arr[i - 1].pos, s.pos, Color(0.4, 0.4, 0.4, 0.4), 1);
		}
	};

	draw_sort_group(sorter->get_static_sorted(), Color(0.4, 0.8, 1), Color(0.6, 0.9, 1), "S");
	draw_sort_group(sorter->get_dynamic_sorted(), Color(1, 0.4, 0.4), Color(1, 0.6, 0.6), "D");

	draw_rect(sorter->get_screen_rect(), Color(0, 0, 1, 1), false, 4);
}

void LayerSorterDebugDrawer::_exit_tree() {
	if (sorter) {
		sorter->unlink_drawer();
	}
}
