# SPX 脚本调度

[English](../../../en/dev/engine/scheduling.md)

## 概述

SPX 使用协作式脚本调度。脚本执行到让出执行权或结束后，调度器再让其他
可运行的脚本继续。循环边界、显式等待、重绘请求和条件事件共同决定脚本何时恢复。

本文描述 SPX 当前运行时的帧阶段、循环轮次，以及固定帧截图和输入回放的保证范围。
这些规则适用于所有使用相应 API 的项目。

## 帧阶段

一次 engine update 推进一次帧时钟，其中可以包含多轮脚本执行。
开始下一轮不会推进帧号或计时器。

1. 启动脚本到达首次让出或结束后，在帧时钟推进前采样条件事件。
2. 推进时钟并缓存引擎输入，派发已匹配的条件处理函数，处理活跃的输入 session，
   执行启动事件或已到期的帧回调。
3. 处理可运行的协程，以及满足条件的后续循环轮次。
4. 同步截图所需的视觉状态，派发截图队列，再完成输入 session 的帧收尾。
   Web host 等待渲染屏障后读取画布。

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

## 条件事件

1. **先检查条件，再刷新时钟。** 启动脚本到达首次让出或结束后，
   在每帧时钟推进前统一求值条件；时钟推进后再派发已匹配的处理函数，
   派发时不重复求值。条件按现有目标顺序检查，全部检查完成后才启动处理函数。
2. **条件事件按上升沿触发。** 持续为真不会重复启动处理函数。
   处理函数运行期间暂停该事件的条件检查，结束后继续检测上升沿。
   条件函数必须快速返回，不调用等待类 API。

## 实现结构

| 职责 | 位置 |
| --- | --- |
| 帧等待、循环轮次、重绘预算、主线程状态读取 | `internal/coroutine/frame.go` |
| 等待任务处理与调度统计 | `internal/coroutine/update.go` |
| 控制流与调度器衔接 | `internal/engine/coro.go` |
| 帧阶段顺序 | `internal/engine/engine.go`、`runtime_engine.go` |
| 条件注册、状态采样与已匹配事件派发 | `runtime_conditions.go` |
| 可见性判断与重绘通知 | `sprite_render.go` 及对应视觉操作入口 |

帧边界的状态读取与脚本执行互斥，同时处理已排队的引擎主线程调用，
避免正在等待引擎调用的脚本阻塞条件采样。读取阶段不推进循环或帧等待任务。

## 固定帧截图与输入回放

游戏运行期间，`AtFrame` 按 engine-session 帧号安排回调，`Snapshot` 将截图请求入队，
在帧末视觉状态同步后派发。
这两个 API 确定帧边界，不固定到达该边界前完成的脚本工作量。
host 接入方式见[《Web 端截图与固定帧接入说明》](web_capture.md)。

[输入录制与回放](input_replay.md)固定输入序列、逻辑时间步长和脚本随机种子，
但不固定或记录每帧的脚本轮数。循环调度仍然检查真实耗时。
如果画面状态依赖预算内完成的循环工作量，即使使用相同构建和输入记录，
机器负载差异也可能让相同 input tick 的截图不同。

回归场景应使用显式帧边界或逻辑时间边界推进可观察状态，避免依赖预算内的迭代次数。
调度规则变化后重新生成 baseline 可以反映新的语义，但不能消除真实耗时调度带来的波动。

## 适用范围

这些规则覆盖普通循环、重绘边界和条件事件。启动、取消、广播和克隆仍有各自的
生命周期契约，调度器不对全部协程施加全局 FIFO 顺序。实现位于 Go 层，
无需改变 Godot C++ 接口。

循环和条件事件的规则参考 Scratch 的相应机制，但不代表完整兼容 Scratch VM，
也不保证不同运行或平台具有相同的迭代次数。

## 回归覆盖

- 无重绘循环可同帧多轮执行，重绘不截断当前轮其他脚本。
- 显式等待仍跨越约定边界，`Warp` 的让出规则保持一致。
- 预算耗尽后脚本可在后续帧继续执行，并保留取消能力。
- 条件与处理函数分别观察时钟推进前、后的状态；慢帧不破坏阶段顺序。
- 条件事件维持上升沿、防重入和拥有者隔离语义。
- 采样阶段不会因待处理的主线程调用发生死锁。

```sh
go test -race ./internal/coroutine ./internal/engine ./internal/core/runtime .
go test ./...
GOOS=js GOARCH=wasm go test -c -o /tmp/spx-scheduling.test.wasm .
```

核心回归位于 `runtime_scratch_scheduler_test.go`、`runtime_events_test.go`、
`runtime_events_order_test.go` 和 `internal/coroutine/loop_test.go`。
WASM 编译检查不替代浏览器运行验证。

## 参考实现

- [Scratch Sequencer.stepThreads](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/engine/sequencer.js)：轮次、预算与重绘边界。
- [Scratch Runtime.startHats / _step](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/engine/runtime.js)：条件事件与脚本执行阶段。
- [Scratch Clock](https://github.com/scratchfoundation/scratch-vm/blob/develop/src/io/clock.js)：共享帧时钟。
