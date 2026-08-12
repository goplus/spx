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
