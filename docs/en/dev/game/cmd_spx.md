# SPX Command-Line Guide

## Basic usage

```sh
spx <command> [arguments]
spx help
spx version
```

Most project commands accept `--path <directory>` and default to the current directory.

## Command groups

### Project management

| Command | Description |
| --- | --- |
| `help` | Display commands, examples, and flags. |
| `version` | Display the SPX version. |
| `init [directory]` | Create an SPX project. |
| `editor` | Open a project in editor mode. |
| `clear` | Clear generated project data. |
| `clearbuild` | Clear build artifacts. |

### Development

| Command | Description |
| --- | --- |
| `build` | Build the current project. |
| `run` | Run in interpreted mode. |
| `runnative` | Run with the native desktop runtime. |
| `rune` | Import assets and run with the editor runtime. |
| `export` | Export a desktop package. |
| `runm` | Run multiplayer mode. |

### Web development

| Command | Description |
| --- | --- |
| `buildweb` | Build the project's WASM output. |
| `runweb` | Build and serve a Web project. |
| `runwebworker` | Build and serve in Web Worker mode. |
| `exportweb` | Export the standard Web package. |
| `exportwebworker` | Export the Web Worker package. |
| `exporttemplateweb` | Export Web template assets. |
| `stopweb` | Stop the SPX Web server. |

### Platform exports

| Command | Description |
| --- | --- |
| `exportapk` | Export an Android APK. |
| `exportios` | Export an iOS package. |
| `exportminigame` | Export a mini-game package. |
| `exportminiprogram` | Export a mini-program package. |
| `exportbot` | Export a bot package. |
| `buildtinygo` | Build a TinyGo static library for the selected board. |

## Command details

### Create a project

```sh
spx init
spx init ./games/demo
```

### Build and run

```sh
spx build
spx build --servermode
spx run
spx run --path ./games/demo
spx runnative --path ./games/demo
spx rune --path ./games/demo
```

`run` uses interpreted mode. `runnative` requires prepared host runtime assets. `rune` uses the editor/import runtime and is useful when assets must first be imported by Godot.

### Desktop export

```sh
spx export
spx export --fullscreen
```

The host must have the matching desktop export template/runtime assets.

### Multiplayer

```sh
spx runm
spx runm --onlys
spx runm --onlyc --serveraddr 127.0.0.1:8080
```

### Web

```sh
spx buildweb
spx runweb
spx runweb --debugweb
spx runwebworker
spx exportweb
spx stopweb
```

Web commands require prepared Web runtime assets. Worker exports also require worker-mode assets.

### Android and iOS

```sh
spx exportapk
spx exportapk --install
spx exportios
```

Android export requires the Android SDK/NDK and JDK. iOS export requires the Apple toolchain and signing setup. Engine templates must match the SPX release.

### Mini-game targets

```sh
spx exportminigame
spx exportminigame -build=fast
spx exportminiprogram
```

`-build=fast` skips expensive packaging work where supported and is intended for development iterations.

## Common flags

| Flag | Description |
| --- | --- |
| `--path` | Project directory. |
| `--serveraddr` | Multiplayer server address. |
| `--servermode` | Build server mode. |
| `--headless` | Run without a display. |
| `--tags` | Go/XGo build tags. |
| `--nomap` | Disable map-related output where supported. |
| `--install` | Install after export where supported. |
| `--debugweb` | Enable the Web debug service. |
| `--fullscreen` | Export in full-screen mode. |
| `--movie` | Enable movie recording mode. |
| `--goenv` | Use a portable Go environment directory. |
| `-v` | Print verbose diagnostics. |

## Troubleshooting

If a native command cannot find the engine, run `make prepare-host` from the SPX repository or install the correct release assets. For Web failures, prepare the same Web mode used by the command. For Android/iOS errors, first verify the platform SDK and engine template independently. Run with `-v` to expose invoked tools and artifact paths.
