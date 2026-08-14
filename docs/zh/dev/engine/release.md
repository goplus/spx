# SPX 与 Godot runtime 发布流程

SPX 发布与 Godot runtime 使用两套版本，并由同一受控流程按显式映射配对。当前发布声明可用以下命令查看：

```sh
go run ./.github/scripts/runtime_version.go --json
```

只需要单值的脚本仍可使用 `--spx-version`、`--runtime-tag`，或默认的 runtime version 输出。发布 CI 使用 `--github-output` 一次描述已校验的 lock 与 release mapping，不再通过多次 JSON 查询重复拼装。

外置模块首个原子版本为 SPX `v3.2.0`、runtime `2.4.0`（tag `runtime-v2.4.0`）、runtime ABI `2`。历史 `v3.1.0 -> Godot spx2.3.0` 映射保持 legacy，不得移动或重新解释旧 tag。

## 发布身份边界

| 身份/产物 | 内容或复用输入 |
| --- | --- |
| Godot engine/editor/template | Godot commit、`godot_modules/spx` tree（含 SCons profile）、引擎工具链、平台参数 |
| `spx-runtime-assets.zip` | SPX runtime pack source、固定的 pack build recipe、生成 pack 所用的锁定 Godot 引擎 |
| 完整 runtime release | canonical `runtime.lock.json` SHA、module tree、pack source、build recipe、全部资产 checksum |
| SPX 产品包 | SPX release commit、选定的原子 runtime、各平台打包流程 |

SPX 与 Godot Actions 都调用所选 SPX commit 中的 `.github/scripts/runtime_build_contract.py` 校验 lock 与 SCons profile。引擎 cache 的工具链摘要按平台收敛：native 只包含 SCons，Web 额外包含 EMSDK，Android 额外包含 JDK 与 NDK。未知的 NDK installer alias 只会阻断 Android 构建，不会误伤其他平台。

manifest 分别记录 `module_tree`、`runtime_pack_source_sha256` 和 `build_recipe_sha256`。完整 lock SHA 也是 runtime release 的复用契约，因此 ABI、required assets、repository/manifest、Godot ref/version/commit、module path 或任一工具链字段变化都会拒绝复用旧 runtime。Godot SCons cache 使用更窄的独立身份；只改版本号、release 元数据、资产清单或文档不会重新编译 Godot，但可能要求新的 runtime release 身份。文档本身不进入这两类 runtime digest。

pack build recipe 只覆盖 `.github/scripts/build_runtime_pack.sh` 与实际参与生成产物的 buildctl 实现。workflow 排版、checkout/upload/download 等外部 Action 版本属于 CI 运输层，不进入该摘要；Dependabot 升级这些 Action 不需要提升 runtime 版本。修改 pack 脚本或相关 buildctl 代码仍会严格拒绝复用旧 runtime。

## 冻结顺序

1. 先选定精确的 Godot candidate commit，以及最终承载它的持久 branch/tag（例如 `spx4.4.1`）。在该 commit 进入 canonical ref 之前，只允许为尚未发布的 runtime 固定它并执行 exact-SHA dry-run：

   ```sh
   make pin-godot-candidate GODOT_SHA=<40-character-sha>
   python3 .github/scripts/runtime_lock_snapshot.py check
   ```

   该命令默认保留 lock 当前的 ref；只有切换持久 branch/tag 时才传 `GODOT_REF=<branch-or-tag>`。`pin-godot-candidate` 由操作者明确声明 current runtime 尚未发布、允许替换其 snapshot，并允许使用远端已验证的 pre-merge candidate；它不能削弱 release verifier 或授权发布，runtime 公开后绝不能再使用。等价的底层命令是 `python3 .github/scripts/runtime_lock_snapshot.py pin-godot <sha> --premerge`。
2. 在 `.github/workflows/release.yml` 中独立使用完整 SHA 固定 reusable workflow 的实现。`uses: ...@SHA` 选择的是 workflow 代码，不是 Godot 源码，不要求与 `godot.commit` 相等，也不受 source ancestry 规则约束。
3. 在 `goplus/spx` 的发布分支冻结 SPX candidate commit。确认 `currentSPXVersion` 通过 current lock 解析，并保持历史 mapping 与 snapshot 不变；atomic runtime definition 由这些 snapshot 自动派生，不再手写重复。执行 release `dry-run`；它会构建 lock 中精确的 Godot commit，并在 workflow summary 标出 ancestry 结果。只有 verifier 已确认可获取、以 canonical ref tip 为祖先但尚未被该 ref 包含的 commit，才会标记为 candidate-only；无关 commit、ref 无法解析或比较失败会直接阻断 dry-run。
4. 将该 Godot commit 原样 merge 或提升到 `godot.ref`，然后校验发布边界：

   ```sh
   python3 .github/scripts/runtime_lock_snapshot.py verify-godot
   ```

   普通 merge 会保留 candidate 的祖先关系；squash 或 rebase 会改变源码身份，此时运行 `make pin-godot-unpublished GODOT_SHA=<resulting-sha>` 固定最终 commit 并重跑 dry-run，不能发布旧 candidate SHA。
5. ancestry verifier 成功后才能发布新的 runtime。resolver 判定必须构建 runtime 时，两种 publish 操作都会先在 release setup 中运行同一个严格 verifier，再启动 Godot/runtime 构建；长构建结束后，`publish-runtime` 还会在创建或上传 release 前立即复验。已经完整校验的公开 runtime 属于不可变复用，不要求其历史 source ref 永久存在。

不要从 fork 发布。`publish-runtime` 与 `publish-release` 操作只允许在 lock 的 `release_repository`（当前为 `goplus/spx`）执行。

## 发布前验证

```sh
GODOT_SRC=/absolute/path/to/final-godot make doctor
python3 .github/scripts/runtime_lock_snapshot.py check
python3 .github/scripts/runtime_lock_snapshot.py verify-godot
go list ./... | grep -v '/internal/webffi' | xargs go test
git diff --check
```

此外至少完成以下验证：

- 使用 lock 的最终 Godot SHA，以 `tests=yes` 编译并执行 Godot callback/module 测试。
- native editor 与 runtime demo smoke；Web normal、worker、minigame、miniprogram 模式 smoke。
- 录屏 live/offline、音频、SVG/复杂字体回归；Android/iOS 真机 smoke。
- Windows 发布要求 ANGLE 时，确认 ANGLE 下载失败会使构建失败，而不是静默降级。

## runtime 感知的 CI 与三阶段自举

普通 CI 会在启动 runtime consumer 前解析 lock 对应的 runtime release。resolver 只读取 release metadata 与 manifest，不下载全部 runtime 资产；它会校验精确的资产名集合、lock、module tree、runtime-pack source digest 与 build-recipe digest。

- runtime 不存在或仍为 draft 时，CI 跳过基于已发布 runtime 的 Web 产品 smoke，改为把当前 SPX module 放入 lock 的 Godot source，执行 Linux SPX tests 与 Web normal compile smoke。
- runtime 已公开且与当前身份完全一致时，下一次 CI 会跳过 source integration 重编译，强制执行使用已发布资产的 Web normal 产品 smoke。
- 已公开 release 缺 manifest、资产集合或 provenance 不一致，以及 GitHub API 异常，都不属于前两种状态；resolver 与 CI gate 会 fail closed。

这个切换同时避免发布循环与 runtime 重复构建。普通 CI 不构建完整 release runtime；只有 release workflow 执行该构建。release assemble 在复用或发布前仍会下载全部资产，并校验 `SHA256SUMS` 与 manifest 中每个文件的 checksum。

canonical-ref ancestry 规则刻意只用于 release。普通 runner 与 module-integration workflow 可以测试 lock 中精确的 candidate SHA，不要求它已经进入 `godot.ref`。不存在可复用的公开 runtime 时，release setup 会调用共享 verifier：只有 verifier 已确认 exact commit 可从 canonical repo 获取、canonical ref tip 是该 commit 的祖先且反向尚不成立时，`dry-run` 才能以 pre-merge candidate 继续，并在 workflow summary 明确标记 candidate-only；两种 publish 操作都必须先证明 ancestry 才会开始构建。ref 查询失败、ref 歧义、网络异常或比较失败会阻断所有 release 操作，不能伪装成 candidate 结果。新 runtime 构建结束后，publish job 还会在发布前立即复验。如果 resolver 已完整校验不可变的公开 runtime，summary 会标记 source ancestry 无需检查，release 不再依赖历史 ref。

三阶段自举仍在 `goplus/spx` 的冻结发布分支上执行：

| `operation` | 结果 | `platforms` |
| --- | --- | --- |
| `dry-run` | 构建并校验精确的锁定 candidate，但不发布；报告 canonical-ref ancestry | 通常为 `all` |
| `publish-runtime` | 只发布不可变的 `runtime-v*` 资产 | 忽略 |
| `publish-release` | 发布 runtime、SPX 产品与 npm | 必须为 `all` |

`release_tag` 必须精确等于所选 commit 声明的 SPX tag。这里的产品 `platforms=all` 仅指 Web、macOS、Windows、Linux 包；Android/iOS 属于完整 runtime 资产矩阵和真机 smoke，不是 SPX 产品 target。

1. 先对 lock 中精确的 Godot candidate 使用 `release_tag=<当前声明的-SPX-tag>`、`platforms=all`、`operation=dry-run`。检查 workflow summary：pre-merge candidate 在此阶段可以继续，但会明确标记为尚不可发布。下载并检查所有 runtime/product artifacts，完成安装与 demo smoke。
2. 将该 Godot commit 原样提升到 canonical `godot.ref` 并通过严格 ancestry verifier，再对同一个 SPX commit 设置 `operation=publish-runtime`。没有可复用版本时，该模式会构建、校验并公开全部 runtime 资产，但跳过 SPX 产品包、SPX release 和 npm。
3. 让普通 CI 自动切换到已公开 runtime 路径，并通过 Web normal 产品 smoke；合并时不得改变 module/pack/recipe 身份输入。随后在最终 SPX commit 上使用 `platforms=all`、`operation=publish-release`，流程会验证并复用同一 runtime，再发布 SPX 产品与 npm。

runtime manifest、`SHA256SUMS` 和 lock 的 required asset 集合必须完全一致；已公开 tag 的来源或资产不同会直接失败，不能覆盖。未公开的 runtime/SPX draft tag 必须指向当前 `GITHUB_SHA`；已公开 runtime 可来自前一阶段的 candidate commit，但只有完整复用契约一致时才能用于最终 SPX commit。SPX tag 始终指向最终 commit。如果合并修改了任一 runtime 身份输入，最终运行会拒绝复用，此时必须重新冻结并提升 `runtime_version`。

## 开发版 npm 包

`publish-dev-npm` 是独立的按需操作，不属于上述三阶段正式发版。它只允许从 canonical `goplus/spx` 的 `dev` 分支触发；`release_tag` 必须留空，`platforms` 会被忽略：

```sh
gh workflow run release.yml \
  --repo goplus/spx \
  --ref dev \
  -f operation=publish-dev-npm
```

该操作固定使用 dispatch 捕获的精确 commit SHA，构建 Web normal 包，计算 pseudo-version，并发布到 npm `dev` dist-tag。它不会构建 release runtime、平台产品包或创建 GitHub Release；Web 构建仍会下载 lock 对应的公开 runtime，因此该 runtime release 必须已经公开且完整，否则依赖准备会失败。如果目标 SHA 恰好已有正式 `v3.*` tag，则会 fail closed。开发版 npm 与正式发布共用 `spx-release` 并发组，避免不同 SHA 或正式/开发发布同时竞争 dist-tag。

不要恢复 `dev` 每次 push 自动发布。显式操作可以避免 runtime lock 已推进但对应 runtime 尚未公开时反复失败，也避免为每次合入产生不可撤销的 npm 版本。npm Trusted Publisher 应固定为 organization `goplus`、repository `spx`、workflow filename `release.yml`，environment 留空；正式版与开发版都通过该顶层 workflow 的 OIDC 身份发布。`publish_web_package.yml` 只是 reusable workflow，不应单独触发。

## 后续版本维护

- 仅 SPX 产品变化且 runtime 两类产物均未变化时，可以用 SPX-only mapping 保留 current runtime，但必须先由 release dry-run 证明公开 runtime 的完整 provenance 可复用；`bump-release` 不会在本地擅自做这个复用决定。
- Godot、`godot_modules/spx`、toolchain 或 runtime pack 输出变化时，用一个事务同时提升两套身份：

  ```sh
  make bump-release SPX_VERSION=v3.x.y RUNTIME_VERSION=x.y.z
  ```

  只有 runtime ABI 本身变化时才额外传 `RUNTIME_ABI=N`。该命令会先通过已认证的 `gh` API 只读检查确认两项 current release 已公开、两项目标 release/tag 均未占用，再归档上一条 SPX mapping、推进 current lock、创建新的不可变 snapshot，并执行 release metadata 测试；它不会发布，也不会修改 Godot pin。current pair 仍是未发布 candidate 时，应继续使用原版本，不能把它归档成发布历史。
- 每个原子 runtime definition 都必须有不可变的 `internal/release/runtime_locks/<runtime-version>.json` 快照；校验历史 manifest 时读取对应快照，不能读取更新后的默认 lock。
- 新 runtime version 创建后，严格固定使用 `make pin-godot GODOT_SHA=<sha>`；commit 已进入 lock ref 且需要替换未发布 snapshot 时使用 `make pin-godot-unpublished GODOT_SHA=<sha>`；合并前 dry-run candidate 使用 `make pin-godot-candidate GODOT_SHA=<sha>`。三者默认保留 current lock ref，只有传入 `GODOT_REF=...` 才会切换，并且只根据 current lock 推导 snapshot 文件名，不会触碰其他历史版本。
- 项目 `go.mod` scaffold 会自动渲染当前声明的 SPX 版本。`internal/cmd/codegen/go.mod` 中的 `v3.0.0` 只是本地 `replace` 所需的 major-version floor，发布时不要修改这两个文件。
- runtime 一旦公开，其 snapshot 永久冻结。公开 tag 绝不能使用 `sync --unpublished`、`pin-godot-unpublished` 或 `pin-godot-candidate`；必须通过 `make bump-release` 使用新的 runtime version。release setup 会运行 `check`，package 初始化、drift 与 catalog 测试也会共同校验 current mapping。
