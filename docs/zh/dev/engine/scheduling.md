# SPX 脚本调度

[English](../../../en/dev/engine/scheduling.md)

## 概述

SPX 使用协作式脚本调度。脚本执行到让出执行权或结束后，调度器再让其他
可运行的脚本继续。循环边界、显式等待、重绘请求和条件事件共同决定脚本何时恢复。

本文描述 SPX 当前运行时的帧阶段、循环轮次，以及截图和输入回放的保证范围。
这些规则适用于所有使用相应 API 的项目。

## 执行权与结束

每个获准执行的脚本使用一个 Go goroutine 保存调用栈，通过 `runMu` 串行执行脚本片段。
`yield` 释放执行权；被唤醒的脚本重新取得执行权后才能继续。`Current()` 表示当前
执行权的拥有者，不代表调用 API 的 goroutine 身份。
`Task.Owner` 和 `Thread.Owner` 表示任务的逻辑归属对象。

普通时间、帧、Join 和外部任务等待会让出执行权。`WaitMainThread` 是同步引擎调用，
等待期间保留当前脚本片段：尚未开始的请求可以取消，已经开始的请求必须完成后，
调用脚本才能退出。主线程等待脚本时继续处理引擎请求，不推进未来帧。

channel 接收复用受管理的外部任务，只在脚本正常恢复后提交接收结果；等待因取消
退出时不会写入调用方的数据。`JoinFirstSlice(thread)` 等待目标首次让出或结束。
Join 和 Latch 的等待登记在正常返回或取消时都会解除。
`Execute` 等待任务生命周期结束，因此任务启动前被取消或拒绝也能返回。

请求取消、脚本结束和运行时清理完成是三个阶段。`Cancel` 与 `Stop` 都请求停止并
唤醒挂起脚本，但不能强制终止任意 Go 阻塞调用。重置屏障等待脚本、外部工作和最终
异常处理收尾。宿主通过 `RunAfterStop(nil)` 等待全部已登记工作退出；脚本使用不等待收尾的停止接口。
`WaitToDo` 的 worker 若通过 `Goexit` 退出，会取消并唤醒所属脚本，使其执行收尾。

### 停止接口

| 接口 | 语义 |
| --- | --- |
| `Stop(thread)` / `StopIf` | 请求目标协程停止，不等待收尾。 |
| `StopCurrent` | 退出当前协程，穿透自定义过程边界。 |
| `StopThisScript` | 退出最近的自定义过程并让外层继续；没有过程边界时结束协程。 |
| `StopAll` | 请求全部已登记协程停止，不等待收尾。 |
| `RunAfterStop(call)` | 宿主关闭准入、请求停止、等待脚本和 worker 收尾，再执行非 nil 回调；受管理脚本调用会在改变状态前被拒绝。 |

`Stop` 表达脚本操作，`Cancel` 传递取消，内部 `drain` 等待收尾。
`ErrAbortThread` 保留为退出当前协程的控制流信号。

### 停止屏障

`RunAfterStop` 是仅供宿主调用的同步屏障，没有超时和返回值。它关闭准入、取消已登记
脚本，等待脚本、worker 和最终异常处理退出，再执行清理回调。脚本使用不等待收尾的
停止接口；脚本自身的取消 context 仍然独立于宿主屏障。

若主线程清理调用在 drain 完成前退出等待的 goroutine，准入保持关闭；后续成功的
屏障可以完成清理并恢复准入。只有全部已登记工作退出后，才会执行清理回调。

### 回调与同步等待

`RegisterInvocation` 在创建方同步执行，此时线程已登记到调度器，但尚未执行任务；
它返回的 cleanup 在任务退出时执行，执行前被取消的任务也会执行 cleanup。
`RegisterInvocation` 不得等待本批任务，因为本批执行需要全部登记回调返回。
准入拒绝直接返回已取消、已完成的线程，不启动 goroutine，也不调用登记和执行回调。
两种 Join 等待都立即完成，包括来自受管脚本的等待。

非空批次的 `BatchWaitFirstSlice` 仅允许受管理脚本调用：返回需要在批次的首个执行片段后
重新取得脚本执行权。宿主调用会在登记任何任务前被拒绝。`BatchAsync` 和 `BatchWaitDone`
也接受宿主调用；空批次在所有模式下都立即返回。

脚本、worker、`RegisterInvocation`、cleanup、最终异常处理、停机回调和独占脚本执行权的
回调不能同步 drain；调用会在取消任务或改变准入前抛出 `ErrReentrantWait`。独占回调也不能再次
同步执行脚本或等待未完成的 Join / Latch，否则脚本无法取得执行权；已经完成的等待可直接返回。

停机回调期间，准入保持关闭，但不持有准入锁。回调可以调用 `StopAll`，新任务会被拒绝；
正常返回、panic 或 `Goexit` 都会恢复准入。

取消直接唤醒挂起脚本，无需未来的 `Update`。原生引擎主线程等待屏障和清理时继续
处理主线程任务，不推进帧或脚本轮次；这允许在帧号不变时完成，不保证在一帧的时间预算内完成。
Web 后台等待让出 Go goroutine；同步 JS 回调若依赖后续事件，须把等待转到独立 goroutine 后返回。

停止屏障等待已经开始的引擎调用和回调完成，脚本重新取得执行权也依赖协作。
外部 worker 必须响应工作 context 或操作自身的取消机制；任意阻塞和循环等待仍可能阻止清理。

### 并发约束

- `schedulerMu` 保护 runnable 状态和共享的当前任务队列；Update 与引擎等待者在
  锁内选择并取出任务，解锁后执行。引擎串行调用 Update，由它独占延后和下一轮队列；
  队列不再单独加锁。
- 准入读锁允许并行登记，原子计数 `pendingSetups` 将重新开放准入推迟到所有登记回调结束。
  登记回调在锁外执行；原子 epoch 使准入重新开放后仍能拒绝停止前的过期请求。
- 等待登记与 blocked 状态一起发布，锁序为 `schedulerMu` → `waiterSet.mu`。
  关闭等待集合时移交等待者，解锁后再唤醒，避免与登记形成锁序环。
- 任务结束时先把 Join 等待者标为 runnable，再取消 context、关闭完成通知并移除
  调度状态，避免 Update 在两者之间误判没有可运行脚本。
- 外部工作启动前登记，worker 退出时统一注销。取消调用脚本不能提前减少
  外部工作计数；重置必须等实际工作结束。
- 清理等待在生命周期登记锁下同时检查条件并订阅完成通知，避免遗漏唤醒。
  等待期间准入保持关闭，最后一个线程或 worker 注销时通知所有等待者，
  包括最终异常处理结束；不以脚本的完成 channel 替代。
- 广播执行上下文和每棵广播树的接收器集合仅由持有 `runMu` 的脚本访问，使用普通
  map。消息处理函数退出时删除执行上下文；等待因取消退出时，也先重新取得执行权再
  执行删除。外部广播创建独立上下文，无调度器的直接派发不登记执行上下文。重置事件
  登记表不清空正在执行的上下文，其清理由协程退出负责。

## 帧阶段

一次 engine update 推进一次帧时钟，其中可以包含多轮脚本执行。
开始下一轮不会推进帧号或计时器。

1. 缓存引擎已经交付的输入和碰撞事件。在同一次 `RunBetweenScripts` 阶段，
   从 Godot 回读物理精灵位置，为普通输入、录制或回放冻结帧输入批次，
   并在帧时钟推进前采样条件事件。条件采样在启动脚本首次让出或结束后启用。
2. 推进时钟，再通过受管理的派发协程推进 runtime 延时计时和 tween 状态。
   精灵 `OnUpdate`、到期延时和已完成 tween 的回调按登记顺序组成一个独立任务批次。
   某个回调等待时，后续任务仍可执行，不会挂起帧推进。
   回调新建的 timer 和 tween 从下一帧开始推进。
3. 准备游戏更新并处理截图快捷键请求；冻结已到期的脚本 Timer，登记已匹配的
   条件处理函数，路由输入和 Timer，再登记缓存的碰撞处理函数，最后派发启动事件。
   登记顺序不保证不同批次处理函数的执行顺序。
4. 仅调用一次 `gco.Update`，处理可运行的协程和满足条件的后续循环轮次，
   包括已登记的碰撞处理函数。让出执行权后，后续工作可以留到未来帧；
   帧收尾不要求每个协程都退出。
5. 与脚本执行互斥，将待提交的 Go 精灵变化、气泡和画笔命令写入 Godot。
   随后派发截图队列，完成输入 session 的帧收尾。Web host 等待渲染屏障后读取画布。

当前 Godot fork 在进入 SPX 更新回调前，已执行零到多次固定物理更新、场景处理和
本轮渲染提交。`OnEngineRender` 是 Go 状态提交阶段，不会再调用一次 Godot 绘制；
提交的变化参与后续物理和渲染。提交也不会立即重新计算碰撞，只有帧边界前已经交付的
碰撞事件进入本帧事件批次。

物理位置回读会按轴保留尚未提交的显式 Go 坐标修改。仅修改可见性不占用坐标，
因此不会用旧 Go 坐标覆盖刚回读的物理位置。底层 proxy 单轴移动只确认对应轴的写入，
完整 `SetPosition` 才覆盖双轴。需要最新几何状态的查询仍可在脚本执行期间
同步精灵，帧末批量提交并不是唯一可能的 Go→Godot 调用。提交后不会额外运行一次
调度器来强行推进等待未来帧的碰撞处理函数。

### 输入与 Timer 的帧归属

输入快照截止前采集的边沿属于本帧，截止后到达的输入留到下一帧。
每个已准备的输入帧都会在帧时钟推进后、事件派发前清理过期的滑动手势，
包括鼠标没有移动的帧；没有已准备的帧时，派发不会清理手势。鼠标移动本身不产生路由事件。
鼠标按钮边沿、按键和到期 Timer 按此顺序路由，不保证用户处理函数的全局执行顺序。
派发期间新登记或由 `ResetTimer` 重新激活的 Timer 在后续帧批次检查。
Timer 在 `OnEngineUpdate` 的前置事件阶段路由；待播放音频和已完成动画仍由
`logicLoop` 收尾，不保证该收尾先于 Timer 登记或回调。

整批 `handleEvent` 路由和处理函数登记持有脚本执行权，在本帧脚本调度前完成。
输入不再经过容量为 16 的 `Game.events` 通道及其异步接收 worker。
处理函数仍使用 `BatchAsync`；正常的 `Update` 会等待可运行的处理函数首次让出或结束。
`Wait`、`WaitNextFrame` 和外部任务等待可以把后续工作留到未来帧。
取消、停止、重入策略及 watchdog 提前收尾仍然生效，登记完成不保证每个处理函数已运行或执行完毕。
点击命中和 50 ms 点击节流规则保持生效。

整批登记不等于每个输入边沿都执行回调。按键注册采用 `IgnoreWhileRunning`，
已有调用未结束时忽略新调用；空闲的同一份 `OnAnyKey` 或多键 `OnKey` 注册，
在同批中只接纳第一个匹配事件。

升级时移除 Go 配置中的 `EventQueuePolicy` 引用和 JSON 中的 `eventQueuePolicy`。
该 Go 字段已删除；当前项目 JSON 加载器会忽略旧字段。帧批次不再提供
`DropNewest`、`DropOldest` 或 `Block` 队列策略，处理函数仍独立应用上述接纳规则。

## 循环、重绘与等待

1. **区分帧与执行轮次。** `Forever`、`Repeat`、`RepeatUntil`、`WaitUntil`
   在普通模式下让其他脚本执行；没有重绘请求且预算允许时，可以在同帧继续下一轮。
   每帧迭代次数由实际工作量决定。
2. **在轮次边界处理重绘。** 重绘请求允许当前轮的其他脚本继续执行，
   下一轮留到下一帧。可见精灵的变换、造型、特效，显示/隐藏、背景、
   可见气泡和画笔绘制通过统一入口通知调度器。隐藏精灵的变换不请求重绘。
3. **保留独立的等待语义。** 显式时间等待和 `WaitNextFrame` 各有自己的恢复条件。
   普通循环按真实耗时计算帧内预算，为默认 30 Hz 间隔的 75%，即 25 ms。
   预算只决定是否开始下一轮，不会中断当前轮。`Warp` 使用独立的 500 ms 协作让出预算。
4. **跨帧保留脚本顺序。** 延后到下一帧的等待和循环任务，按脚本注册顺序
   检查各自的恢复条件。可运行脚本（包括新启动的处理函数）让出或结束后，
   再恢复下一项已排队的脚本；期间仍处理引擎主线程调用。
   重启仍在运行的事件处理函数时，新实例继承原实例的跨帧排序位置，Thread ID
   仍独立分配。旧实例已经停止或完成后再次触发，使用新的注册位置。

## 事件登记快照

事件 Manager 在锁内追加登记，快照可以与后续追加共享底层数组。`Snapshot` 和
`SnapshotStartOnce` 返回浅只读视图：调用者不得修改已有元素，slice 的容量限制为
当前长度，因此在快照上追加会使用独立存储。Sink 引用的 Handler 等对象仍保持原有
可变性；快照不提供这些对象的同步或 owner 生命周期保证。

删除 owner 时复制保留项，Reset 只释放 Manager 持有的 slice 引用，均不修改已经发布
的元素。历史快照跨追加、删除和 Reset 保持原有登记顺序与内容。启动登记仍在首次
`SnapshotStartOnce` 后关闭，Reset 后重新开放。

## 条件事件

1. **先检查条件，再刷新时钟。** 启动脚本到达首次让出或结束后，
   在每帧时钟推进前统一求值条件；时钟推进后再派发已匹配的处理函数，
   派发时不重复求值。条件按现有目标顺序检查，全部检查完成后才启动处理函数。
   输入条件读取本帧 session tick，包括回放暂停前的最后一个 tick。
2. **条件事件按上升沿触发。** 持续为真不会重复启动处理函数。
   处理函数运行期间暂停该事件的条件检查，结束后继续检测上升沿。
   条件函数必须快速返回，不调用等待类 API。

## 实现结构

| 职责 | 位置 |
| --- | --- |
| 协程准入与生命周期、停止与清理屏障 | `internal/coroutine/lifecycle.go`、`shutdown.go` |
| 回调重入限制、主线程调用、帧边界回调 | `internal/coroutine/callback.go`、`mainthread.go` |
| 定时与通道等待、worker 生命周期 | `internal/coroutine/wait.go` |
| 帧等待、循环轮次、重绘预算 | `internal/coroutine/frame.go` |
| 等待任务处理与调度统计 | `internal/coroutine/update.go`、`stats.go` |
| 控制流与调度器衔接 | `internal/engine/coro.go` |
| 帧阶段顺序 | `internal/engine/engine.go`、`runtime_engine.go` |
| 引擎层更新回调与 runtime 延时、tween | `internal/gdengine/callbacks.go`、`internal/engine/runtime_bridge.go` |
| 物理位置回读与帧末精灵提交 | `runtime_sync.go`、`component_transform.go`、`shape_manager.go` |
| 帧输入采样、统一输入与 Timer 派发 | `runtime_input.go` |
| 条件注册、状态采样与已匹配事件派发 | `runtime_events.go` |
| 可见性判断与重绘通知 | `sprite_render.go` 及对应视觉操作入口 |

`UpdateStats()` 返回最近一次更新的统计，耗时单位为毫秒。
`queue`、`waitJob` 和 `waitKind` 是调度器的私有实现。

帧边界回调与脚本执行互斥，同时处理已排队的引擎主线程调用，
避免正在等待引擎调用的脚本阻塞条件采样。该阶段不推进循环或帧等待任务。
帧边界回调仍在调用线程执行；runtime 状态推进和外部原生事件派发使用受管理的派发协程，
等待期间只处理引擎主线程任务。同步派发过程登记异步任务，不能等待未来帧。
runtime 用户回调由独立任务执行，可以直接等待未来帧，也可以等待含跨帧操作的事件处理函数。

## 截图与输入回放

游戏运行期间，`Snapshot` 将截图请求入队，在帧末视觉状态同步后派发。
配置 `captureKey` 后，录制和回放中的按键 press 边沿会在相同 input tick 请求截图。
这些请求不固定截图前完成的脚本工作量。
host 接入方式见[《Web 端截图接入说明》](web_capture.md)。

[输入录制与回放](input_replay.md)固定输入序列、逻辑时间步长和脚本随机种子，
但不固定或记录每帧的脚本轮数。循环调度仍然检查真实耗时。
如果画面状态依赖预算内完成的循环工作量，即使使用相同构建和输入记录，
机器负载差异也可能让相同 input tick 的截图不同。

回归场景应使用显式帧边界或逻辑时间边界推进可观察状态，避免依赖预算内的迭代次数。
调度规则变化后重新生成 baseline 可以反映新的语义，但不能消除真实耗时调度带来的波动。

## 适用范围

这些规则覆盖帧输入、Timer、普通循环、显式等待、重绘边界和条件事件。启动、取消、广播和克隆仍有各自的
生命周期契约；跨帧等待任务的排序不代表外部事件或全部协程遵循全局 FIFO 顺序。实现位于 Go 层，
无需改变 Godot C++ 接口。

循环和条件事件的规则参考 Scratch 的相应机制，但不代表完整兼容 Scratch VM，
也不保证不同运行或平台具有相同的迭代次数。
例如，异步广播新建的可运行处理函数可能先于仍在等待队列中的旧脚本执行；
这与 Scratch 将新线程追加到既有线程列表末尾的规则不同。
调度 watchdog 只在调度循环取回控制权后检查时间，不提供抢占或硬超时保证。

## 回归覆盖

- 无重绘循环可同帧多轮执行，重绘不截断当前轮其他脚本。
- 显式等待仍跨越约定边界，`Warp` 的让出规则保持一致。
- 跨帧等待保留注册顺序；尚未到期的等待不会阻塞可恢复的脚本。
- 预算耗尽后脚本可在后续帧继续执行，并保留取消能力。
- 条件与处理函数分别观察时钟推进前、后的状态；慢帧不破坏阶段顺序。
- 普通输入、录制和回放共享帧批次；整批路由与登记在脚本调度前完成，Timer 使用推进后的时钟。
- 条件采样前回读物理位置，纯视觉修改保留物理位移，碰撞处理函数的修改进入帧末提交；
  显式等待未来帧的后续代码仍在后续帧恢复。
- runtime 更新回调能在最终提交前登记脚本和调用引擎，不进入独占采样回调。
- 条件事件维持上升沿、防重入和拥有者隔离语义。
- 采样阶段不会因待处理的主线程调用发生死锁。
- 同步引擎调用保持脚本片段连续；取消不会让正在执行的调用越过重置屏障。
- 接收结果只由恢复后的脚本提交，取消的 Join 等待者不残留引用。
- 活跃事件重启保留原排序位置，已停止或完成的事件重新注册。
- 并发清理等待者全部得到通知，最后一个外部工作结束前不会执行重置。
- 并发打开 Latch 不会重复恢复脚本，且 Open 返回时 Done 已关闭。
- 回调同步 drain 与独占回调同步执行脚本快速拒绝；停机等待继续处理 worker 的主线程收尾。
- worker 的 `Goexit` 唤醒并停止所属脚本，停机回调异常退出后恢复准入。

```sh
go test -race ./internal/coroutine ./internal/engine ./internal/core/runtime .
go test ./...
GOOS=js GOARCH=wasm go test -c -o /tmp/spx-scheduling.test.wasm .
```

核心回归位于 `runtime_scratch_scheduler_test.go`、`runtime_events_test.go`、
`runtime_event_order_test.go`、`runtime_engine_test.go`、`runtime_replay_test.go`、
`runtime_event_dispatch_native_test.go`、
`runtime_sync_test.go`、`runtime_engine_native_test.go`、`internal/engine/runtime_delta_test.go`、
`internal/engine/execute_native_test.go`、
`sprite_clone_collision_test.go`、`internal/coroutine/loop_test.go` 和
`internal/coroutine/order_test.go`。
取消与重启覆盖位于 `internal/coroutine/thread_test.go`、
`internal/coroutine/worker_test.go`、`internal/coroutine/join_test.go`、
`internal/coroutine/handler_test.go` 和 `internal/engine/execute_test.go`。
WASM 编译检查不替代浏览器运行验证。

## 参考实现

- [Scratch Sequencer.stepThreads](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/engine/sequencer.js)：轮次、预算与重绘边界。
- [Scratch Runtime.startHats / _step](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/engine/runtime.js)：条件事件与脚本执行阶段。
- [Scratch Clock](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/io/clock.js)：共享帧时钟。
