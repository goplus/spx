# [Proposal] XGo Project Driver v1 与 SPX 运行时集成

> 状态：source mode 和 published Engine/PCK 获取已实现；published bridge mode 与协调发布仍待完成
>
> 范围：`goplus/mod`、`goplus/xgo`、`goplus/spx`

## 摘要

为 XGo class project 引入 framework-owned、project-scoped、版本化的 project driver。framework 在自己的 `gox.mod` 中声明 driver；XGo 根据应用的有效 Go module/workspace graph 找到并构建它，然后把 `run` 或 transactional `build` 委托给 driver；`install` 复用 `build`。

XGo 只实现通用的发现、身份校验、进程管理和输出事务，不包含 SPX、Godot、Engine、PCK 或资源格式特判。SPX driver 负责解释执行、运行时资源、项目打包和自包含 launcher。

当前实现完成了 main module、workspace module 和 local replace 的 source mode。runtime `2.4.3` 的 Engine/PCK 从代码内 pin 的 release manifest 获取，并校验大小与 SHA-256；bridge 仍必须从有效 graph 中的 SPX 源码构建。纯版本依赖的 SPX 还需要不可变 bridge manifest 与协调发布链。

## 问题与目标

SPX 的运行模型不是普通的“生成 Go 再执行”：它需要匹配的 Engine Runtime、PCK、解释器 bridge、项目资源目录以及隔离的运行 session。把这些规则写入 XGo 会让通用工具链依赖具体 framework，也无法保证 SPX module、bridge 和 Engine ABI 来自一致版本。

本设计提供以下用户能力：

```bash
xgo run ./game --headless
xgo build -o ./bin/game ./game
xgo install ./game
```

并满足以下约束：

- XGo 不感知任何 SPX/Godot 实现细节；
- project driver 与 class metadata 来自同一份有效 Go graph；
- `xgo run` 不生成 `xgo_autogen.go`，也不向项目写入 `.temp`、`.godot` 或构建中间物；
- `xgo build` 生成可离线运行的 host 平台单文件程序；
- 未声明 driver 的项目完全沿用现有 GenGo 路径；
- 一旦匹配 driver，后续错误必须终止，不得静默回退。

## 设计原则

1. **Framework owns policy**：framework 决定如何运行和打包自己的项目，XGo 只提供生命周期。
2. **One graph, one identity**：metadata、driver package、bridge 源码都由同一有效 module/workspace graph 决定。
3. **Fail closed after match**：只有明确未匹配 driver 时才允许进入旧路径。
4. **Immutable inputs**：关键 metadata、发布 manifest、bundle 和输出都绑定内容摘要或文件身份。
5. **Project remains read-only**：运行状态与用户源码、资源严格分离。
6. **Content-addressed reuse**：跨项目复用相同 Engine/bridge bundle，同时验证每次 cache hit。

## 总体架构

```text
应用 go.mod / go.work
        |
        | 有效 Go graph + //xgo:class
        v
framework gox.mod ---- driver v1 <driver-package>
        |
        v
XGo resolver
  - 解析 target
  - 解析 class metadata 与来源
  - 校验声明模块的 XGo 版本要求
  - 构建 driver
        |
        | driver protocol v1
        v
SPX driver
  - 校验请求与实时文件身份
  - 获取 Engine/PCK 并构建 bridge
  - run: 创建隔离 session 后解释执行
  - build: 生成内嵌完整 payload 的 launcher
```

三仓职责边界如下：

| 组件 | 职责 |
| --- | --- |
| `goplus/mod` | `driver` metadata、resolved module provenance、共享的 typed request 与 argv codec |
| `goplus/xgo` | graph/target 解析、driver 发现与构建、协议调用、信号转发、build/install 输出事务 |
| `goplus/spx` | SPX 协议适配、Engine/bridge/project bundle、解释执行、缓存和自包含 launcher |

共享层只定义 driver-neutral 数据，不提供 driver lifecycle SDK，也不包含 SPX domain rule。

## Metadata 与版本边界

SPX 的声明形式为：

```text
xgo 1.8.0

project main.spx Game github.com/goplus/spx/v3 math
driver v1 github.com/goplus/spx/v3/cmd/xgodriver

class -embed *.spx SpriteImpl
pack assets index.json
```

`driver` 作用于它前面最近的 `project`，每个 project 最多声明一个。语法必须是：

```text
driver <protocol> <driver-import-path>
```

- `protocol` 当前只执行 `v1`；
- driver 必须是合法 Go import path，不允许相对路径、绝对路径或 `path@version`；
- 已知但格式错误的 `driver` 在 strict/lax 解析中都报错；
- driver module version 不写入 `gox.mod`，由应用的有效 Go graph 唯一决定。

五个版本与能力维度各自独立，只在兼容性检查时组合：

| 身份 | 含义 | 真相来源 |
| --- | --- | --- |
| Driver protocol `v1` | XGo 与 driver 的调用契约代际 | `driver` directive |
| Project-driver v1 能力基线 | 首个理解并调度 `driver v1` 的 XGo 版本，当前为 `1.8.0` | XGo 协议实现与兼容策略 |
| 声明模块的 XGo 要求 | 该 framework 版本使用 metadata 或工具特性所需的最低 XGo 版本 | declaring `gox.mod`/`gop.mod` 的 `xgo` directive |
| SPX version/source | driver 与 bridge 的代码身份 | 应用有效 Go graph |
| Engine Runtime/ABI | Engine、PCK 与接口兼容性 | SPX runtime lock 与 release manifest |

`driver v1` 与 `xgo` directive 不会互相改写。例如后续 SPX 将 `xgo` 提升为 `1.9.0`，只表示该 SPX 版本需要 XGo 1.9.0 的能力；v1 协议基线仍是 1.8.0，而该 SPX 项目的有效下限是 `max(1.8.0, 1.9.0) = 1.9.0`，不能反述为“driver v1 最低要求 XGo 1.9.0”。

因此，runtime lock 中记录的 XGo `1.7.5` 只表示该 Engine Runtime 的构建工具链；它不表示 XGo `1.7.5` 具备 project-driver 能力。

## 有效 Graph 与来源身份

XGo 使用标准 Go toolchain 得到有效 build list，并把 logical selection 与 replacement source 分开保存：

- `Selected` 表示 MVS 选择的 module path/version；
- `Replace` 表示实际读取的替代来源；
- `Effective` 表示最终用于读取 metadata 和构建 driver 的源码身份。

同一份受支持 graph policy 必须贯穿 metadata discovery、driver 校验和 driver build，包括 `GOWORK`、`-mod` 与 `-modfile`。不能从原始 `go.mod` 手算依赖，也不能在 driver 侧重新解析出另一张 graph。如果调用方的 `GOFLAGS`/`GOWORK` 无法形成唯一可信的 graph policy，discovery 必须在分类前失败，不能构造替代 graph 或回退到 legacy 路径。project-driver v1 尚未定义 overlay-aware 的项目快照，因此 `-overlay` 只在 target 确认声明了 driver 后明确拒绝；普通项目继续保持 legacy 行为。

class module 的顺序和身份来自应用实际生效的 modfile 中的 `//xgo:class` 标记。只有 target 所属 main/workspace module，或被应用显式标记的 class dependency，能够提供 driver。

resolved metadata 同时携带：

- 声明 project/driver 的 module provenance；
- declaring `gox.mod`/`gop.mod` 的 canonical path 与 SHA-256；
- declaring module 的 XGo 版本要求；
- target modfile 的 path 与内容摘要。

XGo 在导入 resolved class metadata 时校验 target modfile snapshot；driver 在执行前重新校验 declaration、module source 和其他实际使用的路径，但不重新解释 metadata。这样既避免 metadata/driver 来自不同版本，也避免 discovery 后关键文件被替换。

当前 vendor mode 无法提供等价的 module provenance。active module 没有外部 class marker 时，XGo 仍可只依靠该 module 自身的 metadata 分类：非 driver target 继续走旧路径，匹配 driver 后明确报不支持。如果实际生效的 modfile 含外部 class marker，v1 会在分类前 fail closed，因为标准 Go vendor 数据可能省略依赖的 `gox.mod`/`gop.mod`；改读 live replacement 又会破坏 vendor snapshot identity。若要让这类 graph 的 legacy 行为继续兼容，后续必须由 XGo 自有 vendor manifest 固化完整 metadata 身份与摘要。

## Target 发现与分发

driver discovery 位于 target 解析之后、所有 Dir/PkgPath/Files 分支及 GenGo 之前。

一旦确认目标声明了 driver，后续 driver-specific 代码生成由 driver 在自行创建的私有隔离工作目录中完成；v1 不定义 XGo 预生成代码或生成产物交接协议。

| target | v1 行为 |
| --- | --- |
| directory | 支持；扫描目录顶层 project file |
| 单个 project file | 支持；必须是目录内唯一 project file |
| import/package path | 支持；基于调用方有效 graph 定位 |
| 多文件 target | driver 项目拒绝 |
| `...` pattern | driver 项目拒绝 |
| `pkg@version` | 普通目标继续走 legacy；确认匹配 driver 后拒绝；版本只能来自当前 graph |

一个目录必须恰好对应一个 project file。没有 class project 或 project 未声明 driver 时返回 `NotHandled`；只有这个结果允许 XGo 调用旧实现。graph、metadata、协议、driver build 或 driver execution 的任何其他错误都直接返回用户。

`run` 保留应用参数的元素边界和顺序。必要时，target 后的第一个 `--` 仅作为 XGo 的 source/argument 分隔符并由 XGo 移除；后续内容原样交给应用。

`install` 复用 `build` 语义并安装到有效 `GOBIN`；`GOBIN` 为空时使用 `GOPATH` 第一项下的 `bin`。driver v1 一次只接受一个 install target。

## Driver Protocol v1

协议使用共享的 typed request，并编码为确定性的 argv；不使用 JSON request file，也不占用 stdin。stdin、stdout、stderr 直接继承，因此交互和管道行为与普通命令一致。

请求包含五组信息：

1. action：`run` 或 `build`；
2. project snapshot：目录、project file、module root、扩展名和可选 pack metadata；
3. driver identity：package、selected/replacement provenance、declaration path 与摘要；
4. graph/build policy：Go command、work directory、workspace 与允许的 flags；
5. action payload：run 的应用参数，或 build 的 staging/final output。

codec 对未知、重复、缺失、组合不完整或 action 不适用的字段统一报错。路径在共享层做结构校验，在 driver 层绑定真实文件身份。协议 argv 与环境总大小超过平台安全预算时，在启动 driver 前失败。

XGo 当前只向 project driver 传递以下策略：

| 类型 | 支持范围 |
| --- | --- |
| Graph | `-mod=mod|readonly`、`-modfile`；`-overlay` 只在 driver match 后用于给出明确的不支持错误；`-mod=vendor` 只允许基于 active module 做保守 discovery，driver match 或外部 class metadata 无法确定时明确拒绝 |
| Build | `-v`、`-x`、`-work`、`-trimpath=true`、`-buildvcs=false` |

其他 flag 只在 target 确认声明了 driver 后报错，避免改变普通项目的既有行为。

## Driver 构建与进程边界

XGo 在构建 driver 前确认：

- package 是 `main`；
- package 位于声明 driver 的有效 module 内；
- package 的 selected/replacement/source identity 与 metadata 完全一致；
- 当前 XGo 满足 declaring module 的 `xgo` 版本要求；
- `GOOS/GOARCH` 等于 host，不允许借 driver 做交叉构建。

driver 构建在私有临时目录完成，沿用同一 graph policy，并保留调用环境的 CGO 选择。driver 进程继承标准流。每个进程只有一个 command boundary 持有宿主信号；内层 supervisor 只消费 cancellation（包含作为 cause 传入的原始信号），不再重复订阅相同信号，并负责清理整个子进程树。一旦观察到 cancellation，即使子进程在关闭期间以 0 退出，也不能把请求报告为成功。Unix 保留正常退出码或原始信号，Windows 使用 Job Object 管理进程树并将中断表示为退出码。project-driver v1 拒绝任意嵌套 driver dispatch，包括转入另一个 driver。

`XGO_DRIVER=off` 是显式禁用开关，但语义是“报错并停止”，不是回退到 GenGo。

## SPX Source Mode

SPX driver 当前接受以下源码身份：

- 应用本身是 SPX main module；
- SPX 位于当前 workspace；
- SPX 通过无版本的 local replace 引入。

portable driver snapshot 只包含 project root 内的文件，因此会明确拒绝 legacy `extasset` 配置。该限制只属于 project-driver 路径；SPX 现有的 run、native、export 与 pack 命令继续保持 legacy 外部资源行为。

`.config` 合约绑定到实际消费的字节。driver 快照其不存在/存在状态与 SHA-256，在交接前重新校验原路径，之后 run/build 只接收已捕获字节，不再重新打开项目副本。

driver 每次以 host `CGO_ENABLED=1` 从该有效源码构建 interpreter bridge，并校验构建产物仍来自同一 module identity；编译可复用 Go build cache，但 bridge 文件本身不做持久缓存。普通版本依赖、pseudo-version 和 versioned replace 当前均拒绝 published bridge mode；仅有可下载的 Engine/PCK 不能证明 bridge 与该 SPX 版本及 Engine interface 匹配。

Engine/PCK 有两个可信来源：

1. local runtime manifest：显式或从 SPX source tree 发现，包含 runtime/ABI/platform 和每个文件的 SHA-256；
2. published runtime release：使用 `2.4.3` 的代码内 pin 固定 release manifest，再按 manifest 获取对应平台的 Engine 与 PCK。

manifest pin 与 lock 一起内嵌。缺少 pin、大小或摘要不匹配、lock 不匹配时，在使用任何 runtime 资源前 fail closed。

`$GOPATH/bin` 不参与资源发现。文件名、存在性或大小都不能作为 runtime 身份。无本地产物时，干净 checkout 可以下载并校验已发布 Engine/PCK，无需预先执行 install workflow；offline mode 只允许命中完整且校验通过的缓存。

SPX 解释运行使用三个独立根：

| 根 | 用途 |
| --- | --- |
| `ProjectDir` | 用户源码与项目级引用，只读 |
| `AssetDir` | `pack` 指定的项目资源根，只读 |
| `SessionDir` | Engine cwd、临时配置和运行状态，可丢弃 |

driver 保留 `SessionDir` 的 `--path` 控制权，用户不能覆盖它。每次 run 创建新 session，准备 bridge/Engine 配置后启动 Engine；项目目录在成功和失败路径中都不应发生变化。

## 自包含 Build

`xgo build` 生成一个 Go launcher，并在链接前通过 `go:embed` 写入完整 payload。payload 包含：

- Engine executable 与 PCK；
- 当前 graph 构建的 interpreter bridge；
- canonical project bundle；
- SPX/source、host platform、runtime/ABI、component digest 和完整 entry table。

project bundle 采用 allowlist，而不是遍历整个仓库：只收集顶层项目源码、可选 `.config`、完整 pack 目录以及资源索引显式引用且仍位于 `ProjectDir` 内的文件。symlink、特殊文件、路径逃逸、大小超限和大小写/Unicode collision 都拒绝。固定排序、时间、权限与压缩策略使相同输入得到相同 project bundle digest。

生成的 launcher 依赖 SPX 的公开 launcher package，因为生成代码在用户 module graph 中编译，不能导入 SPX `internal` package。launcher 自身只负责校验 payload、物化组件、创建 session、启动 Engine 和复现退出状态。

Darwin payload 在 link 前已经固定，link 后执行 ad-hoc signing；签名后不再追加或修改 executable。该签名保证 Mach-O 完整性，不代表 Developer ID 或 notarization。

driver 只能写 XGo 分配的私有 staging path。返回成功后，XGo 验证产物是非空、非 symlink 的 host executable，再通过同文件系统替换提交最终输出；在该提交点之前，driver 失败不会改变已有目标。原子可见性与已有目标替换只在 host platform/filesystem 提供保证的范围内成立；Windows 的真实替换及 crash/recovery 行为仍需 host CI 验证。`install` 使用相同事务，只改变最终目录。

构建后的 launcher 不需要 Go、XGo、SPX 或网络。它先校验 payload 及 host platform，再从内嵌数据物化 Engine、bridge 和 project，最后在全新 session 中运行。

## 缓存与复用

组件物化缓存按 `namespace + full digest` 寻址，Engine、bridge 和 project 使用独立 namespace：

- 相同 runtime 的不同项目复用同一 Engine；
- 不同 launcher 在执行时可以复用相同 bridge 或 project bundle；
- 不同内容即使文件名相同也不会共用；
- driver 与 source bridge 每次写入新的临时产物，但编译过程自然复用 Go build cache；XGo 不维护另一套持久 driver binary cache。

下载文件和已物化目录在 cache hit 时都会重新校验 manifest、类型、大小和 SHA-256。同大小篡改不会被接受；损坏 entry 在独占锁下修复。首次物化使用 sibling temp、完整校验和同文件系统 rename；原子发布只在 host platform/filesystem 提供保证的范围内依赖，Windows 的真实 publish/repair 与 crash-recovery 场景仍需 host CI 验证。多进程通过 shared/exclusive lease 避免观察 partial state 或删除正在使用的 entry。

launcher 的全部资源来自内嵌 payload，因此第一次运行即使 cache 为空也不会下载。当前不自动执行配额回收；后续 GC 必须继续服从 lease，不能删除正在运行的组件。

## 信任与失败模型

以下输入都视为不可信：module/workspace metadata、driver argv、环境变量、release manifest、ZIP/payload、项目资源索引、cache 内容和已有输出路径。

边界统一遵循：

- canonical path 与真实文件身份同时校验，不只比较字符串；
- driver request 与各类 manifest 使用严格解析，拒绝未知或重复字段；
- ZIP 拒绝 absolute path、`..`、反斜线穿越、duplicate/collision、symlink、device 和压缩炸弹；
- 文件在读取前后校验身份，关键大文件通过已打开的 handle 消费；
- 任一身份、digest、runtime、ABI、platform 或来源不匹配都终止；
- driver、Engine 和 launcher 的子进程由 supervisor 管理，取消后不得遗留子进程；
- driver 路径绝不回退到 native/GenGo。

## 当前边界

- XGo `1.8.0` 是 project-driver 能力基线；更早版本不受支持，也不能依赖旧 parser 给出可靠升级提示；
- 只支持 host desktop：Darwin amd64/arm64、Linux amd64、Windows amd64；
- 不支持 `xgo test`、Web、Android、iOS 和 `GOOS/GOARCH` 交叉构建；
- 不支持 vendor mode、overlay、多个声明 driver 的 target 或任意 Go build flag；
- SPX 要求独立的 project pack directory；
- published SPX bridge mode 尚未启用，仓库外仅指定已发布 SPX 版本不会进入当前 driver；
- launcher 包含项目源码与资源，不提供源码保密；
- launcher 体积主要由 Engine/PCK/bridge 决定，不承诺整个 executable bit-for-bit reproducible；
- XGo 的公开 `tool.RunDir/BuildDir/InstallDir` API 维持原语义；driver dispatch 当前是 CLI 能力。

## 发布顺序

正式启用 published bridge mode 需要一次协调发布：

1. 发布包含 driver metadata、provenance 和 protocol codec 的 `goplus/mod`；
2. XGo 依赖该版本并发布包含 project-driver 能力的版本；
3. SPX 补齐 released identity 的 bridge manifest 消费，并为目标 runtime/ABI 发布可校验的 Engine、PCK、bridge 及不可变 manifest；
4. 验证公开下载、offline cache、source mode 和 self-contained launcher；
5. 最后发布包含 `driver v1` 声明的新 SPX module 版本。

在第 3 步完成前，SPX 对 released module identity 必须保持显式失败，不能借用本机 bridge 或只凭 runtime 文件名猜测兼容性。

## 验收条件

- 普通 XGo 项目的 run/build/install 与变更前一致；
- 声明 driver 的 directory、单文件和 package target 在 discovery 后不执行 GenGo；
- workspace/local replace 的 metadata、driver 与 bridge 始终来自同一有效 graph；
- 干净 SPX checkout 无需 `$GOPATH/bin` 预装资源即可 run/build；
- cache miss、cache hit、offline、并发、kill-recovery 和同大小篡改由平台无关测试覆盖；Windows host CI 另行验证真实 publish/replace 与 crash-recovery 行为；
- run 完整保留 argv、stdin/stdout/stderr 与平台退出语义，Unix 复现信号、Windows 中断返回 130，并且不修改项目；
- build/install 失败不破坏已有输出；
- launcher 在空 cache、无工具链、无网络环境完成首次运行；
- Darwin、Linux、Windows 的真实 host artifact 通过平台 smoke test。
