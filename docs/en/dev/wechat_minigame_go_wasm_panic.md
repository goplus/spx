# Fixing Go WASM String Decoding in WeChat Mini Games

## Problem

The Go WASM runtime can panic while decoding strings in a WeChat Mini Game host when the available `TextDecoder` implementation is missing, incomplete, or incompatible with the browser behavior assumed by `wasm_exec.js`.

## Root cause

Go passes UTF-8 bytes from WASM memory to JavaScript. Standard browser launchers use `TextDecoder`. A mini-game polyfill may mishandle typed-array views, offsets, streaming state, or malformed sequences, causing incorrect strings or an exception during runtime calls.

## Solution

Use a platform-safe UTF-8 decoder and pass the exact WASM memory slice. Do not decode the entire backing buffer when the string occupies only a view.

Problematic pattern:

```js
decoder.decode(new Uint8Array(memory.buffer))
```

Correct boundary pattern:

```js
const bytes = new Uint8Array(memory.buffer, address, length);
const text = decodeUTF8(bytes);
```

`decodeUTF8` should use a verified host `TextDecoder` when available and a standards-compatible fallback otherwise. The fallback must handle multibyte UTF-8 and replacement behavior correctly.

## Implementation

Keep the change in the mini-game runtime adapter or its platform-specific `wasm_exec.js`. Document the upstream Go version on which it is based so upgrades can re-evaluate the patch instead of silently carrying an obsolete copy.

## Verification

Test ASCII, Chinese, emoji/non-BMP characters, combining sequences, empty strings, and slices with nonzero offsets. Exercise both JavaScript-to-Go and Go-to-JavaScript calls on the WeChat simulator and a real device. Confirm that normal Web mode still uses the standard browser path.
