# 输入录制与回放

输入录制与回放用于把一局游戏中的键盘、鼠标输入保存为 `InputReplay`，并在一局全新的游戏中按相同 input tick 重放。

它的基本单位是一次完整的 Game lifecycle，而不是运行中可以随时打开、关闭的输入过滤器。

```text
startGame({ input })
        │
        ▼
Prepared → Running → Finishing → Completed
                      │
reset / panic / crash └────────→ Aborted
```

- 一局 Game 最多拥有一个 Input Session。
- Session 必须在解释器和游戏 bootstrap 启动前准备。
- 同一局中不能从 live input 切换到 record/replay，也不能在回放后切回 live input。
- Game reset、destroy、panic 或 crash 会中止 Session；未完成的录制不能作为 baseline。
- 同一个 `*Game` 在 reload 时可能被复用，但新的 bootstrap generation 会获得新的 Session 和 tick `0`。

runner 的 `defaultMode` 默认为 `normal`。不传 `input` 时会使用这个默认模式；显式传 `input: null` 则始终强制普通实时输入，此时 `Game.inputSession == nil`。

## Web host API

普通 Web runner 将输入配置直接放在 `startGame` 参数中。准备 Session 和启动解释器由同一个串行任务完成，不需要单独 arm。

### 录制一局

```js
await runnerWindow.startGame({
	input: {
		mode: 'record',
	},
})

// 用户完成操作后结束整局游戏。
const { inputReplay } = await runnerWindow.stopGame()
```

`inputReplay` 是 JSON `Blob`；普通游戏或被中止的录制返回 `null`。

Web runner 默认以 `30 FPS` 录制，并为录制和回放配置 `captureKey: 'P'`。`captureKey` 使用 `KeyFromString` 支持的键名，例如 `P`、`PrintScreen`、`F8`；设为 `null` 可关闭。配置后，该按键的 press 边沿会在录制和回放的相同 input tick 请求截图，同时仍会正常派发给游戏脚本。SPX core 本身不注册默认截图键，这些默认值只属于 Web runner。

`stopGame()` 会按以下顺序执行：

1. 完成当前录制并冻结结果；
2. 等待当前 Web 截图队列清空；
3. 请求解释器退出；
4. 等待该局真实退出，而不是只等待 `ispx_stop()` 返回；
5. 再次确认截图队列为空并返回 replay Blob。

只有该受控流程返回的数据才应提交为 baseline。浏览器崩溃、runtime panic 或直接 reset 都属于 `Aborted`。

### 回放一局

```js
await runnerWindow.startGame({
	input: {
		mode: 'replay',
		data: inputReplay,
	},
})

await runnerWindow.waitInputSessionCompleted({ timeoutMs: 30_000 })
await runnerWindow.stopGame()
```

`data` 支持 JSON string、`Blob` / `File`、`ArrayBuffer` 和 `Uint8Array`。runner 会复制输入字节，避免调用方在启动任务排队期间修改数据。

### 默认配置与上层覆盖

runner 公开一份深度冻结的有效配置，并用它实际生成 Input Session，而不只是把它当作能力说明：

```js
runnerWindow.spxRunnerConfig
// {
//   input: {
//     defaultMode: 'normal',
//     record: { fps: 30, captureKey: 'P' },
//     replay: { captureKey: 'P' },
//   },
//   limits: { inputReplayBytes: 16 * 1024 * 1024 },
// }
```

上层在 `runnerReady` 后、启动游戏前只覆盖需要改变的字段：

```js
runnerWindow.spxConfigureRunner({
	input: {
		record: { captureKey: 'PrintScreen' },
		replay: { captureKey: 'PrintScreen' },
	},
})
```

覆盖会合并到当前配置并返回新的冻结快照，显式 `captureKey: null` 会关闭快捷键。`limits.inputReplayBytes` 对应 core 的固定解码上限，只读且不可覆盖。

需要在上层构造完整参数时，可复用同一配置源：

```js
const input = runnerWindow.spxCreateInputSession('record')
// { mode: 'record', fps: 30, captureKey: 'PrintScreen' }
```

baseline/run 目录、replay 文件名、模式按钮和图片比较仍属于上层 host，不进入 runner 配置。

`waitInputSessionCompleted()` 在最后一个完整帧结束后返回，并等待截图队列清空。流程控制应使用该 Promise，不要由产品代码轮询 tick。

调试时可以查询：

```js
const status = await runnerWindow.getInputSessionStatus()
// {
//   mode: 'idle' | 'recording' | 'replaying',
//   phase: 'prepared' | 'running' | 'finishing' | 'completed' | 'aborted',
//   completed: boolean,
//   exhausted: boolean,
//   currentTick: number | null,
//   nextFrame: number,
//   frameCount: number,
//   error?: string,
// }
```

`currentTick` 是最近消费的 effective input tick，首个 tick 前为 `null`。`exhausted` 只表示回放输入已经读完；`completed` 表示最后一帧的脚本、协程、视觉同步和截图派发均已完成。

`Completed` / `Aborted` 诊断状态属于当前 Game generation，会保留到下一局开始；下一局认领输入生命周期后重置为它自己的 `Prepared` / `Running`，普通启动则回到 `Idle`。

## 推荐的上层流程

录制：

```text
停止并等待旧游戏退出
→ 加载项目
→ 安装 baseline capture host
→ startGame(record)
→ 用户操作
→ stopGame，取得 inputReplay
→ 提交 replay 和 baseline 截图
```

回放：

```text
停止并等待旧游戏退出
→ 加载相同项目
→ 安装 run capture host
→ startGame(replay)
→ 等待 Input Session completed
→ 停止并等待游戏退出
→ 比较 run 与 baseline
```

项目文件、baseline/run 目录、图片比较和 UI 都属于上层 host；SPX core 只负责 Game Session、输入数据和截图请求中的 input tick 元数据。

## Input tick

Input Session 使用独立的 input tick，不等同于 `spx.CurrentFrame()`：

- `Prepared` 不产生 tick；
- 第一条通过 `IsRunned` 门槛的有效 `OnEngineUpdate` 消费 tick `0`；
- 此后每个有效 update 前进一个 tick；
- 资源加载、bootstrap 和 engine frame 不会改变 tick 编号。

输入在 `OnEngineUpdate` 开头解析，之后才执行当帧的 SPX 脚本逻辑。

```text
Initial         = tick 0 有序输入边沿发生前的状态
Frames[0].State = tick 0 结束时的完整状态
Frames[0].Time  = 0
```

录制在新 Game 的第一个 tick 才采样真实输入。运行时会从 tick `0` 的最终状态和有序边沿反推 `Initial`，因此同一 tick 内完成的 press/release 短点击不会丢失。

空 replay 合法。它不会在 `Prepared` 时直接完成，而是在第一个有效 input tick 使用 `Initial`，并于该帧末变为 `Completed`。

## v1 数据范围

每个 tick 记录：

- 鼠标位置；
- 左、中、右键当前状态及有序 press/release 边沿；
- 当前按键集合及有序 key press/release 边沿。

完整快照用于轮询 API，有序边沿用于保留同一 tick 内的短点击。回放期间仍会排空真实输入队列，但不会把真实输入交给游戏。

v1 不记录鼠标与键盘事件之间的全局交错顺序。鼠标位置也是每 tick 一份快照，不是每条边沿一份坐标。

Replay JSON 使用严格字段校验，编解码和 Web 参数均限制为 16 MiB。

## EOF 与帧末完成

消费最后一条 replay frame 后，Session 先进入 `Finishing`：

1. 输入 controller 标记 `exhausted`；
2. 当前 tick 的事件、协程和脚本继续执行；
3. 视觉代理同步；
4. Go capture 请求派发到 Web；
5. `OnEngineFrameEnd` 将 Session 标记为 `Completed` 并暂停引擎。

因此上层不会在最后一条输入刚取出时过早停止游戏，也不会截断最后一帧的点击回调或截图。

Replay EOF 后不恢复实时输入。需要提前取消回放时，直接结束整局游戏。

## 确定性环境

Input Session 自己拥有本局的确定性环境：

```text
重新启动游戏       保证相同游戏初始状态
InputReplay.Initial 保证相同输入初始状态
InputReplay.Frames  保证每个 input tick 的输入一致
FixedTimestep       保证 SPX logical delta
随机种子 1          保证脚本随机源可重现
```

- 录制 FPS 转换为 `InputReplay.FixedTimestep`。
- 回放在 `FixedTimestep > 0` 时使用 replay 中保存的 logical timestep；值为 `0` 的旧数据保持原有 variable timestep 语义。
- record/replay 使用随机种子 `1` 的逐协程确定性随机流。
- Session 环境持续到 Game lifecycle 结束，然后恢复普通运行环境。
- Session 不修改项目 `MaxFPS`；MaxFPS 只控制运行速度，不属于确定性协议。

项目 `Config` 不提供 `deterministic`、`fixedTimestep` 或 `randomSeed`。这些不是项目配置，而是 Input Session 的运行约束。

输入一致仍不等于整个游戏完全确定。网络、文件 I/O、外部异步顺序、字体、GPU 和 Godot fixed physics 都可能造成差异。

## Go host 底层接口

这些函数用于 host bridge，必须在下一局 Game 创建前调用：

```go
preparation, err := spx.PrepareInputRecording(30, spx.InputSessionOptions{
	CaptureKey: spx.KeyP,
})
if err != nil {
	return err
}
defer preparation.Cancel()

// 启动并运行一局 Game；Game 创建时会认领 preparation。

replay, err := spx.FinishInputRecording()
if err != nil {
	return err
}
jsonText, err := spx.EncodeInputReplay(replay)
```

只需要持久化 JSON 的 host 可直接调用 `FinishInputRecordingJSON()`；它返回录制提交时已经生成并缓存的 JSON，不会再次编码：

```go
jsonText, err := spx.FinishInputRecordingJSON()
```

回放：

```go
replay, err := spx.DecodeInputReplay(jsonText)
if err != nil {
	return err
}
preparation, err := spx.PrepareInputReplay(replay, spx.InputSessionOptions{
	CaptureKey: spx.KeyP,
})
if err != nil {
	return err
}
defer preparation.Cancel()

// 启动下一局 Game，并通过 GetInputSessionStatus 观察诊断状态。
```

`InputSessionPreparation` 是一次性句柄。若 Game 尚未认领它，`Cancel` 会取消本次准备；认领之后调用 `Cancel` 是安全的空操作，也不会误删后续准备。因此 host 应持有句柄，并在启动流程退出时调用 `Cancel` 清理未认领状态。

运行中的 Game 已经拥有 Session 时，再次 Prepare 会返回 `ErrInputSessionActive`。不存在 `StopInputReplay`：回放和 Game 一起结束。

## 截图配合

`spx.Snapshot` 会在 decorator body 成功后排入帧末截图请求，请求始终带有 engine `frame`。当前 Game Session 已经消费输入时，还会带有最近的 `inputTick`。

```text
<timeline>_<sequence>[_<name>].png
```

消费首个 input tick 后，`timeline` 是补齐四位的 `inputTick`；此前回退为 `frame_<frame>`。`name` 为空时文件名直接结束在 sequence，例如 `0833_0004.png`。新的 Game 不会继承上一局的 input tick。Session 启动后、tick `0` 之前发出的截图没有 `inputTick`。

脚本只声明 `Snapshot` 点位；图片是 baseline 还是 run，以及是否对比，由 `Record` / `Replay` 模式和外部 host 决定。

core 默认不注册截图快捷键。host 可以通过 `InputSessionOptions.CaptureKey` 显式启用；Web 测试页默认使用 `P`，可通过 `?captureKey=PrintScreen` 修改，或使用 `?captureKey=off` 关闭。快捷键、capture marker 和测试 UI 仍由上层决定，普通项目不会被隐式占用按键。

项目已有的 `run.screenshotKey` / `SPX_SCREENSHOT_KEY` 属于引擎原生手工截图通道，不携带 Input Session 的 replay 语义。确定性回归应使用 `captureKey`，并避免同时配置两套快捷键造成重复截图。

## v1 暂不支持

- Godot action / axis；
- 原生 UI 文本输入和输入法组合状态；
- touch / 多点触控；
- 鼠标滚轮；
- gamepad / joystick；
- Godot fixed physics lockstep。

这些输入或物理状态可能继续读取平台实时值，依赖它们的项目不能只靠 v1 replay 保证截图一致。
