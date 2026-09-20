# 内容尺寸、统一拉伸接口与留边背景

本修复保留在 `fix/godot-web-stage-letterbox`，基于完整重构分支 `refactor/godot-integrated` 的 `55583c46a6`，没有反向合入整合分支。

## 问题的本质

`05-Animation` 的逻辑舞台是 202×360，宿主 iframe 是 480×360。宿主画布需要填满容器，游戏内容却必须保留舞台比例；二者不能共用同一组内容尺寸。

旧实现把宿主尺寸同时写入 Godot 的 `content_scale_size`。DPR 1 时相机看到 480×360，而舞台边界只有 202×360。`Camera2D::get_camera_transform()` 依次按左右边界夹紧可见矩形：宽度超过两边界的间距，右侧修正最终把相机左边移到 -379，舞台左边 -101 显示在画布 x=278 附近。它是视口尺寸与相机边界不匹配，不是缺少一次宽高比切换来刷新缓存。

修复使用已有 `ResolvePlatformLayout` 计算的拟合内容尺寸，同时保持宿主尺寸及相机缩放：

| DPR | 宿主画布像素 | 内容尺寸 | 相机 zoom | 可见世界 |
| --- | --- | --- | --- | --- |
| 1 | 480×360 | 202×360 | 1 | 202×360 |
| 2 | 960×720 | 404×720 | 2 | 202×360 |

内容采用 KEEP，因此在 CSS 坐标下两侧各约 139 像素。C++ 不再乘除 DPR 或 `WindowScale`；相机限制和输入坐标约定不变。

## 历史修复与顺序判断

- Godot `f49c98577225a551f80763761a8cb3f45f3dc289`（2025-10-30，`Fix Web Black Borders / Fix Web Pixel Ratio`）明确把 `set_stretch_aspect(enable)` 改为 `false`，同时修改画布的 CSS 尺寸和 DPR 换算。它最终使用 IGNORE，没有后续 KEEP。
- 配套 SPX `f4f03625573b26ef27d921a58fcc9163261a93ac` 改用宿主窗口尺寸，并把拉伸设置移到 `camera.init` 前。这些顺序和尺寸处理继续保留。
- Web 末尾设置拟合内容尺寸、再设 KEEP，首次出现于本修复的 `2261037bd3`。历史记录没有证明 IGNORE→KEEP 是必须的刷新技巧。
- 用于顺序验证的 Godot `09227b82bffef39163abbbe03fc696095d2cb207` 的 `scene/main/window.cpp` 与上述历史修复时相同。`Window::_update_viewport_size()` 每次从当前尺寸、模式和宽高比重算 `window_transform`；`Viewport::_set_size()` 同样重新计算 `stretch_transform`。这里没有需要 IGNORE 清除的累计变换状态。

通过公开 Window/Camera2D API 对比 `mode → IGNORE → size → KEEP` 与 `mode → size → KEEP`：36 组场景覆盖宽窄、横竖、零尺寸、缩放、相机边界、重复设置和关闭后重新启用；连同另一种目标宽高比优先的顺序，共 576 组逐步比较，即时、两帧后及强制相机更新后的最终变换与实际鼠标输入坐标一致。原顺序累计触发 516 次 `size_changed`，省略 IGNORE 后为 336 次；典型重复设置从两次变为零次。

这个原生 headless 实验使用与 Web 相同的 Window 实现，浏览器仍另行验证。结论针对最终布局和输入坐标，不声称中间通知流相同；省略中间切换本身会减少布局通知。

## 最终接口

不保留兼容入口，三个旧绑定由一个接口替代：

```cpp
SPX_BIND void set_stretch(GdBool enabled, GdInt content_width, GdInt content_height);
```

Go 只调用一次：

```go
platformMgr.SetStretch(p.displayState.StretchMode, layout.ContentWidth, layout.ContentHeight)
```

- Web 启用：CANVAS_ITEMS → 已拟合内容尺寸 → KEEP，省略临时 IGNORE。
- Native 启用：CANVAS_ITEMS + KEEP，沿用 `set_window_size` 设置的内容尺寸和 macOS HiDPI 换算。调整窗口宽高比时等比缩放并留边，避免横纵方向分别拉伸；窗口和内容宽高比相同时，显示结果与 IGNORE 一致。
- 关闭或 reset：DISABLED + IGNORE，不使用内容尺寸参数。

`set_window_size` 内部直接调用 Godot 尺寸 setter。没有新增尺寸缓存或测试钩子；Go/JS/Native 绑定均由生成器生成。新的 Go 绑定和 Godot runtime 必须配套构建。

## 留边沿用项目背景色

SPX `e4d7f38cdc`（2026-05-28）将 Web runner 的页面、canvas 和 Godot 默认背景统一设为白色。KEEP 产生的窗口留边不属于内容视口；Godot 4.4.1 的 GLES3 和 RD 合成器原先把这部分清成黑色，所以这些白色配置不能直接控制留边。

Godot 补丁只修改这两条合成路径：读取已有 `texture_storage->get_default_clear_color()` 的 RGB，保留不透明 alpha，再绘制内容视口。项目的 `rendering/environment/defaults/default_clear_color` 和公开的 `RenderingServer.set_default_clear_color()` 因而同时控制视口背景及窗口留边。没有新增 SPX 接口、布局状态或输入补偿；场景显式使用透明背景时，窗口留边仍保持原有的不透明语义。

分支依赖：

- SPX：`fix/godot-web-stage-letterbox`，包含 Native/Web 的 KEEP 策略。
- Godot：`joeykchen/godot` 的 `fix/window-letterbox-background`，提交 `f168f0bc95f19dbbee37a70802bd5cfa57df76a9`；基于原来的 `09227b82…`，只修改两个合成器。
- SPX 锁定上述 Godot 提交，使用 SPX `v3.3.1` / runtime `3.0.1` 候选声明，ABI 仍为 3。已发布的 runtime `3.0.0` 快照及 SPX `v3.3.0` 映射保持不变。依赖通过 `pin-godot-candidate` 验证，目前未发布；Godot 补丁合入锁定的上游分支前，发布检查仍会拒绝正式发布。

## 统一接口验证

以下记录对应 `bf8edcd2cf` 的统一接口与顺序验证；留边颜色补丁的验证另列。

- `make generate` 完成；再次生成与已有修改逐字节一致。三个旧绑定和导出已移除，宽高继续按 64 位值精确传输。
- 固定 Go 1.25.8 的根包、项目布局、Native binding、enginewrap、codegen 及 js/wasm binding/impl 测试通过；普通 Web 的 33 项 Node 测试通过。
- 在本分支执行 `SPX_EMBED_RUNTIME=0 make dev MODE=normal`，使用 Godot `09227b82bffef39163abbbe03fc696095d2cb207`、Apple Clang、Emscripten 3.1.62，完成编辑器、Native runtime、普通 Web runtime 及 Go 运行库构建安装。
- 最终代码实跑 `05-Animation` 的 `spx runnative`：敌机、碰撞动画、左右鼠标发射列正常，退出返回 0。
- 普通 Web 分别使用保留临时 IGNORE 与省略临时 IGNORE 的真实构建产物，完成 DPR 1/2 对照。初始状态以及两轮关闭、开启、重复设置后的视口、世界可见矩形、相机 zoom、舞台 limits 和宿主尺寸快照完全一致。
- 最终版本在 DPR 1/2 下均通过持续动画、鼠标输入和 Stop/Start 检查，舞台边界约 x=140…341、中心误差 0.5 CSS 像素，子弹列移动约 80 像素。
- 没有游戏、JavaScript 或 Wasm 异常；缺失 favicon 的 404 与主动 Stop 的 `onGameExit 0` 单独识别。结束后关闭本次启动的窗口、浏览器和服务器。

本轮没有增加生产测试接口。公开 API 临时对照项目及原始报告保存在 `/tmp/spx-window-order-probe/`；浏览器对照报告位于 `/tmp/spx-stretch-{ordered,direct}-dpr{1,2}/report.json`。验证覆盖当前普通 Web 示例与上述窗口场景，不替代完整游戏集或跨浏览器回归。

前轮已通过 `05-Animation` 的动画、鼠标输入、Stop/Start 和 DPR 1/2 检查。前轮首次截图采样曾在 12 帧中只捕获 1 帧目标子弹，未达到至少 2 帧断言；未修改游戏或断言，复测通过。它不作为本轮新接口的验收结果。

## 白色留边验证

- 对新的固定 Godot 提交执行 `make generate` 和 `SPX_EMBED_RUNTIME=0 make dev MODE=normal`，完成 Native 编辑器、runtime、普通 Web 和 Go 工具链的配套构建安装。`internal/release`、构建来源校验和项目布局测试通过。
- 公开 Window/RenderingServer API 的外部临时项目在 macOS OpenGL3 和 Metal 上完成 8 个真实窗口场景。项目白色、运行时蓝绿色和运行时白色均正确填充横向或纵向留边；即使默认颜色 alpha 为 0 或 0.15，留边仍保持不透明。只改变颜色不会改变内容位置或尺寸。原始截图按显示器 ICC 转换为 sRGB 后取样验证，两种渲染器已有的纵向取整差异为 1 像素。
- 普通 Web `05-Animation` 在 DPR 1/2 下分别检查初始、放大和恢复尺寸，全部留边取样均为 RGBA `(255,255,255,255)`。canvas 和 WebGL 绘图缓冲区均等于 CSS 尺寸乘 DPR，可见世界始终为 `[-101,-180,202,360]`，没有 JavaScript 页面异常。
- 额外的浏览器动画、鼠标和 Stop/Start 检查在首轮停止时记录了一次 `destroy_sprite` 空对象警告；动画、输入和重新启动仍正常，但该轮严格控制台检查失败。为避免再次打开用户测试窗口，使用同一脚本的 headless 模式复核，保留全部断言，动画、左右输入、重新启动及控制台检查通过。这个警告未被忽略，也没有为颜色修改扩大对象生命周期改动范围；一次复核通过不代表清理警告已被修复。

临时报告位于 `/tmp/spx-white-margin-probe/summary.json`、`/tmp/spx-white-margins-dpr{1,2}.json`、`/tmp/spx-white-margins-play-dpr2/report.json` 和 `/tmp/spx-white-margins-recheck-dpr2/report.json`。这些检查不向生产代码添加测试接口。
