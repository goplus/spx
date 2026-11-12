# SPX TileMap Exporter

将 Godot TileMap 图层、装饰器、纹理和碰撞数据导出为 SPX 支持的 JSON 格式。
插件还支持导出透明背景的 TileMap PNG 预览，并提供 **Ctrl+Shift+E** 编辑器快捷键。

需要 **Godot 4.3+ 编辑器**可执行文件。Python 脚本需要 Python 3.8+。
PNG 导出需要图形显示环境；数据导出可以在无界面模式运行。

## 快速开始

在 SPX 仓库中运行，复制插件到已有 Godot 项目并导出场景：

```sh
python3 tools/godot/addons/spx_tilemap_exporter/export.py \
  --godot /path/to/godot --project /path/to/project \
  --copy --scene levels/level1.tscn
```

脚本先导入项目资源，再导出场景。生成预览时会短暂打开 Godot 窗口。
自动化任务只需要数据时，加上 `--no-preview`：

```sh
python3 tools/godot/addons/spx_tilemap_exporter/export.py \
  --godot /path/to/godot --project /path/to/project \
  --copy --scene levels/level1.tscn --no-preview
```

## 安装与 Python 参数

也可以手动将 `tools/godot/addons/spx_tilemap_exporter` 复制到目标项目的
`addons/` 目录。安装后的脚本可以从任意工作目录运行：

```sh
python3 /path/to/project/addons/spx_tilemap_exporter/export.py \
  --godot /path/to/godot --scene levels/level1.tscn
```

| 参数 | 说明 |
| --- | --- |
| `--godot PATH` | Godot 编辑器的可执行文件路径或 PATH 中的命令，优先于 `GODOT_PATH` |
| `--project PATH` | 包含 `project.godot` 的目标目录；默认使用当前插件所在项目 |
| `--copy` | 将此脚本所在插件目录复制或更新到目标项目，保留无关文件；源和目标相同时跳过复制 |
| `--scene PATH` | 项目相对场景路径，可带 `res://` 前缀；默认 `res://main.tscn` |
| `--no-preview` | 跳过预览渲染，不打开窗口 |

也可以在 Linux/macOS 使用 `export GODOT_PATH=/path/to/godot`，或在 Windows
命令提示符使用 `set GODOT_PATH=C:\path\to\godot.exe`，省略 `--godot` 参数。
开发插件时，从仓库执行脚本并指定 `--project PATH --copy`，即可更新目标项目。

## 直接使用 Godot CLI

安装插件后，需要时先导入纹理资源：

```sh
godot --headless --path /path/to/project --import
```

导出数据和 PNG 预览：

```sh
godot --path /path/to/project \
  --script addons/spx_tilemap_exporter/export_cli.gd \
  -- --scene res://levels/level1.tscn
```

仅导出数据：

```sh
godot --headless --path /path/to/project \
  --script addons/spx_tilemap_exporter/export_cli.gd \
  -- --scene levels/level1.tscn --no-preview
```

脚本参数放在 `--` 之后。无界面模式会自动跳过预览，即使没有传入
`--no-preview`。脚本无需预先生成编辑器的脚本类缓存；新增图片资源仍需要导入。
参数错误、场景不存在或导出失败时，程序返回非零退出码。

## 编辑器菜单与快捷键

在 **项目 > 项目设置 > 插件** 中启用 **SPX TileMap Exporter**。
**项目 > 工具** 菜单提供：

| 菜单项 | 导出内容 |
| --- | --- |
| SPX Export TileMap... | TileMap JSON 和纹理 |
| SPX Export Decorators... | 装饰器 JSON 和纹理 |
| SPX Export All... | 上述两类数据 |
| SPX Export Preview PNG (Ctrl+Shift+E) | TileMap PNG 预览 |

编辑 2D 节点时按 **Ctrl+Shift+E** 可导出预览。预览使用场景副本，排除装饰器，
不会修改当前打开的场景。

## 输出

导出文件保存在 `res://_export/<场景名>/`：

```text
_export/level1/
├── tilemap.json
├── tilemap/          # 复制的 TileSet 纹理
├── decorator.json
├── decorator/        # 复制的装饰器纹理
└── preview.png      # 透明背景，仅包含 TileMap
```

场景名取不带扩展名的文件名。不同目录下的同名场景共用输出目录。
场景中没有 TileMap 或装饰器时，跳过对应导出；禁用预览不会删除已有 PNG。

装饰器导出会跳过 `spx_ignore` 分组中的节点，以及名称含 `_ignore` 或 `_skip`
的节点。TileMap 与装饰器保留原有 SPX 数据格式及坐标转换。
可以在 `export_cli.gd` 中通过 `DEFAULT_SCENE_PATH`、`EXPORT_TILEMAP`、
`EXPORT_DECORATORS` 和 `EXPORT_PREVIEW` 修改默认导出配置。
