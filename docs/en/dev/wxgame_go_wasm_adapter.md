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

## Working approach

Normalize the platform result at the adapter boundary and return the exact standard result shape expected by the bundled Go runtime. Where the platform API provides a genuine underlying module/instance, preserve those objects instead of wrapping them with ordinary JavaScript objects. Patch only the platform copy of `wasm_exec.js` or its adapter; do not change normal Web behavior.

Conceptually:

```js
async function instantiateForGo(bytes, imports) {
  const result = await platformInstantiate(bytes, imports);
  return normalizeInstantiateResult(result);
}
```

`normalizeInstantiateResult` must verify the supported platform shapes and return `{ module, instance }` with a genuine instance. It must fail explicitly for an unknown shape.

## Why it works

The adapter translates between two host contracts before Go observes the result. Go receives the same object shape and native identity it expects, while SPX keeps mini-game-specific behavior out of shared runtime code.

## Applicability

Use this pattern for non-browser hosts that provide real WebAssembly execution but return a nonstandard instantiate result. It cannot emulate missing WebAssembly features or manufacture native internal slots from a plain object.

## Notes

- Keep the adapter synchronized with the Go version shipped by SPX.
- Test both compiled modules and raw bytes if both inputs are supported.
- Preserve instantiate errors and stack information.
- Test on a real device/runtime, not only the desktop simulator.
- Never apply the workaround globally to normal browsers.
