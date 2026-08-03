# Go WASM String-Decoding Fix for Web Mini-Game Hosts

This engine-facing copy summarizes the platform fix described in [the full developer guide](../wechat_minigame_go_wasm_panic.md).

Mini-game environments may lack a browser-compatible `TextDecoder`. Go runtime calls then fail or corrupt text when JavaScript decodes a wrong memory range or uses an incomplete polyfill.

Always create a `Uint8Array` with the exact WASM address and length, then decode it with a verified UTF-8 implementation:

```js
const bytes = new Uint8Array(memory.buffer, address, length);
const text = decodeUTF8(bytes);
```

Keep the fallback in the mini-game adapter, not shared browser code. Test ASCII, Chinese, emoji, combining characters, empty strings, and nonzero-offset views on both simulator and device. Re-evaluate the patch whenever the bundled Go `wasm_exec.js` changes.
