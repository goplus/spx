# SPX Makefile 命令指南

本文档介绍了 SPX 项目中 Makefile 提供的各种命令及其功能。使用这些命令可以简化开发、构建和运行过程。

## 命令概览

### 工具命令

| 命令 | 描述 |
| --- | --- |
| `make init` | 初始化用户环境 |
| `make initdev` | 初始化开发环境 |
| `make download` | 下载引擎 |
| `make cmd` | 安装 spx 命令 |
| `make wasm` | 安装 spx 命令并构建 wasm |
| `make wasmopt` | 安装 spx 命令并构建优化的 wasm |
| `make fmt` | 格式化代码 |
| `make gen` | 生成代码 |

### 构建命令

| 命令 | 描述 |
| --- | --- |
| `make pce` | 构建当前平台的引擎（编辑器模式） |
| `make pc` | 构建当前平台的引擎模板 |
| `make web` | 构建 Web 引擎模板 |
| `make android` | 构建 Android 引擎模板 |
| `make ios` | 构建 iOS 引擎模板 |

### 导出命令

| 命令 | 描述 |
| --- | --- |
| `make exportpack` | 导出运行时引擎 pck 文件 |
| `make exportweb` | 为 xbuilder 导出 Web 引擎 |

### 运行命令

| 命令 | 描述 |
| --- | --- |
| `make run` | 在 PC 上运行演示（默认：tutorial/01-Weather） |
| `make rune` | 在 PC 编辑器模式下运行演示（默认：tutorial/01-Weather） |
| `make runweb` | 在 Web 上运行演示（默认：tutorial/01-Weather） |
| `make runtest` | 运行测试 |

## 命令详细说明

### 工具命令

#### `make init`

初始化用户环境，包括为工具脚本添加执行权限并下载引擎。

```bash
make init
```

#### `make initdev`

初始化完整的开发环境，这个命令会依次执行以下步骤：
1. 安装 spx 命令
2. 构建 wasm
3. 构建当前平台的引擎（编辑器模式）
4. 构建当前平台的引擎模板
5. 构建 Web 引擎模板

```bash
make initdev
```

#### `make fmt`

格式化 Go 代码。

```bash
make fmt
```

#### `make gen`

运行代码生成器，然后格式化生成的代码。

```bash
make gen
```

#### `make download`

安装 spx 命令并下载引擎。

```bash
make download
```

#### `make cmd`

安装 spx 命令行工具。

```bash
make cmd
```

#### `make wasm`

安装 spx 命令行工具并构建 WebAssembly。

```bash
make wasm
```

#### `make wasmopt`

安装 spx 命令行工具并构建优化的 WebAssembly。

```bash
make wasmopt
```

### 构建命令

#### `make pce`

构建当前平台的引擎（编辑器模式）。

```bash
make pce
```

#### `make pc`

构建当前平台的引擎模板并导出运行时包。

```bash
make pc
```

#### `make web`

构建 Web 引擎模板并提取 Web 模板。

```bash
make web
```

#### `make android`

构建 Android 引擎模板。

```bash
make android
```

#### `make ios`

构建 iOS 引擎模板。

```bash
make ios
```

### 导出命令

#### `make exportpack`

导出运行时引擎 pck 文件。

```bash
make exportpack
```

#### `make exportweb`

为 xbuilder 导出 Web 引擎，包括安装带优化的 Web 命令工具。

```bash
make exportweb
```

### 运行命令

#### `make rune`

在 PC 编辑器模式下运行演示项目。

```bash
# 运行默认演示
make rune

# 运行指定路径的演示
make rune path=demos/demo1
```

#### `make run`

在 PC 运行时模式下运行演示项目。

```bash
# 运行默认演示
make run

# 运行指定路径的演示
make run path=demos/demo1
```

#### `make runweb`

在 Web 浏览器中运行演示项目。

```bash
# 运行默认演示
make runweb

# 运行指定路径的演示
make runweb path=demos/demo1
```

#### `make runtest`

运行测试套件。

```bash
make runtest
```

## 使用示例

- 构建当前平台：
  ```bash
  make pc
  ```

- 运行特定演示项目（PC）：
  ```bash
  make run path=demos/demo1
  ```

- 在 Web 浏览器中运行特定演示项目：
  ```bash
  make runweb path=demos/demo1
  ```

- 运行测试：
  ```bash
  make runtest
  ```

## 帮助命令

如果您不确定可用的命令或其用途，可以运行 `make help` 显示所有可用命令及其简短描述。

```bash
make help
```