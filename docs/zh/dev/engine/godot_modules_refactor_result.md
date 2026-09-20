**godot_modules 本轮重构交付**

基线：`8b3790203a`。范围为 Native 与普通 Web，保留音频播放；本轮不修改录制或 Worker 实现。

集成分支：`refactor/godot-integrated`，工作目录为 `/Users/joeykchen/spx-godot-refactor-worktrees/integration`。原 `dev` 分支保留原状。

| 分支 | 交付 |
| --- | --- |
| `refactor/godot-visual-resources` | clip 完整构建后发布；每实例独立播放元数据、共享纹理；统一视觉准备/提交；修正 SVG 缩放、reload 像素缓存和克隆内部节点 |
| `refactor/godot-audio-ownership` | 一份 voice 表管理播放器；ObjectID 解析；暂停保留播放；统一幂等回收 |
| `refactor/godot-lifecycle-controls` | Engine 脱离 BaseMgr；集中跨线程请求、reset 参数和接收状态；删除重复 deferred pause 状态 |
| `refactor/godot-object-access` | 将无所有权的 guard 类简化为 checked lookup；保留线程检查、日志和失败默认值 |
| `refactor/godot-abi-boundaries` | 独立 ABI 内存工具；生成默认回调；导出识别不依赖继承。绑定注解方案由下方简化分支替代 |
| `refactor/godot-bindings-simple` | 无参数 SPX_BIND；由 static 推导调用目标；输入和 callback 字符串借用、返回值转交；删除高层 FreeStr、绑定策略及兼容转发 |
| `fix/godot-web-session-lifetime` | reset/destroy 隔离旧 contacts；批次回调中切换 session 后停止剩余事件；过期数组借用拒绝继续使用 |

对象访问分支基于音频分支，绑定简化分支基于前一轮集成结果；其他主线从同一基线分出。集成分支保留各分支提交历史，可分别审查。

遵循用户后续约束：移除 Spx / SpxEngine 的测试专用 friend 与私有访问器，删除新增的音频私有注入测试。保留的检查通过公开 API、独立 ABI 工具、生成器或 JS/Go callback 入口执行，不为测试新增生产接口。

保持的契约包括：callback ABI 布局；destroy → manager 清理 → 删除 engine → destroyed 的顺序；控制请求优先级；主线程控制立即执行；音频 bus 的 owned/borrowed 区分；AnimatedSprite2D 及纹理共享。

**最终 binding 约定**

```cpp
SPX_BIND GdVec2 get_global_mouse_pos();
SPX_BIND static void pause();
SPX_BIND static void request_reset(GdInt exit_code);
```

`SPX_BIND` 是唯一导出标记，在 C++ 中展开为空，不接受参数。声明与标记写在同一行。普通方法通过 manager 调用；`static` 方法直接按类调用，生命周期方法的实现再进入 `Spx`。生成器无需维护控制方法名称白名单或第二套行为注解。数组的方向继续使用参数标记 `SPX_OUT`。两个标记定义在 `spx_abi.h`，无需从 BaseMgr 继承才能使用。

删除 `SPX_API`、`SPX_BINDING(...)` 及 `web=noop`、`web=reuse_result`、`abi=free_string`、`control=...` 的解析、校验和模板分支。Web 结构化返回值默认独立创建，仍可显式提供输出容器；常规鼠标位置读取继续使用已有帧快照缓存。

删除 Go/JS 高层 `FreeStr(string)` 和旧 `spx_res_free_str` 导出。Native 初始化时解析新的全局 `spx_global_free_string`；返回值转换复制字符串后调用它，始终由 C++ 释放 C++ 地址。它直接使用 `SpxAbi`，不查找 engine 或 manager。分配、释放、数组类型检查集中在 `spx_abi.h/.cpp`，manager 调用点直接使用该工具。

默认 callback 表仍由已有 `SpxCallbackInfo` 字段和 callback typedef 生成到 `spx_callback_defaults.gen.h`，无需额外 schema。

字符串所有权按调用方向约定，不引入逐方法注解：

| 位置 | 约定 |
| --- | --- |
| 方法输入 | 调用期间借用；调用方负责原始存储 |
| callback 输入 | 同步借用；接收端在返回前复制需要保留的内容 |
| 方法返回值 | 转交所有权；Native 复制后通过全局 ABI 释放，Web 使用返回包装的释放流程 |

场景实例化 callback 使用局部 `CharString`，runtime panic 直接借用输入字符串。Native callback 仅复制字符串；默认空 callback 和 Web callback 无需处理转交分配。这同时消除了旧发送端分配、默认或 Web callback 未释放的漏洞。

这是按用户要求实施的不兼容变更。新的 Go 绑定必须与重新构建的 runtime 配套使用；本轮未发布 runtime，也未更新下载锁文件。独立复核确认：新释放函数在注册 callback 前解析；复制与释放次序正确；static 分派仍共用字符串、数组和输出状态转换；除替换释放入口外，379 个保留的 ABI typedef 签名一致。

**验证记录**

- 固定 Godot 源码 `09227b82bffef39163abbbe03fc696095d2cb207`，使用模块内 `spx_scons_profile.json`。
- 普通 Web：Emscripten 3.1.62，`threads=no` 的 template_release 完整编译和链接通过。
- `make generate-bindings` 后 tracked 文件零差异；新全局释放 ABI 与高层接口删除同步生成。
- codegen 的 Go 测试、Native binding/native 和 enginewrap 测试、impl 包编译、js/wasm binding/web 测试通过。
- Node 普通 Web 测试 30/30 通过，包含 batch 与 fallback 在 teardown 时丢弃旧事件、借用失效等场景。
- 独立 ABI 内存与 Web 字符串 ABI 两套检查，普通执行及 UBSan 均通过。
- Native Godot：固定 Apple Clang 编译、链接通过；`*SPX*,*Loop phase callbacks*` 集成回归 63 个用例、523 个断言全部通过。

本机最初的 Native 增量构建混用了 Apple Clang 21 与 LLVM 19；两者对 Godot SpinLock 的布局计算不同，测试场景初始化因此崩溃。最终验证采用固定编译器并使用独立构建后缀，避免复用不兼容对象；无需改动 Godot 源码。

本轮没有进行完整浏览器游戏场景或平台矩阵验收。ASan 本机运行库初始化卡住，未计为通过；上述内存检查使用 UBSan。

**后续独立工作**

本轮完成已确认行为问题与相关结构收敛，未实施完整路线图中的后续探索：SVG 缓存全局生命周期迁入 ResMgr、字体配置进一步收敛、逐个 manager 去掉空 owner 节点、碰撞算法拆分，以及测量后的 Web 标量按值优化。

AddImpulse 的帧率语义调整会改变既有作品效果，保持为单独行为变更，本轮未修改。当前摄像机单独缩放时刷新 SVG 清晰度的机制也保持原行为。
