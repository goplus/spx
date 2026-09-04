# [Proposal] XGo Project Driver v1 与 SPX 运行时集成

> 状态：source mode 与 published bundle 路径已实现；published driver 由 exact SPX module version 直接选择，并在使用前校验 SPX/runtime 版本与资产完整性；immutable-release rollout 与 native Windows 证据仍是生产验收门禁
>
> 范围：`goplus/mod`、`goplus/xgo`、`goplus/spx`
>
> Driver-neutral wire 与 provenance 的 canonical contract：`goplus/mod/driverprotocol/spec-v1.md`（必须先在 mod 落地，再做三仓协调依赖发布）

## 摘要

为 XGo class project 引入 framework-owned、project-scoped、版本化的 project driver。framework 在自己的 `gox.mod` 中声明 driver；XGo 根据应用的有效 Go module/workspace graph 找到并构建它，然后把 `run` 或 transactional `build` 委托给 driver；`install` 复用 `build`。

XGo 只实现通用的发现、身份校验、进程管理和输出事务，不包含 SPX、Godot、Engine、PCK 或资源格式特判。SPX driver 负责解释执行、运行时资源、项目打包和自包含 launcher。

当前实现完成了 main module、workspace module 和 local replace 的 source mode。published mode 采用 combined driver bundle v1：canonical SPX module 的 exact version 选择 tag 恰为 `spx_version`（例如 `v3.2.4`）的 SPX Release 中的 driver 资产，每个 host ZIP 恰好包含 Engine、PCK 和 interpreter bridge。该 ZIP 是独立的 driver 资产，但不是独立的 GitHub Release，也不是现有 standalone Engine/PCK runtime ZIP。manifest 的 `spx_version` 必须与所选 module version 一致，`runtime_version` 必须与当前 module lock 一致；bundle 仍校验大小与 SHA-256，其身份和 URL 不从 `runtime_version` 推导。

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
2. **One graph, one identity**：metadata 与 driver package 来自同一有效 module/workspace graph；source mode 的 bridge 也从该 graph 构建，published mode 则使用 exact module version 对应的 bundle。
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
  - 获取 source runtime 或 published driver bundle
  - source mode 构建 bridge
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
| SPX version/source | driver 与 bridge 的代码身份 | source mode 的有效 Go graph；published mode 所选的 exact module version 与 driver manifest 的同值 `spx_version` |
| Engine Runtime/ABI | Engine、PCK 与接口兼容性 | SPX runtime lock 选择的 runtime version，以及 runtime/driver manifest 的同值 `runtime_version` |

`driver v1` 与 `xgo` directive 不会互相改写。例如后续 SPX 将 `xgo` 提升为 `1.9.0`，只表示该 SPX 版本需要 XGo 1.9.0 的能力；v1 协议基线仍是 1.8.0，而该 SPX 项目的有效下限是 `max(1.8.0, 1.9.0) = 1.9.0`，不能反述为“driver v1 最低要求 XGo 1.9.0”。

因此，runtime lock 中记录的 XGo `1.7.5` 只表示该 Engine Runtime 的构建工具链；它不表示 XGo `1.7.5` 具备 project-driver 能力。

## 有效 Graph 与来源身份

XGo 使用标准 Go toolchain 得到有效 build list，并把 logical selection 与 replacement source 分开保存：

- `Selected` 表示 MVS 选择的 module path/version；
- `Replace` 表示实际读取的替代来源；
- `Effective` 表示最终用于读取 metadata 和构建 driver 的源码身份。

除 `pkg@version` 的隔离分类探针外，同一份受支持 graph policy 必须贯穿 metadata
discovery、driver 校验和 driver build，包括 `GOWORK`、`-mod` 与 `-modfile`；不能从原始
`go.mod` 手算依赖，也不能在 driver 侧重新解析出另一张 graph。host Go 缺失、GOFLAGS
词法解析失败、GOWORK 查询/canonicalization 失败，或 graph 输入出现非 `not exist` 的
检查错误时，discovery 必须在分类前失败。

为了不让 v1-only flag policy 改变普通 XGo 命令，另一类错误可以 deferred：语法上能
分离但 v1 不支持的 flag/value，以及不存在的 `-modfile`/`-overlay`，可以从只读
metadata compatibility probe 的 graph policy 中移除并记录原错误。这个 probe 只能回答
“是否声明 driver”，不能校验/build/执行 driver，也不是可传给 consumer 的替代 graph；
未匹配时原始 flag 交回 legacy 命令自行处理，匹配时先返回记录的 driver-policy error。
合法 `-overlay` 可以参与 classification view，但同样在 positive match 后拒绝。
project-driver v1 尚未定义 overlay-aware 的项目快照，因此普通项目继续保持 legacy 行为。

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
| `pkg@version` | 在 `GOWORK=off`、`-mod=mod` 的临时隔离 graph 中只分类所请求版本；未匹配 driver 时走 legacy，匹配后明确拒绝执行 |

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

CLI 显式默认值 `-trimpath=false` 与 `-buildvcs=auto` 等价于省略，XGo 接受后将其
归一化掉，不写入 protocol；其他有语义或未知 flag 仍在 driver match 后拒绝。

其他 flag 只在 target 确认声明了 driver 后报错，避免改变普通项目的既有行为。

SPX driver 不会为了项目只读而把 `-mod=mod` 强制收紧为 `-mod=readonly`。workspace
模式稳定复制并改写私有 `go.work/go.work.sum`；`GOWORK=off` 且使用 `-mod=mod`
时，稳定复制 active modfile 与配套 sum 到私有 `graph.mod/graph.sum`，并以 active
module root 为基准把相对 local replacement 改为绝对路径，即使显式 modfile 位于别处。
后续 SPX-owned Go 命令只更新私有副本，原始 graph 文件和
协议声明的 target modfile 仍由 verifier 绑定。

## Driver 构建与进程边界

XGo 在构建 driver 前确认：

- package 是 `main`；
- package 位于声明 driver 的有效 module 内；
- package 的 selected/replacement/source identity 与 metadata 完全一致；
- 当前 XGo 满足 `max(1.8.0, declaring module 的 xgo 要求)`；
- `GOOS/GOARCH` 等于 host，不允许借 driver 做交叉构建。

driver 构建在私有临时目录完成，沿用同一 graph policy，并保留调用环境的 CGO 选择。
driver 进程继承标准流。外部 driver/Engine chain 只有一个 command boundary 持有宿主
信号；内层 supervisor 只消费 cancellation（包含作为 cause 传入的原始信号），不再重复
订阅相同信号，并负责清理该 chain 的整个子进程树。Unix driver 必须在独立 process group
中运行；Windows driver 必须在执行用户代码前加入 kill-on-close Job。discovery 与 driver
build 使用的同步 Go helper 必须受 context 约束并完成 `Wait`，但 v1 不把它们扩展成另一套
process-group/Job tree 合同。一旦观察到 cancellation，即使子进程在关闭期间以 0 退出，
也不能把请求报告为成功。Unix 保留正常退出码或原始信号，Windows 将宿主中断表示为
`130`。project-driver v1 拒绝任意嵌套 driver dispatch，包括转入另一个 driver。

`XGO_DRIVER=off` 是显式禁用开关，但语义是“报错并停止”，不是回退到 GenGo。

## SPX Source 与 Published Mode

SPX driver 当前接受以下源码身份：

- 应用本身是 SPX main module；
- SPX 位于当前 workspace；
- SPX 通过无版本的 local replace 引入。

这些身份始终使用 source mode。即使 selected module 带有版本，main/workspace module 或无版本 local replace 也不会切换到 published mode。

portable driver snapshot 只包含 project root 内的文件，因此会明确拒绝 legacy `extasset` 配置。该限制只属于 project-driver 路径；SPX 现有的 run、native、export 与 pack 命令继续保持 legacy 外部资源行为。

`.config` 合约绑定到实际消费的字节。driver 快照其不存在/存在状态与 SHA-256，在交接前重新校验原路径，之后 run/build 只接收已捕获字节，不再重新打开项目副本。

source mode 中，driver 每次以 host `CGO_ENABLED=1` 从该有效源码构建 interpreter bridge，并校验构建产物仍来自同一 module identity；编译可复用 Go build cache，但 bridge 文件本身不做持久缓存。

published mode 不从有效 graph 构建或借用 bridge，只接受 canonical module `github.com/goplus/spx/v3` 的 exact canonical release version，且不得有 replacement。pseudo-version、versioned replacement 和 foreign module path 都 fail closed。driver 直接由所选 module version 定位 tag 与所选 module version 及校验后的 `spx_version` 完全相同的 SPX Release 下的 `driver-manifest.json` 与 host ZIP 资产，而不是现有 standalone runtime ZIP。manifest 的 `spx_version` 必须等于所选 module version，`runtime_version` 必须等于当前 module lock 选择的 runtime version；manifest 与 ZIP 仍严格校验 schema、host、entry 名称、大小和 SHA-256。每个 host ZIP 必须恰好包含 Engine、PCK 和 bridge，缺失、额外、重复或不匹配的 entry 都 fail closed。已验证 ZIP 可通过 content-addressed cache 复用；offline mode 只有在完整且校验通过的 cache hit 时成功。

Runtime 输入按 mode 选择：

1. source mode：显式 local override 优先；否则先获取 lock 所选的 `runtime-v<runtime_version>` release，并要求 manifest 中的 runtime version 一致；只有发布资源不可用时才使用 exact-version 的 source/GOPATH local runtime；
2. published mode：exact SPX module version 选择 tag 与该版本及 `spx_version` 完全相同的 Release 中包含三个组件的 driver host ZIP，不按 `runtime_version` 选择 driver bundle，也不单独查找 Engine/PCK。

三个 Engine 范围内的 manifest 使用不同名称并具有不同生命周期：`engine-source-manifest.json` 是本地 source-mode 的 Engine/PCK 输入，`engine-component-manifest.json` 在 launcher payload 的 `engine/` 下现场生成，`engine-acquisition-manifest.json` 记录 Engine/PCK 如何进入内部 Engine cache。Runtime Release 级 manifest 仍名为 `runtime-manifest.json`；对应 lock 字段、Release 资产名和 URL 合同均不变。

尚未发布的 SPX 源码升级不会引入 published driver 资产依赖。main/workspace/local replace 始终保持 source mode；只要 `runtime_version` 未变，就继续复用同一个 published Engine/PCK，并从当前源码重新构建 bridge。只有 exact published module version 才要求对应 SPX Release 中的 driver manifest 与 host bundle，release CI 必须在这些资产校验完成后再公开 module tag。

这里的“源码模式”只指 main/workspace/local replace。外部 demo 的 `require github.com/goplus/spx/v3 vX.Y.Z //xgo:class` 即使已把源码下载到 Go module cache，仍属于 published mode，并使用 `vX.Y.Z` SPX Release 中的 driver 资产；module cache 不是可变源码工作区，也不作为本机构建 bridge 的隐式后门。

manifest 格式错误或 version、size、digest、host 不匹配时，两种 mode 都会在使用任何 runtime 资源前 fail closed。

published mode 不使用 `$GOPATH/bin` 兜底。发布身份由所选 module/lock version 与 manifest 中对应的版本声明确定，文件名、存在性或大小本身不足以证明内容完整。无本地产物时，干净 source checkout 可以下载并校验已发布 Engine/PCK；published mode 下载并校验 combined driver ZIP。offline mode 只允许命中完整且校验通过的缓存。

SPX 解释运行使用三个独立根：

| 根 | 用途 |
| --- | --- |
| `ProjectDir` | 用户源码与项目级引用；除 XGo 指定的 staging 文件外只读 |
| `AssetDir` | `pack` 指定的项目资源根，只读 |
| `SessionDir` | Engine cwd、临时配置和运行状态，可丢弃 |

driver 保留 `SessionDir` 的 `--path` 控制权，用户不能覆盖它。每次 run 创建新 session，
准备 bridge/Engine 配置后启动 Engine；driver-owned 准备与清理只写 XGo 指定的 staging
文件，Engine/应用自身的文件写入不属于 v1 sandbox 保证。

## 自包含 Build

`xgo build` 生成一个 Go launcher，并在链接前通过 `go:embed` 写入完整 payload。payload 包含：

- Engine executable 与 PCK；
- source mode 当前 graph 构建的 bridge，或 published driver bundle 中已验证的 bridge；
- canonical project bundle；
- SPX/source、host platform、runtime/ABI、component digest 和完整 entry table。

project bundle 采用 allowlist，而不是遍历整个仓库：只收集顶层项目源码、可选 `.config`、完整 pack 目录以及资源索引显式引用且仍位于 `ProjectDir` 内的文件。symlink、特殊文件、路径逃逸、大小超限和大小写/Unicode collision 都拒绝。固定排序、时间、权限与压缩策略使相同输入得到相同 project bundle digest。

生成的 launcher 依赖 SPX 的公开 launcher package，因为生成代码在用户 module graph 中编译，不能导入 SPX `internal` package。launcher 自身只负责校验 payload、物化组件、创建 session、启动 Engine 和复现退出状态。

Darwin payload 在 link 前已经固定，link 后执行 ad-hoc signing；签名后不再追加或修改 executable。该签名保证 Mach-O 完整性，不代表 Developer ID 或 notarization。

driver 只能在 XGo 分配的私有 staging 目录内写入，并必须在指定 staging path 产生目标。
返回成功后，XGo 只验证并提交该目标；其他临时或诊断文件不参与提交，并随 staging
目录清理。目标必须是非空、非 symlink 的 host executable，再通过同文件系统替换提交
最终输出；在该提交点之前，driver 失败不会改变已有目标。原子可见性与已有目标替换只在
host platform/filesystem 提供保证的范围内成立；Windows 的真实替换及 crash/recovery
行为仍需 host CI 验证。`install` 使用相同事务，只改变最终目录。

构建后的 launcher 不需要 Go、XGo、SPX 或网络。它先校验 payload 及 host platform，再从内嵌数据物化 Engine、bridge 和 project，最后在全新 session 中运行。

## 缓存与复用

组件物化缓存按 `namespace + full digest` 寻址，driver、Engine、bridge 和 project 使用独立 namespace：

published acquisition 先在 driver namespace 中完整校验并缓存 combined ZIP；只有整包校验通过后，launcher 执行阶段才按各组件摘要物化和复用 Engine、bridge 与 project，组件缓存不能绕过整包信任边界。

- 相同 runtime 的不同项目复用同一 Engine；
- 不同 launcher 在执行时可以复用相同 bridge 或 project bundle；
- 不同内容即使文件名相同也不会共用；
- source mode 每次写入新的临时 driver 与 bridge，编译过程自然复用 Go build cache；published mode 只复用已验证的 bundle 组件。

下载文件和已物化目录在 cache hit 时都会重新校验 manifest、类型、大小和 SHA-256。
在 manifest 不变时，bundle 或组件的同大小篡改不会被接受；损坏 entry 在独占锁下
修复。首次物化使用 sibling temp、完整校验和同文件系统 rename；原子发布只在 host
platform/filesystem 提供保证的范围内依赖，Windows 的真实 publish/repair 与
crash-recovery 场景仍需 host CI 验证。多进程通过 shared/exclusive lease 避免观察
partial state 或删除正在使用的 entry。

launcher 的全部资源来自内嵌 payload，因此第一次运行即使 cache 为空也不会下载。当前不自动执行配额回收；后续 GC 必须继续服从 lease，不能删除正在运行的组件。

## 信任与失败模型

以下输入都视为不可信：module/workspace metadata、driver argv、环境变量、release manifest、ZIP/payload、项目资源索引、cache 内容和已有输出路径。被选中的 framework driver 本身是以 XGo 同等 OS 权限运行的可信可执行代码。项目只读与输出事务只约束符合规范的 driver，并隔离意外失败；v1 不是用来对抗恶意 driver 的访问控制 sandbox。

published acquisition 当前信任 canonical `goplus/spx` GitHub Release 的 HTTPS 入口，以及有权公开该 Release 的仓库权限控制。下载到的 manifest 是 bundle 与组件 SHA-256 的完整性根；这些摘要只能证明内容与 manifest 一致，不能独立认证发布者，也不会把 manifest 与 Go module source 做密码学绑定。offline cache 中的 raw manifest 也没有另一个外部摘要，它的已缓存字节是本地完整性根；v1 不声称能抵御同时替换该根和组件的本地攻击者。project-driver v1 目前不定义额外签名、透明度证明或源码内嵌的 manifest digest。需要更强供应链身份保证的部署，必须把这类签名绑定作为版本化合同加入，不能把现有 checksum 当成发布者认证。

生产发布还要求新的 public runtime 与 SPX Release 使用 [GitHub Immutable Releases](https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases)。资产组装期间 draft 可以变化；公开后 tag 与资产必须不可变，并携带 GitHub release attestation。这是发布侧门禁：v1 acquisition 与 offline cache 仍只校验 manifest 和资产摘要，不传输或重新验证 GitHub attestation。历史 mutable Release 不能满足 project-driver v1 的生产发布门禁。

边界统一遵循：

- canonical path 与真实文件身份同时校验，不只比较字符串；
- driver request 与各类 manifest 使用严格解析，拒绝未知或重复字段；
- ZIP 拒绝 absolute path、`..`、反斜线穿越、duplicate/collision、symlink、device 和压缩炸弹；
- 文件在读取前后校验身份，关键大文件通过已打开的 handle 消费；
- 任一版本、digest、runtime、ABI 或 platform 不匹配都终止；
- driver、Engine 和 launcher 的子进程由 supervisor 管理，取消后不得遗留子进程；
- driver 路径绝不回退到 native/GenGo。

## 当前边界

- XGo `1.8.0` 是 project-driver 能力基线；更早版本不受支持，也不能依赖旧 parser 给出可靠升级提示；
- 只支持 host desktop：Darwin amd64/arm64、Linux amd64、Windows amd64；
- 不支持 `xgo test`、Web、Android、iOS 和 `GOOS/GOARCH` 交叉构建；
- 不支持 vendor mode、overlay、多个声明 driver 的 target 或任意 Go build flag；
- SPX 要求独立的 project pack directory；
- Windows 显式 `-o` 输出名保持原样；只有默认输出、目录目标和 install 输出补 `.exe`；
- published mode 只接受 canonical SPX exact release module，以及 tag 恰等于 `spx_version`、且 `spx_version`、`runtime_version` 分别匹配 module 与 lock 的 Release host bundle；缺少 Release 资产、manifest 版本不一致或发布输入无效时必须 fail closed；
- launcher 包含项目源码与资源，不提供源码保密；
- launcher 体积主要由 Engine/PCK/bridge 决定，不承诺整个 executable bit-for-bit reproducible；
- build/install 只保证单次调用的私有 staging/commit 事务，不为多个进程写入同一最终路径提供跨进程锁；
- XGo 的公开 `tool.RunDir/BuildDir/InstallDir` API 维持原语义；driver dispatch 当前是 CLI 能力。

## 发布顺序

`goplus/mod` 的 metadata/provenance/codec 与 XGo project-driver 支持是前置条件。
必须先发布包含完整 `driverprotocol` v1（包括 target modfile 身份）的 mod 版本，再提升
XGo/SPX 的 module 依赖并通过各自 `GOWORK=off` 测试。正式发布随后在一次
`publish-release` 中按 runtime→driver 构建→SPX Release 组装与公开的顺序推进，使
driver bundle 不与 `runtime_version` 绑定：

首次 project-driver v1 生产发布前，仓库管理员必须启用 GitHub Immutable Releases。
pipeline 可以组装 mutable draft，但新公开的 runtime 或 SPX Release 必须在满足发布门禁
前变为 immutable。

1. **Runtime 阶段**：解析 lock 选择的 `runtime-v<runtime_version>`。只有在 immutable-release 门禁下公开的同版本 Release 才能复用；若已有同版本 Release 属于历史 mutable 资产，必须先提升 runtime lock/version，再构建、校验并发布新的 immutable runtime 资产。
2. **Driver asset 阶段**：从 exact SPX release source 为每个支持的 host 构建 ZIP，并生成 manifest；每个 ZIP 必须恰好包含 Engine、PCK 和 bridge，并冻结名称、大小和 SHA-256。
3. **SPX Release 阶段**：将产品资产、driver manifest 和四个平台 ZIP 合并到 tag 恰为 `spx_version` 的同一个 draft，生成覆盖全部资产的 `SHA256SUMS`。在所有资产通过校验后，以 exact release tag/version 公开 canonical SPX module 对应的这一个 Release；发布前验证 exact-module 下载、manifest/ZIP 严格校验、cache/offline、source mode 与自包含 launcher。

统一的 `publish-release` 状态机自动执行三个阶段；driver 阶段只构建和校验资产，不创建或公开第二个 Release，也不在阶段之间停下来生成文件或要求额外 commit。runtime 的选择身份只来自 lock 中的 `runtime_version`，复用时另行满足 immutable publication 与 manifest/资产完整性门禁；driver 资产的选择只使用所选 `spx_version` 与 `runtime_version`。这些门禁不形成另一套发布身份。

driver bundle 的 URL 与身份只来自 tag 与 exact SPX module version、`spx_version` 完全相同的 SPX Release，绝不从 `runtime_version` 推导；manifest 的 runtime version 只确认 bundle 使用当前 lock 对应的 Engine/PCK。对应 Release 的 driver 资产尚未齐备或 manifest 版本不一致时，published mode 必须显式失败，不能借用本机 bridge 或只凭 runtime 文件名猜测兼容性。

## 验收条件

- 普通 XGo 项目的 run/build/install 与变更前一致；
- 声明 driver 的 directory、单文件和 package target 在 discovery 后不执行 GenGo；
- workspace/local replace 的 metadata、driver 与 bridge 始终来自同一有效 graph；
- 干净 SPX checkout 无需 `$GOPATH/bin` 预装资源即可 run/build；
- 发布验证必须覆盖 published bundle 的 cache miss/hit、offline、并发、kill-recovery 和同大小篡改；Windows host CI 对真实 publish/replace 与 crash-recovery 的覆盖是发布前条件；
- canonical released SPX module 能按 exact module version 下载 `driver-manifest.json` 与 host ZIP，要求其中 SPX/runtime version 分别匹配 module 与 lock，并拒绝 pseudo-version、versioned replacement、foreign module、格式错误的 manifest，以及不是恰好 Engine/PCK/bridge 的 ZIP；
- run 完整保留 argv、stdin/stdout/stderr 与平台退出语义，Unix 复现信号、Windows 中断返回 130，并且不修改项目；
- build/install 失败不破坏已有输出；
- 自包含 launcher 内嵌 Engine、PCK、bridge 和 project，在空 cache、无工具链、无网络环境完成首次运行；
- Darwin、Linux、Windows 的真实 host artifact 在发布前通过平台 smoke test；
- 本 project-driver v1 合同下新发布的每个 runtime/SPX Release 均为 immutable，且 native Windows CI 通过 descendant cleanup 与 transactional output 测试。

## v1 规范性合同

Driver-neutral metadata、argv、schema 与 provenance 规则由
`goplus/mod/driverprotocol/spec-v1.md` canonical 持有；该文件必须先于三仓协调依赖发布
落地。下文凡重述 driver-neutral metadata、调用帧、codec、provenance 与 transport budget
的内容均为说明性摘要；若有冲突，以 canonical spec 为准。XGo dispatch 与 SPX runtime
扩展仍是本提案的规范要求。修改 driver-neutral wire 或 provenance 时必须先更新 canonical
spec，再同步三仓；仅属于 XGo/SPX 的 dispatch 或 runtime 合同由本提案持有。未知字段不得
被“尽量忽略”。

### 规范关键词与符合性

本节中的“必须（MUST）”“不得（MUST NOT）”“应该（SHOULD）”和“可以（MAY）”是
规范关键词；只有明确标为说明性摘要、示例、说明或实现注记的文字不是强制合同。一个 v1 实现只有在
以下三个层次同时成立时才符合本规范：

1. **Codec 层**：producer 产生唯一的 canonical argv；consumer 接受本节允许的等价
   argv，并拒绝未知、重复、缺失或组合无效的字段。
2. **Dispatcher 层**：XGo 从同一有效 graph 得到 target、metadata 与 driver identity，
   严格执行 `NotHandled`/matched 分界、版本门槛和输出事务。
3. **Driver 层**：SPX 在使用路径、graph 或发布资产前重新绑定 live identity，并遵守
   source/published mode、项目只读、进程和缓存合同。

较低层验证成功不能替代较高层验证：codec 的 absolute/clean path 检查不证明文件存在，
resolver 的 snapshot 不授权 consumer 跳过 live revalidation，manifest checksum 也不等于
发布者签名。本节明确列为 wire/archive 合同的数值限额、排序、摘要公式和退出状态是 v1
互操作要求；未列出的 graph 内部上限，以及 acquisition/component/cache manifest 和
payload 的内部字段可以在不改变外部选择、完整性与启动行为时演进，不自动要求协议升级。

### Metadata 与能力协商

`gox.mod`/`gop.mod` parser 接受语法正确的 `driver vN <package>`，其中 `N >= 1`，
并把 protocol 原值保留在 resolved metadata 中；parser 不替 dispatcher 推断能力。
XGo v1 dispatcher 只执行 `v1`。一旦 target 的 project metadata 存在任何 `driver`
声明，它就是 driver match：`v2` 或更高版本必须返回 `unsupported driver protocol`
之类的终止错误，不能伪装成 `NotHandled` 后进入 GenGo。未来 dispatcher 只有在同时
实现对应 argv、进程和事务合同后才能声明新 protocol capability；提高 declaring
module 的 `xgo` directive 只能提高 XGo 最低版本，不能把未知 protocol 降级成 v1。

`driver` 归属于它前面最近的 `project`，每个 project 最多一个；protocol 必须匹配
`v[1-9][0-9]*`，package 必须通过 Go import-path 校验。已知 directive 的参数个数、
protocol 或 package 格式错误在 strict/lax metadata 解析中都属于错误。

### 调用帧与字段

driver 可执行文件的参数帧固定以 `xgo-driver-v1` 开始，随后是一个 action：

`Request.Version` 不编码成独立 option。前两个 argv 元素必须分别是大小写敏感的
`xgo-driver-v1` 与 `run`/`build`；decoder 从 preamble 得到 `Version == "v1"`。
producer 必须通过接受 argument slice 的结构化进程 API 启动 driver，不得经 shell 调用。

```text
xgo-driver-v1 run   <common-options> <graph-flag>* <build-flag>* -- <application-arg>*
xgo-driver-v1 build <common-options> <graph-flag>* <build-flag>* --output=<staging> --final-output=<final>
```

canonical encoder 必须按下列顺序输出；`<selected-source>` 与 `<replacement>` 二选一，
`<pack>` 整组可选，两个 repeatable flag block 分别保留 XGo 归一化后的 slice 顺序：

```text
xgo-driver-v1 <action>
  --project-dir=...
  --project-file=...
  --module-root=...
  --driver-package=...
  --selected-path=...
  --selected-version=...
  --origin-main=true|false
  ( --selected-dir=... --selected-gomod=... |
    --replace-path=... --replace-version=... --replace-dir=... --replace-gomod=... )
  --project-ext=...
  --project-full-ext=...
  [ --pack-dir=... --pack-index=... ]
  --declaration-file=...
  --declaration-sha256=...
  --target-modfile=...
  --target-modfile-sha256=...
  --go-command=...
  --graph-work-dir=...
  --go-work=...
  { --graph-flag=... }
  { --build-flag=... }
  ( -- <application-arg>* |
    --output=<staging> --final-output=<final> )
```

`Parse` 必须接受 run delimiter 前或 build action 后任意顺序的合法 protocol option，
保持语义兼容。singular option 的 placement 及不同 option 类别的 interleaving 不参与
身份；每个 repeatable flag slice 和 application args 的内部顺序必须保留。singular option
恰好一次或至多一次，取决于下表的必填/可选条件；`graph-flag`/`build-flag` 可以重复
option 名，但同一底层 flag 名不得重复。option 在第一个 `=` 处分割，value 可以包含后续
`=`；v1 没有 quoting、percent-encoding、响应文件或环境变量替换。argv 元素中的 NUL
一律非法。

所有 option 必须是单个 `--name=value` 参数。`run` 的第一个独立 `--` 是唯一的协议
分隔符；它之后的空字符串、含空格参数、以 `-` 开头的参数及字面值 `--` 都按元素原样
传递。`build` 不允许协议分隔符或应用参数。producer 必须在启动前拒绝自己构造出的
无效 request；consumer 仍必须独立拒绝重复、未知、缺失、组合无效或 action 不适用的
字段，并在 acquisition/build/Engine 等副作用前以 request-validation error 退出。协议
不使用 request 文件，也不占用 stdin。

| 字段 | 约束 |
| --- | --- |
| `project-dir` / `project-file` | shared 层要求绝对 clean path，且 project file 在字符串结构上是目录顶层文件 |
| `module-root` | shared 层要求绝对 clean path，并在路径结构上包含 `project-dir` |
| `project-ext` / `project-full-ext` | 非空且不含 NUL；来自 XGo 的目标解析结果；SPX 要求 `.spx` / `main.spx`，且 project-file basename 必须是 `main.spx` |
| `driver-package` | 合法 Go import path，且位于 `selected-path` 模块内 |
| `selected-path` / `selected-version` | MVS 逻辑选择；path 必须是合法 Go module path，main module 的 version 为空，其他 module 必须有 canonical version |
| `origin-main` | 只能是 `true` 或 `false` |
| `selected-dir` / `selected-gomod` | 无 replacement 时必须成组出现；只在结构层表达 selected source |
| `replace-path` / `replace-version` / `replace-dir` / `replace-gomod` | 有 replacement 时必须四项齐全；此时禁止 `selected-dir`/`selected-gomod` |
| `declaration-file` / `declaration-sha256` | declaring module 的 `gox.mod` 或 `gop.mod` 及其小写 SHA-256 |
| `target-modfile` / `target-modfile-sha256` | XGo 解析 class graph 时实际读取的 modfile 及其小写 SHA-256；只做结构与身份绑定，不要求位于 project/module root 内 |
| `go-command` / `graph-work-dir` / `go-work` | graph 使用的 Go 可执行文件、工作目录和 workspace；directory/file 使用 canonical project dir，package 使用 caller cwd；`go-work=off` 表示禁用 workspace |
| `graph-flag` | shared codec 识别 `-mod=mod|readonly|vendor`、`-modfile=<abs>`、`-overlay=<abs>`；后两种特殊策略由 consumer 决定是否执行 |
| `build-flag` | 只允许 `-v=true`、`-x=true`、`-work=true`、`-trimpath=true`、`-buildvcs=false` |
| `pack-dir` / `pack-index` | 可选但必须成组；目录是 `.` 或 portable relative slash path，index 是 portable 普通文件名；SPX 额外拒绝 `.` |
| `output` / `final-output` | 仅 build 使用，均为绝对路径且词法不同；前者是 XGo 私有 staging，后者是用户目标 |

option 是否出现与 value 是否允许为空是两件事。`selected-version` 始终出现，main module
时值为空；无 replacement 时 selected source 两项必须出现，replacement 四项不得出现；
有 replacement 时规则相反，其中 local replacement 的 `replace-version` 仍出现但值为空。
pack 两项可以整组省略。build output 两项必须同时出现，run 中必须都不出现。除明确允许
空 version 的情形外，required 字段不得借空 value 满足基数。

两个 digest 都必须恰好是 64 个小写十六进制字符。graph/build flag 统一使用非空的
`-name=value` 并按 name 去重。shared 层只要求 `output` 与 `final-output` 的 lexical
spelling 不同，不声明它们是否指向同一文件；SPX launchpack validation 必须拒绝
staging 位于 pack/asset root 内，但允许 XGo 指定的事务位于 `ProjectDir` 其他位置。
portable pack path 使用 `/`，每个元素不得含低于 U+0020 的字符、反斜线、Windows
非法字符/保留设备名，也不得以点或空格结尾。

`ResolvedModule` 的身份比较同时包含 `Main`、selected path/version/源信息和
replacement 信息。local replacement 的 `replace-path` 必须是 clean absolute
directory spelling；versioned replacement 仍须保留 module path/version，但 SPX
published policy 会拒绝它。`Main=true` 要求 selected version 为空且没有 replacement；
非 main selection 要求非空 canonical module version。无 replacement 的 selected
source 与有 replacement 的 effective source 都必须提供 absolute clean `Dir/GoMod`；
local replacement 的 path 必须与 replacement dir 使用相同的 clean absolute spelling，
version 为空。shared codec 只做不读取文件系统的结构校验：绝对/clean
path、lexical containment、字段组合、import/module version 和 digest 形状。XGo 在
discovery 时解析真实路径并固定来源；SPX consumer 收到后必须重新执行 `Lstat`、
`EvalSymlinks`、路径相等性、普通文件/目录/可执行类型、适用于该字段的 containment、
`SameFile`（读取跨越时）和内容摘要校验，不能把某个字段的 containment 无差别套到
所有 identity path。因而“通过 shared validation”不表示文件存在，也不表示路径不是
symlink。

`FileIdentity` 是不可拆分的 `{Path, SHA256}`：path 表示 producer 实际读取的文件，
SHA-256 表示该次读取的精确 bytes。`Declaration` 必须是 effective driver source 目录的
直接子文件，basename 为 `gox.mod` 或 `gop.mod`；`TargetModFile` 必须来自产生该 class
graph 的同一次 resolution。后者可以是项目 `go.mod`、显式 `-modfile` 或 module download
cache 中的 `.mod`，不得对它施加 project/module containment 或 active-modfile equality。
identity 不匹配必须终止当前 request，不能在原 request 中静默刷新摘要；调用方只能从头
discovery 形成新 request。v1 不因此把完整 module graph、`go.sum` 或 `go.work.sum` 扩展
进 wire request，consumer 只按自身执行需求建立额外 live graph snapshot。graph verifier
对 `TargetModFile` 只固定该精确文件；只有它同时是 active modfile 时，active-graph 规则才
另外固定 matching sum，不能仅因邻近关系隐式加入 sidecar。

XGo 必须在启动 driver 前检查 argv/environment 预算。Unix 把 executable、每个 argv/env
字符串及终止 NUL，加上 `8 * (len(args) + len(env) + 3)` 的 64-bit native pointer 预算，
计入同一个 `128 KiB` 上限。Windows 对 executable 计 `UTF16Len+1`，对每个 arg 计
`2*UTF16Len+3` 的保守 quoting 上界，command line 限制为 `30,000` code units；environment
block 连同每项及最终终止 NUL 限制为 `32,767` UTF-16 code units。任一预算超限统一返回
`ErrDriverArgvTooLarge`，不得截断参数或改用未定义的 request file。

### Graph、target 与 flag policy

对普通 directory/file/package target，XGo 只快照一次 ambient `GOFLAGS` 和
`GOWORK`。它先读取并解析 `GOFLAGS`，再以中性的 `GOFLAGS=-x=false` 查询
`GOWORK`；所有受支持 graph flag 随后都作为独立 argv 传给 Go command，
子进程环境固定同一非空 no-op，`GOWORK` 固定为 canonical go.work path 或
`off`。这条规则适用于 discovery、driver package 校验、driver build、SPX
provenance、source bridge 与 launcher build，避免 ambient flag 在任一阶段
改变 graph。

三类需要分类后拒绝的输入必须区分：

| 输入 | discovery/classification | driver match 后 |
| --- | --- | --- |
| `pkg@version` | 在新临时 module 中使用 `GOWORK=off`、`GOFLAGS=-mod=mod` 下载并解析所请求版本及其 class graph；不得复用 caller graph 的 metadata | 明确报 v1 不支持 `@version`；探针只分类，不执行 driver；未匹配时返回 `NotHandled` |
| `-mod=vendor` | 仅 active main/workspace module metadata 可作为权威来源；无 external class marker 的普通 target 可返回 `NotHandled`；external class metadata 无法由 vendor snapshot 证明时在分类前 fail closed | 任一 driver match 明确报 vendor unsupported；shared codec 接受该值仅用于忠实表达/防御性拒绝 |
| `-overlay` | 使用 overlay view 只判断 target/metadata 是否声明 driver，ordinary target 保持 legacy 行为 | 因 v1 snapshot 只消费 physical filesystem，明确报 overlay unsupported；不得把 overlay 内容交给 driver |

directory 必须只含一个匹配 project file；single-file 必须就是该唯一文件。multi-file 和
包含 `...` 的 pattern 只有在其中存在 driver-backed project 时才报不支持，否则仍由
legacy 路径处理。所有 graph probe 都服从 context cancellation，临时 graph 必须清理。

### 调度状态机

XGo 的 dispatcher 只有以下两条合法路径：

```text
target -> graph -> class metadata -> driver match
                                |-- NotHandled -> legacy GenGo
                                `-- matched -> validate -> build driver -> invoke
```

内部判定必须是三态而不是“错误也算未匹配”：

| 状态 | 含义 | 结果 |
| --- | --- | --- |
| `NoMatch` | 已证明没有被 target/graph 授权的 driver，或本节明确允许的 compatibility probe 把原始输入交还 unchanged legacy | 返回 `NotHandled` |
| `Matched` | target 内匹配的 project metadata 含任意 `driver` declaration，包括未知 protocol | 进入 post-match gates；后续失败全部 terminal |
| `Indeterminate` | 权威 graph/metadata 已建立，但读取、解析、来源、边界或 TOCTOU 校验无法完成 | terminal error，不得降级 |

dispatcher 的规范阶段如下；阶段边界和“不回退”是 MUST，多个 terminal error 同时存在时
选择哪一个诊断不属于 wire identity：

| 阶段 | 必须行为 |
| --- | --- |
| 0. CLI target | 既有 target parser 只运行一次，在 GenGo 或源码解析前把 typed `DirProj`/`FilesProj`/`PkgPathProj` 交给 dispatcher；driver 不重新解释原始 argv |
| 1. Policy snapshot | canonicalize cwd/Go command，只快照一次 ambient GOFLAGS，以 neutral GOFLAGS 查询并固定 GOWORK；除明确 deferred 输入外，取样失败 terminal |
| 2. Target classification | directory/file 先 canonicalize parent 并保留 leaf identity；unsafe leaf 错误只可延迟到 positive match，不能借 symlink 改变 driver 归属 |
| 3. Graph/owner | directory/file 锚定 canonical project dir，package 锚定 caller cwd；owner 是包含项目目录的唯一最深 effective module root，并列 owner 报 ambiguous；读取实际 target modfile identity |
| 4. Driver match | 必须是 target 目录内匹配的 project file 及该 project 自己的 driver declaration；同 module 中其他 project 的 driver 不构成 match |
| 5. Post-match gates | 校验 target form、唯一 project/file identity、来源、XGo 版本、protocol、pack/declaration、禁用/递归 guard 与 deferred flags；任一失败 terminal |
| 6. Driver build | 复验 target modfile，校验 package provenance，再复验；以同 graph 构建并校验 host executable，随后再次复验 |
| 7. Handoff | strict encode、预算检查、spawn 前最后复验 target modfile；consumer 重新绑定 request identity |
| 8. Supervise/finalize | 等待受管 driver/Engine chain；观察取消后不得成功；build/install 只有在成功且未取消时提交 staging |

`FilesProj` 必须按输入顺序逐项分类：只有全部为 `NoMatch` 才能整体 `NotHandled`；任一
terminal error 立即终止，任一 `Matched` 都必须在启动 driver 前把整个 multi-file target
拒绝。driver-backed 直接文件的最终分量必须保持 regular、非 symlink、身份稳定，并与
唯一 project file 是同一文件。recursive pattern 的规范形态是 clean 后等于 `...` 或以
`/...` 结尾；扫描不能跨 nested module，并跳过 symlink directory、`vendor`、`testdata`
以及以 `.`/`_` 开头的目录。

`NotHandled` 是唯一允许回到 legacy 的结果。匹配后发生的 graph、metadata、
version、protocol、driver build、driver exit 或 asset 错误都必须原样终止，不能
再次尝试 GenGo。`XGO_DRIVER=off` 也属于显式错误，不是 fallback。XGo dispatcher
禁止任何嵌套 driver dispatch；SPX 还会拒绝直接嵌套的 SPX driver 调用。

`run` 的顺序是解析、构建临时 driver、编码 argv、启动 driver；stdin/stdout/stderr
直接继承。`build` 和 `install` 先解析所有目标，再创建 staging 或 install 目录；
driver 必须在指定 staging path 产生一个非空、非符号链接的 host executable，目录内
其他文件不参与提交并随私有目录清理。XGo 在
同一 filesystem 上完成最后一次身份检查和 rename 后才提交目标；提交前的任何
失败都不得改变已有输出。XGo 交接的 staging 必须初始不存在，并位于最终目标同目录下
权限为 `0700` 的私有事务目录；最终目标位于 `ProjectDir` 内时，该事务目录也可以位于
项目内。driver 只可写指定的 staging 文件；`final-output` 只描述最终名字，driver 不得
打开、创建、删除、改名或 chmod 该路径，最终目标的全部 mutation 只属于 XGo commit boundary。
`-work=true` 只保留诊断目录，不改变提交语义。

进程边界只有宿主 dispatcher 持有信号订阅，driver/Engine 内部 supervisor 只消费
取消及其 cause。取消一旦被观察，即使子进程在清理时返回 0，也不能报告成功；Unix
保留正常退出码或信号，Windows 使用 Job Object 管理进程树并把中断映射为 130。

`cmd/xgodriver` 的退出合同是：argv/protocol parse 或 live request validation 失败返回
code `2`；acquisition、graph、build、packaging 或其他执行错误在没有更具体 child status
时返回 code `1`；Engine 的正常非零 code 原样返回。Unix 的 signal status 由 wrapper
对自身重发原始 signal（若重发失败才使用 `128+signal`）；Windows 没有 POSIX signal
status，宿主中断返回 `130`。`driverprotocol.Encode/Parse/Validate` 本身只返回 Go error；
producer 的 encode、预算或 pre-spawn identity 失败发生在 driver 启动前，因此不属于
`cmd/xgodriver` 的 code 2。stderr 的命令错误只带一个 `xgodriver: ` 前缀；错误文字不是
wire ABI，调用方不得通过字符串解析错误类别。

匹配 driver 后，以下边界构成最小 revalidation 时序；任何一项失败都是 terminal error，
不得回到 legacy：

| 边界 | 必须仍成立的不变量 |
| --- | --- |
| resolved metadata 导入后 | target modfile 与 declaration 的 canonical path、类型和 SHA-256 仍等于 discovery snapshot |
| driver package 校验、driver build 前后 | effective module identity、driver package provenance、target modfile 和 graph policy 不变 |
| driver spawn 前 | target modfile 再次匹配；argv/environment 未超过平台预算；staging 仍由 XGo 私有控制 |
| SPX 任何 Go query/acquisition 前 | request 的 declaration、target modfile、module source、project/pack 路径通过 live identity 校验 |
| source bridge 与 launcher build 前后 | selection identity 与 required/optional graph-file snapshot 不变；原始 graph 文件未被内部 Go 命令修改 |
| Engine spawn 或 build commit 前 | 已验证的 published 资产与 payload 仍匹配摘要并持有 cache lease；source 资产保持已验证的 regular/non-symlink identity；输出只来自指定 staging |

这里要求的是身份边界，而不是给整个源码树加全局锁。v1 不承诺阻止普通 Go 编译期间的
并发源码编辑；它只在实际定义 metadata、graph、bundle 和提交结果的边界上 fail closed。

### XGo 版本判定

声明模块的 `xgo` directive 与 `driver v1` 协议基线独立取最大值。当前基线是
`1.8.0`。从源码或 workspace 直接构建的 XGo，Go build info 可能显示标准
pseudo-version（例如 `v1.2.0-pre.1.0.20260821130422-831eec0b6b4e`）；这类
版本表示开发构建，按 driver capability `1.8.0` 比较，但错误信息仍显示原始
版本和 capability。真正的 release prerelease（如 `v1.8.0-rc1`）不自动视为开发
构建。该规则只适用于 XGo 自身能力检查，不改变 SPX published mode 对 pseudo
SPX module version 的拒绝。

### SPX mode 与环境输入

SPX 只在以下身份使用 source mode：main module、当前 workspace module、或无版本
local replacement。其他 canonical `github.com/goplus/spx/v3@vX.Y.Z[-prerelease]` 且无
replacement 的 graph 使用 published mode；canonical prerelease 可以使用自身 exact tag
的资产，pseudo-version、versioned replacement 与 foreign module 必须 fail closed。

source bridge 的 build info 必须把 effective SPX module 记为 main module；生成的
launcher 必须把它记为 dependency。对 source/workspace 中的 main module，Go 生成的
main build-info version（包括标准 pseudo-version）只用于诊断，不参与身份比较；其身份
由 module path 与已校验的 graph/source snapshots 固定。launcher 中该 workspace
dependency 的 version 必须为空或 `(devel)`，避免误连 versioned SPX source。

source mode 的运行时输入优先级固定为：显式 local runtime manifest -> 已校验
runtime release/cache（或 `SPX_RUNTIME_ASSET_DIR` mirror）-> 在线获取（offline 时跳过）
-> release 确实不可用时的 exact-version source/GOPATH runtime fallback。bridge package
固定来自 effective SPX source，不能由环境改成其他 package，并以 host
`CGO_ENABLED=1` 构建。

显式 local runtime manifest 必须完整匹配当前 lock；自动发现的 exact-version
source/GOPATH manifest 只绑定 runtime version、host 与实际文件内容，不伪造 release
provenance。只有 release unavailable 或 offline cache miss 可以进入 source/GOPATH
fallback；取消、manifest 格式、version/size/digest、显式 local/mirror 错误都必须终止。
指定 `SPX_RUNTIME_ASSET_DIR` 时 manifest 必须来自 mirror；已有 exact-digest cache hit
可以满足资产读取，cache miss 才读取 mirror，mirror 失败不转向网络或 GOPATH。相对地，
`SPX_DRIVER_ASSET_DIR` 的 manifest 与 bundle 都必须从该显式 mirror 实际读取并校验，warm
cache 不能隐藏缺失或损坏。

published mode 只接受 combined driver bundle。programmatic `launchpack.Config` 中的
runtime source root、runtime manifest path、runtime asset directory 或 source bridge
package 与该模式冲突，必须拒绝。继承的 `SPX_RUNTIME_LOCAL_MANIFEST` 与
`SPX_RUNTIME_ASSET_DIR` 在 published mode 中均忽略，既不参与选择，也不因存在或重复而
报错。本机/GOPATH bridge 同样永不参与 published selection，但无需为无关的
ambient 环境变量额外报错。唯一允许的本地发布镜像是 `SPX_DRIVER_ASSET_DIR`，且必须
同时提供并通过 exact manifest/bundle 校验；显式 mirror 缺失或损坏时必须失败，不能
被 warm cache 或网络隐藏。

相关环境变量如下：

| 变量 | 语义 |
| --- | --- |
| `SPX_RUNTIME_LOCAL_MANIFEST` | source mode 指定并严格校验 local runtime manifest；失败不回退；published mode 忽略且不参与选择 |
| `SPX_RUNTIME_ASSET_DIR` | source mode 指定 runtime release mirror；内容和 manifest 必须匹配；published mode 忽略且不参与选择 |
| `SPX_DRIVER_ASSET_DIR` | published mode 的 `driver-manifest.json` 与 host ZIP mirror，必须是 absolute clean path；source mode 不把它当 runtime 输入 |
| `SPX_RUNTIME_CACHE` | 绝对 clean 的 content-addressed cache 根目录 |
| `SPX_RUNTIME_OFFLINE` | `1/true/yes/on` 时禁止网络，只接受完整校验的 cache/local hit |
| `GOWORK` / `GOFLAGS` | XGo 把 request 固定为 canonical workspace path/`off`；SPX 快照原文件，并让自身 Go 子进程使用语义等价的私有 workspace；graph/build flags 只走 argv，且固定中性的 `GOFLAGS=-x=false` |

`ProjectDir`、`AssetDir`、`SessionDir` 三根目录职责不能互换；用户参数不能覆盖
driver 写入的 `SessionDir --path`。`.config`、pack index、module metadata 和
release manifest 都按“快照后再校验”的规则消费。“项目只读”表示 SPX-owned
driver/scaffold/cache/build 操作只可写 XGo 指定的 staging 文件，不得写项目其他路径；
v1 不提供阻止 Engine 或用户应用逻辑自行写文件的 OS sandbox。

SPX graph verifier 从结构化 `go list -m -json all` 结果生成确定性的 selection
identity，并快照 effective active modfile（默认 `go.mod` 或显式 `-modfile`）及其
matching sum、request workspace 文件及其 `<GOWORK>.sum` sidecar，以及每个 workspace main module 和无版本 local
replacement（包括 local SPX module）所报告的 `go.mod/go.sum` 的存在状态和内容摘要。
source bridge build 与 launcher build 都在
关键命令前后重做选择和文件快照；selection 或任一 required/optional graph file 的
内容、出现/消失或身份变化都终止。project-driver v1 固定 module selection 与 graph
metadata，而不是 local module source tree 的每个字节；并发源码修改仍遵循普通 Go build
语义。workspace mode 下，SPX 创建稳定的私有副本，把 relative `use` 和 workspace-level
local `replace` 按原 workspace 目录解析，并让自身全部 Go command 使用该副本；原 workspace
与 sum 继续作为 verifier 输入，Go 必须执行的 workspace-sum 更新只落到私有副本。用于
graph/provenance/launcher 的 host Go 环境固定 host `GOOS/GOARCH`、`CGO_ENABLED=0`、
中性的 `GOFLAGS=-x=false` 与私有 workspace path 或 `off`；非空值会覆盖 GOENV 中的
持久化设置。source bridge
额外移除继承的 `CGO_*` 后固定 `CGO_ENABLED=1`。Engine 进程不继承这些 Go graph/target
变量。

driver package 与公开 `github.com/goplus/spx/v3/x/xgolauncher` package 都必须在同一
graph 下校验完整 `ResolvedModule`。absolute local replacement 用文件身份等价比较；
relative replacement 保留并比较同一 graph 的 `go list -m` spelling。bridge build info
中的 SPX 必须是 main，launcher 中必须是 dependency；main/workspace 的 build-info
version 与 dirty VCS 只用于诊断。published direct run 在 package origin 已确认且不再执行
Go command 后无需持续 graph verifier；source bridge 与所有 launcher build 仍必须在关键
命令前后验证。

私有 graph 的构造算法属于 v1 合同：

- workspace mode 必须稳定读取原始 `go.work` 与可选 `<go.work>.sum`，写入私有副本，
  并以原 workspace 目录为基准把 relative `use` 和 workspace-level local `replace`
  改成绝对路径；所有后续 Go command 固定使用该私有 `GOWORK`。
- `GOWORK=off` 且 `-mod=mod` 时，必须先解析显式 `-modfile` 或 active `GOMOD`，稳定复制
  modfile 与 matching sum 为私有 `graph.mod/graph.sum`；relative local replacement
  必须以 active module root（中性查询所得 `GOMOD` 的目录，而非外置显式 modfile 的目录）
  改成绝对路径，再替换或追加唯一的 `-modfile=<private graph.mod>`。
- `GOWORK=off` 的 `-mod=readonly` 不需要私有可写 modfile，但 workspace 仍执行上一条的
  私有复制；两者都受相同 snapshot/verifier 约束。不得为了项目只读把调用者明确选择的
  `-mod=mod` 改写成 readonly。
- 原始 mod/work 与 sum 始终是 verifier 输入。只有 Go 针对私有副本报告的 main-module
  `GoMod` 可以映射回 request 中的原路径，而且必须同时满足 private path、`Main=true`、
  module path 和 directory 全部匹配；其他 provenance 字段不得归一化或忽略。
- 私有 workspace/modfile 及其 sum 在成功、失败和取消路径都必须清理；`-work=true` 可以
  保留 bridge/launcher/session 诊断目录，但不把内部 graph 副本变成公开诊断合同。

### 兼容性与协议演进

v1 是封闭 schema，不存在隐式 minor version negotiation。以下变化必须同时使用新的
metadata `driver vN` 与 wire preamble `xgo-driver-vN`：增加 action，增加/删除字段，改变
字段含义、基数、empty-value 规则、摘要公式、unknown/duplicate 行为或 accepted-frame
集合，允许新的 graph/build flag，或改变 run/build 的事务与进程合同。因为 v1 consumer
必须拒绝未知字段，producer 不得向 v1 frame 试探性加入 option；能力协商发生在 metadata
protocol 值，而不是 argv 中。

本节列出的 canonical frame（包含 required target-modfile identity）是 v1 的 freeze
point；此前缺少这些字段的实验性 producer/consumer 不在兼容范围内。含 codec 的 mod、
XGo producer 与 SPX consumer 必须按“先发布 mod，再提升 XGo/SPX 依赖”的协调顺序形成
同一个兼容集合。混用实验性新旧端不受支持；当前受支持 dispatcher 在 driver match 后
必须 fail closed，不得猜测缺失身份或回退 GenGo。

不改变可观察合同的实现修复可以保留 v1，例如补足漏掉的 snapshot revalidation、让错误
更早发生，或在不改变选择优先级的前提下重构缓存。decoder 接受不同 option 顺序只是传输
层兼容，不允许重复 singular 字段、忽略未知字段或接受非 canonical flag spelling。

三仓 conformance test 必须共同覆盖同一种 golden request shape：mod codec 固定 exact
canonical argv 与 round trip，XGo producer 断言其生成顺序和预算，SPX consumer 断言解析与
live validation；测试集合覆盖 order-independent parse、缺失/重复/未知字段、两个来源分支、
run delimiter、build output 与 graph/build flag，不要求复制一份跨仓测试夹具。任何协议
变更只有在 mod codec、XGo producer、SPX consumer 与本节同时更新并通过本地 workspace
测试后才算完成。

### Cache 与 lease 合同

- cache key 必须是 `namespace + full digest`；不同 namespace 或不同完整摘要不得共享
  entry，不能用截断摘要、文件名或 interface digest 代替。
- production cache 必须使用跨进程 shared/exclusive lock。publish 与 repair 持有
  exclusive lock；最后一次读取或 Engine 使用结束前持有 shared lease。exclusive 转为
  shared 后必须重新校验 entry，不能假设锁转换期间内容未变。
- entry 只能在 sibling temp 中完整写入、校验并同步，再通过同文件系统 rename 发布。
  失败或取消不能形成 valid cache hit；正常错误应该尽力删除 temp，进程崩溃留下的 orphan
  temp 也不得被识别为 entry。
- 每个 cache hit 重新校验对应 manifest、类型、大小和 SHA-256。未来 GC 必须服从 lease，
  不能删除正在运行的组件；v1 可以不实现容量配额或自动 GC。
- lease 只协调遵守协议的进程，不是抵御同 UID 不合作写入者的 OS sandbox；published raw
  manifest 的本地信任根边界仍以前述威胁模型为准。

### Published driver manifest 与 host ZIP

`driver-manifest.json` 最大 `16 MiB`，使用 strict JSON：未知字段、重复 key、尾随值、
错误类型都拒绝。完整 schema 如下；数组顺序属于 v1 合同：

```json
{
  "schema": 1,
  "spx_version": "vX.Y.Z",
  "runtime_version": "R",
  "bundles": [
    {
      "goos": "darwin",
      "goarch": "amd64",
      "name": "spx-driver-darwin-amd64.zip",
      "size": 1,
      "sha256": "<64 lowercase hex>",
      "engine_interface_digest": "<64 lowercase hex>",
      "files": [
        {"name": "gdspxrtR", "mode": 493, "size": 1, "sha256": "<64 lowercase hex>"},
        {"name": "gdspxrtR.pck", "mode": 420, "size": 1, "sha256": "<64 lowercase hex>"},
        {"name": "gdspx-darwin-amd64.dylib", "mode": 493, "size": 1, "sha256": "<64 lowercase hex>"}
      ]
    }
  ]
}
```

实际 `bundles` 必须恰好四项并按以下顺序出现；上例单项只是字段示意：

| 顺序 | target | ZIP | files，严格按 Engine/PCK/bridge 顺序 |
| --- | --- | --- | --- |
| 1 | `darwin/amd64` | `spx-driver-darwin-amd64.zip` | `gdspxrtR` `0755`; `gdspxrtR.pck` `0644`; `gdspx-darwin-amd64.dylib` `0755` |
| 2 | `darwin/arm64` | `spx-driver-darwin-arm64.zip` | `gdspxrtR` `0755`; `gdspxrtR.pck` `0644`; `gdspx-darwin-arm64.dylib` `0755` |
| 3 | `linux/amd64` | `spx-driver-linux-amd64.zip` | `gdspxrtR` `0755`; `gdspxrtR.pck` `0644`; `gdspx-linux-amd64.so` `0755` |
| 4 | `windows/amd64` | `spx-driver-windows-amd64.zip` | `gdspxrtR.exe` `0755`; `gdspxrtR.pck` `0644`; `gdspx-windows-amd64.dll` `0755` |

其中 `R` 是不带前导 `v` 的 `runtime_version`。所有 size 必须为正数并与实际字节数
相等；任一 runtime/driver archive 的 manifest size 还必须不超过 `8 GiB`，consumer
必须在 mirror/cache/network asset fetch 前拒绝超限 manifest，不能等下载或写盘后再由
ZIP extractor 拒绝。bundle SHA-256 覆盖整个 ZIP，file SHA-256 覆盖解压后的文件字节。
`engine_interface_digest = SHA256(ASCII("spx-engine-interface/v1") || 0x00 ||
hexDecode(engine.sha256) || hexDecode(pck.sha256))`。`spx_version` 必须等于 graph 选择的
含前导 `v` 的 exact canonical module version（允许 canonical prerelease，但不允许
pseudo-version），`runtime_version` 必须等于 SPX runtime lock；release tag 与
`spx_version` 字符串完全相同，driver asset URL 使用这个 SPX Release，不创建或引用
`driver-<spx_version>` 独立 Release。

release packager 必须按 Engine/PCK/bridge 顺序写恰好三个 regular entry，使用 ZIP
`Store`、UTC `1980-01-01T00:00:00Z`、上表 mode、portable basename，不写 directory、
extra、duplicate 或 symlink entry。相同三个输入必须产生相同 ZIP bytes。consumer 还
按 manifest 的 exact archive size/SHA-256 和逐文件 name/mode/size/SHA-256 校验，不得
仅凭文件名或 interface digest 建立信任。

### Archive 与 payload 限额

所有限额在读取/解压或启动 launcher build 前 fail closed；不能通过压缩、ZIP64 或
manifest 声明绕过：

| 对象 | v1 限额 | 确定性参数 |
| --- | --- | --- |
| runtime/driver ZIP verifier | 最多 `10,000` entries；单 entry `512 MiB`；解压总计 `4 GiB`；archive `8 GiB`；compression ratio `200:1`。driver ZIP 另要求恰好 3 files | SPX Release 中的 driver ZIP 为 `Store`、1980 epoch、canonical mode/order；manifest 固定完整 archive/file digests |
| canonical project ZIP | 最多 `10,000` files；单文件 `64 MiB`；输入总计 `256 MiB`；archive `512 MiB` | UTF-8 slash path 字节序排序，Deflate `BestCompression`，1980 epoch，所有 entry `0644` |
| embedded runtime payload ZIP | 含 top-level manifest 少于等于 `10,000` entries；单 entry `512 MiB`；总计 `4 GiB`；archive `8 GiB`；payload manifest `1 MiB` | entry name 排序，`Store`，1980 epoch，可执行 `0755`、其他 `0644`；payload 与 manifest 均固定 SHA-256 |

runtime/driver ZIP 与 embedded payload 的 untrusted-archive verifier 拒绝
absolute/`..`/backslash traversal、NUL、非 UTF-8、duplicate、Unicode
normalization/case-fold collision、file-as-parent、重叠 data range、encrypted entry、
symlink/device/special entry。project ZIP 是受约束的 producer：它在打包前对 allowlist
输入施加对应的 portable-path、collision、regular non-symlink file 与大小规则，而不是
把任意外部 ZIP 当输入。多阶段 project/payload snapshot 在实现有双读/identity check 的
边界拒绝变化；driver bundle packager 则固定已打开 regular file 所读字节的 size/SHA-256，
后续 acquisition 必须与这些 digest 完全相等。project ZIP 的完整字节作为
`project/project.zip` 以 `Store` 内嵌，不二次改写。

## 验证计划

本地联调必须使用同一份 workspace，确保 shared codec、XGo dispatcher 和 SPX
driver 不落回 module cache 中的旧版本。推荐流程（`CODE` 为三个仓父目录）：

```sh
integ=$(mktemp -d)
(cd "$integ" && GOWORK=off go work init \
  "$CODE/mod" "$CODE/xgo" "$CODE/spx")

(cd "$CODE/mod" && GOWORK="$integ/go.work" \
  go test ./driverprotocol ./modfile ./modload ./xgomod)
(cd "$CODE/xgo" && GOWORK="$integ/go.work" \
  go test ./cmd/internal/projectdriver)
(cd "$CODE/spx" && GOWORK="$integ/go.work" \
  go test ./internal/driverbundle ./internal/envutil \
    ./internal/xgodriver ./internal/launchpack ./cmd/xgodriver)
```

联调 workspace 只用于读取；不要执行 `go work sync` 后提交被回写的各仓
`go.mod/go.sum`。CLI smoke 必须从 XGo 仓构建同一 workspace 中的临时 `xgo`，再对
真实 SPX fixture 执行 `run`、`build` 和独立运行 launcher。dirty checkout 可能在 Go
build info 中记录 VCS 状态，但当前 provenance 只比较 module path/version/replacement，
不把 dirty 状态当作拒绝条件。联调仍建议显式传 `-buildvcs=false`，以排除环境差异并
得到可重复的身份结果，而不是增加一套 VCS 严格校验。下面第一次 run 验证 cache miss，
第二次再以 offline mode 验证同一份已校验 cache：

```sh
(cd "$CODE/xgo" && GOWORK="$integ/go.work" \
  go build -o "$integ/xgo" ./cmd/xgo)
(cd "$CODE/spx" && GOWORK="$integ/go.work" \
  SPX_RUNTIME_CACHE="$integ/cache" SPX_RUNTIME_OFFLINE=0 \
  "$integ/xgo" run -buildvcs=false ./test/CI --headless)
(cd "$CODE/spx" && GOWORK="$integ/go.work" \
  SPX_RUNTIME_CACHE="$integ/cache" SPX_RUNTIME_OFFLINE=1 \
  "$integ/xgo" run -buildvcs=false ./test/CI --headless)
(cd "$CODE/spx" && GOWORK="$integ/go.work" \
  SPX_RUNTIME_CACHE="$integ/cache" SPX_RUNTIME_OFFLINE=1 \
  "$integ/xgo" build -buildvcs=false -o "$integ/spx-ci" ./test/CI)
"$integ/spx-ci" --headless
```

交互式教程可用同一方式验证：对 `./tutorial/05-Animation` 执行 `xgo run`，确认 Engine
启动后发送 `SIGINT`；再 `xgo build -o "$integ/animation"` 并独立启动该 launcher。
两条运行路径都应返回 130、清理 Engine/driver 子进程，且教程 tracked 文件摘要不变。

包含 codec 的 mod 版本发布并完成依赖提升后，还必须在 XGo 与 SPX 仓分别执行相关
`GOWORK=off go test`，避免本地三仓 workspace 掩盖未闭环依赖。

`xgo run` 与独立 launcher 都必须输出 `SPX_CI_TEST_OK`，`xgo build` 必须成功产生
非空 host executable。完整验收还必须覆盖：普通项目 legacy 不变、source mode 的
main/workspace/local replace、published bundle 的 cache miss/hit/offline/并发/
kill-recovery、argv/stdin/信号、原子 build/install、同大小篡改，以及 Darwin、
Linux、Windows 的真实 host artifact。发布顺序固定为 runtime -> 构建并校验 driver 资产 -> 将资产并入 tag 恰为 `spx_version` 的 Release；缺少任一前置产物时不得公开 module tag。

## 附录：最小端到端例子

下面用一个外部 `hello-spx` 项目展示 v1 协议的最短完整路径。示例使用 SPX `v3.2.4`
和 runtime lock `2.4.4`；实际使用时两个版本都必须由
有效 graph 和 lock 分别确定，不能从文件名推断。

### 1. 项目输入

应用目录如下：

```text
hello-spx/
├── go.mod
└── game/
    ├── main.spx
    └── assets/index.json
```

应用只需要在 `go.mod` 中标记 class dependency：

```go
module example.com/hello

go 1.25

require github.com/goplus/spx/v3 v3.2.4 //xgo:class
```

SPX module 自己的 `gox.mod`（不复制到应用目录）声明 project 和 driver：

```text
project main.spx Game github.com/goplus/spx/v3 math
driver v1 github.com/goplus/spx/v3/cmd/xgodriver
pack assets index.json
```

### 2. `xgo run` 的调用

用户执行：

```sh
xgo run ./game -- --level 2
```

这里的 `./game` 是相对于当前工作目录的 directory target，目录中必须能唯一确定
`main.spx`。命令行中的第一个 `--` 是 XGo CLI 的 target/应用参数分隔符；`--level`
和 `2` 只是示例应用参数，不是 XGo 或 SPX v1 的固定选项。没有应用参数时可直接执行
`xgo run ./game`。

XGo 生成的真实 argv 还包含 declaration 摘要、Go command、workspace 和 graph/build
flags；下面只保留能说明协议的字段：

```text
xgo-driver-v1 run
  --project-dir=/work/hello-spx/game
  --project-file=/work/hello-spx/game/main.spx
  --module-root=/work/hello-spx
  --driver-package=github.com/goplus/spx/v3/cmd/xgodriver
  --selected-path=github.com/goplus/spx/v3
  --selected-version=v3.2.4
  --pack-dir=assets
  --pack-index=index.json
  ...
  --
  --level
  2
```

用户命令中的分隔符由 XGo CLI 消费后，XGo 在生成 driver argv 时会再次写入协议层的
`--`。因此上面的协议帧最终把应用参数表示为两个独立元素
`[]string{"--level", "2"}`；协议层分隔符之后的参数按元素边界原样交给 Engine。协议
不使用 request 文件，也不占用 stdin。

由于这里是 canonical SPX module 的 exact version，且没有 replacement，SPX 判断为
published mode，读取同一个 `v3.2.4` SPX Release 中的：

```text
https://github.com/goplus/spx/releases/download/v3.2.4/driver-manifest.json
https://github.com/goplus/spx/releases/download/v3.2.4/spx-driver-darwin-arm64.zip
```

manifest 必须声明 `spx_version = "v3.2.4"`、`runtime_version = "2.4.4"`，并且
host ZIP 必须恰好包含以下三个文件，随后按 manifest 的 archive/file 大小与 SHA-256
校验：

```text
gdspxrt2.4.4
gdspxrt2.4.4.pck
gdspx-darwin-arm64.dylib
```

校验通过后，driver 将资产物化到 cache/session 并启动 Engine；项目源目录保持不变。

### 3. `xgo build` 的调用

用户执行：

```sh
xgo build -o hello ./game
```

协议 action 变为 `build`，XGo 为 driver 分配私有 staging：

```text
xgo-driver-v1 build
  ...
  --output=/work/hello-spx/.xgo-driver-output-<random>/hello
  --final-output=/work/hello-spx/hello
```

driver 只能写 `--output` 指定的 staging 文件。SPX 在其中生成包含 Engine、PCK、
bridge 和 canonical project bundle 的 launcher；成功后由 XGo 在同一文件系统上原子
提交到 `--final-output`。因此最终得到的是一个当前 host 的 Mach-O/ELF/PE，而不是需要
用户手工组合的三个文件。

### 4. Release 资产关系

SPX `v3.2.4` 的 GitHub Release 可以包含：

```text
v3.2.4
├── driver-manifest.json
├── spx-driver-darwin-amd64.zip
├── spx-driver-darwin-arm64.zip
├── spx-driver-linux-amd64.zip
├── spx-driver-windows-amd64.zip
├── SHA256SUMS
└── 其他 SPX 产品资产
```

每个 driver ZIP 是可独立下载、缓存和校验的 Release asset，但不是另一个
`driver-v3.2.4` GitHub Release。这样，module `v3.2.4`、driver 资产和下载入口具有
同一个发布身份；只有所有 runtime/driver 资产校验完成后，canonical SPX module tag
才公开。

如果应用把 SPX 通过无版本 `replace` 指向本地 checkout，例如：

```text
replace github.com/goplus/spx/v3 => /path/to/spx
```

其余协议帧不变，但来源身份变为 source mode：driver 从同一份有效 graph 构建 bridge，
不会把本地 module cache 当作 published mode，也不会借用 published driver 资产。
