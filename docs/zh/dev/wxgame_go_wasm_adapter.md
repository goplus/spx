# 微信小程序 Go WASM 兼容性解决方案

## 问题概述

在微信小程序环境中运行 Go WebAssembly (WASM) 时，会遇到类型兼容性错误：

```
Error: Go.run: WebAssembly.Instance expected
```

尽管 WASM 实例功能正常，但 Go 运行时拒绝接受微信小程序创建的 WebAssembly.Instance 对象。

## 问题根本原因

### 1. 微信小程序的 WebAssembly 实现差异

**标准浏览器环境**：
```javascript
globalThis.WebAssembly = {
  Instance: WebAssembly.Instance,        // 标准构造函数
  instantiate: WebAssembly.instantiate   // 标准实例化函数
}
```

**微信小程序环境**：
```javascript
globalThis.WebAssembly = {
  Instance: WXWebAssembly.Instance,      // 微信定制版本
  instantiate: WXWebAssembly.instantiate // 微信定制版本
}
```

**关键差异**：
- 微信小程序使用了自己的 `WXWebAssembly` 实现
- `WXWebAssembly.instantiate` 只支持文件路径，不支持 ArrayBuffer
- 创建的实例对象虽然功能正常，但**类型身份不被 Go 运行时认可**

### 2. Go 运行时的严格类型检查

Go 运行时在 `go_wasm_exec.js` 中进行严格的类型检查：

```javascript
// Go 运行时源码（简化）
if (!(instance instanceof WebAssembly.Instance)) {
  throw new Error("Go.run: WebAssembly.Instance expected");
}
```

**问题表现**：
```javascript
// 微信小程序创建的实例
console.log("instance instanceof WebAssembly.Instance:", false)  // ❌ 失败
console.log("instance.exports:", {mem: Memory(600), run: ƒ, ...}) // ✅ 功能正常
```

### 3. 原型链和构造函数问题

**微信小程序实例的原型链**：
```
微信实例 → WXWebAssembly.Instance.prototype → Object.prototype
```

**Go 期望的原型链**：
```
标准实例 → WebAssembly.Instance.prototype → Object.prototype
```

## 失败的解决方案尝试

### 方法1：修改原型链
```javascript
Object.setPrototypeOf(instance, WebAssembly.Instance.prototype);
instance.constructor = WebAssembly.Instance;
```
**❌ 失败原因**：`instanceof` 检查仍然基于原始的构造函数

### 方法2：代理对象
```javascript
const proxy = new Proxy(instance, {
  get(target, prop) {
    if (prop === 'constructor') return WebAssembly.Instance;
    return target[prop];
  }
});
```
**❌ 失败原因**：
- 错误：`WebAssembly.Instance.exports(): Receiver is not a WebAssembly.Instance`
- `exports` 是 getter 属性，会检查调用者（receiver）的真实类型

## 成功的解决方案

### 方法3：创建真正的 WebAssembly.Instance

```javascript
// 在微信小程序环境中加载 WASM
const wasmResult = await WebAssembly.instantiate(url, go.importObject);

// 创建与 Go 运行时兼容的 WebAssembly.Instance 对象
const compatibleInstance = Object.create(WebAssembly.Instance.prototype);
compatibleInstance.exports = wasmResult.instance.exports;
Object.defineProperty(compatibleInstance, 'constructor', {
  value: WebAssembly.Instance,
  writable: false,
  enumerable: false,
  configurable: true
});

// 运行 Go WASM
await go.run(compatibleInstance);
```

### 为什么这样有效

1. **正确的原型链**：`compatibleInstance` 真正继承自 `WebAssembly.Instance.prototype`
2. **通过 instanceof 检查**：`compatibleInstance instanceof WebAssembly.Instance === true`
3. **绕过 exports getter 限制**：直接赋值而不是通过 getter 访问
4. **保持功能完整性**：所有 WASM 功能（内存、函数等）都正常工作

## 技术原理

这个问题的本质是**对象身份认证**问题：

```javascript
// 问题：身份不匹配
微信实例.constructor !== WebAssembly.Instance
微信实例.__proto__ !== WebAssembly.Instance.prototype  

// 解决方案：创建正确身份的对象
兼容实例.constructor === WebAssembly.Instance  ✅
兼容实例.__proto__ === WebAssembly.Instance.prototype  ✅
兼容实例.exports === 原始实例.exports  ✅（功能保持）
```

### 类比说明

这就像是**"身份证问题"**：
- 微信小程序给了你一个有效的护照（功能正常）
- 但 Go 运行时要求的是身份证（特定类型）
- 我们的解决方案是**用身份证的格式重新制作一个证件，但保留原护照的所有信息**

## 完整的实现代码

```javascript
// loader.js 中的关键部分
async function loadGoWASM() {
  // load wasm
  let url = config.assetURLs["gdspx.wasm"];
  const go = new Go();
  
  // 在微信小程序环境中加载 WASM
  const wasmResult = await WebAssembly.instantiate(url, go.importObject);
  
  // 创建与 Go 运行时兼容的 WebAssembly.Instance 对象
  const compatibleInstance = Object.create(WebAssembly.Instance.prototype);
  compatibleInstance.exports = wasmResult.instance.exports;
  Object.defineProperty(compatibleInstance, 'constructor', {
    value: WebAssembly.Instance,
    writable: false,
    enumerable: false,
    configurable: true
  });
  
  // 运行 Go WASM
  await go.run(compatibleInstance);
}
```

## 适用场景

这种模式在以下场景中很常见：
- **跨平台兼容性**：不同 JavaScript 运行时有自己的对象实现
- **运行时环境差异**：小程序、Node.js、浏览器等环境的 API 差异
- **第三方库集成**：当库对对象类型有严格要求时

## 注意事项

1. **性能影响**：创建兼容对象有轻微的性能开销，但在 WASM 初始化场景中可以忽略
2. **维护性**：这是一个 workaround，需要关注微信小程序和 Go 运行时的更新
3. **测试覆盖**：确保在微信小程序环境中充分测试 WASM 功能

## 总结

通过创建一个具有正确原型链和构造函数的兼容对象，我们成功解决了微信小程序中 Go WASM 的类型兼容性问题。这个解决方案既保持了原有的功能完整性，又满足了 Go 运行时的类型要求。
