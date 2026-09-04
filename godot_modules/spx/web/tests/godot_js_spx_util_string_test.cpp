#include <algorithm>
#include <cassert>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <limits>
#include <new>
#include <string>
#include <string_view>
#include <unordered_map>
#include <unordered_set>
#include <utility>
#include <vector>

#define NOT_GODOT_ENGINE

using real_t = float;

struct Vector2 {
    union {
        real_t x = 0;
        real_t width;
    };
    union {
        real_t y = 0;
        real_t height;
    };
};

struct Vector3 {
    real_t x = 0;
    real_t y = 0;
    real_t z = 0;
};

struct Vector4 {
    real_t x = 0;
    real_t y = 0;
    real_t z = 0;
    real_t w = 0;
};

struct Color {
    real_t r = 0;
    real_t g = 0;
    real_t b = 0;
    real_t a = 0;
};

struct Rect2 {
    Vector2 position;
    Vector2 size;
};

inline void print_error(const char *) {
}

static std::unordered_set<void *> gdspxTestAllocations;

static void *gdspx_test_malloc(size_t size) {
    void *ptr = std::malloc(size);
    if (ptr != nullptr) {
        assert(gdspxTestAllocations.insert(ptr).second);
    }
    return ptr;
}

static void gdspx_test_free(void *ptr) {
    if (ptr == nullptr) {
        return;
    }
    assert(gdspxTestAllocations.erase(ptr) == 1);
    std::free(ptr);
}

#define malloc gdspx_test_malloc
#define free gdspx_test_free
#include "../godot_js_spx_util.cpp"
#undef free
#undef malloc

static const void *invalid_nested_pointer(uintptr_t value) {
    return reinterpret_cast<const void *>(value);
}

static void test_uncached_nested_pointer_tamper() {
    const std::string input(GDSPX_STRING_CACHE_MAX_LEN + 1, 'x');
    GdString *wrapper = gdspx_new_string(input.data(), static_cast<uint32_t>(input.size()));
    assert(wrapper != nullptr);

    const char *trusted = gdspx_get_string(wrapper);
    assert(trusted != nullptr);
    assert(gdspx_get_string_len(wrapper) == static_cast<int32_t>(input.size()));
    assert(std::memcmp(trusted, input.data(), input.size()) == 0);

    char *mutable_trusted = const_cast<char *>(trusted);
    mutable_trusted[input.size()] = '!';
    GdString value = trusted;
    assert(gdspx_get_string_len(wrapper) == static_cast<int32_t>(input.size()));
    assert(!gdspx_get_string_value(wrapper, &value));
    assert(value == nullptr);
    assert(gdspx_get_string(wrapper) == nullptr);
    mutable_trusted[input.size()] = '\0';

    *wrapper = invalid_nested_pointer(1);
    value = trusted;
    assert(!gdspx_validate_string_wrapper(wrapper));
    assert(!gdspx_get_string_value(wrapper, &value));
    assert(value == nullptr);
    assert(gdspx_get_string(wrapper) == nullptr);
    assert(gdspx_get_string_len(wrapper) == 0);

    gdspx_free_string(wrapper);
    assert(gdspxTestAllocations.count(const_cast<char *>(trusted)) == 0);
    gdspx_free_string(wrapper);
}

static void test_cached_nested_pointer_tamper() {
    static constexpr char input[] = "cached string";
    GdString *first = gdspx_new_string(input, sizeof(input) - 1);
    GdString *second = gdspx_new_string(input, sizeof(input) - 1);
    assert(first != nullptr && second != nullptr);

    const char *trusted = gdspx_get_string(first);
    assert(trusted != nullptr && gdspx_get_string(second) == trusted);
    CachedGdStringEntry *cached = find_cached_gdstring_by_ptr(trusted);
    assert(cached != nullptr && cached->refcount == 2);

    const_cast<char *>(trusted)[0] = 'X';
    GdString *replacement = gdspx_new_string(input, sizeof(input) - 1);
    assert(replacement != nullptr);
    assert(gdspx_get_string(replacement) != trusted);
    assert(std::memcmp(gdspx_get_string(replacement), input, sizeof(input) - 1) == 0);
    gdspx_free_string(replacement);
    const_cast<char *>(trusted)[0] = input[0];

    *first = invalid_nested_pointer(3);
    gdspx_free_string(first);
    assert(cached->refcount == 1);
    assert(gdspx_get_string_len(second) == static_cast<int32_t>(sizeof(input) - 1));
    assert(std::memcmp(gdspx_get_string(second), input, sizeof(input) - 1) == 0);

    gdspx_free_string(second);
    assert(cached->refcount == 0);
}

static void test_manager_bind_and_tamper() {
    GdString *wrapper = gdspx_alloc_string();
    assert(wrapper != nullptr && gdspx_prepare_string_wrapper(wrapper));

    char *result = static_cast<char *>(gdspx_test_malloc(8));
    std::memcpy(result, "manager", 8);
    assert(gdspx_bind_string_wrapper(wrapper, result));
    assert(gdspx_get_string(wrapper) == result);
    assert(gdspx_get_string_len(wrapper) == 7);

    *wrapper = invalid_nested_pointer(5);
    gdspx_free_string(wrapper);
    assert(gdspxTestAllocations.count(result) == 0);
}

static void test_manager_bind_failure_cleanup() {
    GdString *wrapper = gdspx_alloc_string();
    assert(wrapper != nullptr && gdspx_prepare_string_wrapper(wrapper));

    char *result = static_cast<char *>(gdspx_test_malloc(7));
    std::memcpy(result, "failed", 7);
    *wrapper = invalid_nested_pointer(7);
    assert(!gdspx_bind_string_wrapper(wrapper, result));
    assert(gdspxTestAllocations.count(result) == 0);

    gdspx_free_string(wrapper);
    gdspx_free_string(wrapper);
}

static void clear_string_cache() {
    while (!gdspxStringCache.empty()) {
        assert(gdspxStringCache.front().refcount == 0);
        remove_cached_gdstring_at(0);
    }
}

int main() {
    test_uncached_nested_pointer_tamper();
    test_cached_nested_pointer_tamper();
    test_manager_bind_and_tamper();
    test_manager_bind_failure_cleanup();
    clear_string_cache();
    assert(gdspxStringSnapshots.empty());
    assert(gdspxTestAllocations.empty());
    return 0;
}
