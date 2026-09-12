const GDSPX_HAS_BIG_INT64 = typeof DataView.prototype.getBigInt64 === 'function';
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
let gdspxFreeCstr = null;
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
let gdspxNewInt = null;
let gdspxNewObj = null;
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
    gdspxFreeCstr = module['_gdspx_free_cstr'];
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
    gdspxNewInt = module['_gdspx_new_int'];
    gdspxNewObj = module['_gdspx_new_obj'];
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

let gdspxHeapDataViewBuffer = null;
let gdspxHeapDataView = null;

function GetHeapDataView() {
    const memoryBuffer = Module['HEAPU8'].buffer;
    if (gdspxHeapDataViewBuffer !== memoryBuffer) {
        gdspxHeapDataViewBuffer = memoryBuffer;
        gdspxHeapDataView = new DataView(memoryBuffer);
    }
    return gdspxHeapDataView;
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

function PrintGdBool(ptr) {
    console.log(ToJsBool(ptr));
}

function FreeGdBool(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeBool(ptr);
}

// Legacy Object aliases keep the Object/ObjectPtr naming used by some generated
// bridge code while delegating to the canonical GdObj helpers below.
function ToGdObject(object) {
    return ToGdObj(object);
}
function ToJsObject(ptr) {
    return ToJsObj(ptr);
}
function FreeGdObject(ptr) {
    FreeGdObj(ptr);
}
function AllocGdObject() {
    return AllocGdObj();
}
function PrintGdObject(ptr) {
    PrintGdObj(ptr);
}

// JS values use (low, high); native int and object constructors take (high, low).
function ToGdObj(value) {
    EnsureGdspxFunctionPointers();
    return gdspxNewObj(value['high'], value['low']);
}

function ToJsObj(ptr, result) {
    return ToJsInt(ptr, result);
}

function ToJsBigObj(ptr) {
    return ToJsBigInt(ptr);
}

function AllocGdObj() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocObj();
}

function PrintGdObj(ptr) {
    console.log(ToJsObj(ptr));
}

function FreeGdObj(ptr) {
    EnsureGdspxFunctionPointers();
    gdspxFreeObj(ptr);
}

function ToGdInt(value) {
    EnsureGdspxFunctionPointers();
    return gdspxNewInt(value['high'], value['low']);
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

function ToJsBigInt(ptr) {
    const dataView = GetHeapDataView();
    if (GDSPX_HAS_BIG_INT64) {
        return dataView.getBigInt64(ptr, true);
    }
    const low = dataView.getUint32(ptr, true);
    const high = dataView.getUint32(ptr + 4, true);
    return BigInt.asIntN(64, (BigInt(high) << 32n) | BigInt(low));
}

function AllocGdInt() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocInt();
}

function PrintGdInt(ptr) {
    console.log(ToJsInt(ptr));
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

function PrintGdFloat(ptr) {
    console.log(ToJsFloat(ptr));
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
    return toJsString(gdstrPtr, false);
}

function toJsString(gdstrPtr, isFree) {
    EnsureGdspxFunctionPointers();
    if (!gdstrPtr || typeof gdspxGetStringLen !== 'function' ||
            typeof gdspxGetString !== 'function') {
        return '';
    }
    const length = gdspxGetStringLen(gdstrPtr);
    const ptr = gdspxGetString(gdstrPtr);
    if (!Number.isSafeInteger(length) || length < 0 || length > GDSPX_MAX_ARRAY_BYTES ||
            !Number.isSafeInteger(ptr) || ptr <= 0 || !IsHeapRange(ptr, length)) {
        if (isFree && typeof gdspxFreeString === 'function') {
            gdspxFreeString(gdstrPtr);
        }
        return '';
    }
    const stringBytes = Module['HEAPU8'].subarray(ptr, ptr + length);
    const result = GDSPX_UTF8_DECODER.decode(stringBytes);
    if (isFree) {
        // The GdString wrapper owns the returned C string. Free the wrapper so
        // cached and uncached strings follow the same ownership path.
        gdspxFreeString(gdstrPtr);
    }
    return result;
}

function AllocGdString() {
    EnsureGdspxFunctionPointers();
    return gdspxAllocString();
}

function PrintGdString(gdstrPtr) {
    console.log(toJsString(gdstrPtr, false));
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

function PrintGdVec2(ptr) {
    console.log(ToJsVec2(ptr));
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

function PrintGdVec3(ptr) {
    const vec3 = ToJsVec3(ptr);
    console.log(`Vec3(${vec3['x']}, ${vec3['y']}, ${vec3['z']})`);
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

function PrintGdVec4(ptr) {
    const vec4 = ToJsVec4(ptr);
    console.log(`Vec4(${vec4['x']}, ${vec4['y']}, ${vec4['z']}, ${vec4['w']})`);
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

function PrintGdColor(ptr) {
    const color = ToJsColor(ptr);
    console.log(`Color(${color['r']}, ${color['g']}, ${color['b']}, ${color['a']})`);
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

function PrintGdRect2(ptr) {
    const rect = ToJsRect2(ptr);
    console.log(`Rect2(position: (${rect['position']['x']}, ${rect['position']['y']}), size: (${rect['size']['x']}, ${rect['size']['y']}))`);
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
const GDSPX_RET_POOL = "return";
const GDSPX_EMPTY_U8 = new Uint8Array(0);
const GDSPX_MAX_ARRAY_ELEMENTS = 16 * 1024 * 1024;
const GDSPX_MAX_ARRAY_BYTES = 256 * 1024 * 1024;
const GDSPX_MAX_I32 = 0x7fffffff;
const GDSPX_MAX_ALIGNED_BYTES = GDSPX_MAX_I32 - (GDSPX_MAX_I32 % GDSPX_ARRAY_ALIGNMENT);

let arrayArenaModule = null;
const arrayArenas = new Map();
const deferredArenaFrees = [];
function HasActiveModule() {
    return typeof Module !== 'undefined' && Module !== null;
}

function HasActiveModuleHeap() {
    return HasActiveModule() && !!Module['HEAPU8'];
}

function FreePtrMap(map) {
    for (const item of map.values()) {
        if (item.ptr !== 0 && typeof item.free === 'function') {
            try {
                item.free(item.ptr);
            } catch {
                // The previous wasm instance may already be torn down during restart.
            }
        }
    }
    map.clear();
}

function AlignArrayBytes(size) {
    if (!Number.isSafeInteger(size) || size <= 0 || size > GDSPX_MAX_ALIGNED_BYTES) {
        return 0;
    }
    const aligned = Math.ceil(size / GDSPX_ARRAY_ALIGNMENT) * GDSPX_ARRAY_ALIGNMENT;
    return aligned <= GDSPX_MAX_ALIGNED_BYTES ? aligned : 0;
}

function ArrayArenaCapacity(minSize) {
    let capacity = GDSPX_ARRAY_ARENA_BYTES;
    while (capacity < minSize) {
        if (capacity > Math.floor(GDSPX_MAX_ALIGNED_BYTES / 2)) {
            return GDSPX_MAX_ALIGNED_BYTES;
        }
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
    if (!Number.isSafeInteger(byteLength) || byteLength < 0 || byteLength > GDSPX_MAX_ARRAY_BYTES) {
        return GDSPX_EMPTY_U8;
    }
    if (!HasActiveModuleHeap() || module !== Module) {
        return GDSPX_EMPTY_U8;
    }
    if (!IsHeapRange(ptr, byteLength) || (byteLength > 0 && ptr === 0)) {
        return GDSPX_EMPTY_U8;
    }
    return Module['HEAPU8'].subarray(ptr, ptr + byteLength);
}

function DeferArenaFree(ptr, freeFn) {
    if (!Number.isInteger(ptr) || ptr === 0 || typeof freeFn !== 'function') {
        return;
    }
    deferredArenaFrees.push({ ptr, free: freeFn });
}

function GdspxFlushDeferredFrees() {
    // Frame boundaries end all transient input/return borrows.
    for (const arena of arrayArenas.values()) {
        arena.offset = 0;
    }
    if (deferredArenaFrees.length === 0) {
        return;
    }
    const pending = deferredArenaFrees.splice(0, deferredArenaFrees.length);
    for (const item of pending) {
        if (!item || item.ptr === 0 || typeof item.free !== 'function') {
            continue;
        }
        try {
            item.free(item.ptr);
        } catch {
            // The previous wasm instance may already be torn down during restart.
        }
    }
}

function GetArrayArena(minSize, poolName = GDSPX_ARRAY_POOL, rotate = false) {
    EnsureGdspxFunctionPointers();
    if (typeof gdspxMalloc !== 'function' || typeof gdspxFree !== 'function') {
        return null;
    }
    if (arrayArenaModule !== Module) {
        FreePtrMap(arrayArenas);
        arrayArenaModule = Module;
    }

    const pool = String(poolName || GDSPX_ARRAY_POOL);
    let arena = arrayArenas.get(pool);
    const required = AlignArrayBytes(minSize);
    if (required === 0 && minSize !== 0) {
        return null;
    }
    if (arena && !rotate && required <= arena.capacity) {
        return arena;
    }

    const capacity = ArrayArenaCapacity(required);
    if (capacity === 0 || capacity > GDSPX_MAX_ARRAY_BYTES) {
        return null;
    }
    const ptr = gdspxMalloc(capacity);
    if (!Number.isSafeInteger(ptr) || ptr <= 0 || !IsHeapRange(ptr, capacity)) {
        return null;
    }
    if (arena && arena.ptr !== 0) {
        DeferArenaFree(arena.ptr, arena.free);
    }

    arena = {
        ptr,
        capacity,
        offset: 0,
        sequence: 0,
        module: Module,
        free: gdspxFree,
        pool,
    };
    arrayArenas.set(pool, arena);
    return arena;
}

// Keep raw Wasm pointer provenance private to the bridge.
const [GdspxBorrowNativeArray, GetNativeArrayMetadata] = (() => {
    const registry = new WeakMap();

    function borrow(arrayType, count, dataSize, poolName = GDSPX_ARRAY_POOL) {
        if (!Number.isSafeInteger(dataSize) || dataSize < 0 || dataSize > GDSPX_MAX_ARRAY_BYTES) {
            return null;
        }
        if (!IsSafeArrayCount(count)) {
            return null;
        }
        if (!IsNativeArrayByteLength(arrayType, count, dataSize)) {
            return null;
        }
        if (!HasActiveModuleHeap()) {
            return null;
        }

        let arena = GetArrayArena(dataSize, poolName);
        if (!arena || arena.ptr === 0) {
            return null;
        }

        const alignedSize = AlignArrayBytes(dataSize);
        if (alignedSize > arena.capacity) {
            return null;
        }
        if (arena.offset + alignedSize > arena.capacity) {
            // Earlier arguments may still refer to this block until the call finishes.
            arena = GetArrayArena(dataSize, poolName, true);
            if (!arena) {
                return null;
            }
        }

        const ptr = arena.ptr + arena.offset;
        if (!IsHeapRange(ptr, dataSize) || (dataSize > 0 && ptr === 0)) {
            return null;
        }
        arena.offset += alignedSize;
        arena.sequence += 1;

        const metadata = {
            module: Module,
            ptr,
            byteLength: dataSize,
            type: arrayType,
            count,
            sequence: arena.sequence,
            pool: arena.pool,
        };
        Object.freeze(metadata);
        const wrapper = {};
        // Freeze metadata; expose only a bounded data view.
        Object.defineProperties(wrapper, {
            [GDSPX_ARRAY_TAG]: { value: true, enumerable: true },
            'type': { value: arrayType, enumerable: true },
            'count': { value: count, enumerable: true },
            'ptr': { value: ptr, enumerable: true },
            'module': { value: Module, enumerable: true },
            'byteLength': { value: dataSize, enumerable: true },
            'sequence': { value: arena.sequence, enumerable: true },
            'pool': { value: arena.pool, enumerable: true },
            'shared': {
                value: typeof SharedArrayBuffer === 'function' && Module['HEAPU8'].buffer instanceof SharedArrayBuffer,
                enumerable: true,
            },
            'data': {
                configurable: false,
                enumerable: true,
                get() {
                    return NativeArrayDataView(metadata.ptr, metadata.byteLength, metadata.module);
                },
            },
        });
        registry.set(wrapper, metadata);
        return Object.freeze(wrapper);
    }

    function get(array) {
        if (!array || typeof array !== 'object') {
            return null;
        }
        return registry.get(array) || null;
    }

    return [borrow, get];
})();

function NativeArrayType(array) {
    const metadata = GetNativeArrayMetadata(array);
    const value = metadata !== null ?
        (metadata.module === Module ? metadata.type : -1) : Number(array && array['type']);
    return Number.isSafeInteger(value) ? value : -1;
}

function NativeArrayCount(array) {
    const metadata = GetNativeArrayMetadata(array);
    // A wrapper from an old module must not remain usable after a restart.
    const count = metadata !== null ?
        (metadata.module === Module ? metadata.count : -1) : Number(array && array['count']);
    return IsSafeArrayCount(count) ? count : -1;
}

function NativeArrayByteLength(array) {
    const metadata = GetNativeArrayMetadata(array);
    if (metadata !== null) {
        return metadata.module === Module && Number.isSafeInteger(metadata.byteLength) &&
            metadata.byteLength >= 0 && metadata.byteLength <= GDSPX_MAX_ARRAY_BYTES ?
            metadata.byteLength : -1;
    }
    const data = array && array['data'];
    const length = Number(data && data.length);
    return Number.isSafeInteger(length) && length >= 0 && length <= GDSPX_MAX_ARRAY_BYTES ? length : -1;
}

function IsNativeArray(array) {
    if (!array || typeof array !== 'object') {
        return false;
    }
    const arrayType = NativeArrayType(array);
    const count = NativeArrayCount(array);
    const byteLength = NativeArrayByteLength(array);
    return IsNativeArrayByteLength(arrayType, count, byteLength);
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
    return size > 0 && count <= Math.floor(GDSPX_MAX_ARRAY_BYTES / size) &&
        byteLength === count * size;
}

function IsCompatibleArrayType(actualType, expectedType) {
    if (actualType === expectedType) {
        return true;
    }
    return expectedType === GDSPX_ARRAY_TYPE_GDOBJ && actualType === GDSPX_ARRAY_TYPE_INT64;
}

function CopyToNativeArray(array, poolName = GDSPX_INPUT_POOL) {
    if (!IsNativeArray(array)) {
        return null;
    }
    const data = array['data'];
    const dataSize = data.length;
    const count = NativeArrayCount(array);
    // Keep input copies separate from return buffers.
    const borrowed = GdspxBorrowNativeArray(NativeArrayType(array), count, dataSize, poolName);
    if (dataSize > 0 && (!borrowed || borrowed['ptr'] === 0)) {
        throw new Error("Failed to allocate native array input buffer");
    }
    if (dataSize > 0) {
        Module['HEAPU8'].set(data, borrowed['ptr']);
    }
    return borrowed;
}

function GetNativeArrayPointer(array) {
    if (!IsNativeArray(array)) {
        return 0;
    }
    const metadata = GetNativeArrayMetadata(array);
    if (metadata !== null) {
        if (metadata.module !== Module) {
            return 0;
        }
        const byteLength = metadata.byteLength;
        const elemSize = NativeArrayElementSize(metadata.type);
        const ptr = metadata.ptr;
        if (!IsHeapRange(ptr, byteLength) || (byteLength > 0 && ptr === 0) ||
                (byteLength > 0 && elemSize > 1 && ptr % elemSize !== 0)) {
            return 0;
        }
        return ptr;
    }

    const borrowed = CopyToNativeArray(array);
    return borrowed ? borrowed['ptr'] : 0;
}

// Writable calls require the caller's Wasm storage; read-only calls may copy.
function RequireNativeArray(array, opName, expectedType = null, writable = false) {
    if (!array || array[GDSPX_ARRAY_TAG] !== true) {
        throw new Error(opName + " requires a native array");
    }
    if (writable && GetNativeArrayMetadata(array) === null) {
        throw new Error(opName + " requires a pre-allocated Wasm array");
    }
    if (!IsNativeArray(array)) {
        throw new Error(opName + " requires a valid native array shape");
    }
    if (expectedType !== null && NativeArrayType(array) !== expectedType) {
        throw new Error(opName + " received an incompatible native array type");
    }
    const ptr = GetNativeArrayPointer(array);
    if (ptr === 0 && NativeArrayByteLength(array) > 0) {
        throw new Error(opName + " requires accessible native array data");
    }
    return ptr;
}

// Transform bridges return views over the shared return pool.
function TryTransformArray(call, input, inputArrayType, outputArrayType, outputCountScale) {
    if (!IsNativeArray(input)) {
        return null;
    }
    if (!IsCompatibleArrayType(NativeArrayType(input), inputArrayType)) {
        return null;
    }

    const count = NativeArrayCount(input);
    const inputElemSize = NativeArrayElementSize(inputArrayType);
    if (inputElemSize === 0 || NativeArrayByteLength(input) !== count * inputElemSize) {
        return null;
    }

    if (typeof call !== 'function') {
        return null;
    }
    if (!Number.isInteger(outputCountScale) || outputCountScale < 0) {
        return null;
    }
    if (count > 0 && outputCountScale > Math.floor(GDSPX_MAX_ARRAY_ELEMENTS / count)) {
        return null;
    }

    const inputPtr = GetNativeArrayPointer(input);
    if (count > 0 && inputPtr === 0) {
        return null;
    }

    const outCount = count * outputCountScale;
    const outputElemSize = NativeArrayElementSize(outputArrayType);
    if (outputElemSize === 0 || outCount > Math.floor(GDSPX_MAX_ARRAY_BYTES / outputElemSize)) {
        return null;
    }
    const outBytes = outCount * outputElemSize;
    const out = GdspxBorrowNativeArray(
        outputArrayType,
        outCount,
        outBytes,
        GDSPX_RET_POOL,
    );
    const outputPtr = out ? GetNativeArrayPointer(out) : 0;
    if (outputPtr === 0) {
        return null;
    }

    if (count > 0) {
        call(inputPtr, count, outputPtr, outCount);
    }
    return out;
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
    if (!out || GetNativeArrayPointer(out) === 0) {
        return null;
    }
    call(GetNativeArrayPointer(out), NativeArrayCount(out));
    return out;
}

function ToGdArray(array) {
    EnsureGdspxFunctionPointers();
    const data = RequireNativeArray(array, "ToGdArray");
    const wrapper = gdspxBorrowArray(data, NativeArrayByteLength(array), NativeArrayCount(array), NativeArrayType(array));
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

function PrintGdArray(ptr) {
    const val = ToJsArray(ptr);
    console.log(`Array: ${val}`);
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
