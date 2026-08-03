# Go WASM Compatibility Adapter for Web Mini-Game Hosts

This engine-facing copy documents the same compatibility boundary as [the developer-level adapter guide](../wxgame_go_wasm_adapter.md).

Go's `wasm_exec.js` expects standard WebAssembly instantiate results. Mini-game hosts may return wrappers with incompatible constructor identity. Changing prototypes or adding a `Proxy` does not provide WebAssembly internal slots and is therefore unreliable.

The supported solution is a platform adapter that recognizes documented host result shapes and returns the genuine module and instance in the exact `{ module, instance }` structure expected by Go. Unknown shapes must fail explicitly. Keep this code limited to mini-game templates and synchronized with the Go runtime version shipped by SPX.

Validate raw-byte and compiled-module inputs where supported, preserve host errors, and test both simulator and device. A compatibility adapter can normalize API shape; it cannot emulate WebAssembly features missing from the host.
