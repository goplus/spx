# Web 平台项目字体与 SVG cluster fallback

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
  "fontPreferences": "Pixel, \"Basic Chinese\", default"
}
```

`default` 是唯一的内置 family 名称，对应 `res://engine/fonts/CnFont.ttf`。它是保留名，不能在 `assets/fonts/default` 中重新声明。未配置 `fontPreferences` 时等价于只使用 `default`。

## 加载与校验

加载规则如下：

1. `index_pack.json` 包含 `fonts` 字段时，以 packed catalog 为准，不扫描目录。
2. 没有 `index_pack.json` 或 packed index 缺少 `fonts` 字段时，枚举 `assets/fonts/` 的直接子目录。
3. Family Name 按 ASCII 大小写折叠后必须唯一，不能包含逗号、引号、路径分隔符、控制字符或首尾空白。
4. Face 路径必须是 Family 目录内的相对 POSIX 路径，不能为空，也不能通过 `..` 逃逸。
5. `fontPreferences` 中未知的 Family 会被忽略，剩余 Family 按声明顺序组成回退链。

项目字体与声音资源使用相同的内容路径和打包流程。内部字体随项目资源打包，配置引用的外部字体也会被 pack 收集。

## 运行时流程

字体注册发生在游戏资源构建阶段：

1. `game_build.go` 打开项目资源。
2. `internal/core/project.LoadProjectFonts()` 从 packed catalog 或 `assets/fonts/` 加载 Font Collection。
3. `ResolveDisplaySettings()` 设置窗口参数和 `default` 字体，不注入任何兼容 family。
4. `AddProjectFonts()` 加入项目声明的 Family 和 preference。
5. `RegisterDisplayFonts()` 设置默认字体、注册项目 Face，并把 preference 同时交给普通 Godot 文本和 LunaSVG。

Godot 侧只在 `SpxResMgr` 中用 `SPX_API` 声明接口；Go、Native、Web bridge 都通过 `make generate` 生成，不手工维护生成文件。

## SVG family 与 cluster fallback

SVG 显式声明的 `font-family` 优先于项目全局 `fontPreferences`。显式列表里的未知名称会被跳过；如果没有可用候选，不会继续使用全局列表或系统字体。没有显式 family 时才使用项目全局顺序。

fallback 按 extended grapheme cluster 选择字体：

1. Godot TextServer 返回 UTF-32 character-break 边界。
2. LunaSVG 保留 combining mark、variation selector、emoji modifier、regional indicator 和 ZWJ 序列的完整 cluster。
3. 对每个 cluster 按 Family 顺序选择第一个覆盖完整 cluster 的 Face。
4. 同一个 cluster 不会被拆给多个字体；所有候选都不支持时只绘制一个 missing-glyph box。
5. cluster 内的 SVG `x/y/dx/dy/rotate` 使用 cluster 起点，避免定位属性再次拆分 cluster。

彩色字体不依赖特殊 Family 名称。选中的 Face 如果为 shaped glyph 提供 OpenType-SVG 数据，LunaSVG 会直接栅格化该内嵌 SVG；例如项目 Family `Heart Emoji` 不需要改名为 `Emoji`。

每个候选 Face 都会先用 Godot 构建中已有的 HarfBuzz 对完整 cluster 做 GSUB/GPOS shaping，再检查结果里是否存在 `.notdef`。因此 `e + combining acute` 可以组合成字体已有的 `é` glyph，`👩‍💻` 等 ZWJ 序列也可以命中字体的连字 glyph；fallback 选择、advance 计算和绘制始终复用同一组 glyph ID 与 position。

## reset 行为

SPX reset 会把字体视为项目级状态并完整清理：

- 释放普通文本的 FontFile 回退链并恢复 ThemeDB 默认主题。
- 清空 SVG default face、named families 和 preference。
- 递增字体 registry generation，让每个 LunaSVG 渲染线程在下次使用时清掉旧 face cache。
- 清除已经栅格化的 SVG 图片、尺寸、动画和动画判定缓存。

因此 reset 后不会解析到上一个项目的 Family，也不会复用旧字体生成的 SVG 像素。

## 默认字体

`res://engine/fonts/CnFont.ttf` 仍用于项目默认 UI 和 `default` family。模板仓库只跟踪对应 `.import` 文件；字体本体由安装流程下载，或由 `cmd/spx/setup_font.sh` 从本机字体生成。

运行时创建的 FontFile 会设置 `allow_system_fallback=false`，所以项目最终显示不依赖本机字体环境。

## 示例

`test/ProjectFonts` 是完整的项目字体示例：

- 每个 Scratch family 都位于自己的 `assets/fonts/<Family>` 目录。
- `Heart Emoji` 作为项目 Family 携带完整的 Twitter Color Emoji 字体，提供 `❤️` 等 emoji 字形。
- SVG 显式 family 只解析项目声明，不依赖 runtime 内置映射。
- `Pixel, Heart Emoji, default` 展示多级全局回退。
- emoji、中文、combining mark、variation selector 和 ZWJ 序列覆盖 cluster fallback。
- 不提供 `index_pack.json`，覆盖 source 模式目录扫描。

运行：

```bash
spx rune --path test/ProjectFonts
```

## 当前限制

- 每个 Family 暂时只支持一个 Face，尚未按 weight/style 选择多个 Face。
- 默认 `CnFont.ttf` 体积仍然较大，中文子集化或按需加载需要单独推进。
- emoji 必须由项目显式携带；运行时不再提供隐藏的 emoji family。
