# SPX Build Command Guide

The root `Makefile` is the stable, user-facing build interface. It builds and invokes the repository-local `./.bin/buildctl` binary as needed, so normal development should start with `make` commands.

`buildctl` is the lower-level orchestration interface used by the Makefile, CI, and build-system debugging. Its commands are documented below for maintainers, but they are not the primary user workflow.

## Quick start

Prepare the prebuilt host editor and runtime assets, then run a demo:

```sh
make setup
make list-demos
make run DEMO_INDEX=2
```

Prepare Web assets for one runtime mode:

```sh
make setup-web MODE=normal
make runweb DEMO_INDEX=2
```

Build the complete development stack from source when working on the engine or the SPX module:

```sh
GODOT_SRC=/absolute/path/to/godot make dev MODE=normal
```

Run `make help` for the primary commands and `make help-advanced` for lower-level targets. `make buildctl` is optional; normal Make targets build the cached binary automatically.

## Primary commands

| Command | Description |
| --- | --- |
| `make setup` | Set up the host editor and native runtime assets. |
| `make setup-web MODE=...` | Set up Web assets for one runtime mode. |
| `make dev MODE=...` | Build the complete local development stack from source. |
| `make doctor` | Validate the resolved lock, toolchain, module profile, and Godot checkout. |
| `make build-editor` | Build the host editor from source. |
| `make build-desktop` | Build the host desktop template and runtime pack from source. |
| `make build-web MODE=...` | Build a Web template and matching runtime from source. |
| `make build-android` | Build the Android template from source. |
| `make build-ios` | Build the iOS template from source. |

Use `setup` and `setup-web` when prebuilt locked assets are sufficient. Use `dev` or a focused `build-*` target when changing Godot, `godot_modules/spx`, generated bindings, or platform build behavior.

## Build variables

| Variable | Default | Scope |
| --- | --- | --- |
| `GODOT_SRC` | `./godot` | Godot source checkout used by `dev` and the engine `build-*` targets. |
| `SPX_MODULE_SRC` | `./godot_modules/spx` | External SPX Godot module used by engine builds and `make generate`. |
| `MODE` | `normal` | Web setup, build, export, and the Web portion of `dev`. |
| `DEMO_INDEX` | `3` | Tutorial selected by demo commands. |
| `PORT` | `8106` | Port used by Web demo commands. |

Valid `MODE` values are `normal`, `worker`, `minigame`, and `miniprogram`. Keep the same mode through setup, build, run, and export steps so the Web template and runtime match.

`GODOT_SRC` and `SPX_MODULE_SRC` may be absolute or relative paths. Relative paths are resolved from the SPX repository root. Prefer an absolute `GODOT_SRC` when using a Godot checkout outside this repository. Normally leave `SPX_MODULE_SRC` at its default so the repository-owned external module and its build profile remain the single module source.

`GODOT_SRC` is not needed by asset-only setup commands. It is used only when compiling an engine target from source.

## Version and profile sources

Release metadata has one runtime version source: `internal/release/runtime.lock.json` stores `runtime_version` without duplicating a release tag. Release tooling derives the tag as `runtime-v<runtime_version>`, for example `runtime-v2.4.0`.

Local `buildctl` setup reads every pinned tool version from this lock and passes the locked Go toolchain to child build scripts. SPX CI installs Go and XGo from the same location without separate workflow defaults. Installer aliases such as NDK `r23c` are validated adapters derived from the full revision; an unknown mapping fails closed and never becomes a second version source. Godot SCons functional options have a different single source: `godot_modules/spx/spx_scons_profile.json`. Change shared engine feature flags in that profile instead of duplicating them in the Makefile or platform CI workflows.

Godot engine artifacts and the SPX runtime pack have separate build identities:

- Godot engine artifacts are determined by the pinned Godot commit, the `godot_modules/spx` tree (including its SCons profile), the engine toolchain, and platform-specific axes. Changes to `buildctl`, release metadata, or documentation do not invalidate the Godot compilation cache.
- `spx-runtime-assets.zip` is a separate SPX runtime pack generated from `cmd/spx`, the project templates, the SPX Go runtime, and the `runtime export-pack` path. Those changes can require regenerating the pack, but they are not Godot engine source changes.

The current runtime tag publishes both asset classes atomically, so a change to either output still requires a `runtime_version` bump. Unchanged Godot engine inputs nevertheless retain their independent compilation cache and are not rebuilt merely because a version or release-orchestration file changed.

## Focused workflows

### Local SPX development with prebuilt assets

```sh
make setup
make install
spx run --path tutorial/00-Hello
```

### Web development

```sh
make setup-web MODE=worker
make runwebworker DEMO_INDEX=2
```

### Engine and module development

```sh
GODOT_SRC=/absolute/path/to/godot \
SPX_MODULE_SRC=./godot_modules/spx \
make dev MODE=normal
```

For a narrower source build, replace `dev` with `build-editor`, `build-desktop`, `build-web MODE=...`, `build-android`, or `build-ios`.

## Direct buildctl use

Direct `buildctl` calls are intended for CI, build-system maintenance, and debugging the orchestration layer. Build the cached executable first when invoking it yourself:

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

The Makefile maps `setup`, `setup-web`, `dev`, and `build-*` to these commands and supplies defaults consistently. In particular, `make setup` selects the published, manifest-verified runtime pack. Prefer the Make targets unless a CI job needs a direct orchestration step.

## Run and export commands

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

## Generation and maintenance

`make generate` regenerates native and Web bindings, runtime registration, and formatting. It reads and writes the external module selected by `SPX_MODULE_SRC`; do not edit generated files directly.

```sh
make generate
make format
```

`make clean-projects` and `make clean-assets` remove generated or installed data. Review their target scope before running them.
