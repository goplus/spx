# SPX 绑定代码生成系统

本文档说明 SPX 当前的绑定代码生成链路，覆盖以下内容：

- Godot `spx` 模块头文件如何进入生成流程
- Go / Web / Native / Godot C++ 侧分别生成哪些文件
- 新增一个 manager 接口时应该改哪里
- 出现生成异常时优先看哪些位置

这里讨论的是 `internal/cmd/codegen` 这一套绑定生成器，不包含 `make generate-runtime` 触发的其他 `go generate` 任务。

## 1. 常用命令

最常用的入口有两个：

```bash
make generate-bindings
make generate
```

它们在 `Makefile` 中的定义如下：

- `make generate-bindings`
  - 执行 `cd ./internal/cmd/codegen && go run .`（`SPX_MODULE_SRC` 由顶层 Make 环境统一导出）
  - 只生成绑定相关代码
- `make generate`
  - 先执行 `generate-bindings`
  - 再执行 `generate-runtime`
  - 最后执行 `format`

如果没有显式传入 `SPX_MODULE_SRC`，生成器默认读写仓库下的 `./godot_modules/spx`。相对覆盖路径从 SPX 仓库根目录解析。

生成器自身是一个 nested Go module，并通过本地 `replace` 指回仓库根目录。因此其中的 `github.com/goplus/spx/v3 v3.0.0` 只是 `go run` 所需的最小合法 v3 module 版本，不是发布版本声明，SPX 发版时不要跟随提升。

推荐用法：

```bash
SPX_MODULE_SRC=/path/to/spx-module make generate-bindings
```

## 2. 整体链路

当前生成流程可以概括为：

```mermaid
graph TD
    A[godot_modules/spx/spx*mgr.h] --> B[合并 public 接口]
    B --> C[生成 gdextension_spx_ext.h 原始头]
    C --> D[预处理 include / 宏]
    D --> E[Clang 解析 AST]
    E --> F[Native Go 绑定]
    E --> G[Web Go 绑定]
    E --> H[Engine 接口层]
    E --> I[Godot C++ 桥接]
    E --> J[Web JS 桥接]
    E --> K[worker.wrap.gen.js]
```

`internal/cmd/codegen/main.go` 中的主流程分成三步：

1. `gdext.GenerateHeader(...)`
   - 从 Godot `spx` 模块头文件生成 `gdextension_spx_ext.h`
2. `gdextensionparser.GenerateGDExtensionInterfaceAST(...)`
   - 展开 include、预处理宏、生成 AST
3. `ffi.Generate(...)`、`webffi.Generate(...)`、`gdext.Generate(...)`
   - 基于 AST 生成 Go / JS / C++ 绑定

## 3. 关键目录与产物

### 3.1 输入源

真正的接口来源是 SPX 仓库自有模块中的 manager 头文件：

- `$(SPX_MODULE_SRC)/spx*mgr.h`

当前生成器只会扫描：

- 文件名匹配 `spx*mgr.h`
- `public:` 区域中的方法
- 带 `SPX_API` 或 `SPX_BIND` 标记的方法

不会进入生成流程的内容包括：

- `spx_base_mgr.h`
- `spx_object_mgr.h`
- 没有 `SPX_API` / `SPX_BIND` 的 helper 方法
- 内联函数定义
- 注释内容

### 3.2 中间文件

生成过程中常见的中间文件有两个：

- `internal/gdengine/binding/native/gdextension_spx_codegen_header.h`
  - AST 解析入口头文件
- `internal/gdengine/binding/native/_temp_output.h`
  - include 展开后的临时头文件

另外，默认还会在 `internal/cmd/codegen` 工作目录下写出：

- `_debug_parsed_ast.json`

这个 JSON 很适合调试“为什么某个方法没被识别”。

### 3.3 主要输出文件

按目标侧分类，当前会生成以下内容：

#### Native 绑定

- `internal/gdengine/binding/native/ffi_wrapper.gen.h`
- `internal/gdengine/binding/native/ffi_wrapper.gen.go`
- `internal/gdengine/binding/native/ffi.gen.go`

#### Web 绑定

- `internal/gdengine/binding/web/callbacks.gen.go`
- `internal/gdengine/binding/web/ffi.gen.go`

#### 引擎实现层

- `internal/gdengine/impl/manager_native.gen.go`
- `internal/gdengine/impl/manager_web.gen.go`

#### 引擎同步包装层

- `internal/enginewrap/sync.gen.go`
- `internal/enginewrap/sync_pure.gen.go`

#### 对外 Go API

- `pkg/spx/pkg/engine/interface.gen.go`
- `pkg/spx/pkg/engine/sprite.gen.go`
- `pkg/spx/pkg/engine/sprite_pure.gen.go`

#### Godot C++ / Web JS 注入文件

- `godot_modules/spx/gdextension_spx_ext.h`
- `godot_modules/spx/gdextension_spx_ext.cpp`
- `godot_modules/spx/web/godot_js_spx.cpp`
- `godot_modules/spx/web/js/engine/gdspx.js`

#### Web Worker 包装文件

- `cmd/spx/template/platform/webworker/worker.wrap.gen.js`

## 4. 代码结构

### 4.1 主入口

`internal/cmd/codegen/main.go`

负责串起整条生成链路：

- 计算默认 `SPX_MODULE_SRC`
- 触发 header 生成
- 触发 AST 解析
- 分别调度 `ffi`、`webffi`、`gdext`

### 4.2 Header 收集与 ABI 入口生成

`internal/cmd/codegen/generate/gdext/header_generator.go`

这是整个系统最关键的入口之一，负责：

- 扫描 `spx*mgr.h`
- 提取 manager 名称
- 收集 `SPX_API` / `SPX_BIND` 方法
- 生成 `GDExtensionSpx...` 形式的 typedef
- 记录 native array / array transform 的桥接元数据

### 4.3 AST 解析

`internal/cmd/codegen/gdextensionparser/parse.go`

流程如下：

1. 找到项目根目录
2. 读取 `gdextension_spx_codegen_header.h`
3. 展开本地 include
4. 交给 `preprocessor` 做预处理
5. 调用 `clang.ParseCString(...)` 得到 AST
6. 按需写出 `_debug_parsed_ast.json`

### 4.4 模板生成

模板主要分布在以下目录：

- `internal/cmd/codegen/generate/ffi`
- `internal/cmd/codegen/generate/webffi`
- `internal/cmd/codegen/generate/gdext`

公共辅助逻辑在：

- `internal/cmd/codegen/generate/common/funcs.go`

`GenerateFile(...)` 会统一做几件事：

- 渲染模板
- 给 `.go` 文件补 license header
- 对生成的 Go 文件执行 `go fmt`
- 对生成的 Go 文件执行 `goimports -w`

所以日常维护时，应该改模板，不要直接改 `.gen.go`。

## 5. 接口导出规则

### 5.1 基本规则

一个方法要进入绑定生成，至少要满足：

1. 位于 `public:` 区域
2. 使用 `SPX_API` 或 `SPX_BIND`
3. 声明形式能被当前正则规则识别

典型例子：

```cpp
class SpxSpriteMgr {
public:
    SPX_API void batch_update_transforms(GdArray buffer);
    SPX_BIND GdBool destroy_sprite(GdObj obj);
};
```

### 5.2 `_raw` 方法的特殊规则

以 `_raw` 结尾的方法不会直接暴露到共享 ABI 中，而是作为 fast path 辅助方法使用。

例如，生成器会把下面这种“高层接口 + `_raw` fast path”配对识别为一组桥接规则：

```cpp
SPX_API void some_batch_method(GdArray buffer);
SPX_API void some_batch_method_raw(const float *buffer_data, int len);
```

这里的 `some_batch_method_raw` 会被识别为高性能桥接实现，但不会直接生成一个对外可见的 `...Raw` 共享接口 typedef。

从 `header_generator_test.go` 可以看出，这套逻辑主要用来支持：

- `GdArray` 到原生指针数组的快速桥接
- Web 侧 WASM fast array 通道
- Native / Web 共用的高层 API 名称

### 5.3 直接原生数组桥接

即使没有高层 `GdArray` 版本，只要签名是这种模式：

```cpp
SPX_API void batch_update_transforms(const float *buffer_data, int len);
```

生成器也会把它识别为 native array bridge，并记录为 `NativeArrayBridgeSpec`。

目前内建支持的原始数组类型主要包括：

- `float *` / `real_t *`
- `int64_t *`
- `uint8_t *`

`GdObj *` 相关 fast path 目前出现在 array-transform bridge 路径里，不属于这里讨论的 direct `NativeArrayBridgeSpec` 检测范围。

### 5.4 数组转换桥接

还有一类特殊情况：底层接口是“输入数组 + 输出数组缓冲区”，高层希望暴露成“输入 `GdArray`，返回 `GdArray`”。

当前这类规则通过 `arrayTransformOverrides` 维护，例如：

- `batch_retrieve_positions`

这表示：

- 原始实现走裸数组 fast path
- 对外 API 仍然可以表现为 `GdArray -> GdArray`

如果以后新增类似接口，只改模板通常不够，还要补这个 override。

## 6. 新增一个接口时怎么改

推荐按下面顺序操作：

1. 在 `$(SPX_MODULE_SRC)/spx*_mgr.h` 中新增方法声明。
2. 确保它位于 `public:` 区域，并带上 `SPX_API` 或 `SPX_BIND`。
3. 如果需要高性能数组桥接：
   - 加一组 `_raw` 方法
   - 或直接使用可识别的指针 + 长度签名
4. 如果是“输入数组，返回数组”的特殊 fast path，检查是否需要更新 `arrayTransformOverrides`。
5. 运行 `make generate-bindings`。
6. 检查生成结果是否覆盖到了预期文件。
7. 补相应测试，尤其是：
   - `internal/cmd/codegen/generate/gdext/header_generator_test.go`
   - `internal/cmd/codegen/generate/webffi/generate_test.go`

## 7. 调试建议

### 7.1 方法完全没生成

优先检查：

- 方法是否在 `public:` 下
- 是否加了 `SPX_API` / `SPX_BIND`
- 是否写成了生成器当前支持的声明形式
- 是否被写成了 inline 定义

### 7.2 AST 里没有这个方法

优先看：

- `internal/gdengine/binding/native/_temp_output.h`
- `internal/cmd/codegen/_debug_parsed_ast.json`

如果这两个文件里都没有，问题通常出在 header 合并或预处理阶段。

### 7.3 SPX 模块侧文件没有更新

优先检查：

- `SPX_MODULE_SRC` 是否指向正确的 SPX 模块目录
- `$(SPX_MODULE_SRC)` 是否存在
- `$(SPX_MODULE_SRC)/web/js/engine` 是否可写

从代码逻辑看，如果 `SPX_MODULE_SRC` 目录不存在，`gdext.GenerateHeader(...)` 和 `gdext.Generate(...)` 会给出 warning 并跳过 Godot C++ 侧更新。

### 7.4 Web fast path 异常

优先看：

- `internal/cmd/codegen/generate/webffi/generate_test.go`
- `internal/cmd/codegen/generate/gdext/header_generator_test.go`

这两组测试已经覆盖了：

- GdObj 扁平参数 ABI
- 固定 scratch 缓冲区读取
- native array bridge
- array transform bridge
- 输入缓存 override

## 8. 不要直接修改哪些文件

以下文件默认都应视为生成产物：

- `*.gen.go`
- `*.gen.h`
- `cmd/spx/template/platform/webworker/worker.wrap.gen.js`
- `godot_modules/spx/web/js/engine/gdspx.js`
- `godot_modules/spx/web/godot_js_spx.cpp`
- `godot_modules/spx/gdextension_spx_ext.h`
- `godot_modules/spx/gdextension_spx_ext.cpp`

需要改行为时，优先改这些位置：

- Godot manager 头文件
- `internal/cmd/codegen/generate/**` 下的模板
- `internal/cmd/codegen/generate/common/funcs.go`
- `internal/cmd/codegen/generate/gdext/header_generator.go`

## 9. 小结

SPX 当前的绑定生成系统，本质上是：

- 先从 Godot `spx` 模块头文件提取稳定 ABI
- 再把 ABI 解析成 AST
- 最后从同一份 AST 同步生成 Native / Web / Engine / Godot C++ 多端桥接代码

维护这套系统时，最重要的经验有三条：

- 先改源头 header 或模板，不要手改生成产物
- 新增数组 fast path 时，别忘了 `_raw` / override 规则
- 出问题先看 `_temp_output.h` 和 `_debug_parsed_ast.json`
