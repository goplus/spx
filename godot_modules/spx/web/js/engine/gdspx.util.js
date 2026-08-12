const GDSPX_HAS_BIG_INT64 = typeof DataView.prototype.getBigInt64 === 'function';
const GDSPX_UTF8_ENCODER = new TextEncoder();
const GDSPX_UTF8_DECODER = new TextDecoder("utf-8");

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
    const ptr = gdspxMalloc(stringBytes.length + 1);
    Module['HEAPU8'].set(stringBytes, ptr);
    Module['HEAPU8'][ptr + stringBytes.length] = 0;
    const gdstrPtr = gdspxNewString(ptr, stringBytes.length);
    gdspxFree(ptr);
    return gdstrPtr;
}

function ToJsString(gdstrPtr) {
    return toJsString(gdstrPtr, false);
}

function toJsString(gdstrPtr, isFree) {
    EnsureGdspxFunctionPointers();
    const length = gdspxGetStringLen(gdstrPtr);
    const ptr = gdspxGetString(gdstrPtr);
    const stringBytes = Module['HEAPU8'].subarray(ptr, ptr + length);
    const result = GDSPX_UTF8_DECODER.decode(stringBytes);
    if (isFree) {
        gdspxFreeCstr(ptr);
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

let fastRingModule = null;
const fastRings = new Map();
const deferredFastRingFrees = [];
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
    if (size <= 0) {
        return 0;
    }
    return Math.ceil(size / GDSPX_FAST_RING_ALIGN) * GDSPX_FAST_RING_ALIGN;
}

function NextFastRingCap(minSize) {
    let capacity = GDSPX_FAST_RING_BYTES;
    while (capacity < minSize) {
        capacity *= 2;
    }
    return capacity;
}

function GetFastArrayDataView(ptr, byteLength, module) {
    if (!Number.isInteger(byteLength) || byteLength < 0) {
        return GDSPX_EMPTY_U8;
    }
    if (!HasActiveModuleHeap() || module !== Module) {
        return GDSPX_EMPTY_U8;
    }
    if (!Number.isInteger(ptr) || ptr < 0) {
        return byteLength === 0 ? Module['HEAPU8'].subarray(0, 0) : GDSPX_EMPTY_U8;
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
    if (ring && required <= ring.capacity) {
        return ring;
    }

    const capacity = NextFastRingCap(required);
    const ptr = gdspxMalloc(capacity);
    if (ptr === 0) {
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
    if (!Number.isInteger(dataSize) || dataSize < 0) {
        return null;
    }
    if (!Number.isInteger(count) || count < 0) {
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
    ring.offset += alignedSize;
    ring.sequence += 1;

    const wrapper = {
        '__gdspx_fast_array': true,
        '__gdspx_wasm_array': true,
        'type': arrayType,
        'count': count,
        'ptr': ptr,
        'module': Module,
        'byteLength': dataSize,
        'sequence': ring.sequence,
        'pool': ring.pool,
        'shared': typeof SharedArrayBuffer === 'function' && Module['HEAPU8'].buffer instanceof SharedArrayBuffer,
    };
    Object.defineProperty(wrapper, "data", {
        configurable: true,
        enumerable: true,
        get() {
            return GetFastArrayDataView(this['ptr'], this['byteLength'], this['module']);
        },
    });
    return wrapper;
}

function FastArrayCount(array) {
    const count = Number(array && array['count']);
    return Number.isInteger(count) && count >= 0 ? count : -1;
}

function FastArrayByteLength(array) {
    const data = array && array['data'];
    const length = Number(data && data.length);
    return Number.isInteger(length) && length >= 0 ? length : -1;
}

function IsFastArrayLike(array) {
    if (!array || typeof array !== 'object') {
        return false;
    }
    if (!Number.isInteger(array['type'])) {
        return false;
    }
    return FastArrayCount(array) >= 0 && FastArrayByteLength(array) >= 0;
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
    const data = array['data'];
    const dataSize = data.length;
    const count = FastArrayCount(array);
    if (count < 0) {
        return null;
    }
    // Keep transient input copies separate from return buffers so fast-path
    // calls do not trample results that are still being read by Go.
    const borrowed = GdspxBorrowFastArray(array['type'], count, dataSize, poolName);
    if (dataSize > 0 && (!borrowed || borrowed['ptr'] === 0)) {
        throw new Error("Failed to allocate fast GdArray input buffer");
    }
    if (dataSize > 0) {
        Module['HEAPU8'].set(data, borrowed['ptr']);
    }
    return borrowed;
}

function GetFastArrayWasmPtr(array) {
    if (array['__gdspx_wasm_array'] === true) {
        if (array['module'] !== Module) {
            return 0;
        }
        if (!Number.isInteger(array['ptr']) || array['ptr'] < 0) {
            return 0;
        }
        return array['ptr'];
    }

    const borrowed = BorrowCopiedFastArray(array);
    return borrowed ? borrowed['ptr'] : 0;
}

function RequireWasmFastArray(array, opName) {
    if (!array || array['__gdspx_wasm_array'] !== true) {
        throw new Error(opName + " requires a pre-allocated Wasm array");
    }
    if (array['module'] !== Module) {
        throw new Error(opName + " requires a Wasm array from the active module");
    }
    if (!Number.isInteger(array['ptr']) || array['ptr'] < 0) {
        throw new Error(opName + " requires a valid Wasm array pointer");
    }
    return array['ptr'];
}

// Array-transform bridges take a fast-array input, run a raw wasm transform,
// and return another fast-array view over the shared return pool.
function TryArrayTransformFastPath(call, input, inputArrayType, outputArrayType, outputCountScale) {
    if (!IsFastArrayLike(input)) {
        return null;
    }
    if (!IsCompatibleFastArrayType(input['type'], inputArrayType)) {
        return null;
    }

    const count = FastArrayCount(input);
    if (count < 0 || count > 0x3fffffff) {
        return null;
    }
    const inputElemSize = GetFastArrayElemSize(inputArrayType);
    if (inputElemSize === 0 || FastArrayByteLength(input) < count * inputElemSize) {
        return null;
    }

    if (typeof call !== 'function') {
        return null;
    }
    if (!Number.isInteger(outputCountScale) || outputCountScale < 0) {
        return null;
    }
    if (count > 0 && outputCountScale > Math.floor(0x7fffffff / count)) {
        return null;
    }

    const inputPtr = GetFastArrayWasmPtr(input);
    if (count > 0 && inputPtr === 0) {
        return null;
    }

    const outCount = count * outputCountScale;
    const outputElemSize = GetFastArrayElemSize(outputArrayType);
    if (outputElemSize === 0 || outCount > Math.floor(0x7fffffff / outputElemSize)) {
        return null;
    }
    const outBytes = outCount * outputElemSize;
    const out = GdspxBorrowFastArray(
        outputArrayType,
        outCount,
        outBytes,
        GDSPX_RET_POOL,
    );
    if (!out || out['ptr'] === 0) {
        return null;
    }

    if (count > 0) {
        call(inputPtr, count, out['ptr'], outCount);
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
    if (!out || out['ptr'] === 0) {
        return null;
    }
    call(out['ptr'], out['count']);
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
    if (!buffer || buffer['__gdspx_fast_array'] !== true) {
        return false;
    }
    if (buffer['type'] !== GDSPX_ARRAY_TYPE_FLOAT) {
        return false;
    }
    const count = FastArrayCount(buffer);
    if (count <= 0 || FastArrayByteLength(buffer) < count * 4) {
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
        const dataPtr = GetFastArrayWasmPtr(array);
        if (array['data'].length > 0 && dataPtr === 0) {
            throw new Error("Failed to access fast GdArray data");
        }
        return gdspxToGdArrayRaw(dataPtr, array['data'].length, array['count'], array['type']);
    }
    const dataSize = array.length;
    const dataPtr = gdspxMalloc(dataSize);
    try {
        if (dataSize > 0) {
            Module['HEAPU8'].set(array, dataPtr);
        }
        const gdArrayPtr = gdspxToGdArray(dataPtr, dataSize);
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
    const outputSizePtr = gdspxMalloc(4);
    try {
        const serializedPtr = gdspxToJsArray(gdArrayPtr, outputSizePtr);
        if (!serializedPtr) {
            return null;
        }
        const outputSize = Module['HEAP32'][outputSizePtr >> 2];
        const data = new Uint8Array(outputSize);
        data.set(Module['HEAPU8'].subarray(serializedPtr, serializedPtr + outputSize));
        gdspxFree(serializedPtr);
        return data;
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
