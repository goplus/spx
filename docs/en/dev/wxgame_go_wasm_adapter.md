# Go WASM Compatibility Adapter for WeChat Mini Games

## Problem

Go's `wasm_exec.js` expects `WebAssembly.instantiate` to return browser-compatible `WebAssembly.Instance` and `WebAssembly.Module` objects. Some WeChat Mini Game runtimes expose equivalent functionality through platform wrappers whose constructor identity or prototype chain does not satisfy Go's strict checks.

## Root cause

### WebAssembly implementation differences

The mini-game host is not a browser and may wrap native WebAssembly objects. A value can provide exports but still fail `instanceof WebAssembly.Instance` or other assumptions made by the Go runtime.

### Strict Go runtime checks

The Go launcher validates the instantiate result and expects the standard shape before reading exports and starting the runtime.

### Prototype and constructor identity

JavaScript prototype manipulation does not change an object's underlying host identity. Objects from different realms or native wrapper implementations may also have incompatible constructor identities.

## Approaches that failed

### Changing the prototype chain

`Object.setPrototypeOf` can make property lookup appear correct but cannot reliably turn a platform wrapper into a genuine WebAssembly instance. Native internal slots remain missing.

### Proxying the object

A `Proxy` can forward properties and exports but does not supply required WebAssembly internal slots or constructor identity. Strict runtime checks still fail.

## Current SPX workaround

The current SPX mini-game path loads the module with the platform `WebAssembly.instantiate`, then creates a JavaScript compatibility facade for the Go launcher's `instanceof` check:

```js
const wasmResult = await WebAssembly.instantiate(url, go.importObject);
const compatibleInstance = Object.create(WebAssembly.Instance.prototype);
compatibleInstance.exports = wasmResult.instance.exports;
Object.defineProperty(compatibleInstance, "constructor", {
  value: WebAssembly.Instance,
  writable: false,
  enumerable: false,
  configurable: true,
});
await go.run(compatibleInstance);
```

This facade is **not a native `WebAssembly.Instance`**. It only works because the current bundled Go runtime reads the copied `exports` object and does not require native instance internal slots after the type check. Do not describe it as a genuine instance, and revalidate it whenever the Go runtime or mini-game host changes. Patch only the platform export path; do not change normal Web behavior.

## Why it works

The adapter translates between two host contracts before Go observes the result. Go receives a compatible JavaScript shape, while SPX keeps mini-game-specific behavior out of shared runtime code. The facade cannot emulate missing WebAssembly features or native internal slots.

## Applicability

Use this pattern for non-browser hosts that provide real WebAssembly execution but return a nonstandard instantiate result. It cannot emulate missing WebAssembly features or manufacture native internal slots from a plain object.

## Notes

- The exporter copies `$(go env GOROOT)/lib/wasm/wasm_exec.js` to `go.wasm.exec.js`; there is no fixed `js/wasm_exec.js` source file in this repository.
- Keep the adapter synchronized with the Go version shipped by SPX.
- Test both compiled modules and raw bytes if both inputs are supported.
- Preserve instantiate errors and stack information.
- Test on a real device/runtime, not only the desktop simulator.
- Never apply the workaround globally to normal browsers.
