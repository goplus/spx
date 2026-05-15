# SPX 构建命令指南

这份文档描述当前仓库里真实存在的构建入口，其中 `buildctl` 是主编排入口，`make` 主要作为兼容和快捷命令层。

为了减少重复的 `go run` 启动开销，可以先执行一次 `make buildctl` 生成 `./.bin/buildctl`。之后 `make` 和兼容 shell wrapper 会优先复用这个本地二进制；如果源码有更新，launcher 也会自动重新构建。

## 构建分层

SPX 现在的构建入口分成三层：

- `Makefile`
  对外提供短命令和兼容入口。
- `internal/cmd/buildctl`
  负责核心编排、参数校验和步骤顺序，当前已承接 `env`、`prepare`、`tool install/setup-ndk`、`engine download/build`、`runtime build/export`、`workflow build/demo`。
- `internal/cmd/buildctl/*.sh`
  只保留少量兼容壳和便捷入口，例如 `buildctl.sh`、docker 相关脚本。

如果只是使用仓库，优先调用 `make` 目标；如果要做自动化、CI 或重构构建链，优先调用 `buildctl`，不要自己拼 shell 流程。

已经删除的 legacy shell 入口：

- `internal/tools/bootstrap/prepare.sh`
- `internal/tools/build_engine.sh`
- `internal/tools/build_game.sh`
- `internal/tools/engine/build.sh`
- `internal/tools/common/setup_ndk.sh`
- `internal/tools/common/setup_env.sh`
- `internal/tools/engine/download.sh`
- `internal/tools/make_util.sh`
- `internal/tools/runtime/export_pack.sh`
- `internal/tools/runtime/export_web.sh`
- `internal/tools/runtime/web_template.sh`
- `internal/tools/runtime/compress_wasm.sh`
- `internal/tools/run.sh`

自动化入口示例：

```bash
go run ./internal/cmd/buildctl prepare --setup-mode runtime
go run ./internal/cmd/buildctl env export-shell
go run ./internal/cmd/buildctl tool install --web
go run ./internal/cmd/buildctl tool setup-ndk --manual-install --ndk-path /path/to/android-ndk-r23c-darwin.zip
go run ./internal/cmd/buildctl engine download --runtime
go run ./internal/cmd/buildctl engine build --target template --platform web --mode worker
go run ./internal/cmd/buildctl runtime build-wasm --opt
go run ./internal/cmd/buildctl runtime export-web --mode worker
go run ./internal/cmd/buildctl workflow build-dev
go run ./internal/cmd/buildctl workflow install-apk --project-dir tutorial/00-Hello
go run ./internal/cmd/buildctl workflow run-demo --demo-index 1 --mode web
```

## 快速开始

```bash
# 预编译本地 buildctl 二进制
make buildctl

# 查看所有可用命令
make help

# 查看 buildctl 子命令
./.bin/buildctl help

# 准备本机 host 资产
make prepare-host

# 构建完整本地开发环境
make build-dev MODE=normal

# 准备 Web 导出资产
make prepare-web MODE=normal

# 同时准备 host + Web 导出资产
make prepare-full MODE=worker
```

## 常用参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `GODOT_SRC` | `./godot` | Godot 源码目录；仅 `build-dev`、`build-editor`、`build-desktop`、`build-web`、`build-android`、`build-ios`、`generate` 使用 |
| `MODE` | `normal` | Web 模式，可选 `normal`、`worker`、`minigame`、`miniprogram` |
| `WEB` | `0` | `make install` 是否追加 `--web`，可选 `1/true/TRUE/yes/YES/on/ON` 或 `0/false/FALSE/no/NO/off/OFF` |
| `PLATFORM` | 当前宿主平台 | `download-engine` 使用的平台名 |
| `DEMO_INDEX` | `3` | `tutorial/*` 演示索引 |
| `APK_PROJECT_DIR` | `tutorial/00-Hello` | `install-apk` 使用的项目目录 |
| `PORT` | `8106` | `runweb` 和 `runwebworker` 使用的端口 |
| `MOVIE` | `false` | 运行 demo 时是否启用录制模式 |

## buildctl 总览

当前已经稳定的 `buildctl` 分组：

| 分组 | 示例 |
| --- | --- |
| `env` | `go run ./internal/cmd/buildctl env export-shell` |
| `prepare` | `go run ./internal/cmd/buildctl prepare --setup-mode runtime` |
| `tool` | `go run ./internal/cmd/buildctl tool install --web` / `go run ./internal/cmd/buildctl tool setup-ndk` |
| `engine` | `go run ./internal/cmd/buildctl engine build --target template --platform android` |
| `runtime` | `go run ./internal/cmd/buildctl runtime export-web --mode worker` |
| `workflow` | `go run ./internal/cmd/buildctl workflow run-demo --demo-index 1 --mode web` / `go run ./internal/cmd/buildctl workflow install-apk --project-dir tutorial/00-Hello` |

## 设置命令

| 命令 | 说明 |
| --- | --- |
| `make prepare-host` | 安装 `spx`，下载当前平台 editor、runtime template 和 `gdspxrt.pck` |
| `make build-dev [MODE=...]` | 一次性构建带指定 Web mode runtime 的 `spx` 工具链、当前平台 editor/template、runtime pck 和 Web template |
| `make prepare-web MODE=...` | 安装 Web 导出所需工具链，下载指定模式的 Web template/runtime，并补齐 `exporttemplateweb` 依赖的 host editor |
| `make prepare-full [MODE=...]` | 一次性准备 host editor/runtime 资产和指定 Web mode 的导出资产 |
| `make install [WEB=1]` | 安装 `spx` 命令；`WEB=1` 时会透传为 `buildctl tool install --web` |
| `make download` | 下载当前平台 runtime 所需的 editor/template/pck 资产 |
| `make download-engine PLATFORM=... [MODE=...]` | 下载指定平台的引擎模板或 editor 资产 |

示例：

```bash
make install
make install WEB=1
make download-engine PLATFORM=android
make download-engine PLATFORM=web MODE=worker
```

## 构建命令

以下构建命令会读取 `GODOT_SRC`，默认值为 `./godot`；此外 `make generate` 也会读取这个变量。其他 `make` 目标会忽略这个变量。

| 命令 | 说明 |
| --- | --- |
| `make build-editor` | 构建当前平台 editor，并安装到 `GOPATH/bin` |
| `make build-desktop` | 构建当前平台 desktop template，并导出 runtime pck |
| `make build-web [MODE=...]` | 构建指定模式的 Web template，并导出对应 Web runtime |
| `make build-wasm` | 构建 WebAssembly 版本的 `ispx` |
| `make build-wasm-opt` | 构建优化版 `ispx.wasm`，并执行 brotli 压缩 |
| `make build-android` | 构建 Android template |
| `make build-ios` | 构建 iOS template |
| `make install-apk [APK_PROJECT_DIR=...]` | 导出并安装 Android APK 到设备 |

示例：

```bash
make build-editor
make build-web MODE=worker
make build-android
make install-apk APK_PROJECT_DIR=tutorial/00-Hello
```

## 导出命令

| 命令 | 说明 |
| --- | --- |
| `make export-pack` | 导出 runtime assets zip `spx-runtime-assets.zip` 到 `GOPATH/bin` |
| `make export-web [MODE=...]` | 导出 Web bundle，输出 zip 到仓库根目录 |

`make export-web` 的输出文件：

| 模式 | 输出文件 |
| --- | --- |
| `normal` | `spx_web.zip` |
| `worker` | `spx_web_worker.zip` |
| `minigame` | `spx_web_minigame.zip` |
| `miniprogram` | `spx_web_miniprogram.zip` |

示例：

```bash
make export-web
make export-web MODE=worker
```

## 运行命令

| 命令 | 说明 |
| --- | --- |
| `make list-demos` | 打印 `tutorial/*` 的索引 |
| `make editor DEMO_INDEX=N` | 打开指定 demo 的编辑器模式 |
| `make run DEMO_INDEX=N` | 以解释器运行模式启动指定 demo（对应 `spx run`） |
| `make runnative DEMO_INDEX=N` | 以原生运行时启动指定 demo（对应 `spx runnative`） |
| `make rune DEMO_INDEX=N` | 以编辑器运行时模式启动指定 demo（对应 `spx rune`） |
| `make runweb DEMO_INDEX=N` | 构建 wasm 后启动指定 demo 的 Web 版本 |
| `make runwebworker DEMO_INDEX=N` | 构建 wasm 后启动指定 demo 的 Web Worker 版本 |
| `make stop` | 停止本地 Web server 进程 |

示例：

```bash
make list-demos
make run DEMO_INDEX=1
make runnative DEMO_INDEX=1
make rune DEMO_INDEX=1
make runweb DEMO_INDEX=2 PORT=8080
```

## 其他命令

| 命令 | 说明 |
| --- | --- |
| `make format` | 运行 `go fmt ./...` |
| `make generate` | 执行代码生成并格式化；可用 `GODOT_SRC` 覆盖源码目录 |

## 推荐流程

### 本地开发

```bash
	make build-dev MODE=normal
	make run DEMO_INDEX=1
	make runnative DEMO_INDEX=1
	make runweb DEMO_INDEX=1
```

### 仅准备 Web 导出环境

```bash
	make prepare-web MODE=worker
	make export-web MODE=worker
```

### CI 中下载预编译资产

```bash
make install
make download
make download-engine PLATFORM=android
make export-web MODE=minigame
```
