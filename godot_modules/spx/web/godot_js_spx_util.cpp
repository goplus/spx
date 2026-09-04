#include "../gdextension_spx_ext.h"
#include "core/extension/gdextension.h"

#include "godot_js_spx_util.h"
#include <cstdlib>
#include <cstdint>
#include <cstring>
#include <limits>
#include <string>
#include <unordered_map>
#include <unordered_set>
#include <utility>
#include <vector>
#include <emscripten/emscripten.h>

static ObjectPool<GdVec2> vec2Pool(100);
static ObjectPool<GdString> stringPool(100);
static ObjectPool<GdObj> objPool(100);
static ObjectPool<GdInt> intPool(100);
static ObjectPool<GdFloat> floatPool(100);
static ObjectPool<GdBool> boolPool(100);
static ObjectPool<GdVec3> vec3Pool(100);
static ObjectPool<GdVec4> vec4Pool(100);
static ObjectPool<GdColor> colorPool(100);
static ObjectPool<GdRect2> rect2Pool(100);
static ObjectPool<GdArray> arrayPool(100);

struct CachedGdStringEntry {
    char *data = nullptr;
    uint32_t len = 0;
    uint64_t refcount = 0;
    uint64_t last_used_tick = 0;
    std::string value;
};

static constexpr size_t GDSPX_STRING_CACHE_MAX_ENTRIES = 128;
static constexpr uint32_t GDSPX_STRING_CACHE_MAX_LEN = 256;
static std::vector<CachedGdStringEntry> gdspxStringCache;
static std::unordered_map<std::string, size_t> gdspxStringCacheByValue;
static std::unordered_map<const char *, size_t> gdspxStringCacheByPtr;
static uint64_t gdspxStringCacheTick = 0;

static CachedGdStringEntry *find_cached_gdstring_by_value(const char *str, uint32_t len);
static CachedGdStringEntry *find_cached_gdstring_by_ptr(const char *ptr);

namespace {

constexpr size_t GDSPX_MAX_STRING_BYTES = 256 * 1024 * 1024;
constexpr size_t GDSPX_ARRAY_HEADER_SIZE = 8;
// Bound counts before allocation.
constexpr int32_t GDSPX_MAX_ARRAY_ELEMENTS = 16 * 1024 * 1024;
// Keep all serialized payloads bounded.
constexpr size_t GDSPX_MAX_ARRAY_BYTES = 256 * 1024 * 1024;

static bool checked_array_bytes(int32_t count, size_t element_size, size_t &r_bytes) {
    if (count < 0 || count > GDSPX_MAX_ARRAY_ELEMENTS || element_size == 0) {
        return false;
    }

    const size_t count_size = static_cast<size_t>(count);
    if (count_size > std::numeric_limits<size_t>::max() / element_size) {
        return false;
    }
    r_bytes = count_size * element_size;
    return true;
}

static bool checked_add_size(size_t left, size_t right, size_t &r_sum) {
    if (left > std::numeric_limits<size_t>::max() - right) {
        return false;
    }
    r_sum = left + right;
    return true;
}

static bool array_element_size(int32_t type, size_t &r_element_size) {
    switch (type) {
        case GD_ARRAY_TYPE_INT64:
        case GD_ARRAY_TYPE_GDOBJ:
            r_element_size = sizeof(int64_t);
            return true;
        case GD_ARRAY_TYPE_FLOAT:
            r_element_size = sizeof(float);
            return true;
        case GD_ARRAY_TYPE_BOOL:
        case GD_ARRAY_TYPE_BYTE:
            r_element_size = sizeof(uint8_t);
            return true;
        default:
            return false;
    }
}

struct GdArrayStringSlotSnapshot {
    char *ptr = nullptr;
    size_t len = 0;
};

// Trusted array metadata and ownership.
struct GdArrayMetadataSnapshot {
    GdArrayInfo *info = nullptr;
    int32_t size = 0;
    int32_t type = GD_ARRAY_TYPE_UNKNOWN;
    void *data = nullptr;
    size_t data_bytes = 0;
    size_t serialized_data_bytes = 0;
    std::vector<GdArrayStringSlotSnapshot> string_slots;
};

// Private bindings prevent forged wrapper metadata.
static std::unordered_map<GdArray *, GdArrayInfo *> gdspxArrayBindings;
static std::unordered_map<GdArrayInfo *, GdArrayMetadataSnapshot> gdspxArraySnapshots;
static std::unordered_map<GdArrayInfo *, GdArray *> gdspxArrayOwners;

enum class GdStringReleaseKind {
    NONE,
    MALLOC,
    CACHE,
};

// Trusted string ownership and length.
struct GdStringSnapshot {
    const char *ptr = nullptr;
    uint32_t len = 0;
    GdStringReleaseKind release_kind = GdStringReleaseKind::NONE;
    bool bound = false;
};

static std::unordered_map<GdString *, GdStringSnapshot> gdspxStringSnapshots;

static size_t bounded_cstr_len(const char *str, size_t max_len) {
    if (str == nullptr) {
        return 0;
    }
    size_t len = 0;
    while (len < max_len && str[len] != '\0') {
        ++len;
    }
    return len;
}

static bool string_snapshot_matches_live(GdString *wrapper, const GdStringSnapshot &snapshot) {
    // Compare only; never trust the mutable value for access or release.
    return wrapper != nullptr && *wrapper == snapshot.ptr;
}

static void release_string_snapshot(const GdStringSnapshot &snapshot) {
    if (snapshot.ptr == nullptr) {
        return;
    }

    if (snapshot.release_kind == GdStringReleaseKind::CACHE) {
        // Use the snapshot pointer; referenced entries are not evicted.
        CachedGdStringEntry *cached = find_cached_gdstring_by_ptr(snapshot.ptr);
        if (cached != nullptr && cached->data == snapshot.ptr && cached->len == snapshot.len &&
                cached->refcount > 0) {
            cached->refcount -= 1;
        }
        return;
    }

    if (snapshot.release_kind == GdStringReleaseKind::MALLOC) {
        free(const_cast<char *>(snapshot.ptr));
    }
}

static bool make_manager_string_snapshot(GdString value, GdStringSnapshot &r_snapshot) {
    GdStringSnapshot snapshot;
    snapshot.bound = true;
    if (value == nullptr) {
        r_snapshot = snapshot;
        return true;
    }

    const char *ptr = static_cast<const char *>(value);
    const size_t len = bounded_cstr_len(ptr, GDSPX_MAX_STRING_BYTES + 1);
    if (len > GDSPX_MAX_STRING_BYTES || len > std::numeric_limits<uint32_t>::max()) {
        return false;
    }
    snapshot.ptr = ptr;
    snapshot.len = static_cast<uint32_t>(len);
    snapshot.release_kind = GdStringReleaseKind::MALLOC;
    r_snapshot = snapshot;
    return true;
}

static void discard_manager_string_result(GdString value, GdString *wrapper) {
    if (value == nullptr) {
        return;
    }
    // Manager results are malloc'ed; never free the wrapper itself.
    if (value != static_cast<GdString>(wrapper)) {
        free(const_cast<void *>(value));
    }
}

static bool make_array_snapshot(GdArrayInfo *info, GdArrayMetadataSnapshot &r_snapshot) {
    if (info == nullptr) {
        return false;
    }

    // Capture the header once; later checks compare against this snapshot.
    const int32_t size = info->size;
    const int32_t type = info->type;
    void *data = info->data;
    if (size < 0 || size > GDSPX_MAX_ARRAY_ELEMENTS) {
        return false;
    }

    GdArrayMetadataSnapshot snapshot;
    snapshot.info = info;
    snapshot.size = size;
    snapshot.type = type;
    snapshot.data = data;

    // Metadata and payload must not alias.
    if (data == info) {
        return false;
    }

    size_t element_size = 0;
    if (array_element_size(type, element_size)) {
        if (!checked_array_bytes(size, element_size, snapshot.data_bytes) ||
                snapshot.data_bytes > GDSPX_MAX_ARRAY_BYTES ||
                (size > 0 && data == nullptr) || (size == 0 && data != nullptr)) {
            return false;
        }
        snapshot.serialized_data_bytes = snapshot.data_bytes;
        r_snapshot = std::move(snapshot);
        return true;
    }

    if (type != GD_ARRAY_TYPE_STRING) {
        return false;
    }

    size_t slot_bytes = 0;
    if (!checked_array_bytes(size, sizeof(char *), slot_bytes) ||
            slot_bytes > GDSPX_MAX_ARRAY_BYTES || (size > 0 && data == nullptr) ||
            (size == 0 && data != nullptr)) {
        return false;
    }
    snapshot.data_bytes = slot_bytes;
    snapshot.string_slots.resize(static_cast<size_t>(size));
    char **strings = static_cast<char **>(data);
    for (int32_t i = 0; i < size; ++i) {
        char *str = strings[i];
        if (str == reinterpret_cast<char *>(info) || str == reinterpret_cast<char *>(data)) {
            return false;
        }
        snapshot.string_slots[static_cast<size_t>(i)].ptr = str;
        if (str != nullptr) {
            // Bound the scan; validation checks the terminator later.
            const size_t len = bounded_cstr_len(str, GDSPX_MAX_ARRAY_BYTES + 1);
            if (len > GDSPX_MAX_ARRAY_BYTES) {
                return false;
            }
            snapshot.string_slots[static_cast<size_t>(i)].len = len;
        }

        size_t encoded_length = 0;
        if (!checked_add_size(sizeof(uint32_t), snapshot.string_slots[static_cast<size_t>(i)].len,
                    encoded_length) ||
                encoded_length > GDSPX_MAX_ARRAY_BYTES - GDSPX_ARRAY_HEADER_SIZE ||
                snapshot.serialized_data_bytes >
                        GDSPX_MAX_ARRAY_BYTES - GDSPX_ARRAY_HEADER_SIZE - encoded_length ||
                !checked_add_size(snapshot.serialized_data_bytes, encoded_length,
                        snapshot.serialized_data_bytes)) {
            return false;
        }
    }

    r_snapshot = std::move(snapshot);
    return true;
}

static bool array_snapshot_matches_live(const GdArrayMetadataSnapshot &snapshot) {
    GdArrayInfo *info = snapshot.info;
    if (info == nullptr || info->size != snapshot.size || info->type != snapshot.type ||
            info->data != snapshot.data) {
        return false;
    }

    if (snapshot.type != GD_ARRAY_TYPE_STRING) {
        return true;
    }

    if (snapshot.size == 0) {
        return snapshot.data == nullptr;
    }
    if (snapshot.data == nullptr || snapshot.string_slots.size() != static_cast<size_t>(snapshot.size)) {
        return false;
    }

    char **strings = static_cast<char **>(snapshot.data);
    for (int32_t i = 0; i < snapshot.size; ++i) {
        const GdArrayStringSlotSnapshot &expected = snapshot.string_slots[static_cast<size_t>(i)];
        char *current = strings[i];
        if (current != expected.ptr) {
            return false;
        }
        if (current != nullptr) {
            // Keep the original terminator within the recorded length.
            if (bounded_cstr_len(current, expected.len + 1) != expected.len) {
                return false;
            }
        }
    }
    return true;
}

static bool array_header_matches_snapshot(const GdArrayMetadataSnapshot &snapshot) {
    return snapshot.info != nullptr && snapshot.info->size == snapshot.size &&
            snapshot.info->type == snapshot.type && snapshot.info->data == snapshot.data;
}

static void free_array_snapshot(const GdArrayMetadataSnapshot &snapshot) {
    std::unordered_set<void *> freed_allocations;
    if (snapshot.type == GD_ARRAY_TYPE_STRING) {
        for (const GdArrayStringSlotSnapshot &slot : snapshot.string_slots) {
            if (slot.ptr != nullptr && freed_allocations.insert(slot.ptr).second) {
                free(slot.ptr);
            }
        }
    }
    if (snapshot.data != nullptr && freed_allocations.insert(snapshot.data).second) {
        free(snapshot.data);
    }
    if (snapshot.info != nullptr && freed_allocations.insert(snapshot.info).second) {
        free(snapshot.info);
    }
}

static bool release_array_snapshot(GdArrayInfo *info, GdArray *expected_owner) {
    auto snapshot_it = gdspxArraySnapshots.find(info);
    if (snapshot_it == gdspxArraySnapshots.end()) {
        return false;
    }

    auto owner_it = gdspxArrayOwners.find(info);
    if (expected_owner == nullptr) {
        // Manager-side release is valid only before wrapper binding.
        if (owner_it != gdspxArrayOwners.end()) {
            return false;
        }

        // Refresh string slots while an array is still under construction.
        if (array_header_matches_snapshot(snapshot_it->second)) {
            GdArrayMetadataSnapshot current_snapshot;
            if (make_array_snapshot(info, current_snapshot)) {
                snapshot_it->second = std::move(current_snapshot);
            }
        }
    } else {
        if (owner_it == gdspxArrayOwners.end() || owner_it->second != expected_owner) {
            return false;
        }
        gdspxArrayBindings.erase(expected_owner);
        gdspxArrayOwners.erase(owner_it);
        if (arrayPool.is_active(expected_owner)) {
            *expected_owner = nullptr;
        }
    }

    GdArrayMetadataSnapshot snapshot = std::move(snapshot_it->second);
    gdspxArraySnapshots.erase(snapshot_it);
    free_array_snapshot(snapshot);
    return true;
}

// Release an unbound array using only trusted metadata.
static void dispose_unbound_array_info(GdArrayInfo *info) {
    if (info == nullptr) {
        return;
    }
    GdArrayMetadataSnapshot snapshot;
    if (make_array_snapshot(info, snapshot)) {
        free_array_snapshot(snapshot);
    } else {
        free(info);
    }
}

} // namespace

extern "C" bool gdspx_bind_array_wrapper(GdArray *wrapper) {
    if (wrapper == nullptr || !arrayPool.is_active(wrapper)) {
        return false;
    }

    GdArrayInfo *info = *wrapper;
    auto binding_it = gdspxArrayBindings.find(wrapper);
    if (info == nullptr) {
        // Null is valid only for a never-bound wrapper.
        return binding_it == gdspxArrayBindings.end();
    }

    if (binding_it != gdspxArrayBindings.end()) {
        // Bindings are immutable; allow only an unchanged object.
        if (binding_it->second != info) {
            return false;
        }
        auto snapshot_it = gdspxArraySnapshots.find(info);
        return snapshot_it != gdspxArraySnapshots.end() &&
                array_snapshot_matches_live(snapshot_it->second);
    }

    auto snapshot_it = gdspxArraySnapshots.find(info);
    if (snapshot_it == gdspxArraySnapshots.end() ||
            !array_header_matches_snapshot(snapshot_it->second)) {
        // Only internal constructors may establish trust.
        return false;
    }

    auto owner_it = gdspxArrayOwners.find(info);
    if (owner_it != gdspxArrayOwners.end() && owner_it->second != wrapper) {
        // A payload may have only one owner.
        return false;
    }

    // Seal string slots when the array crosses into a wrapper.
    GdArrayMetadataSnapshot sealed_snapshot;
    if (!make_array_snapshot(info, sealed_snapshot)) {
        return false;
    }
    snapshot_it->second = std::move(sealed_snapshot);

    gdspxArrayOwners[info] = wrapper;
    gdspxArrayBindings[wrapper] = info;
    return true;
}

extern "C" bool gdspx_validate_array_wrapper(GdArray *wrapper) {
    if (wrapper == nullptr || !arrayPool.is_active(wrapper)) {
        return false;
    }

    auto binding_it = gdspxArrayBindings.find(wrapper);
    if (*wrapper == nullptr) {
        // Preserve nullable arrays, but reject tampered bound wrappers.
        return binding_it == gdspxArrayBindings.end();
    }
    if (binding_it == gdspxArrayBindings.end() || binding_it->second != *wrapper) {
        return false;
    }

    auto owner_it = gdspxArrayOwners.find(binding_it->second);
    auto snapshot_it = gdspxArraySnapshots.find(binding_it->second);
    return owner_it != gdspxArrayOwners.end() && owner_it->second == wrapper &&
            snapshot_it != gdspxArraySnapshots.end() &&
            array_snapshot_matches_live(snapshot_it->second);
}

extern "C" bool gdspx_prepare_array_wrapper(GdArray *wrapper) {
    return wrapper != nullptr && arrayPool.is_active(wrapper) && *wrapper == nullptr &&
            gdspxArrayBindings.find(wrapper) == gdspxArrayBindings.end();
}

extern "C" bool gdspx_validate_array_info(GdArray array) {
    if (array == nullptr) {
        return false;
    }
    auto snapshot_it = gdspxArraySnapshots.find(array);
    if (snapshot_it == gdspxArraySnapshots.end()) {
        return false;
    }
    // Managers may fill string slots before binding; bound arrays are sealed.
    if (gdspxArrayOwners.find(array) == gdspxArrayOwners.end()) {
        return array_header_matches_snapshot(snapshot_it->second);
    }
    return array_snapshot_matches_live(snapshot_it->second);
}

extern "C" bool gdspx_register_array_info(GdArray array) {
    if (array == nullptr) {
        return false;
    }

    auto snapshot_it = gdspxArraySnapshots.find(array);
    if (snapshot_it != gdspxArraySnapshots.end()) {
        return gdspxArrayOwners.find(array) == gdspxArrayOwners.end() &&
                array_header_matches_snapshot(snapshot_it->second);
    }

    GdArrayMetadataSnapshot snapshot;
    if (!make_array_snapshot(array, snapshot)) {
        return false;
    }
    gdspxArraySnapshots.emplace(array, std::move(snapshot));
    return true;
}

extern "C" bool gdspx_release_array_info(GdArray array) {
    return release_array_snapshot(array, nullptr);
}

static CachedGdStringEntry *find_cached_gdstring_by_value(const char *str, uint32_t len) {
    auto it = gdspxStringCacheByValue.find(std::string(str, len));
    if (it == gdspxStringCacheByValue.end()) {
        return nullptr;
    }
    return &gdspxStringCache[it->second];
}

static CachedGdStringEntry *find_cached_gdstring_by_ptr(const char *ptr) {
    auto it = gdspxStringCacheByPtr.find(ptr);
    if (it == gdspxStringCacheByPtr.end()) {
        return nullptr;
    }
    return &gdspxStringCache[it->second];
}

static bool should_cache_gdstring(uint32_t len) {
    return len <= GDSPX_STRING_CACHE_MAX_LEN;
}

static void remove_cached_gdstring_at(size_t index) {
    CachedGdStringEntry &entry = gdspxStringCache[index];
    gdspxStringCacheByValue.erase(entry.value);
    gdspxStringCacheByPtr.erase(entry.data);
    free(entry.data);

    size_t last_index = gdspxStringCache.size() - 1;
    if (index != last_index) {
        std::swap(gdspxStringCache[index], gdspxStringCache[last_index]);
        const CachedGdStringEntry &moved = gdspxStringCache[index];
        gdspxStringCacheByValue[moved.value] = index;
        gdspxStringCacheByPtr[moved.data] = index;
    }
    gdspxStringCache.pop_back();
}

static bool evict_oldest_unused_gdstring() {
    size_t oldest_index = static_cast<size_t>(-1);
    uint64_t oldest_tick = UINT64_MAX;
    for (size_t i = 0; i < gdspxStringCache.size(); i++) {
        const auto &entry = gdspxStringCache[i];
        if (entry.refcount == 0 && entry.last_used_tick < oldest_tick) {
            oldest_tick = entry.last_used_tick;
            oldest_index = i;
        }
    }
    if (oldest_index == static_cast<size_t>(-1)) {
        return false;
    }
    remove_cached_gdstring_at(oldest_index);
    return true;
}

// Check if the machine is little-endian
inline bool isLittleEndian() {
    static const uint32_t test = 0x12345678;
    return *reinterpret_cast<const uint8_t*>(&test) == 0x78;
}

// LittleEnd read functions
uint64_t readUint64LE(const uint8_t* bytes) {
    if (isLittleEndian()) {
        uint64_t value;
        memcpy(&value, bytes, sizeof(value));
        return value;
    }
    return (uint64_t)bytes[0] |
           ((uint64_t)bytes[1] << 8) |
           ((uint64_t)bytes[2] << 16) |
           ((uint64_t)bytes[3] << 24) |
           ((uint64_t)bytes[4] << 32) |
           ((uint64_t)bytes[5] << 40) |
           ((uint64_t)bytes[6] << 48) |
           ((uint64_t)bytes[7] << 56);
}

uint32_t readUint32LE(const uint8_t* bytes) {
    if (isLittleEndian()) {
        uint32_t value;
        memcpy(&value, bytes, sizeof(value));
        return value;
    }
    return (uint32_t)bytes[0] |
           ((uint32_t)bytes[1] << 8) |
           ((uint32_t)bytes[2] << 16) |
           ((uint32_t)bytes[3] << 24);
}

void writeUint64LE(uint8_t* bytes, uint64_t value) {
    if (isLittleEndian()) {
        memcpy(bytes, &value, sizeof(value));
        return;
    }
    bytes[0] = value & 0xFF;
    bytes[1] = (value >> 8) & 0xFF;
    bytes[2] = (value >> 16) & 0xFF;
    bytes[3] = (value >> 24) & 0xFF;
    bytes[4] = (value >> 32) & 0xFF;
    bytes[5] = (value >> 40) & 0xFF;
    bytes[6] = (value >> 48) & 0xFF;
    bytes[7] = (value >> 56) & 0xFF;
}

void writeUint32LE(uint8_t* bytes, uint32_t value) {
    if (isLittleEndian()) {
        memcpy(bytes, &value, sizeof(value));
        return;
    }
    bytes[0] = value & 0xFF;
    bytes[1] = (value >> 8) & 0xFF;
    bytes[2] = (value >> 16) & 0xFF;
    bytes[3] = (value >> 24) & 0xFF;
}

static_assert(sizeof(bool) == 1, "Boolean size must be 1 byte for web array bridge");
static_assert(sizeof(GdInt) == sizeof(uint64_t), "GdInt must be 64-bit for web ABI");
static_assert(sizeof(GdObj) == sizeof(uint64_t), "GdObj must be 64-bit for web ABI");
static_assert(sizeof(GdFloat) == sizeof(float), "Web GdFloat ABI requires single precision");

extern "C" {

// other functions
EMSCRIPTEN_KEEPALIVE
float gdspx_get_value(float* array, int idx) {
    return array[idx];
}


// bool functions
EMSCRIPTEN_KEEPALIVE
GdBool* gdspx_alloc_bool() {
    return boolPool.acquire();
}

EMSCRIPTEN_KEEPALIVE
GdBool* gdspx_new_bool(bool val) {
    GdBool* ptr = gdspx_alloc_bool();
    if (ptr == nullptr) {
        return nullptr;
    }
    *ptr = (GdBool)val;
    return ptr;
}

EMSCRIPTEN_KEEPALIVE
void gdspx_free_bool(GdBool* b) {
	if (b == nullptr || !boolPool.is_active(b)) {
		return;
	}
    boolPool.release(b);
}


// float functions
EMSCRIPTEN_KEEPALIVE
GdFloat* gdspx_alloc_float() {
    return floatPool.acquire();
}

EMSCRIPTEN_KEEPALIVE
GdFloat* gdspx_new_float(float val) {
    GdFloat* ptr = gdspx_alloc_float();
    if (ptr == nullptr) {
        return nullptr;
    }
    *ptr = (GdFloat)val;
    return ptr;
}

EMSCRIPTEN_KEEPALIVE
void gdspx_free_float(GdFloat* f) {
	if (f == nullptr || !floatPool.is_active(f)) {
		return;
	}
    floatPool.release(f);
}

// int functions
EMSCRIPTEN_KEEPALIVE
GdInt* gdspx_alloc_int() {
    return intPool.acquire();
}

EMSCRIPTEN_KEEPALIVE
GdInt* gdspx_new_int(uint32_t high,uint32_t low) {
    GdInt* ptr = gdspx_alloc_int();
    if (ptr == nullptr) {
        return nullptr;
    }
    const uint64_t val = (static_cast<uint64_t>(high) << 32) | static_cast<uint64_t>(low);
    memcpy(ptr, &val, sizeof(val));
    return ptr;
}

EMSCRIPTEN_KEEPALIVE
void gdspx_free_int(GdInt* i) {
    if (i == nullptr || !intPool.is_active(i)) {
        return;
    }
    *i = 0;
    intPool.release(i);
}

// object functions
EMSCRIPTEN_KEEPALIVE
GdObj* gdspx_alloc_obj() {
    return objPool.acquire();
}

EMSCRIPTEN_KEEPALIVE
GdObj* gdspx_new_obj(uint32_t high,uint32_t low) {
    GdObj* ptr = gdspx_alloc_obj();
    if (ptr == nullptr) {
        return nullptr;
    }
    const uint64_t val = (static_cast<uint64_t>(high) << 32) | static_cast<uint64_t>(low);
    memcpy(ptr, &val, sizeof(val));
    return ptr;
}

EMSCRIPTEN_KEEPALIVE
void gdspx_free_obj(GdObj* obj) {
    if (obj == nullptr || !objPool.is_active(obj)) {
        return;
    }
    *obj = 0;
    objPool.release(obj);
}

// vec2 functions
EMSCRIPTEN_KEEPALIVE
GdVec2* gdspx_alloc_vec2() {
    return vec2Pool.acquire();
}

EMSCRIPTEN_KEEPALIVE
GdVec2* gdspx_new_vec2(float x, float y) {
    GdVec2* ptr = gdspx_alloc_vec2();
    if (ptr == nullptr) {
        return nullptr;
    }
    ptr->x = x;
    ptr->y = y;
    return ptr;
}

EMSCRIPTEN_KEEPALIVE
void gdspx_free_vec2(GdVec2* vec) {
	if (vec == nullptr || !vec2Pool.is_active(vec)) {
		return;
	}
    vec2Pool.release(vec);
}

// vec3 functions
EMSCRIPTEN_KEEPALIVE
GdVec3* gdspx_alloc_vec3() {
    return vec3Pool.acquire();
}

EMSCRIPTEN_KEEPALIVE
GdVec3* gdspx_new_vec3(float x, float y, float z) {
    GdVec3* ptr= gdspx_alloc_vec3();
    if (ptr == nullptr) {
        return nullptr;
    }
    ptr->x = x;
    ptr->y = y;
    ptr->z = z;
    return ptr;
}

EMSCRIPTEN_KEEPALIVE
void gdspx_free_vec3(GdVec3* vec) {
	if (vec == nullptr || !vec3Pool.is_active(vec)) {
		return;
	}
    vec3Pool.release(vec);
}

// vec4 functions
EMSCRIPTEN_KEEPALIVE
GdVec4* gdspx_alloc_vec4() {
    return vec4Pool.acquire();
}

EMSCRIPTEN_KEEPALIVE
GdVec4* gdspx_new_vec4(float x, float y, float z, float w) {
    GdVec4* ptr = gdspx_alloc_vec4();
    if (ptr == nullptr) {
        return nullptr;
    }
    ptr->x = x;
    ptr->y = y;
    ptr->z = z;
    ptr->w = w;
    return ptr;
}

EMSCRIPTEN_KEEPALIVE
void gdspx_free_vec4(GdVec4* vec) {
	if (vec == nullptr || !vec4Pool.is_active(vec)) {
		return;
	}
    vec4Pool.release(vec);
}

// color functions
EMSCRIPTEN_KEEPALIVE
GdColor* gdspx_alloc_color() {
    return colorPool.acquire();
}

EMSCRIPTEN_KEEPALIVE
GdColor* gdspx_new_color(float r, float g, float b, float a) {
    GdColor* ptr = gdspx_alloc_color();
    if (ptr == nullptr) {
        return nullptr;
    }
    ptr->r = r;
    ptr->g = g;
    ptr->b = b;
    ptr->a = a;
    return ptr;
}

EMSCRIPTEN_KEEPALIVE
void gdspx_free_color(GdColor* color) {
	if (color == nullptr || !colorPool.is_active(color)) {
		return;
	}
    colorPool.release(color);
}

// rect2 functions
EMSCRIPTEN_KEEPALIVE
GdRect2* gdspx_alloc_rect2() {
    return rect2Pool.acquire();
}

EMSCRIPTEN_KEEPALIVE
GdRect2* gdspx_new_rect2(float x, float y, float width, float height) {
    GdRect2* ptr = gdspx_alloc_rect2();
    if (ptr == nullptr) {
        return nullptr;
    }
    ptr->position.x = x;
    ptr->position.y = y;
    ptr->size.width = width;
    ptr->size.height = height;
    return ptr;
}

EMSCRIPTEN_KEEPALIVE
void gdspx_free_rect2(GdRect2* rect) {
	if (rect == nullptr || !rect2Pool.is_active(rect)) {
		return;
	}
    rect2Pool.release(rect);
}

// string functions
EMSCRIPTEN_KEEPALIVE
GdString* gdspx_alloc_string() {
    GdString *wrapper = stringPool.acquire();
    if (wrapper == nullptr) {
        return nullptr;
    }

    // Clear nested pointers and stale ownership on reuse.
    auto stale_it = gdspxStringSnapshots.find(wrapper);
    if (stale_it != gdspxStringSnapshots.end()) {
        GdStringSnapshot stale = stale_it->second;
        gdspxStringSnapshots.erase(stale_it);
        release_string_snapshot(stale);
    }
    *wrapper = nullptr;
    gdspxStringSnapshots.emplace(wrapper, GdStringSnapshot{});
    return wrapper;
}

EMSCRIPTEN_KEEPALIVE
GdString* gdspx_new_string(const char* str, uint32_t len) {
    if ((str == nullptr && len != 0) || static_cast<size_t>(len) > GDSPX_MAX_STRING_BYTES) {
        return nullptr;
    }
    const size_t allocation_size = static_cast<size_t>(len) + 1;
    if (allocation_size <= static_cast<size_t>(len)) {
        return nullptr;
    }

    const char *input = str != nullptr ? str : "";
    GdString* ptr = gdspx_alloc_string();
    if (ptr == nullptr) {
        return nullptr;
    }
    CachedGdStringEntry *cached = should_cache_gdstring(len) ?
            find_cached_gdstring_by_value(input, len) : nullptr;
    const bool cache_key_occupied = cached != nullptr;
    const bool cached_value_intact = cached != nullptr && cached->data != nullptr &&
            cached->len == len && cached->value.size() == static_cast<size_t>(len) &&
            memcmp(cached->data, cached->value.data(), len) == 0 && cached->data[len] == '\0';
    if (cached_value_intact && cached->refcount != std::numeric_limits<uint64_t>::max()) {
        cached->refcount += 1;
        cached->last_used_tick = ++gdspxStringCacheTick;
        gdspxStringSnapshots[ptr] = GdStringSnapshot{
            cached->data,
            cached->len,
            GdStringReleaseKind::CACHE,
            true,
        };
        *ptr = cached->data;
        return ptr;
    }

    char* result = (char*)malloc(allocation_size);
    if (result == nullptr) {
        gdspxStringSnapshots.erase(ptr);
        stringPool.release(ptr);
        return nullptr;
    }
    if (len > 0) {
        memcpy(result, input, len);
    }
    result[len] = '\0';

    GdStringReleaseKind release_kind = GdStringReleaseKind::MALLOC;
    if (should_cache_gdstring(len) && !cache_key_occupied) {
        bool cache_has_room = gdspxStringCache.size() < GDSPX_STRING_CACHE_MAX_ENTRIES;
        if (!cache_has_room) {
            cache_has_room = evict_oldest_unused_gdstring();
        }
        if (cache_has_room) {
            size_t cache_index = gdspxStringCache.size();
            gdspxStringCache.push_back(CachedGdStringEntry{
                result,
                len,
                1,
                ++gdspxStringCacheTick,
                std::string(result, len),
            });
            gdspxStringCacheByValue[gdspxStringCache.back().value] = cache_index;
            gdspxStringCacheByPtr[result] = cache_index;
            release_kind = GdStringReleaseKind::CACHE;
        }
    }

    gdspxStringSnapshots[ptr] = GdStringSnapshot{
        result,
        len,
        release_kind,
        true,
    };
    *ptr = result;
    return ptr;
}

bool gdspx_prepare_string_wrapper(GdString *wrapper) {
    if (wrapper == nullptr || !stringPool.is_active(wrapper)) {
        return false;
    }
    auto snapshot_it = gdspxStringSnapshots.find(wrapper);
    return snapshot_it != gdspxStringSnapshots.end() && !snapshot_it->second.bound &&
            snapshot_it->second.ptr == nullptr &&
            snapshot_it->second.release_kind == GdStringReleaseKind::NONE &&
            string_snapshot_matches_live(wrapper, snapshot_it->second);
}

bool gdspx_bind_string_wrapper(GdString *wrapper, GdString value) {
    if (!gdspx_prepare_string_wrapper(wrapper) || value == static_cast<GdString>(wrapper)) {
        discard_manager_string_result(value, wrapper);
        return false;
    }

    GdStringSnapshot snapshot;
    if (!make_manager_string_snapshot(value, snapshot)) {
        discard_manager_string_result(value, wrapper);
        return false;
    }

    // Publish trusted metadata before exposing the nested pointer.
    gdspxStringSnapshots[wrapper] = snapshot;
    *wrapper = value;
    return true;
}

bool gdspx_validate_string_wrapper(GdString *wrapper) {
    if (wrapper == nullptr || !stringPool.is_active(wrapper)) {
        return false;
    }
    auto snapshot_it = gdspxStringSnapshots.find(wrapper);
    return snapshot_it != gdspxStringSnapshots.end() && snapshot_it->second.bound &&
            string_snapshot_matches_live(wrapper, snapshot_it->second);
}

bool gdspx_get_string_value(GdString *wrapper, GdString *r_value) {
    if (r_value == nullptr) {
        return false;
    }
    *r_value = nullptr;
    if (!gdspx_validate_string_wrapper(wrapper)) {
        return false;
    }
    const GdStringSnapshot &snapshot = gdspxStringSnapshots.find(wrapper)->second;
    // Managers consume GdString as a C string. Check only the terminator at
    // the trusted allocation boundary; never scan the mutable wrapper value.
    if (snapshot.ptr != nullptr && snapshot.ptr[snapshot.len] != '\0') {
        return false;
    }
    *r_value = static_cast<GdString>(snapshot.ptr);
    return true;
}

EMSCRIPTEN_KEEPALIVE
const char* gdspx_get_string(GdString* ptr) {
    GdString value = nullptr;
    if (!gdspx_get_string_value(ptr, &value)) {
        return nullptr;
    }
    return static_cast<const char *>(value);
}

EMSCRIPTEN_KEEPALIVE
void gdspx_free_cstr(const char* str) {
    // Legacy API; ownership stays with the wrapper.
    (void)str;
}

EMSCRIPTEN_KEEPALIVE
int32_t gdspx_get_string_len(GdString* ptr) {
    if (!gdspx_validate_string_wrapper(ptr)) {
        return 0;
    }
    const uint32_t len = gdspxStringSnapshots.find(ptr)->second.len;
    if (len > static_cast<uint32_t>(std::numeric_limits<int32_t>::max())) {
        return 0;
    }
    return static_cast<int32_t>(len);
}

EMSCRIPTEN_KEEPALIVE
void gdspx_free_string(GdString* p_gdstr) {
    if (p_gdstr == nullptr || !stringPool.is_active(p_gdstr)) {
        return;
    }

    auto snapshot_it = gdspxStringSnapshots.find(p_gdstr);
    if (snapshot_it != gdspxStringSnapshots.end()) {
        // Detect tampering, then release from the snapshot.
        const bool live_pointer_matches =
                string_snapshot_matches_live(p_gdstr, snapshot_it->second);
        (void)live_pointer_matches;
        GdStringSnapshot snapshot = snapshot_it->second;
        gdspxStringSnapshots.erase(snapshot_it);
        release_string_snapshot(snapshot);
    }
    *p_gdstr = nullptr;
    stringPool.release(p_gdstr);
}



// string functions
EMSCRIPTEN_KEEPALIVE
GdArray* gdspx_alloc_array() {
    GdArray *wrapper = arrayPool.acquire();
    if (wrapper != nullptr) {
        // A wrapper may be reused after a prior call. The nested pointer and
        // C++-side binding must never survive allocator reuse.
        auto binding_it = gdspxArrayBindings.find(wrapper);
        if (binding_it != gdspxArrayBindings.end()) {
            GdArrayInfo *stale_info = binding_it->second;
            if (!release_array_snapshot(stale_info, wrapper)) {
                gdspxArrayBindings.erase(wrapper);
                auto owner_it = gdspxArrayOwners.find(stale_info);
                if (owner_it != gdspxArrayOwners.end() && owner_it->second == wrapper &&
                        gdspxArraySnapshots.find(stale_info) == gdspxArraySnapshots.end()) {
                    gdspxArrayOwners.erase(owner_it);
                }
            }
        }
        *wrapper = nullptr;
    }
    return wrapper;
}


EMSCRIPTEN_KEEPALIVE
void gdspx_free_array(GdArray* p_gdstr) {
    if (p_gdstr == nullptr || !arrayPool.is_active(p_gdstr)) {
        return;
    }

    auto binding_it = gdspxArrayBindings.find(p_gdstr);
    if (binding_it == gdspxArrayBindings.end()) {
        // Ignore unbound nested pointers; they may be forged.
        *p_gdstr = nullptr;
        arrayPool.release(p_gdstr);
        return;
    }

    GdArrayInfo *info = binding_it->second;
    if (!release_array_snapshot(info, p_gdstr)) {
        // Fail closed if trusted metadata is missing.
        gdspxArrayBindings.erase(binding_it);
        *p_gdstr = nullptr;
    }
    arrayPool.release(p_gdstr);
}

GdArrayInfo* deserializeGdArray(uint8_t* bytes, int byteSize) {
    if (bytes == nullptr || byteSize < static_cast<int>(GDSPX_ARRAY_HEADER_SIZE) ||
            static_cast<size_t>(byteSize) > GDSPX_MAX_ARRAY_BYTES) {
        return nullptr;
    }

    GdArrayInfo* info = (GdArrayInfo*)malloc(sizeof(GdArrayInfo));
    if (info == nullptr) {
        return nullptr;
    }
    info->data = nullptr;

    // Header: [size:4][type:4].
    const uint32_t encoded_size = readUint32LE(bytes);
    if (encoded_size > static_cast<uint32_t>(std::numeric_limits<int32_t>::max())) {
        free(info);
        return nullptr;
    }
    info->size = static_cast<int32_t>(encoded_size);
    info->type = (int32_t)readUint32LE(bytes + 4);

    uint8_t* dataBytes = bytes + GDSPX_ARRAY_HEADER_SIZE;
    const size_t dataSize = static_cast<size_t>(byteSize) - GDSPX_ARRAY_HEADER_SIZE;

    size_t element_size = 0;
    if (array_element_size(info->type, element_size)) {
        size_t required_bytes = 0;
        if (!checked_array_bytes(info->size, element_size, required_bytes) || required_bytes != dataSize) {
            free(info);
            return nullptr;
        }
        if (required_bytes == 0) {
            return info;
        }

        info->data = malloc(required_bytes);
        if (info->data == nullptr) {
            free(info);
            return nullptr;
        }

        if (info->type == GD_ARRAY_TYPE_BOOL) {
            bool *data = static_cast<bool *>(info->data);
            for (int32_t i = 0; i < info->size; i++) {
                data[i] = dataBytes[i] != 0;
            }
        } else if (isLittleEndian()) {
            memcpy(info->data, dataBytes, required_bytes);
        } else if (info->type == GD_ARRAY_TYPE_INT64 || info->type == GD_ARRAY_TYPE_GDOBJ) {
            int64_t *data = static_cast<int64_t *>(info->data);
            for (int32_t i = 0; i < info->size; i++) {
                data[i] = static_cast<int64_t>(readUint64LE(dataBytes + static_cast<size_t>(i) * sizeof(int64_t)));
            }
        } else if (info->type == GD_ARRAY_TYPE_FLOAT) {
            float *data = static_cast<float *>(info->data);
            for (int32_t i = 0; i < info->size; i++) {
                const uint32_t bits = readUint32LE(dataBytes + static_cast<size_t>(i) * sizeof(float));
                memcpy(&data[i], &bits, sizeof(float));
            }
        }
        return info;
    }

    if (info->type != GD_ARRAY_TYPE_STRING) {
        free(info);
        return nullptr;
    }

    size_t string_slots = 0;
    if (!checked_array_bytes(info->size, sizeof(char *), string_slots)) {
        free(info);
        return nullptr;
    }
    char **strings = nullptr;
    if (string_slots > 0) {
        strings = static_cast<char **>(calloc(static_cast<size_t>(info->size), sizeof(char *)));
        if (strings == nullptr) {
            free(info);
            return nullptr;
        }
        info->data = strings;
    }

    size_t offset = 0;
    for (int32_t i = 0; i < info->size; i++) {
        if (dataSize - offset < sizeof(uint32_t)) {
            dispose_unbound_array_info(info);
            return nullptr;
        }

        const uint32_t strLen = readUint32LE(dataBytes + offset);
        offset += sizeof(uint32_t);
        if (static_cast<size_t>(strLen) > dataSize - offset) {
            dispose_unbound_array_info(info);
            return nullptr;
        }

        const size_t allocation_size = static_cast<size_t>(strLen) + 1;
        if (allocation_size <= static_cast<size_t>(strLen)) {
            dispose_unbound_array_info(info);
            return nullptr;
        }
        strings[i] = static_cast<char *>(malloc(allocation_size));
        if (strings[i] == nullptr) {
            dispose_unbound_array_info(info);
            return nullptr;
        }
        if (strLen > 0) {
            memcpy(strings[i], dataBytes + offset, strLen);
        }
        strings[i][strLen] = '\0';
        offset += static_cast<size_t>(strLen);
    }

    if (offset != dataSize) {
        dispose_unbound_array_info(info);
        return nullptr;
    }

    return info;
}

GdArrayInfo* deserializeGdArrayRaw(uint8_t* bytes, int byteSize, int32_t arraySize, int32_t arrayType) {
    if (arraySize < 0 || byteSize < 0 || static_cast<size_t>(byteSize) > GDSPX_MAX_ARRAY_BYTES) {
        return nullptr;
    }
    if (byteSize > 0 && bytes == nullptr) {
        return nullptr;
    }

    GdArrayInfo* info = (GdArrayInfo*)malloc(sizeof(GdArrayInfo));
    void* data = nullptr;
    if (info == nullptr) {
        return nullptr;
    }

    info->size = arraySize;
    info->type = arrayType;
    info->data = nullptr;

    switch (arrayType) {
        case GD_ARRAY_TYPE_INT64:
        case GD_ARRAY_TYPE_GDOBJ: {
            size_t requiredBytes = 0;
            if (!checked_array_bytes(arraySize, sizeof(int64_t), requiredBytes) ||
                    static_cast<size_t>(byteSize) != requiredBytes) {
                goto cleanup;
            }
            if (arraySize == 0) {
                return info;
            }
            data = malloc(requiredBytes);
            if (data == nullptr) {
                goto cleanup;
            }
            int64_t* int_data = (int64_t*)data;
            if (isLittleEndian()) {
                memcpy(data, bytes, requiredBytes);
            } else {
                for (int32_t i = 0; i < arraySize; i++) {
                    int_data[i] = (int64_t)readUint64LE(bytes + static_cast<size_t>(i) * sizeof(int64_t));
                }
            }
            break;
        }
        case GD_ARRAY_TYPE_FLOAT: {
            size_t requiredBytes = 0;
            if (!checked_array_bytes(arraySize, sizeof(float), requiredBytes) ||
                    static_cast<size_t>(byteSize) != requiredBytes) {
                goto cleanup;
            }
            if (arraySize == 0) {
                return info;
            }
            data = malloc(requiredBytes);
            if (data == nullptr) {
                goto cleanup;
            }
            float* float_data = (float*)data;
            if (isLittleEndian()) {
                memcpy(data, bytes, requiredBytes);
            } else {
                for (int32_t i = 0; i < arraySize; i++) {
                    uint32_t bits = readUint32LE(bytes + static_cast<size_t>(i) * sizeof(float));
                    memcpy(&float_data[i], &bits, sizeof(float));
                }
            }
            break;
        }
        case GD_ARRAY_TYPE_BOOL: {
            size_t requiredBytes = 0;
            if (!checked_array_bytes(arraySize, sizeof(bool), requiredBytes) ||
                    static_cast<size_t>(byteSize) != requiredBytes) {
                goto cleanup;
            }
            if (arraySize == 0) {
                return info;
            }
            data = malloc(requiredBytes);
            if (data == nullptr) {
                goto cleanup;
            }
            bool* bool_data = (bool*)data;
            for (int32_t i = 0; i < arraySize; i++) {
                bool_data[i] = bytes[i] != 0;
            }
            break;
        }
        case GD_ARRAY_TYPE_BYTE: {
            size_t requiredBytes = 0;
            if (!checked_array_bytes(arraySize, sizeof(uint8_t), requiredBytes) ||
                    static_cast<size_t>(byteSize) != requiredBytes) {
                goto cleanup;
            }
            if (arraySize == 0) {
                return info;
            }
            data = malloc(requiredBytes);
            if (data == nullptr) {
                goto cleanup;
            }
            memcpy(data, bytes, requiredBytes);
            break;
        }
        default:
            goto cleanup;
    }

    info->data = data;
    return info;

cleanup:
    free(data);
    free(info);
    return nullptr;
}

uint8_t* serializeGdArray(const GdArrayMetadataSnapshot &snapshot, int* outSize) {
    if (outSize == nullptr) {
        return nullptr;
    }
    *outSize = 0;
    if (snapshot.info == nullptr || snapshot.size < 0 ||
            snapshot.size > GDSPX_MAX_ARRAY_ELEMENTS) {
        return nullptr;
    }
    if (snapshot.size > 0 && snapshot.data == nullptr) {
        return nullptr;
    }

    size_t dataSize = snapshot.serialized_data_bytes;
    size_t element_size = 0;

    if (array_element_size(snapshot.type, element_size)) {
        size_t expected_data_size = 0;
        if (!checked_array_bytes(snapshot.size, element_size, expected_data_size) ||
                expected_data_size != snapshot.data_bytes || expected_data_size != dataSize) {
            return nullptr;
        }
    } else if (snapshot.type == GD_ARRAY_TYPE_STRING) {
        if (snapshot.string_slots.size() != static_cast<size_t>(snapshot.size)) {
            return nullptr;
        }
        size_t expected_data_size = 0;
        for (const GdArrayStringSlotSnapshot &slot : snapshot.string_slots) {
            if (slot.ptr == nullptr || slot.len > std::numeric_limits<uint32_t>::max()) {
                return nullptr;
            }
            size_t encoded_length = 0;
            if (!checked_add_size(sizeof(uint32_t), slot.len, encoded_length) ||
                    encoded_length > GDSPX_MAX_ARRAY_BYTES - GDSPX_ARRAY_HEADER_SIZE ||
                    expected_data_size >
                            GDSPX_MAX_ARRAY_BYTES - GDSPX_ARRAY_HEADER_SIZE - encoded_length ||
                    !checked_add_size(expected_data_size, encoded_length, expected_data_size)) {
                return nullptr;
            }
        }
        if (expected_data_size != dataSize) {
            return nullptr;
        }
    } else {
        return nullptr;
    }

    if (dataSize > GDSPX_MAX_ARRAY_BYTES - GDSPX_ARRAY_HEADER_SIZE) {
        return nullptr;
    }
    size_t totalSize = 0;
    if (!checked_add_size(GDSPX_ARRAY_HEADER_SIZE, dataSize, totalSize) ||
            totalSize > static_cast<size_t>(std::numeric_limits<int>::max())) {
        return nullptr;
    }
    uint8_t* result = (uint8_t*)malloc(totalSize);
    if (result == nullptr) {
        return nullptr;
    }

    // Header: [size:4][type:4].
    writeUint32LE(result, static_cast<uint32_t>(snapshot.size));
    writeUint32LE(result + 4, static_cast<uint32_t>(snapshot.type));

    uint8_t* dataPtr = result + 8;
    switch (snapshot.type) {
        case GD_ARRAY_TYPE_INT64:
        case GD_ARRAY_TYPE_GDOBJ: {
            int64_t* data = static_cast<int64_t *>(snapshot.data);
            if (isLittleEndian() && dataSize > 0) {
                memcpy(dataPtr, data, dataSize);
            } else {
                for (int32_t i = 0; i < snapshot.size; i++) {
                    writeUint64LE(dataPtr + static_cast<size_t>(i) * sizeof(int64_t), (uint64_t)data[i]);
                }
            }
            break;
        }
        case GD_ARRAY_TYPE_FLOAT: {
            float* data = static_cast<float *>(snapshot.data);
            if (isLittleEndian() && dataSize > 0) {
                memcpy(dataPtr, data, dataSize);
            } else {
                for (int32_t i = 0; i < snapshot.size; i++) {
                    uint32_t bits;
                    memcpy(&bits, &data[i], sizeof(bits));
                    writeUint32LE(dataPtr + static_cast<size_t>(i) * sizeof(float), bits);
                }
            }
            break;
        }
        case GD_ARRAY_TYPE_BOOL: {
            bool* data = static_cast<bool *>(snapshot.data);
            for (int32_t i = 0; i < snapshot.size; i++) {
                dataPtr[i] = data[i] ? 1 : 0;
            }
            break;
        }
        case GD_ARRAY_TYPE_BYTE: {
            if (dataSize > 0) {
                memcpy(dataPtr, snapshot.data, dataSize);
            }
            break;
        }
        case GD_ARRAY_TYPE_STRING: {
            size_t offset = 0;
            for (const GdArrayStringSlotSnapshot &slot : snapshot.string_slots) {
                writeUint32LE(dataPtr + offset, static_cast<uint32_t>(slot.len));
                offset += sizeof(uint32_t);
                if (slot.len > 0) {
                    memcpy(dataPtr + offset, slot.ptr, slot.len);
                }
                offset += slot.len;
            }
            break;
        }
    }

    *outSize = static_cast<int>(totalSize);
    return result;
}

EMSCRIPTEN_KEEPALIVE
uint8_t* gdspx_to_js_array(GdArray* p_gdstr, int* outSize) {
    if (outSize != nullptr) {
        *outSize = 0;
    }
    if (p_gdstr == nullptr || !arrayPool.is_active(p_gdstr)) {
        return nullptr;
    }
    auto binding_it = gdspxArrayBindings.find(p_gdstr);
    if (binding_it == gdspxArrayBindings.end() || *p_gdstr != binding_it->second) {
        return nullptr;
    }
    auto snapshot_it = gdspxArraySnapshots.find(binding_it->second);
    if (snapshot_it == gdspxArraySnapshots.end() ||
            !array_snapshot_matches_live(snapshot_it->second)) {
        return nullptr;
    }
    return serializeGdArray(snapshot_it->second, outSize);
}
EMSCRIPTEN_KEEPALIVE
GdArray* gdspx_to_gd_array(uint8_t* bytes, int byteSize) {
    GdArrayInfo* info = deserializeGdArray(bytes, byteSize);
    if (info == nullptr) {
        return nullptr;
    }
    if (!gdspx_register_array_info(info)) {
        dispose_unbound_array_info(info);
        return nullptr;
    }

    GdArray* p_gdstr = gdspx_alloc_array();
    if (p_gdstr == nullptr) {
        gdspx_release_array_info(info);
        return nullptr;
    }
    *p_gdstr = info;
    if (!gdspx_bind_array_wrapper(p_gdstr)) {
        *p_gdstr = nullptr;
        gdspx_release_array_info(info);
        arrayPool.release(p_gdstr);
        return nullptr;
    }
    return p_gdstr;
}

EMSCRIPTEN_KEEPALIVE
GdArray* gdspx_to_gd_array_raw(uint8_t* bytes, int byteSize, int32_t arraySize, int32_t arrayType) {
    GdArrayInfo* info = deserializeGdArrayRaw(bytes, byteSize, arraySize, arrayType);
    if (info == nullptr) {
        return nullptr;
    }
    if (!gdspx_register_array_info(info)) {
        dispose_unbound_array_info(info);
        return nullptr;
    }

    GdArray* p_gdstr = gdspx_alloc_array();
    if (p_gdstr == nullptr) {
        gdspx_release_array_info(info);
        return nullptr;
    }
    *p_gdstr = info;
    if (!gdspx_bind_array_wrapper(p_gdstr)) {
        *p_gdstr = nullptr;
        gdspx_release_array_info(info);
        arrayPool.release(p_gdstr);
        return nullptr;
    }
    return p_gdstr;
}

}// extern "C"
