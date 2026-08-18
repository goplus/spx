# [Proposal] XGo Project Runtime Provider v1 与 SPX 解释运行/单文件构建集成

> 建议提交位置：`goplus/xgo`，并拆分关联的 `goplus/mod`、`goplus/spx` 子 issue/PR。
>
> 状态：Proposed
>
> 范围：XGo、goplus/mod、SPX 三仓协同变更
>
> Supersedes: [goplus/mod#142](https://github.com/goplus/mod/pull/142) 及已撤回的 `Runner` 设计
>
> 设计输入：[XGo × SPX 本地解释运行时集成设计（29 轮审查修订版）](https://chatgpt.com/share/6a826233-44b0-83ec-b9d5-309d4819d908)

## 摘要

为 class project 增加 framework-owned、project-scoped、版本化的 runtime provider。XGo 在解析 target 后、执行任何 GenGo 之前发现并构建 provider，再将 `run` 或 `build` 委托给它；`install` 复用 `build`。SPX provider 负责解释器、Engine Runtime、项目资源和单文件 launcher，XGo 内部不包含任何 SPX/Godot 特判。

本 proposal 接受原方案主线，但不按原稿直接实现。以下条件是正式启用 SPX `gox.mod` 中 `runtime` 指令的前置门槛：

1. class metadata 与 provider 必须来自同一份有效 Go module/workspace graph；
2. provider protocol 必须在持久 metadata 中显式版本化；
3. XGo 必须在所有 target 分支和 GenGo 之前统一分发；
4. SPX 必须先解耦 `ProjectDir`、`AssetDir`、`SessionDir`；
5. SPX 必须发布与 SPX 版本绑定的 interpreter bridge，并发布带 interface digest 的新 Engine Runtime；
6. engine/bridge/project bundle 必须完成逐项摘要、并发锁、原子发布和不可信 ZIP 防护；
7. flags、argv、stdin、退出码、信号、输出替换均采用 fail-closed 契约。

v1 是一次协调升级，不承诺兼容旧 XGo。旧 XGo 可能因 `ParseLax` 静默忽略独立 `runtime` 指令并继续旧路径；这是已知限制，不是 v1 blocker。SPX 只在 mod schema、XGo 分发和 SPX 发布物全部就绪后才启用该指令。

## 审查基线与结论

审查日期：2026-08-17。

| 仓库 | 审查基线 | 结果 |
| --- | --- | --- |
| `goplus/xgo` | [`f53e0ce`](https://github.com/goplus/xgo/commit/f53e0ce4080bf622f378e841d1f401ce1ed8792e) | focused tests 通过，worktree clean |
| `goplus/mod` | [`f4300a7`](https://github.com/goplus/mod/commit/f4300a72541225debcd0de44e166ee7213d54f8d)（`v0.22.0`） | `go test ./...` 通过，worktree clean |
| `goplus/spx` | [`766f5d5`](https://github.com/goplus/spx/commit/766f5d514baef0640149ad9aaad72f048ccb4e1d) | focused tests 通过，worktree clean |

原方案中应保留、修订和删除的部分如下。

| 原方案主张 | 评审决定 | 原因/修订 |
| --- | --- | --- |
| framework 在 `gox.mod` 声明 provider | 保留 | XGo 保持通用，生命周期归 framework |
| provider 版本来自应用 module graph | 保留并强化 | metadata、provider package、source/release mode 必须使用同一有效 graph |
| 在 CLI 的 GenGo 前分发 | 保留并前移 | 必须位于 `ParseOne/ParseAll` 后、Dir/PkgPath/Files switch 前；v1 不改变直接调用 `tool.RunDir/BuildDir/InstallDir` 的语义 |
| `runtime <package>` | 修订 | 改为 `runtime v1 <package>`，`v1` 是协议代际，不是 module version |
| 旧 XGo 会提示升级 | 删除为 v1 要求 | 现有主/依赖 `gox.mod` 都经 `ParseLax`，且 `xgo` 行没有最低版本门禁 |
| 传递“原始 build flag tokens” | 修订 | 当前 XGo 已规范化成 `-name=value`；v1 对每个现有 flag 明确支持或拒绝 |
| `xgo run target -- --headless` | 修订 | 正确用户语法是 `xgo run target --headless`；协议内部的 `--` 不暴露给用户 |
| standalone 已有可直接复用的 SHA/原子 materializer | 否定 | 当前 cache hit 只比较 size、仅进程内锁、逐文件 rename，不能处理不可信 payload |
| 当前 runtime release 已含 interpreter bridge | 否定 | release 只有 Engine/PCK 类资产；bridge 目前由 standalone 构建流程现场编译 |
| 现有 `PackProject` 可直接复用 | 否定 | 它遍历范围过宽、先删除输出、缺少 symlink/size/collision 防护 |
| 任意 temp session 只需 `SPX_PROJECT_DIR` | 否定 | SPX 资源根仍通过相对 `../assets` 推导，必须同时解耦三类根目录 |
| 复制 provider 后追加 ZIP/footer | 否定 | Apple 明确禁止向 Mach-O executable 追加数据；统一改为 build-time `go:embed` payload，Darwin 在 link 后显式 ad-hoc sign，签名后不再修改文件 |
| provider 失败后可回退 native/GenGo | 否定 | 一旦匹配 runtime metadata，任何协议、构建或执行错误都必须 fail closed |

关键代码事实：

- XGo 当前对 Dir、PkgPath、Files 分别调用不同路径，见 [`run`](https://github.com/goplus/xgo/blob/f53e0ce4080bf622f378e841d1f401ce1ed8792e/cmd/internal/run/run.go#L63-L111)、[`build`](https://github.com/goplus/xgo/blob/f53e0ce4080bf622f378e841d1f401ce1ed8792e/cmd/internal/build/build.go#L69-L109) 和 [`install`](https://github.com/goplus/xgo/blob/f53e0ce4080bf622f378e841d1f401ce1ed8792e/cmd/internal/install/install.go#L62-L101)。
- 现有 `RunDir/BuildDir/InstallDir` 都先 GenGo，见 [`tool/build_install_run.go`](https://github.com/goplus/xgo/blob/f53e0ce4080bf622f378e841d1f401ce1ed8792e/tool/build_install_run.go)。
- v1 的 provider dispatch 是 `cmd/xgo` 的 CLI 能力；直接使用 `tool.RunDir/BuildDir/InstallDir` 的调用方继续沿用 legacy GenGo 路径。若要把 provider 暴露为 public tool API，应另行设计可返回 status/error 且可管理 provider 生命周期的新接口，不能静默改变现有函数语义。
- mod loader 对主模块和依赖模块均使用 `ParseLax`，见 [`modload/module.go`](https://github.com/goplus/mod/blob/f4300a72541225debcd0de44e166ee7213d54f8d/modload/module.go#L215-L230)。
- `xgomod.ImportClasses` 当前丢失声明 project 的 module provenance，且相同 extension 可被覆盖，见 [`xgomod/classfile.go`](https://github.com/goplus/mod/blob/f4300a72541225debcd0de44e166ee7213d54f8d/xgomod/classfile.go#L92-L155)。
- SPX bridge 当前从 cwd 的父目录推导项目根，见 [`cmd/ispxnative/main.go`](https://github.com/goplus/spx/blob/766f5d514baef0640149ad9aaad72f048ccb4e1d/cmd/ispxnative/main.go#L30-L48)。
- SPX embedded runtime cache 只按 size 接受已有文件，见 [`runtimeasset.go`](https://github.com/goplus/spx/blob/766f5d514baef0640149ad9aaad72f048ccb4e1d/cmd/spx/internal/runtimeasset/runtimeasset.go#L182-L251)。

## 目标

用户侧目标命令：

```bash
xgo run ./game --headless
xgo build -o ./bin/game ./game
xgo install ./game
```

完成后应满足：

- XGo 不认识 SPX、Godot、PCK 或 bridge；
- 应用只通过 `require ... //xgo:class` 和 framework 的 `gox.mod` 获得能力；
- `xgo run` 不生成或修改 `xgo_autogen.go`，不在项目中创建 `.temp`、`.godot` 或 `.builds`；
- `xgo build` 产生一个 host 平台可执行文件；
- 该可执行文件在没有 Go、XGo、SPX 且无网络的机器上可运行；
- provider、framework 源码、bridge 和 Engine Runtime 的身份可追踪且不会跨 graph/ABI 混用；
- 普通非-runtime XGo 项目的 run/build/install 行为不变；vendor class project 只有在 preflight 已确认命中 runtime 后才返回明确的 unsupported 错误。

## v1 明确不做

- 不保证旧 XGo 的友好升级提示或 fail-closed；
- 不支持 `xgo test`、Web、Android、iOS；
- 不支持 `GOOS/GOARCH` 跨目标构建，只支持 host desktop；
- 不保证任意第三方 Go import；仅保证当前 ispx embedded package surface，不支持时明确报错；
- 不静默回退到 native/GenGo；
- 不提供 provider lifecycle/execution SDK，也不引入 JSON request/schema；`goplus/mod/runtimeprotocol` 只提供无 I/O 的 typed request 与唯一 argv codec；
- 不承诺 Developer ID、notarization 或 Authenticode；Darwin v1 仍必须在 link 后显式生成并校验 ad-hoc signature；
- 不承诺整个 launcher bit-for-bit reproducible；project/runtime payload 必须 canonical；
- 不提供源码保密，`.spx` 源码可从 launcher payload 中提取；
- 不建立 XGo 持久 provider binary cache；v1 依赖 Go build cache 并做进程内复用。

## 术语与唯一版本真相

必须区分四组身份：

1. **Provider Protocol**：本 proposal 的 wire protocol `v1`，由 XGo 与 provider 共同实现；
2. **XGo Version**：正式构建沿用既有 `env.Version()`；无法比较的 development build 保留真实 build identity，并只在 runtimeprovider 内使用私有 capability baseline 做 `RequiredXGo` 比较；
3. **SPX/Interpreter**：由应用有效 Go graph 选择，例如 `github.com/goplus/spx/v3@v3.x.y` 或 local replace/workspace source；
4. **Engine Runtime/ABI**：由所选 SPX 源码中的 runtime lock/release metadata 选择。

Protocol、XGo build version、SPX module identity 与 Engine ABI 互不推导。runtimeprovider 的 development capability baseline 不是新的公共 XGo 版本 API，也不改变 release linker flags、`env.Version()` 或发布物身份；SPX/Engine 发布物继续记录各自真实的 source/build provenance。

`gox.mod` 不得再声明 provider module version、SPX version 或 Engine Runtime version。禁止 `path@version`。应用的有效 Go graph 是 provider 代码版本的唯一真相，SPX runtime lock 是 Engine Runtime/ABI 的唯一真相。

## 1. `goplus/mod` metadata

### 语法

```text
xgo 1.8.0

project main.spx Game github.com/goplus/spx/v3 math
class -embed *.spx SpriteImpl
pack assets index.json
runtime v1 github.com/goplus/spx/v3/cmd/xgoruntime
```

`runtime` 作用于它前面最近的 `project`，与 `class`、`pack` 的 project scope 规则一致。每个 project 最多一个 runtime。

推荐 parser 数据模型：

```go
type Runtime struct {
    Protocol string // syntactically vN; XGo in this proposal supports v1
    Package  string // Go command import path, never path@version
    Syntax   *Line
}

type Project struct {
    // existing fields...
    Runtime *Runtime
}
```

Parser 规则：

- 必须恰好有两个参数：protocol 与 package；
- parser 接受并保存 `v[1-9][0-9]*`；本 proposal 的 XGo consumer 只支持 `v1`，其他 protocol 明确报 unsupported，不得降级；
- package 使用 `golang.org/x/mod/module.CheckImportPath` 校验；
- 拒绝相对/绝对路径、`@version`、leading dash、反斜线、`..`、多余 token；
- project 前出现 runtime、同 project 重复 runtime、singleton block 形式均报错；
- 已知但 malformed 的 runtime 在 `Parse` 和 `ParseLax` 下都报错；只有真正未知 directive 才能被 lax 忽略；
- formatter 必须 `Parse -> Format -> Parse` round-trip 保留该行、注释和 `Syntax`。

### provenance API

保留现有 `LookupClass`，新增不破坏 API 的 resolved-graph 输入与完整查询：

```go
type ModuleRef struct {
    Path    string
    Version string
    Dir     string // physical source; populated only when this ref is effective
    GoMod   string // same rule as Dir
}

// ResolvedModule mirrors the logical selection and optional effective
// replacement returned by `go list -m -json` without flattening them.
type ResolvedModule struct {
    Selected ModuleRef
    Replace  *ModuleRef
    Main     bool
}

type ResolvedClassGraph struct {
    Target        ResolvedModule
    ClassModules  []ResolvedModule // exact marker order from the effective target modfile
    TargetModFile FileIdentity
}

type FileIdentity struct {
    Path   string // absolute canonical path
    SHA256 string // digest of the bytes parsed by XGo
}

type ProjectInfo struct {
    Project     *modfile.Project
    Origin      *ResolvedModule // nil only for built-in Gsh/Test projects
    Declaration FileIdentity    // exact declaring gox.mod/gop.mod snapshot; zero for built-ins
    RequiredXGo string          // declaring gox.mod xgo version; empty for built-ins
}

func (m *Module) ImportClassesResolved(graph ResolvedClassGraph, importClass ...func(*ProjectInfo)) error
func (m *Module) LookupClassInfo(ext string) (*ProjectInfo, bool)
```

`ResolvedModule.Effective()` 在有 `Replace` 时返回 replacement，否则返回 `Selected`；`IsLocal()` 在 `Main == true` 或 replacement 没有 version（filesystem replace）时为 true；`ValidateSyntax()` 不读磁盘，供协议边界校验，`Validate()` 才验证 effective source 的真实文件身份。禁止把 logical selection 与 replacement 压平：local replace 仍可能在 `Selected.Version` 中保留一个 release version。

路径字段采用条件式不变量，避免照抄 `go list -m -json` 后形成混合身份：无 replacement 时 `Selected.Dir/GoMod` 必须是 effective source；有 replacement 时 `Selected` 只保留 logical `Path/Version` 且 `Dir/GoMod` 必须为空，物理 source 只放在 `Replace.Dir/GoMod`。adapter 不得把 Go 命令顶层已被 replacement 覆盖的 `Dir/GoMod` 复制到 `Selected`；provider argv builder/parser 采用同一规则。

`ResolvedClassGraph.Target` 是包含 target directory 的 logical module；`ClassModules` 必须与 exact target modfile 中 `//xgo:class` marker 的数量、顺序和 logical path 一一对应，且不得重复 target 或其他 logical path。非 vendor record 的 effective `Dir/GoMod` 必须存在、absolute、canonical，并位于同一 source identity；缺 record、错序、重复、effective source 缺路径或错误 GoMod identity 都返回错误。该 API 不自行执行 `go`，只消费 XGo 已解析的 records。

`ClassModules` 的顺序必须来自 XGo 对**精确 effective target modfile bytes** 的一次解析：显式 `-modfile` 优先，否则使用 target module 的实际 modfile；workspace 中不得误用 workspace root 或调用者 cwd 的另一份 `go.mod`。`TargetModFile` 绑定 path 与所解析 bytes 的 SHA-256。receiver `m` 必须由同一份 target modfile snapshot 和 target `gox.mod` 构造；`ImportClassesResolved` 只使用 `graph.ClassModules`，禁止回读旧 `m.Opt.ClassMods`、`conf.Mod` 或另一张 path map 来决定 class dependencies。

project ext 与所有 work-class ext 都返回同一个 `ProjectInfo`。同一 project 注册自己的 project/work ext 不算 collision；只有已存在 ext 指向不同 project/origin 且至少一方携带 runtime 时才报 runtime collision。built-in Gsh/Test 返回 `Origin == nil`、`Declaration == (FileIdentity{})`、`RequiredXGo == ""` 且不得携带 runtime；runtime project 缺 origin 或 declaration snapshot 是内部错误。main/imported project 的 `Declaration` 与 `RequiredXGo` 都来自实际声明它的 `gox.mod`/`gop.mod` 同一次解析快照。

## 2. 有效 module/workspace graph

### One graph rule

XGo 从规范化后的 graph work directory 调用标准 Go toolchain 获取有效 build list，并将结果交给 `xgomod` 导入 class metadata。metadata discovery、provider `go list` 和 provider `go build` 必须使用同一个 canonical `GraphPolicy.WorkDir` 与其余 **GraphPolicy**；provider 进程自身的 cwd 仍是 project directory。provider/launcher/bridge 编译另使用明确的 **HostBuildPolicy**，不能把两者混为“相同 GOFLAGS”。

不得继续用原始 `go.mod require/replace` 手算 runtime-aware class metadata。必须覆盖：

- MVS 升级；
- wildcard replace 与 version-specific replace；
- `go.work use` 与 `go.work replace`；
- `GOWORK=off`；
- local replace；
- versioned module cache；
- vendor mode 的明确失败路径。

对每个声明为 `//xgo:class` 的 module path，导入有效 module 的 `Dir/gox.mod` 并记录 effective provenance。provider package 再通过同图的 `go list -json` 验证：

- package 存在且 `Name == "main"`；
- package 的 logical selection、完整 replacement 与 source identity 和 `ProjectInfo.Origin` 一致；非 vendor path identity 使用 canonical path + `os.SameFile`，不用字符串相等；
- package import path 位于 origin module 内。

framework 如需使用其他 module 实现 provider，必须在声明 class metadata 的 module 内提供一个薄 command wrapper。

### vendor mode

v1 明确不支持在 vendor mode **执行已经命中的 runtime provider**。标准 vendor tree 不保证包含 framework 根部 `gox.mod`，module record 也不提供可与 module-cache Dir 等价比较的 source layout；只 retention provider package 不足以建立 metadata provenance。

resolver 必须先做不执行 provider、也不要求完整 MVS graph 的 metadata preflight。已证明没有 runtime 的 main/dependency class project 返回 `NotHandled` 并沿用 legacy 路径；只有 preflight 已确认目标 project 携带 runtime 时才报 `ErrRuntimeVendorUnsupported`，说明当前 policy 和由用户显式选择 `-mod=readonly`/`-mod=mod` 的方法。无法可靠判定时保留原始 metadata error，不能把“不知道”伪装为 `NotHandled`。XGo 不得自行改写 policy。未来如需执行 vendor runtime，另行定义 vendored class-metadata manifest、package retention 与 provenance 格式。

## 3. XGo target 解析与分发

runtime discovery 位于 `xgoprojs.ParseOne/ParseAll` 之后、现有 Dir/PkgPath/Files type switch 和任何 `GenGo*` 之前。只有 resolver 返回 `NotHandled` 才进入现有路径。

### v1 target matrix

| target | v1 行为 |
| --- | --- |
| directory（`.`、`./game`、绝对目录） | 支持，canonicalize 后扫描顶层 project file |
| 单个 project file（如 `main.spx`） | 支持，归一化到其父目录并验证它是唯一 project file；推荐 run 时仍使用 directory target |
| 多文件 target | runtime project 明确拒绝；非-runtime 项目沿用旧行为 |
| 单个 import/package path | 支持，必须解析为有效 graph 中唯一目录 |
| `...` pattern | runtime v1 拒绝；`xgo run` 本来也不能运行多 package |
| `pkg@version` | 拒绝；provider/version 只能来自当前 graph |
| `xgo install` 多 target 且含 runtime project | v1 在任何构建副作用前整体拒绝；普通多 target 沿用旧行为 |

### resolver 算法

1. 将 target 转为 canonical absolute project directory；不复用从调用者 cwd 预先加载的 `conf.Mod`。
2. 从该目录加载有效 Go graph 和 provenance-aware class metadata。
3. 只枚举项目目录顶层普通文件，不解析或生成源码。
4. 对每个文件执行 `ClassExt -> LookupClassInfo -> Project.IsProj`。
5. 必须恰好找到一个 project file；两个文件即使指向同一 `Project` 也报歧义。
6. 没有 class project 或 project 没有 runtime：返回 `NotHandled`。
7. 找到 runtime 后，任何 metadata、graph、provider、协议或执行错误都直接返回错误，不得 fallback。

`NotHandled` 只能在 normal graph 或 vendor preflight 已成功判定目标没有 runtime 后返回。只要存在 class metadata/dependency 而有效身份无法确定，必须保留原始 graph/metadata error 并 fail closed。

import/package path 的目录解析必须基于 effective module records 做最长 module-path 匹配并校验目录边界，不能要求目录先含有可被标准 `go list` 识别的 `.go` 文件；class project 可能只有 `.spx` 文件。

run 的普通参数无需 delimiter。为解决 `xgo run main.spx existing-file` 被 `ParseOne` 识别为多 source file 的歧义，runtime v1 额外定义：紧跟已解析 target 的第一个 `--` 是用户 delimiter，由 XGo 去除；后续 `--` 保留。示例：

```bash
xgo run main.spx -- input.txt   # app argv: ["input.txt"]
xgo run . -- --                # app argv: ["--"]
```

统一返回：

```go
type ResolvedRuntime struct {
    TargetKind       TargetKind
    OriginalTarget   string
    TargetImportPath string
    DefaultExecName  string
    ProjectDir       string
    ProjectFile      string
    ModuleRoot       string
    Project          *modfile.Project
    Origin           xgomod.ResolvedModule
    RequiredXGo      string
    Protocol         string
    Package          string
}
```

展示和错误信息保留用户输入；安全/identity 比较使用 `Abs+Clean` 后的真实文件身份，对既有路径执行 `EvalSymlinks`/`os.SameFile`，Windows/macOS case-insensitive 与 Unicode normalization 有专门测试。不得只比较 path string。runtime 路径的失败分支也不得创建/修改 `xgo_autogen.go`。

在构建 provider 前，XGo 比较 `ProjectInfo.RequiredXGo` 与既有 `env.Version()`。可比较的 release/prerelease build 直接按原版本判断；`(devel)` 和现有 `vX.Y.Z devel` 源码构建保留真实 build identity，但使用 `cmd/internal/runtimeprovider` 私有的 capability baseline 做比较。两端统一去除可选 `v`、补齐缺省 patch 后按 semantic version 比较，错误同时打印 required、真实 build identity 与 development capability。built-in 的空值不检查；runtimeprovider 依赖更高 XGo 能力时必须更新该私有 baseline 并补回归测试。这个 fallback 不改变 XGo 公共版本机制，也不得通过测试专用 linker version 冒充正式版本。

## 4. Provider 信任与构建边界

引入 provider 后，`xgo run/build/install` 会执行 class dependency 提供的程序。这是新的 supply-chain 执行边界，必须显式约束：

- 只有 target 所属 main/workspace module 自己的 `gox.mod`，或应用在 `go.mod` 中明确标记为 `//xgo:class` 的直接 dependency module，可提供 runtime；
- provider 必须属于声明 metadata 的同一 effective module；
- provider 必须是 command package；
- 使用参数数组直接执行，禁止 shell 拼接；
- `-x` 输出 provider package、origin、protocol、graph/workspace 和 cache decision，但不打印环境 secret；
- `XGO_RUNTIME=off` 是审计/应急开关；匹配到 runtime project 时它返回明确错误，不回退 GenGo；
- 设置递归 guard，provider 间接再次对同一 target 触发 runtime 时立即失败。

### provider host build

XGo v1 每次命令都在私有临时目录（POSIX `0700`；Windows private DACL）中构建 provider，依赖 Go build cache；v1 runtime install 只允许一个 target，因此不承诺多 target provider 复用。

要求：

- 使用标准 `go`，忽略 `XGO_GOCMD` 的 llgo/tinygo 选择；
- 不调用会注入 XGo linker flags 的现有 `gocmd.Build`；
- XGo 构建 generic provider 时保留调用者环境中的 `CGO_ENABLED`；未设置时交给标准 Go toolchain 选择 host 默认值，不得由 XGo 强制改为 `0`；
- 显式构建 host `runtime.GOOS/runtime.GOARCH`、`-buildmode=exe`；
- 若用户 `GOOS/GOARCH` 指向非 host target，在 runtime discovery 后、provider build 前报 host-only 错误；
- GraphPolicy 包含 canonical absolute `WorkDir`、解析后的 `GOWORK`、`-mod`、`-modfile`、`-overlay`；list、metadata import、provider package validation 及所有嵌套 Go env/list/build 使用同一份；
- HostBuildPolicy 固定 host GOOS/GOARCH 与 `-buildmode=exe`，generic provider 保留 ambient `CGO_ENABLED`；SPX generated launcher 显式使用 `CGO_ENABLED=0`，source bridge 单独显式启用受控 CGO/toolchain；
- Darwin launcher build 在 `go build` 后使用系统 `/usr/bin/codesign --force --sign -` 做 ad-hoc signing，并以 `codesign --verify --strict` 为成功条件；签名后禁止再改写 output；
- XGo 读取并解析 effective `GOFLAGS`。允许的 graph flags 与 `-trimpath`、`-buildvcs=false` 被规范化为 provider argv；`-n`、`-tags`、`-toolexec`、非 exe buildmode 等其余冲突项明确拒绝并点名；
- 执行 provider 时显式设置 `GOFLAGS=`，防止 provider 的嵌套 Go command 再从 GOENV 隐式读取另一套值；provider 只使用 argv 中的 canonical GraphPolicy/BuildFlags；
- Windows 输出使用 `.exe`；
- 记录 cold/warm benchmark，在数据证明需要前不增加自定义持久 executable cache。

## 5. Provider CLI protocol v1

XGo 对 provider 的调用采用纯 argv，不创建 request 文件、不使用 JSON，也不占用 stdin。所有字段使用独立 argument 直接传给 `exec`，禁止经过 shell。所有 XGo 生成的 path 都是 absolute、clean、已 canonicalize 的路径。

### run

```text
<provider> xgo-runtime-v1 run
  <common-option>...
  --
  [application-argument]...
```

### build

```text
<provider> xgo-runtime-v1 build
  <common-option>...
  --output=<absolute-private-staging-output>
  --final-output=<absolute-user-output>
```

`install` 不增加 provider verb；XGo 解析安装路径后复用 `build`。

本文与 `github.com/goplus/mod/runtimeprotocol` 的 typed request/argv codec 共同定义 normative contract：文档规定语义，codec 是唯一可执行 grammar。XGo 只构造 `runtimeprotocol.Request` 并调用 deterministic encoder；provider 调用同包 strict parser，再在自身 domain layer 执行 live path pinning 与业务校验。shared codec 不读磁盘、不执行 provider、也不包含 SPX/Godot 规则。跨仓 round-trip vectors 与非 SPX fake-provider e2e 锁定一致性。

Graph snapshot 明确包含执行 Go graph 命令的锚点，不能从 provider cwd 或 project module root 反推：

```go
type Graph struct {
    GoCommand string
    WorkDir   string
    GoWork    string
    Flags     []string
}
```

所有 option 只接受 `--name=value` 形式；以第一个 `=` 分隔，因此值可以包含空格、`=` 或 leading dash。required singular option 必须恰好出现一次，conditional option 按下表出现，任何 singular option 重复都报错：

| option | 约束 |
| --- | --- |
| `--project-dir`、`--project-file`、`--module-root` | canonical absolute path |
| `--provider-package` | XGo 已验证的 command import path |
| `--selected-path`、`--selected-version`、`--origin-main` | logical module identity；只有 main 的 version 为空，main 必须为 `true`/`false` 且不得 replacement |
| `--selected-dir`、`--selected-gomod` | 无 replacement 时必需；有 replacement 时禁止 |
| `--replace-path`、`--replace-version`、`--replace-dir`、`--replace-gomod` | replacement 时整组必需，否则整组禁止；version 为空表示 canonical filesystem replacement，且 path 必须等于 dir |
| `--project-ext`、`--project-full-ext` | 已解析 project identity |
| `--pack-dir`、`--pack-index` | optional complete pair；不使用 pack 的 provider 两项均不接收 |
| `--declaration-file`、`--declaration-sha256` | provider effective source 根部 declaring `gox.mod`/`gop.mod` 的 canonical path 与完整摘要；它是顶层 declaration identity，不属于 target Project |
| `--go-command` | XGo 解析出的 host Go executable absolute path |
| `--graph-work-dir` | XGo discovery 使用的 canonical absolute cwd；provider 的所有嵌套 Go env/list/build 必须使用它 |
| `--go-work` | `off` 或 canonical absolute workspace file |
| `--graph-flag` | repeated；每次一个 canonical graph flag，保持顺序 |
| `--build-flag` | repeated；每次一个已允许的 canonical build flag，保持顺序 |
| `--output`、`--final-output` | 仅 build 必需；run 中禁止 |

module path/version、project metadata 和 declaration identity 都是 XGo 已解析状态的快照；provider 不自行从 ambient module 重新做 project/runtime discovery。执行前必须以 no-follow open 重新校验 `--declaration-file` 的 SHA-256，发现 discovery 后文件变化即 fail closed。module origin 仍遵守条件式路径不变量：存在 replacement 时禁止 `--selected-dir/--selected-gomod`，只有 replacement options 携带 effective source。

v1 flag set 发布后冻结：未知 option、缺失/重复 singular option、残缺 replacement group、错误 action 或错误 run/build 专属 option 全部失败；任何协议字段或语义演进都使用 `xgo-runtime-v2`。argv 只包含 path、module identity、摘要和 build policy，不得放 credential、token 或其他 secret。XGo 在 spawn 前检查平台 argv+environment 限制，超限返回 `ErrRuntimeArgvTooLarge`，不得暗中回退 request 文件或 JSON。

协议规则：

- provider 进程 cwd 是 project directory；provider 内部所有 Go env/list/build 的 cwd 是 `--graph-work-dir`；
- stdin/stdout/stderr 直接继承；
- 除 XGo 明确清理/设置的 host-build 项外，运行环境继承；
- provider 对未知 action、未知/重复 CLI flag、违反条件组合、相对 path、缺失分隔符和多余参数返回 usage error；
- `--output` 的父目录由 XGo 创建且与 `--final-output` 同一 filesystem，provider 只写 staging output；final output 仅供安全收集/命名判断，provider 不得写它；
- build 返回 0 时 output 必须存在；XGo 仍会独立 `Lstat` 校验；
- run 必须有一个 protocol `--`，其后的 application argv 逐字节、逐元素、保持顺序传递；build 禁止 `--` 和任何 positional argument；
- 用户通常不写额外 separator：`xgo run ./game --headless`。只有紧跟 target 的首个 `--` 按上一节规则作为可选用户 delimiter；之后的字面量 `--` 必须原样保留。

### flags 契约

XGo 当前将 build flags 规范化为 `-name=value`，不是原始 token。SPX provider v1 采用最小、明确的集合：

| flags | v1 行为 |
| --- | --- |
| `-v`, `-x` | 支持，分别控制 verbose/trace；不改变应用 stdout/stderr |
| `-work` | 支持，保留并打印 XGo/provider/session 临时目录 |
| `-quiet`, `-debug` | 由 XGo orchestration 消费，不作为 Go build flag 转发 |
| `-o` | 由 XGo 独占并转成协议 `--output` |
| `-n` | 拒绝；它不产生产物，与 build 成功契约冲突 |
| `-asm`, `-nc`, `-prof` | runtime v1 明确拒绝 |
| `-trimpath`, `-buildvcs=false` | 支持，同时应用于 provider 与 generated launcher Go build；其他 `-buildvcs` 值拒绝 |
| `-a`, `-linkshared`, `-race`, `-msan`, `-asan` | v1 拒绝，直到定义 launcher/解释器的等价语义 |
| `-p`, `-asmflags`, `-compiler`, `-buildmode`, `-gcflags`, `-gccgoflags`, `-installsuffix`, `-ldflags`, `-pkgdir`, `-tags`, `-toolexec` | v1 拒绝，错误必须点名 flag |

未来新增 XGo build flag 时，runtime dispatcher 的测试必须迫使维护者为它选择 supported/consumed/rejected，不能默认透传。

### 退出码与信号

- provider 正常退出 N，XGo 最终退出 N；
- run 中 Engine 正常退出 N，SPX provider 与 XGo 都保留 N；
- Unix 被 signal S：完成可行清理后恢复默认 handler，使顶层进程以同一 signal 终止；
- Unix 上 XGo 为 provider 建独立 process group，并把 SIGINT/SIGTERM/SIGHUP 转发给整个 group；Engine 继承该 group；
- Windows 使用 console control 继承并用 Job Object 保证 XGo 退出时不遗留 provider/Engine；
- `runtimeexec`、provider、`interpruntime` 新路径的 lower layers 只返回 typed status/error，不调用 `os.Exit`、`Fatalf` 或 panic；现有 XGo `Command.Run` 仍可保持 void，由 run/build/install command handler 在显式完成 provider temp/cache cleanup 后统一退出或重发 signal，不要求本 proposal 重构全部旧 CLI。

## 6. SPX 路径契约与解释执行核心

当前 `cmd/ispxnative` 假定项目是 cwd 的父目录，而资源路径仍通过相对 `../assets` 推导。任意隔离 session 会导致图片、音频、字体等失效。因此先引入三个独立且始终 absolute 的根：

- `ProjectDir`：`.spx` 源码和 `.config`；
- `AssetDir`：项目资源的物理根；
- `SessionDir`：Godot `--path`、`.godot`、extension scaffold 和运行时临时状态。

推荐库边界：

```go
type Config struct {
    ProjectDir string
    AssetDir   string
    SessionDir string
    Bundle     runtimebundle.Bundle
    Args       []string
    Env        []string
    Stdin      io.Reader
    Stdout     io.Writer
    Stderr     io.Writer
    KeepWork   bool
}

func Run(ctx context.Context, cfg Config) (ProcessStatus, error)
```

实现放在顶层 `internal/interpruntime`，不得依赖 `CmdTool`、GOPATH、全局 cwd 或 `cmd/spx/internal`。现有 `spx run` 先改成该库的 adapter，回归通过后 provider 再接入。

bridge 启动时接收并校验：

```text
SPX_PROJECT_DIR=<absolute>
SPX_ASSET_DIR=<absolute>
SPX_SESSION_DIR=<absolute>
```

两种模式的映射固定为：

```text
provider run:
  ProjectDir = protocol --project-dir
  AssetDir   = ProjectDir / validated Project.Pack.Directory
  SessionDir = fresh private run directory

generated launcher:
  ProjectDir = materialized project content root
  AssetDir   = ProjectDir / canonical packed asset directory
  SessionDir = fresh private run directory
```

`.config` 始终从 `ProjectDir` 读取。`Pack.Directory` 必须是 clean relative path，拒绝 absolute、`..`、空值和 symlink root。provider 从继承环境中删除所有已有 `SPX_PROJECT_DIR/SPX_ASSET_DIR/SPX_SESSION_DIR` 后写入唯一的已校验值，不能让调用者环境覆盖。随后以 `os.DirFS(ProjectDir)` 构建源码，并把 project config root 与 asset root 分别传入 engine API；不能只在当前 `engine.SetAssetDir` 上加绝对路径特判。

v1 解释器只支持 ispx 已注册的 package surface。发现未支持的 import 时必须返回包含 import path 的错误，不得 native fallback。

## 7. Runtime/bridge 发布物与缓存

### 发布身份

Engine executable/PCK 继续由 Engine Runtime version/ABI 管理，避免每个 SPX 版本重复发布几十 MB。新增与 SPX 产品版本绑定的 bridge 资产，例如：

```text
spx-interpreter-bridge-<spx-version>-darwin-amd64.zip
spx-interpreter-bridge-<spx-version>-darwin-arm64.zip
spx-interpreter-bridge-<spx-version>-linux-amd64.zip
spx-interpreter-bridge-<spx-version>-windows-amd64.zip
spx-interpreter-bridge-manifest-<spx-version>.json
```

manifest 至少包含：schema、SPX module/version/commit、Engine Runtime version/ABI、GOOS/GOARCH、bridge name/mode/size/SHA-256、构建 provenance。release workflow 在上传前验证 manifest，并让 standalone 与 provider 消费同一组已验证输入。

bridge 是 SPX product asset，不加入 Engine Runtime `runtime.lock.required_assets`。正式 bridge 先发布到不构成 Go module version 的 immutable release tag（例如 `spx-bridge-v3.x.y`），provider 使用该稳定 URL；最终 `v3.x.y` module tag 只在该 URL 已公开验证后创建。provider 不下载、执行或拆解 standalone `spx` binary；standalone 与 provider 只共享 `runtimebundle` 代码和同一 CI run 产出的 bridge artifact。

source mode 只适用于 main/workspace source 或 local filesystem replace，并从其可验证的 effective source 构建 bridge。只有“无 replace、非 local、canonical release semver 且 module version 与 provider 内建 SPX version 一致”的 SPX 可以使用发布 bridge。v1 对 pseudo-version 和 versioned remote replace 一律返回明确的 unsupported/identity error：它们不是 local source identity，不能进入 source mode，也不能借用任意预装的 Engine/PCK/bridge。snapshot manifest 支持属于后续 proposal，不是 v1 的隐含 fallback。版本自检不一致同样明确失败。source mode 要求：

- 从 `ProviderOrigin.Effective().Dir` 构建 bridge；
- 明确要求 CGO 与本机 clang/Xcode/对应 linker toolchain；
- bridge build 显式清空 ambient `GOFLAGS`，复用 argv GraphPolicy，再叠加受控的 host CGO/compiler/linker policy；
- 不因 dirty worktree 拒绝，但按真实输出 digest 标识，不得只按版本号复用；
- runtime lock、Engine manifest 与 bridge manifest 都增加 `engine_interface_digest`：对 Godot SPX module interface、GDExtension registration schema 和生成 bindings 的规范输入列表计算完整摘要；source mode 只有现场计算值与 published Engine manifest 匹配时才能复用 published Engine，否则强制显式 local Engine bundle；仅比较整数 ABI 不足；
- 审查基线的 `runtime-v2.4.1` 已公开且不可变，不能补写新 manifest 字段。activation 必须选择新的 immutable runtime version（基线下可为 `2.4.2`；是否同时 bump 数字 ABI 独立评估），更新 lock schema/snapshot，构建并先发布含同一 `engine_interface_digest` 的 Engine manifest；
- 不得公开发布含 runtime directive 的 SPX release，直到同一 activation SHA 的 bridge/standalone same-run artifacts 与全部 gate 已组装完成；activation SHA/draft 中可以同时包含该 directive。

### `internal/runtimebundle`

从 buildctl 中抽取不认识 repo root、GOPATH 或 Godot source tree 的纯库：

```text
internal/runtimebundle/
  identity.go
  manifest.go
  acquire.go
  verify.go
  materialize.go
  lock_*.go
  gc.go
```

要求：

- 每个 entry 有 name、mode、size、完整 SHA-256；bundle identity 使用完整 digest；
- release download 到 sibling temp，校验 size/digest 后原子发布；
- cache hit 重新校验 complete marker 与摘要，不能只比 size；
- 8–16 个并发进程使用跨进程锁，只产生一个完整目录；
- 整个 bundle 在 sibling temp dir materialize，完成后一次 rename；
- POSIX cache 目录 `0700`、普通文件 `0600`、可执行文件 `0700`；Windows 创建仅当前用户可访问的 private DACL，不把 POSIX mode 当作 Windows 安全边界；
- cache 分为 `engine/<digest>`、`bridge/<digest>`、`project/<digest>` 三个 namespace；相同 Engine、不同 bridge/project 只物化一个 Engine entry；
- 中途 kill、损坏、同大小篡改可自动恢复；
- 默认 quota 为 5 GiB、最大闲置期为 30 天；分别允许通过 `SPX_RUNTIME_CACHE_MAX_BYTES`、`SPX_RUNTIME_CACHE_MAX_AGE` 配置；Engine/bridge/project 在整个运行期持有 shared lease，GC 取得 exclusive lock 后才可删除；
- provider 的 release-download source 在 `SPX_RUNTIME_OFFLINE=1` 下 cache hit 成功，miss 报完整 identity 和所需 URL且不联网；generated launcher 的 embedded-payload source 永不下载，即使 cache 为空且网络被阻断，也必须从自身 payload 首次 materialize 成功。

所有 ZIP/manifest 输入都视为不可信，统一拒绝：absolute path、`..`、反斜线穿越、NUL、duplicate、Unicode/case-fold collision、symlink、hardlink、device、越界 offset、超大 entry count/单文件/总解压量/压缩比。限制值作为代码常量并有边界测试；初始上限建议为 10,000 entries、512 MiB/entry、4 GiB total、200:1 ratio。

SHA-256 证明完整性，不证明发布者身份；签名/attestation 另立 issue，不在文档中把 checksum 描述为真实性证明。

## 8. SPX provider 的 run/build

推荐代码边界：

```text
internal/interpruntime/   # prepare session + execute Engine
internal/runtimebundle/   # acquire/verify/materialize Engine + bridge
internal/projectbundle/   # safe collect/canonical archive/extract
x/xgolauncher/            # public, pure-Go launcher runtime imported by generated main
cmd/spx/                  # existing CLI adapter
cmd/xgoruntime/           # provider protocol only
internal/cmd/buildctl/    # release/build adapter
```

`cmd/xgoruntime` 必须通过 `CGO_ENABLED=0 go build ./cmd/xgoruntime`，且只解析 provider protocol。这里的 pure-Go/CGO 约束属于 SPX provider capability，不覆盖 XGo generic executor 保留 ambient `CGO_ENABLED` 的规则；其他 framework 若要求特定 CGO policy，必须自行声明并在不满足时明确失败。最终 launcher 是 provider 在私有目录生成的独立 Go `main`；它从同一 application graph 导入 `github.com/goplus/spx/v3/x/xgolauncher`，并在 link 前通过 `go:embed` 包含完整 payload。

### provider run 流程

1. 校验 protocol argv、paths、host target、flags 和 recursion guard；
2. 显式拒绝 pseudo-version/versioned remote replace，再按 argv 的 logical/replacement identity 选择 release/source mode，读取 runtime lock；
3. acquire + verify Engine/PCK/bridge；
4. 在项目外创建私有 session；
5. 设置三根路径，生成 extension scaffold；
6. 拒绝用户传入 `--path` 或 `--path=...`，provider 独占 Engine project path；
7. 继承 stdio，启动 Engine，保留 argv/exit/signal；
8. 除 `-work` 外清理 session，项目目录保持不变。

### provider build 流程

1. 执行 run 所需的 identity/asset 校验并得到已验证的 Engine、PCK、bridge；
2. 使用 `internal/projectbundle` 收集 canonical `project.zip`；
3. 组装下节定义的 canonical `payload.spxpkg`，其中同时包含 Engine/PCK、bridge manifest/binary 和 project.zip；
4. 在私有 temp 中生成固定模板 `main.go`，`go:embed payload.spxpkg`，同时写入 payload/manifest expected SHA-256 常量；
5. 使用 argv 中的 GoCommand + GraphPolicy + HostBuildPolicy，从 `GraphPolicy.WorkDir` 执行 `go build -buildmode=exe -o <--output> <generated-main.go>`；`GOFLAGS` 置空、`CGO_ENABLED=0`；
6. `go list` 验证 `x/xgolauncher` 与 provider 来自相同 effective SPX origin；
7. 验证 output 是目标 host 的普通 executable；Darwin 对完整 embedded output 执行 `/usr/bin/codesign --force --sign -`，随后执行 `codesign --verify --strict`，此后不再修改该 staging 文件；
8. 成功返回，由 XGo 完成最终 atomic replace。

generated main 的形状固定并由 golden test 锁定：

```go
package main

import (
    _ "embed"
    "os"

    "github.com/goplus/spx/v3/x/xgolauncher"
)

//go:embed payload.spxpkg
var payload []byte

const payloadSHA256 = "<generated>"
const manifestSHA256 = "<generated>"

func main() {
    status := xgolauncher.Run(payload, payloadSHA256, manifestSHA256, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
    xgolauncher.Exit(status)
}
```

`xgolauncher.Run` 只返回 typed `ProcessStatus`，并在返回前完成 session/lease cleanup；`xgolauncher.Exit` 是唯一 command boundary：正常状态调用 `os.Exit(code)`，Unix signal 状态先恢复默认 handler 再向自身重发同一 signal，不能折叠成 `128+signal`。Windows 按 console-control/process status 契约映射并保证 Job cleanup。golden/e2e 必须对 Unix `WaitStatus.Signaled()` 与 `Signal()` 断言，不能只检查数字退出码。

## 9. Project bundle 与 embedded launcher payload

### Project 收集范围

禁止 archive 整个 repository。v1 allowlist：

- 项目根下由当前 project metadata 识别的普通 `.spx` 文件；
- 可选根 `.config`；
- validated `Project.Pack.Directory`（SPX 当前为 `assets`）中的普通文件；
- 被配置引用、位于 `ProjectDir` 内但不在 Pack 下的普通文件，由 caller 作为 `ProjectFiles` 显式列出。

v1 不兼容 `.config.extasset` 或历史 shared-assets fallback。所有 filesystem resource 在按其配置文件目录归一化后必须位于 canonical `ProjectDir` 内；绝对 host path、其他工程路径及任何逃逸 `ProjectDir` 的 `../` 都在构建前明确失败，不做 remap 或静默忽略。

收集使用 no-follow open，并在 open 后 `fstat` 复核 identity/type，防止检查与读取间 symlink swap。明确排除 VCS、`go.mod/go.sum/go.work`、`.temp`、`.godot`、`.builds`、socket/device、symlink 和未声明仓库文件。协议 `--final-output`、`--output` 及 staging subtree 都排除；如果 final output 位于 Pack root 内，v1 直接拒绝构建，避免旧产物被误打包。`-x` 可打印收集清单，默认不打印内容。

canonical project ZIP 固定：UTF-8 slash path、lexical sort、固定 timestamp/mode、固定 compression policy、无平台额外字段。连续两次收集同一输入必须得到同一 project payload SHA-256。

### `payload.spxpkg` v1

payload 是由 Go linker 嵌入 executable 的 canonical ZIP，固定布局：

```text
META-INF/spx-runtime-v1.json
engine/runtime-manifest.json
engine/<platform-runtime-executable>
engine/<runtime-pck>
bridge/bridge-manifest.json
bridge/<platform-bridge>
project/project.zip
```

顶层 manifest 包含 protocol、SPX logical/replacement/source identity、Engine Runtime version/ABI/interface digest、target、engine digest、bridge digest、project digest，以及除顶层 manifest 自身外所有 ZIP entry 的 path/mode/size/SHA-256。manifest 固定为上述精确 path，禁止第二份 `META-INF` manifest。generated main 的 `manifestSHA256` 单独覆盖 manifest bytes，`payloadSHA256` 覆盖完整 ZIP，因此没有自引用。

launcher 按以下顺序处理：

1. 校验 embedded payload 与 manifest expected SHA-256；
2. 严格解析唯一 manifest 并校验全部 entry table 与安全上限；
3. 分别按完整 digest materialize/reuse engine、bridge、project cache；
4. 从 materialized project 建立 fresh session 并按三根路径运行；
5. 永不访问网络；空 cache 首次运行也只从 embedded payload materialize。

SHA 常量和 entry digest 用于损坏检测，不构成发布者真实性证明。修改 executable 后再同步修改常量的攻击者仍可重打包；Developer ID/Authenticode/attestation 属后续发布安全层。

### Darwin 签名决策

不采用 appended-data/EOF-footer。Apple 的 code-signing technote 明确指出向 Mach-O executable 追加数据会使签名校验失败；Go toolchain 内部虽有 Mach-O ad-hoc signing 支持，但不能把“所有 host `go build` 都会产生已签名文件”当作协议保证。参见 [Apple TN2206](https://developer.apple.com/library/archive/technotes/tn2206/) 与 [Go `cmd/internal/codesign`](https://go.dev/src/cmd/internal/codesign/codesign.go)。

本机 Go 1.26.5/Darwin amd64 spike 已验证：外部生成的 `main.go` 可从应用 module graph 编译、embedded payload 可执行，但原始产物被 `codesign --verify --strict` 判为未签名；随后执行 `/usr/bin/codesign --force --sign -`，严格校验和运行均成功。因此 v1 把系统 `codesign` 定义为 Darwin **构建时**依赖：payload 先通过 `go:embed` 进入 binary，link 完成后再 ad-hoc sign，签名后不得 append 或重写。运行已构建 launcher 不依赖该工具。

Darwin amd64/arm64 CI 必须执行真实 launcher smoke 和 `codesign --verify --strict`。v1 只有 ad-hoc signature，不具有 Developer ID/notarization 所代表的发布者身份；用户后续可替换为 Developer ID 签名，但必须在所有内容固定后完成。

PR 8 开始时先做 go/no-go spike，测量 100+ MB embedded payload 的 Go compile/link 时间、峰值内存、最终体积、启动与签名行为；若超过发布预算，必须回到 issue 重新选择容器格式，不能退回未经验证的 append。

本机基线中 standalone `spx` 约 102 MB、Engine Runtime 约 43–59 MB、bridge 约 27 MB，因此 v1 单文件预期约 70–100+ MB。release notes 必须公开体积、首次 materialize 成本与源码明文属性。

## 10. XGo build/install 输出事务

XGo 负责最终目标路径，provider 永远只写私有 temp output。

### build

- `-o file`：解析为 absolute final path；
- `-o existing-dir` 或以平台 separator 结尾：在目录下追加默认 executable name；
- 无 `-o`：directory target 使用项目目录名，package path 使用与 cmd/go 一致的 executable name 规则并正确处理尾部 `/vN`，单文件 target 使用其 project directory name；
- Windows 只追加一次 `.exe`；
- final path 若是 symlink 或非普通文件，拒绝且不跟随；
- 在 final parent 下创建 `0700` sibling temp dir，将其中的 output 传给 provider；
- provider 返回 0 后 `Lstat`：必须是单个、非 symlink、非空、host executable；
- fsync file，然后使用单一平台 atomic primitive，禁止先删除旧目标；POSIX 使用 same-filesystem rename，Windows 对“目标存在”使用 `ReplaceFileW`、对“不存在”使用对应原子 move；
- commit 前任意错误都使已有 final artifact 逐字节不变；commit syscall 失败按平台契约保留旧目标并报告 `Lstat` 后的最终状态；
- 不对非原子 network filesystem、掉电或平台 syscall 契约之外的情形作超额保证。

### install

- 使用相同 `build` staging/validation；
- 安装目录为有效 `go env GOBIN`，为空时取 `go env GOPATH` 第一项的 `bin`；
- v1 runtime install 只接受一个 target；多个 target 在任何 provider build 前报错；
- 不通过 shell，不假定 Unix path separator；
- 与 build 相同地保护已有安装文件。

## 11. 兼容性与回滚

- 新 XGo 应沿用现有 build version 校验可见 `xgo` minimum version；只有 unversioned development build 使用 runtimeprovider 私有 capability baseline，但 issue 不声称能改变已发布旧二进制；
- v1 独立 `runtime` directive 在旧 parser 下可能被忽略，旧 XGo 明确不受支持；
- 新 XGo 对没有 runtime metadata 的项目完全走旧路径；
- preflight 已命中 runtime 后若 effective mode 是 vendor，由于缺失可验证的依赖 metadata 明确失败；未声明 runtime 的 vendor class project 仍沿用 legacy 路径；
- 新 XGo 对已有 runtime metadata 的任何失败不回退；
- 紧急禁用使用 `XGO_RUNTIME=off` 并明确失败；
- 产品回滚方式是 pin 到未启用 runtime 的 SPX 版本，或发布移除该 directive 的修订版，不提供隐式 fallback 开关；
- 新 proposal 合入时关闭/替代 [goplus/mod#142](https://github.com/goplus/mod/pull/142)，不得复活其独立 optional version、弱 path 校验和多余参数静默忽略行为。

## 12. 分阶段 PR 计划

每个 PR 都必须可独立 review/test；在第 1–8 阶段完成前不得修改正式 SPX `gox.mod`。

### PR 1 — `goplus/mod`: schema、校验与 provenance model

- `runtime v1 <import-path>` parser/formatter；
- `Runtime` 字段；
- `ProjectInfo/ResolvedModule/FileIdentity/LookupClassInfo`；
- 接受 XGo 提供的 effective module records 并从其目录导入 class metadata 的 API；
- provider-neutral `runtimeprotocol.Request`、无 I/O structural validation 与唯一 deterministic argv encoder/parser；不提供 lifecycle/execution helper，不引入 JSON；
- runtime-backed extension conflict；
- parser、modload、xgomod tests；
- 明确 supersede #142。

**Release gate A：** PR 1 合入后立即发布包含 schema/API 的新 mod 版本；后续 XGo PR 不依赖未发布 commit。

### PR 2 — `goplus/xgo`: effective graph class loading

- bump 到 Release gate A 的 mod 版本；
- 标准 `go list -m -json` graph adapter；
- MVS/replace/go.work/GOWORK、exact effective modfile/ordered ClassModules 与 runtime-only vendor-unsupported 语义；
- provenance-aware class import；
- `RequiredXGo` minimum-version check；
- fake modules 覆盖 graph mismatch；
- 普通 class loading 回归。

### PR 3 — `goplus/xgo`: generic runtime executor

- 统一 target resolver，置于所有 GenGo 前；
- provider identity validation 与保留 ambient CGO policy 的 host build；
- 从 resolved runtime 构造 shared `runtimeprotocol.Request`；argv grammar/unknown/group/action 校验只由 mod codec 实现，不新增 JSON schema；
- v1 run/build protocol、project metadata snapshot/digest、flag policy、stdio/status/signal；
- fake framework/provider e2e，禁止 SPX/Godot 特判；
- build atomic output，install single-runtime-target policy；
- 功能默认未被 SPX 正式 metadata 激活。

### PR 4 — `goplus/spx`: roots 与 interpreted core

- `ProjectDir/AssetDir/SessionDir`；
- `internal/interpruntime`；
- `cmd/ispxnative`/engine path API 改造；
- 现有 `spx run` 改为 adapter；
- 图片、音频、字体、SVG 从 `ProjectDir` 内显式 AssetDir 加载，并在项目外 session 做 native smoke；
- `.config.extasset` 与跨出 `ProjectDir` 的资源引用 fail closed。

### PR 5 — `goplus/spx`: verified runtimebundle

- `internal/runtimebundle`、完整 manifest、download/materialize/lock/GC/offline；
- buildctl 与 standalone 改用 adapter；
- 并发、kill、corruption、ZIP adversarial tests；
- 保持现有 standalone release 行为。

### PR 6 — `goplus/spx`: versioned bridge release workflow

- host platform bridge assets + manifest/provenance；
- runtime lock/manifest schema 支持 `engine_interface_digest`，并为下一次 activation 准备新的 runtime version；不得修改或复用审查基线已发布的 `runtime-v2.4.1`；
- 通过现有 buildctl/release workflow 暴露 dry-run/publish/verify，Makefile 只作稳定入口；
- release dry-run、same-run artifact、download/verify；bridge 正式资产使用 immutable 非-semver tag，最终 module semver tag 不得充当 draft/staging tag；正式发布留到 coordinated activation；
- SPX/runtime/ABI/platform mismatch fail closed；
- release mode 与 local source mode tests。

### PR 7 — `goplus/spx`: provider run

- pure-Go `cmd/xgoruntime`；
- shared `runtimeprotocol.Parse` adapter + SPX 独立 live identity/domain validation；不依赖 XGo internal package；
- protocol adapter + `interpruntime`；
- release/local replace/go.work/offline modes，以及 v1 对 pseudo-version/versioned remote replace 的显式拒绝；
- import/flag fail-closed、argv/stdin/exit/signal e2e。

### PR 8 — `goplus/spx`: projectbundle 与 generated embedded launcher

- 先完成 100+ MB `go:embed`/Darwin signature go-no-go spike；
- canonical allowlist collector；
- canonical self-contained payload + generated `main.go` + `x/xgolauncher`；
- engine/bridge/project shared cache；
- host macOS/Linux/Windows self-contained artifact e2e；
- Darwin post-link ad-hoc signing + strict verification、size/source disclosure docs。

### PR 9 — coordinated activation/release

- 先发布已完成 PR 2/3 的 XGo；mod 已在 gate A 发布；
- 冻结一个同时包含 provider、launcher、bridge recipe、minimum XGo 和 `runtime v1 ...` 的 SPX activation SHA；
- 从该 SHA bump 新 runtime version（不能复用 `runtime-v2.4.1`），更新 lock/schema/snapshot；同一 workflow 构建 Engine Runtime、bridge、standalone 和 manifests，dry-run/smoke 只消费 same-run artifact；
- 先执行 `publish-runtime`，下载公开 runtime manifest 并验证 version/ABI/`engine_interface_digest` 与 lock 完全一致；该 gate 未通过不得发布 SPX；
- 再把 bridge/standalone/manifest 发布到 immutable 非-semver product-assets release（或等价稳定对象 URL），下载回验后才算“assets ready”；provider 是 module 内 Go command，不单独发布 binary；
- 整个 staging/finalization 期间断言远端最终 `refs/tags/v3.x.y` 不存在，禁止用最终 module tag 创建 draft；所有公开依赖和 gates 就绪后才从同一 activation SHA 创建最终 semver tag/release；
- 最终 tag 创建后执行 versioned module + public runtime/bridge download，以及 local workspace/full offline matrix；
- 关闭 superseded PR，迁移文档与 rollback 文档。

## 13. 测试与验收矩阵

### 基础测试

```bash
# goplus/mod
go test ./...

# goplus/xgo
go test ./...

# goplus/spx
go test ./pkg/ispx/... ./internal/release/...
go test ./cmd/spx/internal/runtimeasset/... ./cmd/spx/internal/command/...
go test ./internal/cmd/buildctl/...

# 新包
go test ./internal/runtimebundle/... ./internal/interpruntime/... ./internal/projectbundle/... ./x/xgolauncher/...
go test -race ./internal/runtimebundle/... ./internal/interpruntime/... ./internal/projectbundle/...
go test ./cmd/xgoruntime/...
CGO_ENABLED=0 go build ./cmd/xgoruntime

git diff --check
```

### mod 必须覆盖

- [ ] 两个 project 各自 runtime，不串 scope；project/work ext 查询同一 info；
- [ ] before-project、duplicate、缺参/多参、非法 protocol、非法 import path；
- [ ] strict/lax 对 known malformed directive 都报错；
- [ ] parse/format/save/reload 保留 syntax 与注释；
- [ ] local fake class module 经 replace 导入 runtime/provenance；
- [ ] logical selection + local/version replacement 的 in-memory adapter round-trip，不被压平；replacement 下 `Selected.Dir/GoMod` 为空且只有 `Effective()` 携带物理 source；built-in origin/required-version 为空；
- [ ] runtime-backed ext collision 报错。

### XGo 必须覆盖

- [ ] MVS、wildcard/version-specific replace、go.work use/replace、`GOWORK=off`、local replace；原 `go.mod` 与 `-modfile` 含不同 class marker 时只使用 effective 文件，workspace target module 不误读 workspace root；
- [ ] metadata vA/provider vB 的构造场景被拒绝；
- [ ] `RequiredXGo` 覆盖 release/prerelease、`(devel)`/`vX.Y.Z devel` 私有 capability fallback、空值、非法 required、dependency minimum 高于当前 capability，且 E2E 构建真实命名的 `xgo`、不注入伪造 linker version；
- [ ] Dir、单 project file、package path；多文件、`...`、`@version` 明确拒绝；`main.spx -- existing-file` 正确分界；
- [ ] runtime 路径成功和失败均不创建/修改 `xgo_autogen.go`；
- [ ] provider package 非 main、跨 origin module、symlink/错误 dir、未知 protocol；
- [ ] `XGO_GOCMD`、target GOOS/GOARCH、ambient `CGO_ENABLED` 的 unset/0/1、`GOFLAGS=-buildmode=c-shared` 污染；`-modfile`/`-overlay` 一致；`GOFLAGS=-n/-tags/-toolexec` 拒绝；
- [ ] 普通 vendor 项目以及 preflight 已证明无 runtime 的 vendor class 项目行为不变；已命中 runtime 才得到带修复信息的 `ErrRuntimeVendorUnsupported`，且 XGo 不改写 mod policy；
- [ ] graph/metadata 无法确定时 fail closed；只有成功判定无 runtime 才 `NotHandled`；
- [ ] symlink module/project、Windows case difference、Unicode path 使用文件身份比较；
- [ ] 当前所有 build flags 均有显式 policy test；
- [ ] shared codec 的 round-trip/valid/invalid vectors、deterministic argument ordering、action/option 组合和平台长度超限错误；没有 request 文件或 JSON fallback；
- [ ] discovery 后修改 declaring `gox.mod`、非默认 pack directory、本地 replace 下的 project metadata snapshot/digest；变化必须 fail closed，provider 不自行重解析 metadata；
- [ ] `xgo run . --headless "" "a b" --` 的 argv 元素和顺序完全保真；
- [ ] stdin、stdout、stderr；exit `0/1/42`；Unix SIGINT/SIGTERM 断言 `WaitStatus.Signaled/Signal` 而非 `128+S`，Windows Ctrl-C；无 orphan child；
- [ ] provider build failure、crash、返回 0 但无 output、symlink output、rename failure；
- [ ] 所有失败场景中已有 output 不变；
- [ ] 除 preflight 已命中 runtime 后明确不支持的 vendor runtime project 外，普通 XGo（包括未声明 runtime 的 vendor class project）Dir/PkgPath/Files run/build/install 回归零变化；
- [ ] generic fake framework tests 中没有任何 `spx`/`godot` 字符串；无 Pack 的 fake provider 真正调用 shared parser，并通过 run/build/install CLI e2e。

### SPX 必须覆盖

- [ ] 项目 path 含空格、Unicode；session 位于项目外；
- [ ] 真实图片、音频、字体、SVG 都从 `ProjectDir` 内显式 AssetDir/项目内资源目录加载；`.config.extasset` 与 ProjectDir 外引用明确失败；
- [ ] 运行前后项目 `git status --porcelain` 不变，无 `.temp/.godot`；
- [ ] PATH 中没有 `spx` 仍可 provider run；
- [ ] canonical release、local replace、go.work，以及 pseudo-version/versioned remote replace 在 v1 明确失败且不借用本机 runtime；
- [ ] provider release source：offline cache hit 成功，offline miss 不联网且给出 identity/URL；
- [ ] generated launcher：空 cache、断网首次运行仍从 embedded payload 成功；
- [ ] 8–16 并发进程共享 cache，无 partial/temp 泄漏；
- [ ] materialize 中途 kill 后恢复；同大小 cache 篡改被发现；
- [ ] SPX/runtime/ABI/platform/interface-digest mismatch 被拒绝；source mismatch 强制 local Engine bundle；
- [ ] activation 使用新 runtime version，公开 Engine manifest 与 lock 的 `engine_interface_digest` 一致；明确拒绝复用不可变的 `runtime-v2.4.1`；
- [ ] unsupported import/flag 明确失败且无 native fallback；
- [ ] payload/project ZIP 的 traversal、collision、symlink、device、bomb、duplicate manifest、bad payload/manifest/entry hash；manifest entry table 不自引用；
- [ ] project payload 两次构建 SHA-256 相同；若最终 launcher 不同，文档不声称整体可复现；
- [ ] provider/launcher stdin、exit `0/1/42`、SIGINT/Ctrl-C，无 orphan Engine；
- [ ] SPX adapter 调用 shared parser；unknown/duplicate/missing option、残缺 replacement group、run/build positional argument 由唯一 codec 拒绝，live provenance/TOCTOU 与 Pack requirement 由 SPX domain layer 拒绝，且 SPX 不依赖 XGo internal package；
- [ ] 两个 launcher 共用 Engine、不同 bridge/project 时只产生一个 Engine cache entry；GC 不删除持有 lease 的 entry；
- [ ] `-o` 位于 Pack root 时拒绝；其他位置不把旧 final/staging 打进 payload；
- [ ] generated launcher 将首个 `xgo-runtime-v1` argv 当应用参数，不进入 provider mode；
- [ ] macOS amd64/arm64、Linux amd64、Windows amd64 真实 host runner 执行 embedded artifact；Darwin 未签 staging 不被发布，post-link ad-hoc signing 后 `codesign --verify --strict` 通过。
- [ ] activation staging 期间最终 `refs/tags/v3.x.y` 不存在；非-semver bridge assets 与新 runtime 已公开回验后才创建 module tag。

### 自包含产物 gate

新增 `test/RuntimeProviderCI` deterministic fixture：从项目内 Pack/资源目录真实加载图片、音频、字体、SVG，打印唯一 `SPX_RUNTIME_PROVIDER_OK` marker 并主动 exit 0；同一 fixture 另有 `.config.extasset` 和 ProjectDir 外引用的 fail-closed vectors。CI 必须构建后复制/重命名 artifact，在空 cache、平台级阻断网络、且断言 `go`、`xgo`、`spx` 均不可发现的环境运行；所有 host 由 harness 设置 deadline/kill process tree。仅 `env -i` 不算阻断网络。Linux gate 的等价核心为：

```bash
xgo build -o "$TEST_ROOT/game" ./test/RuntimeProviderCI

docker run --rm --network=none \
  -v "$TEST_ROOT:/artifact:ro" \
  -v "$TEST_ROOT/home:/home/runtime" \
  -e HOME=/home/runtime \
  -e XDG_CACHE_HOME=/home/runtime/cache \
  runtime-provider-empty-image \
  /artifact/game --headless
```

`runtime-provider-empty-image` 的构建脚本必须断言 PATH 中不存在三个工具并提供显式 60 秒 timeout。macOS/Windows 使用各自网络 sandbox/firewall、空 PATH/用户 cache 和 Job/process-group deadline 的等价测试。该 gate 必须分别验证空 cache 首次 materialize 与 cache-hit 第二次运行，并检查 success marker。

## 14. Definition of Done

- [ ] mod 的公开 metadata API/shared structural codec、XGo request adapter、SPX live/domain adapter 与 normative protocol 文档一致；没有 provider lifecycle SDK、request file 或 JSON schema；
- [ ] effective graph 是 metadata 与 provider 的唯一来源；
- [ ] CLI runtime 分发覆盖 Dir/PkgPath/Files target 入口并位于 GenGo 前；`tool.RunDir/BuildDir/InstallDir` 保持 legacy 语义；
- [ ] provider identity、protocol、flags、argv、status 和 trust boundary 全部 fail closed；
- [ ] `spx run` 与 provider 共用无全局 cwd/exit 的 interpreted core；
- [ ] 三根路径通过 native resource smoke；
- [ ] 首批支持平台的 SPX-version bridge 已发布并可验证；
- [ ] engine/bridge/project cache 具备完整摘要、跨进程锁/lease、原子目录、恢复与 quota；
- [ ] project bundle 是 allowlist/canonical/safe，而非 repository walk；
- [ ] build/install 对已有 artifact 具备事务保护；
- [ ] self-contained embedded launcher 在空 cache、无工具、无网络 host 上运行；Darwin ad-hoc signature 有真实 gate；
- [ ] 除已命中 runtime 的 vendor 项目明确 unsupported 外，普通 XGo 项目无回归；
- [ ] 文档明确 old-XGo、host-only、ispx import surface、无发布者身份签名、体积和源码明文限制；
- [ ] release/rollback 顺序演练完成；
- [ ] #142 已关闭或明确替代。

## 最终决策建议

接受“declarative project runtime provider + SPX-owned interpreted launcher”方向；拒绝把当前实现估算为两个 command 的轻量改造。它是 3 个仓库、约 9 个可独立验收 PR 的协议与发布链升级。

一期以以下边界落地最稳妥：

- 独立 `runtime v1 <package>`；
- 不兼容旧 XGo；
- effective Go graph 唯一真相；
- generic host provider 保留 ambient CGO policy，SPX provider/launcher 可在 `CGO_ENABLED=0` 下构建；
- SPX release bridge + local source bridge；
- host-only interpreted run/build/install；
- generated pure-Go launcher + build-time embedded verified payload；
- 无 fallback、仅 ad-hoc 签名、源码明文。

只有在 bridge 发布、三根路径、verified materializer 和跨平台进程/输出事务全部通过 gate 后，才在正式 SPX `gox.mod` 启用 runtime。
