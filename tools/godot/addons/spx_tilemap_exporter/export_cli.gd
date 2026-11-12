@tool
extends SceneTree
## Command-line export script for SPX TileMap and Decorator data
##
## Usage: godot --path <project_path> -s addons/spx_tilemap_exporter/export_cli.gd [-- --scene <scene_path> --no-preview]
##
## Arguments:
##   --scene <path>  Path to the scene file to export (default: res://main.tscn)
##   --no-preview    Export only data (also automatic with --headless)
##
## Examples:
##   godot --headless --path . -s addons/spx_tilemap_exporter/export_cli.gd
##   godot --headless --path . -s addons/spx_tilemap_exporter/export_cli.gd -- --scene res://levels/level1.tscn

const TileMapExtractor = preload("res://addons/spx_tilemap_exporter/tilemap_extractor.gd")
const DecoratorExtractor = preload("res://addons/spx_tilemap_exporter/decorator_extractor.gd")
const PreviewExporter = preload("res://addons/spx_tilemap_exporter/preview_exporter.gd")

# ============================================================================
# Configuration - default values (can be overridden via command line)
# ============================================================================
const DEFAULT_SCENE_PATH = "res://main.tscn"
const EXPORT_TILEMAP = true
const EXPORT_DECORATORS = true
const EXPORT_PREVIEW = true  # Export scene preview PNG

var _scene_path: String = DEFAULT_SCENE_PATH
var _export_preview: bool = EXPORT_PREVIEW


func _init() -> void:
	# Run after SceneTree initialization so a preview viewport can enter the tree.
	call_deferred("_run")


func _run() -> void:
	if not _parse_arguments():
		quit(1)
		return
	if DisplayServer.get_name() == "headless":
		_export_preview = false

	var export_base = "res://_export/" + _scene_path.get_file().get_basename()
	print("SPX Export CLI")
	print("Scene: ", _scene_path)
	print("Output: ", export_base)
	print("Export Preview: ", _export_preview)

	var packed_scene = load(_scene_path)
	if not packed_scene is PackedScene:
		printerr("ERROR: Failed to load a PackedScene: ", _scene_path)
		quit(1)
		return

	var scene_root = packed_scene.instantiate()
	if not scene_root:
		printerr("ERROR: Failed to instantiate scene: ", _scene_path)
		quit(1)
		return
	var global_export_dir = ProjectSettings.globalize_path(export_base)
	var err = DirAccess.make_dir_recursive_absolute(global_export_dir)
	if err != OK:
		printerr("ERROR: Failed to create export directory: ", global_export_dir)
		scene_root.free()
		quit(1)
		return

	var has_error = false
	var node_offset = Vector2.ZERO
	if EXPORT_TILEMAP:
		var tilemap_path = ProjectSettings.globalize_path(export_base + "/tilemap.json")
		var tilemap_result = _export_tilemap(scene_root, tilemap_path)
		if tilemap_result.success:
			node_offset = tilemap_result.node_offset
		elif tilemap_result.error != "skipped":
			has_error = true

	if EXPORT_DECORATORS:
		var decorator_path = ProjectSettings.globalize_path(export_base + "/decorator.json")
		var decorator_result = _export_decorators(scene_root, decorator_path, node_offset)
		if not decorator_result.success and decorator_result.error != "skipped":
			has_error = true

	if _export_preview:
		var preview_result = await PreviewExporter.export_scene_as_png_async(scene_root, root)
		if not preview_result.success:
			if preview_result.error == "No TileMapLayer content found in scene":
				print("Preview: No TileMapLayer content found (skipped)")
			else:
				printerr("ERROR: Preview export failed: ", preview_result.error)
				has_error = true

	scene_root.free()
	print("Export completed with errors" if has_error else "Export completed successfully!")
	quit(1 if has_error else 0)


# ============================================================================
# TileMap Export
# ============================================================================

class TileMapResult:
	var success: bool = false
	var error: String = ""
	var node_offset: Vector2 = Vector2.ZERO

func _export_tilemap(scene_root: Node, output_path: String) -> TileMapResult:
	var result = TileMapResult.new()

	# Find all TileMapLayer nodes
	var layers = _find_tilemap_layers(scene_root)
	if layers.is_empty():
		print("TileMap: No TileMapLayer nodes found (skipped)")
		result.error = "skipped"
		return result

	print("Found ", layers.size(), " TileMapLayer(s)")

	# Check if any layer has a TileSet
	var has_tileset = false
	for layer in layers:
		if layer.tile_set:
			has_tileset = true
			break

	if not has_tileset:
		printerr("ERROR: No TileSet found in TileMapLayer nodes")
		result.error = "No TileSet found"
		return result

	# Calculate node_offset for decorator alignment
	result.node_offset = _calculate_tilemap_offset(layers)

	# Export using TileMapExtractor with "tilemap" as textures subdirectory
	var extractor = TileMapExtractor.new()
	var export_result = extractor.export_tilemap(layers, output_path, "tilemap")

	if export_result.success:
		print("TileMap Export successful!")
		print("  Output: ", output_path)
		print("  Layers: ", export_result.layer_count)
		print("  Textures: ", export_result.texture_count)
		result.success = true
	else:
		printerr("ERROR: TileMap export failed: ", export_result.error)
		result.error = export_result.error

	return result


# ============================================================================
# Decorator Export
# ============================================================================

class DecoratorResult:
	var success: bool = false
	var error: String = ""

func _export_decorators(scene_root: Node, output_path: String, node_offset: Vector2) -> DecoratorResult:
	var result = DecoratorResult.new()

	# Export using DecoratorExtractor with "decorator" as textures subdirectory
	var extractor = DecoratorExtractor.new()
	var export_result = extractor.export_decorators(scene_root, output_path, node_offset, "decorator")

	if export_result.success:
		print("Decorator Export successful!")
		print("  Output: ", output_path)
		print("  Decorators: ", export_result.decorator_count)
		print("  Textures: ", export_result.texture_count)
		result.success = true
	else:
		if export_result.error == "No decorators found in scene":
			print("Decorators: None found (skipped)")
			result.error = "skipped"
		else:
			printerr("ERROR: Decorator export failed: ", export_result.error)
			result.error = export_result.error

	return result


# ============================================================================
# Helper Functions
# ============================================================================

func _parse_arguments() -> bool:
	var args = OS.get_cmdline_user_args()
	var i = 0
	while i < args.size():
		match args[i]:
			"--no-preview":
				_export_preview = false
			"--scene":
				i += 1
				if i >= args.size() or args[i].begins_with("--") or args[i].is_empty():
					printerr("ERROR: --scene requires a project-relative scene path")
					return false
				var scene_arg = args[i].trim_prefix("res://")
				if scene_arg.is_absolute_path() or ".." in scene_arg.split("/"):
					printerr("ERROR: Scene must be inside the project: ", args[i])
					return false
				_scene_path = "res://" + scene_arg
			_:
				printerr("ERROR: Unknown argument: ", args[i])
				return false
		i += 1
	return true


func _find_tilemap_layers(node: Node) -> Array[TileMapLayer]:
	var layers: Array[TileMapLayer] = []
	_find_tilemap_layers_recursive(node, layers)
	return layers


func _find_tilemap_layers_recursive(node: Node, layers: Array[TileMapLayer]) -> void:
	if node is TileMapLayer:
		layers.append(node)
	elif node is TileMap:
		# TileMap node contains TileMapLayer as internal children
		for child in node.get_children(true):
			if child is TileMapLayer:
				layers.append(child)

	# Continue searching in regular children
	for child in node.get_children():
		_find_tilemap_layers_recursive(child, layers)


func _calculate_tilemap_offset(layers: Array[TileMapLayer]) -> Vector2:
	if layers.is_empty():
		return Vector2.ZERO

	var tileset = layers[0].tile_set
	if tileset == null:
		return Vector2.ZERO

	var tile_size = tileset.tile_size

	# Calculate bounds
	var min_x: int = 0x7FFFFFFF
	var max_x: int = -0x80000000
	var min_y: int = 0x7FFFFFFF
	var max_y: int = -0x80000000
	var has_tiles: bool = false

	for layer in layers:
		var global_pos = layer.position
		var layer_offset_x: int = floori(global_pos.x / tile_size.x)
		var layer_offset_y: int = floori(global_pos.y / tile_size.y)

		var used_cells = layer.get_used_cells()
		for cell in used_cells:
			has_tiles = true
			var world_x = cell.x + layer_offset_x
			var world_y = cell.y + layer_offset_y
			min_x = mini(min_x, world_x)
			max_x = maxi(max_x, world_x)
			min_y = mini(min_y, world_y)
			max_y = maxi(max_y, world_y)

	if not has_tiles:
		return Vector2.ZERO

	# Calculate center offset
	var center_x: int = (min_x + max_x) / 2
	var center_y: int = (min_y + max_y) / 2

	return Vector2(-center_x * tile_size.x, -center_y * tile_size.y)
