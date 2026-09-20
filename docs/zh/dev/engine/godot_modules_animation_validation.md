# 05-Animation 实际运行验证

2026-09-20 在 macOS arm64 上验证完整重构代码，覆盖 Native 与普通 Web，不涉及录制或 Worker。最终整合入口为 `refactor/godot-integrated`；本次构建在同源的 `refactor/godot-simplify-three-rounds` 工作区完成。

## 构建

```sh
make generate
SPX_EMBED_RUNTIME=0 make dev MODE=normal \
  GODOT_SRC=/Users/joeykchen/godot \
  SPX_MODULE_SRC="$PWD/godot_modules/spx"
```

- Godot 固定为 `09227b82bffef39163abbbe03fc696095d2cb207`，使用模块内 SCons profile。
- 编辑器与 macOS Native template 完整编译、链接成功；Native PCK 导出成功。
- 普通 Web 使用 Emscripten 3.1.62、`threads=no`，构建及资源导出成功。
- CLI、Native Go 运行库和 `ispx.wasm` 均由当前分支安装；构建流程为 Go 部分固定 Go 1.25.8。
- Web 实际 Wasm 导出包含新按值入口，已删除 `gdspx_new_int` / `gdspx_new_obj` 输入装箱导出。
- 完整生成更新了 `export.types` 的源码位置编码，类型与签名不变。

未发布 runtime，未修改下载锁文件。新 Go 绑定与本地新 runtime 配套使用。

## Native

在本分支的 `tutorial/05-Animation` 目录执行 `spx runnative`，成功打开游戏窗口。

持续观察到敌机生成、移动和子弹发射；鼠标移到舞台左右不同位置后，子弹发射列随之改变。保存了三张游戏窗口截图；退出应用后命令返回 0，未出现运行时异常。

本机链接器仍有重复 rpath / LLVM 库提示，以及构建 x86_64 分片时忽略 arm64 LLVM 库的提示；它们未阻止链接或 arm64 示例运行。

## 普通 Web

同目录执行 `spx runweb -serveraddr=:8106`，在隔离的可见 Chrome 中打开页面，点击 Normal 模式的 Start。

Godot 与 Go Wasm 完成初始化，游戏进入 `Running: normal`。游戏画布连续采样的两组各 6 次帧间比较均有变化；没有 JavaScript 异常、运行时崩溃或游戏资源加载失败。浏览器仅报告缺少 `favicon.ico` 的 404。

默认实验页的 iframe 为 480×360，而本示例舞台为 202×360。已有窗口内容尺寸与相机限制组合使舞台贴到右侧；这不是本轮标量传参或生命周期重构引入的变化。该历史问题在独立的 `fix/godot-web-stage-letterbox` 分支处理，重构整合分支不包含它。

这次验证是单个示例在本机的实际运行检查，不替代完整游戏集、跨浏览器或跨平台回归。
