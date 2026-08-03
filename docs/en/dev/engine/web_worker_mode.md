# SPX Web Worker Mode

## Requirements

Worker mode keeps expensive engine and runtime work away from the browser main thread while preserving input, rendering, audio, lifecycle, and host integration. It addresses main-thread stalls and environments where long-running WASM execution would make the page unresponsive.

## Approaches evaluated

### Emscripten proxy-to-pthread

Godot can be compiled with Emscripten's worker/pthread proxy support. This approach reuses engine integration but depends on browser threading requirements such as cross-origin isolation and shared memory.

### Dedicated Worker and OffscreenCanvas

The alternative explicitly starts the runtime in a worker, transfers an `OffscreenCanvas`, and implements message bridges for APIs that remain on the main thread. This provides clearer ownership but requires more host adaptation.

## Architecture

### Components

- Main thread: DOM, page lifecycle, browser UI, and APIs restricted to the main thread.
- Worker: Go WASM runtime, engine execution, and worker-safe rendering.
- Message manager: request/response and event routing.
- Canvas bridge: transfers or proxies rendering state.
- Input bridge: normalizes browser events into SPX input snapshots.

### Initialization

1. The host creates a worker and the target canvas.
2. If supported, it transfers the canvas with `transferControlToOffscreen`.
3. The host sends runtime URLs, launch configuration, viewport, and environment data.
4. The worker loads JavaScript glue, Go WASM, and the engine.
5. The runtime reports readiness before the game is launched.

### Runtime data flow

Main-thread input is normalized and forwarded to the worker. The worker advances SPX and Godot, renders to the offscreen canvas, and emits lifecycle, error, capture, audio, and host-operation messages. Responses carry correlation identifiers when operations are asynchronous.

## Communication paths

### Main thread and worker

Use structured messages with explicit types and versioned payloads. Transfer large binary buffers instead of cloning them where ownership permits. Never assume delivery and frame execution are synchronous.

### Go and the C++ engine

Generated WASM/JavaScript bindings and batched transfer buffers handle high-frequency runtime state. See [batched synchronization](web_sync_batch.md).

### Main thread and Go runtime

Browser-only operations are represented as bridge requests or events. Keep DOM objects and functions out of worker payloads.

## Input

Pointer, keyboard, wheel, focus, and viewport events originate on the main thread. The host converts coordinates using the current canvas bounds and device scale, then forwards normalized state. The worker consumes a coherent input snapshot at the SPX input tick.

Prevent browser defaults only where the game owns the interaction. Handle blur/focus loss so keys or buttons cannot remain stuck.

## Rendering

Worker rendering requires `OffscreenCanvas` and a compatible WebGL context. Detect WebGL2 first and report a clear failure or supported fallback. Canvas size changes must update both display dimensions and backing-buffer dimensions.

Example Emscripten builds must use the repository build workflow rather than an ad hoc command so SPX-specific flags remain consistent:

```sh
make prepare-web MODE=worker
make build-web MODE=worker
make runwebworker DEMO_INDEX=2
```

## Browser requirements and failure handling

Feature-detect Worker, OffscreenCanvas, WebAssembly, and required WebGL capabilities. Threaded Emscripten variants may also require `SharedArrayBuffer` and cross-origin isolation. Report initialization phases and propagate worker exceptions, promise rejections, engine errors, and termination to the host.

## References

- Emscripten pthread and OffscreenCanvas documentation
- MDN Worker, OffscreenCanvas, WebGL, and cross-origin isolation documentation
- SPX worker templates under `cmd/spx/template/platform/webworker`
