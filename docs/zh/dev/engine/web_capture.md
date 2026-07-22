# Web 端截图与固定帧接入说明

本文档说明外部页面如何像 Web 测试模板一样，接入 SPX 的固定帧截图能力。模板的页面结构、行为和样式分别位于 [`index.html`](../../../../cmd/spx/template/platform/web/index.html)、[`lab/index.js`](../../../../cmd/spx/template/platform/web/lab/index.js) 和 [`lab/index.css`](../../../../cmd/spx/template/platform/web/lab/index.css)。

如果截图回归还需要录制并重放固定输入，请同时参考[《输入录制与回放说明》](./input_replay.md)。input tick 是独立时间轴，不等同于 `spx.CurrentFrame()` 返回的 engine-session 帧。

适用场景：

- 在 Web 端做固定输入回归截图
- 在自定义 `index.html` 中接管截图保存、对比、上报
- 在某一帧触发截图，而不是手动点击按钮截图

## 1. SPX 侧能力

SPX 运行时对外暴露了这几个入口，定义在 [`runtime_frame.go`](../../../../runtime_frame.go) 和 [`runtime_snapshot.go`](../../../../runtime_snapshot.go)：

- `spx.CurrentFrame()`：返回从 engine `onStart` 开始递增的帧号
- `spx.AtFrame(frame, fn)`：在 engine-session 绝对帧 `frame` 执行一次回调，`frame` 的类型是 `int64`
- `spx.Snapshot(name, fn)`：先执行 `fn`，成功后在状态同步后的当前 engine frame 末尾排入一次截图请求；没有活跃 Game 时立即派发请求

一个最小例子：

```go
spx.AtFrame(60, func() {
	spx.Snapshot("frame_060", nil)
})

spx.AtFrame(120, func() {
	spx.Snapshot("frame_120", nil)
})
```

`Snapshot` 的 `fn` 会先执行，成功后才排入截图请求；如果 `fn` 返回错误，运行时会报告错误且不会截图。传入 `nil` 可以不执行 body，直接排入请求。这样可以把“操作”和“截图”写在同一个步骤里。如果当前 Game 有活跃的 Input Session，请求还会携带最近消费的 input tick。

上例中的 `60` 和 `120` 是从 engine `onStart` 起算的绝对帧，不是相对 bootstrap 或游戏 `OnStart` 的帧数。异步资源加载也会消耗 engine frame，因此游戏逻辑注册回调时这些目标可能已经到期。通常应显式计算相对当前时刻的目标：

```go
base := spx.CurrentFrame()
spx.AtFrame(base+60, func() {
	spx.Snapshot("after_060", nil)
})
spx.AtFrame(base+120, func() {
	spx.Snapshot("after_120", nil)
})
```

固定帧语义如下：

- `CurrentFrame()` 直接读取 `itime.Frame()`；`time.Start()` 在 engine `onStart` 中将它置为 `0`
- 每次 engine update 都会推进帧号，包括异步资源加载和 bootstrap 期间的 update
- bootstrap、游戏 `OnStart` 和普通运行阶段都不会再次归零
- 游戏内 reload 不重置帧号；新的 engine time session 才会从 `0` 重新开始
- reset/reload 仍会清除未执行的 `AtFrame` 回调和截图请求，但不修改 engine-session 帧计数
- `AtFrame(frame, fn)` 的 `frame` 始终是 engine-session 绝对目标帧；如果注册时目标帧已经到达，回调会立即进入可调度状态
- `Snapshot` 写入请求的 `frame` 是调用时的 engine-session 帧；有活跃 Input Session 且已消费输入时，还会写入最近的 `inputTick`
- coroutine 调度使用内部单调 scheduler frame，新的 engine time session 不会提前唤醒遗留的 coroutine wait
- `AtFrame` 回调会在注册对象所属的 SPX coroutine 中执行，可以安全使用 `wait`、`waitNextFrame`、`animateAndWait` 等会 yield 的操作
- 同一帧的回调按注册顺序执行到首次 yield 或结束；yield 后回到普通 coroutine 调度，不承诺所有 coroutine 的全局执行顺序
- 已到期的回调会在当前或紧随其后的 coroutine update 中执行；从 coroutine 内注册时，会先推进回调到首次 yield 或结束再继续调用方
- 从普通事件 coroutine 调用 `Snapshot` 时，运行时会在截图前再次同步本帧产生的视觉状态

### 后续装饰器简化

当前这套能力已经可以直接使用，但写法还是显式调用 `spx.AtFrame(...)` / `spx.Snapshot(...)`。

后续如果 xgo 的装饰器能力可用（见 `goplus/xgo#2797`），那么同样的逻辑可以进一步简化成直接写在 XGo/SPX 源码里的装饰器风格。

也就是说，上面的当前写法：

```go
spx.AtFrame(60, func() {
	spx.Snapshot("frame_060", nil)
})

spx.AtFrame(120, func() {
	spx.Snapshot("frame_120", nil)
})
```

后续原则上可以写成类似下面的声明式形式：

```go
@atFrame(60)
@snapshot("frame_060")
func SnapshotFirstState() {
}

@atFrame(120)
@snapshot("frame_120")
func SnapshotSecondState() {
}
```

装饰器会把被装饰的函数 body 传给 `Snapshot`；body 成功后才排入截图。这样测试脚本会更接近 issue #1652 想要的“在引擎内部声明式地按帧触发截图”的目标。

需要注意：

- 目前仓库中稳定可用的是显式函数调用方式，文档后续示例只是说明未来可简化的方向
- 最终装饰器名字和展开细节，仍以 xgo 对装饰器能力的实际支持为准

## 2. Web 端桥接点

Web 运行时由 [`capture.js`](../../../../cmd/ispx/web/capture.js) 排队和投递截图请求，[`runner.html`](../../../../cmd/ispx/web/runner.html) 只负责把这项能力接到 Game 生命周期与公开 API。

外部页面需要关心的接口有三个：

- `runnerWindow.spxSetCaptureHost(host)`
- `runnerWindow.spxGetCaptureHost()`
- `await runnerWindow.spxFlushCaptureQueue()`

其中 `host` 可以是：

- 一个函数：`async function (request, blob) {}`
- 一个对象：`{ async handleCapture(request, blob) {} }`

如果没有安装 host，Web 端会退化为浏览器下载 PNG。截图会按 request sequence 串行交给 host；同一 engine frame 的多个请求共享同一份经过 render fence 的画面数据。

`spxFlushCaptureQueue()` 会等待调用前已经排入的截图完成采图和 host 保存。停止游戏、切换项目、替换 host 或提交 baseline 前必须等待它完成。

## 3. Web host 截图请求格式

传给 host 的 `request` 是 Web 平台桥接对象，不是 SPX 脚本的公开类型。当前 `Snapshot` 产生的结构如下：

```js
{
  name: "title",
  filename: "0060_0001_title.png",
  inputTick: 60,
  frame: 62,
  sequence: 1
}
```

字段含义：

- `name`：SPX 脚本传入的名字
- `filename`：runner 生成的稳定文件名，保证是 `.png`
- `inputTick`：截图请求发出时最近消费的 input tick；没有活跃 input tick 时为 `null`
- `frame`：截图请求对应的 engine-session 帧号，作为采图协调与诊断元数据保留
- `sequence`：当前运行期内的截图序号

脚本只声明 `Snapshot` 点位。当前图片是 baseline、run，还是需要立即对比，由 `Record` / `Replay` 运行模式和外部 host 决定。

runner 的命名规则是：

```text
<timeline>_<sequence>[_<name>].png
```

其中数字至少补齐四位，`name` 为空时不会添加占位词，例如 `0833_0004.png`；显式命名时则生成 `0833_0004_title.png`。输入录制/回放 session 内，baseline 和 run 以 `inputTick + sequence + name` 配对，不受启动加载使同一 input tick 落到不同 engine frame 的影响。没有活跃 input tick 时，timeline 段回退为 `frame_<frame>`，例如 `frame_0062_0001_title.png`。Start 只武装 session，因此 tick `0` 尚未消费前也会使用 frame 回退。

旧版本按 engine frame 命名的 baseline 不会与新文件名自动配对；升级后应选择 `Record` 模式，通过统一的 `Start` / `Stop` 流程重新生成 baseline。

## 4. 最小接入方式

如果你有自己的 `index.html`，最简单的接法如下：

```html
<iframe id="runnerFrame" src="runner.html"></iframe>
<script>
  const runnerFrame = document.getElementById('runnerFrame')
  const runnerWindow = runnerFrame.contentWindow

  function waitForRunnerReady() {
    if (typeof runnerWindow.spxSetCaptureHost === 'function') {
      return Promise.resolve()
    }
    return new Promise((resolve) => {
      runnerWindow.addEventListener('runnerReady', resolve, { once: true })
    })
  }

  async function installCaptureHost() {
    await waitForRunnerReady()
    runnerWindow.spxSetCaptureHost({
      async handleCapture(request, blob) {
        console.log('capture request', request)

        // 这里可以上传、保存到浏览器文件系统、或者做自定义对比
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = request.filename
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        URL.revokeObjectURL(url)
      },
    })
  }

  installCaptureHost()

  async function finishCaptureSession() {
    await runnerWindow.spxFlushCaptureQueue()
    // 此后再停止游戏、切换项目或替换 host。
  }
</script>
```

这个例子只是把 runner 默认下载逻辑搬到了外部页面。真正有用的做法通常是：

- 自己控制保存目录
- 直接使用 `request.filename` 保存，或按 `request.inputTick`、`request.sequence` 和 `request.name` 自定义命名
- 把 baseline 和 runs 分开
- 在 host 中直接做相似度对比

## 5. 像模板页那样保存 baseline / runs

仓库里的 Web 测试模板提供了完整参考实现：[`index.html`](../../../../cmd/spx/template/platform/web/index.html) 只保留页面结构，[`lab/index.js`](../../../../cmd/spx/template/platform/web/lab/index.js) 管理配置、Game 生命周期和文件保存，[`lab/index.css`](../../../../cmd/spx/template/platform/web/lab/index.css) 管理样式。

页面只有一套 `Start` / `Stop` 游戏生命周期接口。`Mode` 按钮在游戏停止时按 `Normal → Record → Replay` 循环切换，所选模式只决定下一局传给 `startGame` 的 `input` 配置，运行中不会切换 Input Session：

- `Normal`：按原有实时输入启动，截图保存到 `spx-snapshots/<project>/runs/<timestamp>/`
- `Record`：以 `30 FPS` 录制输入，截图写入 `baseline/`；必须调用 `Stop` 正常结束整局，模板才会取得 replay Blob 并保存为 `baseline/input-replay.json`
- `Replay`：启动前读取 `baseline/input-replay.json`，以相同 input tick 重放输入，并把截图写入 `runs/<timestamp>/`；输入耗尽不会自动停止 Game，调用方仍需按正常生命周期调用 `Stop`

Web runner 的 `Record` 和 `Replay` 默认配置 `captureKey: 'P'`。按下 `P` 的 press 边沿会在两次运行的相同 input tick 请求截图；测试页只通过 `spxConfigureRunner(...)` 覆盖差异，支持用 `?captureKey=PrintScreen` 替换按键，或用 `?captureKey=off` 关闭。core 本身不默认占用 `P`，这是 Web runner 的 host 默认值。

runner 的输入默认值定义在 [`input-session.js`](../../../../cmd/ispx/web/input-session.js)，也是 `startGame` 实际使用的唯一配置源。自定义上层可以读取 `spxRunnerConfig`，调用 `spxConfigureRunner(overrides)` 覆盖下一局默认值，并用 `spxCreateInputSession(mode, overrides)` 构造完整参数；baseline/run 目录和 replay 文件名仍由上层管理。完整接口见[《输入录制与回放》](./input_replay.md#默认配置与上层覆盖)。

模板保存 baseline / runs 的核心流程是：

1. 页面启动项目时，从 zip 中读取：
   - `builder-meta.json`
   - `assets/index.json`
2. 用 `displayName` 或项目名作为目录名
3. `Normal` 和 `Replay` 保存到 `spx-snapshots/<project>/runs/<timestamp>/`
4. `Record` 保存到 `baseline/`，并在 `Stop` 成功返回后把录制 JSON 写为 `baseline/input-replay.json`
5. `Replay` 读取这份 JSON，并由 Snapshot Comparison 面板对比本次 run 与已提交的 baseline

项目的 `run` 配置不再提供 `deterministic`、`fixedTimestep` 或 `randomSeed`。`Record` 和 `Replay` 的 Go Input Session 使用 `30 FPS` logical timestep 与种子 `1` 的确定性随机源；原始项目 zip 和 `Normal` 模式不受影响。

baseline/run 使用显式分类：`Record` 会覆盖已有 baseline，`Replay` 和 `Normal` 始终创建新的 run。

baseline 采用两阶段提交：采图期间只写 PNG；`Record` 模式调用 `Stop` 后，模板取得并验证 replay Blob，等待 capture queue，确认 JSON 和本 session 的截图全部保存成功，再写 `.baseline.ready`。如果 Game 没有经过 `Stop` 就自行结束，录制会被丢弃；crash、写文件失败或没有截图时也不会提交 marker。下一次 `Record` session 会清理未提交的残留文件后重新生成。

## 6. 外部页面推荐实现步骤

如果你想复制模板页思路，建议按下面的顺序实现：

1. 启动项目前先拿到项目元信息
2. 选择一个截图根目录
3. 调用 `runnerWindow.spxSetCaptureHost(...)`
4. 在 Game 停止时选择 `Normal`、`Record` 或 `Replay`，把相应 `input` 配置传给统一的 `startGame`；不要在同一局中切换模式
5. 在 host 里直接使用 `request.filename`；自定义命名时优先使用 `request.inputTick`，把 `request.frame` 留作诊断
6. `Record` 写 baseline；`Replay` 读取 replay JSON 并写 runs；`Normal` 直接写 runs
7. 所有预期 `Snapshot` 都触发后，在停止、切换项目或替换 host 前等待 `spxFlushCaptureQueue()`；官方 runner 的 `stopGame` 已包含这一步，自定义停止流程需要显式处理
8. 调用统一的 `stopGame`；`Record` 只有此时才返回可保存的 replay Blob，`Replay` 输入耗尽后也不会自动调用它
9. 如果要做回归测试，再把已提交的 baseline 与当前 run 做对比

runner 默认生成的文件名：

```text
0060_0001_title.png
0120_0002_enemy_wave.png
```

## 7. 模板页额外提供了什么

当前模板页除了保存截图，还额外做了这些事情：

- `Mode` 按钮：选择下一局的 `Normal`、`Record` 或 `Replay` 模式
- 统一的 `Start` / `Stop`：三种模式共享同一套 Game 生命周期
- `Choose Folder` 按钮：选择浏览器文件系统目录
- `Refresh` 按钮：重新读取 baseline / runs
- `Snapshot Comparison` 面板：并排展示 baseline 与当前 run
- 相似度计算：先缩略灰度快速比较，再按需做更细的 RGBA 比较

如果你只是要“接入能力”，这些都不是必须的；真正必须的只有：

- 在 SPX 里调用 `AtFrame` / `Snapshot`
- 在外部页面安装 `spxSetCaptureHost`
- 在停止、切换 session 或替换 host 前等待 `spxFlushCaptureQueue()`

## 8. 与 runner 的事件协作

`runner.html` 在截图完成或失败时还会按 request sequence 派发 `spxCaptureScreenshot` 事件。你如果不想直接通过 host 做逻辑，也可以监听这个事件：

```js
window.addEventListener('spxCaptureScreenshot', (event) => {
  console.log(event.detail)
})
```

事件里会包含：

- `ok`
- `name`
- `filename`
- `inputTick`
- `frame`
- `sequence`
- 成功时的 `destination`、`size`、`type`
- 失败时的 `error`

## 9. 注意事项

- `Record` / `Replay` 依赖浏览器文件系统目录选择能力，用于持久化或读取 `baseline/input-replay.json`；`Normal` 截图在没有 host 时仍可退回浏览器下载
- `Snapshot` 只声明截图点位；Record 写 baseline，Replay 写 run，真正何时以及如何对比由外部 host 决定
- Web 测试页在 `Record` / `Replay` 中默认使用 `P` 作为确定性截图键；可通过 `captureKey` 配置更换或关闭，普通项目不会被 core 隐式占用该键
- 输入录制/回放 session 统一 SPX logical update delta 和脚本随机源；公开帧号从 engine `onStart` 起算并包含异步资源加载，bootstrap 完成后会先把游戏 OnStart 推进到既定 coroutine 边界，再继续处理积压的 `AtFrame` 回调；它不会把全部 coroutine 强制成全局 FIFO，也不包含 Godot fixed physics delta
- `Replay` 只重放输入，不接管 Game 生命周期；输入耗尽不会自动停止 Game，host 必须根据自己的生命周期调用 `stopGame`
- 如果项目逻辑依赖物理 fixed-update，本方案不保证截图一致性
- 真实输入、网络/文件 I/O 的完成顺序不会被虚拟化；回归脚本应使用固定输入并避免让外部异步结果决定目标帧状态
- 字体、GPU 和浏览器渲染仍可能产生少量像素差异，回归比较应使用合理的相似度阈值，而不是要求 PNG 字节完全相同
- `Record` 必须在所有预期截图触发后通过 `stopGame` 正常结束；Game 自行退出、panic 或 crash 都不会提交可靠 baseline

## 10. 参考文件

- [`runtime_frame.go`](../../../../runtime_frame.go)
- [`runtime_snapshot.go`](../../../../runtime_snapshot.go)
- [`runtime_capture_js.go`](../../../../runtime_capture_js.go)
- [`internal/time/time.go`](../../../../internal/time/time.go)
- [`cmd/ispx/web/runner.html`](../../../../cmd/ispx/web/runner.html)
- [`cmd/ispx/web/input-session.js`](../../../../cmd/ispx/web/input-session.js)
- [`cmd/ispx/web/capture.js`](../../../../cmd/ispx/web/capture.js)
- [`cmd/spx/template/platform/web/index.html`](../../../../cmd/spx/template/platform/web/index.html)
- [`cmd/spx/template/platform/web/lab/index.js`](../../../../cmd/spx/template/platform/web/lab/index.js)
- [`cmd/spx/template/platform/web/lab/index.css`](../../../../cmd/spx/template/platform/web/lab/index.css)
