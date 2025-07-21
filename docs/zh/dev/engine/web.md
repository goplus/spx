### web 平台模式实现注意事项

#### 0.web 平台注意事项：
- 如果使用了 cgo， 将无法生成 wasm, 所以web相关代码，需要进行条件编译绕开cgo
- go 生成的wasm，默认不能和c++的wasm 直接拼接，需要使用js 进行缝合拼接 

#### 1. 普通模式(normal)
1. 多线程
2. 需要注意web平台go相关的特别处理即可

#### 2. 独立Worker模式(worker)
1. 多线程 + proxy_to_pthread
2. 通信机制需要借由 postMessage 实现，需要在 emcc 生成的 js 代码中插入额外的代码进行处理
  （这属于emcc内部实现，更改emcc版本的时候注意需要进行兼容实现）
3. js的worker 不是其他语言的线程，不同worker之间不能共享全局变量，需要通过 postMessage 进行传递，且postMessage 不能传递指针
4. 因为worker的沙盒属性，所以go 必须和引擎在同一个worker中，不能跨worker，否则无法用js 进行缝合，
   需要在恰当的时机进行go wasm的初始化，以及go wasm, js, c++ wasm 之间的交互问题需要特别注意

#### 3. 小游戏模式(minigame)
1. 必须单线程，因为微信小游戏限制 (godot4.2.2 版本不支持，需要升级到godot4.3或更加后面的版本)
2. 包体大小需要进行限制，最大30M,所以 wasm 需要进行brotli压缩
3. 音频需要特殊处理, godot 当前版本音频解决方案依赖于 AudioWorklet，微信小游戏不支持，需要进行替换
4. go wasm 解析需要进行适配，(需要对go官方wasm_exec.js进行修改适配)
5. 文件系统需要进行适配
6. wasm 加载机制需要进行适配, 注意 WebAssembly 不是原生类型，是wx.WebAssembly类型，是要绕开

#### 4. 小程序模式(miniprogram)
1. 单线程 || 多线程 都可以，目前用的是单线程
2. 借用 web-view 进行实现(个人开发者版本无这个功能)
2. 因为微信小程序的限制， 消息传递机制 需要特别处理
 - 小程序 -> web-view 需要通过 url参数 进行传递
 - web-view -> 小程序 需要通过 postMessage 进行传递（但是只能在特定时机生效）

### web平台适配代码规范
`cmd/gox/template/platform/` 目录下是多平台的适配代码
其中 `cmd/gox/template/platform/web` 目录是web平台的公共代码
- `webnormal` 是普通模式
- `webworker` 是独立Worker模式
- `webminigame` 是小游戏模式
- `webminiprogram` 是小程序模式










