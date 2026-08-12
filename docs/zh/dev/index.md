# SPX 开发文档导航

欢迎来到 SPX 开发文档中心。根据您的角色和需求，您可以选择下面的文档进行阅读。

## 用户角色导航

### SPX 使用者

如果您是 SPX 的使用者，您可能对以下文档更感兴趣：

- [命令行工具 (spx) 使用指南](./game/cmd_spx.md) - 学习如何使用 SPX 命令行工具创建、运行和导出项目

### SPX 开发者

如果您是 SPX 的开发者或贡献者，您可能对以下文档更感兴趣：

- [Makefile 命令指南](./engine/cmd_make.md) - 学习如何使用项目中的 Makefile 命令进行开发和构建
- [SPX 与 Godot runtime 发布流程](./engine/release.md) - 了解版本身份、冻结顺序、验证矩阵和三阶段 runtime 自举

## 文档分类

### 游戏开发文档

游戏开发文档主要面向使用 SPX 进行游戏开发的用户，包括：

- 命令行工具使用指南
- [动画绑定音频说明](./game/animation_audio.md) - 了解 `onStart` / `onPlay` 的生命周期和动画停止时的音频行为
- [项目字体配置](./project_fonts.md) - 了解项目字体目录和字体元数据规则
- [微信小游戏旧版说明](./wechat_minigame.md) - 了解历史方案及当前文档入口
- [微信小游戏 Go WASM 兼容层](./wxgame_go_wasm_adapter.md) - 了解当前小游戏 Go WASM 启动兼容层
- [Go WASM 字符串解码排查](./wechat_minigame_go_wasm_panic.md) - 排查真机字符串解码问题

仓库路径：`docs/zh/dev/game/` 和 `docs/zh/dev/`

### 引擎开发文档

引擎开发文档主要面向 SPX 引擎的开发者和贡献者，包括：

- Makefile 命令指南
- SPX 与 Godot runtime 发布流程
- 引擎架构
- 构建系统
- [Web 端截图与固定帧接入说明](./engine/web_capture.md) - 了解外部页面如何像模板 `index.html` 一样接入截图 host、baseline/runs 保存与对比流程
- [输入录制与回放说明](./engine/input_replay.md) - 了解 Web host 固定 FPS 输入录制、逐 tick 回放与截图测试配合方式
- [SPX 坐标系统与 Godot 边界](./engine/coordinate_system.md) - 了解逻辑、资产、渲染、物理和 Godot 坐标的所有权与转换规则
- [代码生成器说明](./engine/code_generator.md) - 了解生成代码和接口绑定规则
- [物理 API 设计草案](./engine/physic_api.md) - 了解物理 API 的历史设计和当前实现差异
- [Web 平台模式说明](./engine/web.md) - 了解 normal、worker、minigame 和 miniprogram 模式
- [Web Worker 方案说明](./engine/web_worker_mode.md) - 了解 worker 模式的实现和历史方案
- [Web Sync Batch](./engine/web_sync_batch.md) - 了解 Web 批量同步协议
- [小游戏 Go WASM 兼容层](./engine/web_minigame_go_wasm_adapter.md) - 兼容层文档入口
- [小游戏 Go WASM panic 排查](./engine/web_minigame_go_wasm_panic.md) - 字符串解码排查文档入口

仓库路径：`docs/zh/dev/engine/`

## 快速开始

### 对于 SPX 使用者

准备预编译的本机 editor 和 runtime 资产，然后运行 demo：

```sh
make setup
make list-demos
make run DEMO_INDEX=2
```

使用 Web 预编译资产时：

```sh
make setup-web MODE=normal
make runweb DEMO_INDEX=2
```

项目命令的详细用法参见 [命令行工具 (spx) 使用指南](./game/cmd_spx.md)。

### 对于 SPX 开发者

修改 Godot 或 SPX 外置模块时，从源码构建完整的本地开发环境：

```sh
GODOT_SRC=/absolute/path/to/godot make dev MODE=normal
```

详细参数和专用平台目标参见 [Makefile 命令指南](./engine/cmd_make.md)。资产准备和源码构建统一使用 `setup`、`setup-web` 和 `dev` 入口。

## 参与贡献

我们欢迎对 SPX 项目的贡献，无论是代码贡献、文档改进还是问题报告。如果您有兴趣参与贡献，请阅读开发者文档了解更多信息。
