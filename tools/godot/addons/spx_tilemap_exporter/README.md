# SPX TileMap Exporter

Export Godot TileMap layers, decorators, textures and collision data to the JSON
format loaded by SPX. The plugin also exports a transparent PNG preview of the
TileMap layers and provides a **Ctrl+Shift+E** editor shortcut.

Requires a **Godot 4.3+ editor** executable. The Python wrapper requires Python
3.8+. PNG export needs a rendering display; data export can run headless.

## Quick start

From the SPX repository, copy the addon and export a scene in an existing Godot
project:

```sh
python3 tools/godot/addons/spx_tilemap_exporter/export.py \
  --godot /path/to/godot --project /path/to/project \
  --copy --scene levels/level1.tscn
```

The wrapper imports project resources first, then exports the scene. Preview
export briefly opens a Godot window. Add `--no-preview` for data-only automation:

```sh
python3 tools/godot/addons/spx_tilemap_exporter/export.py \
  --godot /path/to/godot --project /path/to/project \
  --copy --scene levels/level1.tscn --no-preview
```

## Installation and Python options

You can also copy `tools/godot/addons/spx_tilemap_exporter` into the target
project's `addons/` directory manually. Run the installed script from anywhere:

```sh
python3 /path/to/project/addons/spx_tilemap_exporter/export.py \
  --godot /path/to/godot --scene levels/level1.tscn
```

| Option | Description |
| --- | --- |
| `--godot PATH` | Godot editor executable path or command on PATH; overrides `GODOT_PATH` |
| `--project PATH` | Target directory containing `project.godot`; defaults to the project containing this addon |
| `--copy` | Copy/update this script's addon directory in the target project, preserving unrelated files; copying onto itself does nothing |
| `--scene PATH` | Project-relative scene path, with optional `res://`; defaults to `res://main.tscn` |
| `--no-preview` | Skip preview rendering and run without a window |

Use `export GODOT_PATH=/path/to/godot` on Linux/macOS or
`set GODOT_PATH=C:\path\to\godot.exe` in Windows Command Prompt as an alternative
to `--godot`. For plugin development, run the repository's script with
`--project PATH --copy` to update the target project.

## Direct Godot CLI

After installing the addon, import texture resources when needed:

```sh
godot --headless --path /path/to/project --import
```

Export with a PNG preview:

```sh
godot --path /path/to/project \
  --script addons/spx_tilemap_exporter/export_cli.gd \
  -- --scene res://levels/level1.tscn
```

Export only data:

```sh
godot --headless --path /path/to/project \
  --script addons/spx_tilemap_exporter/export_cli.gd \
  -- --scene levels/level1.tscn --no-preview
```

Script options belong after `--`. Headless mode automatically skips preview
rendering, even without `--no-preview`. The scripts do not require an editor
script-class cache; importing textures is still necessary for new image assets.
Invalid arguments, missing scenes and failed exports return a nonzero exit code.

## Editor menu and shortcut

Enable **SPX TileMap Exporter** under **Project > Project Settings > Plugins**.
The **Project > Tools** menu provides:

| Menu item | Output |
| --- | --- |
| SPX Export TileMap... | TileMap JSON and textures |
| SPX Export Decorators... | Decorator JSON and textures |
| SPX Export All... | Both data exports |
| SPX Export Preview PNG (Ctrl+Shift+E) | TileMap PNG preview |

**Ctrl+Shift+E** exports a preview while editing 2D nodes. Preview export uses a
copy of the scene and excludes decorators. It preserves the open scene.

## Output

Files are written below `res://_export/<scene_name>/`:

```text
_export/level1/
├── tilemap.json
├── tilemap/          # Copied TileSet textures
├── decorator.json
├── decorator/        # Copied decorator textures
└── preview.png      # Transparent TileMap-only preview
```

The scene name is its filename without the extension. Scenes with the same name
share the output directory, even when stored in different folders. Missing
TileMap or decorator content is skipped. Disabling preview leaves any previously
exported PNG in place.

Decorator export skips nodes in the `spx_ignore` group and names containing
`_ignore` or `_skip`. TileMap data and decorators retain the existing SPX format,
including its coordinate conversion. Default export selection is configurable
through `DEFAULT_SCENE_PATH`, `EXPORT_TILEMAP`, `EXPORT_DECORATORS` and
`EXPORT_PREVIEW` in `export_cli.gd`.
