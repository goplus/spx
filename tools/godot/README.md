# SPX Godot project tools

These files are SPX-owned tools for Godot projects. They live in the SPX
repository so the Godot fork contains only the generic hooks needed to build
the external module.

## TileMap exporter

Copy `tools/godot/addons/spx_tilemap_exporter` from this repository into the
target Godot project's `addons` directory, then enable **SPX TileMap Exporter**
in the project's Plugins settings.

For headless export, run the copied script from the target project:

```sh
godot --headless --path /path/to/project \
  -s addons/spx_tilemap_exporter/export_cli.gd
```

The exporter writes SPX TileMap and decorator data under `res://_export`.
It also provides a Python wrapper with scene selection and addon copying, plus
TileMap PNG previews through the CLI and the editor's Ctrl+Shift+E shortcut.

See the [exporter guide](addons/spx_tilemap_exporter/README.md) or
[中文说明](addons/spx_tilemap_exporter/README.zh.md) for installation and usage.
These features are ported from goplus/godot [#243](https://github.com/goplus/godot/pull/243)
and [#244](https://github.com/goplus/godot/pull/244).

Run exporter tests with a Godot editor executable:

```sh
GODOT_BIN=/path/to/godot python3 -m unittest discover -s tools/godot/tests -v
```

With a rendering display available, set `SPX_TEST_PREVIEW=1` as well to verify
the exported PNG's dimensions, tile pixels and transparent background.
