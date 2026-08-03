# 微信小游戏实现（历史方案记录）

本文记录早期的微信小游戏适配思路。当前导出入口和实现已经迁移到 `spx exportminigame`，本页中的文件系统、音频和 wasm 修改步骤不要直接照做。

## 当前入口

```bash
spx exportminigame
spx exportminigame -build=fast  # 本地调试时跳过 Brotli 压缩
```

导出流程位于 `cmd/spx/internal/command/export_web.go`，小游戏模板位于 `cmd/spx/template/platform/webminigame/`。模式和线程限制见 [Web 平台模式说明](./engine/web.md)。

## 当前适配边界

- 小游戏构建使用单线程；不能把普通 Web Worker 构建直接当作小游戏构建。
- 音频、文件系统、`WXWebAssembly` 和 Go WASM 启动由模板适配层共同处理。
- Go WASM 的兼容 facade 说明见[微信小游戏 Go WASM 兼容层](./wxgame_go_wasm_adapter.md)。
- 真机字符串解码问题见[Go WASM 微信小游戏字符串解码排查](./wechat_minigame_go_wasm_panic.md)。
- 微信包体和运行时限制可能变化，发布前应以微信官方文档和当前基础库为准，不要使用本页旧的固定大小或版本结论。

## 历史记录

早期方案曾讨论过替换 AudioWorklet、封装文件系统接口和修改 wasm 加载方式。这些内容仅用于理解演进背景；如果需要调整适配器，应修改模板源文件并重新执行导出，不要编辑导出目录中的临时文件。

## 参考资料

1. [微信小游戏官方文档](https://developers.weixin.qq.com/minigame/dev/guide/index.html)
2. [Godot 官方网站](https://godotengine.org/)
3. [godot-wechat](https://github.com/yuchenyang1994/godot-love-wechat)
