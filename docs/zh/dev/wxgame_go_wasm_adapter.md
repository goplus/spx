# 微信小游戏 Go WASM 兼容层

## 适用范围

本文描述当前 SPX 小游戏导出流程中的 Go WASM 启动兼容层。它针对微信小游戏的 `WXWebAssembly` 与 Go `wasm_exec.js` 之间的类型检查差异，不是通用的 WebAssembly 适配方案。

## 当前导出流程

1. `spx exportminigame` 使用当前 Go 环境的 `$(go env GOROOT)/lib/wasm/wasm_exec.js`，将它复制到生成目录的 `go.wasm.exec.js`。仓库中没有固定的 `js/wasm_exec.js` 文件。
2. 小游戏适配器 [`cmd/spx/template/platform/webminigame/js/adpter.js`](../../../cmd/spx/template/platform/webminigame/js/adpter.js) 将 `GameGlobal.WebAssembly` 指向微信提供的 `WXWebAssembly`。
3. 页面运行代码 [`cmd/ispx/web/game.js`](../../../cmd/ispx/web/game.js) 通过 `WebAssembly.instantiate(url, go.importObject)` 加载 Go wasm，然后创建兼容 facade 并交给 `go.run`。

## 为什么需要 facade

某些 Go 运行时会检查：

```javascript
instance instanceof WebAssembly.Instance
```

微信返回的实例可能来自 `WXWebAssembly.Instance`，因此不能通过这个检查。当前实现保留微信实例的 `exports`，并创建一个拥有标准 `WebAssembly.Instance.prototype` 的 JavaScript 对象：

```javascript
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

## 重要限制

- `compatibleInstance` **不是原生的 `WebAssembly.Instance`**，只是当前 Go 运行时所需的兼容 facade。它没有原生实例的内部槽位，也不能据此推断所有 WebAssembly API 都可用。
- 该实现依赖当前 Go `wasm_exec.js` 只通过 `instance.exports` 访问实例。升级 Go 或更换小游戏运行时后，必须重新验证 `instanceof`、内存访问和导出函数调用。
- `exports` 必须来自刚刚实例化的真实微信实例，不要手动伪造导出对象，也不要用 `Proxy` 替代 facade。
- 如果兼容层失败，应先保留导出的 `go.wasm.exec.js` 和运行时版本，再在对应版本的导出产物上定位问题；不要修改不存在的仓库路径。

## 验证清单

在微信开发者工具和真机上分别验证：

1. `go.run` 不再报告 `WebAssembly.Instance expected`。
2. `instance.exports.mem`、`getsp` 和 Go 导出的业务函数均可调用。
3. 页面退出、panic 和重新启动流程仍能触发 SPX 的错误回调。
4. 升级 Go、微信基础库或 SPX 模板后重新执行以上检查。
