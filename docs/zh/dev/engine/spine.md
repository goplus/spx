# Spine 动画

精灵可以使用 Spine 骨骼代替逐帧造型。在
`assets/sprites/<name>/index.json` 中配置 `spineAnim`：

```json
{
  "size": 1,
  "visible": true,
  "heading": 90,
  "spineAnim": {
    "atlas": "hero.atlas",
    "skeleton": "hero.skel",
    "defaultMix": 0.1,
    "stepDuration": 0.01,
    "turnToDuration": 1,
    "animMap": { "idle": "idle", "walk": "run" }
  },
  "defaultAnimation": "idle",
  "animBindings": { "step": "walk" }
}
```

图集和骨骼路径相对于精灵配置目录解析，遵循其他精灵资源的路径规则。
骨骼支持 Spine 二进制（`.skel`）或 JSON 格式。图集内的纹理页路径相对于
图集文件解析，且必须位于图集目录或其子目录中。绝对路径、URL 协议、
父目录（`..`）片段和符号链接会被拒绝。打包时需要同时包含图集、骨骼和全部纹理页。
项目打包器保留本地资源，并收集引用的外部图集纹理页；原生启动包会收集
Spine 类型资源引用，包括位于项目内其他目录的文件。

`animMap` 将 SPX 动画名称映射为骨骼内的动画名称，别名和目标名称都不能为空。
`fAnimations` 还可以声明 `isLoop`、`isKeepOnStop`、`stepDuration`、
`turnToDuration` 等动画元数据，但逐帧索引和帧率不决定骨骼动画的播放。
Spine 模式不需要造型；同时配置逐帧造型时也不会使用它们渲染。
动画时长从骨骼资源读取。
Spine 支持虚像透明度和整体颜色调制；颜色、亮度等位图 shader 特效不作用于骨骼网格。

`defaultMix`、`stepDuration`、`turnToDuration` 为零或未设置时，默认值分别为
0.1、0.01、1 秒。负数或非有限时间值会被拒绝。`fAnimations` 中声明的动画
使用自身的时间设置，步进和转向时间默认分别为 0.01 和 1 秒；只有未在
`fAnimations` 中声明的映射动画才使用 `spineAnim` 的时间设置。

## 构建与验证

Spine **默认关闭**。公开绑定仍然存在，但默认构建中的
`engine.Managers().SpriteMgr.IsSpineSupported()` 返回 false。加载 Spine 内容需要使用
`SPX_SPINE=1` 本地构建编辑器和运行时。模块已包含 Spine 运行时及其 Godot
集成，无需额外的 Spine 模块。

请使用全新、独立的 GOPATH，避免开启 Spine 的二进制覆盖或混用默认已发布
运行时。`SPX_SPINE_GOPATH` 明确声明该目录仅用于 Spine；首次使用要求 `bin`
目录为空，并且不能使用默认 GOPATH。在子 shell 中执行以下命令，
将 `GODOT_SRC` 指向与仓库运行时锁定配置一致的源码目录：

```sh
(
  unset GOBIN
  export GOPATH="$HOME/.spx-spine"
  export SPX_SPINE_GOPATH="$GOPATH"
  export SPX_SPINE=1
  export GODOT_SRC=/absolute/path/to/godot
  make dev MODE=normal
  make run DEMO_INDEX=1
)
```

在同一环境中，后续可按需执行：

```sh
make build-editor
make build-desktop
make build-web MODE=normal
```

Makefile 会将开关导出给 buildctl，后者在编辑器、桌面、Web 及挂载模块的
SCons 参数路径中传入 `spx_spine=yes`。关闭时传入 `spx_spine=no`；共享默认
SCons 配置不能启用 Spine。每次构建都会执行 SCons，切换特性不会直接复用
未经检查的已就绪二进制。独立 `bin` 目录会生成持久的 `.spx-spine` 标记及
Godot 的 `_sc_` 自包含标记。导出模板安装到
`$GOPATH/bin/editor_data/export_templates/<engine-version>`，不会覆盖用户的
全局 Godot 模板目录。此 GOPATH 只能用于 Spine；关闭开关后 buildctl 会拒绝使用它。

要构建不包含 Spine 的版本，请退出子 shell（恢复普通 GOPATH），然后执行：

```sh
GODOT_SRC=/absolute/path/to/godot make dev SPX_SPINE=0 MODE=normal
```

独立工作区不会下载默认已发布运行时，请勿在其中执行 `make setup`、
`make setup-web`、`make download` 或 `make download-engine`。首次请使用
`make dev` 构建完整本地环境。本功能不会新增已发布的 Spine 产物，也不会
修改不可变的运行时快照。发布 Spine 发行版需要单独规划运行时标识和发布流程；
不得将这些二进制发布到默认运行时标签。
CLI 也只选择此独立目录中的运行时，不使用内嵌或 PATH 中的备用运行时；
原生启动包使用其中本地的引擎/PCK 配对，而非已发布缓存。缺失本地产物时会
提示重新构建，不会下载默认运行时。前置条件参见[当前模块构建流程](cmd_make.md)。

Web 的准备、构建、导出和运行应使用同一模式。请使用目标版本导出的骨骼资源，
在原生和 Web 环境验证多页图集、动画切换、克隆和碰撞边界。

Linux amd64 下可使用自制资源的原生 smoke fixture 验证真实 Go 绑定和动画完成：

```sh
./testdata/spine_native_smoke/run.sh /path/to/enabled/godot enabled
./testdata/spine_native_smoke/run.sh /path/to/enabled/godot enabled export
./testdata/spine_native_smoke/run.sh /path/to/default/godot disabled
SPX_SPINE_GODOT=/path/to/enabled/godot go test ./internal/release -run TestSpineNoticeExport
```

这些无界面测试不验证 GPU 画面。Web 绑定具备 Go/WASM 和 JavaScript 测试，
但完整的 Spine Web 引擎及浏览器渲染仍需在发布前完成平台验证。

内置 Spine 运行时适用独立的许可证。使用或分发 Spine 内容前，请查看
[运行时许可条款](https://github.com/EsotericSoftware/spine-runtimes/blob/4.2/LICENSE)
以及 `godot_modules/spx/thirdparty/spine_godot` 中的许可证文件。
开启此功能不等于获得 Spine Editor 许可。创建包含该运行时应用的开发者必须
满足适用的 Editor 许可要求；工具作者的许可不能覆盖下游开发者。
分发启用版时应保留随附的两份许可通知。
