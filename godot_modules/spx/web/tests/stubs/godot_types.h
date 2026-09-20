#pragma once

// Minimal value types for standalone Web ABI tests and benchmarks.
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

