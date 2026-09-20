// The runner adds the actual utility and generated wrappers from each revision.
#include <cstddef>
#include <cstdint>
#include <cstdlib>
#include <emscripten/emscripten.h>
#include <emscripten/heap.h>
#include "godot_types.h"

static uint32_t heap_allocations = 0;
extern "C" void *__real_malloc(size_t size);
extern "C" void *__wrap_malloc(size_t size) {
    ++heap_allocations;
    return __real_malloc(size);
}

// Identical sinks isolate bridge overhead; these are not Godot scene operations.
static volatile float last_float;
static volatile uint8_t last_bool;
static volatile int64_t last_id;
struct ScalarSink {
    void set_global_gravity(float value) { last_float = value; }
    void set_camera_smoothing(uint8_t value) { last_bool = value; }
    void set_rotation(int64_t id, float value) { last_id = id; last_float = value; }
    void set_visible(int64_t id, uint8_t value) { last_id = id; last_bool = value; }
    void set_range(int64_t id, float minimum, float maximum, float step, float value) {
        last_id = id;
        last_float = minimum + maximum + step + value;
    }
};
static ScalarSink sink;
#define physicsMgr (&sink)
#define cameraMgr (&sink)
#define spriteMgr (&sink)
#define uiMgr (&sink)

extern "C" {
EMSCRIPTEN_KEEPALIVE uint32_t benchmark_allocations() { return heap_allocations; }
EMSCRIPTEN_KEEPALIVE float benchmark_float() { return last_float; }
EMSCRIPTEN_KEEPALIVE uint8_t benchmark_bool() { return last_bool; }
EMSCRIPTEN_KEEPALIVE uint32_t benchmark_id_low() { return uint32_t(last_id); }
EMSCRIPTEN_KEEPALIVE uint32_t benchmark_id_high() { return uint64_t(last_id) >> 32; }
EMSCRIPTEN_KEEPALIVE int benchmark_grow_heap() {
    return emscripten_resize_heap(emscripten_get_heap_size() + 65536);
}
}
