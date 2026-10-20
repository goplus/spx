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
static int allocations_before_failure = -1;
static bool fail_allocation() {
	if (allocations_before_failure < 0) {
		return false;
	}
	return allocations_before_failure-- == 0;
}
static void *test_malloc(size_t size) {
	if (fail_allocation()) {
		return nullptr;
	}
	void *ptr = std::malloc(size);
	assert(ptr != nullptr);
	assert(allocations.insert(ptr).second);
	return ptr;
}
static void *test_calloc(size_t count, size_t size) {
	if (fail_allocation()) {
		return nullptr;
	}
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
#include "../../spx_batch_validation.h"

static void test_batch_numeric_boundaries() {
	const float lower = static_cast<float>(std::numeric_limits<int>::min());
	const float upper = std::nextafter(static_cast<float>(std::numeric_limits<int>::max()), 0.0f);
	for (float value : { lower, upper, -0.0f, 1.0f }) {
		int decoded = 7;
		assert(SpxBatchValidation::decode_int(value, decoded));
		assert(static_cast<double>(decoded) == static_cast<double>(value));
		assert(SpxBatchValidation::decode_nonnegative_count(value, decoded) == (value >= 0));
	}
	for (float value : { std::nextafter(lower, -std::numeric_limits<float>::infinity()),
			static_cast<float>(std::numeric_limits<int>::max()),
			std::numeric_limits<float>::infinity(), -std::numeric_limits<float>::infinity(),
			std::numeric_limits<float>::quiet_NaN(), 0.5f, -0.5f }) {
		int decoded = 7;
		assert(!SpxBatchValidation::decode_int(value, decoded));
		assert(decoded == 7);
	}
	assert(SpxBatchValidation::all_finite(nullptr, 0));
	for (int lane = 0; lane < 3; ++lane) {
		float values[] = { 0.0f, -1.0f, 3.5f };
		assert(SpxBatchValidation::all_finite(values, 3));
		values[lane] = std::numeric_limits<float>::quiet_NaN();
		assert(SpxBatchValidation::all_finite(values, lane));
		assert(!SpxBatchValidation::all_finite(values, lane + 1));
	}
}

int main() {
	test_batch_numeric_boundaries();
	// Failed string, array header and payload allocations must retain no ownership.
	allocations_before_failure = 0;
	assert(!SpxAbi::to_return_cstr("unallocated"));
	assert(allocations.empty());
	for (int failure : { 0, 1 }) {
		allocations_before_failure = failure;
		assert(!SpxAbi::create_array(GD_ARRAY_TYPE_STRING, 2));
		assert(allocations.empty());
	}
	allocations_before_failure = -1;
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
	// Invalid writes leave the valid element unchanged.
	SpxAbi::set_array<float>(array, 0, 2.0f);
	SpxAbi::set_array<GdObj>(array, -1, 7);
	SpxAbi::set_array<GdObj>(array, 1, 7);
	assert(*SpxAbi::get_array<GdObj>(array, 0) == std::numeric_limits<int64_t>::max());
	SpxAbi::free_array(array);
	assert(allocations.empty());
	assert(!SpxAbi::get_array<float>(nullptr, 0));
	SpxAbi::set_array<float>(nullptr, 0, 1.0f);
	float value = 3.0f;
	GdArrayInfo malformed = {};
	malformed.data = &value;
	malformed.type = GD_ARRAY_TYPE_FLOAT;
	// Test malformed metadata without passing borrowed storage to free_array.
	for (int32_t size : { -1, 0, std::numeric_limits<int32_t>::max() }) {
		malformed.size = size;
		assert(!SpxAbi::get_array<float>(&malformed, 0));
		SpxAbi::set_array<float>(&malformed, 0, 9.0f);
		assert(value == 3.0f);
	}
	malformed.size = 1;
	malformed.data = nullptr;
	assert(!SpxAbi::get_array<float>(&malformed, 0));

	// Defaults also require no initialized engine and retain their exact ABI types.
	SpxCallbackInfo callbacks = get_default_spx_callbacks();
	callbacks.func_on_engine_start();
	callbacks.func_on_engine_reset();
	callbacks.func_on_engine_destroyed();
	callbacks.func_on_trigger_enter(1, 2);
	callbacks.func_on_ui_text_changed(1, "changed");
}
