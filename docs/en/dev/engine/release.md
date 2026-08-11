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

1. Merge the Godot changes first and record the final commit. A merge or squash can change the SHA, so do not keep a PR-head pin after the final commit is available.
2. Pin that commit together with a durable branch or tag, such as `spx4.4.1`. This single operation validates and rewrites canonical `runtime.lock.json` plus its current snapshot; updating an existing snapshot requires an explicit unpublished-runtime acknowledgement:

   ```sh
   make pin-godot \
     GODOT_SHA=<40-character-sha> \
     GODOT_REF=spx4.4.1 \
     UNPUBLISHED_RUNTIME=1
   python3 .github/scripts/runtime_lock_snapshot.py --check
   ```

   Omit `UNPUBLISHED_RUNTIME=1` when the new runtime version has no snapshot yet. It is an explicit acknowledgement that replacing the current snapshot is safe; never set it after that runtime is public. The commit determines the exact engine source, while the ref makes that commit reachable to shallow fetches; both are part of the canonical lock.
3. Independently pin the reusable-workflow implementation with a full SHA in `.github/workflows/release.yml`. That `uses: ...@SHA` selects workflow code; it does not select the Godot source tree and does not need to equal `godot.commit`.
4. Freeze the SPX candidate commit on a release branch in `goplus/spx`. Confirm that `currentSPXVersion` resolves through `spxRuntimeMappings` to the runtime definition matching the lock, and that historical definitions and mappings are unchanged; merge only after the bootstrap below succeeds.

Never publish from a fork. The `publish-runtime` and `publish-release` operations are allowed only in the lock's `release_repository`, currently `goplus/spx`.

## Pre-release verification

```sh
GODOT_SRC=/absolute/path/to/final-godot make doctor
python3 .github/scripts/runtime_lock_snapshot.py --check
go list ./... | grep -v '/internal/webffi' | xargs go test
git diff --check
```

Also complete at least these checks:

- Build with `tests=yes` at the final locked Godot SHA and run the Godot callback/module tests.
- Smoke-test the native editor and runtime demos plus Web normal, worker, minigame, and miniprogram modes.
- Regress live/offline capture, audio, SVG/complex fonts, and perform Android/iOS device smoke tests.
- If Windows releases require ANGLE, ensure an ANGLE download failure fails the build instead of silently downgrading it.

## Runtime-aware CI and three-stage bootstrap

Ordinary CI resolves the locked runtime release before starting a runtime consumer. The resolver reads release metadata and the manifest only; it validates the exact asset-name set, lock, module tree, runtime-pack source digest, and build-recipe digest without downloading every runtime asset.

- If the runtime is absent or still a draft, CI skips the published Web product smoke and instead builds the current SPX module with the locked Godot source, runs the Linux SPX tests, and performs the Web worker compile smoke.
- If the runtime is public and matches the current identity, the next CI run skips that source integration rebuild and requires the Web normal product smoke against the published assets.
- A public release with a missing manifest, a different asset set/provenance, or a GitHub API error is neither state: resolution fails closed and the CI gate fails.

This switch avoids both the publication circular dependency and duplicate runtime builds. Ordinary CI never builds a complete release runtime; only the release workflow does that. The release assembly still downloads every asset and verifies `SHA256SUMS` plus every manifest checksum before reuse or publication.

Use a frozen release branch in `goplus/spx` for the bootstrap operations:

| `operation` | Result | `platforms` |
| --- | --- | --- |
| `dry-run` | Build and verify without publishing | Usually `all` |
| `publish-runtime` | Publish only the immutable `runtime-v*` bundle | Ignored |
| `publish-release` | Publish runtime, SPX products, and npm | Must be `all` |

`release_tag` must exactly equal the SPX tag declared by the selected commit. Product `platforms=all` means Web, macOS, Windows, and Linux packages; Android and iOS belong to the complete runtime asset matrix and device smoke tests, not the SPX product targets.

1. Run `release_tag=v3.2.0`, `platforms=all`, and `operation=dry-run`. Download and inspect every runtime/product artifact, then complete install and demo smoke tests.
2. Rerun the same commit with `operation=publish-runtime`. If no reusable runtime exists, this mode builds, verifies, and publishes the complete runtime asset set while skipping SPX products, the SPX release, and npm.
3. Let ordinary CI automatically switch to the public-runtime path and pass the Web normal product smoke. Merge without changing any module/pack/recipe identity input, then run the final SPX commit with `platforms=all` and `operation=publish-release`. The workflow verifies and reuses that runtime before publishing SPX products and npm.

The runtime manifest, `SHA256SUMS`, and the lock's required asset set must match exactly. A public tag with different provenance or assets fails rather than being overwritten. An unpublished runtime/SPX draft tag must target the current `GITHUB_SHA`; a public runtime may target the candidate commit from the previous stage, but only an identical full reuse contract allows the final SPX commit to consume it. The SPX tag always targets the final commit. If the merge changes any runtime identity input, the final run rejects reuse; freeze again and bump `runtime_version` instead.

## Maintaining later versions

- If only SPX products change and both runtime artifact classes remain identical, bump SPX and add an explicit atomic mapping to the existing runtime.
- If Godot/module/toolchain inputs or runtime pack output changes, bump `runtime_version` and add an explicit atomic mapping for the new SPX version.
- Every atomic runtime definition must have an immutable lock snapshot at `internal/release/runtime_locks/<runtime-version>.json`. Consumers use that snapshot, rather than the newer default lock, when validating a historical manifest.
- For a new version, `python3 .github/scripts/runtime_lock_snapshot.py --sync` creates the missing snapshot. Prefer `--pin-godot --godot-commit ... --godot-ref ...` when changing the Godot pin so the canonical lock and snapshot move together. While that current runtime is confirmed unpublished, add `--allow-unpublished-update` to refresh an existing snapshot. Both modes derive the filename from the current lock and never touch other versions.
- Once a runtime is public, freeze its snapshot permanently. Never use `--allow-unpublished-update` for a public tag; bump `runtime_version`, update the release mapping, and create a new snapshot instead. Release setup runs `--check`, and package initialization plus drift/catalog tests enforce the current mapping.
