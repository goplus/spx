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

#include "stubs/godot_types.h"

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

static void test_borrowed_native_arrays() {
    auto *values = static_cast<int64_t *>(gdspx_test_malloc(2 * sizeof(int64_t)));
    values[0] = -1;
    values[1] = int64_t(1) << 40;
    GdArray *wrapper = gdspx_borrow_array(reinterpret_cast<uint8_t *>(values), 16, 2, GD_ARRAY_TYPE_INT64);
    assert(wrapper != nullptr);
    const GdArrayInfo *info = gdspx_get_array_info(wrapper);
    assert(info != nullptr && info->data == values && info->size == 2);
    // Borrowing must not copy or own the caller's storage.
    values[0] = 7;
    assert(static_cast<int64_t *>(info->data)[0] == 7);
    (*wrapper)->size = 3;
    assert(gdspx_get_array_info(wrapper) == nullptr);
    gdspx_free_array(wrapper);
    assert(gdspxTestAllocations.count(values) == 1);
    gdspx_test_free(values);

    for (int type = GD_ARRAY_TYPE_INT64; type <= GD_ARRAY_TYPE_GDOBJ; ++type) {
        wrapper = gdspx_borrow_array(nullptr, 0, 0, type);
        assert(wrapper != nullptr);
        info = gdspx_get_array_info(wrapper);
        assert(info != nullptr && info->size == 0 && info->type == type && info->data == nullptr);
        gdspx_free_array(wrapper);
    }

    alignas(8) uint8_t bad[16] = {2};
    assert(gdspx_borrow_array(bad, 1, 1, GD_ARRAY_TYPE_BOOL) == nullptr);
    assert(gdspx_borrow_array(bad, 7, 1, GD_ARRAY_TYPE_INT64) == nullptr);
    assert(gdspx_borrow_array(bad + 1, 8, 1, GD_ARRAY_TYPE_INT64) == nullptr);
    assert(gdspx_borrow_array(bad, 0, -1, GD_ARRAY_TYPE_BYTE) == nullptr);
    assert(gdspx_borrow_array(bad, 0, 0, 99) == nullptr);
}

static void test_borrowed_string_arrays() {
    uint8_t bytes[] = {16, 0, 0, 0, 2, 0, 0, 0, 19, 0, 0, 0, 0, 0, 0, 0, 'h', 'i', 0, 0};
    GdArray *wrapper = gdspx_borrow_array(bytes, sizeof(bytes), 2, GD_ARRAY_TYPE_STRING);
    assert(wrapper != nullptr);
    const GdArrayInfo *info = gdspx_get_array_info(wrapper);
    assert(info != nullptr);
    auto **strings = static_cast<char **>(info->data);
    assert(strings[0] == reinterpret_cast<char *>(bytes + 16));
    assert(std::strcmp(strings[0], "hi") == 0 && std::strcmp(strings[1], "") == 0);
    bytes[18] = '!';
    assert(gdspx_get_array_info(wrapper) == nullptr);
    gdspx_free_array(wrapper); // Must not free bytes or either borrowed string.
    assert(gdspx_borrow_array(bytes, sizeof(bytes), 2, GD_ARRAY_TYPE_STRING) == nullptr);
    bytes[18] = 0;
    bytes[0] = 0;
    assert(gdspx_borrow_array(bytes, sizeof(bytes), 2, GD_ARRAY_TYPE_STRING) == nullptr);
}

static void test_owned_array_result() {
    auto *info = static_cast<GdArrayInfo *>(gdspx_test_malloc(sizeof(GdArrayInfo)));
    info->size = 2;
    info->type = GD_ARRAY_TYPE_FLOAT;
    info->data = gdspx_test_malloc(2 * sizeof(float));
    assert(gdspx_register_array_info(info));
    GdArray *wrapper = gdspx_alloc_array();
    *wrapper = info;
    assert(gdspx_bind_array_wrapper(wrapper));
    assert(gdspx_get_array_info(wrapper) == info);
    void *data = info->data;
    gdspx_free_array(wrapper);
    assert(gdspxTestAllocations.count(data) == 0 && gdspxTestAllocations.count(info) == 0);
}

static void test_array_owner_binding() {
    auto *info = static_cast<GdArrayInfo *>(gdspx_test_malloc(sizeof(GdArrayInfo)));
    info->size = 1;
    info->type = GD_ARRAY_TYPE_FLOAT;
    void *data = info->data = gdspx_test_malloc(sizeof(float));
    assert(gdspx_register_array_info(info));
    assert(gdspx_register_array_info(info));

    GdArray *owner = gdspx_alloc_array();
    GdArray *alias = gdspx_alloc_array();
    *owner = *alias = info;
    assert(gdspx_bind_array_wrapper(owner));
    assert(gdspx_bind_array_wrapper(owner));
    assert(!gdspx_bind_array_wrapper(alias));
    assert(!gdspx_validate_array_wrapper(alias));
    assert(!gdspx_register_array_info(info));
    assert(!gdspx_release_array_info(info));
    gdspx_free_array(alias);
    gdspx_free_array(alias);
    assert(gdspx_validate_array_wrapper(owner));
    assert(gdspxTestAllocations.count(info) == 1 && gdspxTestAllocations.count(data) == 1);

    gdspx_free_array(owner);
    gdspx_free_array(owner);
    assert(gdspxTestAllocations.count(info) == 0 && gdspxTestAllocations.count(data) == 0);
    GdArray *reused = gdspx_alloc_array();
    assert(*reused == nullptr && gdspx_prepare_array_wrapper(reused));
    assert(gdspx_bind_array_wrapper(reused));
    gdspx_free_array(reused);
}

static void test_owned_string_array_lifecycle() {
    // Managers fill string slots after registering the descriptor. Both release
    // before binding and sealing into a wrapper must capture those final slots.
    for (int mode = 0; mode < 5; ++mode) {
        auto *info = static_cast<GdArrayInfo *>(gdspx_test_malloc(sizeof(GdArrayInfo)));
        auto **slots = static_cast<char **>(gdspx_test_malloc(2 * sizeof(char *)));
        slots[0] = slots[1] = nullptr;
        info->size = 2;
        info->type = GD_ARRAY_TYPE_STRING;
        info->data = slots;
        assert(gdspx_register_array_info(info));
        char *value = static_cast<char *>(gdspx_test_malloc(6));
        std::memcpy(value, "value", 6);
        slots[0] = slots[1] = value; // Aliased slots must release the allocation once.
        assert(gdspx_validate_array_info(info));

        if (mode == 0) {
            assert(gdspx_release_array_info(info));
            assert(!gdspx_release_array_info(info));
        } else {
            GdArray *wrapper = gdspx_alloc_array();
            *wrapper = info;
            assert(gdspx_bind_array_wrapper(wrapper));
            assert(gdspx_validate_array_wrapper(wrapper));
            assert(!gdspx_release_array_info(info));
            if (mode == 2) {
                info->data = const_cast<void *>(invalid_nested_pointer(11));
            } else if (mode == 3) {
                slots[0] = static_cast<char *>(const_cast<void *>(invalid_nested_pointer(13)));
            } else if (mode == 4) {
                *wrapper = static_cast<GdArrayInfo *>(const_cast<void *>(invalid_nested_pointer(15)));
            }
            if (mode != 1) {
                assert(!gdspx_validate_array_wrapper(wrapper));
            }
            // Even tampered live pointers must release only the trusted data.
            gdspx_free_array(wrapper);
            gdspx_free_array(wrapper);
        }
        assert(gdspxTestAllocations.count(info) == 0);
        assert(gdspxTestAllocations.count(slots) == 0);
        assert(gdspxTestAllocations.count(value) == 0);
    }
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
    test_borrowed_native_arrays();
    test_borrowed_string_arrays();
    test_owned_array_result();
    test_array_owner_binding();
    test_owned_string_array_lifecycle();
    assert(gdspxArraySnapshots.empty() && gdspxArrayBindings.empty());
    clear_string_cache();
    assert(gdspxStringSnapshots.empty());
    assert(gdspxTestAllocations.empty());
    return 0;
}
