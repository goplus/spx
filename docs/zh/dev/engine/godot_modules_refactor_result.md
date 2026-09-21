**godot_modules 本轮重构交付**

基线：`8b3790203a`。范围为 Native 与普通 Web，保留音频播放；本轮不修改录制或 Worker 实现。

集成分支：`refactor/godot-integrated`，工作目录为 `/Users/joeykchen/spx-godot-refactor-worktrees/integration`。原 `dev` 分支保留原状。

本文记录前两阶段的实现与验证。后续三轮去冗余也已同步到整合分支，见 [分支关系与三轮结果](godot_modules_three_rounds.md)；实际 `05-Animation` Native / Web 运行情况见 [示例验证记录](godot_modules_animation_validation.md)。

| 分支 | 交付 |
| --- | --- |
| `refactor/godot-visual-resources` | clip 完整构建后发布；每实例独立播放元数据、共享纹理；统一视觉准备/提交；修正 SVG 缩放、reload 像素缓存和克隆内部节点 |
| `refactor/godot-audio-ownership` | 一份 voice 表管理播放器；ObjectID 解析；暂停保留播放；统一幂等回收 |
| `refactor/godot-lifecycle-controls` | Engine 脱离 BaseMgr；集中跨线程请求、reset 参数和接收状态；删除重复 deferred pause 状态 |
| `refactor/godot-object-access` | 将无所有权的 guard 类简化为 checked lookup；保留线程检查、日志和失败默认值 |
| `refactor/godot-abi-boundaries` | 独立 ABI 内存工具；生成默认回调；导出识别不依赖继承。绑定注解方案由下方简化分支替代 |
| `refactor/godot-bindings-simple` | 无参数 SPX_BIND；由 static 推导调用目标；输入和 callback 字符串借用、返回值转交；删除高层 FreeStr、绑定策略及兼容转发 |
| `fix/godot-web-session-lifetime` | reset/destroy 隔离旧 contacts；批次回调中切换 session 后停止剩余事件；过期数组借用拒绝继续使用 |

后续四个分支基于上一轮集成提交 `f263460539`，现已全部合入：

| 分支 | 本轮补齐 |
| --- | --- |
| `refactor/godot-resource-lifetimes` | SVG 缓存由 ResMgr 持有；字体加载和校验共用准备函数；增量配置变更同步失效缓存；退出恢复宿主主题字体 |
| `refactor/godot-manager-lifecycle` | `SpxManager` 只提供生命周期与上下文；删除默认空 owner 节点；UI、Sprite、Camera、Input、Tilemap 显式清理实际节点；Ext 改为静态入口 |
| `refactor/godot-sprite-algorithms` | 像素采样与合成独立；批量更新统一完整校验；射线查询统一参数与结果转换；冲量帧率修正单独提交 |
| `perf/godot-web-scalars` | 生成器统一判定 bool/float 按值输入；删除对应输入装箱；补充真实 Wasm 微基准 |

集成过程补上生成器的嵌套作用域识别：manager 内部结构体、内联函数及注释中的括号不会截断后续导出。所有分支保留各自提交历史。

遵循用户后续约束：移除 Spx / SpxEngine 的测试专用 friend 与私有访问器，删除新增的音频私有注入测试。保留的检查通过公开 API、独立 ABI 工具、生成器或 JS/Go callback 入口执行，不为测试新增生产接口。

后续复扫也清理了原有的测试侵入：

- 删除 SPX 的七个测试专用查询接口：主循环注册状态、callback 存在状态、字体发布代数、overlay 颜色/可见性请求/目标、对象数量。字体内部代数仍用于实际缓存失效。保留的测试观察回调执行次数、字体渲染结果、对象查找和克隆节点的实际变化。
- `fs/zip` 删除 HTTP client 注入、可变大小限制和原子测试限额覆盖，生产直接使用既有限额；测试使用真实 ZIP 条目、声明大小和本地 HTTP 响应。
- `runtimebundle` 删除仅返回固定错误的 `UnsupportedCrossProcessLockProvider`。有实际锁实现的进程内缓存策略保留，不将正常业务选项当作测试钩子删除。
- `buildctl` 删除下载、构建、命令分派和工具链准备中的可变函数别名与测试参数；锁等待、下载超时、NDK 大小限制保持原值。macOS 环境仍在全部准备成功后才提交修改。
- `cmd/spx` 的运行入口直接准备嵌入资源；删除嵌入文件系统和缓存目录函数的测试替换，缓存回归使用已有嵌入文件及临时目录。
- 删除依赖替换生产函数的编排测试及无用 fixture；保留参数解析、本地 HTTP/ZIP、文件发布与子进程行为检查。未增加新的 friend、私有访问器或替代注入入口。

录制模块中的测试下载入口仍在此前排除范围内；`SpxEngine` 与音频捕获类的运行期 friend 不属于测试访问器。

保持的契约包括：callback ABI 布局；destroy → manager 清理 → 删除 engine → destroyed 的顺序；控制请求优先级；主线程控制立即执行；音频 bus 的 owned/borrowed 区分；AnimatedSprite2D 及纹理共享。

**最终 binding 约定**

```cpp
SPX_BIND GdVec2 get_global_mouse_pos();
SPX_BIND static void pause();
SPX_BIND static void request_reset(GdInt exit_code);
```

`SPX_BIND` 是唯一导出标记，在 C++ 中展开为空，不接受参数。声明与标记写在同一行。普通方法通过 manager 调用；`static` 方法直接按类调用，生命周期方法的实现再进入 `Spx`。生成器无需维护控制方法名称白名单或第二套行为注解。数组的方向继续使用参数标记 `SPX_OUT`。两个标记定义在 `spx_abi.h`，无需从 manager 基类继承才能使用。

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

这是按用户要求实施的不兼容变更。新的 Go 绑定必须与重新构建的 runtime 配套使用；本轮未发布 runtime，也未更新下载锁文件。独立复核确认：新释放函数在注册 callback 前解析；复制与释放次序正确；static 分派仍共用字符串、数组和输出状态转换；相对 `f263460539` 核对全部 380 个 ABI typedef，仅三项 `Persistant` → `Persistent` 改名，参数和返回协议没有遗漏或其他变化。普通 Web 的 bool/float 输入签名单独按值生成。

**本轮的简化与行为调整**

- SVG 缓存以路径/动画 key 和整数倍率分层索引，接受已解析的路径与 clip 来源，不再反向依赖全局 manager；包括 1× 动画在内的所有 SVG 帧均经过当前缓存，避免失效后复用旧像素。
- 字体的完整配置仍遵循 decode / validate / prepare / commit；旧分步 API 保留每次调用立即生效的范围，共用字体准备和 preference 校验。非法 preference、字体或保留名称不会修改已发布状态；清空 preferences 使用带类型的空数组。
- 删除 `SPXCLASS`、空生命周期转发及无用途的虚析构声明。`SpxObjectMgr` 仅负责拥有的 wrapper 与实际场景根节点，Sprite 保留自己的节点生命周期。
- `spx_object_guard.h` 改为 `spx_object_access.h`，宏名使用 `LOOKUP`；`get_anim_frames` 改为 `get_animation_frames`，`ToArray` 改为内部 `to_array`，`persistant` 统一改为 `persistent`，同步生成 C++ / Go / JS 接口。注释集中说明借用期、原子发布、采样顺序和坐标等约束。
- `SpxPixelQuery` 提供每次查询的资源快照、坐标采样和颜色合成；manager 保留对象筛选与事件职责。批量更新共用长度校验，继续先检查完整包、再修改节点，并保留 destroy-wins 顺序。
- `AddImpulse` 使用 `Δv = J / mass`，去掉多余的时间步乘数；同一 tick 的冲量累计且只消费一次。独立提交为 `d9fd022e02`，同步更新中英文文档和 Go 注释；30 / 60 / 120 Hz 回归覆盖该语义。

**Web 标量测量**

使用实际生成的 JS / C++ 包装和 Emscripten 3.1.62，替换为相同的轻量 C++ 接收函数，隔离桥接开销。Apple M4 / Node 24.20.0 上，7 轮交替测量，每轮 20 万次调用：float 2.43 → 16.23 百万次/秒，bool 2.38 → 15.38，ID + float 1.44 → 3.00，ID + 4 float 0.56 → 2.98。

每个优化后的标量少一次 pool 获取/归还与两次 JS → Wasm 调用；预热后的真实 malloc 次数前后均为 0。模拟 100 个精灵更新旋转和可见性时，跨边界调用由 1000 降到 600，桥接耗时由 140.8 µs 降到 68.6 µs。这里只测桥接成本，不将其当作真实游戏 FPS 或完整帧耗时。复现方法和原始数据见 [基准说明](../../../../godot_modules/spx/web/benchmarks/README.md)。

**验证记录**

- 固定 Godot 源码 `09227b82bffef39163abbbe03fc696095d2cb207`，使用模块内 `spx_scons_profile.json`。
- Native：固定 Apple Clang 编译、链接通过；清理测试接口后，`*SPX*,*Loop phase callbacks*` 集成回归 71 个用例、621 个断言全部通过。
- 普通 Web：Emscripten 3.1.62，`threads=no` 的 template_release 完整编译和链接。
- `make generate-bindings` 重复执行 tracked 文件零差异；380 个 ABI typedef 逐项核对。
- codegen 全部 Go 测试、Native binding/native 和 enginewrap 测试、impl 包编译、js/wasm binding/web 测试通过。
- Node 普通 Web 测试 31/31 通过，包含标量按值、64 位 ID、异常清理、reset/destroy 事件隔离与数组借用失效。
- 独立 ABI 内存与 Web 字符串 ABI 两套检查，普通执行及 UBSan 均通过。
- 在集成代码上重新运行真实 Wasm 标量微基准，完成数值、完整 ID、heap growth 校验和前后对照。
- 清理后重新通过 Native / 普通 Web 编译及链接、绑定生成零差异检查；`fs/zip`、`internal/runtimebundle`、`internal/cmd/buildctl/...`、`cmd/spx/internal/command` 和 `cmd/spx/internal/runtimeasset` 的 Go 测试通过。

上一轮列出的未实施代码项已补齐。尚未进行完整浏览器游戏场景或跨平台矩阵验收；不将微基准替代这些验收。ASan 本机运行库初始化问题未计为通过。新的 Go 绑定需与本分支编译的 runtime 配套，本轮未发布 runtime 或更新下载锁文件。
