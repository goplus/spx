# SPX Makefile 命令指南

本文档介绍了 SPX 项目中 Makefile 提供的各种命令及其功能。使用这些命令可以简化开发、构建和运行过程。

## 快速开始

```bash
# 查看所有可用命令
make help

# 初始化用户环境
make setup

# 初始化开发环境
make setup-dev
```

## 命令概览

### 设置命令

| 命令 | 别名 | 描述 |
| --- | --- | --- |
| `make setup` | `make init` | 初始化用户环境 |
| `make setup-dev` | `make initdev` | 初始化开发环境 |
| `make download` | - | 下载引擎 |
| `make install` | `make cmd` | 安装 spx 命令 |
| `make build-wasm` | `make wasm` | 安装 spx 命令并构建 wasm |
| `make build-wasm-opt` | `make wasmopt` | 安装 spx 命令并构建优化的 wasm |
| `make format` | `make fmt` | 格式化代码 |
| `make generate` | `make gen` | 生成代码 |
| `make stop` | - | 停止运行进程 |

### 构建命令

| 命令 | 别名 | 描述 |
| --- | --- | --- |
| `make build-editor` | `make pce` | 构建当前平台的引擎（编辑器模式） |
| `make build-desktop` | `make pc` | 构建当前平台的引擎模板 |
| `make build-web` | `make web` | 构建 Web 引擎模板 |
| `make build-minigame` | `make minigame` | 构建 Web 小游戏模板 |
| `make build-miniprogram` | `make miniprogram` | 构建 Web 小程序模板 |
| `make build-web-worker` | `make webworker` | 构建 Web Worker 模板 |
| `make build-android` | `make android` | 构建 Android 引擎模板 |
| `make build-ios` | `make ios` | 构建 iOS 引擎模板 |

### 导出命令

| 命令 | 别名 | 描述 |
| --- | --- | --- |
| `make export-pack` | `make exportpack` | 导出运行时引擎 pck 文件 |
| `make export-web` | `make exportweb` | 导出 Web 引擎给构建器 |

### 运行命令

| 命令 | 别名 | 描述 |
| --- | --- | --- |
| `make run` | - | 在 PC 上运行演示（运行时模式） |
| `make run-editor` | `make rune` | 在 PC 编辑器模式下运行演示 |
| `make run-web` | `make runweb` | 在 Web 上运行演示 |
| `make run-web-worker` | `make runwebworker` | 在 Web Worker 模式下运行演示 |
| `make test` | `make runtest` | 运行测试 |
| `make run-minigame` | `make runmg` | 运行小游戏 |
| `make run-minigame-opt` | `make runmgopt` | 运行优化的小游戏 |
| `make run-miniprogram` | `make runmp` | 运行小程序 |
| `make run-miniprogram-opt` | `make runmpopt` | 运行优化的小程序 |
| `make serve` | `make runwebserver` | 运行 Web 服务器 |

## 参数说明

可以通过以下参数自定义命令行为：

| 参数 | 默认值 | 描述 |
| --- | --- | --- |
| `path` | `tutorial/01-Weather` | 指定演示项目路径 |
| `port` | `8106` | 指定端口号 |
| `mode` | `""` | 指定运行模式 |

## 命令详细说明

### 设置命令

#### `make setup` / `make init`

初始化用户环境，包括以下步骤：
1. 为工具脚本添加执行权限
2. 安装 spx 命令
3. 下载引擎
4. 准备开发环境
5. 准备 Web 模板

```bash
make setup
```

#### `make setup-dev` / `make initdev`

初始化完整的开发环境，执行以下步骤：
1. 安装 spx 命令
2. 构建 wasm
3. 构建当前平台的引擎（编辑器模式）
4. 构建当前平台的引擎模板
5. 构建 Web 引擎模板

```bash
make setup-dev
```

#### `make download`

安装 spx 命令并下载引擎。

```bash
make download
```

#### `make install` / `make cmd`

安装 spx 命令行工具。

```bash
make install
```

#### `make build-wasm` / `make wasm`

安装 spx 命令行工具并构建 WebAssembly。

```bash
make build-wasm
```

#### `make build-wasm-opt` / `make wasmopt`

安装 spx 命令行工具并构建优化的 WebAssembly，还会压缩 wasm 文件。

```bash
make build-wasm-opt
```

#### `make format` / `make fmt`

格式化 Go 代码。

```bash
make format
```

#### `make generate` / `make gen`

运行代码生成器，然后格式化生成的代码。

```bash
make generate
```

#### `make stop`

停止所有正在运行的进程，包括 Web 服务器进程。支持 Windows 和 Unix/Linux 系统。

```bash
make stop
```

### 构建命令

#### `make build-editor` / `make pce`

构建当前平台的引擎（编辑器模式）。

```bash
make build-editor
```

#### `make build-desktop` / `make pc`

构建当前平台的引擎模板并导出运行时包。

```bash
make build-desktop
```

#### `make build-web` / `make web`

构建 Web 引擎模板并提取普通 Web 模板。

```bash
make build-web
```

#### `make build-minigame` / `make minigame`

构建 Web 小游戏模板并提取小游戏模板。

```bash
make build-minigame
```

#### `make build-miniprogram` / `make miniprogram`

构建 Web 小程序模板并提取小程序模板。

```bash
make build-miniprogram
```

#### `make build-web-worker` / `make webworker`

构建 Web Worker 模式的模板并提取 Worker 模板。

```bash
make build-web-worker
```

#### `make build-android` / `make android`

构建 Android 引擎模板。

```bash
make build-android
```

#### `make build-ios` / `make ios`

构建 iOS 引擎模板。

```bash
make build-ios
```

### 导出命令

#### `make export-pack` / `make exportpack`

导出运行时引擎 pck 文件。

```bash
make export-pack
```

#### `make export-web` / `make exportweb`

为构建器导出 Web 引擎，包括安装带优化的 Web 命令工具。

```bash
make export-web
```

### 运行命令

#### `make run`

在 PC 运行时模式下运行演示项目。

```bash
# 运行默认演示
make run

# 运行指定路径的演示
make run path=test/Hello
```

#### `make run-editor` / `make rune`

在 PC 编辑器模式下运行演示项目。

```bash
# 运行默认演示
make run-editor

# 运行指定路径的演示
make run-editor path=test/Hello
```

#### `make run-web` / `make runweb`

在 Web 浏览器中运行演示项目。会先停止现有进程，构建 wasm，然后启动 Web 服务器。

```bash
# 运行默认演示
make run-web

# 运行指定路径和端口的演示
make run-web path=test/Hello port=8080
```

#### `make run-web-worker` / `make runwebworker`

在 Web Worker 模式下运行演示项目。

```bash
# 运行默认演示
make run-web-worker

# 运行指定路径的演示
make run-web-worker path=test/Hello
```

#### `make test` / `make runtest`

运行测试套件（运行 test/All 目录下的测试）。

```bash
make test
```

#### `make run-minigame` / `make runmg`

运行小游戏（快速构建模式）。

```bash
# 运行默认路径的小游戏
make run-minigame

# 运行指定路径的小游戏
make run-minigame path=test/Hello
```

#### `make run-minigame-opt` / `make runmgopt`

运行优化的小游戏（完整优化构建）。

```bash
# 运行优化的小游戏
make run-minigame-opt path=test/Hello
```

#### `make run-miniprogram` / `make runmp`

运行小程序并启动 Web 服务器。

```bash
# 运行小程序
make run-miniprogram path=test/Hello
```

#### `make run-miniprogram-opt` / `make runmpopt`

运行优化的小程序并启动 Web 服务器。

```bash
# 运行优化的小程序
make run-miniprogram-opt path=test/Hello
```

#### `make serve` / `make runwebserver`

启动 Web 服务器来服务构建的项目。

```bash
# 启动 Web 服务器
make serve path=test/Hello port=8080
```

## 使用示例

### 基本开发流程

```bash
# 1. 初始化开发环境
make setup-dev

# 2. 运行演示项目
make run path=tutorial/01-Weather

# 3. 在 Web 浏览器中测试
make run-web path=tutorial/01-Weather port=8080

# 4. 运行测试
make test
```

### 构建不同平台

```bash
# 构建当前平台
make build-desktop

# 构建 Web 小游戏
make build-minigame

# 构建 Web 小程序
make build-miniprogram

# 构建 Android
make build-android

# 构建 iOS
make build-ios
```

### Web 开发

```bash
# 构建并运行 Web 版本
make run-web path=test/Hello port=8080

# 在 Web Worker 模式下运行
make run-web-worker path=test/Hello

# 停止所有 Web 服务器
make stop
```

### 小游戏和小程序开发

```bash
# 快速测试小游戏
make run-minigame path=test/Hello

# 发布优化的小游戏
make run-minigame-opt path=test/Hello

# 测试小程序
make run-miniprogram path=test/Hello
```

## 帮助命令

如果您不确定可用的命令或其用途，可以运行 `make help` 显示所有可用命令及其简短描述。

```bash
make help
```

这将显示完整的命令列表、参数说明和使用示例。