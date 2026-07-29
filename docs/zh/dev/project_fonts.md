# 项目字体与 SVG cluster fallback

## 设计目标

Web 运行时不能依赖浏览器或操作系统恰好安装了某种字体。SPX 因此把非默认字体定义为项目资源：项目声明自己拥有的 Font Family、Face 和全局回退顺序，Native 与 Web 使用同一份配置。

运行时不再内置 `Sans Serif`、`Marker`、`Scratch`、`Emoji`、`SPX Default` 等兼容 family。SVG 使用了这些名称时，项目必须在自己的 Font Collection 中声明；未声明的名称不会回退到旧映射或系统字体。

## Font Collection

项目字体放在 `assets/fonts/` 下。每个直接子目录表示一个 Font Family，目录名就是 Family Name；当前每个 Family 只允许一个 Face：

```text
assets/fonts/
  Basic Chinese/
    index.json
    basic.ttf
  Pixel/
    index.json
    pixel.ttf
```

Family 的 `index.json` 格式如下：

```json
{
  "faces": [
    { "path": "basic.ttf" }
  ]
}
```

全局回退顺序由 `assets/index.json` 中可选的 `fontPreferences` 指定：

```json
{
  "fontPreferences": ["Pixel", "Basic Chinese", "default"]
}
```

`default` 是唯一的内置 family 名称，对应 SPX 模板中的拉丁字体 `res://engine/fonts/default.ttf`。它是保留名，不能在 `assets/fonts/default` 中重新声明，也不保证中文、Emoji 或 Scratch 字形。未配置 `fontPreferences` 时等价于只使用 `default`。

`fontPreferences` 的输入语义由上层 builder 配置决定，SPX 只消费配置，不负责把这份 JSON 再序列化：

- 字段缺失或为 `null`：使用默认值 `["default"]`。
- 显式 `[]`：不使用任何全局字体；不会自动补回 `default`。
- 显式 `["default"]`：只使用模板默认字体。

## 加载与校验

加载规则如下：

1. `index_pack.json` 包含 `fonts` 字段时，以 packed catalog 为准，不扫描目录。
2. 没有 `index_pack.json` 或 packed index 缺少 `fonts` 字段时，枚举 `assets/fonts/` 的直接子目录。
3. Family Name 按 ASCII 大小写折叠后必须唯一，不能为 `default`（任意 ASCII 大小写），且必须是一个目录路径段。
4. Face 路径必须是 Family 目录内的相对 POSIX 路径，不能为空，也不能通过 `..` 逃逸。
5. `fontPreferences` 的每项必须是一个非空且可用的 Family 名称；名称按 ASCII 大小写折叠后不能重复。

项目字体与声音资源使用相同的内容路径和打包流程。内部字体随项目资源打包，配置引用的外部字体也会被 pack 收集。

## 运行时流程

字体注册发生在游戏资源构建阶段：

1. `game_build.go` 打开项目资源。
2. `internal/core/project.LoadProjectFonts()` 从 packed catalog 或 `assets/fonts/` 加载 Font Collection。
3. `ResolveDisplaySettings()` 和 `AddProjectFonts()` 生成完整的字体计划，不注入任何兼容 family。
4. SPX 通过一次 `ApplyProjectFonts()` 调用把默认字体、所有项目 Face 和 preference 交给 Godot。
5. Godot 先校验数组、字体数据、Family 和 preference；全部成功后才同时发布普通文本 fallback chain、ThemeDB 默认字体和 LunaSVG registry。任一步失败都保留上一份完整配置。

Godot 侧只在 `SpxResMgr` 中用 `SPX_API` 声明接口；Go、Native、Web bridge 都通过 `make generate` 生成，不手工维护生成文件。

Native bridge 会区分 nil slice 与显式空 slice，并在旧 Godot 模板缺少原子字体接口时返回明确错误，不会调用空函数指针。Web bridge 使用同一份生成接口。

## SVG family 与 cluster fallback

SVG 显式声明的 `font-family` 优先于项目全局 `fontPreferences`。只会使用有效的自定义 CSS family；generic family 和 system-font keyword 不属于项目 family。显式列表里的未知名称会被跳过；如果没有可用候选，不会继续使用全局列表或系统字体。没有显式 family 时才使用项目全局顺序。若项目 family 与 CSS keyword 冲突（包括 `default`），SVG 中必须引用为带引号的字符串。

fallback 按 extended grapheme cluster 选择字体，但 shaping 不按 cluster 拆开：

1. Godot TextServer 返回 UTF-32 character-break 边界。
2. LunaSVG 以宿主返回的 EGC 边界为主，并用本地 UAX #29 子集守住最低原子性；宿主缺少完整 Unicode break iterator 时也不会拆开 combining mark、variation selector、emoji modifier、regional indicator 或 ZWJ 序列。
3. 文本先按 bidi visual run、script、language、style 和 SVG positioning 边界切分 shaping run，并显式把 direction、script、`xml:lang`/`lang` 传给 HarfBuzz。
4. 每个候选 Face 先对完整 shaping run 执行 HarfBuzz；根据返回的 cluster map，把含 `.notdef` 的 EGC 标为该字体不支持。
5. 对每个 EGC 按 Family 顺序选择第一个完整支持它的 Face，再把连续使用同一 Face 的 EGC 合并并重新 shape，从而保留 joining、ligature、kerning 和 GSUB/GPOS 上下文。
6. 同一个 EGC 不会被拆给多个字体；所有候选都不支持时只绘制一个 missing-glyph box。glyph cluster 始终映射回原始 UTF-32 offset。
7. cluster 内的 SVG `x/y/dx/dy/rotate` 使用 cluster 起点，避免定位属性再次拆分 cluster。

彩色字体不依赖特殊 Family 名称。选中的 Face 如果为 shaped glyph 提供 OpenType-SVG 数据，LunaSVG 会直接栅格化该内嵌 SVG；例如项目 Family `Color Emoji` 不需要改名为 `Emoji`。

因此 `e + combining acute` 可以组合成字体已有的 `é` glyph，`👩‍💻` 等 ZWJ 序列可以命中连字 glyph；阿拉伯 joining、Indic/东南亚上下文形态、希伯来文方向、拉丁 kerning 和跨 EGC substitution 不会因为 fallback 的原子单位而被逐 cluster shaping 破坏。

LunaSVG registry 保存一份不可变字体数据快照。各渲染线程按 generation 安装完整快照；某个 Face 注册失败时不会把线程推进到半完成 generation，下次渲染会重试。registry 切换会同时清理 HarfBuzz face cache 和已栅格化的 OpenType-SVG glyph cache。

## reset 行为

SPX reset 会把字体视为项目级状态并完整清理：

- 释放普通文本的 FontFile 回退链并恢复 ThemeDB 默认主题。
- 清空 SVG default face、named families 和 preference。
- 递增字体 registry generation，让每个 LunaSVG 渲染线程在下次使用时清掉旧 face cache。
- 清除已经栅格化的 SVG 图片、尺寸、动画和动画判定缓存。

因此 reset 后不会解析到上一个项目的 Family，也不会复用旧字体生成的 SVG 像素。

## 默认字体

`default` family 和项目默认 UI 使用 SPX 模板自带的 `default.ttf`，不再携带完整 `CnFont.ttf`。`default.ttf` 是从 Source Han Sans CN Medium 1.000 的 `CnFont.ttf` 裁出的拉丁向子集，保留拉丁字母、数字、常用标点和必要控制/替代字符。字体与随附的 `default.LICENSE.txt` 均使用上游 1.000 版本的 Apache License 2.0。

中文字体必须像其他项目字体一样放入 Font Collection。例如 `test/ProjectFonts` 的 `basic-chinese.ttf` 也来自同一份 `CnFont.ttf`，保留汉字、CJK 部首/笔画、中文标点、全角标点和兼容字形，排除拉丁字形。它在 `fontPreferences` 中显式声明为 `basic-chinese`，使中文覆盖不再是 `default` 的隐式能力。

运行时创建的 FontFile 会设置 `allow_system_fallback=false`，所以项目最终显示不依赖本机字体环境。

## 示例

`test/ProjectFonts` 是完整的项目字体示例：

- 每个 Scratch family 和汉字子集 `basic-chinese` 都位于自己的 `assets/fonts/<Family>` 目录。
- `Color Emoji` 作为项目 Family 携带完整的 Twitter Color Emoji 字体，提供 `❤️` 等 emoji 字形。
- SVG 显式 family 只解析项目声明，不依赖 runtime 内置映射。
- `Pixel, Color Emoji, Arabic, basic-chinese, default` 展示多级全局回退。
- emoji、中文、combining mark、variation selector 和 ZWJ 序列覆盖 cluster fallback。
- 混排文本以逻辑字符序列 `SPX 123 | \u0633\u0644\u0627\u0645 | 456 END` 保存；渲染后 `456` visual run 位于连接后的阿拉伯文 run 之前，覆盖 ICU Unicode BiDi、阿拉伯连接形和跨 Family fallback。
- 不提供 `index_pack.json`，覆盖 source 模式目录扫描。

运行：

```bash
spx rune --path test/ProjectFonts
```

## 当前限制

- 每个 Family 暂时只支持一个 Face，尚未按 weight/style 选择多个 Face。
- `basic-chinese` 保留较完整的汉字覆盖，因此仍会显著增加项目体积。
- emoji 必须由项目显式携带；运行时不再提供隐藏的 emoji family。
