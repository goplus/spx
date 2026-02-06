# SPX Makefile 命令指南

本文档说明当前 `Makefile` 的可用命令。以下内容以仓库根目录为执行目录。

## 快速开始

```bash
# 1. 查看命令
make help

# 2. 构建并下载引擎资源
make setup

# 3. 准备 Web 模板（首次使用 Web 模式前执行）
make setup-web MODE=normal

# 4. 运行示例
make run DEMO_INDEX=1
```

## 命令概览

### 构建命令

| 命令 | 说明 |
| --- | --- |
| `make build` | 构建 `dist/bin/spx` 和 `dist/bin/spxrun` |
| `make build-web` | 在 `make build` 基础上构建 Web 运行时资源（`ispx.wasm`、`wasm_exec.js`） |
| `make build-native` | 在 `make build` 基础上构建本机动态库到 `dist/lib` |
| `make build-all` | 等价于 `build + build-web + build-native` |
| `make clean` | 清理 `dist/` 目录 |

### 初始化命令

| 命令 | 说明 |
| --- | --- |
| `make setup` | 执行 `build-all`，并下载引擎到 `dist/share/engines` |
| `make setup-engines` | 仅下载引擎到 `dist/share/engines` |
| `make setup-web MODE=<mode>` | 生成 Web 模板到 `dist/share/templates/<mode>` |

`MODE` 支持：`normal`、`worker`、`minigame`、`miniprogram`。

### 演示运行命令

| 命令 | 说明 |
| --- | --- |
| `make editor DEMO_INDEX=N` | 打开第 N 个示例工程的编辑器 |
| `make run DEMO_INDEX=N` | 运行第 N 个示例工程 |
| `make run-editor DEMO_INDEX=N` | 以 rune 模式运行第 N 个示例工程 |
| `make run-web DEMO_INDEX=N` | 构建 Web 资源后运行 Web 模式 |
| `make run-web-worker DEMO_INDEX=N` | 构建 Web 资源后运行 Web Worker 模式 |

### 工具命令

| 命令 | 说明 |
| --- | --- |
| `make format` | 执行 `go fmt ./...` |
| `make generate` | 运行代码生成并格式化 |
| `make export-pack` | 导出运行时 PCK 包 |
| `make export-web [MODE=...]` | 导出 Web 发布包，默认 `MODE=normal` |
| `make stop` | 停止本地 Web 相关 Python 进程 |

## 参数说明

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `DEMO_INDEX` | `3` | 示例索引（来自 `tutorial/*`） |
| `PORT` | `8106` | Web 运行端口 |
| `MOVIE` | `false` | 是否启用录制参数 |
| `MODE` | 无 | 用于 `setup-web` 和 `export-web` |

## 目录结构（dist）

迁移后所有产物位于 `dist/`：

- `dist/bin/`：`spx`、`spxrun`
- `dist/share/engines/`：运行时引擎与 pck
- `dist/share/templates/`：Web 模板
- `dist/share/ispx/`：Web 相关 ispx 资源
- `dist/share/wasm_exec.js`：Go wasm runtime JS
- `dist/lib/`：本机动态库

## 常见流程

### 本地开发（桌面）

```bash
make setup
make run DEMO_INDEX=1
```

### 本地 Web 开发

```bash
make setup
make setup-web MODE=normal
make run-web DEMO_INDEX=1 PORT=8106
```

### 导出 Web 包

```bash
make setup
make setup-web MODE=worker
make export-web MODE=worker
```

### 查看帮助

```bash
make help
```
