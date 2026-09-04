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
let gdspxToGdArray = null;
let gdspxToGdArrayRaw = null;
let gdspxToJsArray = null;

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
    gdspxToGdArray = module['_gdspx_to_gd_array'];
    gdspxToGdArrayRaw = module['_gdspx_to_gd_array_raw'];
    gdspxToJsArray = module['_gdspx_to_js_array'];
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

function ToGdObj(value) {
    EnsureGdspxFunctionPointers();
    return gdspxNewObj(value['high'], value['low']);
}

function ToJsObj(ptr) {
    const dataView = GetHeapDataView();
    const low = dataView.getUint32(ptr, true);
    const high = dataView.getUint32(ptr + 4, true);
    return {
        'low': low,
        'high': high
    };
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

function ToJsInt(ptr) {
    const dataView = GetHeapDataView();
    const low = dataView.getUint32(ptr, true);  // 低32位
    const high = dataView.getUint32(ptr + 4, true);  // 高32位
    return {
        'low': low,
        'high': high
    };
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

function ToJsVec2(ptr) {
    const HEAPF32 = Module['HEAPF32'];
    const floatIndex = ptr / 4;
    return {
        'x': HEAPF32[floatIndex],
        'y': HEAPF32[floatIndex + 1]
    };
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

function ToJsVec3(ptr) {
    const HEAPF32 = Module['HEAPF32'];
    const floatIndex = ptr / 4;
    return {
        'x': HEAPF32[floatIndex],
        'y': HEAPF32[floatIndex + 1],
        'z': HEAPF32[floatIndex + 2]
    };
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

function ToJsVec4(ptr) {
    const HEAPF32 = Module['HEAPF32'];
    const floatIndex = ptr / 4;
    return {
        'x': HEAPF32[floatIndex],
        'y': HEAPF32[floatIndex + 1],
        'z': HEAPF32[floatIndex + 2],
        'w': HEAPF32[floatIndex + 3]
    };
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

function ToJsColor(ptr) {
    const HEAPF32 = Module['HEAPF32'];
    const floatIndex = ptr / 4;
    return {
        'r': HEAPF32[floatIndex],
        'g': HEAPF32[floatIndex + 1],
        'b': HEAPF32[floatIndex + 2],
        'a': HEAPF32[floatIndex + 3]
    };
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

function ToJsRect2(ptr) {
    const HEAPF32 = Module['HEAPF32'];
    const floatIndex = ptr / 4;
    return {
        'position': {
            'x': HEAPF32[floatIndex],
            'y': HEAPF32[floatIndex + 1]
        },
        'size': {
            'x': HEAPF32[floatIndex + 2],
            'y': HEAPF32[floatIndex + 3]
        }
    };
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
// Fast Arrays and Shared Wasm Transfer Buffers
// -----------------------------------------------------------------------------

const GDSPX_ARRAY_TYPE_INT64 = 1;
const GDSPX_ARRAY_TYPE_FLOAT = 2;
const GDSPX_ARRAY_TYPE_BYTE = 5;
const GDSPX_ARRAY_TYPE_GDOBJ = 6;
const GDSPX_FAST_RING_BYTES = 1024 * 1024;
const GDSPX_FAST_RING_ALIGN = 8;
const GDSPX_FAST_POOL = "default";
const GDSPX_INPUT_POOL = "input";
const GDSPX_INPUT_SNAP_POOL = "input-snapshot";
const GDSPX_RET_POOL = "return";
const GDSPX_EMPTY_U8 = new Uint8Array(0);
const GDSPX_MAX_ARRAY_ELEMENTS = 16 * 1024 * 1024;
const GDSPX_MAX_ARRAY_BYTES = 256 * 1024 * 1024;
const GDSPX_MAX_I32 = 0x7fffffff;
const GDSPX_MAX_ALIGNED_BYTES = GDSPX_MAX_I32 - (GDSPX_MAX_I32 % GDSPX_FAST_RING_ALIGN);

let fastRingModule = null;
const fastRings = new Map();
const deferredFastRingFrees = [];
// A fast-array wrapper contains a raw Wasm pointer. Keep its provenance in a
// private registry instead of trusting the JavaScript-visible metadata. This
// prevents callers from manufacturing a shape-valid wrapper that redirects a
// raw bridge into an arbitrary range of the active Wasm heap.
const gdspxTrustedFastArrayMetadata = new WeakMap();
const inputActionRegistry = {
    module: null,
    epoch: 0,
    ids: new Map(),
};
let inputBridgeModule = null;
let inputBridge = null;

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

function AlignFastSize(size) {
    if (!Number.isSafeInteger(size) || size <= 0 || size > GDSPX_MAX_ALIGNED_BYTES) {
        return 0;
    }
    const aligned = Math.ceil(size / GDSPX_FAST_RING_ALIGN) * GDSPX_FAST_RING_ALIGN;
    return aligned <= GDSPX_MAX_ALIGNED_BYTES ? aligned : 0;
}

function NextFastRingCap(minSize) {
    let capacity = GDSPX_FAST_RING_BYTES;
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

function GetFastArrayDataView(ptr, byteLength, module) {
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

function QueueDeferredFastRingFree(ptr, freeFn) {
    if (!Number.isInteger(ptr) || ptr === 0 || typeof freeFn !== 'function') {
        return;
    }
    deferredFastRingFrees.push({ ptr, free: freeFn });
}

function GdspxFlushDeferredFrees() {
    if (deferredFastRingFrees.length === 0) {
        return;
    }
    const pending = deferredFastRingFrees.splice(0, deferredFastRingFrees.length);
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

function GetFastRing(minSize, poolName = GDSPX_FAST_POOL) {
    EnsureGdspxFunctionPointers();
    if (typeof gdspxMalloc !== 'function' || typeof gdspxFree !== 'function') {
        return null;
    }
    if (fastRingModule !== Module) {
        FreePtrMap(fastRings);
        fastRingModule = Module;
    }

    const pool = String(poolName || GDSPX_FAST_POOL);
    let ring = fastRings.get(pool);
    const required = AlignFastSize(minSize);
    if (required === 0 && minSize !== 0) {
        return null;
    }
    if (ring && required <= ring.capacity) {
        return ring;
    }

    const capacity = NextFastRingCap(required);
    if (capacity === 0 || capacity > GDSPX_MAX_ARRAY_BYTES) {
        return null;
    }
    const ptr = gdspxMalloc(capacity);
    if (!Number.isSafeInteger(ptr) || ptr <= 0 || !IsHeapRange(ptr, capacity)) {
        return null;
    }
    if (ring && ring.ptr !== 0) {
        QueueDeferredFastRingFree(ring.ptr, ring.free);
    }

    ring = {
        ptr,
        capacity,
        offset: 0,
        sequence: 0,
        module: Module,
        free: gdspxFree,
        pool,
    };
    fastRings.set(pool, ring);
    return ring;
}

function GdspxBorrowFastArray(arrayType, count, dataSize, poolName = GDSPX_FAST_POOL) {
    if (!Number.isSafeInteger(dataSize) || dataSize < 0 || dataSize > GDSPX_MAX_ARRAY_BYTES) {
        return null;
    }
    if (!IsSafeArrayCount(count)) {
        return null;
    }
    const elemSize = GetFastArrayElemSize(arrayType);
    if (elemSize === 0 || count > Math.floor(GDSPX_MAX_ARRAY_BYTES / elemSize) ||
            dataSize !== count * elemSize) {
        return null;
    }
    if (!HasActiveModuleHeap()) {
        return null;
    }

    const ring = GetFastRing(dataSize, poolName);
    if (!ring || ring.ptr === 0) {
        return null;
    }

    const alignedSize = AlignFastSize(dataSize);
    if (alignedSize > ring.capacity) {
        return null;
    }
    if (ring.offset + alignedSize > ring.capacity) {
        ring.offset = 0;
    }

    const ptr = ring.ptr + ring.offset;
    if (!IsHeapRange(ptr, dataSize) || (dataSize > 0 && ptr === 0)) {
        return null;
    }
    ring.offset += alignedSize;
    ring.sequence += 1;

    const metadata = {
        module: Module,
        ptr,
        byteLength: dataSize,
        type: arrayType,
        count,
        sequence: ring.sequence,
        pool: ring.pool,
    };
    Object.freeze(metadata);
    const wrapper = {};
    // Define the ABI fields as immutable own properties. The data view remains
    // writable, but its address and extent are captured from the private
    // metadata rather than read from a mutable object field.
    Object.defineProperties(wrapper, {
        '__gdspx_fast_array': { value: true, enumerable: true },
        '__gdspx_wasm_array': { value: true, enumerable: true },
        'type': { value: arrayType, enumerable: true },
        'count': { value: count, enumerable: true },
        'ptr': { value: ptr, enumerable: true },
        'module': { value: Module, enumerable: true },
        'byteLength': { value: dataSize, enumerable: true },
        'sequence': { value: ring.sequence, enumerable: true },
        'pool': { value: ring.pool, enumerable: true },
        'shared': {
            value: typeof SharedArrayBuffer === 'function' && Module['HEAPU8'].buffer instanceof SharedArrayBuffer,
            enumerable: true,
        },
        'data': {
            configurable: false,
            enumerable: true,
            get() {
                return GetFastArrayDataView(metadata.ptr, metadata.byteLength, metadata.module);
            },
        },
    });
    gdspxTrustedFastArrayMetadata.set(wrapper, metadata);
    return Object.freeze(wrapper);
}

function GetTrustedFastArrayMetadata(array) {
    if (!array || typeof array !== 'object') {
        return null;
    }
    return gdspxTrustedFastArrayMetadata.get(array) || null;
}

function FastArrayType(array) {
    const metadata = GetTrustedFastArrayMetadata(array);
    const value = metadata !== null ?
        (metadata.module === Module ? metadata.type : -1) : Number(array && array['type']);
    return Number.isSafeInteger(value) ? value : -1;
}

function FastArrayCount(array) {
    const metadata = GetTrustedFastArrayMetadata(array);
    // A wrapper from an old module must not remain usable after a restart.
    const count = metadata !== null ?
        (metadata.module === Module ? metadata.count : -1) : Number(array && array['count']);
    return IsSafeArrayCount(count) ? count : -1;
}

function FastArrayByteLength(array) {
    const metadata = GetTrustedFastArrayMetadata(array);
    if (metadata !== null) {
        return metadata.module === Module && Number.isSafeInteger(metadata.byteLength) &&
            metadata.byteLength >= 0 && metadata.byteLength <= GDSPX_MAX_ARRAY_BYTES ?
            metadata.byteLength : -1;
    }
    const data = array && array['data'];
    const length = Number(data && data.length);
    return Number.isSafeInteger(length) && length >= 0 && length <= GDSPX_MAX_ARRAY_BYTES ? length : -1;
}

function IsFastArrayLike(array) {
    if (!array || typeof array !== 'object') {
        return false;
    }
    const arrayType = FastArrayType(array);
    if (!Number.isSafeInteger(arrayType) || GetFastArrayElemSize(arrayType) === 0) {
        return false;
    }
    const count = FastArrayCount(array);
    const byteLength = FastArrayByteLength(array);
    const elemSize = GetFastArrayElemSize(arrayType);
    return count >= 0 && byteLength >= 0 && count <= Math.floor(GDSPX_MAX_ARRAY_BYTES / elemSize) &&
        byteLength === count * elemSize;
}

function GetFastArrayElemSize(arrayType) {
    switch (arrayType) {
    case GDSPX_ARRAY_TYPE_INT64:
    case GDSPX_ARRAY_TYPE_GDOBJ:
        return 8;
    case GDSPX_ARRAY_TYPE_FLOAT:
        return 4;
    case GDSPX_ARRAY_TYPE_BYTE:
        return 1;
    default:
        return 0;
    }
}

function IsCompatibleFastArrayType(actualType, expectedType) {
    if (actualType === expectedType) {
        return true;
    }
    return expectedType === GDSPX_ARRAY_TYPE_GDOBJ && actualType === GDSPX_ARRAY_TYPE_INT64;
}

function BorrowCopiedFastArray(array, poolName = GDSPX_INPUT_POOL) {
    if (!IsFastArrayLike(array)) {
        return null;
    }
    const data = array['data'];
    const dataSize = data.length;
    const count = FastArrayCount(array);
    if (count < 0) {
        return null;
    }
    // Keep transient input copies separate from return buffers so fast-path
    // calls do not trample results that are still being read by Go.
    const borrowed = GdspxBorrowFastArray(FastArrayType(array), count, dataSize, poolName);
    if (dataSize > 0 && (!borrowed || borrowed['ptr'] === 0)) {
        throw new Error("Failed to allocate fast GdArray input buffer");
    }
    if (dataSize > 0) {
        Module['HEAPU8'].set(data, borrowed['ptr']);
    }
    return borrowed;
}

function GetFastArrayWasmPtr(array) {
    if (!IsFastArrayLike(array)) {
        return 0;
    }
    if (array['__gdspx_wasm_array'] === true) {
        const metadata = GetTrustedFastArrayMetadata(array);
        if (metadata === null || metadata.module !== Module || array['module'] !== Module) {
            return 0;
        }
        const byteLength = metadata.byteLength;
        const elemSize = GetFastArrayElemSize(metadata.type);
        const ptr = metadata.ptr;
        if (!IsHeapRange(ptr, byteLength) || (byteLength > 0 && ptr === 0) ||
                (byteLength > 0 && elemSize > 1 && ptr % elemSize !== 0)) {
            return 0;
        }
        return ptr;
    }

    const borrowed = BorrowCopiedFastArray(array);
    return borrowed ? borrowed['ptr'] : 0;
}

function RequireWasmFastArray(array, opName, expectedType = null) {
    if (!array || array['__gdspx_wasm_array'] !== true) {
        throw new Error(opName + " requires a pre-allocated Wasm array");
    }
    if (GetTrustedFastArrayMetadata(array) === null) {
        throw new Error(opName + " requires an internally allocated Wasm array");
    }
    if (array['module'] !== Module) {
        throw new Error(opName + " requires a Wasm array from the active module");
    }
    if (!IsFastArrayLike(array)) {
        throw new Error(opName + " requires a valid Wasm array shape");
    }
    if (expectedType !== null && FastArrayType(array) !== expectedType) {
        throw new Error(opName + " received an incompatible Wasm array type");
    }
    const ptr = GetFastArrayWasmPtr(array);
    if (ptr === 0 && FastArrayByteLength(array) > 0) {
        throw new Error(opName + " requires a valid Wasm array pointer");
    }
    return ptr;
}

// Native array bridges may receive either a zero-copy Wasm view or a regular
// fast-array wrapper. In the latter case GetFastArrayWasmPtr creates a checked
// transient input copy before invoking the native function.
function RequireFastArray(array, opName, expectedType = null) {
    if (!array || array['__gdspx_fast_array'] !== true) {
        throw new Error(opName + " requires a fast array");
    }
    if (array['__gdspx_wasm_array'] === true && GetTrustedFastArrayMetadata(array) === null) {
        throw new Error(opName + " requires an internally allocated Wasm array");
    }
    if (!IsFastArrayLike(array)) {
        throw new Error(opName + " requires a valid fast array shape");
    }
    if (expectedType !== null && FastArrayType(array) !== expectedType) {
        throw new Error(opName + " received an incompatible fast array type");
    }
    const byteLength = FastArrayByteLength(array);
    const ptr = GetFastArrayWasmPtr(array);
    if (ptr === 0 && byteLength > 0) {
        throw new Error(opName + " requires accessible fast array data");
    }
    return ptr;
}

// Array-transform bridges take a fast-array input, run a raw wasm transform,
// and return another fast-array view over the shared return pool.
function TryArrayTransformFastPath(call, input, inputArrayType, outputArrayType, outputCountScale) {
    if (!IsFastArrayLike(input)) {
        return null;
    }
    if (input['__gdspx_wasm_array'] === true && GetTrustedFastArrayMetadata(input) === null) {
        return null;
    }
    if (!IsCompatibleFastArrayType(FastArrayType(input), inputArrayType)) {
        return null;
    }

    const count = FastArrayCount(input);
    if (count < 0 || count > GDSPX_MAX_ARRAY_ELEMENTS) {
        return null;
    }
    const inputElemSize = GetFastArrayElemSize(inputArrayType);
    if (inputElemSize === 0 || FastArrayByteLength(input) !== count * inputElemSize) {
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

    const inputPtr = GetFastArrayWasmPtr(input);
    if (count > 0 && inputPtr === 0) {
        return null;
    }

    const outCount = count * outputCountScale;
    const outputElemSize = GetFastArrayElemSize(outputArrayType);
    if (outputElemSize === 0 || outCount > Math.floor(GDSPX_MAX_ARRAY_BYTES / outputElemSize)) {
        return null;
    }
    const outBytes = outCount * outputElemSize;
    const out = GdspxBorrowFastArray(
        outputArrayType,
        outCount,
        outBytes,
        GDSPX_RET_POOL,
    );
    if (!out || GetFastArrayWasmPtr(out) === 0) {
        return null;
    }

    if (count > 0) {
        call(inputPtr, count, GetFastArrayWasmPtr(out), outCount);
    }
    return out;
}

function GdspxInputSnapshot() {
    if (!HasActiveModule()) {
        return null;
    }
    const call = Module['_gdspx_input_write_snapshot'];
    if (typeof call !== 'function') {
        return null;
    }

    const out = GdspxBorrowFastArray(GDSPX_ARRAY_TYPE_FLOAT, 3, 12, GDSPX_INPUT_SNAP_POOL);
    if (!out || GetFastArrayWasmPtr(out) === 0) {
        return null;
    }
    call(GetFastArrayWasmPtr(out), FastArrayCount(out));
    return out;
}

// -----------------------------------------------------------------------------
// Input Bridge and Action-ID Cache
// -----------------------------------------------------------------------------

function GetInputBridge() {
    if (!HasActiveModule()) {
        return null;
    }
    if (typeof globalThis['gdspx_input_register_action'] === 'function') {
        return globalThis;
    }
    if (inputBridgeModule !== Module) {
        inputBridgeModule = Module;
        inputBridge = typeof GdspxFuncs === 'function' ? new GdspxFuncs() : null;
    }
    return inputBridge;
}

function ToStableActionID(value) {
    if (value && typeof value['low'] === 'number') {
        return value['low'] | 0;
    }
    if (typeof value === 'bigint') {
        return Number(BigInt.asIntN(32, value));
    }
    return Number(value) | 0;
}

// Input bridge wrappers expose simple JS calls while hiding whether the active
// implementation comes from direct globals or a generated GdspxFuncs instance.
function GetBoundBridgeCall(bridge, functionName) {
    if (!bridge || typeof functionName !== 'string') {
        return null;
    }
    const call = bridge[functionName];
    return typeof call === 'function' ? call.bind(bridge) : null;
}

function GetInputActionBoolCallName(kind) {
    switch (kind | 0) {
        case 1:
            return "gdspx_input_is_action_pressed_id";
        case 2:
            return "gdspx_input_is_action_just_pressed_id";
        case 3:
            return "gdspx_input_is_action_just_released_id";
        default:
            return null;
    }
}

function EnsureInputActionRegistry() {
    if (!HasActiveModule()) {
        return false;
    }
    if (inputActionRegistry.module !== Module) {
        inputActionRegistry.module = Module;
        inputActionRegistry.epoch += 1;
        inputActionRegistry.ids.clear();
    }
    return true;
}

function GdspxInputActionEpoch() {
    EnsureInputActionRegistry();
    return inputActionRegistry.epoch;
}

function GdspxInputActionID(action) {
    if (!EnsureInputActionRegistry()) {
        return -1;
    }
    if (inputActionRegistry.ids.has(action)) {
        return inputActionRegistry.ids.get(action);
    }

    const bridge = GetInputBridge();
    const call = GetBoundBridgeCall(bridge, "gdspx_input_register_action");
    if (typeof call !== 'function') {
        return -1;
    }

    const id = ToStableActionID(call(action));
    if (id >= 0) {
        inputActionRegistry.ids.set(action, id);
    }
    return id;
}

function GdspxInputActionBool(kind, actionID) {
    if (!EnsureInputActionRegistry()) {
        return null;
    }

    const bridge = GetInputBridge();
    const callName = GetInputActionBoolCallName(kind);
    if (callName == null) {
        return null;
    }
    const call = GetBoundBridgeCall(bridge, callName);
    if (typeof call !== 'function') {
        return null;
    }
    return !!call(actionID | 0, 0);
}

function GdspxInputAxisByID(negActionID, posActionID) {
    if (!EnsureInputActionRegistry()) {
        return null;
    }
    const bridge = GetInputBridge();
    const call = GetBoundBridgeCall(bridge, "gdspx_input_get_axis_id");
    if (typeof call !== 'function') {
        return null;
    }
    return Number(call(negActionID | 0, 0, posActionID | 0, 0));
}

// -----------------------------------------------------------------------------
// Batch Helpers and Generic Array Serialization
// -----------------------------------------------------------------------------

function GdspxBatchSpritePhysics(buffer) {
    if (buffer && buffer['__gdspx_wasm_array'] === true &&
            GetTrustedFastArrayMetadata(buffer) === null) {
        return false;
    }
    if (!IsFastArrayLike(buffer) || buffer['__gdspx_fast_array'] !== true) {
        return false;
    }
    if (FastArrayType(buffer) !== GDSPX_ARRAY_TYPE_FLOAT) {
        return false;
    }
    const count = FastArrayCount(buffer);
    if (count <= 0 || FastArrayByteLength(buffer) !== count * 4) {
        return false;
    }
    if (!HasActiveModule()) {
        return false;
    }
    const call = Module['_gdspx_sprite_batch_update_physics'];
    if (typeof call !== 'function') {
        return false;
    }
    const ptr = GetFastArrayWasmPtr(buffer);
    if (ptr === 0) {
        return false;
    }
    call(ptr, count);
    return true;
}

function ToGdArray(array) {
    EnsureGdspxFunctionPointers();
    if (!array) {
        throw new Error('Invalid array structure. Expected {type, count, data}');
    }
    if (array['__gdspx_fast_array'] === true) {
        if (array['__gdspx_wasm_array'] === true && GetTrustedFastArrayMetadata(array) === null) {
            throw new Error("Invalid untrusted Wasm fast GdArray");
        }
        if (!IsFastArrayLike(array)) {
            throw new Error("Invalid fast GdArray structure");
        }
        const dataSize = FastArrayByteLength(array);
        const count = FastArrayCount(array);
        if (dataSize < 0 || count < 0) {
            throw new Error("Invalid fast GdArray length");
        }
        const dataPtr = GetFastArrayWasmPtr(array);
        if (dataSize > 0 && dataPtr === 0) {
            throw new Error("Failed to access fast GdArray data");
        }
        const gdArrayPtr = gdspxToGdArrayRaw(dataPtr, dataSize, count, FastArrayType(array));
        if (!gdArrayPtr) {
            throw new Error("Failed to deserialize fast GdArray");
        }
        return gdArrayPtr;
    }
    if (!(array instanceof Uint8Array) || !Number.isSafeInteger(array.length) ||
            array.length < 8 || array.length > GDSPX_MAX_ARRAY_BYTES) {
        throw new Error("Invalid serialized GdArray data");
    }
    const dataSize = array.length;
    if (typeof gdspxMalloc !== 'function' || typeof gdspxFree !== 'function') {
        throw new Error("GdArray allocator is unavailable");
    }
    const dataPtr = gdspxMalloc(dataSize);
    if (!Number.isSafeInteger(dataPtr) || dataPtr <= 0 || !IsHeapRange(dataPtr, dataSize)) {
        throw new Error("Failed to allocate GdArray input buffer");
    }
    try {
        if (dataSize > 0) {
            Module['HEAPU8'].set(array, dataPtr);
        }
        const gdArrayPtr = gdspxToGdArray(dataPtr, dataSize);
        if (!gdArrayPtr) {
            throw new Error("Failed to deserialize GdArray");
        }
        return gdArrayPtr;
    } finally {
        gdspxFree(dataPtr);
    }
}

function ToJsArray(gdArrayPtr) {
    EnsureGdspxFunctionPointers();
    if (!gdArrayPtr) {
        return null;
    }
    if (typeof gdspxMalloc !== 'function' || typeof gdspxFree !== 'function') {
        return null;
    }
    const outputSizePtr = gdspxMalloc(4);
    if (!Number.isSafeInteger(outputSizePtr) || outputSizePtr <= 0 ||
            outputSizePtr % 4 !== 0 || !IsHeapRange(outputSizePtr, 4)) {
        return null;
    }
    try {
        const serializedPtr = gdspxToJsArray(gdArrayPtr, outputSizePtr);
        if (!serializedPtr) {
            return null;
        }
        const outputSize = Module['HEAP32'][outputSizePtr >> 2];
        const canFreeSerializedPtr = Number.isSafeInteger(serializedPtr) && serializedPtr > 0 &&
            IsHeapRange(serializedPtr, 0);
        if (!Number.isSafeInteger(outputSize) || outputSize < 8 ||
                outputSize > GDSPX_MAX_ARRAY_BYTES || !canFreeSerializedPtr ||
                !IsHeapRange(serializedPtr, outputSize)) {
            if (canFreeSerializedPtr) {
                gdspxFree(serializedPtr);
            }
            return null;
        }
        const data = new Uint8Array(outputSize);
        try {
            data.set(Module['HEAPU8'].subarray(serializedPtr, serializedPtr + outputSize));
            return data;
        } finally {
            gdspxFree(serializedPtr);
        }
    } finally {
        gdspxFree(outputSizePtr);
    }
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
globalThis['GdspxBorrowFastArray'] = GdspxBorrowFastArray;
globalThis['GdspxInputSnapshot'] = GdspxInputSnapshot;
globalThis['GdspxInputActionEpoch'] = GdspxInputActionEpoch;
globalThis['GdspxInputActionID'] = GdspxInputActionID;
globalThis['GdspxInputActionBool'] = GdspxInputActionBool;
globalThis['GdspxInputAxisByID'] = GdspxInputAxisByID;
globalThis['GdspxBatchSpritePhysics'] = GdspxBatchSpritePhysics;
