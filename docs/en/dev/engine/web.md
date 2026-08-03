# Web Platform Modes

## General notes

SPX Web exports combine the Go WASM runtime, a Godot/Emscripten build, JavaScript glue, and host-page assets. Prepare, build, and export with the same mode to avoid mismatched artifacts.

## `normal`

The standard browser mode. Use it unless a target platform requires another mode.

```sh
make prepare-web MODE=normal
make build-web MODE=normal
make export-web MODE=normal
```

## `worker`

Runs the engine workload in a Web Worker and uses worker-compatible canvas and messaging bridges. Browser APIs that require the main thread must be proxied deliberately. See [Web Worker mode](web_worker_mode.md).

## `minigame`

Targets mini-game environments whose JavaScript and WebAssembly implementations differ from a normal browser. The export includes compatibility adapters and platform packaging.

## `miniprogram`

Targets mini-program integration and its host/runtime constraints. Do not reuse `minigame` artifacts unless the build workflow explicitly supports it.

## Capture and fixed frames

Normal and supported specialized hosts can integrate deterministic screenshot capture and input replay. See [Web capture](web_capture.md) and [input replay](input_replay.md).

## Adaptation rules

- Keep platform-specific code under the corresponding Web template directory.
- Keep generic runtime behavior out of host adapters.
- Feature-detect host APIs and report unsupported features clearly.
- Avoid direct DOM access from worker or mini-game runtime code.
- Preserve generated bridge sections; change their generator or template instead.
- Test every mode affected by shared Web code.
