# Godot 全分支去冗余记录

2026-09-20 对本次 Godot 工作的 14 个分支各完成一轮实质简化。范围为 Native 与普通 Web；不包含录制、Worker，也不修改无关的项目分支。每条主题线独立提交，再以 merge 汇入最终整合分支。

## 每个分支的改动

| 分支 | 本轮提交 | 简化内容 |
| --- | --- | --- |
| `refactor/godot-lifecycle-controls` | `e26944aca8` | 用一个状态更新函数统一静默暂停和恢复，保留控制发布顺序 |
| `refactor/godot-audio-ownership` | `26791d34d5` | 合并设置、读取、回收时的 panner 查找，删除重复遍历 |
| `refactor/godot-manager-lifecycle` | `8279e9965d` | 用成员函数分发统一八组 manager 生命周期遍历 |
| `refactor/godot-object-access` | `a437ab197b` | 合并五个入口的主线程校验和对象查找，保留调用者诊断 |
| `refactor/godot-visual-resources` | `3e367b9576` | 正向和反向动画共用准备结果提交、播放及帧更新流程 |
| `refactor/godot-resource-lifetimes` | `b3d0a7e493` | 删除 WAV 薄封装；MP3 复用已打开的文件；缓存只查找一次 |
| `refactor/godot-sprite-algorithms` | `1aded100a8` | 边界触碰直接按原优先级返回，删除距离临时量和最小值选择 |
| `refactor/godot-abi-boundaries` | `2ae629d5a1` | 数组类型尺寸统一定义，空数组与非空数组共用 Web 注册和失败清理 |
| `fix/godot-web-session-lifetime` | `3848e2f6bb` | 共用 reset/destroy 收尾；用跨越阈值检测替代重复告警状态 |
| `refactor/godot-bindings-simple` | `0c7fbace08` | ABI 方法名称统一生成；raw/standard typedef 共用输出流程 |
| `perf/godot-web-scalars` | `25e72bfb4b` | 普通标量直接传给 Invoke，删除重复转换及约 80 个生成临时变量，保留 float32 精度 |
| `refactor/godot-integrated` | `0d0b61443f` | 删除空工具源文件及未用转换；射线定点打包仅保留在物理实现中 |
| `refactor/godot-simplify-three-rounds` | `0e19a55fae` | 删除没有消费者的数组 descriptor 状态，保留借用期和原实例内存释放 |
| `fix/godot-web-stage-letterbox` | `e7d4eb71c6` | 平台布局一次计算内容尺寸，调用处直接使用，消除重复缩放计算 |

整合时沿用最终代码的 `_require_main_thread` / `_find_object` 命名与统一纹理加载签名。额外提交 `d04afa413d` 调整生成器分派顺序：先处理需要低/高位拆分的 64 位值，再处理普通标量，防止标量优化绕过精确传输。原有混合参数测试覆盖该交叉问题。

收尾扫描另外清掉了五个旧主题分支遗留的 `TestSpxInternalsAccessor` / `TestSpxEngineInternalsAccessor` 及测试专用 friend：ABI `7c7c693d5b`、audio `d6526cd779`、object-access `ab3c3444a2`、visual `0179cbdec0`、Web session `5c42dfff1e`。保留公开 API 测试，没有引入替代钩子。上述分支的新 HEAD 也已合入整合历史；整合版本原本已含这些清理，代码与已验证版本保持一致。

相对上一轮整合 `6a4dec9b57`，本轮整合代码净减 326 行（含生成文件，不含文档）。没有增加测试专用 friend、生产钩子或新的 schema。

## 最终分支关系

- `refactor/godot-integrated` 是完整重构入口，包含上表除 letterbox 外全部提交及整合修正。
- `refactor/godot-simplify-three-rounds` 同步到整合分支，最终两个分支指向同一提交。
- `fix/godot-web-stage-letterbox` 在完整重构之上保留历史布局修复及本轮布局简化。整合分支不反向包含该修复。
- 其余 11 个主题分支保留各自独立改动与历史依赖，没有把完整整合反灌到每条主题线。它们的新 HEAD 都已通过 merge 成为最终整合分支的祖先。
- 早期分支的创建基点与依赖见[三轮重构记录](godot_modules_three_rounds.md)。需要使用全部代码时，直接采用整合分支；需要同时修复历史 Web 布局时，采用 letterbox 分支，无需重复 cherry-pick。

## 验证

在与整合分支同源的 `three-rounds` 工作区构建；Godot 保持 `09227b82bffef39163abbbe03fc696095d2cb207`，Go 固定为 1.25.8，普通 Web 使用 Emscripten 3.1.62、`threads=no`。

- `make generate` 完成，生成产物与提交内容一致。
- Native 测试构建、链接通过；`*SPX*,*Loop phase callbacks*` 共 72 个用例、639 个断言通过。
- 普通 Web 编译、链接与模板打包通过；正式 `make dev MODE=normal` 完成编辑器、Native runtime、普通 Web runtime 及 Go 运行库安装。
- codegen 全部 Go 测试、33 项 Node 测试、Go js/wasm 的 `binding/web` 与 `impl` 测试通过。
- ABI 分支的独立数组、Web 字符串 ABI 检查及 UBSan 通过；letterbox 分支的 `internal/core/project` 测试通过。
- 合并后的 C++ 与 Web 改动完成独立只读复核；没有发现新的行为回退。

整合分支实跑 `05-Animation` 的 `spx runweb`：动画、左右鼠标发射子弹以及 Stop/Start 后重跑均通过，子弹列移动约 80 像素，没有游戏或 Wasm 异常。主整合分支保留已知历史布局问题；letterbox 独立验证见其分支上的 `godot_web_stage_letterbox.md`。

Native 实跑同一示例，确认敌机移动、碰撞动画、左右鼠标位置对应的子弹发射列正常；保存四张窗口截图，正常退出后 `spx runnative` 返回 0。

letterbox 分支采用同一批新 Godot 产物，重新安装本分支 Go 运行时后，DPR 1 与 DPR 2 均通过舞台居中、持续动画、左右鼠标输入及 Stop/Start 检查。舞台边界约为 x=140…341，中心误差 0.5 CSS 像素，子弹发射列横移约 80 像素；无游戏或 Wasm 异常。

首次 letterbox 截图检查在 12 帧中只捕获到 1 帧目标子弹，未满足至少 2 帧的断言；同一未改代码重跑及 DPR 2 检查均通过。该视觉采样存在波动，未据此修改游戏实现，也不把单个示例验证视为完整游戏集回归。
