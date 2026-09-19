const GDSPX_UTF8_ENCODER = new TextEncoder();
const GDSPX_UTF8_DECODER = new TextDecoder("utf-8");
const GDSPX_MAX_STRING_BYTES = 256 * 1024 * 1024;

// -----------------------------------------------------------------------------
// Wasm Function Pointers
// -----------------------------------------------------------------------------

let gdspxFunctionPointerModule = null;
let gdspxMalloc = null;
let gdspxFree = null;
let gdspxAllocArray = null;
let gdspxAllocBool = null;
let gdspxAllocColor = null;
let gdspxAllocFloat = null;
let gdspxAllocInt = null;
let gdspxAllocObj = null;
let gdspxAllocRect2 = null;
let gdspxAllocString = null;
let gdspxAllocVec2 = null;
let gdspxAllocVec3 = null;
let gdspxAllocVec4 = null;
let gdspxFreeArray = null;
let gdspxFreeBool = null;
let gdspxFreeColor = null;
let gdspxFreeFloat = null;
let gdspxFreeInt = null;
let gdspxFreeObj = null;
let gdspxFreeRect2 = null;
let gdspxFreeString = null;
let gdspxFreeVec2 = null;
let gdspxFreeVec3 = null;
let gdspxFreeVec4 = null;
let gdspxGetString = null;
let gdspxGetStringLen = null;
let gdspxNewBool = null;
let gdspxNewColor = null;
let gdspxNewFloat = null;
let gdspxNewRect2 = null;
let gdspxNewString = null;
let gdspxNewVec2 = null;
let gdspxNewVec3 = null;
let gdspxNewVec4 = null;
let gdspxBorrowArray = null;
let gdspxGetArrayInfo = null;

function BindGdspxFunctionPointers(module) {
    gdspxMalloc = module['_cmalloc'];
    gdspxFree = module['_cfree'];
    gdspxAllocArray = module['_gdspx_alloc_array'];
    gdspxAllocBool = module['_gdspx_alloc_bool'];
    gdspxAllocColor = module['_gdspx_alloc_color'];
    gdspxAllocFloat = module['_gdspx_alloc_float'];
    gdspxAllocInt = module['_gdspx_alloc_int'];
    gdspxAllocObj = module['_gdspx_alloc_obj'];
    gdspxAllocRect2 = module['_gdspx_alloc_rect2'];
    gdspxAllocString = module['_gdspx_alloc_string'];
    gdspxAllocVec2 = module['_gdspx_alloc_vec2'];
    gdspxAllocVec3 = module['_gdspx_alloc_vec3'];
    gdspxAllocVec4 = module['_gdspx_alloc_vec4'];
    gdspxFreeArray = module['_gdspx_free_array'];
    gdspxFreeBool = module['_gdspx_free_bool'];
    gdspxFreeColor = module['_gdspx_free_color'];
    gdspxFreeFloat = module['_gdspx_free_float'];
    gdspxFreeInt = module['_gdspx_free_int'];
    gdspxFreeObj = module['_gdspx_free_obj'];
    gdspxFreeRect2 = module['_gdspx_free_rect2'];
    gdspxFreeString = module['_gdspx_free_string'];
    gdspxFreeVec2 = module['_gdspx_free_vec2'];
    gdspxFreeVec3 = module['_gdspx_free_vec3'];
    gdspxFreeVec4 = module['_gdspx_free_vec4'];
    gdspxGetString = module['_gdspx_get_string'];
    gdspxGetStringLen = module['_gdspx_get_string_len'];
    gdspxNewBool = module['_gdspx_new_bool'];
    gdspxNewColor = module['_gdspx_new_color'];
    gdspxNewFloat = module['_gdspx_new_float'];
    gdspxNewRect2 = module['_gdspx_new_rect2'];
    gdspxNewString = module['_gdspx_new_string'];
    gdspxNewVec2 = module['_gdspx_new_vec2'];
    gdspxNewVec3 = module['_gdspx_new_vec3'];
    gdspxNewVec4 = module['_gdspx_new_vec4'];
    gdspxBorrowArray = module['_gdspx_borrow_array'];
    gdspxGetArrayInfo = module['_gdspx_get_array_info'];
}

function EnsureGdspxFunctionPointers() {
    if (gdspxFunctionPointerModule === Module) {
        return;
    }
    BindGdspxFunctionPointers(Module);
    gdspxFunctionPointerModule = Module;
}

// -----------------------------------------------------------------------------
// Scalar and Object Value Bridges
// -----------------------------------------------------------------------------

function ToGdBool(value) {
    EnsureGdspxFunctionPointers();
    return gdspxNewBool(value);
}

function ToJsBool(ptr) {
    const HEAPU8 = Module['HEAPU8'];
    const boolValue = HEAPU8[ptr];
    return boolValue !== 0;
}

function AllocGdBool() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocBool();
}

function FreeGdBool(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeBool(ptr);
}

function ToJsObj(ptr, result) {
    return ToJsInt(ptr, result);
}

function AllocGdObj() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocObj();
}

function FreeGdObj(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeObj(ptr);
}

// Wasm scalar pointers are word-aligned. Supplying a result reuses its storage;
// callers must consume or copy it before the next write to that result.
function ToJsInt(ptr, result = {}) {
    const words = Module['HEAPU32'];
    const index = ptr >>> 2;
    result['low'] = words[index];
    result['high'] = words[index + 1];
    return result;
}

function AllocGdInt() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocInt();
}

function FreeGdInt(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeInt(ptr);
}

// -----------------------------------------------------------------------------
// Strings and Structured Math Types
// -----------------------------------------------------------------------------

function ToGdFloat(value) {
    EnsureGdspxFunctionPointers();
    return gdspxNewFloat(value);
}

function ToJsFloat(ptr) {
    const HEAPF32 = Module['HEAPF32'];
    const floatIndex = ptr / 4;
    const floatValue = HEAPF32[floatIndex];
    return floatValue;
}

function AllocGdFloat() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocFloat();
}

function FreeGdFloat(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeFloat(ptr);
}

function ToGdString(str) {
    EnsureGdspxFunctionPointers();
    const stringBytes = GDSPX_UTF8_ENCODER.encode(str);
    const allocationSize = stringBytes.length + 1;
    if (!Number.isSafeInteger(allocationSize) || allocationSize > GDSPX_MAX_STRING_BYTES ||
            typeof gdspxMalloc !== 'function' || typeof gdspxFree !== 'function') {
        throw new Error("String is too large or the Wasm allocator is unavailable");
    }
    const ptr = gdspxMalloc(allocationSize);
    if (!Number.isSafeInteger(ptr) || ptr <= 0 || !IsHeapRange(ptr, allocationSize)) {
        throw new Error("Failed to allocate a Wasm string buffer");
    }
    Module['HEAPU8'].set(stringBytes, ptr);
    Module['HEAPU8'][ptr + stringBytes.length] = 0;
    const gdstrPtr = gdspxNewString(ptr, stringBytes.length);
    gdspxFree(ptr);
    if (!Number.isSafeInteger(gdstrPtr) || gdstrPtr <= 0) {
        throw new Error("Failed to allocate a GdString wrapper");
    }
    return gdstrPtr;
}

function ToJsString(gdstrPtr) {
    EnsureGdspxFunctionPointers();
    if (!gdstrPtr || typeof gdspxGetStringLen !== 'function' ||
            typeof gdspxGetString !== 'function') {
        return '';
    }
    const length = gdspxGetStringLen(gdstrPtr);
    const ptr = gdspxGetString(gdstrPtr);
    if (!Number.isSafeInteger(length) || length < 0 || length > GDSPX_MAX_ARRAY_BYTES ||
            !Number.isSafeInteger(ptr) || ptr <= 0 || !IsHeapRange(ptr, length)) {
        return '';
    }
    const stringBytes = Module['HEAPU8'].subarray(ptr, ptr + length);
    return GDSPX_UTF8_DECODER.decode(stringBytes);
}

function AllocGdString() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocString();
}

function FreeGdString(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeString(ptr);
}

function ToGdVec2(vec) {
    EnsureGdspxFunctionPointers();
    return gdspxNewVec2(vec['x'], vec['y']);
}

function ToJsVec2(ptr, out = {}) {
    const HEAPF32 = Module['HEAPF32'];
    const floatIndex = ptr / 4;
    out['x'] = HEAPF32[floatIndex];
    out['y'] = HEAPF32[floatIndex + 1];
    return out;
}

function AllocGdVec2() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocVec2();
}

function FreeGdVec2(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeVec2(ptr);
}

function ToGdVec3(vec) {
    EnsureGdspxFunctionPointers();
    return gdspxNewVec3(vec['x'], vec['y'], vec['z']);
}

function ToJsVec3(ptr, out = {}) {
    const HEAPF32 = Module['HEAPF32'];
    const floatIndex = ptr / 4;
    out['x'] = HEAPF32[floatIndex];
    out['y'] = HEAPF32[floatIndex + 1];
    out['z'] = HEAPF32[floatIndex + 2];
    return out;
}

function AllocGdVec3() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocVec3();
}

function FreeGdVec3(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeVec3(ptr);
}

function ToGdVec4(vec) {
    EnsureGdspxFunctionPointers();
    return gdspxNewVec4(vec['x'], vec['y'], vec['z'], vec['w']);
}

function ToJsVec4(ptr, out = {}) {
    const HEAPF32 = Module['HEAPF32'];
    const floatIndex = ptr / 4;
    out['x'] = HEAPF32[floatIndex];
    out['y'] = HEAPF32[floatIndex + 1];
    out['z'] = HEAPF32[floatIndex + 2];
    out['w'] = HEAPF32[floatIndex + 3];
    return out;
}

function AllocGdVec4() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocVec4();
}

function FreeGdVec4(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeVec4(ptr);
}

function ToGdColor(color) {
    EnsureGdspxFunctionPointers();
    return gdspxNewColor(color['r'], color['g'], color['b'], color['a']);
}

function ToJsColor(ptr, out = {}) {
    const HEAPF32 = Module['HEAPF32'];
    const floatIndex = ptr / 4;
    out['r'] = HEAPF32[floatIndex];
    out['g'] = HEAPF32[floatIndex + 1];
    out['b'] = HEAPF32[floatIndex + 2];
    out['a'] = HEAPF32[floatIndex + 3];
    return out;
}

function AllocGdColor() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocColor();
}

function FreeGdColor(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeColor(ptr);
}

function ToGdRect2(rect) {
    EnsureGdspxFunctionPointers();
    return gdspxNewRect2(rect['position']['x'], rect['position']['y'], rect['size']['x'], rect['size']['y']);
}

function ToJsRect2(ptr, out = { 'position': {}, 'size': {} }) {
    const HEAPF32 = Module['HEAPF32'];
    const floatIndex = ptr / 4;
    out['position']['x'] = HEAPF32[floatIndex];
    out['position']['y'] = HEAPF32[floatIndex + 1];
    out['size']['x'] = HEAPF32[floatIndex + 2];
    out['size']['y'] = HEAPF32[floatIndex + 3];
    return out;
}

function AllocGdRect2() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocRect2();
}

function FreeGdRect2(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeRect2(ptr);
}

// -----------------------------------------------------------------------------
// Native Arrays and Wasm Allocation Arenas
// -----------------------------------------------------------------------------

// BEGIN GENERATED ARRAY TYPES
// Code generated by spx codegen; DO NOT EDIT.
const GDSPX_ARRAY_TAG = "__gdspx_array";
const GDSPX_ARRAY_TYPE_UNKNOWN = 0;
const GDSPX_ARRAY_TYPE_INT64 = 1;
const GDSPX_ARRAY_TYPE_FLOAT = 2;
const GDSPX_ARRAY_TYPE_BOOL = 3;
const GDSPX_ARRAY_TYPE_STRING = 4;
const GDSPX_ARRAY_TYPE_BYTE = 5;
const GDSPX_ARRAY_TYPE_GDOBJ = 6;

function NativeArrayElementSize(arrayType) {
    switch (arrayType) {
    case GDSPX_ARRAY_TYPE_INT64:
        return 8;
    case GDSPX_ARRAY_TYPE_FLOAT:
        return 4;
    case GDSPX_ARRAY_TYPE_BOOL:
        return 1;
    case GDSPX_ARRAY_TYPE_BYTE:
        return 1;
    case GDSPX_ARRAY_TYPE_GDOBJ:
        return 8;
    default:
        return 0;
    }
}
// END GENERATED ARRAY TYPES
const GDSPX_ARRAY_ARENA_BYTES = 1024 * 1024;
const GDSPX_ARRAY_ALIGNMENT = 8;
const GDSPX_ARRAY_POOL = "default";
const GDSPX_INPUT_POOL = "input";
const GDSPX_EMPTY_U8 = new Uint8Array(0);
const GDSPX_MAX_ARRAY_ELEMENTS = 16 * 1024 * 1024;
const GDSPX_MAX_ARRAY_BYTES = 256 * 1024 * 1024;

let arrayArenaModule = null;
const arrayArenas = new Map();
const deferredArenaFrees = [];

// Only descriptors created here can lend their Wasm pointer to a native call.
// The frozen descriptor is also its metadata; no second object is needed.
const NativeArrays = (() => {
    const borrowed = new WeakSet();

    function borrow(type, count, byteLength, poolName = GDSPX_ARRAY_POOL) {
        if (!HasActiveModuleHeap() || !IsNativeArrayByteLength(type, count, byteLength)) {
            return null;
        }
        const arena = GetArrayArena(byteLength, poolName);
        if (!arena) {
            return null;
        }
        const module = arena.module;
        const ptr = arena.ptr + arena.offset;
        arena.offset += AlignArrayBytes(byteLength);
        arena.sequence += 1;

        const array = Object.freeze({
            [GDSPX_ARRAY_TAG]: true,
            'type': type,
            'count': count,
            'ptr': ptr,
            'module': module,
            'byteLength': byteLength,
            'sequence': arena.sequence,
            'pool': arena.pool,
            'shared': typeof SharedArrayBuffer === 'function' && module['HEAPU8'].buffer instanceof SharedArrayBuffer,
            get 'data'() {
                return NativeArrayDataView(ptr, byteLength, module);
            },
        });
        borrowed.add(array);
        return array;
    }

    function metadata(array) {
        return borrowed.has(array) ? array : null;
    }

    return Object.freeze({ borrow, metadata });
})();

const GdspxBorrowNativeArray = NativeArrays.borrow;

function HasActiveModule() {
    return typeof Module !== 'undefined' && Module !== null;
}

function HasActiveModuleHeap() {
    return HasActiveModule() && !!Module['HEAPU8'];
}

function FreeArrayArena(arena) {
    try {
        arena.free(arena.ptr);
    } catch {
        // The previous wasm instance may already be torn down during restart.
    }
}

function AlignArrayBytes(size) {
    return Math.ceil(size / GDSPX_ARRAY_ALIGNMENT) * GDSPX_ARRAY_ALIGNMENT;
}

function ArrayArenaCapacity(minSize) {
    let capacity = GDSPX_ARRAY_ARENA_BYTES;
    while (capacity < minSize) {
        capacity *= 2;
    }
    return capacity;
}

function IsSafeArrayCount(value) {
    return Number.isSafeInteger(value) && value >= 0 && value <= GDSPX_MAX_ARRAY_ELEMENTS;
}

function IsHeapRange(ptr, byteLength) {
    if (!HasActiveModuleHeap() || !Number.isSafeInteger(ptr) || ptr < 0 ||
            !Number.isSafeInteger(byteLength) || byteLength < 0) {
        return false;
    }
    const heapLength = Module['HEAPU8'].length;
    return ptr <= heapLength && byteLength <= heapLength - ptr;
}

function NativeArrayDataView(ptr, byteLength, module) {
    if (!HasActiveModule() || module !== Module || !IsHeapRange(ptr, byteLength)) {
        return GDSPX_EMPTY_U8;
    }
    return module['HEAPU8'].subarray(ptr, ptr + byteLength);
}

function GdspxFlushDeferredFrees() {
    // Update entry, reset, and destroy end all transient array borrows.
    for (const arena of arrayArenas.values()) {
        arena.offset = 0;
    }
    for (const arena of deferredArenaFrees.splice(0)) {
        FreeArrayArena(arena);
    }
}

// Reserve room in the current block, or rotate without invalidating earlier
// arguments. Counts and byte lengths have already been checked by borrow().
function GetArrayArena(byteLength, poolName) {
    EnsureGdspxFunctionPointers();
    if (typeof gdspxMalloc !== 'function' || typeof gdspxFree !== 'function') {
        return null;
    }
    if (arrayArenaModule !== Module) {
        for (const arena of arrayArenas.values()) {
            FreeArrayArena(arena);
        }
        arrayArenas.clear();
        arrayArenaModule = Module;
    }

    const pool = String(poolName || GDSPX_ARRAY_POOL);
    const previous = arrayArenas.get(pool);
    const required = AlignArrayBytes(byteLength);
    if (previous && required <= previous.capacity - previous.offset) {
        return previous;
    }

    const capacity = ArrayArenaCapacity(required);
    const ptr = gdspxMalloc(capacity);
    if (!Number.isSafeInteger(ptr) || ptr <= 0 || ptr % GDSPX_ARRAY_ALIGNMENT !== 0 || !IsHeapRange(ptr, capacity)) {
        return null;
    }
    if (previous) {
        deferredArenaFrees.push(previous);
    }
    const arena = { ptr, capacity, offset: 0, sequence: 0, module: Module, free: gdspxFree, pool };
    arrayArenas.set(pool, arena);
    return arena;
}

function IsNativeArrayByteLength(type, count, byteLength) {
    if (!IsSafeArrayCount(count) || !Number.isSafeInteger(byteLength) ||
            byteLength < 0 || byteLength > GDSPX_MAX_ARRAY_BYTES) {
        return false;
    }
    if (type === GDSPX_ARRAY_TYPE_STRING) {
        return count === 0 ? byteLength === 0 : byteLength >= count * 9;
    }
    const size = NativeArrayElementSize(type);
    return size > 0 && byteLength === count * size;
}

// Borrowed descriptors are already validated and immutable. External descriptors
// are normalized without retaining their untrusted pointer or module fields.
function DescribeNativeArray(array) {
    if (!array || typeof array !== 'object') {
        return null;
    }
    const metadata = NativeArrays.metadata(array);
    if (metadata) {
        return HasActiveModule() && metadata['module'] === Module ? metadata : null;
    }
    const type = Number(array['type']);
    const count = Number(array['count']);
    const data = array['data'];
    const byteLength = Number(data && data.length);
    if (!IsNativeArrayByteLength(type, count, byteLength)) {
        return null;
    }
    return { 'type': type, 'count': count, 'byteLength': byteLength, 'data': data };
}

function NativeArrayCount(array) {
    const info = DescribeNativeArray(array);
    return info ? info['count'] : -1;
}

// Writable calls retain caller-provided Wasm storage. External read-only inputs
// are copied into the input pool; their pointer fields are never used.
function RequireNativeArrayBuffer(array, opName, expectedType = null, writable = false) {
    if (!array || array[GDSPX_ARRAY_TAG] !== true) {
        throw new Error(opName + " requires a native array");
    }
    const metadata = NativeArrays.metadata(array);
    if (writable && !metadata) {
        throw new Error(opName + " requires a pre-allocated Wasm array");
    }
    const info = DescribeNativeArray(array);
    if (!info) {
        throw new Error(opName + " requires a valid native array shape");
    }
    if (expectedType !== null && info['type'] !== expectedType) {
        throw new Error(opName + " received an incompatible native array type");
    }
    if (metadata) {
        if (!IsHeapRange(metadata['ptr'], metadata['byteLength'])) {
            throw new Error(opName + " requires accessible native array data");
        }
        return metadata;
    }

    const copy = GdspxBorrowNativeArray(info['type'], info['count'], info['byteLength'], GDSPX_INPUT_POOL);
    if (!copy) {
        throw new Error(opName + " failed to allocate native array input buffer");
    }
    copy['data'].set(info['data']);
    return copy;
}

function RequireNativeArray(array, opName, expectedType = null, writable = false) {
    return RequireNativeArrayBuffer(array, opName, expectedType, writable)['ptr'];
}

function ReadArrayOutput(exportName, type, count) {
    if (!HasActiveModule()) {
        return null;
    }
    const call = Module[exportName];
    if (typeof call !== 'function') {
        return null;
    }
    const out = GdspxBorrowNativeArray(type, count, count * NativeArrayElementSize(type), exportName);
    if (!out) {
        return null;
    }
    call(out['ptr']);
    return out;
}

function ToGdArray(array) {
    EnsureGdspxFunctionPointers();
    const input = RequireNativeArrayBuffer(array, "ToGdArray");
    const wrapper = gdspxBorrowArray(input['ptr'], input['byteLength'], input['count'], input['type']);
    if (!wrapper) {
        throw new Error("Invalid native array data");
    }
    return wrapper;
}

function ToJsArray(wrapper) {
    EnsureGdspxFunctionPointers();
    const info = gdspxGetArrayInfo(wrapper);
    if (!info) {
        return null;
    }
    if (!IsHeapRange(info, 12) || info % 4 !== 0) {
        return null;
    }
    const words = Module['HEAPU32'];
    const count = words[info / 4];
    const type = words[info / 4 + 1];
    const ptr = words[info / 4 + 2];
    if (!IsSafeArrayCount(count)) {
        return null;
    }
    let data;
    if (type === GDSPX_ARRAY_TYPE_STRING) {
        if (!IsHeapRange(ptr, count * 4) || ptr % 4 !== 0 || (count > 0 && ptr === 0)) {
            return null;
        }
        const heap = Module['HEAPU8'];
        const strings = [];
        let total = count * 8;
        for (let i = 0; i < count; i++) {
            const start = words[ptr / 4 + i];
            if (!start || !IsHeapRange(start, 1)) {
                return null;
            }
            let end = start;
            while (end < heap.length && heap[end] !== 0 && end - start < GDSPX_MAX_STRING_BYTES) {
                end++;
            }
            if (end === heap.length || heap[end] !== 0 || total > GDSPX_MAX_ARRAY_BYTES - (end - start + 1)) {
                return null;
            }
            strings.push([start, end]);
            total += end - start + 1;
        }
        data = new Uint8Array(total);
        const table = new DataView(data.buffer);
        let offset = count * 8;
        for (let i = 0; i < count; i++) {
            const [start, end] = strings[i];
            table.setUint32(i * 8, offset, true);
            table.setUint32(i * 8 + 4, end - start, true);
            data.set(heap.subarray(start, end), offset);
            offset += end - start + 1;
        }
    } else {
        const length = count * NativeArrayElementSize(type);
        if (!IsNativeArrayByteLength(type, count, length) || !IsHeapRange(ptr, length) ||
                (length > 0 && ptr === 0)) {
            return null;
        }
        // Detach before the generated wrapper releases the native result.
        data = Module['HEAPU8'].slice(ptr, ptr + length);
    }
    return { [GDSPX_ARRAY_TAG]: true, 'type': type, 'count': count, 'data': data };
}

function AllocGdArray() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocArray();
}

function FreeGdArray(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeArray(ptr);
}

// These functions are called from Go/Wasm or Emscripten's separately compiled
// library glue. Bracketed global exports keep that ABI stable under Advanced
// Closure property and symbol renaming.
globalThis['GdspxFlushDeferredFrees'] = GdspxFlushDeferredFrees;
globalThis['GdspxBorrowNativeArray'] = GdspxBorrowNativeArray;
