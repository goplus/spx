package webffi

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"syscall/js"
	"unsafe"

	. "github.com/goplus/spbase/mathf"
	. "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

var isLittleEndian bool
var byteOrder binary.ByteOrder

func init() {
	buf := [2]byte{}
	*(*uint16)(unsafe.Pointer(&buf[0])) = uint16(0xABCD)

	switch buf {
	case [2]byte{0xCD, 0xAB}:
		isLittleEndian = true
		byteOrder = binary.LittleEndian
	case [2]byte{0xAB, 0xCD}:
		isLittleEndian = false
		byteOrder = binary.BigEndian
	default:
		panic("Could not determine native endianness.")
	}
}

const (
	GD_ARRAY_TYPE_UNKNOWN = 0
	GD_ARRAY_TYPE_INT64   = 1
	GD_ARRAY_TYPE_FLOAT   = 2
	GD_ARRAY_TYPE_BOOL    = 3
	GD_ARRAY_TYPE_STRING  = 4
	GD_ARRAY_TYPE_BYTE    = 5
	GD_ARRAY_TYPE_GDOBJ   = 6
)

type GdArrayInfo struct {
	Size int32
	Type int32
	Data interface{}
}

func serializeGdArray(info *GdArrayInfo) ([]byte, error) {
	if info == nil {
		return nil, fmt.Errorf("GdArrayInfo is null")
	}

	dataBytes, err := serializeDataByType(info.Type, info.Data)
	if err != nil {
		return nil, err
	}

	totalSize := 8 + len(dataBytes)
	result := make([]byte, totalSize)

	if isLittleEndian {
		*(*uint32)(unsafe.Pointer(&result[0])) = uint32(info.Size)
		*(*uint32)(unsafe.Pointer(&result[4])) = uint32(info.Type)
	} else {
		binary.LittleEndian.PutUint32(result[0:4], uint32(info.Size))
		binary.LittleEndian.PutUint32(result[4:8], uint32(info.Type))
	}

	copy(result[8:], dataBytes)

	return result, nil
}

func deserializeGdArray(data []byte) (*GdArrayInfo, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("data length is not enough")
	}

	var size, arrayType int32
	if isLittleEndian {
		size = int32(*(*uint32)(unsafe.Pointer(&data[0])))
		arrayType = int32(*(*uint32)(unsafe.Pointer(&data[4])))
	} else {
		size = int32(binary.LittleEndian.Uint32(data[0:4]))
		arrayType = int32(binary.LittleEndian.Uint32(data[4:8]))
	}

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
func serializeDataByType(arrayType int32, data interface{}) ([]byte, error) {
	if data == nil {
		return []byte{}, nil
	}

	// Check for empty arrays
	switch arrayType {
	case GD_ARRAY_TYPE_INT64, GD_ARRAY_TYPE_GDOBJ:
		if arr := data.([]int64); len(arr) == 0 {
			return []byte{}, nil
		}
		return serializeInt64Array(data.([]int64))
	case GD_ARRAY_TYPE_FLOAT:
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
	case GD_ARRAY_TYPE_BOOL:
		if arr := data.([]bool); len(arr) == 0 {
			return []byte{}, nil
		}
		return serializeBoolArray(data.([]bool))
	case GD_ARRAY_TYPE_BYTE:
		if arr := data.([]byte); len(arr) == 0 {
			return []byte{}, nil
		}
		return data.([]byte), nil
	case GD_ARRAY_TYPE_STRING:
		if arr := data.([]string); len(arr) == 0 {
			return []byte{}, nil
		}
		return serializeStringArray(data.([]string))
	default:
		return nil, fmt.Errorf("array type is not supported: %d", arrayType)
	}
}

func deserializeDataByType(arrayType int32, data []byte, size int32) (interface{}, error) {
	if len(data) == 0 || size == 0 {
		switch arrayType {
		case GD_ARRAY_TYPE_INT64, GD_ARRAY_TYPE_GDOBJ:
			return []int64{}, nil
		case GD_ARRAY_TYPE_FLOAT:
			return []float32{}, nil
		case GD_ARRAY_TYPE_BOOL:
			return []bool{}, nil
		case GD_ARRAY_TYPE_BYTE:
			return []byte{}, nil
		case GD_ARRAY_TYPE_STRING:
			return []string{}, nil
		default:
			return nil, fmt.Errorf("array type is not supported: %d", arrayType)
		}
	}

	switch arrayType {
	case GD_ARRAY_TYPE_INT64, GD_ARRAY_TYPE_GDOBJ:
		return deserializeInt64Array(data, size)
	case GD_ARRAY_TYPE_FLOAT:
		return deserializeFloatArray(data, size)
	case GD_ARRAY_TYPE_BOOL:
		return deserializeBoolArray(data, size)
	case GD_ARRAY_TYPE_BYTE:
		return data, nil
	case GD_ARRAY_TYPE_STRING:
		return deserializeStringArray(data)
	default:
		return nil, fmt.Errorf("array type is not supported: %d", arrayType)
	}
}

func serializeInt64Array(data []int64) ([]byte, error) {
	if isLittleEndian {
		// Zero-copy conversion: directly convert []int64 to []byte
		return (*[1 << 30]byte)(unsafe.Pointer(&data[0]))[: len(data)*8 : len(data)*8], nil
	} else {
		// Big endian machines need byte order conversion
		result := make([]byte, len(data)*8)
		for i, val := range data {
			binary.LittleEndian.PutUint64(result[i*8:(i+1)*8], uint64(val))
		}
		return result, nil
	}
}

func deserializeInt64Array(data []byte, size int32) ([]int64, error) {
	if len(data) < int(size*8) {
		return nil, fmt.Errorf("array data length is not enough")
	}

	if isLittleEndian {
		// Zero-copy conversion: directly convert []byte to []int64
		return (*[1 << 27]int64)(unsafe.Pointer(&data[0]))[:size:size], nil
	} else {
		// Big endian machines need byte order conversion
		result := make([]int64, size)
		for i := int32(0); i < size; i++ {
			result[i] = int64(binary.LittleEndian.Uint64(data[i*8 : (i+1)*8]))
		}
		return result, nil
	}
}

func serializeFloatArray(data []float32) ([]byte, error) {
	if isLittleEndian {
		// Zero-copy conversion: directly convert []float32 to []byte
		return (*[1 << 30]byte)(unsafe.Pointer(&data[0]))[: len(data)*4 : len(data)*4], nil
	} else {
		// Big endian machines need byte order conversion
		result := make([]byte, len(data)*4)
		for i, val := range data {
			bits := *(*uint32)(unsafe.Pointer(&val))
			binary.LittleEndian.PutUint32(result[i*4:(i+1)*4], bits)
		}
		return result, nil
	}
}

func deserializeFloatArray(data []byte, size int32) ([]float32, error) {
	if len(data) < int(size*4) {
		return nil, fmt.Errorf("array data length is not enough")
	}

	if isLittleEndian {
		// Zero-copy conversion: directly convert []byte to []float32
		return (*[1 << 28]float32)(unsafe.Pointer(&data[0]))[:size:size], nil
	} else {
		// Big endian machines need byte order conversion
		result := make([]float32, size)
		for i := int32(0); i < size; i++ {
			bits := binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
			result[i] = *(*float32)(unsafe.Pointer(&bits))
		}
		return result, nil
	}
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
	for i := int32(0); i < size; i++ {
		result[i] = data[i] != 0
	}
	return result, nil
}

func serializeStringArray(data []string) ([]byte, error) {
	var result []byte
	for _, str := range data {
		strBytes := []byte(str)
		lengthBytes := make([]byte, 4)

		if isLittleEndian {
			*(*uint32)(unsafe.Pointer(&lengthBytes[0])) = uint32(len(strBytes))
		} else {
			binary.LittleEndian.PutUint32(lengthBytes, uint32(len(strBytes)))
		}

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

		var strLen int
		if isLittleEndian {
			strLen = int(*(*uint32)(unsafe.Pointer(&data[offset])))
		} else {
			strLen = int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		}
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
		return &GdArrayInfo{Size: int32(len(data)), Type: GD_ARRAY_TYPE_INT64, Data: data}
	case []float32:
		return &GdArrayInfo{Size: int32(len(data)), Type: GD_ARRAY_TYPE_FLOAT, Data: data}
	case []float64:
		val := f64Tof32(data)
		return &GdArrayInfo{Size: int32(len(data)), Type: GD_ARRAY_TYPE_FLOAT, Data: val}
	case []bool:
		return &GdArrayInfo{Size: int32(len(data)), Type: GD_ARRAY_TYPE_BOOL, Data: data}
	case []string:
		return &GdArrayInfo{Size: int32(len(data)), Type: GD_ARRAY_TYPE_STRING, Data: data}
	case []byte:
		return &GdArrayInfo{Size: int32(len(data)), Type: GD_ARRAY_TYPE_BYTE, Data: data}
	case []uint64:
		int64Data := make([]int64, len(data))
		for i, v := range data {
			int64Data[i] = int64(v)
		}
		return &GdArrayInfo{Size: int32(len(data)), Type: GD_ARRAY_TYPE_GDOBJ, Data: int64Data}
	default:
		return nil
	}
}

func jsValue2Go(value js.Value) any {
	switch value.Type() {
	case js.TypeObject:
		obj := make(map[string]any)
		keys := js.Global().Get("Object").Call("keys", value)
		for i := 0; i < keys.Length(); i++ {
			key := keys.Index(i).String()
			obj[key] = jsValue2Go(value.Get(key)) // 递归处理嵌套对象
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
		fmt.Println("Error converting to JSON:", err)
		return
	}
	fmt.Println(string(jsonData))
}

func JsFromGdObj(val Object) js.Value {
	return JsFromGdInt(int64(val))
}

func JsFromGdInt(val int64) js.Value {
	vec2Js := js.Global().Get("Object").New()

	low := uint32(val & 0xFFFFFFFF)
	high := uint32((val >> 32) & 0xFFFFFFFF)
	vec2Js.Set("low", low)
	vec2Js.Set("high", high)
	return vec2Js
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

	int64Value := int64(high)<<32 | int64(low)
	return int64Value
}

func JsFromGdString(object string) js.Value {
	return js.ValueOf(object)
}

func JsFromGdVec2(vec Vec2) js.Value {
	vec2Js := js.Global().Get("Object").New()
	vec2Js.Set("x", float32(vec.X))
	vec2Js.Set("y", float32(vec.Y))
	return vec2Js
}

func JsFromGdVec3(vec Vec3) js.Value {
	vec3Js := js.Global().Get("Object").New()
	vec3Js.Set("x", float32(vec.X))
	vec3Js.Set("y", float32(vec.Y))
	vec3Js.Set("z", float32(vec.Z))
	return vec3Js
}

func JsFromGdVec4(vec Vec4) js.Value {
	vec4Js := js.Global().Get("Object").New()
	vec4Js.Set("x", float32(vec.X))
	vec4Js.Set("y", float32(vec.Y))
	vec4Js.Set("z", float32(vec.Z))
	vec4Js.Set("w", float32(vec.W))
	return vec4Js
}

func JsFromGdColor(color Color) js.Value {
	colorJs := js.Global().Get("Object").New()
	colorJs.Set("r", float32(color.R))
	colorJs.Set("g", float32(color.G))
	colorJs.Set("b", float32(color.B))
	colorJs.Set("a", float32(color.A))
	return colorJs
}

func JsFromGdRect2(rect Rect2) js.Value {
	rectJs := js.Global().Get("Object").New()
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

func JsFromGdArray(arrayPtr Array) js.Value {
	if arrayPtr == nil {
		panic("JsFromGdArray doesn't support nil array")
		return js.ValueOf(nil)
	}

	info := arrayToGdArrayInfo(arrayPtr)
	if info == nil {
		return js.ValueOf(nil)
	}

	serializedBytes, err := serializeGdArray(info)
	if err != nil {
		return js.ValueOf(nil)
	}

	jsBytes := js.Global().Get("Uint8Array").New(len(serializedBytes))
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
