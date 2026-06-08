# Web 平台字体与 Scratch SVG 兼容说明

## 背景

Web 导出时，SVG 里的 `<text>` 节点不能假设浏览器一定能找到目标系统字体。  
Scratch 导出的 SVG 通常会使用一组固定的字体族名，例如 `Sans Serif`、`Marker`、`Scratch`。如果运行时没有把这些族名显式映射到项目内的字体文件，Web 平台就容易出现下面几类问题：

- 文本回退到浏览器默认字体
- 字体风格与 Scratch 原稿不一致
- 文本宽度变化，导致排版错位
- 不同平台渲染结果不一致

当前仓库已经落地了一套可用实现：保留 `CnFont.ttf` 作为项目默认字体，同时把 `engine/fonts/scratch` 目录里的 7 个 Scratch 兼容字体注册给 SVG 渲染层。

## 当前实现概览

当前方案分成两部分：

### 1. 默认 UI 字体

- 路径：`res://engine/fonts/CnFont.ttf`
- 作用：作为项目默认字体，用于普通 UI / 文本显示
- 大小：约 `8.1 MB`
- 模板配置位置：`cmd/spx/template/project/project.godot`

模板工程的默认配置如下：

```ini
[gui]
theme/custom_font="res://engine/fonts/CnFont.ttf"
```

### 2. Scratch SVG 专用字体

- 路径：`res://engine/fonts/scratch`
- 作用：为 Scratch 常见的 SVG `font-family` 提供稳定映射
- 总大小：约 `1.1 MB`
- 来源：[`scratch-render-fonts`](https://github.com/scratchfoundation/scratch-render-fonts)
- 许可证：`cmd/spx/template/project/engine/fonts/scratch/LICENSE.txt`、`cmd/spx/template/project/engine/fonts/scratch/OFL.txt`

这批字体文件位于模板目录 `cmd/spx/template/project/engine/fonts/scratch`，会跟随模板项目一起进入新工程。

## Scratch 字体映射

当前内置映射如下：

| Scratch `font-family` | 字体文件 | 体积 |
| --- | --- | --- |
| `Sans Serif` | `NotoSans-Medium.ttf` | 442 KB |
| `Serif` | `SourceSerifPro-Regular.otf` | 217 KB |
| `Handwriting` | `handlee-regular.ttf` | 38 KB |
| `Marker` | `Knewave.ttf` | 44 KB |
| `Curly` | `Griffy-Regular.ttf` | 204 KB |
| `Pixel` | `Grand9K-Pixel.ttf` | 22 KB |
| `Scratch` | `Scratch.ttf` | 36 KB |

对应关系来自 `cmd/spx/template/project/engine/fonts/scratch/README.md`，代码侧注册定义位于 `internal/core/project/display.go`。

## 运行时是怎么生效的

字体注册入口在游戏构建阶段：

1. `game_build.go` 的 `loadResources()` 会读取项目资源。
2. `internal/core/project.ResolveDisplaySettings()` 返回默认字体和 Scratch SVG 字体注册表。
3. `internal/core/project.RegisterDisplayFonts()` 执行两件事：
   - 调用 `SetDefaultFont()` 设置默认字体 `CnFont.ttf`
   - 逐个调用 `RegisterSvgFontFace()` 注册 Scratch 字体族
4. Web 和 Native 平台都会走各自的 `ResMgr` 绑定，把字体信息交给底层引擎。

核心注册表如下：

```go
var scratchSVGFontRegistrations = []SVGFontFaceRegistration{
	{Path: "res://engine/fonts/scratch/NotoSans-Medium.ttf", Family: "Sans Serif"},
	{Path: "res://engine/fonts/scratch/SourceSerifPro-Regular.otf", Family: "Serif"},
	{Path: "res://engine/fonts/scratch/handlee-regular.ttf", Family: "Handwriting"},
	{Path: "res://engine/fonts/scratch/Knewave.ttf", Family: "Marker"},
	{Path: "res://engine/fonts/scratch/Griffy-Regular.ttf", Family: "Curly"},
	{Path: "res://engine/fonts/scratch/Grand9K-Pixel.ttf", Family: "Pixel"},
	{Path: "res://engine/fonts/scratch/Scratch.ttf", Family: "Scratch"},
}
```

## 资源导入配置

`scratch` 目录中的字体都带有对应的 `.import` 文件，当前导入参数基本一致：

- `importer="font_data_dynamic"`
- `type="FontFile"`
- `compress=true`
- `allow_system_fallback=true`

这意味着：

- 字体资源按 Godot 的动态字体资源导入
- 导入后会进行压缩
- 字形缺失时允许底层尝试系统字体回退

不过需要注意，Web 平台的核心目标仍然是“不要依赖系统字体是否存在”，所以 Scratch 专用字体注册依然是必要的。

## 这个方案解决了什么

当前实现主要解决的是 Scratch SVG 字体兼容问题：

- 让 `Sans Serif`、`Marker`、`Scratch` 等族名在 Web 平台可用
- 让 Scratch 风格 SVG 在不同平台更接近原始效果
- 避免把一份大字体强行当成所有 Scratch 字体族来使用
- 把 Scratch 兼容字体和默认 UI 字体职责拆开

## 这个方案没有解决什么

当前实现并没有完全解决 Web 平台字体包体问题：

- 默认字体 `CnFont.ttf` 仍然约 `8.1 MB`
- Scratch 兼容字体只是额外补齐了 SVG 的固定字体族映射
- 如果目标是继续压缩整体下载体积，还需要单独推进中文字体子集化或按需加载方案

换句话说，这一版重点是“先保证显示正确”，不是“把所有字体体积都降到最小”。

## 使用和排查时的注意事项

### 1. 字体族名必须精确匹配

这里注册的是 Scratch 约定名称，不是 CSS 通用族名。

- `Sans Serif` 是内置映射名
- `sans-serif` 是浏览器通用字体族

这两个名字不是一回事，大小写和空格也不能随意改。

### 2. 这批字体主要服务 Scratch 风格 SVG

`CnFont.ttf` 仍然是项目默认字体；`scratch` 目录里的字体主要用于匹配 Scratch 导出的 SVG `font-family`，不是用来替代整个项目的中文默认字体。

### 3. 新增字体时不能只拷贝文件

如果后续需要增加新的 SVG 字体族，至少要同步更新下面几处：

- `cmd/spx/template/project/engine/fonts/scratch`
- `cmd/spx/template/project/engine/fonts/scratch/README.md`
- `internal/core/project/display.go`
- `internal/core/project/project_test.go`
- `test/ScratchFonts/assets/backdrop.svg`

## 维护建议

如果要调整这套 Scratch 字体方案，建议按下面顺序操作：

1. 在 `cmd/spx/template/project/engine/fonts/scratch` 中加入或替换字体文件。
2. 确保对应 `.import` 文件存在，并保持和现有资源一致的导入参数。
3. 更新 `internal/core/project/display.go` 中的 `scratchSVGFontRegistrations`。
4. 更新 `cmd/spx/template/project/engine/fonts/scratch/README.md` 的映射说明。
5. 更新 `internal/core/project/project_test.go` 里的断言。
6. 用 `test/ScratchFonts` 示例工程做一次目检。
7. 最后再更新本文档。

## 验证方式

仓库内已经有一个专门的示例工程：

- 示例目录：`test/ScratchFonts`
- 示例 SVG：`test/ScratchFonts/assets/backdrop.svg`

这个示例会把 7 组 Scratch 字体族都画出来，适合在改动字体资源或映射关系后做快速回归。

示例里的关键写法如下：

```svg
<text x="120" y="2" font-family="'Marker'" font-size="26">
  The quick brown fox 012345
</text>
```

如果某一组字体注册失效，最直观的现象通常就是该行文字样式明显变成浏览器默认字体。

## 小结

当前 Web 字体方案的核心思路很简单：

- `CnFont.ttf` 负责项目默认文本显示
- `scratch` 目录负责补齐 Scratch SVG 固定字体族
- 运行时统一由 `RegisterDisplayFonts()` 完成注册

这套实现已经足够支撑 Scratch 风格 SVG 在 Web 平台的基础兼容。后续如果继续做体积优化，可以在此基础上再推进中文字体裁剪、分级加载或远程字体分发。
