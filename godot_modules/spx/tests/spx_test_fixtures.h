/**************************************************************************/
/*  spx_test_fixtures.h                                                 */
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

#ifndef SPX_TEST_FIXTURES_H
#define SPX_TEST_FIXTURES_H

#include "../spx.h"
#include "../spx_abi.h"
#include "../spx_engine.h"
#include "core/io/dir_access.h"
#include "core/io/file_access.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "servers/audio_server.h"
#include "tests/test_macros.h"
#include "tests/test_utils.h"
#include <memory>

namespace TestSpx {

using Array = std::unique_ptr<GdArrayInfo, decltype(&SpxAbi::free_array)>;
inline Array take_array(GdArray value) {
	return Array(value, SpxAbi::free_array);
}
inline String take_string(GdString value) {
	String result = SpxStr(value);
	SpxAbi::free_return_cstr(value);
	return result;
}

struct TempDirectory {
	TempDirectory(const TempDirectory &) = delete;
	TempDirectory &operator=(const TempDirectory &) = delete;

	String path;

	explicit TempDirectory(const String &name) {
		path = TestUtils::get_temp_path(name + "_" + itos(OS::get_singleton()->get_process_id()) + "_" + itos(OS::get_singleton()->get_ticks_usec()));
		REQUIRE(DirAccess::make_dir_recursive_absolute(path) == OK);
	}

	~TempDirectory() {
		Ref<DirAccess> directory = DirAccess::open(path);
		if (directory.is_valid()) {
			directory->erase_contents_recursive();
			DirAccess::remove_absolute(path);
		}
	}

	String file(const String &name) const { return path.path_join(name); }

	void write(const String &name, const String &contents) const {
		Ref<FileAccess> output = FileAccess::open(file(name), FileAccess::WRITE);
		REQUIRE(output.is_valid());
		output->store_string(contents);
	}
};

// Use the same manager lifecycle as a running project; always tear it down,
// including when a REQUIRE assertion aborts the test.
struct EngineScene {
	enum Mode { MANAGERS_ONLY,
		FULL_RUNTIME };

	EngineScene(const EngineScene &) = delete;
	EngineScene &operator=(const EngineScene &) = delete;

	Node *root = nullptr;
	SpxEngine *engine = nullptr;

	const Mode mode;

	explicit EngineScene(Mode p_mode = MANAGERS_ONLY) :
			mode(p_mode) {
		REQUIRE_FALSE(SpxEngine::is_initialized());
		if (AudioServer::get_singleton() == nullptr) {
			AudioDriverManager::initialize(AudioDriverManager::get_driver_count() - 1);
			memnew(AudioServer)->init();
		}
		SceneTree *tree = SceneTree::get_singleton();
		REQUIRE(tree != nullptr);
		SpxEngine::register_callbacks(nullptr);
		engine = SpxEngine::get_singleton();
		if (mode == FULL_RUNTIME) {
			Spx::on_start(tree);
			root = engine->get_spx_root();
		} else {
			root = memnew(Node);
			tree->get_root()->add_child(root);
			engine->set_root_node(tree, root);
			engine->on_awake();
		}
	}

	~EngineScene() {
		if (mode == FULL_RUNTIME) {
			Spx::on_destroy();
		} else {
			SpxEngine::shutdown();
		}
		memdelete(root);
	}
};

} // namespace TestSpx

#endif // SPX_TEST_FIXTURES_H
