# Web Platform Modes

## General notes

SPX Web exports combine the Go WASM runtime, a Godot/Emscripten build, JavaScript glue, and host-page assets. Prepare, build, and export with the same mode to avoid mismatched artifacts.

The build planner currently uses `threads=no` and `proxy_to_pthread=no` for `normal`, `minigame`, and `miniprogram`. Only `worker` enables pthreads and `proxy_to_pthread`.

## `normal`

The standard browser mode. Use it unless a target platform requires another mode.

```sh
make prepare-web MODE=normal
make build-web MODE=normal
make export-web MODE=normal
```

## `worker`

Runs the engine workload through Emscripten `PROXY_TO_PTHREAD` and uses worker-compatible canvas and messaging bridges. Browser APIs that require the main thread must be proxied deliberately. This is not process-level memory or error isolation. Threaded deployment generally requires `SharedArrayBuffer` and cross-origin isolation. See [Web Worker mode](web_worker_mode.md).

## `minigame`

Targets mini-game environments whose JavaScript and WebAssembly implementations differ from a normal browser. The export includes compatibility adapters and platform packaging. The Go launcher facade is a compatibility workaround, not a native `WebAssembly.Instance`; see [the mini-game adapter guide](../wxgame_go_wasm_adapter.md).

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
