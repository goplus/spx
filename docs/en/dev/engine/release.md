# SPX and Godot Runtime Release Flow

SPX and the Godot runtime use separate versions and are paired by one controlled workflow through explicit mappings. Inspect the declaration with:

```sh
go run ./.github/scripts/runtime/version.go --json
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

The module tree remains a strict digest of the complete `godot_modules/spx` tree. The pack-source digest is narrower: it follows the dedicated pack-only `spx exportpack` inputs and evaluates Go build constraints for the fixed Linux/amd64 CGO pack builder with no extra tags. It projects `export_presets.cfg` to the Linux preset, while hashing packaged files such as `gdspx.gdextension` in full. The pack-only path writes the PCK directly and does not require a platform export template. Independent commands such as `run`, `buildlauncher`, Web/mobile exporters, platform templates, and release orchestration stay outside it. The build-recipe digest follows the dedicated local Linux engine preparation and export-pack path; other platform dispatch, remote transport, local-manifest publication, and CI transport stay outside it. Changes to any selected input still reject reuse of the old runtime.

## Freeze order

1. Select the exact Godot candidate commit and the durable branch or tag that will own it, such as `spx4.4.1`. Before that commit reaches the canonical ref, an unpublished runtime may pin it for an exact-SHA dry-run only:

   ```sh
   make pin-godot-candidate GODOT_SHA=<40-character-sha>
   python3 .github/scripts/runtime_lock_snapshot.py check
   ```

   The command retains the current lock ref; provide `GODOT_REF=<branch-or-tag>` only when changing it. `pin-godot-candidate` explicitly asserts that the current runtime is unpublished, permits replacing its snapshot, and allows a remotely verified pre-merge candidate. It cannot weaken the release verifier or authorize publication, and must never be used after that runtime is public. The equivalent low-level command is `python3 .github/scripts/runtime_lock_snapshot.py pin-godot <sha> --premerge`.
2. Independently pin the reusable-workflow implementation with a full SHA in `.github/workflows/release.yml`. That `uses: ...@SHA` selects workflow code; it does not select the Godot source tree, does not need to equal `godot.commit`, and is outside the source-ancestry rule.
3. Freeze the SPX candidate commit on a release branch in `goplus/spx`. Confirm that `currentSPXVersion` resolves through the current lock while historical mappings and snapshots remain unchanged. Atomic runtime definitions are derived from those snapshots rather than repeated by hand. Run the release `dry-run`; it builds the exact locked Godot commit and marks the ancestry result in the workflow summary. A fetchable commit proven to descend from the canonical ref, but not yet be contained by it, is labeled candidate-only and cannot be published; an unrelated commit, unresolved ref, or comparison failure stops the dry-run.
4. Merge or otherwise promote that exact Godot commit into `godot.ref`, then verify the publication boundary:

   ```sh
   python3 .github/scripts/runtime_lock_snapshot.py verify-godot
   ```

   A normal merge preserves the candidate as an ancestor. A squash or rebase changes the source identity; run `make pin-godot-unpublished GODOT_SHA=<resulting-sha>` and repeat the dry-run instead of publishing the old candidate SHA.
5. Only after the ancestry verifier succeeds may a new runtime be published. When the resolver says the runtime must be built, both publish operations execute the same strict verifier in release setup before the Godot/runtime build, and `publish-runtime` verifies it again after the long build immediately before creating or uploading the release. A fully verified public runtime is immutable reuse and does not depend on its historical source ref remaining available.

Never publish from a fork. The `publish-runtime` and `publish-release` operations are allowed only in the lock's `release_repository`, currently `goplus/spx`.

## Pre-release verification

```sh
GODOT_SRC=/absolute/path/to/final-godot make doctor
python3 .github/scripts/runtime_lock_snapshot.py check
python3 .github/scripts/runtime_lock_snapshot.py verify-godot
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

- If the runtime is absent or still a draft, CI skips the published Web product smoke and instead builds the current SPX module with the locked Godot source, runs the Linux SPX tests, and performs the Web normal compile smoke.
- If the runtime is public and matches the current identity, the next CI run skips that source integration rebuild and requires the Web normal product smoke against the published assets.
- A public release with a missing manifest, a different asset set/provenance, or a GitHub API error is neither state: resolution fails closed and the CI gate fails.

This switch avoids both the publication circular dependency and duplicate runtime builds. Ordinary CI never builds a complete release runtime; only the release workflow does that. The release assembly still downloads every asset and verifies `SHA256SUMS` plus every manifest checksum before reuse or publication.

The canonical-ref ancestry rule is deliberately release-only. Ordinary runner and module-integration workflows may test an exact locked candidate SHA without requiring that it has already reached `godot.ref`. When no reusable public runtime exists, release setup runs the shared verifier: `dry-run` may continue only after the verifier positively classifies a fetchable exact commit as a pre-merge candidate, and records that state in the workflow summary; both publish operations require verified ancestry before building. Ref lookup, ambiguity, network, and comparison failures fail every release operation rather than being treated as a candidate result. A publish job that built a new runtime checks ancestry again immediately before publication. If the resolver has already verified an immutable public runtime, source ancestry is reported as not required and the release no longer depends on the historical ref.

Use a frozen release branch in `goplus/spx` for the bootstrap operations:

| `operation` | Result | `platforms` |
| --- | --- | --- |
| `dry-run` | Build and verify the exact locked candidate without publishing; report canonical-ref ancestry | Usually `all` |
| `publish-runtime` | Publish only the immutable `runtime-v*` bundle | Ignored |
| `publish-release` | Publish runtime, SPX products, and npm | Must be `all` |

`release_tag` must exactly equal the SPX tag declared by the selected commit. Product `platforms=all` means Web, macOS, Windows, and Linux packages; Android and iOS belong to the complete runtime asset matrix and device smoke tests, not the SPX product targets.

1. Run `release_tag=<declared-SPX-tag>`, `platforms=all`, and `operation=dry-run` against the exact locked Godot candidate. Check the workflow summary: a pre-merge candidate is allowed here but is explicitly marked as not publication-ready. Download and inspect every runtime/product artifact, then complete install and demo smoke tests.
2. Promote that exact Godot commit into the canonical `godot.ref` and run the strict ancestry verifier. Rerun the same SPX commit with `operation=publish-runtime`. If no reusable runtime exists, this mode builds, verifies, and publishes the complete runtime asset set while skipping SPX products, the SPX release, and npm.
3. Let ordinary CI automatically switch to the public-runtime path and pass the Web normal product smoke. Merge without changing any module/pack/recipe identity input, then run the final SPX commit with `platforms=all` and `operation=publish-release`. The workflow verifies and reuses that runtime before publishing SPX products and npm.

The runtime manifest, `SHA256SUMS`, and the lock's required asset set must match exactly. A public tag with different provenance or assets fails rather than being overwritten. An unpublished runtime/SPX draft tag must target the current `GITHUB_SHA`; a public runtime may target the candidate commit from the previous stage, but only an identical full reuse contract allows the final SPX commit to consume it. The SPX tag always targets the final commit. If the merge changes any runtime identity input, the final run rejects reuse; freeze again and bump `runtime_version` instead.

## Development npm package

`publish-dev-npm` is an independent on-demand operation, not a fourth stage of the production release. It is restricted to the canonical `goplus/spx` `dev` branch; leave `release_tag` empty and note that `platforms` is ignored:

```sh
gh workflow run release.yml \
  --repo goplus/spx \
  --ref dev \
  -f operation=publish-dev-npm
```

The operation pins the exact commit SHA captured by the dispatch, builds the Web normal bundle, computes a pseudo-version, and publishes it under the npm `dev` dist-tag. It does not build the release runtime or platform products and does not create a GitHub Release. The Web build still downloads the public runtime selected by the lock, so that runtime release must already be public and complete; otherwise dependency setup fails. A target SHA carrying an exact production `v3.*` tag fails closed. Development npm publication shares the `spx-release` concurrency group with production releases so different SHAs and production/development publication cannot race over dist-tags.

Do not restore publication on every push to `dev`. Explicit publication avoids repeated failures while a newly advanced runtime lock has no public runtime and avoids creating an irreversible npm version for every merge. Configure the npm Trusted Publisher with organization `goplus`, repository `spx`, workflow filename `release.yml`, and no environment. Production and development packages then use the same top-level OIDC identity. `publish_web_package.yml` is reusable-only and must not be dispatched directly.

## Maintaining later versions

- If only SPX products change and both runtime artifact classes remain identical, an SPX-only mapping may retain the current runtime, but the release dry-run must first prove that the public runtime's complete provenance is reusable. `bump-release` deliberately does not make this reuse decision locally.
- If Godot, `godot_modules/spx`, toolchain inputs, or runtime pack output changes, advance both identities in one transaction:

  ```sh
  make bump-release SPX_VERSION=v3.x.y RUNTIME_VERSION=x.y.z
  ```

  Add `RUNTIME_ABI=N` only when the runtime ABI itself changes. The command uses authenticated `gh` API reads to require both current releases to be public and both target release/tag names to be unused before it writes. It then archives the previous SPX mapping, advances the current lock, creates the new immutable snapshot, and runs the release-metadata tests. It never publishes and never changes the Godot pin. If the current pair is still an unpublished candidate, keep those versions instead of archiving them as release history.
- Every atomic runtime definition must have an immutable lock snapshot at `internal/release/runtime_locks/<runtime-version>.json`. Consumers use that snapshot, rather than the newer default lock, when validating a historical manifest.
- After creating a new runtime version, use `make pin-godot GODOT_SHA=<sha>` for a strict pin, `make pin-godot-unpublished GODOT_SHA=<sha>` to replace its unpublished snapshot after the commit reaches the lock ref, or `make pin-godot-candidate GODOT_SHA=<sha>` for a verified pre-merge dry-run candidate. All three retain the current lock ref unless `GODOT_REF=...` is supplied, derive the snapshot filename from the current lock, and never touch other historical versions.
- The project `go.mod` scaffold renders the declared SPX version automatically. The `v3.0.0` requirement in `internal/cmd/codegen/go.mod` is only the major-version floor for its local `replace`; do not bump either file during a release.
- Once a runtime is public, freeze its snapshot permanently. Never use `sync --unpublished`, `pin-godot-unpublished`, or `pin-godot-candidate` for a public tag; use `make bump-release` with a new runtime version instead. Release setup runs `check`, and package initialization plus drift/catalog tests enforce the current mapping.
