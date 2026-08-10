# SPX Development Documentation

This directory contains development documentation for SPX users and engine contributors.

## Choose by role

### SPX users

- [SPX command-line guide](game/cmd_spx.md)
- [Animation-bound audio](game/animation_audio.md)
- [Project fonts and SVG cluster fallback](project_fonts.md)
- [WeChat Mini Game support](wechat_minigame.md)

### SPX engine developers

- [Architecture](engine/architecture.md)
- [Build commands](engine/cmd_make.md)
- [SPX and Godot runtime release flow](engine/release.md)
- [Binding code generation](engine/code_generator.md)
- [Coordinate systems and the Godot boundary](engine/coordinate_system.md)
- [Physics API design](engine/physic_api.md)
- [Web platform modes](engine/web.md)
- [Web Worker mode](engine/web_worker_mode.md)
- [Web batch synchronization](engine/web_sync_batch.md)
- [Web capture and fixed-frame integration](engine/web_capture.md)
- [Input recording and replay](engine/input_replay.md)

## Platform compatibility notes

- [WeChat Mini Game Go WASM adapter](wxgame_go_wasm_adapter.md)
- [WeChat Mini Game Go WASM panic fix](wechat_minigame_go_wasm_panic.md)
- [Engine copy: Go WASM adapter](engine/web_minigame_go_wasm_adapter.md)
- [Engine copy: Go WASM panic fix](engine/web_minigame_go_wasm_panic.md)

## Quick start

For normal SPX development:

```sh
make setup
make list-demos
make run DEMO_INDEX=2
```

For Web development with prebuilt assets:

```sh
make setup-web MODE=normal
make runweb DEMO_INDEX=2
```

For a complete local engine and Web development environment built from source:

```sh
GODOT_SRC=/absolute/path/to/godot make dev MODE=normal
```

Run `make help` for the primary command list. Asset preparation and source builds use the single `setup`, `setup-web`, and `dev` entry points.

## Contributing

Keep the English and Chinese document trees structurally aligned. Preserve commands, identifiers, paths, and code examples when translating technical documentation.
