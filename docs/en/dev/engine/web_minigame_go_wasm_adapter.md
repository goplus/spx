# Go WASM Compatibility Adapter for Web Mini-Game Hosts

This engine-facing copy documents the same compatibility boundary as [the developer-level adapter guide](../wxgame_go_wasm_adapter.md). Read that page for the current SPX implementation: it uses a JavaScript compatibility facade around the platform instance, not a genuine native `WebAssembly.Instance`.

Keep the facade limited to the mini-game template and synchronized with the Go runtime version shipped by SPX. Revalidate simulator and device behavior after changing Go, the WeChat base library, or the generated `go.wasm.exec.js`.
