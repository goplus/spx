//go:build js && wasm

package webffi

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"syscall/js"
	"unsafe"

	. "github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v2/internal/log"
	. "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

const (
	GdArrayTypeUnknown = 0
	GdArrayTypeInt64   = 1
	GdArrayTypeFloat   = 2
	GdArrayTypeBool    = 3
	GdArrayTypeString  = 4
	GdArrayTypeByte    = 5
	GdArrayTypeGdObj   = 6
)

const (
	fastGdArrayFlagKey  = "__gdspx_fast_array"
	fastGdArrayTypeKey  = "type"
	fastGdArrayCountKey = "count"
	fastGdArrayDataKey  = "data"
)

var (
	jsObject     = js.Global().Get("Object")
	jsUint8Array = js.Global().Get("Uint8Array")

	fastGdArrayScratchByType = map[int32]*fastGdArrayScratch{}
)

type fastGdArrayScratch struct {
	bytes   js.Value
	wrapper js.Value
	byteCap int
}

type GdArrayInfo struct {
	Size int32
	Type int32
	Data any
}

func serializeGdArray(info *GdArrayInfo) ([]byte, error) {
	if info == nil {
		return nil, fmt.Errorf("nil GdArrayInfo")
	}

	dataBytes, err := serializeDataByType(info.Type, info.Data)
	if err != nil {
		return nil, err
	}

	totalSize := 8 + len(dataBytes)
	result := make([]byte, totalSize)

	*(*uint32)(unsafe.Pointer(&result[0])) = uint32(info.Size)
	*(*uint32)(unsafe.Pointer(&result[4])) = uint32(info.Type)

	copy(result[8:], dataBytes)

	return result, nil
}

func deserializeGdArray(data []byte) (*GdArrayInfo, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("data length is not enough")
	}

	size := int32(*(*uint32)(unsafe.Pointer(&data[0])))
	arrayType := int32(*(*uint32)(unsafe.Pointer(&data[4])))

	arrayData, err := deserializeDataByType(arrayType, data[8:], size)
	if err != nil {
		return nil, err
	}

	return &GdArrayInfo{
		Size: size,
		Type: arrayType,
		Data: arrayData,
	}, nil
}

func f64Tof32(slice []float64) []float32 {
	if slice == nil {
		return []float32{}
	}
	out := make([]float32, len(slice))
	for i, v := range slice {
		out[i] = float32(v)
	}
	return out
}

func serializeDataByType(arrayType int32, data any) ([]byte, error) {
	if data == nil {
		return []byte{}, nil
	}

	switch arrayType {
	case GdArrayTypeInt64, GdArrayTypeGdObj:
		if arr := data.([]int64); len(arr) == 0 {
			return []byte{}, nil
		}
		return serializeInt64Array(data.([]int64))
	case GdArrayTypeFloat:
		val, ok := data.([]float32)
		if !ok {
			slice, ok := data.([]float64)
			if !ok {
				return []byte{}, fmt.Errorf("array type is not supported: %T", data)
			}
			val = f64Tof32(slice)
		}
		if arr := val; len(arr) == 0 {
			return []byte{}, nil
		}
		return serializeFloatArray(val)
	case GdArrayTypeBool:
		if arr := data.([]bool); len(arr) == 0 {
			return []byte{}, nil
		}
		return serializeBoolArray(data.([]bool))
	case GdArrayTypeByte:
		if arr := data.([]byte); len(arr) == 0 {
			return []byte{}, nil
		}
		return data.([]byte), nil
	case GdArrayTypeString:
		if arr := data.([]string); len(arr) == 0 {
			return []byte{}, nil
		}
		return serializeStringArray(data.([]string))
	default:
		return nil, fmt.Errorf("array type is not supported: %d", arrayType)
	}
}

func deserializeDataByType(arrayType int32, data []byte, size int32) (any, error) {
	if len(data) == 0 || size == 0 {
		switch arrayType {
		case GdArrayTypeInt64, GdArrayTypeGdObj:
			return []int64{}, nil
		case GdArrayTypeFloat:
			return []float32{}, nil
		case GdArrayTypeBool:
			return []bool{}, nil
		case GdArrayTypeByte:
			return []byte{}, nil
		case GdArrayTypeString:
			return []string{}, nil
		default:
			return nil, fmt.Errorf("array type is not supported: %d", arrayType)
		}
	}

	switch arrayType {
	case GdArrayTypeInt64, GdArrayTypeGdObj:
		return deserializeInt64Array(data, size)
	case GdArrayTypeFloat:
		return deserializeFloatArray(data, size)
	case GdArrayTypeBool:
		return deserializeBoolArray(data, size)
	case GdArrayTypeByte:
		return data, nil
	case GdArrayTypeString:
		return deserializeStringArray(data)
	default:
		return nil, fmt.Errorf("array type is not supported: %d", arrayType)
	}
}

func serializeInt64Array(data []int64) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}
	return bytesFromInt64Slice(data), nil
}

func deserializeInt64Array(data []byte, size int32) ([]int64, error) {
	if size < 0 {
		return nil, fmt.Errorf("array size is invalid")
	}
	requiredBytes := int64(size) * 8
	if int64(len(data)) < requiredBytes {
		return nil, fmt.Errorf("array data length is not enough")
	}
	if size == 0 {
		return []int64{}, nil
	}
	return unsafe.Slice((*int64)(unsafe.Pointer(unsafe.SliceData(data))), int(size)), nil
}

func serializeFloatArray(data []float32) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}
	return bytesFromFloat32Slice(data), nil
}

func deserializeFloatArray(data []byte, size int32) ([]float32, error) {
	if size < 0 {
		return nil, fmt.Errorf("array size is invalid")
	}
	requiredBytes := int64(size) * 4
	if int64(len(data)) < requiredBytes {
		return nil, fmt.Errorf("array data length is not enough")
	}
	if size == 0 {
		return []float32{}, nil
	}
	return unsafe.Slice((*float32)(unsafe.Pointer(unsafe.SliceData(data))), int(size)), nil
}

func serializeBoolArray(data []bool) ([]byte, error) {
	result := make([]byte, len(data))
	for i, val := range data {
		if val {
			result[i] = 1
		} else {
			result[i] = 0
		}
	}
	return result, nil
}

func deserializeBoolArray(data []byte, size int32) ([]bool, error) {
	if len(data) < int(size) {
		return nil, fmt.Errorf("array data length is not enough")
	}

	result := make([]bool, size)
	for i := 0; i < int(size); i++ {
		result[i] = data[i] != 0
	}
	return result, nil
}

func serializeStringArray(data []string) ([]byte, error) {
	var result []byte
	for _, str := range data {
		strBytes := []byte(str)
		lengthBytes := make([]byte, 4)

		*(*uint32)(unsafe.Pointer(&lengthBytes[0])) = uint32(len(strBytes))

		result = append(result, lengthBytes...)
		result = append(result, strBytes...)
	}
	return result, nil
}

func deserializeStringArray(data []byte) ([]string, error) {
	var result []string
	offset := 0

	for offset < len(data) {
		if offset+4 > len(data) {
			break
		}

		strLen := int(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4

		if offset+strLen > len(data) {
			return nil, fmt.Errorf("string data is not complete")
		}

		str := string(data[offset : offset+strLen])
		result = append(result, str)
		offset += strLen
	}

	return result, nil
}

func arrayToGdArrayInfo(arrayPtr Array) *GdArrayInfo {
	switch data := arrayPtr.(type) {
	case []int64:
		return &GdArrayInfo{Size: int32(len(data)), Type: GdArrayTypeInt64, Data: data}
	case []float32:
		return &GdArrayInfo{Size: int32(len(data)), Type: GdArrayTypeFloat, Data: data}
	case []float64:
		val := f64Tof32(data)
		return &GdArrayInfo{Size: int32(len(data)), Type: GdArrayTypeFloat, Data: val}
	case []bool:
		return &GdArrayInfo{Size: int32(len(data)), Type: GdArrayTypeBool, Data: data}
	case []string:
		return &GdArrayInfo{Size: int32(len(data)), Type: GdArrayTypeString, Data: data}
	case []byte:
		return &GdArrayInfo{Size: int32(len(data)), Type: GdArrayTypeByte, Data: data}
	case []uint64:
		int64Data := make([]int64, len(data))
		for i, v := range data {
			int64Data[i] = int64(v)
		}
		return &GdArrayInfo{Size: int32(len(data)), Type: GdArrayTypeGdObj, Data: int64Data}
	default:
		return nil
	}
}

func jsValue2Go(value js.Value) any {
	switch value.Type() {
	case js.TypeObject:
		obj := make(map[string]any)
		keys := jsObject.Call("keys", value)
		for i := 0; i < keys.Length(); i++ {
			key := keys.Index(i).String()
			obj[key] = jsValue2Go(value.Get(key)) // Recursively process nested objects
		}
		return obj
	case js.TypeString:
		return value.String()
	case js.TypeNumber:
		return value.Float()
	case js.TypeBoolean:
		return value.Bool()
	default:
		return nil
	}
}

func PrintJs(rect js.Value) {
	rectMap := jsValue2Go(rect)
	jsonData, err := json.Marshal(rectMap)
	if err != nil {
		spxlog.Error("Error converting to JSON: %v", err)
		return
	}
	spxlog.Debug("%s", string(jsonData))
}

func JsFromGdObj(val Object) js.Value {
	return JsFromGdInt(int64(val))
}

func JsSplitGdObj(val Object) (uint32, uint32) {
	return JsSplitGdInt(int64(val))
}

// JsSplitGdInt encodes a web bridge GdInt/GdObj as (low, high) uint32 parts.
// JS-facing wrappers keep this order, while the wasm-side constructors rebuild
// the 64-bit value according to the platform ABI.
func JsSplitGdInt(val int64) (uint32, uint32) {
	low := uint32(val & 0xFFFFFFFF)
	high := uint32((val >> 32) & 0xFFFFFFFF)
	return low, high
}

func JsFromGdInt(val int64) js.Value {
	intJs := jsObject.New()
	low, high := JsSplitGdInt(val)
	intJs.Set("low", low)
	intJs.Set("high", high)
	return intJs
}

func JsToGdObject(val js.Value) Object {
	return Object(JsToGdInt(val))
}

func JsToGdObj(val js.Value) int64 {
	return JsToGdInt(val)
}

func JsToGdInt(val js.Value) int64 {
	low := uint32(val.Get("low").Int())
	high := uint32(val.Get("high").Int())
	return gdIntFromParts(low, high)
}

func JsFromGdString(object string) js.Value {
	return js.ValueOf(object)
}

func JsFromGdVec2(vec Vec2) js.Value {
	vec2Js := jsObject.New()
	vec2Js.Set("x", float32(vec.X))
	vec2Js.Set("y", float32(vec.Y))
	return vec2Js
}

func JsFromGdVec3(vec Vec3) js.Value {
	vec3Js := jsObject.New()
	vec3Js.Set("x", float32(vec.X))
	vec3Js.Set("y", float32(vec.Y))
	vec3Js.Set("z", float32(vec.Z))
	return vec3Js
}

func JsFromGdVec4(vec Vec4) js.Value {
	vec4Js := jsObject.New()
	vec4Js.Set("x", float32(vec.X))
	vec4Js.Set("y", float32(vec.Y))
	vec4Js.Set("z", float32(vec.Z))
	vec4Js.Set("w", float32(vec.W))
	return vec4Js
}

func JsFromGdColor(color Color) js.Value {
	colorJs := jsObject.New()
	colorJs.Set("r", float32(color.R))
	colorJs.Set("g", float32(color.G))
	colorJs.Set("b", float32(color.B))
	colorJs.Set("a", float32(color.A))
	return colorJs
}

func JsFromGdRect2(rect Rect2) js.Value {
	rectJs := jsObject.New()
	rectJs.Set("position", JsFromGdVec2(rect.Position))
	rectJs.Set("size", JsFromGdVec2(rect.Size))
	return rectJs
}

func JsFromGdBool(val bool) js.Value {
	return js.ValueOf(val)
}

func JsFromGdFloat(val float64) js.Value {
	return js.ValueOf(float32(val))
}

func JsToGdString(object js.Value) string {
	s := object.String()
	// Strip null terminators from C/FFI layer to ensure proper string comparison
	return strings.TrimRight(s, "\x00")
}

func JsToGdVec2(vec js.Value) Vec2 {
	return Vec2{
		X: float64(vec.Get("x").Float()),
		Y: float64(vec.Get("y").Float()),
	}
}

func JsToGdVec3(vec js.Value) Vec3 {
	return Vec3{
		X: float64(vec.Get("x").Float()),
		Y: float64(vec.Get("y").Float()),
		Z: float64(vec.Get("z").Float()),
	}
}

func JsToGdVec4(vec js.Value) Vec4 {
	return Vec4{
		X: float64(vec.Get("x").Float()),
		Y: float64(vec.Get("y").Float()),
		Z: float64(vec.Get("z").Float()),
		W: float64(vec.Get("w").Float()),
	}
}

func JsToGdColor(color js.Value) Color {
	return Color{
		R: float64(color.Get("r").Float()),
		G: float64(color.Get("g").Float()),
		B: float64(color.Get("b").Float()),
		A: float64(color.Get("a").Float()),
	}
}

func JsToGdRect2(rect js.Value) Rect2 {
	return Rect2{
		Position: JsToGdVec2(rect.Get("position")),
		Size:     JsToGdVec2(rect.Get("size")),
	}
}

func JsToGdBool(val js.Value) bool {
	switch val.Type() {
	case js.TypeNumber:
		return val.Int() != 0
	case js.TypeBoolean:
		return val.Bool()
	default:
		panic("unknow type")
	}
}

func JsToGdFloat(val js.Value) float64 {
	return float64(val.Float())
}

func JsToGdFloat32(val js.Value) float32 {
	return float32(val.Float())
}

func JsToGdInt64(val js.Value) int64 {
	return int64(val.Int())
}

// gdIntFromParts reconstructs the 64-bit GdInt/GdObj value from the web
// bridge's JS-visible (low, high) representation.
func gdIntFromParts(low, high uint32) int64 {
	return int64(uint64(high)<<32 | uint64(low))
}

func bytesFromInt64Slice(data []int64) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(data))), len(data)*8)
}

func bytesFromUint64Slice(data []uint64) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(data))), len(data)*8)
}

func bytesFromFloat32Slice(data []float32) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(data))), len(data)*4)
}

func getFastGdArrayScratch(arrayType int32) *fastGdArrayScratch {
	if scratch, ok := fastGdArrayScratchByType[arrayType]; ok {
		return scratch
	}
	scratch := &fastGdArrayScratch{
		wrapper: jsObject.New(),
	}
	scratch.wrapper.Set(fastGdArrayFlagKey, true)
	scratch.wrapper.Set(fastGdArrayTypeKey, arrayType)
	fastGdArrayScratchByType[arrayType] = scratch
	return scratch
}

func newFastGdArrayValue(arrayType int32, count int, data []byte) js.Value {
	scratch := getFastGdArrayScratch(arrayType)
	if scratch.byteCap < len(data) || scratch.bytes.Type() == js.TypeUndefined {
		scratch.bytes = jsUint8Array.New(len(data))
		scratch.byteCap = len(data)
	}
	if len(data) > 0 {
		js.CopyBytesToJS(scratch.bytes, data)
	}
	// Expose only the valid prefix so JS consumers that rely on byteLength/length
	// do not read stale capacity bytes from previous larger buffers.
	scratch.wrapper.Set(fastGdArrayDataKey, scratch.bytes.Call("subarray", 0, len(data)))
	scratch.wrapper.Set(fastGdArrayCountKey, count)
	return scratch.wrapper
}

func arrayToFastGdArrayValue(arrayPtr Array) (js.Value, bool) {
	switch data := arrayPtr.(type) {
	case []float32:
		return newFastGdArrayValue(GdArrayTypeFloat, len(data), bytesFromFloat32Slice(data)), true
	case []int64:
		return newFastGdArrayValue(GdArrayTypeInt64, len(data), bytesFromInt64Slice(data)), true
	case []uint64:
		return newFastGdArrayValue(GdArrayTypeGdObj, len(data), bytesFromUint64Slice(data)), true
	default:
		return js.Value{}, false
	}
}

func JsFromGdArray(arrayPtr Array) js.Value {
	if arrayPtr == nil {
		panic("JsFromGdArray doesn't support nil array")
	}

	if fastValue, ok := arrayToFastGdArrayValue(arrayPtr); ok {
		return fastValue
	}

	info := arrayToGdArrayInfo(arrayPtr)
	if info == nil {
		return js.ValueOf(nil)
	}

	serializedBytes, err := serializeGdArray(info)
	if err != nil {
		return js.ValueOf(nil)
	}

	jsBytes := jsUint8Array.New(len(serializedBytes))
	js.CopyBytesToJS(jsBytes, serializedBytes)
	return jsBytes
}

func JsToGdArray(val js.Value) Array {
	if val.IsNull() || val.IsUndefined() {
		return nil
	}

	if val.Type() != js.TypeObject {
		return nil
	}

	length := val.Get("length").Int()
	if length == 0 {
		return nil
	}

	serializedBytes := make([]byte, length)
	js.CopyBytesToGo(serializedBytes, val)

	info, err := deserializeGdArray(serializedBytes)
	if err != nil {
		return nil
	}
	return info.Data
}
