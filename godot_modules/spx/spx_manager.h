/**************************************************************************/
/*  spx_manager.h                                                        */
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

#ifndef SPX_MANAGER_H
#define SPX_MANAGER_H

#include "spx_abi.h"
#include "spx_mgr_access.h"

class Node;
class Window;
class SceneTree;

// Managers opt into lifecycle hooks; scene nodes belong to their actual users.
class SpxManager {
protected:
	GdInt get_unique_id();
	SceneTree *get_tree();
	Window *get_root();
	Node *get_spx_root();

public:
	virtual ~SpxManager() = default;
	virtual void on_awake() {}
	virtual void on_start() {}
	virtual void on_update(float delta) {}
	virtual void on_fixed_update(float delta) {}
	virtual void on_destroy() {}
	virtual void on_reset(int reset_code) {}
	virtual void on_exit(int exit_code) {}
	virtual void on_pause() {}
	virtual void on_resume() {}
};

#endif // SPX_MANAGER_H
