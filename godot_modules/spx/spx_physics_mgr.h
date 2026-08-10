/**************************************************************************/
/*  spx_physics_mgr.h                                                     */
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

#ifndef SPX_PHYSICS_MGR_H
#define SPX_PHYSICS_MGR_H

#include "gdextension_spx_ext.h"
#include "spx_base_mgr.h"

class SpxPhysicsDefine {
private:
	static inline GdFloat global_gravity = 1.0;
	static inline GdFloat global_friction = 1.0;
	static inline GdFloat global_air_drag = 1.0;

public:
	static void set_global_gravity(GdFloat gravity);
	static GdFloat get_global_gravity();
	static void set_global_friction(GdFloat friction);
	static GdFloat get_global_friction();
	static void set_global_air_drag(GdFloat air_drag);
	static GdFloat get_global_air_drag();
};

class SpxRaycastInfo {
public:
	GdBool collide;
	GdObj sprite_gid;
	GdVec2 position;
	GdVec2 normal;

public:
	SpxRaycastInfo() = default;
	~SpxRaycastInfo() = default;
	GdArray ToArray();
};

enum class ColliderType {
	NONE = 0,
	AUTO = 1,
	CIRCLE = 2,
	RECT = 3,
	CAPSULE = 4,
	POLYGON = 5
};

enum BoundaryType {
	BOUND_LEFT = 1 << 0,
	BOUND_TOP = 1 << 1,
	BOUND_RIGHT = 1 << 2,
	BOUND_BOTTOM = 1 << 3
};

class SpxPhysicsMgr : public SpxBaseMgr {
	SPXCLASS(SpxPhysicsMgr, SpxBaseMgr)

private:
	GdArray _check_collision(RID shape, GdVec2 pos, GdInt collision_mask);
	SpxRaycastInfo _raycast(GdVec2 from, GdVec2 to, GdArray ignore_sprites, GdInt collision_mask, GdBool collide_with_areas, GdBool collide_with_bodies);

	// Internal boundary check helpers
	GdInt _check_touched_boundaries(GdObj obj, GdBool use_stage_limits);
	GdBool _check_touched_boundary(GdObj obj, GdInt board_type, GdBool use_stage_limits);
	GdInt _check_nearest_touched_boundary(GdObj obj, GdBool use_stage_limits);

public:
	bool is_collision_by_pixel;
	void on_awake() override;
	void on_reset(int reset_code) override;

public:
	virtual ~SpxPhysicsMgr() = default; // Added virtual destructor to fix -Werror=non-virtual-dtor
	SPX_API GdObj raycast(GdVec2 from, GdVec2 to, GdInt collision_mask);
	SPX_API GdBool check_collision(GdVec2 from, GdVec2 to, GdInt collision_mask, GdBool collide_with_areas, GdBool collide_with_bodies);
	SPX_API GdInt check_touched_camera_boundaries(GdObj obj);
	SPX_API GdBool check_touched_camera_boundary(GdObj obj, GdInt board_type);
	SPX_API GdInt check_nearest_touched_camera_boundary(GdObj obj);

	// Stage boundary check functions
	SPX_API GdInt check_touched_stage_boundaries(GdObj obj);
	SPX_API GdBool check_touched_stage_boundary(GdObj obj, GdInt board_type);
	SPX_API GdInt check_nearest_touched_stage_boundary(GdObj obj);
	SPX_API void set_collision_system_type(GdBool is_collision_by_alpha);

	// configs
	SPX_API void set_global_gravity(GdFloat gravity);
	SPX_API GdFloat get_global_gravity();
	SPX_API void set_global_friction(GdFloat friction);
	SPX_API GdFloat get_global_friction();
	SPX_API void set_global_air_drag(GdFloat air_drag);
	SPX_API GdFloat get_global_air_drag();

	// check collision
	SPX_API GdArray check_collision_rect(GdVec2 pos, GdVec2 size, GdInt collision_mask);
	SPX_API GdArray check_collision_circle(GdVec2 pos, GdFloat radius, GdInt collision_mask);
	SPX_API GdArray raycast_with_details(GdVec2 from, GdVec2 to, GdArray ignore_sprites, GdInt collision_mask, GdBool collide_with_areas, GdBool collide_with_bodies);
};

#endif // SPX_PHYSICS_MGR_H
