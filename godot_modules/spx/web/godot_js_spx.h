#ifndef GODOT_JS_SPX_H
#define GODOT_JS_SPX_H

#include <stddef.h>
#include <stdint.h>

typedef float GdFloat;
typedef int64_t GdObj;
typedef const void *GdString;
typedef int64_t GdInt;
typedef uint8_t GdBool;

#include "godot_js_spx_callbacks.gen.h"

#ifdef __cplusplus
extern "C" {
#endif

// GDExtention
extern void godot_js_spx_on_runtime_panic(GdString msg);
extern void godot_js_spx_on_runtime_exit(GdInt code);
extern void godot_js_spx_on_reset_done(GdInt code);
extern void godot_js_spx_request_reset_cb(void (*p_callback)());
extern void godot_js_spx_game_data_cb(void (*p_callback)(const char *p_path, const char **p_filev, int p_filec));
extern void godot_js_spx_window_size_get(int32_t *p_x, int32_t *p_y);

#ifdef __cplusplus
}
#endif


#endif // GODOT_JS_SPX_H
