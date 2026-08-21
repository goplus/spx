# SPX 构建命令指南

仓库根目录的 `Makefile` 是稳定、面向使用者的构建入口。它会按需构建并调用仓库内的 `./.bin/buildctl`，日常开发应从 `make` 命令开始。

`buildctl` 是 Makefile、CI 和构建系统调试使用的底层编排入口。本文保留它的维护者用法，但不把它作为普通使用者的主流程。

## 快速开始

准备预编译的本机编辑器和 runtime 资产，然后运行 demo：

```sh
make setup
make list-demos
make run DEMO_INDEX=2
```

为指定 runtime 模式准备 Web 资产：

```sh
make setup-web MODE=normal
make runweb DEMO_INDEX=2
```

修改引擎或 SPX 模块时，从源码构建完整开发环境：

```sh
GODOT_SRC=/absolute/path/to/godot make dev MODE=normal
```

使用 `make help` 查看主要命令，使用 `make help-advanced` 查看底层目标。`make buildctl` 是可选步骤；普通 Make 目标会自动构建缓存的二进制。

## 主要命令

| 命令 | 说明 |
| --- | --- |
| `make setup` | 准备本机编辑器和 native runtime 资产。 |
| `make setup-web MODE=...` | 为一种 runtime 模式准备 Web 资产。 |
| `make dev MODE=...` | 从源码构建完整的本地开发环境。 |
| `make doctor` | 校验最终解析出的 lock、工具链、模块 profile 和 Godot checkout。 |
| `make build-editor` | 从源码构建本机 editor。 |
| `make build-desktop` | 从源码构建本机 desktop template 和 runtime pack。 |
| `make build-web MODE=...` | 从源码构建 Web template 和匹配的 runtime。 |
| `make build-android` | 从源码构建 Android template。 |
| `make build-ios` | 从源码构建 iOS template。 |

预编译并锁定的资产能够满足需求时，使用 `setup` 和 `setup-web`。修改 Godot、`godot_modules/spx`、生成的 bindings 或平台构建行为时，使用 `dev` 或某个专用的 `build-*` 目标。

## 构建参数

| 参数 | 默认值 | 使用范围 |
| --- | --- | --- |
| `GODOT_SRC` | `./godot` | `dev` 和引擎 `build-*` 目标使用的 Godot 源码 checkout。 |
| `SPX_MODULE_SRC` | `./godot_modules/spx` | 引擎构建和 `make generate` 使用的外置 SPX Godot 模块。 |
| `MODE` | `normal` | Web 环境准备、构建、导出，以及 `dev` 中的 Web 构建。 |
| `DEMO_INDEX` | `3` | demo 命令选择的 tutorial 索引。 |
| `PORT` | `8106` | Web demo 命令使用的端口。 |

`MODE` 可选 `normal`、`worker`、`minigame` 和 `miniprogram`。同一套 Web 环境的准备、构建、运行和导出应保持相同模式，避免 template 与 runtime 不匹配。

`GODOT_SRC` 和 `SPX_MODULE_SRC` 都支持绝对路径或相对路径；相对路径从 SPX 仓库根目录解析。使用仓库外的 Godot checkout 时，建议为 `GODOT_SRC` 传绝对路径。通常应保持 `SPX_MODULE_SRC` 的默认值，让仓库自有的外置模块及其构建 profile 成为唯一模块来源。

只准备预编译资产的命令不需要 `GODOT_SRC`；只有从源码编译引擎目标时才会使用它。

## 版本和 profile 来源

当前 SPX 版本的唯一来源是 `internal/release/current_spx_version.go`，当前 runtime 的唯一来源是 `internal/release/runtime.lock.json`；runtime tag 统一按 `runtime-v<runtime_version>` 推导，不再独立保存。项目 scaffold 会自动渲染 current SPX 版本，atomic runtime definition 则由不可变 lock snapshot 自动派生。

Godot、`godot_modules/spx`、toolchain 或 runtime pack 输出变化时，使用 `make bump-release SPX_VERSION=v3.x.y RUNTIME_VERSION=x.y.z`；仅 ABI 变化时才额外传 `RUNTIME_ABI=N`。该命令刻意要求新的 runtime version；SPX-only release 复用公开 runtime 属于另一条必须校验 provenance 的发布决策。

本地 `buildctl` 会从这份 lock 读取全部锁定工具版本，并把锁定的 Go toolchain 传给子构建脚本；SPX CI 也统一从同一位置安装 Go 和 XGo，workflow 不再另设版本默认值。NDK 安装器所需的 `r23c` 一类别名只是由完整 revision 校验得到的适配值，未知映射会直接失败，不能反过来成为版本来源。Godot 的 SCons 功能参数使用另一份唯一来源：`godot_modules/spx/spx_scons_profile.json`。共享的引擎功能开关应在该 profile 中修改，不要在 Makefile 或各平台 CI workflow 中重复配置。

Godot 引擎制品与 SPX runtime pack 使用不同的构建身份：

- Godot 引擎制品由锁定的 Godot commit、`godot_modules/spx` tree（包含 SCons profile）、引擎工具链和平台动态参数决定。修改 `buildctl`、release 元数据或文档不会让 Godot 编译缓存失效。
- `spx-runtime-assets.zip` 是独立的 SPX runtime pack，由 desktop export 命令、项目模板、SPX Go runtime 和实际生成字节的 `runtime export-pack` 路径决定；独立的 run、launcher、Web/mobile export 与 release 编排变化不会让它失效。

当前 runtime tag 原子地发布这两类资产，因此任一类产物发生变化都应提升 `runtime_version`；不过未变化的 Godot 引擎输入仍会命中独立的编译缓存，不会因为版本号或 release 编排变化而整套重编。

## 常用流程

### 使用预编译资产进行本地 SPX 开发

```sh
make setup
make install
spx run --path tutorial/00-Hello
```

### Web 开发

```sh
make setup-web MODE=worker
make runwebworker DEMO_INDEX=2
```

### 引擎和模块开发

```sh
GODOT_SRC=/absolute/path/to/godot \
SPX_MODULE_SRC=./godot_modules/spx \
make dev MODE=normal
```

如果只需要构建某个平台，可将 `dev` 换成 `build-editor`、`build-desktop`、`build-web MODE=...`、`build-android` 或 `build-ios`。

## 直接使用 buildctl

直接调用 `buildctl` 主要用于 CI、构建系统维护和编排层调试。手动调用前先构建缓存的可执行文件：

```sh
make buildctl

./.bin/buildctl setup host --published-runtime
./.bin/buildctl setup web --mode worker
./.bin/buildctl setup full --mode normal
./.bin/buildctl doctor

./.bin/buildctl build dev --mode normal
./.bin/buildctl build editor
./.bin/buildctl build desktop
./.bin/buildctl build web --mode worker
./.bin/buildctl build android
./.bin/buildctl build ios
```

Makefile 会把 `setup`、`setup-web`、`dev` 和 `build-*` 映射到这些命令，并统一补充默认值；其中 `make setup` 明确选择已发布并经 manifest 校验的 runtime pack。除非 CI 任务需要直接执行某个编排步骤，否则优先使用 Make 目标。

## 运行和导出命令

```sh
make list-demos
make editor DEMO_INDEX=2
make run DEMO_INDEX=2
make runnative DEMO_INDEX=2
make rune DEMO_INDEX=2
make runweb DEMO_INDEX=2 PORT=8106
make runwebworker DEMO_INDEX=2 PORT=8106

make export-pack
make export-web MODE=normal
make install-apk APK_PROJECT_DIR=tutorial/00-Hello
make stop
```

## 生成和维护

`make generate` 会重新生成 native/Web bindings、runtime 注册代码并执行格式化。它会读写 `SPX_MODULE_SRC` 指定的外置模块；不要直接修改生成文件。

```sh
make generate
make format
```

`make clean-projects` 和 `make clean-assets` 会删除生成或安装的数据，执行前请检查各目标的清理范围。
