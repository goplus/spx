# 单实例 Runtime

SPX 一个进程只运行一个 Game。实现围绕正常启动、帧调度和退出组织；reload 是低频辅助入口，不提供取消脚本后的透明恢复，也不提供旧版 backend ABI 兼容。

## 状态归属

| 范围 | 状态与职责 |
| --- | --- |
| 进程 | `internal/engine.activeGame`、协程调度器、逻辑锁、输入和帧缓冲、`engine.Managers()` bridge |
| 一次运行 | `gameBinding` 保存 callback、Game owner、phase、启动屏障和 backend link |
| Game | sprite、事件 registry/队列、配置、physics、tilemap、input session、bootstrap generation |
| 脚本线程 | Main 执行计时和取消状态 |
| Backend link | `LinkSession` 拥有本次连接的 Run/Unlink，不能操作后来建立的连接 |

组件直接访问 `engine.Managers()`；无状态 manager wrapper 不需要再包装成每 Game 对象。事件 owner 则必须保留真实身份：Game、Sprite 和 clone 分别拥有自己的 handler，注册、匹配和删除均使用同一个 owner。

`SetGame` 仅供不连接 backend 的内部使用，不能覆盖 `Main` 建立的运行实例。

## 生命周期

`activeGame` 通过 `atomic.Pointer` 发布。涉及占位、替换、释放和 phase 转换的操作持有 `bindingMu`；读路径使用原子值。

| Phase | 含义 |
| --- | --- |
| `gameStarting` | 已占用单实例槽位，正在初始化和准备 backend |
| `gameRunning` | 接受帧、输入和脚本任务；资源启动任务也在此阶段调度 |
| `gameReloading` | 暂停新运行时任务，准备、排空并重建 Game |
| `gameStopped` | 已请求退出，或 reload 取消旧脚本后失败 |
| `gameClosing` | 已有销毁或 reset 流程负责最终清理 |

正常路径：

```text
无实例 → starting → running → closing → 无实例
                         ↘ stopped → closing
```

`stopped` 只关闭业务任务入口，不表示清理已完成；`closing` 表示清理流程已经取得所有权。这样退出请求不会抢走稍后 backend destroy 的清理职责。

### 启动

1. `XGot_Game_Main` 调用 `engine.Main(game, owner, initialize)`。
2. 初始化前占用 `activeGame`；第二个 Game 返回 `ErrGameAlreadyRunning`，不会先修改共享状态。
3. 初始化 Game，准备 callback 和 manager，取得 `LinkSession`。
4. 确认 binding 仍有效后发布 running，调用 `LinkSession.Run`；backend 准备好后关闭 `startDone`。
5. `Game.OnEngineStart` 创建受管理的启动协程，`loadGame` 顺序加载资源、配置、sprite 和事件循环，然后启动脚本。只有成功时才调用 `OnGameStarted` / `OnInited`。

初始化 panic 会关闭启动屏障并释放未成功建立的连接。启动中遇到 reset/destroy 时，清理等待 `startDone`，阻止尚未建立或已经取消的 backend 被激活。

### 首帧与异步加载

启动协程使用普通调度规则：有可运行脚本或主线程任务就继续服务；工作已完成或等待后续帧/异步 worker 时，当前帧返回。

`Coroutines.Update` 不再循环等待 `initialized=true`。加载失败、取消以及无人再发布初始化成功，都不会让首帧永久等待；也不通过 `OnInited` 把失败伪装成成功。Game 的 `IsRunned` 和 bootstrap 状态负责业务初始化门控。

### 帧与延迟任务

`onUpdate` 通过 `updateMu` 串行处理帧，并在执行前和各阶段结束后检查 binding/phase。脚本请求退出后，本帧不再进入后续渲染、capture 或 frame-end。

`engine.Go`、`engine.Execute` 在入队和真正执行时检查同一个 binding，避免旧队列任务越过关闭边界。输入 callback 同样检查 phase。

bootstrap 通过 `queueBootstrap` 入队，由 `startBootstrap` 直接创建受管理的 Go 协程，顺序执行任务后调用 `completeBootstrap`；纯调度不需要切换到引擎主线程。bootstrap generation 用于失效 reset/destroy 前排队的 Main、sprite Main 和 start event；任务入队、取出和发布前均检查 generation。

## 退出与销毁

### Native

`RequestExit` 在同一次主线程调用中将 binding 置为 stopped、发送 backend exit 请求并取消脚本。必须先让引擎线程接到请求，再关闭帧入口，避免退出请求被留在无人服务的队列里。当前帧返回后，Godot 执行 shutdown。

C++ 的 `on_exit` 屏蔽普通运行事件，但保留两个销毁 callback。无论是否 awake、是否先请求 exit，`SpxEngine::shutdown` 的顺序一致：

```text
OnEngineDestroy
    ↓
manager 销毁、singleton 释放
    ↓
OnEngineDestroyed
```

因此 Go 的 Game 收尾不再依赖“仅收到 Destroyed”时的替代路径，也不会在 backend 已释放后补调任意用户 destroy callback。

Go `onDestroy` 取得 closing 所有权，等待启动完成并排空协程，然后调用 `Game.OnEngineDestroy` 清除 bootstrap、pen 缓冲和 input session，最后设置 `destroyReady`。

destroy 和直接 reset 共用 `drainCoroutines`：排空超时后保持 binding 关闭，继续异步等待。销毁路径中的 `OnEngineDestroyed` 只设置 `backendDestroyed`；脚本清理和 backend 销毁都完成后，才 unlink 并释放单实例槽位。

### Web reset

Web 普通 `RequestExit` 使用 reset 通道：先取得 closing 所有权，在后台一次等待协程排空，再在 engine thread 调用 `RequestReset`。无需超时重试；调用方是协程时立即退出当前脚本。

reset callback 完成且 shutdown barrier 退出后才释放 binding。没有 Game 时使用短暂的 closing 占位，避免 reset 尚未结束就连接新 backend。

## Reload 的有限支持

`XGot_Game_Reload` 要求 active Game、engine main thread，且不能由 managed coroutine 调用。内部只有一次 `engine.Reload` 调用，按顺序传入准备、重建和激活步骤；不再暴露 `ReloadGuard` 的多阶段事务方法。

```text
running → reloading（prepare → drain → rebuild → activate）→ running
               │
               ├─ 取消脚本前失败 → running
               └─ 取消脚本后失败 → stopped
```

- 运行中的帧会拒绝 reload，避免同步等待自己的回调；phase gate 与 `updateMu` 屏障防止新帧进入准备过程。
- prepare 只读取和校验配置，失败或 panic 尚未取消脚本，可以恢复 running。
- 开始 drain 前再次确认原 binding。整个操作保持 reloading；一旦开始取消脚本，失败的目标状态改为 stopped。即使 owner 相同，旧请求也不能修改替换后的 binding。
- 默认排空时限为 2 秒。开始取消后超时或重建失败，返回错误并保留停止状态，等待 reset/teardown；不异步恢复为 running。显式退出会覆盖 reloading 状态，后续激活不得重新开放运行。
- 排空成功后执行 Game reset 和资源重建；shutdown barrier 退出、协程 admission 重新打开后，才创建 event/input/logic loops 并发布 running。

取消协程会丢失用户执行栈。仅重新开放调度入口不能恢复原 Game，因此不提供这种恢复承诺。phase 本身串行化 reload，无需再给 Game 增加 reload mutex。

## ABI 与生成

`OnEngineDestroy` 表示开始逻辑销毁；`OnEngineDestroyed` 表示 backend 已释放。两者是不同屏障，不能合并。

`SpxCallbackInfo` 中两个字段紧邻排列在 engine callback 分组；不保留旧字段偏移或旧版本注册协议。Go runtime、native/Web bridge 和 Godot backend 必须由同一接口定义生成并配套构建。

Web 回调的 `GdObj` / `GdInt` 按 i64 值传给 JS，使用锁定 Godot 构建启用的 `WASM_BIGINT`。普通事件统一通过 `gdspx_dispatch` 分发，64 位 ID 拆为 `{low, high}` 供 Go `syscall/js` 无损读取；碰撞批量入口和 runner 生命周期通知保留各自通道。

修改接口时同时检查：

- `pkg/spx/pkg/engine/interface.go` 与 `internal/gdengine/callbacks.go`。
- `internal/cmd/codegen/generate/gdext/gdextension_spx_ext.h.tmpl`。
- native FFI、Web callback、Emscripten library 和 Worker wrapper。
- Godot `SpxEngine::shutdown/on_exit`。

模板和源文件修改后运行 `make generate`；不要手工维护生成头文件、`*.gen.go`、Web glue 或 `export.types`。

## 验证与文件索引

| 文件 | 职责 |
| --- | --- |
| `internal/engine/engine.go` | binding、启动、帧和销毁屏障 |
| `internal/engine/reload.go` | 有限 reload 流程 |
| `internal/engine/exit.go` | native exit 与 Web deferred reset |
| `internal/coroutine/update.go` | 启动和正常脚本共用的帧调度 |
| `internal/gdengine/linker.go`、`binding/facade/session.go` | backend 连接所有权 |
| `game_run.go`、`runtime_engine.go` | Game 入口、重建和 bootstrap |
| `runtime_events.go` | 事件 owner |
| `godot_modules/spx/tests/test_spx_lifecycle.h` | Godot 销毁顺序和幂等性验证 |

生命周期改动至少验证 Go 测试、race、pure_engine 与 WASM 编译；callback/ABI 改动还需生成一致性检查和 Godot 生命周期测试。重点覆盖首次启动失败、主线程任务可继续执行、退出不再渲染、Game 收尾和 backend 释放顺序，而不是只断言内部状态值。
