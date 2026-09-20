// Exercise ABI memory without linking Godot, SceneTree, or SpxEngine.
#include <cassert>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <limits>
#include <string>
#include <unordered_set>

#define NOT_GODOT_ENGINE
using real_t = float;
struct Vector2 { real_t x, y; };
struct Vector3 { real_t x, y, z; };
struct Vector4 { real_t x, y, z, w; };
struct Color { real_t r, g, b, a; };
struct Rect2 { Vector2 position, size; };

// Only the UTF-8 conversion used by the memory helper is needed here.
class String {
	std::string value;
public:
	String(const char *text) : value(text) {}
	struct UTF8 {
		const std::string &value;
		size_t size() const { return value.size() + 1; }
		const char *get_data() const { return value.c_str(); }
	};
	UTF8 utf8() const { return { value }; }
};

static std::unordered_set<void *> allocations;
static void *test_malloc(size_t size) {
	void *ptr = std::malloc(size);
	assert(ptr != nullptr);
	assert(allocations.insert(ptr).second);
	return ptr;
}
static void *test_calloc(size_t count, size_t size) {
	void *ptr = std::calloc(count, size);
	assert(ptr != nullptr);
	assert(allocations.insert(ptr).second);
	return ptr;
}
static void test_free(void *ptr) {
	if (ptr != nullptr) {
		assert(allocations.erase(ptr) == 1);
		std::free(ptr);
	}
}

#define malloc test_malloc
#define calloc test_calloc
#define free test_free
#include "../../spx_abi.cpp"
#undef free
#undef calloc
#undef malloc
#include "../../spx_callback_defaults.gen.h"

int main() {
	for (const char *text : { "", "hello", "中文 UTF-8" }) {
		GdString result = SpxAbi::to_return_cstr(text);
		assert(std::strcmp(static_cast<const char *>(result), text) == 0);
		SpxAbi::free_return_cstr(result);
	}
	SpxAbi::free_return_cstr(nullptr);
	SpxAbi::free_array(nullptr);
	assert(allocations.empty());

	for (int32_t type : { GD_ARRAY_TYPE_INT64, GD_ARRAY_TYPE_FLOAT, GD_ARRAY_TYPE_BOOL,
			GD_ARRAY_TYPE_STRING, GD_ARRAY_TYPE_BYTE, GD_ARRAY_TYPE_GDOBJ }) {
		GdArray empty = SpxAbi::create_array(type, 0);
		assert(empty && empty->size == 0 && empty->type == type && !empty->data);
		SpxAbi::free_array(empty);
		GdArray array = SpxAbi::create_array(type, 2);
		assert(array && array->size == 2 && array->data);
		if (type == GD_ARRAY_TYPE_STRING) {
			assert(*SpxAbi::get_array<GdString>(array, 0) == nullptr);
			SpxAbi::set_array<GdString>(array, 1, SpxAbi::to_return_cstr("owned child"));
		}
		SpxAbi::free_array(array);
		assert(allocations.empty());
	}
	assert(!SpxAbi::create_array(GD_ARRAY_TYPE_UNKNOWN, 0));
	assert(!SpxAbi::create_array(GD_ARRAY_TYPE_FLOAT, -1));
	assert(!SpxAbi::create_array(GD_ARRAY_TYPE_FLOAT, std::numeric_limits<int32_t>::max()));
	GdArray array = SpxAbi::create_array(GD_ARRAY_TYPE_GDOBJ, 1);
	SpxAbi::set_array<GdObj>(array, 0, std::numeric_limits<int64_t>::max());
	assert(*SpxAbi::get_array<GdInt>(array, 0) == std::numeric_limits<int64_t>::max());
	assert(!SpxAbi::get_array<float>(array, 0));
	assert(!SpxAbi::get_array<GdObj>(array, -1));
	assert(!SpxAbi::get_array<GdObj>(array, 1));
	SpxAbi::free_array(array);
	assert(allocations.empty());

	// Defaults also require no initialized engine and retain their exact ABI types.
	SpxCallbackInfo callbacks = get_default_spx_callbacks();
	callbacks.func_on_engine_start();
	callbacks.func_on_engine_reset();
	callbacks.func_on_engine_destroyed();
	callbacks.func_on_trigger_enter(1, 2);
	callbacks.func_on_ui_text_changed(1, "changed");
}
