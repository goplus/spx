# Web 端截图接入说明

本文档说明外部页面如何像 Web 测试模板一样，接入 SPX 的帧末截图能力。模板的页面结构、行为和样式分别位于 [`index.html`](../../../../cmd/spx/template/platform/web/index.html)、[`lab/index.js`](../../../../cmd/spx/template/platform/web/lab/index.js) 和 [`lab/index.css`](../../../../cmd/spx/template/platform/web/lab/index.css)。

如果截图回归还需要录制并重放固定输入，请同时参考[《输入录制与回放说明》](./input_replay.md)。截图元数据中的 input tick 是独立时间轴，不等同于 engine-session 帧。

相同帧号或 input tick 不保证脚本状态完全一致。循环轮次受真实耗时预算控制，输入录制与回放不固定或记录这些轮次。如果画面状态依赖预算内完成的迭代次数，不同运行的截图仍可能变化。详见[《SPX 脚本调度》](./scheduling.md#截图与输入回放)。

适用场景：

- 在 Web 端做固定输入回归截图
- 在自定义 `index.html` 中接管截图保存、对比、上报
- 在脚本执行到指定步骤时请求截图

## 1. SPX 侧能力

SPX 通过 [`spx.Snapshot(name, fn)`](../../../../runtime_snapshot.go) 请求截图。它先执行 `fn`，成功后将请求入队，并在当前 engine frame 的视觉状态同步后派发；没有活跃 Game 时立即派发请求。

一个最小例子：

```go
// 在游戏脚本到达待验证的步骤时调用。
spx.Snapshot("title", nil)
```

`Snapshot` 的 `fn` 会先执行，成功后才排入截图请求；如果 `fn` 返回错误，运行时会报告错误且不会截图。传入 `nil` 可以不执行 body，直接排入请求。这样可以把“操作”和“截图”写在同一个步骤里。如果当前 Game 有活跃的 Input Session，请求还会携带最近消费的 input tick。

录制与回放可通过 host 配置的 `captureKey` 触发截图。按键的 press 边沿随输入流一起记录，回放时会在相同 input tick 再次请求截图；Web runner 默认使用 `P`。游戏脚本无需额外注册定时回调。

截图时序如下：

- 请求中的 `frame` 来自内部 engine-session 帧计数，从 engine `onStart` 开始，包含资源加载和 bootstrap 期间的 update。
- `Snapshot` 在 body 成功返回后读取 `frame` 和最近消费的 `inputTick`；body 若在协程中 yield，截图点也会随 body 的完成时间后移。
- 从普通事件协程调用 `Snapshot` 时，运行时会在派发截图前再次同步本帧的视觉状态。
- reset 会清除尚未派发的截图请求；engine-session 帧计数只在新的引擎时间会话开始时归零。
- `frame` 用于同帧截图合并和诊断；录制与回放的图片以 `inputTick` 配对，不依赖加载阶段消耗的帧数。

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

- 在 SPX 里调用 `Snapshot`，或在录制与回放中配置 `captureKey`
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
- 输入录制/回放 session 统一 SPX logical update delta 和脚本随机源；它不会把全部 coroutine 强制成全局 FIFO，也不包含 Godot fixed physics delta
- `Replay` 只重放输入，不接管 Game 生命周期；输入耗尽不会自动停止 Game，host 必须根据自己的生命周期调用 `stopGame`
- 如果项目逻辑依赖物理 fixed-update，本方案不保证截图一致性
- 真实输入、网络/文件 I/O 的完成顺序不会被虚拟化；回归脚本应使用固定输入并避免让外部异步结果决定目标帧状态
- 字体、GPU 和浏览器渲染仍可能产生少量像素差异，回归比较应使用合理的相似度阈值，而不是要求 PNG 字节完全相同
- `Record` 必须在所有预期截图触发后通过 `stopGame` 正常结束；Game 自行退出、panic 或 crash 都不会提交可靠 baseline

## 10. 参考文件

- [`runtime_snapshot.go`](../../../../runtime_snapshot.go)
- [`runtime_input.go`](../../../../runtime_input.go)
- [`runtime_capture_js.go`](../../../../runtime_capture_js.go)
- [`internal/time/time.go`](../../../../internal/time/time.go)
- [`cmd/ispx/web/runner.html`](../../../../cmd/ispx/web/runner.html)
- [`cmd/ispx/web/input-session.js`](../../../../cmd/ispx/web/input-session.js)
- [`cmd/ispx/web/capture.js`](../../../../cmd/ispx/web/capture.js)
- [`cmd/spx/template/platform/web/index.html`](../../../../cmd/spx/template/platform/web/index.html)
- [`cmd/spx/template/platform/web/lab/index.js`](../../../../cmd/spx/template/platform/web/lab/index.js)
- [`cmd/spx/template/platform/web/lab/index.css`](../../../../cmd/spx/template/platform/web/lab/index.css)
