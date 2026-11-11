# Tilemap exporter fixture

`exported.json` is the unmodified output of `tools/godot/addons/spx_tilemap_exporter/tilemap_extractor.gd`, generated with the locked Godot fork (`4706c8e85a10313fb6e1bf98fe81358b25ff96c2`). It exercises the exporter-to-runtime format without relying on a C++ serializer.

Input: an 8 × 8 TileSet with source 3, a 16 × 16 `atlas.png`, one triangular collision polygon, and collision layer/mask 2/5. The `ground` layer has cell `(2, -1)`, offset `(2.5, -3)`, and z-index -2; `background` is empty, disabled, and has z-index 4. The loader test creates the texture separately.
