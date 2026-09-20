**godot_modules 深度审查与简化重构建议**

> 历史审查/方案文档，保留最初证据与决策过程。已实施结果以 [重构交付](godot_modules_refactor_result.md) 和 [三轮去冗余记录](godot_modules_three_rounds.md) 为准；后续已排除录制/Worker，并替换早期绑定注解设计。

后续综合分享对话的实施决策见 [最终重构建议](godot_modules_refactor_plan.md)。本文件保留缺陷证据与原始审查；控制请求消费策略、兼容选择和实施依赖以最终建议为准。

审查日期：2026-09-20。SPX 基线：`3e9281aa52`；对照的本地 Godot 源码为 runtime lock 指定的 `09227b82bffef39163abbbe03fc696095d2cb207`。范围包括模块初始化、manager 生命周期、对象管理、sprite/physics/resources、Web/native ABI、音频与 recorder；同时追踪相关 Go 调用方和代码生成器。第三方 LunaSVG 没有逐行全面审计。

本次只新增审查文档，没有修改实现。下文“源码确认”表示调用链和状态变化可以从代码推导，不等同于已跑完端到端复现；单独注明实际运行过的检查。

**1. 总体判断**

最值得简化的是“同一事实由多处维护”：动画资源携带实例播放状态，渲染模式由多组布尔值与字符串共同表达，manager 同时承担 ABI、节点和生命周期职责，录制系统存在多个时钟和缓冲所有者。这些复杂度已经对应到实际行为错误。

建议保留 C++ manager 的对外 API 边界和现有代码生成体系，逐步把内部改成少量有明确所有者的状态与资源。优先修复行为，再合并状态，最后删除兼容分支。继续按方法名拆分 `.cpp` 或增加通用 Manager/Service 层，收益较小。

目前已有值得沿用的实现：字体的 validate/prepare/commit 事务、集中坐标转换、批处理输入先校验再写入、UI binding 的 ObjectID 校验、对象遍历时先收集 ID 再重新解析，以及模块与 Godot checkout 的构建隔离。不要在“精简”中删除这些边界。

**2. 优先处理的行为问题**

P1 表示可能明显破坏正常功能，应优先修复；P2 表示特定转换、重置或配置下的错误。排序同时考虑影响与证据强度，不是安全漏洞分级。

| 优先级 | 问题 | 最小触发条件 | 确认方式 |
| --- | --- | --- | --- |
| P1 | 动画 loop 写入共享资源 | 两个精灵播放同一 clip，loop 参数不同 | 源码、Go 调用链、Godot 基类 headless 复现 |
| P1 | 暂停的非循环音频被当成结束回收 | 暂停一个正在播放的普通 stream，继续引擎更新 | 源码与 Godot 基类 headless 语义验证 |
| P1 | AddImpulse 多乘一次 delta | 改变 physics tick，保持冲量和质量一致 | 源码、计算与仓库文档契约 |
| P1 | 默认 AVI 合并器写错 chunk 长度 | 桌面录制完成后的默认合并 | 可达性与实际写入字节计数 |
| P2 | 录制帧率与采集时钟分离 | 配置视频 FPS 不为 30，或音频 underrun | 源码 |
| P2 | SVG/位图切换遗漏缩放提交 | 非 1× 渲染比例下切换来源模式 | C++ 与 Go 调用顺序 |
| P2 | 动画失败后留下部分缓存 | SVG/帧构建失败后使用同 key 重试 | 源码 |
| P2 | Web reset 后仍派发旧碰撞事件 | 队列非空时 reset，之后 update | Node VM 实际复现 |

**2.1 动画资源必须与实例播放状态分离**

证据：`godot_modules/spx/spx_res_mgr.cpp:371` 的 `get_anim_frames(anim_name)` 忽略参数，返回整个共享 `anim_frames`；`spx_sprite_animation.cpp:74` 将它交给实例，紧接着第 75 行调用 `set_animation_loop(..., p_is_loop)`。SVG 的缓存帧同样被共享，`spx_sprite_render.cpp:147` 在切换栅格倍率时再次修改共享 loop。

因此 A 以单次模式播放，B 以循环模式播放相同 clip，B 会改变 A 所使用资源的 loop。Go 的 `component_animation.go:216` 传入实例参数，后续等待逻辑通过 `IsPlayingAnim` 判断完成，可能表现为等待不结束，或另一个实例提前停止。

补充 headless 验证：现有 Godot 构建中，两个 AnimatedSprite2D 共享两帧、10 FPS 的 clip；A 单次播放后 B 将其改为循环。0.35 秒后 A 仍在播放，完成事件计数为 0。这验证底层共享资源语义，未重新构建最新 SPX 模块跑完整作品。

最小修复是让实例持有该 clip 的独立 `SpriteFrames` 元数据，继续共享纹理引用。不要直接为每个 sprite 深复制整个全局动画库。后续把资源缓存改成按 clip 保存不可变帧、offset、FPS、来源信息；loop、frame、progress、speed 留在实例侧。使用 Godot `AnimatedSprite2D` 时，实例侧元数据承接其资源级 loop 限制即可，无需重写动画播放器。

验收：同 clip 的两个实例分别循环/单次播放，互不改变完成行为；SVG 倍率切换保持当前 frame/progress 和本实例 loop。

**2.2 音频暂停与结束不能都用 `!is_playing()` 表示**

证据：`spx_audio.cpp:148` 的 pause 只设置 `stream_paused`；第 77 行开始的非循环清理在 `!audio->is_playing()` 时删除播放器和 aid。循环分支第 98 行却额外检查了 `!get_stream_paused()`。

锁定的 Godot `scene/audio/audio_stream_player_internal.cpp:385` 查询 AudioServer 活跃状态；`servers/audio_server.cpp:1430` 对普通 stream 只有 PLAYING 返回 true。因此播放已开始、暂停状态生效后，非循环播放器会被后续 SPX 更新回收，resume 找不到原 aid。这里限定普通 stream 路径，未将 Web sample 播放的实现差异一概而论。

补充 headless 验证：AudioStreamPlayer2D 播放 WAV 时 `is_playing=true`；暂停后立即及 0.15 秒后均为 false，同时 `stream_paused=true`；恢复后重新为 true。SPX 的清理行为由上述调用链确认。

先修复暂停状态判定；随后将 `audios`、`loop_audios`、`aid_audios` 收敛为一个以 aid 为键的 voice 表，记录 loop 与播放器身份。完成回收尽量由 `finished`/节点退出驱动；如果保留轮询，也只遍历这一份记录。挂在 sprite 子树上的播放器需要 ObjectID 或销毁通知，不能仅凭表中仍有裸指针判断存活。

验收：播放 → 暂停 → 多次 update → 恢复；自然完成、显式 stop、宿主 sprite 删除都只回收一次。

**2.3 冲量、持续力与加速度的单位要分开**

证据：`spx_sprite_physics.cpp:104` 把 impulse 加入 `applied_forces`；第 142 行计算 `velocity += applied_forces * delta / mass`，第 154 行清零。`component_physics.go:149` 直接转发。仓库 `docs/zh/dev/engine/physic_api.md:265` 附近将它描述为瞬时速度变化。

忽略重力、阻力和摩擦，质量为 1、冲量为 300 时，60 Hz 得到速度增量 5，120 Hz 得到 2.5。这不是单纯命名问题，而是 tick 改变玩法。

将状态改成语义明确的 `pending_impulse` 与持续 force/acceleration。瞬时量不乘 delta。现有文档倾向 delta-v，现有实现又除以 mass，必须先确定兼容单位；不能顺便改成另一个物理模型而不考虑现有作品。`external_forces` 持续累加且未逐步清零，也应明确它究竟是持续加速度还是每步施力。

验收：同样输入在 30/60/120 Hz 下产生一致的瞬时速度变化，单次冲量只消费一次，持续施力按明确的时间单位积分。

**2.4 录制首先需要一个正确的时间轴与容器写入边界**

桌面实际默认走自定义 AVI：`recorder/movie_writer_obs_runtime.cpp:629` 使用推荐后端覆盖配置，而 `post_merge_processor.cpp:549` 无条件优先 always-available 的 CUSTOM_AVI。代码中的“FFmpeg 不可用”日志不能证明真的检测过 FFmpeg。

默认合并器的长度字段与写入量不符：

- `post_merge_processor.cpp:671` 的视频 `LIST strl` 声明 132，实际 payload 为 `4 + (8+56) + (8+40) + (8+16) = 140`。
- 第 739 行音频 `LIST strl` 声明 84，按现有实际写入为 `4 + (8+64) + (8+16) = 100`。
- 第 744 行音频 `strh` 声明 56，但第 746–762 行写入 64；结尾写了四个 32-bit 字段。

按声明长度遍历的读取器会遇到错误 chunk 边界。此结论由源码字节计数确认，本次没有录制真实文件或测试不同播放器的容错行为。修复时应通过 begin/end chunk 按文件位置回填长度，避免再维护多份手算常量，并用独立读取器检查输出。

另有两个时间轴问题：`independent_video_recorder.h:57` 将采集间隔固定为 `1000000/30`，而 `.cpp:43` 给 writer 的是配置 FPS、`.cpp:199` 仍使用固定间隔。配置 60 FPS 时，约每秒采集 30 帧，却按 60 FPS 描述。`independent_audio_recorder.cpp:275` 在 underrun 后填静音但直接返回，未写入这段静音，外层调度时钟仍前进。

建议先让一个 `RecordingSession` 拥有起始时刻、配置和停止状态；视频 deadline 用 frame index/FPS 推导，音频时间轴用 sample count/sample rate 推导。选择“缺数据补静音/重复帧”或其他策略时，最终产物必须与所选时间轴一致。保留线程边界，但不再让多个类自行定义时钟语义。

`HybridAudioDriver` 当前实际是 Master bus 的 AudioEffect tap，并未替换 AudioDriver，名称和恢复驱动状态已脱节。`capture_buffer/get_captured_audio_data` 没有外部消费者，却仍填充数据。可收敛为一个 capture session、一条有界队列和明确消费者，删除无效驱动恢复状态与未消费缓冲。

合并后端需要明确的产品选择：若要求无外部依赖，就保留并修正一个 AVI muxer；若允许 FFmpeg，则显式启用该后端，删除默认“Phase 2 testing”选择。不要继续维护看似自动探测、实际恒选同一路径的分支。FFmpeg 分支还有把 `OS::execute` 的 Error 返回值误当子进程 exit code 的问题（`:127`、`:225`），但正常当前录制链不走它，因此没有将它列为默认路径的高优先级故障。

验收：24/30/60 FPS、48 kHz、音频 underrun、视频停更、停止失败；检查文件可解析、实际时长与音画时间差，不能只断言 `Error == OK`。

**2.5 所有视觉来源切换应共用一次准备与提交**

证据：`spx_sprite_animation.cpp:49` 起修改 `is_svg_mode/current_svg_scale` 并切换帧资源，但 play 和 play_backwards 的结尾都没有 `_update_anim_scale()`。逆向补偿 SVG 栅格倍率只在 `spx_sprite_render.cpp:99` 生效。Go `component_animation.go:240` 先设置 render scale，再在第 216 行调用 PlayAnim，前一次缩放使用的是旧来源模式。

非 SVG → 2× SVG 动画时，可以保留旧 anim2d scale，又换成 2×纹理，造成额外放大。反向切换也可能缩小。另一个问题是 `spx_sprite_render.cpp:94` 在资源加载成功前先发布新的 `current_svg_scale`，加载失败仍用新倍率缩放旧图。

建议只增加一个小型 `VisualSource`/`PreparedVisual` 数据结构，不新增 manager：准备阶段得到帧资源、来源类型和实际 raster scale；成功后一次提交来源、帧、缩放、offset、UV。set_texture、atlas、正播、倒播、SVG 变倍共用这一过程。失败保留完整旧状态。

验收：位图 ↔ SVG、单图 ↔ 动画、正播 ↔ 倒播、倍率变化失败，各种切换后显示尺寸与 offset 一致。

**2.6 动画缓存发布和失效入口应各只有一个**

`spx_res_mgr.cpp:402` 先创建共享动画条目，再加载帧；`_build_normal_frames` 在第 219 行失败会跳过帧，在第 228 行发现混合 SVG 才报错，没有回滚。下次相同 key 在第 392 行直接被视为已有动画。因此错误输入或暂时加载失败会留下部分/空 clip，并阻止修复后重试。

以临时 `AnimationClip` 收集帧、offset、SVG 标记和 FPS，校验 bitmap>0、类型与数量，全部成功后一次插入缓存。失败返回明确结果。这样可以一并减少 `SpriteFrames`、offset map、SVG registry 对同一动画信息的平行维护。现有 `project_font_transaction.cpp:265` 的 prepare 模式可直接作为参考。

资源 reload 也有路径不一致：`spx_res_mgr.cpp:286` 的普通加载用规范化后的路径缓存，`:301` 的 reload 用原始路径查询；未命中后调用普通加载，规范化后又命中旧缓存。相对路径发生转换时，reload 可能只是返回旧纹理。SVG load 走独立缓存，而 reload 没有对应失效。应统一入口处的路径规范化与 invalidate/reload，SVG rasterizer 接受已规范化来源；明确更新已有 texture 引用还是发布新资源。

验收：坏资源 → 修复 → 同 key 重试成功；相对/规范化路径更新同一资源；SVG 与位图 reload 遵循同一可见性契约。

**2.7 Web reset 必须同时结束事件和临时内存的生命周期**

`web/js/libs/library_godot_gdspx.js:125` 把 contact 保存到队列，第 213/219 行在 update/fixed_update 前派发；第 236 行的 reset 只结束临时数组借用并派发 reset，没有清空 contact。destroy 也未清空它。

使用现有测试 fixture 与实际 JS 源码的 Node VM 复现：先排入 `OnTriggerEnter(10,11)`，调用 reset，再 update，观察到顺序为 `OnEngineReset → OnTriggerEnter(10,11) → OnEngineUpdate`。这确认旧事件可跨 reset 被派发；本次没有进一步断言某个具体作品会崩溃。

最小修复是在 reset/destroy 清理 contact 与告警状态，并覆盖整个 teardown 区间：`spx_engine.cpp:411` 先调用 OnEngineReset，再清理 manager；只在 reset 入口清空一次仍可能留下清理期间新产生的事件。应停止接收旧周期事件，或在 reset 完成边界再次处理。如果某些 exit 事件必须保留，先定义 teardown 顺序并在旧 runtime 关闭前处理，不能无意留到下个 update。后续把事件队列、arena generation 和 module identity 归入一个 bridge session，使 reset/close 对三者生效。

还有一个借用期限加固点：`web/js/engine/gdspx.util.js:486` 重置/释放 arena，`:549` 却只凭 metadata 的 module 身份信任旧 descriptor。实际复现中，旧指针 64 已被释放，`RequireNativeArray` 仍接受 64。该 descriptor 的约定本来就是临时借用，目前未证明 Go 主调用链违规保留它，所以这是过期借用缺乏校验，不应直接宣称正常游戏必然 UAF。可用 session/frame generation 校验，或封装成调用范围内 borrow；不要恢复复杂的引用计数池。

**3. 架构简化方案**

**3.1 将 `SpxBaseMgr` 拆成两个真实职责**

`spx_base_mgr.h:73` 同时定义跨 ABI 的字符串/数组分配、Godot owner node、全局 engine 访问和十个生命周期钩子。它又被 `SpxEngine` 继承，engine 因而既是协调者又像一个被协调的 manager。所有 manager 默认创建 Node2D（`spx_base_mgr.cpp:69`、`:198`），即使只是资源/平台/扩展调用的门面。

第一步仅把 ABI 内存工具搬入独立 namespace/header，保留原静态函数兼容转发，不改导出 ABI。第二步让纯服务不再默认建 Node，由确实拥有场景对象的子系统显式创建根节点。`SpxEngine` 自己管理初始化/销毁顺序，不必继承 manager。明确 dependency 次序后再删除空钩子。

无需一次改掉所有 15 个 manager，也不要做动态 DI 容器。用少量显式依赖就能减轻 `spx_mgr_access.h:53` 的全局 service locator 耦合，例如让视觉加载直接接收资源入口，让 recorder 接收 capture source，而不是在深层随时访问所有 manager。

**3.2 生命周期和跨线程控制只保留一个执行边界**

当前存在三层：Go `internal/enginewrap/sync.gen.go:368` 已通过 callInMainThread 调度；`spx.cpp:182` 消费 atomic request flags；`spx_engine.cpp:359` 又有 call_deferred 与延迟 pause 状态。普通 Go 路径已经在主线程，不能把所有分支都当成必要的线程安全实现。

保留外部入口的线程适配，内部 engine 转为 main-thread-only，并断言这一契约。将 shutdown/reset/restart 的状态转换集中到一处；运行阶段、暂停/单步、reset 延迟可以是几个明确正交的状态，不要机械合并成一个包含所有组合的巨大 enum。

独立 pause/resume flags 还会改变顺序：非主线程先 Resume 再 Pause，在同次 update 中仍固定先 pause 再 resume。`is_set()` 后 `clear()` 也不是原子消费。此风险限定绕过正常 Go 同步包装的跨线程入口。确需保序的操作使用小型命令队列；只需最后意图的暂停控制可用单一 desired state。不要给每一个 manager 加锁。

销毁已具有重要的前后回调契约（`spx_engine.cpp:133` 和 `tests/test_spx_lifecycle.h`）：业务停止、manager 清理、engine 销毁后通知。重构必须保留这一区分。

**3.3 简化对象访问助手，保留所有权差异**

`spx_object_guard.h:68` 保存指针、valid 和 context，并实现移动/禁止复制，但没有资源取得/释放行为。它是 checked lookup，而不是延长 Godot 对象生命周期的 RAII guard。当前命名和宏层级容易给调用方错误安全感。

可收敛为 `find_*_checked(id, context)` 或已有的 `with_object`，统一主线程检查与诊断。Godot Node 使用 ObjectID 或绑定销毁通知；C++ wrapper 仍由 manager 独占。不要强迫 Sprite/UI/Audio/Pen 都套入一个泛型 ownership 模板。`SpxObjectMgr::_update_all` 的 ID 快照和重新解析用于防回调重入，应保留。

`SpxUiBinding::_connect_pressed_first` 的复杂连接重排用于保留历史 pressed 回调顺序，而且已有测试；不能简单以“代码多”为由删掉。只在明确允许改变该契约时进一步收敛。

**3.4 ABI 精简应修改生成器，优先移除标量装箱**

大型 `gdextension_spx_ext.*`、`web/godot_js_spx.*` 和 `gdspx.js` 很多是生成结果。优先审查声明、schema 和模板，不能通过手改产物减少代码行数。

数组已有直接 buffer/count 路径，回调标量也已经按值传递。普通 Web 调用却仍用 `ToGdBool/ToGdFloat/Alloc/Free` 等在 C++ pool 中装箱标量（`gdspx.util.js:98`、`:157`）。建议以 bool/float 为起点，在 `internal/cmd/codegen/generate/webffi/calls.go` 和对应 C++ 模板统一生成按值参数/返回值。64-bit ID 保留现有 low/high 精度契约，不能直接转 JS Number。

潜在收益是删除池、注册/释放及异常清理路径，同时减少 Wasm 往返次数；实际速度收益需基准验证，本审查没有性能测量。保持批处理 API，渐进迁移；只有确认所有消费方同步升级后才删旧导出，并按仓库发布规则处理 ABI 变化。

**3.5 建议的内部依赖形态**

```text
Go / JS / Native API
        ↓
生成的 ABI 适配：类型、内存、主线程入口
        ↓
SpxEngine：明确的运行状态、依赖及销毁顺序
        ├─ Sprite/Scene：节点与实例播放状态
        │       └─ ResourceStore：不可变 clip、纹理、SVG 栅格结果
        ├─ Audio：一个 voice 表 + bus pool
        ├─ UI：Control binding + ObjectID
        ├─ Physics：单位明确的积分 + 统一查询结果转换
        └─ RecordingSession：时钟、队列、后端
```

这是所有权与依赖方向的示意，不要求按图新增同名类或重新组织整个目录。优先减少一份状态或一个清理入口，而不是增加抽象层。

**4. 渐进实施顺序**

| 阶段 | 范围 | 验收标准 |
| --- | --- | --- |
| 第一批 | loop 隔离、普通音频 pause、impulse 语义、Web reset 清队列 | 每项独立行为回归；不夹带目录重组 |
| 第一批独立录制修复 | AVI chunk 长度、视频配置 FPS、音频 underrun 时间轴 | 独立容器读取与音画时长检查 |
| 第二批 | 按 clip 资源、prepare/commit visual、原子发布与统一 reload | 跨实例、跨模式、失败重试、倍率变化全部覆盖 |
| 第三批 | ABI 工具移出 BaseMgr、main-thread-only 内部执行、checked lookup | 初始化失败/重复关闭/reset-restart 顺序不变 |
| 第四批 | recorder capture/时钟/后端收敛；Web scalar codegen 按值 | 各自独立 PR，保留 Native/Web ABI 一致性与性能基线 |

其中 impulse 的既有单位、是否允许外部 FFmpeg、UI 回调顺序属于应明确的兼容/产品契约，其余多数内部整理可以保留现有外部 API 完成。优先级不代表必须串行等待，录制与动画修复可以独立推进。

**5. 验证范围与限制**

- 当前源码的 Web 测试：`node --test godot_modules/spx/web/tests/*.test.cjs`，24/24 通过。使用本机 ChatGPT 附带的 Node；默认 shell PATH 没有 node。
- 当前源码独立 C++ ABI 测试：`bash godot_modules/spx/web/tests/run_string_abi_test.sh`，普通构建和 UBSan 均通过。
- 对实际 JS 源码补充的只读 Node VM 复现确认了 reset 后旧 contact 派发，以及超出约定借用期的 descriptor 仍被接受；未把复现脚本写入仓库。
- 用现有 `godot.macos.editor.dev.arm64` 在临时目录验证了共享 SpriteFrames 的 loop 串扰和 AudioStreamPlayer2D 的 pause/is_playing 语义；二进制报告 `v4.4.1.stable.custom_build.09227b82b`。探针退出码为 0，有 ObjectDB 退出泄漏告警；这是底层语义验证，不算最新模块的完整集成回归。
- 本地既有 `godot.spx_lifecycle_tests`：55 cases、398 assertions 通过，但二进制时间为 9 月 19 日，早于 9 月 20 日的源码改动，因此只作为旧基线结果。较新的 editor.dev 二进制未启用 tests。
- 没有重新完整编译当前 Godot 模块，没有跑真实录制/浏览器端到端回归，没有对性能收益给出数值承诺。

现有覆盖对 ABI 校验、字体/SVG helper、批处理和生命周期清理较好。后续新增测试应集中在本次发现的跨实例、跨模式、跨 reset、跨采样率行为，以及最终录制产物；继续增加空钩子或 getter/setter 测试价值较低。
