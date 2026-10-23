#ifndef GDEXTENSION_SPX_WRAP_H
#define GDEXTENSION_SPX_WRAP_H

#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#define NOT_GODOT_ENGINE
#include "gdextension_spx_ext.h"
#ifndef __cplusplus
typedef uint32_t char32_t;
typedef uint16_t char16_t;
#endif

#ifdef __cplusplus
extern "C" {
#endif

typedef uintptr_t pointer;

extern void initialize(void *userdata, GDExtensionInitializationLevel p_level);
extern void deinitialize(void *userdata, GDExtensionInitializationLevel p_level);

static inline void initialization(GDExtensionInitialization *p_init) {
	p_init->initialize = initialize;
	p_init->deinitialize = deinitialize;
}

static inline void *get_proc_address(uintptr_t fn, const char* p_name) {
	return (void *)((GDExtensionInterfaceGetProcAddress)fn)(p_name);
}

#include "callbacks.gen.h"

#ifdef __cplusplus
}
#endif

#endif // GDEXTENSION_SPX_WRAP_H
