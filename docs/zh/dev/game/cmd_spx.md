# SPX 命令工具指南

本文档介绍了 SPX 命令行工具的使用方法和功能。SPX 命令工具是用于管理、开发和导出 SPX 项目的主要工具。

## 基本使用

```bash
spx <命令> [参数]
```

本文中的命令清单以 `cmd/spx/internal/command/args.go` 和命令执行逻辑为准。`runm` 和 `exportbot` 目前虽然会被命令行解析器接受，但没有实际执行逻辑，下面会单独标注为“未实现”。

## 命令分类

SPX 命令工具提供以下几类命令：

### 项目管理命令

| 命令 | 描述 |
| --- | --- |
| `help` | 显示帮助信息 |
| `version` | 显示版本信息 |
| `init` | 在当前目录或指定目录创建 SPX 项目 |
| `editor` | 在编辑器模式下打开当前项目 |
| `clear` | 删除生成的项目目录和临时生成文件（破坏性操作） |
| `clearbuild` | 删除 `.builds` 构建产物并保留项目文件 |

### 开发命令

| 命令 | 描述 |
| --- | --- |
| `build` | 构建动态库 |
| `buildtinygo` | 使用 TinyGo 为 ESP32 构建静态库 |
| `run` | 以解释器模式运行当前项目 |
| `runnative` | 以原生 PC 运行时运行当前项目 |
| `rune` | 在编辑器模式下运行当前项目 |
| `export` | 导出 PC 包（macOS、Windows、Linux） |
| `runm` | 未实现；当前不会启动多人模式 |

### Web 开发命令

| 命令 | 描述 |
| --- | --- |
| `buildweb` | 构建 WebAssembly (WASM) |
| `runweb` | 启动 Web 服务器运行项目 |
| `runwebworker` | 启动 Web Worker 版本 |
| `exportweb` | 导出 Web 包 |
| `exportwebworker` | 导出 Web Worker 包 |
| `exporttemplateweb` | 导出 Web 模板包 |
| `stopweb` | 停止 Web 服务器 |

### 移动端与机器人开发命令

| 命令 | 描述 |
| --- | --- |
| `exportbot` | 导出机器人包 |
| `exportapk` | 导出 Android APK |
| `exportios` | 导出 iOS 包 |
| `exportminigame` | 导出微信小游戏包 |
| `exportminiprogram` | 导出微信小程序包 |

`exportbot` 当前与 `runm` 一样只被解析器保留，尚未实现导出逻辑。

## 命令详细说明

### 项目管理命令

#### `help`

显示 SPX 命令的帮助信息，包括可用命令和参数的说明。

```bash
spx help
```

#### `version`

显示 SPX 命令工具的版本信息。

```bash
spx version
```

#### `init`

在当前目录或指定目录创建一个新的 SPX 项目。

```bash
# 在当前目录创建项目
spx init

# 在指定目录创建项目
spx init ./test/demo01
```

#### `editor`

在编辑器模式下打开当前项目，用于开发和调试。

```bash
spx editor
```

#### `clear`

清理生成内容。此命令会删除 `<path>/project` 整个生成项目目录，以及 `<path>/.temp`、`<path>/go.sum` 和 `<path>/xgo_autogen.go`；不会只删除构建产物。执行前请确认这些文件不包含需要保留的本地修改，并先做好备份。

```bash
spx clear
```

#### `clearbuild`

删除当前生成项目目录下的 `.builds` 构建产物，并保留项目文件及源代码。

```bash
spx clearbuild
```

### 开发命令

#### `build`

构建项目的动态库。

```bash
# 普通构建
spx build

# 服务器模式构建
spx build --servermode
```

#### `buildtinygo`

使用 TinyGo 为 ESP32 构建静态库。该命令主要用于嵌入式目标，不是普通桌面项目的构建步骤。

```bash
spx buildtinygo
```

#### `run`

以解释器模式运行当前项目。

```bash
# 运行当前目录的项目
spx run

# 运行指定路径的项目
spx run --path ./myproject
```

#### `runnative`

以原生 PC 运行时运行当前项目，适合查看桌面运行效果。

```bash
# 运行当前目录的项目
spx runnative

# 运行指定路径的项目
spx runnative --path ./myproject
```

#### `rune`

在编辑器模式下运行当前项目，适合开发和调试。

```bash
# 在编辑器模式下运行当前目录的项目
spx rune

# 在编辑器模式下运行指定路径的项目
spx rune --path ./myproject
```

#### `export`

导出 PC 平台的可执行包，支持 macOS、Windows 和 Linux。

```bash
# 导出当前项目
spx export
```

#### `runm`

当前命令解析器保留了 `runm` 名称，但没有多人模式执行逻辑，执行不会启动服务器或客户端。`--onlys`、`--onlyc` 和 `--serveraddr` 只是已注册的通用参数，不能使该命令获得多人模式功能。

```bash
# 运行多人模式
spx runm  # 当前未实现，不建议用于实际运行
```

### Web 开发命令

#### `buildweb`

构建项目的 WebAssembly (WASM) 版本，用于 Web 平台。

```bash
spx buildweb
```

#### `runweb`

启动 Web 服务器并运行项目的 Web 版本。

```bash
# 启动 Web 服务器
spx runweb

# 启动带调试服务的 Web 服务器
spx runweb --debugweb
```

#### `runwebworker`

构建并启动 Web Worker 版本。浏览器部署通常需要满足 `SharedArrayBuffer` 的跨源隔离要求，具体限制见[Web Worker 文档](../engine/web_worker_mode.md)。

```bash
spx runwebworker
```

#### `exportweb`

导出项目的 Web 包，可以部署到服务器。

```bash
spx exportweb
```

#### `stopweb`

停止正在运行的 Web 服务器。

```bash
spx stopweb
```

#### `exportwebworker`

导出 Web Worker 包。

```bash
spx exportwebworker
```

#### `exporttemplateweb`

导出 Web 模板包，用于检查或复用引擎模板。

```bash
spx exporttemplateweb
```

### 移动端与机器人开发命令

#### `exportbot`

当前命令解析器保留了 `exportbot` 名称，但没有机器人导出逻辑，执行不会生成机器人包。

```bash
spx exportbot  # 当前未实现，不建议使用
```

#### `exportapk`

导出项目的 Android APK 包，可以安装到 Android 设备上。

```bash
# 导出 APK
spx exportapk

# 导出 APK 并安装到连接的设备
spx exportapk --install
```

#### `exportios`

导出项目的 iOS 包，可以安装到 iOS 设备上（需要 macOS 和开发者证书）。

```bash
spx exportios
```

#### `exportminigame`

导出微信小游戏包。默认构建模式会压缩 wasm；使用 `-build=fast` 可跳过压缩以加快本地调试。

```bash
spx exportminigame
spx exportminigame -build=fast
```

#### `exportminiprogram`

导出微信小程序包。

```bash
spx exportminiprogram
```

## 通用参数

SPX 命令工具支持以下通用参数，可以与各种命令组合使用：

| 参数 | 描述 |
| --- | --- |
| `--path <路径>` | 指定项目路径，默认为当前目录 |
| `--serveraddr <地址>` | 指定服务器地址 |
| `--controller <名称>` | 指定控制器类型名称 |
| `--servermode` | 服务器模式 |
| `--headless` | 无界面模式，适用于服务器环境 |
| `--arch <架构>` | 指定 CPU 架构 |
| `--onlys` | 多人模式仅启动服务器（当前 `runm` 未实现） |
| `--onlyc` | 多人模式仅启动客户端（当前 `runm` 未实现） |
| `--tags <标签>` | 指定 Go 构建标签，默认值为空 |
| `--target <目标>` | 指定 TinyGo 目标板，默认 `esp32` |
| `--nomap` | 无地图模式 |
| `--install` | 导出后安装（当前主要用于 Android） |
| `--debugweb` | 开启 Web 调试服务 |
| `--fullscreen` | 全屏模式 |
| `--build <模式>` | 小游戏构建模式：`normal` 或 `fast`，默认 `normal` |
| `--mode <模式>` | Web 构建模式：`none`、`worker` 或 `minigame`，默认 `none` |
| `--movie` | 录制运行画面 |
| `-v` | 输出详细日志 |

## 使用示例

### 创建并运行一个新项目

```bash
# 创建新项目
spx init ./myproject

# 进入项目目录
cd ./myproject

# 在编辑器中运行项目
spx rune
```

### 导出并测试 Web 版本

```bash
# 导出 Web 版本
spx exportweb

# 运行 Web 服务器
spx runweb
```

### 导出 Android APK 并安装

```bash
# 导出 APK 并安装到连接的设备
spx exportapk --install
```

## 常见问题排查

### Web 服务器无法启动

确保已安装 Python，SPX 使用 Python 启动 Web 服务器。如果 `python` 命令不可用，SPX 会尝试使用 `python3`。

### Android 导出失败

确保已设置 `ANDROID_NDK_ROOT` 环境变量，并且已安装 Android SDK 和 NDK。

### iOS 导出失败

确保在 macOS 系统上操作，并且已安装 Xcode 和必要的开发证书。
