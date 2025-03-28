#ifndef GDEXTENSION_SPX_PRE_DEFINE_H
#define GDEXTENSION_SPX_PRE_DEFINE_H

#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#ifndef __cplusplus
typedef uint32_t char32_t;
typedef uint16_t char16_t;
#endif

#ifdef __cplusplus
extern "C" {
#endif


typedef float real_t;

typedef struct {
    real_t X;
    real_t Y;
    real_t Z;
    real_t W;
} Vector4;

typedef struct {
    real_t X;
    real_t Y;
    real_t Z;
} Vector3;

typedef struct {
    real_t X;
    real_t Y;
} Vector2;

typedef struct {
    float R;
    float G;
    float B;
    float A;
} Color;

typedef struct {
	Vector2 Position; // TopLeft point
	Vector2 Size;
} Rect2;

typedef real_t GDReal;

typedef void *GDExtensionStringPtr;
typedef int64_t GDExtensionInt;
typedef uint8_t GDExtensionBool;


#ifdef __cplusplus
}
#endif

#endif // GDEXTENSION_SPX_PRE_DEFINE_H