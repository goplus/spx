# Web host runtime

This directory owns the web host runtime that bridges `engine.*` assets
and `ispx.wasm`.

Ownership rules:

- `pkg/ispx` owns the interpreter logic and the `ispx.wasm` JavaScript ABI
- `internal/webhost` owns the host-side runtime glue such as `runner.html` and `game.js`
- `cmd/gox/template/platform/*` owns product- or platform-specific entry pages and wrappers
