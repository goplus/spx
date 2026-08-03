# SPX Build Command Guide

The root `Makefile` is the public entry point for repository builds. It compiles and invokes `internal/cmd/buildctl`, which owns dependency setup, engine locking, artifact placement, and multi-step workflows.

## Build layers

1. `make` provides stable, discoverable commands.
2. `./.bin/buildctl` orchestrates tools, the engine, runtime artifacts, and workflows.
3. Go, SCons, Emscripten, Android, and Apple toolchains perform platform builds.

Use `make` for normal work. Use `buildctl` directly for CI or orchestration debugging.

## Quick start

```sh
# Precompile the local buildctl binary.
make buildctl

# Show all supported Make commands.
make help

# Prepare editor and native runtime assets.
make prepare-host

# Build a complete local development environment.
GODOT_SRC=/absolute/path/to/godot make build-dev MODE=normal

# Prepare only Web export assets.
make prepare-web MODE=normal

# Prepare host and Web assets together.
make prepare-full MODE=normal
```

## Common variables

| Variable | Meaning | Default |
| --- | --- | --- |
| `GODOT_SRC` | Path to the SPX Godot source tree | `./godot` |
| `MODE` | Web build/export mode | `normal` |
| `DEMO_INDEX` | Tutorial selected by demo workflows | `3` |
| `PORT` | Web development server port | `8106` |
| `WEB` | Install Web tooling with `make install` | `0` |
| `APK_PROJECT_DIR` | Project used by `install-apk` | `tutorial/00-Hello` |

Valid Web modes are `normal`, `worker`, `minigame`, and `miniprogram`.

## Setup commands

| Command | Description |
| --- | --- |
| `make prepare-host` | Download and install host editor/runtime assets. |
| `make prepare-web MODE=...` | Prepare Web export assets for one mode. |
| `make prepare-full MODE=...` | Prepare host and Web assets. |
| `make install [WEB=1]` | Install the `spx` command, optionally with Web tooling. |
| `make download` | Download the runtime engine assets. |
| `make download-engine PLATFORM=...` | Download a platform engine template. |

## Build commands

| Command | Description |
| --- | --- |
| `make build-dev MODE=...` | Build the complete local development stack. |
| `make build-editor` | Build the editor engine. |
| `make build-desktop` | Build the desktop template and runtime pack. |
| `make build-web MODE=...` | Build a Web template. |
| `make build-wasm` | Build the Go WASM runtime. |
| `make build-wasm-opt` | Build an optimized Go WASM runtime. |
| `make build-android` | Build the Android template. |
| `make build-ios` | Build the iOS template. |

## Export and run commands

```sh
make list-demos
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

```sh
GODOT_SRC=/absolute/path/to/godot make generate
make format
```

`make generate` regenerates native/Web bindings, runtime registration, and formatting. Do not edit generated files directly.

`make clean-projects` and `make clean-assets` remove generated or installed data. Review their target scope before running them.

## Recommended workflows

### Local game development

```sh
make prepare-host
make install
spx run --path tutorial/00-Hello
```

### Web-only setup

```sh
make prepare-web MODE=normal
make runweb DEMO_INDEX=2
```

### Engine development

```sh
GODOT_SRC=/absolute/path/to/godot make build-dev MODE=normal
```

Keep `GODOT_SRC` and `MODE` consistent throughout a workflow so that generated bindings and runtime artifacts match.
