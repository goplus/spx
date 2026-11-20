# 项目资源

## 项目索引中的资源名与路径

打开项目时，SPX 会在资源根目录（通常是 `assets/`）查找 `index_pack.json`。该文件可以集中存放项目设置和 `sprites`、`sounds`、`fonts` 三类配置。[打包索引示例](../../../test/IndexPackMode/assets/index_pack.json)。

如果同时存在 `index.json` 和 `index_pack.json`，同名顶层字段以打包索引为准。角色和声音按名称查找，优先使用打包索引中的同名条目，找不到时读取各自目录下的 `index.json`。打包索引中的 `fonts` 字段定义整个字体目录清单，即使该字段为空也以它为准。只有缺少该字段时，SPX 才扫描 `assets/fonts/`。

XBuilder、sb2xbp 等项目生成工具须在创建角色、声音或字体族目录前校验名称。每个名称必须是一个非空的 POSIX 路径段。不能使用 `.`、`..`，也不能包含斜杠（`/`）或 NUL。允许空格和 Unicode。反斜杠和冒号在 POSIX 名称中是普通字符。例如，`"bgm": "主题曲"` 选用 `sounds/主题曲/index.json` 定义的声音，音频文件由该索引的 `path` 字段指定。

资源引用使用 `/`，相对于源配置所在的目录解析。打包条目沿用对应源配置的目录。例如，`assets/sprites/Hero/index.json` 和 `index_pack.json` 的 `sprites.Hero` 条目都可以用 `../../shared/image.png` 引用 `assets/shared/image.png`。旧项目若曾在 `.config` 中使用 `extasset`，迁移时应将共享文件移入 `assets/` 并更新引用。`/a/b`、`C:/a/b`、UNC 路径等绝对路径，`res://` 等 URI，以及指向素材根目录外的引用都会被拒绝。字体文件还须位于对应的字体族目录中。

旧代码若向 `PlaceTile` 或 `PlaceTiles` 传入 `../../tile.png` 等指向项目外的路径，可将图片移至 `assets/tiles/tile.png`，并改传 `tiles/tile.png`。原先调用 `ispx.ConfigureLegacyFilesystemRoots` 的 Go 宿主须改用 `ispx.ConfigureFilesystemRoots`，并将运行时资源放在项目根目录内。

### 不同平台如何读取资源

| 环境 | 资源位置和查找规则 |
| --- | --- |
| Linux、macOS 桌面 | 源项目从选定的 `assets/` 读取和列举文件。从生成的 `project/` 启动且本地没有 `assets/` 时，使用 `../assets/`。原生 POSIX 源索引可以引用名为 `res:image.png` 的文件。 |
| Windows 桌面 | 素材根的选择与其他桌面平台相同。项目配置统一用 `/`，运行时在访问磁盘时转换为 Windows 路径。磁盘文件名不能含冒号。 |
| Web 预览和导出（`ispx`） | 解释器从内存读取源码和 JSON，Godot 从已挂载的项目文件读取图片和音频。造型路径 `res:image.png` 指向角色目录下的同名文件。 |
| Android、iOS | 移动端素材接口以 `assets` 为固定入口。Android 通过 AssetManager 读取应用素材，iOS 从应用包读取；导出的游戏还可通过下述 `packmode` 读取 Godot 资源。 |

桌面直接读取会解析符号链接并校验目标仍在素材根内。桌面解释器逐段拒绝符号链接。导出采用 `packmode` 时，引擎将相对资源路径映射为 `res://assets/…`（默认素材根），项目 JSON 里仍写相对路径。桌面文件名匹配遵循所在卷的规则，SPX 的 ZIP 读取器按条目名精确匹配，因此配置中的大小写应与文件一致。Godot 将反斜杠解释为分隔符，跨平台资源文件名应避开反斜杠和冒号。

## 静态 WebP 图片素材

SPX 在所有接受位图纹理的位置支持静态 WebP，包括背景、独立造型、动画序列帧以及图集源图片。普通序列帧可以混用 WebP、PNG 和 JPEG。

SPX 不会把动画 WebP 文件本身当成动画播放。动画时序和帧范围仍由 SPX 项目配置描述，素材使用多张静态图片或图集。

只把 PNG 或 JPEG 文件重命名为 `.webp` 并不会完成格式转换。图片应实际编码为 WebP，并让扩展名与文件内容保持一致；否则纹理加载器会将其报告为损坏文件。

像素画、清晰的透明边缘以及参与逐像素碰撞的造型，建议使用无损 WebP。只有照片类素材且允许少量颜色变化时，才建议使用有损 WebP。

WebP 可以减小项目和下载体积，但不会降低图片解码后的纹理内存。SPX 可能在角色初始化或注册动画时解码并缓存帧纹理，因此大规模序列帧应在实际 Web 环境和低端设备上验证加载时间。
