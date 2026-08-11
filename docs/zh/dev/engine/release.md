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

## 冻结顺序

1. 先合并 Godot 改动并取得最终 commit。merge 或 squash 可能改变 SHA，最终 commit 产生后不要继续保留 PR head pin。
2. 将该 commit 与持久 branch/tag（例如 `spx4.4.1`）一起固定。以下单次操作会校验并同时重写 canonical `runtime.lock.json` 与 current snapshot；更新已有 snapshot 时必须显式确认该 runtime 尚未发布：

   ```sh
   make pin-godot \
     GODOT_SHA=<40-character-sha> \
     GODOT_REF=spx4.4.1 \
     UNPUBLISHED_RUNTIME=1
   python3 .github/scripts/runtime_lock_snapshot.py --check
   ```

   新 runtime version 尚无 snapshot 时省略 `UNPUBLISHED_RUNTIME=1`。它明确表示当前 snapshot 仍可安全覆盖；runtime 一旦公开就绝不能再设置。commit 决定精确的引擎源码，ref 让 shallow fetch 可以取得该 commit；两者都属于 canonical lock。
3. 在 `.github/workflows/release.yml` 中独立使用完整 SHA 固定 reusable workflow 的实现。`uses: ...@SHA` 选择的是 workflow 代码，不是 Godot 源码，不要求与 `godot.commit` 相等。
4. 在 `goplus/spx` 的发布分支冻结 SPX candidate commit。确认 `currentSPXVersion` 通过 `spxRuntimeMappings` 指向与 lock 一致的 runtime definition，并保持历史 definitions/mappings 不变；首个 runtime 按下文自举通过后再合并。

不要从 fork 发布。`publish-runtime` 与 `publish-release` 操作只允许在 lock 的 `release_repository`（当前为 `goplus/spx`）执行。

## 发布前验证

```sh
GODOT_SRC=/absolute/path/to/final-godot make doctor
python3 .github/scripts/runtime_lock_snapshot.py --check
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

- runtime 不存在或仍为 draft 时，CI 跳过基于已发布 runtime 的 Web 产品 smoke，改为把当前 SPX module 放入 lock 的 Godot source，执行 Linux SPX tests 与 Web worker compile smoke。
- runtime 已公开且与当前身份完全一致时，下一次 CI 会跳过 source integration 重编译，强制执行使用已发布资产的 Web normal 产品 smoke。
- 已公开 release 缺 manifest、资产集合或 provenance 不一致，以及 GitHub API 异常，都不属于前两种状态；resolver 与 CI gate 会 fail closed。

这个切换同时避免发布循环与 runtime 重复构建。普通 CI 不构建完整 release runtime；只有 release workflow 执行该构建。release assemble 在复用或发布前仍会下载全部资产，并校验 `SHA256SUMS` 与 manifest 中每个文件的 checksum。

三阶段自举仍在 `goplus/spx` 的冻结发布分支上执行：

| `operation` | 结果 | `platforms` |
| --- | --- | --- |
| `dry-run` | 构建并校验，但不发布 | 通常为 `all` |
| `publish-runtime` | 只发布不可变的 `runtime-v*` 资产 | 忽略 |
| `publish-release` | 发布 runtime、SPX 产品与 npm | 必须为 `all` |

`release_tag` 必须精确等于所选 commit 声明的 SPX tag。这里的产品 `platforms=all` 仅指 Web、macOS、Windows、Linux 包；Android/iOS 属于完整 runtime 资产矩阵和真机 smoke，不是 SPX 产品 target。

1. 先使用 `release_tag=v3.2.0`、`platforms=all`、`operation=dry-run`。下载并检查所有 runtime/product artifacts，完成安装与 demo smoke。
2. 对同一个 commit 设置 `operation=publish-runtime`。没有可复用版本时，该模式会构建、校验并公开全部 runtime 资产，但跳过 SPX 产品包、SPX release 和 npm。
3. 让普通 CI 自动切换到已公开 runtime 路径，并通过 Web normal 产品 smoke；合并时不得改变 module/pack/recipe 身份输入。随后在最终 SPX commit 上使用 `platforms=all`、`operation=publish-release`，流程会验证并复用同一 runtime，再发布 SPX 产品与 npm。

runtime manifest、`SHA256SUMS` 和 lock 的 required asset 集合必须完全一致；已公开 tag 的来源或资产不同会直接失败，不能覆盖。未公开的 runtime/SPX draft tag 必须指向当前 `GITHUB_SHA`；已公开 runtime 可来自前一阶段的 candidate commit，但只有完整复用契约一致时才能用于最终 SPX commit。SPX tag 始终指向最终 commit。如果合并修改了任一 runtime 身份输入，最终运行会拒绝复用，此时必须重新冻结并提升 `runtime_version`。

## 后续版本维护

- 仅 SPX 产品变化且 runtime 两类产物均未变化时，提升 SPX 版本，并新增一条指向既有 runtime 的显式原子映射。
- Godot/module/toolchain 或 runtime pack 输出变化时，提升 `runtime_version`，并为新的 SPX 版本新增显式原子映射。
- 每个原子 runtime definition 都必须有不可变的 `internal/release/runtime_locks/<runtime-version>.json` 快照；校验历史 manifest 时读取对应快照，不能读取更新后的默认 lock。
- 新版本可运行 `python3 .github/scripts/runtime_lock_snapshot.py --sync` 创建缺失的 snapshot。修改 Godot pin 时优先使用 `--pin-godot --godot-commit ... --godot-ref ...`，让 canonical lock 与 snapshot 一次同步。确认 current runtime 尚未发布后，加 `--allow-unpublished-update` 可刷新已有 snapshot；两种模式都只根据 current lock 推导目标文件名，不会触碰其他历史版本。
- runtime 一旦公开，其 snapshot 永久冻结。公开 tag 绝不能使用 `--allow-unpublished-update`；必须提升 `runtime_version`、更新 release mapping，并创建新 snapshot。release setup 会运行 `--check`，package 初始化、drift 与 catalog 测试也会共同校验 current mapping。
