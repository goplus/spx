### web 平台模式实现注意事项

#### 0. Web 平台注意事项

- 如果使用了 cgo，将无法生成 wasm；Web 相关代码需要通过条件编译绕开 cgo。
- Go 生成的 wasm 默认不能和 C++ 的 wasm 直接拼接，需要使用 JavaScript 胶水代码进行组合。
- 当前模式配置由 `internal/cmd/buildctl/engine/build_shell.go` 统一决定：`normal`、`minigame` 和 `miniprogram` 使用单线程，`worker` 使用 pthread。

#### 1. 普通模式(normal)
1. 单线程（`threads=no`、`proxy_to_pthread=no`）。
2. 需要注意 Web 平台 Go 相关的特别处理。

#### 2. 独立Worker模式(worker)
1. 多线程 + `proxy_to_pthread`。
2. 通信机制借由 `postMessage` 实现，Emscripten 生成的 JavaScript 需要额外兼容代码；升级 Emscripten 时必须重新验证。
3. 不同 Worker 不能直接共享普通 JavaScript 全局变量，也不能直接传递 Go/C++ 指针；应传递可序列化值或明确的 transferable buffer。
4. Go 运行时与引擎需要在同一套 Worker/pthread 桥接中初始化。该模式不是完整的进程级沙箱，资源、内存和错误仍可能通过宿主桥接影响页面。
5. 浏览器通常要求跨源隔离（`Cross-Origin-Opener-Policy` / `Cross-Origin-Embedder-Policy`）才能使用 `SharedArrayBuffer`，部署页面时需要同时满足浏览器和服务器配置要求。

#### 3. 小游戏模式(minigame)
1. 单线程；当前构建配置对小游戏使用 `threads=no`。如果使用旧的 Godot 4.2.2 构建，应先确认小游戏导出兼容性；仓库当前内置版本为 Godot 4.4.1。
2. 微信平台的包体限制可能调整，不能把 30M 当作永久规则。导出流程在压缩模式下会将 wasm 压缩为 Brotli；可用 `-build=fast` 跳过压缩以加快本地调试。
3. 音频需要特殊处理；Godot Web 音频驱动同时包含 AudioWorklet 和 ScriptProcessor 路径，小游戏适配层需要提供兼容的音频上下文，最终使用哪条路径取决于目标运行时能力。
4. Go wasm 启动需要兼容层。导出文件中的 `go.wasm.exec.js` 来自当前 Go 环境，不是仓库内固定的 `js/wasm_exec.js`。
5. 文件系统和 wasm 加载机制需要通过小游戏适配层处理。适配层把 `GameGlobal.WebAssembly` 指向 `WXWebAssembly`；Go 启动代码使用一个仅用于通过当前运行时类型检查的兼容 facade，并非原生 `WebAssembly.Instance`。详见[小游戏 Go WASM 兼容层](./web_minigame_go_wasm_adapter.md)。

#### 4. 小程序模式(miniprogram)
1. 当前构建配置使用单线程；构建规划器也支持显式的 worker 模式，但小程序模板不能据此推断为可用。
2. 借用 web-view 实现（是否可用取决于微信账号和平台限制）。
3. 因为微信小程序的限制，消息传递机制需要特别处理：
 - 小程序 -> web-view 需要通过 url参数 进行传递
 - web-view -> 小程序 需要通过 postMessage 进行传递（但是只能在特定时机生效）

#### 5. Web 截图与固定帧接入

如果需要在外部页面中接入固定帧截图、baseline/runs 保存或截图对比，请参考：

- [Web 端截图与固定帧接入说明](./web_capture.md)

### web平台适配代码规范
`cmd/spx/template/platform/` 目录下是多平台的适配代码
其中 `cmd/spx/template/platform/web` 目录是web平台的公共代码
- `webnormal` 是普通模式
- `webworker` 是独立Worker模式
- `webminigame` 是小游戏模式
- `webminiprogram` 是小程序模式






