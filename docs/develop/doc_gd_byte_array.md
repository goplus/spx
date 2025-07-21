# GdByteArray 类型支持方案设计

## 1. 背景与目标

### 1.1 现状分析
当前的Godot引擎GDExtension跨语言代码生成系统支持多种基础数据类型（`GdInt`、`GdFloat`、`GdVec2`、`GdString`等），但缺乏对**任意字节数组**的直接支持。

现有的数据传递方式存在以下限制：
- 无法直接传递二进制数据（如图片、音频、序列化对象等）
- 只能通过复杂的字符串编码或多次调用来传递数组数据
- 缺少高效的内存管理机制

### 1.2 目标
设计并实现 `GdByteArray` 类型，使其能够：
- **高效传递任意长度的字节数组**
- **支持Go和JavaScript两个平台**
- **保持与现有架构的一致性**
- **提供简洁易用的API接口**

## 2. 架构设计

### 2.1 整体架构图

```
用户层          Go: []byte              JavaScript: Uint8Array
               ↓ ↑                            ↓ ↑
转换层    ToGdByteArray/ToByteSlice    JsToGdByteArray/JsFromGdByteArray
               ↓ ↑                            ↓ ↑
FFI层         GdByteArray                  js.Value
               ↓ ↑                            ↓ ↑
C++层    {uint8_t* data, size_t length}  WebAssembly Memory
```

### 2.2 类型定义

#### C++层结构体
```cpp
typedef struct {
    uint8_t* data;      // 数据指针
    size_t length;      // 数据长度
} GdByteArray;
```

#### Go层类型映射
```go
type GdByteArray C.GdByteArray  // CGO自动映射
```

## 3. 实现方案

### 3.1 C++层实现

#### 3.1.1 类型定义文件修改
**文件**: `pkg/gdspx/internal/ffi/gdextension_spx_pre_define.h`

```cpp
// 添加字节数组结构体定义
typedef struct {
    uint8_t* data;
    size_t length;
} GdByteArray;
```

#### 3.1.2 测试函数添加
**文件**: `pkg/gdspx/godot/core/extension/spx_sprite_mgr.h`

```cpp
public:
    // 测试函数：设置精灵的自定义数据
    void set_sprite_data(GdObj obj, GdByteArray byte_array);
    
    // 测试函数：获取精灵的自定义数据
    GdByteArray get_sprite_data(GdObj obj);
    
    // 测试函数：验证字节数组传递
    GdInt test_byte_array_verification(GdByteArray input_data);
```

### 3.2 Go层实现（桌面平台）

#### 3.2.1 类型转换函数
**文件**: `pkg/gdspx/internal/ffi/gdextension_interface.go`

```go
// []byte → GdByteArray
func ToGdByteArray(data []byte) GdByteArray {
    if len(data) == 0 {
        return GdByteArray{data: nil, length: 0}
    }
    // 分配C内存并复制数据
    cData := C.malloc(C.size_t(len(data)))
    C.memcpy(cData, unsafe.Pointer(&data[0]), C.size_t(len(data)))
    return GdByteArray{
        data:   (*C.uint8_t)(cData),
        length: C.size_t(len(data)),
    }
}

// GdByteArray → []byte
func ToByteSlice(arr GdByteArray) []byte {
    if arr.data == nil || arr.length == 0 {
        return nil
    }
    // 从C内存复制到Go切片
    return C.GoBytes(unsafe.Pointer(arr.data), C.int(arr.length))
}

// 释放GdByteArray内存
func FreeGdByteArray(arr GdByteArray) {
    if arr.data != nil {
        C.free(unsafe.Pointer(arr.data))
    }
}
```

#### 3.2.2 包装器函数示例
```go
// 自动生成的包装器函数
func (sprite *Sprite) SetSpriteData(data []byte) {
    SpriteMgr.SetSpriteData(sprite.Id, data)
}

func (sprite *Sprite) GetSpriteData() []byte {
    return SpriteMgr.GetSpriteData(sprite.Id)
}

// 管理器层实现
func (manager *spriteMgr) SetSpriteData(obj Object, data []byte) {
    arg0 := ToGdObj(obj)
    arg1 := ToGdByteArray(data)
    defer FreeGdByteArray(arg1)  // 确保内存释放
    CallSpriteSetSpriteData(arg0, arg1)
}

func (manager *spriteMgr) GetSpriteData(obj Object) []byte {
    arg0 := ToGdObj(obj)
    result := CallSpriteGetSpriteData(arg0)
    defer FreeGdByteArray(result)  // 确保内存释放
    return ToByteSlice(result)
}
```

### 3.3 JavaScript层实现（Web平台）

#### 3.3.1 类型转换函数
**文件**: `pkg/gdspx/internal/webffi/util.go`

```go
// Uint8Array → []byte
func JsToGdByteArray(val js.Value) []byte {
    if val.IsNull() || val.IsUndefined() {
        return nil
    }
    
    length := val.Get("length").Int()
    if length == 0 {
        return nil
    }
    
    result := make([]byte, length)
    for i := 0; i < length; i++ {
        result[i] = byte(val.Index(i).Int())
    }
    return result
}

// []byte → Uint8Array
func JsFromGdByteArray(data []byte) js.Value {
    if len(data) == 0 {
        return js.Global().Get("Uint8Array").New(0)
    }
    
    arr := js.Global().Get("Uint8Array").New(len(data))
    for i, b := range data {
        arr.SetIndex(i, int(b))
    }
    return arr
}
```

#### 3.3.2 JavaScript绑定代码生成
**自动生成的JavaScript函数**:

```javascript
function gdspx_sprite_set_sprite_data(obj, byteArray) {
    var _gdFuncPtr = GodotEngine.rtenv['_gdspx_sprite_set_sprite_data'];
    
    var _arg0 = ToGdObj(obj);
    var _arg1 = ToGdByteArray(byteArray);  // Uint8Array → WebAssembly内存
    
    _gdFuncPtr(_arg0, _arg1.data, _arg1.length);
    
    FreeGdObj(_arg0);
    FreeGdByteArray(_arg1);
}

function gdspx_sprite_get_sprite_data(obj) {
    var _gdFuncPtr = GodotEngine.rtenv['_gdspx_sprite_get_sprite_data'];
    
    var _arg0 = ToGdObj(obj);
    var result = _gdFuncPtr(_arg0);  // 返回 {data: pointer, length: number}
    
    var byteArray = FromGdByteArray(result);  // WebAssembly内存 → Uint8Array
    
    FreeGdObj(_arg0);
    FreeGdByteArray(result);
    
    return byteArray;
}
```

## 4. 代码生成模板修改

### 4.1 需要修改的模板文件

1. **函数声明生成**: `pkg/gdspx/cmd/codegen/generate/gdext/header_generator.go`
2. **Go包装器生成**: `pkg/gdspx/cmd/codegen/generate/go/manager_wrapper_generator.go`
3. **JS绑定生成**: `pkg/gdspx/cmd/codegen/generate/js/js_generator.go`
4. **FFI接口生成**: `pkg/gdspx/cmd/codegen/generate/go/ffi_generator.go`

### 4.2 类型映射配置

在代码生成器中添加 `GdByteArray` 的类型映射：

```go
// 在类型映射表中添加
var typeMapping = map[string]TypeInfo{
    "GdByteArray": {
        GoType:       "[]byte",
        JSType:       "Uint8Array", 
        CType:        "GdByteArray",
        ToGoFunc:     "ToByteSlice",
        FromGoFunc:   "ToGdByteArray",
        ToJSFunc:     "JsFromGdByteArray",
        FromJSFunc:   "JsToGdByteArray",
        NeedsMemMgmt: true,  // 需要内存管理
    },
}
```

## 5. 使用示例

### 5.1 Go语言使用示例
```go
// 创建精灵并设置二进制数据
sprite := engine.CreateSprite("player.png")

// 准备一些二进制数据
gameData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A}
sprite.SetSpriteData(gameData)

// 读取数据
retrievedData := sprite.GetSpriteData()
fmt.Printf("Retrieved %d bytes\n", len(retrievedData))
```

### 5.2 JavaScript使用示例
```javascript
// 创建精灵并设置二进制数据
const sprite = engine.createSprite("player.png");

// 准备一些二进制数据
const gameData = new Uint8Array([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A]);
sprite.setSpriteData(gameData);

// 读取数据
const retrievedData = sprite.getSpriteData();
console.log(`Retrieved ${retrievedData.length} bytes`);
```

## 6. 内存管理策略

### 6.1 Go平台内存管理
- **分配**: 使用 `C.malloc` 在C堆上分配内存
- **释放**: 调用 `C.free` 释放内存
- **自动释放**: 在函数结束时使用 `defer` 确保内存释放

### 6.2 Web平台内存管理
- **分配**: 在WebAssembly线性内存中分配
- **释放**: 通过WebAssembly运行时自动垃圾回收
- **数据复制**: JavaScript和WebAssembly之间进行数据复制

## 7. 测试方案

### 7.1 单元测试
```go
func TestGdByteArray(t *testing.T) {
    // 测试空数组
    emptyData := []byte{}
    gdArray := ToGdByteArray(emptyData)
    result := ToByteSlice(gdArray)
    assert.Equal(t, emptyData, result)
    
    // 测试正常数据
    testData := []byte{1, 2, 3, 4, 5}
    gdArray = ToGdByteArray(testData)
    result = ToByteSlice(gdArray)
    assert.Equal(t, testData, result)
    FreeGdByteArray(gdArray)
}
```

### 7.2 集成测试
1. **跨平台一致性测试**: 确保Go和JS平台返回相同结果
2. **内存泄漏测试**: 验证内存正确释放
3. **性能测试**: 测试大数据量传输性能
4. **边界条件测试**: 测试空数组、大数组等边界情况

## 8. 实施计划

### 阶段1: 基础实现
- [ ] C++层类型定义
- [ ] Go层转换函数实现
- [ ] 基础测试函数添加

### 阶段2: 代码生成支持
- [ ] 修改代码生成模板
- [ ] 添加类型映射配置
- [ ] 验证自动生成代码

### 阶段3: JavaScript支持
- [ ] Web平台转换函数实现
- [ ] JavaScript绑定代码生成
- [ ] WebAssembly内存管理

### 阶段4: 测试与优化
- [ ] 单元测试完善
- [ ] 性能测试与优化
- [ ] 文档完善

## 9. 风险评估

### 9.1 技术风险
- **内存管理复杂性**: C/Go之间的内存管理需要谨慎处理
- **WebAssembly性能**: 大数据量在Web平台可能存在性能问题

### 9.2 兼容性风险
- **现有代码影响**: 新类型添加不应影响现有功能
- **平台差异**: 确保Go和JavaScript平台行为一致

### 9.3 缓解措施
- 充分的单元测试和集成测试
- 渐进式实施，每个阶段充分验证
- 完善的内存管理机制和错误处理

## 10. 总结

GdByteArray类型的引入将显著增强系统的数据传输能力，使其能够高效处理二进制数据。通过合理的架构设计和严格的实施计划，可以确保新功能的稳定性和性能。
