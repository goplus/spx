## 架构设计

### 0. 整体架构
1. runtime 依赖的是godot
2. 偏底层的逻辑在 godot 用c++进行实现
3. 上层业务逻辑在go 进行实现
4. go 和 c++ 的交互代码，通过工具 make gen 自动生成 [code_generator.md](./code_generator.md)
5. 用户逻辑使用xgo进行实现，在运行时候会编译成go,再编译动态库，或解释执行

### 1. PC 平台
0. 依赖的是cgo
1. 通过工具 make gen ,自动将 c++ 代码生成 go wrapper 代码，用于在 go 中调用 c++ 的接口 
2. 通过工具 make build ,自动将 go 代码代码生成动态链接库
3. godot runtime 在合适的时机，加载并调用go动态链接库，go 代码将回调钩子注册到 godot 中，交由godot 来管理生命期
4. 一套新的go 协程库，保证同一时间最多只有一个go逻辑协程在运行

### 2. Web 平台
0. 依赖的是 wasm, 并通过js 进行缝合拼接
2. 小游戏和小程序也是依赖的web技术，但是需要进行适配，详情请参考 [web.md](./web.md)

### 3. android 平台
0. 依赖的是cgo
1. 和pc类似

### 4. ios 平台
0. 依赖的是cgo
1. 和pc类似，区别是，ios 平台需要进行适配，部分信号量需要实现屏蔽，否则会导致crash

