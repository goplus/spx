# WeChat Mini Game Implementation

## Goal

Run SPX's Go WASM and Godot Web runtime in the WeChat Mini Game environment and package projects through the regular SPX export workflow.

## Constraints

The environment is not a complete browser. DOM APIs, canvas creation, WebAssembly wrappers, module loading, file access, and lifecycle behavior differ from standard Web exports. Code that assumes browser constructors or prototype identities can fail even when equivalent host functionality exists.

## Solution

The mini-game export supplies platform adapters for canvas, events, files, WebAssembly instantiation, and host lifecycle. Generic SPX runtime code remains shared; environment-specific behavior stays in `cmd/spx/template/platform/webminigame`.

## Development plan

1. Keep normal Web mode working as the reference implementation.
2. Adapt host APIs behind narrow compatibility modules.
3. Build and export with `MODE=minigame` consistently.
4. Validate startup, input, rendering, audio, storage, and error reporting on an actual WeChat runtime.
5. Keep adapter patches documented and covered by small JavaScript tests where possible.

## Implementation notes

Use the platform-provided WebAssembly implementation rather than assuming browser prototype behavior. Normalize binary inputs before calling Go's `wasm_exec.js`. Avoid DOM access and route lifecycle events through the platform API. Package paths and asset loading must use the mini-game filesystem rules.

See [Go WASM adapter](wxgame_go_wasm_adapter.md) and [string decoding panic fix](wechat_minigame_go_wasm_panic.md) for two runtime-specific compatibility issues.

## References

Consult current WeChat Mini Game documentation for WebAssembly, canvas, filesystem, networking, package limits, and debugging requirements; these platform details can change independently of SPX.
