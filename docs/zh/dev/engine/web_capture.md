# Web 端截图与固定帧接入说明

本文档说明外部页面如何像 [`cmd/spx/template/platform/web/index.html`](../../../../cmd/spx/template/platform/web/index.html) 一样，接入 SPX 的固定帧截图能力。

适用场景：

- 在 Web 端做 deterministic demo 回归截图
- 在自定义 `index.html` 中接管截图保存、对比、上报
- 在某一帧触发截图，而不是手动点击按钮截图

## 1. SPX 侧能力

SPX 运行时对外暴露了这几个入口，定义在 [`runtime_frame.go`](../../../../runtime_frame.go)：

- `spx.Frame(i, fn)`：在第 `i` 帧执行一次回调
- `spx.Capture(name, fn)`：在当前帧末尾请求一次截图
- `spx.CaptureForCheck(name, fn)`：在当前帧末尾请求一次“用于对比”的截图
- `spx.SetCaptureHandler(handler)`：安装截图后端，通常由平台桥接层处理

一个最小例子：

```go
spx.Frame(60, func() {
	spx.Capture("frame_060", nil)
})

spx.Frame(120, func() {
	spx.CaptureForCheck("frame_120", nil)
})
```

`Capture` / `CaptureForCheck` 的 `fn` 会先执行，成功后才排入截图请求。这样可以把“操作”和“截图”写在同一个步骤里。

### 后续装饰器简化

当前这套能力已经可以直接使用，但写法还是显式调用 `spx.Frame(...)` / `spx.Capture(...)` / `spx.CaptureForCheck(...)`。

后续如果 xgo 的装饰器能力可用（见 `goplus/xgo#2797`），那么同样的逻辑可以进一步简化成直接写在 XGo/SPX 源码里的装饰器风格。

也就是说，上面的当前写法：

```go
spx.Frame(60, func() {
	spx.Capture("frame_060", nil)
})

spx.Frame(120, func() {
	spx.CaptureForCheck("frame_120", nil)
})
```

后续原则上可以写成类似下面的声明式形式：

```go
@frame(60)
@capture("frame_060")
func CaptureBaseline() {
}

@frame(120)
@captureForCheck("frame_120")
func CaptureCheck() {
}
```

这样测试脚本会更接近 issue #1652 想要的“在引擎内部声明式地按帧触发截图”的目标。

需要注意：

- 目前仓库中稳定可用的是显式函数调用方式，文档后续示例只是说明未来可简化的方向
- 最终装饰器名字和展开细节，仍以 xgo 对装饰器能力的实际支持为准

## 2. Web 端桥接点

Web 运行时会把截图请求桥接到 [`cmd/ispx/web/runner.html`](../../../../cmd/ispx/web/runner.html)。

外部页面真正需要关心的接口只有两个：

- `runnerWindow.spxSetCaptureHost(host)`
- `runnerWindow.spxGetCaptureHost()`

其中 `host` 可以是：

- 一个函数：`async function (request, blob) {}`
- 一个对象：`{ async handleCapture(request, blob) {} }`

如果没有安装 host，Web 端会退化为浏览器下载 PNG。

## 3. CaptureRequest 格式

传给 host 的 `request` 结构如下：

```js
{
  name: "frame_060",
  filename: "frame_060.png",
  intent: "snapshot", // 或 "check"
  check: false,       // intent === "check" 时为 true
  frame: 60,
  sequence: 1
}
```

字段含义：

- `name`：SPX 脚本传入的名字
- `filename`：runner 归一化后的文件名，保证是 `.png`
- `intent`：`snapshot` 或 `check`
- `check`：`intent === "check"` 的布尔别名
- `frame`：截图请求对应的逻辑帧号
- `sequence`：当前运行期内的截图序号

## 4. 最小接入方式

如果你有自己的 `index.html`，最简单的接法如下：

```html
<iframe id="runnerFrame" src="runner.html"></iframe>
<script>
  const runnerWindow = document.getElementById('runnerFrame').contentWindow

  async function installCaptureHost() {
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
</script>
```

这个例子只是把 runner 默认下载逻辑搬到了外部页面。真正有用的做法通常是：

- 自己控制保存目录
- 按 `request.frame` 和 `request.intent` 命名
- 把 baseline 和 runs 分开
- 在 host 中直接做相似度对比

## 5. 像模板页那样保存 baseline / runs

仓库里的模板页在 [`cmd/spx/template/platform/web/index.html`](../../../../cmd/spx/template/platform/web/index.html) 中做了一个完整参考实现，核心思路是：

1. 页面启动项目时，从 zip 中读取：
   - `builder-meta.json`
   - `assets/index.json`
2. 用 `displayName` 或项目名作为目录名
3. 如果项目是 deterministic 运行：
   - 第一次保存到 `spx-snapshots/<project>/baseline/`
   - 后续保存到 `spx-snapshots/<project>/runs/<timestamp>/`
4. 如果不是 deterministic：
   - 直接保存到 `runs/<timestamp>/`

模板页判断 deterministic 的依据是：

- `assets/index.json` 中 `run.deterministic == true`
- 或 `run.fixedTimestep > 0`

## 6. 外部页面推荐实现步骤

如果你想复制模板页思路，建议按下面的顺序实现：

1. 启动项目前先拿到项目元信息
2. 选择一个截图根目录
3. 调用 `runnerWindow.spxSetCaptureHost(...)`
4. 在 host 里按 `request.frame`、`request.intent`、`request.name` 生成文件名
5. deterministic 项目先写 baseline，再写 runs
6. 如果要做回归测试，再把 baseline 与当前 run 做对比

一个更实用的命名方式：

```text
frame_0060_snapshot_title.png
frame_0120_check_enemy_wave.png
```

## 7. 模板页额外提供了什么

当前模板页除了保存截图，还额外做了这些事情：

- `CaptureDir` 按钮：选择浏览器文件系统目录
- `RefreshCompare` 按钮：重新读取 baseline / runs
- `Capture Compare` 面板：并排展示 baseline 与当前 run
- 相似度计算：先缩略灰度快速比较，再按需做更细的 RGBA 比较

如果你只是要“接入能力”，这些都不是必须的；真正必须的只有：

- 在 SPX 里调用 `Frame` / `Capture` / `CaptureForCheck`
- 在外部页面安装 `spxSetCaptureHost`

## 8. 与 runner 的事件协作

`runner.html` 在截图完成或失败时还会派发 `spxCaptureScreenshot` 事件。你如果不想直接通过 host 做逻辑，也可以监听这个事件：

```js
window.addEventListener('spxCaptureScreenshot', (event) => {
  console.log(event.detail)
})
```

事件里会包含：

- `ok`
- `name`
- `filename`
- `intent`
- `check`
- `frame`
- `sequence`
- `destination`
- `size`
- `type`
- 失败时的 `error`

## 9. 注意事项

- 当前模板依赖浏览器文件系统目录选择能力；如果浏览器不支持，就只能退回下载模式
- `CaptureForCheck` 只是表达“这张图是拿来比对的”，真正怎么比对由外部 host 决定
- 当前 deterministic 语义主要统一的是 SPX logical update delta，不包含 Godot fixed physics delta
- 如果项目逻辑依赖物理 fixed-update，截图一致性仍可能受影响

## 10. 参考文件

- [`runtime_frame.go`](../../../../runtime_frame.go)
- [`runtime_frame_js.go`](../../../../runtime_frame_js.go)
- [`cmd/ispx/web/runner.html`](../../../../cmd/ispx/web/runner.html)
- [`cmd/spx/template/platform/web/index.html`](../../../../cmd/spx/template/platform/web/index.html)
