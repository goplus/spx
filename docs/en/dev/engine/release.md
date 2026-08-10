# SPX and Godot Runtime Release Flow

SPX and the Godot runtime use separate versions and are paired by one controlled workflow through explicit mappings. Inspect the declaration with:

```sh
go run ./.github/scripts/runtime_version.go --json
```

The same tool supports `--spx-version`, `--runtime-tag`, and the default runtime-version-only output for scripts that need one value. Release CI uses `--github-output` so the validated lock and release mapping are described once rather than reconstructed with separate JSON queries.

The first atomic external-module release is SPX `v3.2.0`, runtime `2.4.0` (tag `runtime-v2.4.0`), and runtime ABI `2`. The historical `v3.1.0 -> Godot spx2.3.0` mapping remains legacy; never move or reinterpret an existing tag.

## Release identity boundaries

| Identity/artifact | Content or reuse inputs |
| --- | --- |
| Godot engine/editor/templates | Godot commit, `godot_modules/spx` tree (including the SCons profile), engine toolchain, and platform axes |
| `spx-runtime-assets.zip` | SPX runtime pack sources, the pinned pack build recipe, and the locked Godot engine used to export the pack |
| Complete runtime release | Canonical `runtime.lock.json` SHA, module tree, pack source, build recipe, and every asset checksum |
| SPX product packages | SPX release commit, selected atomic runtime, and platform packaging flow |

SPX and Godot Actions both invoke `.github/scripts/runtime_build_contract.py` from the selected SPX commit to validate the lock and SCons profile. Engine cache toolchain digests are platform-scoped: native uses SCons, Web adds EMSDK, and Android adds JDK plus the NDK. An unknown NDK installer alias fails only an Android build; it does not block unrelated platforms.

The manifest records `module_tree`, `runtime_pack_source_sha256`, and `build_recipe_sha256` independently. The full lock SHA is also part of the runtime-release reuse contract, so changing the ABI, required assets, repository/manifest, Godot ref/version/commit, module path, or any toolchain field rejects reuse. Godot SCons caches use a narrower identity: changing only a version, release metadata, an asset list, or documentation does not recompile Godot, although it may require a new runtime-release identity. Documentation itself is outside both runtime digests.

## Freeze order

1. Merge the Godot changes first and record the final commit. The SPX lock pins an exact Godot SHA; if a merge or squash changes it, update `internal/release/runtime.lock.json`.
2. Update the full reusable-workflow SHA in `.github/workflows/release.yml`. The workflow verifies that it exactly matches the lock.
3. Change `godot.ref` to a durable branch or tag that will not disappear with a feature-branch cleanup. The commit determines the actual engine source, but the ref is also part of the canonical lock and must be frozen before generating a manifest.
4. Freeze the SPX candidate commit on a release branch in `goplus/spx`. Confirm that `currentSPXVersion` resolves through `spxRuntimeMappings` to the runtime definition matching the lock, and that historical definitions and mappings are unchanged; merge only after the bootstrap below succeeds.

Never publish from a fork. The `publish-runtime` and `publish-release` operations are allowed only in the lock's `release_repository`, currently `goplus/spx`.

## Pre-release verification

```sh
GODOT_SRC=/absolute/path/to/final-godot make doctor
go list ./... | grep -v '/internal/webffi' | xargs go test
git diff --check
```

Also complete at least these checks:

- Build with `tests=yes` at the final locked Godot SHA and run the Godot callback/module tests.
- Smoke-test the native editor and runtime demos plus Web normal, worker, minigame, and miniprogram modes.
- Regress live/offline capture, audio, SVG/complex fonts, and perform Android/iOS device smoke tests.
- If Windows releases require ANGLE, ensure an ANGLE download failure fails the build instead of silently downgrading it.

## Three-stage bootstrap for a new runtime

Before `runtime-v2.4.0` is public, ordinary platform CI cannot download it. Avoid the circular dependency between CI and publication by using a frozen release branch in `goplus/spx`:

| `operation` | Result | `platforms` |
| --- | --- | --- |
| `dry-run` | Build and verify without publishing | Usually `all` |
| `publish-runtime` | Publish only the immutable `runtime-v*` bundle | Ignored |
| `publish-release` | Publish runtime, SPX products, and npm | Must be `all` |

`release_tag` must exactly equal the SPX tag declared by the selected commit. Product `platforms=all` means Web, macOS, Windows, and Linux packages; Android and iOS belong to the complete runtime asset matrix and device smoke tests, not the SPX product targets.

1. Run `release_tag=v3.2.0`, `platforms=all`, and `operation=dry-run`. Download and inspect every runtime/product artifact, then complete install and demo smoke tests.
2. Rerun the same commit with `operation=publish-runtime`. If no reusable runtime exists, this mode builds, verifies, and publishes the complete runtime asset set while skipping SPX products, the SPX release, and npm.
3. Let ordinary platform CI consume the public runtime and pass. Merge without changing any module/pack/recipe identity input, then run the final SPX commit with `platforms=all` and `operation=publish-release`. The workflow verifies and reuses that runtime before publishing SPX products and npm.

The runtime manifest, `SHA256SUMS`, and the lock's required asset set must match exactly. A public tag with different provenance or assets fails rather than being overwritten. An unpublished runtime/SPX draft tag must target the current `GITHUB_SHA`; a public runtime may target the candidate commit from the previous stage, but only an identical full reuse contract allows the final SPX commit to consume it. The SPX tag always targets the final commit. If the merge changes any runtime identity input, the final run rejects reuse; freeze again and bump `runtime_version` instead.

## Maintaining later versions

- If only SPX products change and both runtime artifact classes remain identical, bump SPX and add an explicit atomic mapping to the existing runtime.
- If Godot/module/toolchain inputs or runtime pack output changes, bump `runtime_version` and add an explicit atomic mapping for the new SPX version.
- Every atomic runtime definition must have an immutable lock snapshot at `internal/release/runtime_locks/<runtime-version>.json`. Consumers use that snapshot, rather than the newer default lock, when validating a historical manifest.
- Before publishing a new runtime, add its snapshot and keep its canonical JSON identical to `runtime.lock.json`; package initialization plus the drift and catalog tests enforce the current mapping. Once that runtime is public, freeze its snapshot permanently. Start the next runtime by changing the default lock and adding a new matching snapshot—never rewrite a published snapshot.
