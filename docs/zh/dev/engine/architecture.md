## 架构设计

### 0. 整体架构
1. runtime 依赖的是 Godot
2. 偏底层的逻辑在 Godot 用 C++ 进行实现
3. 上层业务逻辑在 Go 进行实现
4. Go 和 C++ 的交互代码，主要通过 `make generate-bindings` 自动生成；完整代码生成流程可使用 `make generate`，详见 [code_generator.md](./code_generator.md)
5. 用户逻辑使用 xgo 进行实现，在运行时会先编译成 Go，再按目标平台编译为动态库或 WebAssembly，或者解释执行

### 1. PC 平台
0. 依赖的是 cgo
1. 通过 `make generate-bindings` 自动生成 Go wrapper 代码，用于在 Go 中调用 C++ 接口
2. 通过 `cmd/ispxnative/build.sh` 将 Go 代码编译为动态链接库
3. Godot runtime 在合适的时机加载并调用 Go 动态链接库，Go 代码将回调钩子注册到 Godot 中，交由 Godot 来管理生命周期
4. 一套新的 Go 协程库，保证同一时间最多只有一个 Go 逻辑协程在运行

### 2. Web 平台
0. 依赖的是 Wasm，并通过 JS 进行缝合拼接
1. `ispx` 的 WebAssembly 产物通过 `make build-wasm` 构建，优化版可使用 `make build-wasm-opt`
2. 小游戏和小程序也是依赖 Web 技术，但是需要进行适配，详情请参考 [web.md](./web.md)

### 3. Android 平台
0. 依赖的是 cgo
1. 和 PC 类似

### 4. iOS 平台
0. 依赖的是 cgo
1. 和 PC 类似，区别是 iOS 平台需要进行适配，部分信号量需要实现屏蔽，否则会导致 crash
