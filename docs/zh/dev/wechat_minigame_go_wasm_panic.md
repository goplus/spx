# Go WASM 微信小游戏字符串解码排查

## 适用范围

本文记录微信小游戏真机上可能出现的 `syscall/js` 字符串解码问题。它是排查指南，不代表所有微信基础库或 Go 版本都会复现该问题。

## 先确认实际文件

仓库中没有 `js/wasm_exec.js`。导出流程会从当前 Go 环境读取：

```text
$(go env GOROOT)/lib/wasm/wasm_exec.js
```

并复制为：

```text
<project>/project/.builds/web/go.wasm.exec.js
```

小游戏导出随后会把它合并到生成目录的 `js/engine.js`。因此不要按照固定行号修改仓库文件；必须先保存 Go 版本、SPX 版本和实际生成文件，再针对该版本验证补丁。

## 典型症状

```text
panic: syscall/js: call of Value.Get on undefined
```

如果属性名被错误解码为空字符串，`js.Global().Get("fs")` 等调用可能访问到错误对象。应先用最小项目在开发者工具和真机分别复现，并确认问题来自字符串解码，而不是适配器没有初始化 `fs`、`process` 或 `console`。

## `loadString` 修改原则

Go 的字符串由指针和显式长度组成，不以 NUL 作为结束标记。任何针对生成 `go.wasm.exec.js` 的补丁都必须：

1. 同时检查 `saddr >= 0`、`len >= 0` 和 `saddr + len <= buffer.byteLength`，避免越界构造 `TypedArray`。
2. 严格使用传入长度，保留合法的嵌入式 NUL 字节。
3. 使用经过目标运行时验证的 UTF-8 解码器；不能用逐字节 ASCII 拼接代替 UTF-8，也不能把错误静默转换成空字符串或 `?`。
4. 解码失败时抛出带地址和长度的错误，保留原始异常，便于定位内存或 ABI 问题。

安全的边界检查结构可以写成：

```javascript
const loadString = (addr) => {
  const saddr = Number(getInt64(addr + 0));
  const len = Number(getInt64(addr + 8));
  const buffer = this._inst.exports.mem.buffer;

  if (!Number.isSafeInteger(saddr) || !Number.isSafeInteger(len) ||
      saddr < 0 || len < 0 || saddr > buffer.byteLength - len) {
    throw new RangeError(`invalid Go string range: ${saddr}+${len}`);
  }
  return decoder.decode(new Uint8Array(buffer, saddr, len));
};
```

这段代码只解决内存范围和长度语义，不能保证微信运行时的 `TextDecoder` 实现没有缺陷。如果真机仍然复现，应针对当前微信基础库提供并测试 UTF-8 解码器，再将补丁固化到导出流程，而不是手工修改某个临时目录中的文件。

## 验证清单

- `js.Global().Get("fs")`、`process`、`console` 等属性名能正确解码。
- 包含中文、四字节 Unicode 和嵌入式 NUL 的字符串不会被截断。
- 越界地址会报告错误，而不是返回空字符串继续运行。
- Go 版本、微信基础库和 `go.wasm.exec.js` 变更后重新执行真机测试。
