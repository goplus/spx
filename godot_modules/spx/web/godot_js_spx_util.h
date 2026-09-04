#ifndef GODOT_JS_SPX_UTIL_H
#define GODOT_JS_SPX_UTIL_H

#include "../gdextension_spx_ext.h"
#include <algorithm>
#include <new>
#include <vector>

// GdString and GdArray wrappers are pointer-to-pointer values allocated by
// Web ABI pools. Their bindings are kept in private C++ state; callers must
// not manufacture bindings by writing directly into JavaScript memory.
#ifdef __cplusplus
extern "C" {
#endif
bool gdspx_bind_string_wrapper(GdString *wrapper, GdString value);
bool gdspx_get_string_value(GdString *wrapper, GdString *r_value);
bool gdspx_prepare_string_wrapper(GdString *wrapper);
bool gdspx_validate_string_wrapper(GdString *wrapper);
bool gdspx_bind_array_wrapper(GdArray *wrapper);
bool gdspx_validate_array_wrapper(GdArray *wrapper);
bool gdspx_prepare_array_wrapper(GdArray *wrapper);
bool gdspx_validate_array_info(GdArray array);
bool gdspx_register_array_info(GdArray array);
bool gdspx_release_array_info(GdArray array);
#ifdef __cplusplus
}
#endif

template <typename T>
class ObjectPool {
public:
    explicit ObjectPool(size_t size) {
        for (size_t i = 0; i < size; ++i) {
            T *obj = new (std::nothrow) T();
            if (obj != nullptr) {
                pool.push_back(obj);
                allocated.push_back(obj);
            }
        }
    }

    ~ObjectPool() {
        for (auto obj : allocated) {
            delete obj;
        }
    }

    T* acquire() {
        if (pool.empty()) {
            T *obj = new (std::nothrow) T();
            if (obj != nullptr) {
                allocated.push_back(obj);
            }
            return obj;
        } else {
            T* obj = pool.back();
            pool.pop_back();
            return obj;
        }
    }

    bool is_active(T *obj) const {
        if (obj == nullptr || std::find(allocated.begin(), allocated.end(), obj) == allocated.end()) {
            return false;
        }
        return std::find(pool.begin(), pool.end(), obj) == pool.end();
    }

    void release(T* obj) {
        if(obj == nullptr) {
            print_error("ObjectPool::release called with null pointer");
            return;
        }
        if (std::find(pool.begin(), pool.end(), obj) != pool.end()) {
            print_error("ObjectPool::release called twice for the same pointer");
            return;
        }
        if (std::find(allocated.begin(), allocated.end(), obj) == allocated.end()) {
            print_error("ObjectPool::release called for an unowned pointer");
            return;
        }
        pool.push_back(obj);
    }

private:
    std::vector<T*> pool;
    std::vector<T*> allocated;
};

#endif // GODOT_JS_SPX_UTIL_H
