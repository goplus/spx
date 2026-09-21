**godot_modules 重构建议：聚焦 Native 与普通 Web**

> 历史审查/方案文档，保留最初证据与决策过程。已实施结果以 [重构交付](godot_modules_refactor_result.md) 和 [三轮去冗余记录](godot_modules_three_rounds.md) 为准；后续已排除录制/Worker，并替换早期绑定注解设计。

2026-09-20。本次按用户最新要求缩小范围：不考虑录制子系统及 Web Worker 模式；保留音频播放、Native 和普通 Web。排除范围表示本轮不设计、不改造，不代表删除已有功能。

当前仓库 `8b3790203a`。原审查基线为 `3e9281aa52`；两者之间没有修改 `godot_modules/spx`、`internal/cmd/codegen`、`internal/gdengine` 或 `internal/enginewrap`，本文相关代码证据仍适用。完整缺陷证据及既有验证见 [审查文档](godot_modules_review.md)；本文替代此前更广范围的实施方案。

**1. 推荐的最终形态与优先级**

保留单游戏实例、固定 manager API 和 C++ 声明驱动的生成器。主要改动应集中在已有类内部的数据关系与清理规则。

| 优先级 | 重构主线 | 具体目标 |
| --- | --- | --- |
| P1 | 动画与视觉状态 | 共享不可变 clip，实例独立播放；视觉切换一次提交 |
| P1 | 资源发布与音频播放 | 加载失败不污染缓存；暂停不等于结束；播放对象统一回收 |
| P1/P2 | 生命周期与对象访问 | 固定关闭顺序，内部主线程执行，精简无所有权的 guard |
| P2 | ABI 所有权与生成器 | 内存释放独立于游戏；导出识别独立于实现继承 |
| P2 | 普通 Web 生命周期 | reset 隔离旧事件，借用内存到期即失效 |
| P3 | Sprite 算法与桥接优化 | 提取碰撞/batch 纯函数，测量后精简标量装箱 |

Native 的 Go 调度仍涉及线程，普通 Web 仍跨 JS/Wasm 边界。这两类现有契约继续维护；本轮不引入面向 Worker 的通信、代理或异步调度设计。

**2. 第一优先：动画资源与实例播放分离**

证据：`spx_res_mgr.cpp:371` 返回共享 `anim_frames`；`spx_sprite_animation.cpp:75` 将实例 loop 写进它。两个精灵播放相同 clip 时会互相影响。

最终关系应为：

```text
SpxResMgr
  clip key → 帧纹理引用、offset、FPS、来源信息
                         ↑ 共享不可变数据
SpxSprite
  当前 clip 的实例帧元数据 + frame/progress/speed/loop
```

先修复实例 loop 隔离：为当前 clip 创建独立 SpriteFrames 元数据，继续共享 Texture 引用；不复制整个动画库。再把 ResMgr 的全局动画库逐步收敛为按 clip 保存的信息，让 offsets、SVG 标记与帧源在同一处发布。

保留 AnimatedSprite2D。单图、atlas、正播、倒播与 SVG 倍率切换共同使用一个内部 `prepare_visual → commit_visual` 流程：先获得有效资源和实际 raster scale，再统一设置来源、frames、scale、offset、UV。准备失败保留完整旧状态。

这能同时修复位图/SVG 切换遗漏缩放、失败加载后先改状态，以及多个入口各自更新一部分字段的问题。只增加必要的小型数据结构，不新增动画 manager 或通用组件框架。

验收：同 clip 双实例分别循环/单次；位图与 SVG 双向切换；单图与动画切换；换倍率保留帧进度；加载失败不改变旧显示。

**3. 资源入口统一，保留局部事务**

`SpxResMgr` 继续作为资源所有者，内部拥有 clip、普通纹理、SVG 栅格缓存和字体配置；SVG 缓存逐步取消独立全局生命周期。缓存接受规范化路径和 clip 来源，不反向查询全局 resMgr。

动画创建改为“临时构造 → 完整校验 → 发布”。当前 `spx_res_mgr.cpp:402` 先发布 key，再加载帧；失败后同 key 又被当成已经存在，必须先修。路径只在资源入口规范化一次，load/reload/invalidate 使用一致的 key。更新已有纹理时明确已有引用的可见行为，失败保留旧资源。

保留字体的 decode/validate/prepare/commit。旧 default/font-face/preferences API 共享加载、验证和失效实现，但保持各次调用原有的生效时机与修改范围，不能悄悄改成最后一次调用才提交。

reload 的工作限定为修复现有路径和失效错误，不建设全局资源依赖图或完整热重载框架。资源变更验收重点是失败重试、既有引用可见性和字体两种消费者的一致性。

**4. 音频播放保留，一份 voice 表负责回收**

先修 `spx_audio.cpp:77` 的结束判定：普通 stream 暂停后 `is_playing()` 也可能为 false，不能立即 queue_free。相关语义已在原审查中用 Godot headless 探针确认。

随后把普通播放列表、循环列表和 aid 映射收敛为一个以 aid 为键的 voice 表，记录 loop、播放器 ObjectID 和必要状态。自然完成、stop、宿主 sprite 删除调用同一个幂等回收函数。

保留现有播放器与 bus pool 的 owned/borrowed 区分。轮询可以暂时保留，后续按需要用 finished/节点退出信号触发同一回收入口。不同销毁来源不能各维护一套列表清理逻辑。

验收：play → pause → 多帧 update → resume；stop 后重复 stop；自然完成；播放期间宿主删除；共享/专用 bus 释放正确。

**5. 引擎生命周期与对象访问做局部减法**

`SpxEngine` 继续负责固定 manager 的启动、更新和关闭，最终移除其对 `SpxBaseMgr` 的继承。它的头文件不在 manager 接口扫描范围内，可以独立完成，不要求一起重构全部 manager。

保持三个事实的区别：engine 对象存在、manager 已 awake、当前允许普通运行请求。暂停、单步、reset、冻结画面和 timer 也各有独立含义，只消除重复表达，不组合成庞大状态枚举。

必须保留现有 teardown：关闭普通入口 → destroy 回调 → manager 清理 → 删除 engine → destroyed 回调。manager 通知正序与实际 delete 逆序也先保持；顺序调整必须有单独依据。

`SpxObjectGuard` 改成普通 `require_sprite/require_ui` 等 checked lookup。它目前没有持有引用或延长对象生命周期，不需要移动语义、valid 副本和多层控制流宏。保留主线程检查、日志和默认返回值。

对象更新继续先收集 ID，再在调用前重新解析；删除先移除 ID，再触发回调。Godot Node 使用 ObjectID/销毁通知，C++ wrapper 由其 manager 独占，Ref 资源继续使用 Ref，不强行套一个统一销毁模板。

**6. 主线程责任收敛；跨线程控制保持现有契约**

普通 Native Go API 已由 `enginewrap` 和 binding 在进入引擎前进行主线程调度。第一步是把 `SpxEngine` 内部控制方法明确为 main-thread-only，再移除重复的 call_deferred 与延迟 pause 状态；保留外部同步封送。

现有 `Spx` 还公开支持非主线程提交控制请求。其来源不能仅凭排除 Web Worker 就判定为无用。第一轮保留该契约，以一个小型 pending 容器原子提交和取出请求、reset 参数及接收状态；工作线程不解引用可能关闭的 engine。

消费仍分阶段保持 restart → reset → pause → resume → next_frame。restart/reset 取出后跳过普通 update，未消费的低优先级请求继续保留；主线程请求仍立即执行。同类型尚未消费请求可以合并，reset 使用取出前最后提交的 code；取出后的新请求不能被旧 clear 擦掉。锁只保护请求，执行 Godot/业务回调前释放。

跨线程 `is_paused` 通过已有主线程查询或明确同步的状态快照；不要直接读取 engine 普通 bool。关闭时先停止接收、清空 pending，再执行两阶段 teardown。

这是现有入口的正确性收敛，不是本轮主要架构扩展；不增加通用命令总线，不把普通调用改成全异步或 FIFO。验收围绕 reset 参数交错、优先级、关闭拒绝新请求、回调重入和暂停单步。

**7. ABI 工具独立，生成器先解除继承耦合**

将字符串/数组分配、释放和校验移出 `SpxBaseMgr`，作为独立 ABI 工具，保留旧导出名称与必要静态转发。释放合法 ABI 返回值不应要求游戏或 ResMgr 仍存活。

高层 Native `FreeStr(string)` 有静态可见的重复释放路径：`manager_native.gen.go:1063` 分配 CString 并 defer free，又把该地址交给会释放它的 raw 调用。未发现仓库业务调用。保留真实 `ToString` 使用的 raw deallocator，停止生成这种高层字符串释放操作；需要保持高层签名时，可暂时生成弃用的 no-op。

生成入口明确区分启动注册、普通运行、关闭期间允许的清理和独立内存释放，各自定义阶段、线程及失败返回。不能统一加 ready 检查后直接返回，导致释放失败或输出值未定义。

普通 manager 的继承调整有前置条件：`internal/cmd/codegen/generate/gdext/header.go:37` 按指定父类识别类，`:110` 只扫描 `spx*mgr.h`。先让导出识别与实现继承解耦，验证导出集合和顺序不变，再逐个调整 manager；头文件目录最后整理。

节点由实际需要它的 manager 显式拥有，逐个验证节点路径、父子关系和绘制顺序后去掉默认空 Node2D。小型生命周期接口可以继续存在。默认 callback 表从已有定义生成，保留 no-op 默认函数、布局和顺序；不能直接置空，因为部分调用点没有判空。

**8. 普通 Web 修复事件边界，再收紧借用契约与优化桥接**

这些问题位于普通 Web 共用的 JS/Wasm 桥接代码，不依赖 Worker 模式。

第一，reset/destroy 必须隔离整个清理区间的旧 contact。当前队列可能在 reset 后派发旧事件；仅在入口清空一次还不足以覆盖 manager 清理产生的新事件。明确完成边界或停止期间拒绝旧周期入队，batch 与 fallback 派发均需覆盖。

第二，临时数组 borrow 到期后应拒绝旧 descriptor。现有检查只看 Module 和 heap 范围，不能证明地址仍归当前借用所有；原审查复现了过期 descriptor 仍被接受，但未发现正常 Go 调用链跨期限使用它。因此归为后续契约加固，不与已确认的游戏功能错误同级。使用局部 arena generation 或同步调用范围封装即可，保留可信 metadata，不扩展成全局资源版本框架。

第三，标量按值传递作为 P3 独立优化。先从 bool/float 移除包装分配，64-bit ID 保留精度安全的表示；结构输出和变长数据继续使用必要的内存协议。修改生成器而非手工改产物，测量 Wasm 调用次数、分配次数和真实帧耗时后再删对象池。

所有变化保留 `SPX_OUT` 失败不修改输出、字符串恰好释放一次、heap growth 后访问正确，以及 Native/普通 Web 行为一致。

**9. Sprite/Physics 聚焦语义与可测试算法**

`AddImpulse` 单独作为行为修正：推荐 `Δv = J / mass`，保留质量影响，去掉当前多余 delta；同步文档并测试 30/60/120 Hz。该选择会改变旧效果，必须独立提交并通过新 runtime 发布，不能混入纯结构整理。

保留 SpxSpriteMgr 的 API 门面。将像素碰撞、查询结果转换、batch 解码和校验逐步提取为普通函数或内部结构。机械提取时保持遍历顺序、坐标、pivot、flip、负缩放、透明度和像素中心规则；算法优化和永久缓存另行测量与评审。

本轮不增加新的 Physics/Sprite 子 manager，不把 Sprite 强塞进会 memdelete wrapper 的通用对象模板，也不以文件行数为拆分目标。

**10. 推荐实施顺序**

| 阶段 | 具体交付 | 验收重点 |
| --- | --- | --- |
| A：修行为 | loop 隔离、pause 修复、SVG 切换、动画失败重试、Web reset；Native 非主线程兼容入口的 reset 参数与未被业务调用的 FreeStr 包装可独立修正 | 针对真实跨状态行为的回归；不夹带目录搬迁 |
| B：资源与播放收敛 | 按 clip 数据、visual 准备/提交、一个 voice 表、SVG cache 归 ResMgr | 共享资源不被实例写入；失败保留旧状态；清理一次 |
| C：基础设施减法 | checked lookup、ABI memory、默认 callback 生成、Engine 脱离基类及内部主线程化；borrow 期限加固 | ABI/回调布局和 teardown 顺序保持 |
| D：继承与算法整理 | scanner 解耦后调整 manager；按需节点；提取碰撞/batch 函数 | 导出集合不变，算法结果和对象顺序不变 |
| E：按测量优化 | 普通 Web 标量按值、局部热点优化 | 有 Native/普通 Web 基准；ABI 影响单独评审 |

冲量修正是独立行为提交，不作为所有结构工作的前置条件。A 中各问题与 B/C 的独立子项可以并行；有风险的所有权调整以相关回归建立为前提。

合并前用锁定 Godot 源码重新构建当前 SPX 测试，运行 Native 与普通 Web 的必要集成回归，并验证生成器完整导出清单、顺序和重复生成无额外差异。本轮不设置录制或 Worker 模式验收项。

本次仅更新建议文档和核对代码范围，没有修改实现或重新运行测试。原审查中的 Web 24 项、独立 C++ ABI/UBSan 和探针结果保留为此前验证记录，不等同于当前 HEAD 的完整验证。
