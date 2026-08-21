# `spx buildlauncher`

`spx buildlauncher` 不启动 Godot，也不构建或运行 XGo project driver，直接
生成包含 Engine、PCK、bridge 和项目归档的自包含 host launcher。

```bash
spx buildlauncher --path ./game
spx buildlauncher --path ./game -o ./bin/game
```

默认输出为 `./game/.builds/game`（Windows 自动加 `.exe`）。相对 `-o` 路径
相对于当前目录解析。命令在目标目录内私有 staging 目录构建，验证成功后原子
替换旧文件；构建失败不会破坏旧产物。输出不能覆盖 `.spx` 源码、项目配置、
module 元数据、选中的 bridge 源码或 pack 目录。

目标 module 必须包含 `gox.mod` 或 `gop.mod`，其中声明 `.spx` project 和
`pack`。所有顶层 `.spx` 文件以及 pack 目录下的文件都会进入 launcher。
bridge 使用当前 Go graph 选中的 SPX module 构建，支持 module cache、main
module 和 local replacement，不依赖本地 XGo driver。
命令固定本次调用的 `GOWORK` 和 `GOFLAGS`，并在 bridge 与 launcher 构建前后
校验 module 选择和 graph 元数据；bridge 与 payload 通过内容摘要绑定。

显式设置的 `SPX_RUNTIME_LOCAL_MANIFEST` 或 `SPX_RUNTIME_ASSET_DIR` 优先，且
校验失败不会静默回退。其他情况先使用经过校验的 release cache/下载；仅当
当前 Go graph 选中 SPX 源码 checkout 时，runtime 尚未发布或下载失败才回退到
首个 `GOPATH/bin` 中版本名完全一致的 Engine 与 PCK。两者必须是普通非符号
链接文件，打包前会重新计算摘要。文件不存在时应在 SPX checkout 执行
`make dev`；`buildlauncher` 不会隐式启动 Godot 编译。设置
`SPX_RUNTIME_OFFLINE=1` 后不访问网络，只尝试 cache 和本地回退。
