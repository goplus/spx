# Web 端截图与固定帧接入说明

本文档说明如何在 SPX 中声明固定帧截图，并在 Web 页面中保存、比较或上报截图。仓库内的测试模板将页面结构、行为和样式分别放在 [`index.html`](../../../../cmd/spx/template/platform/web/index.html)、[`lab/index.js`](../../../../cmd/spx/template/platform/web/lab/index.js) 和 [`lab/index.css`](../../../../cmd/spx/template/platform/web/lab/index.css)。

适用场景：

- 在指定 engine frame 触发回归截图
- 在自定义页面中接管截图保存或上传
- 维护 baseline，并与后续 run 做相似度比较

## 1. SPX 侧能力

运行时提供三个公开入口：

- `spx.CurrentFrame()`：返回当前 engine session 的绝对帧号
- `spx.AtFrame(frame, fn)`：在绝对帧 `frame` 执行一次回调，`frame` 类型为 `int64`
- `spx.Snapshot(name, fn)`：先执行 `fn`，成功后在当前 engine frame 末尾请求截图

最小示例：

```go
base := spx.CurrentFrame()
spx.AtFrame(base+60, func() {
	spx.Snapshot("title", nil)
})
```

`Snapshot` 的 `fn` 是 XGo decorator 展开后传入的函数 body，也可以在直接调用时传入 `nil`。如果 body 返回错误，运行时会报告错误且不会请求截图。

截图请求在活跃 Game 中排到本帧末尾；没有活跃 Game 时立即交给平台 handler。脚本只声明截图点位，图片保存为 baseline、run，还是直接上传，由平台和上层页面决定。

## 2. 固定帧语义

`AtFrame` 使用 engine session 的绝对帧，而不是相对 Game `OnStart` 或 bootstrap 的帧数。异步资源加载也会推进帧号，因此通常应先读取 `CurrentFrame()` 再计算目标帧。

具体语义如下：

- engine `onStart` 创建新的 time session，并把帧号置为 `0`
- 每次 engine update 推进一次帧号，包括加载和 bootstrap 阶段
- Game reload 不重置帧号，但会清除未执行的回调和截图请求
- 注册时已经到期的目标会立即进入可调度状态
- 回调运行在注册对象所属的 SPX coroutine 中，可以安全使用会 yield 的 wait/animate API
- 同一目标帧按注册顺序启动；yield 后回到普通 coroutine 调度
- coroutine body 在常规 update-side 同步之后修改视觉状态时，运行时会在截图前补做一次视觉同步

例如：

```go
base := spx.CurrentFrame()
spx.AtFrame(base+30, func() {
	spx.Snapshot("after_move", func() error {
		// 在这里修改可见状态；成功返回后才请求截图。
		return nil
	})
})
```

### XGo decorator 形式

显式调用已经可以直接使用。XGo decorator 支持相应展开后，也可以写成：

```go
@atFrame(60)
@snapshot("title")
func SnapshotTitle() {
}
```

decorator 的 frame 仍是绝对 engine-session frame；小目标可能在加载期间已经到期。

## 3. Web capture bridge

[`capture.js`](../../../../cmd/ispx/web/capture.js) 负责截图请求的归一化、采图排队和 host 投递；[`runner.html`](../../../../cmd/ispx/web/runner.html) 负责连接 Canvas 与公开接口。

上层页面通常只需要三个接口：

- `runnerWindow.spxSetCaptureHost(host)`
- `runnerWindow.spxGetCaptureHost()`
- `await runnerWindow.spxFlushCaptureQueue()`

`host` 可以是函数：

```js
async function (request, blob) {}
```

也可以是对象：

```js
{
  async handleCapture(request, blob) {},
  handleCaptureFailure(request, error) {}
}
```

没有安装 host 时，runner 会回退为浏览器下载 PNG。请求按 sequence 串行交付；同一 engine frame 的多个请求共享一份经过 render fence 的 Canvas 截图。

停止 Game、替换 host 或提交 baseline 前，应等待 `spxFlushCaptureQueue()`。官方 runner 的 `stopGame()` 已在 reset 前后执行 flush。

## 4. Request 与文件名

传给 Web host 的 request 结构如下：

```js
{
  name: "title",
  filename: "frame_0062_0001_title.png",
  frame: 62,
  sequence: 1
}
```

字段含义：

- `name`：`Snapshot` 传入的可选名字
- `filename`：runner 生成并清理过的稳定 PNG 文件名
- `frame`：请求对应的 engine-session frame
- `sequence`：当前 engine session 内的截图序号

命名规则：

```text
frame_<frame>_<sequence>[_<name>].png
```

数字至少补齐四位。空名字不会生成占位后缀，例如 `frame_0062_0001.png`；显式名字生成 `frame_0062_0001_title.png`。文件名中的非法字符会替换为 `_`。

## 5. 最小页面接入

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
        const url = URL.createObjectURL(blob)
        const anchor = document.createElement('a')
        anchor.href = url
        anchor.download = request.filename
        document.body.appendChild(anchor)
        anchor.click()
        anchor.remove()
        URL.revokeObjectURL(url)
      },
    })
  }

  installCaptureHost()
</script>
```

自定义 host 可以直接使用 `request.filename`，也可以根据 `frame`、`sequence` 和 `name` 自行组织存储路径。

## 6. 测试模板的 baseline / runs

模板页提供以下 capture 控件：

- `Start` / `Stop`：统一的 Game 生命周期
- `Choose Folder`：选择浏览器文件系统目录
- `Refresh`：重新读取 baseline 和最新 run
- `Snapshot Comparison`：并排显示图片并计算相似度

启动项目时，页面从 zip 中读取 `builder-meta.json` 与 `assets/index.json`，并用项目名创建：

```text
spx-snapshots/<project>/
```

如果 `assets/index.json` 的 `run.deterministic` 为 `true`，或 `run.fixedTimestep` 大于 `0`，第一组截图写入 `baseline/`，后续启动写入 `runs/<timestamp>/`。普通项目直接写入新的 run 目录。

比较流程先对缩略灰度图做快速比较，接近一致时再用较大 RGBA 图细化分数。字体、GPU 和浏览器渲染可能造成少量像素差异，回归判断应使用合理阈值。

## 7. Capture 事件

runner 在成功或失败后派发 `spxCaptureScreenshot`：

```js
window.addEventListener('spxCaptureScreenshot', (event) => {
  console.log(event.detail)
})
```

事件包含：

- `ok`
- `name`
- `filename`
- `frame`
- `sequence`
- 成功时的 `destination`、`size`、`type`
- 失败时的 `error`

## 8. 注意事项

- File System Access API 不可用时，runner 仍可回退到浏览器下载
- `Snapshot` 只声明点位，不决定 baseline/run 分类或比较策略
- 切换项目、停止 Game 或替换 host 前应先 flush capture queue
- 网络、文件 I/O、字体与 GPU 渲染不会被固定帧机制虚拟化
- 项目若依赖 Godot fixed physics delta，仅固定 SPX engine frame 并不能保证像素完全一致

## 9. 参考文件

- [`runtime_frame.go`](../../../../runtime_frame.go)
- [`runtime_snapshot.go`](../../../../runtime_snapshot.go)
- [`runtime_capture_js.go`](../../../../runtime_capture_js.go)
- [`internal/engine/frame.go`](../../../../internal/engine/frame.go)
- [`cmd/ispx/web/capture.js`](../../../../cmd/ispx/web/capture.js)
- [`cmd/ispx/web/runner.html`](../../../../cmd/ispx/web/runner.html)
- [`cmd/spx/template/platform/web/index.html`](../../../../cmd/spx/template/platform/web/index.html)
- [`cmd/spx/template/platform/web/lab/index.js`](../../../../cmd/spx/template/platform/web/lab/index.js)
- [`cmd/spx/template/platform/web/lab/index.css`](../../../../cmd/spx/template/platform/web/lab/index.css)
