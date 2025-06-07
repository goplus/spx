# SPX 命令工具指南

本文档介绍了 SPX 命令行工具的使用方法和功能。SPX 命令工具是用于管理、开发和导出 SPX 项目的主要工具。

## 基本使用

```bash
spx <命令> [参数]
```

## 命令分类

SPX 命令工具提供以下几类命令：

### 项目管理命令

| 命令 | 描述 |
| --- | --- |
| `help` | 显示帮助信息 |
| `version` | 显示版本信息 |
| `init` | 在当前目录或指定目录创建 SPX 项目 |
| `editor` | 在编辑器模式下打开当前项目 |
| `clear` | 清理项目 |
| `clearbuild` | 清理构建产物 |

### 开发命令

| 命令 | 描述 |
| --- | --- |
| `build` | 构建动态库 |
| `run` | 运行当前项目 |
| `rune` | 在编辑器模式下运行当前项目 |
| `export` | 导出 PC 包（macOS、Windows、Linux） |
| `runm` | 在多人模式下运行项目 |

### Web 开发命令

| 命令 | 描述 |
| --- | --- |
| `buildweb` | 构建 WebAssembly (WASM) |
| `runweb` | 启动 Web 服务器运行项目 |
| `exportweb` | 导出 Web 包 |
| `stopweb` | 停止 Web 服务器 |
| `runwebeditor` | 在 Web 编辑器模式下运行项目 |
| `exportwebeditor` | 导出 Web 编辑器包 |
| `exportwebruntime` | 导出 Web 运行时包 |

### 移动端与机器人开发命令

| 命令 | 描述 |
| --- | --- |
| `exportbot` | 导出机器人包 |
| `exportapk` | 导出 Android APK |
| `exportios` | 导出 iOS 包 |

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

清理项目，删除临时文件和构建产物。

```bash
spx clear
```

#### `clearbuild`

清理构建产物，但保留项目文件。

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

#### `run`

运行当前项目。这个命令会在运行时模式下启动项目，适合查看最终效果。

```bash
# 运行当前目录的项目
spx run

# 运行指定路径的项目
spx run --path ./myproject
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

在多人模式下运行项目，支持联机功能。

```bash
# 运行多人模式
spx runm

# 仅启动服务器
spx runm --onlys

# 仅启动客户端
spx runm --onlyc

# 指定服务器地址
spx runm --serveraddr 127.0.0.1:8080
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

#### `runwebeditor`

在 Web 编辑器模式下运行项目，支持在浏览器中进行开发。

```bash
spx runwebeditor
```

#### `exportwebeditor`

导出项目的 Web 编辑器包。

```bash
spx exportwebeditor
```

#### `exportwebruntime`

导出项目的 Web 运行时包。

```bash
spx exportwebruntime
```

### 移动端与机器人开发命令

#### `exportbot`

导出机器人包，用于自动化和机器人应用。

```bash
spx exportbot
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

## 通用参数

SPX 命令工具支持以下通用参数，可以与各种命令组合使用：

| 参数 | 描述 |
| --- | --- |
| `--path <路径>` | 指定项目路径，默认为当前目录 |
| `--serveraddr <地址>` | 指定服务器地址，用于网络功能 |
| `--headless` | 无界面模式，适用于服务器环境 |
| `--arch <架构>` | 指定 CPU 架构 |
| `--tags <标签>` | 指定构建标签，默认为 simulation |
| `--fullscreen` | 全屏模式 |

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