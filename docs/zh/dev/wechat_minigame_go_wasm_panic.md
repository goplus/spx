# Go WASM 微信小程序字符串解码修复

## 问题描述

在微信小程序真机环境中运行 Go WASM 时出现以下错误：

```
panic: syscall/js: call of Value.Get on undefined

goroutine 1 [running]:
syscall/js.Value.Get({{}, 0x0, 0x0}, {0x2707f, 0x9})
    /usr/local/go/src/syscall/js/js.go:296 +0xc
syscall.init()
    /usr/local/go/src/syscall/fs_js.go:20 +0xd
```

**环境差异：**
- 模拟器：正常运行
- 真机：运行时错误

## 根本原因

微信真机环境使用 QuickJS 引擎，其 `TextDecoder` 无法正确解码 WASM 内存中的字符串数据，导致：
- Go 代码：`js.Global().Get("fs")`
- 实际传递：属性名被解码为空字符串 `""`
- 结果：`globalObject[""]` 返回 `undefined`

## 解决方案

修改 `js/wasm_exec.js` 中的 `loadString` 函数，使用手动字符串解码替代 TextDecoder：

### 原版代码（有问题）：
```javascript
const loadString = (addr) => {
    const saddr = getInt64(addr + 0);
    const len = getInt64(addr + 8);
    return decoder.decode(new DataView(this._inst.exports.mem.buffer, saddr, len));
}
```

### 修复版代码：
```javascript
const loadString = (addr) => {
    const saddr = getInt64(addr + 0);
    const len = getInt64(addr + 8);
    
    if (len === 0) return "";
    if (saddr < 0 || saddr >= this._inst.exports.mem.buffer.byteLength) return "";
    
    try {
        // 🔧 QuickJS 环境修复：优先使用手动解码
        const bytes = new Uint8Array(this._inst.exports.mem.buffer, saddr, len);
        
        // 快速 ASCII 检查和解码
        let result = "";
        for (let i = 0; i < bytes.length; i++) {
            const byte = bytes[i];
            if (byte === 0) break; // null 终止符
            if (byte < 128) {
                result += String.fromCharCode(byte);
            } else {
                // 遇到非ASCII字符时回退到TextDecoder
                try {
                    const remaining = bytes.slice(i);
                    result += decoder.decode(remaining);
                    break;
                } catch (e2) {
                    result += "?";
                }
            }
        }
        return result;
    } catch (e) {
        return "";
    }
}
```

## 实施方法

1. 打开 `js/wasm_exec.js` 文件
2. 找到 `loadString` 函数（约160行左右）
3. 将原版代码替换为修复版代码
4. 测试 Go WASM 在真机环境中是否正常运行

## 验证结果

修复后，Go 代码能够正确访问：
- ✅ `js.Global().Get("fs")` 
- ✅ `js.Global().Get("process")`
- ✅ `js.Global().Get("console")`
- ✅ 不再出现 panic 错误

---

**核心原理：** 使用 `String.fromCharCode()` 直接处理 ASCII 字符，避免 QuickJS 环境中 TextDecoder 的兼容性问题。 