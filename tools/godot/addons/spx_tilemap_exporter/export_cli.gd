@tool
extends SceneTree
## Command-line export script for SPX TileMap and Decorator data
##
## Usage: godot --headless --path <project_path> -s addons/spx_tilemap_exporter/export_cli.gd
##
## Configuration is done via constants below:

const TileMapExtractor = preload("res://addons/spx_tilemap_exporter/tilemap_extractor.gd")
const DecoratorExtractor = preload("res://addons/spx_tilemap_exporter/decorator_extractor.gd")

# ============================================================================
# Configuration - modify these as needed
# ============================================================================
const SCENE_PATH = "res://main.tscn"
const EXPORT_TILEMAP = true
const EXPORT_DECORATORS = true
# ============================================================================

func _init() -> void:
	# Get scene name for export directory
	var scene_name = SCENE_PATH.get_file().get_basename()
	var export_base = "res://_export/" + scene_name

	print("SPX Export CLI")
	print("==============")
	print("Scene:  ", SCENE_PATH)
	print("Output: ", export_base)
	print("Export TileMap: ", EXPORT_TILEMAP)
	print("Export Decorators: ", EXPORT_DECORATORS)
	print("")

	# Load the scene
	var packed_scene = load(SCENE_PATH)
	if not packed_scene:
		printerr("ERROR: Failed to load scene: ", SCENE_PATH)
		quit(1)
		return

	if not packed_scene is PackedScene:
		printerr("ERROR: Loaded resource is not a PackedScene: ", SCENE_PATH)
		quit(1)
		return

	var scene_root = packed_scene.instantiate()
	if not scene_root:
		printerr("ERROR: Failed to instantiate scene")
		quit(1)
		return

	# Ensure export directory exists
	var global_export_dir = ProjectSettings.globalize_path(export_base)
	if not DirAccess.dir_exists_absolute(global_export_dir):
		var err = DirAccess.make_dir_recursive_absolute(global_export_dir)
		if err != OK:
			printerr("ERROR: Failed to create export directory: ", global_export_dir)
			scene_root.queue_free()
			quit(1)
			return

	var has_error = false
	var node_offset = Vector2.ZERO

	# Export TileMap
	if EXPORT_TILEMAP:
		var tilemap_path = export_base + "/tilemap.json"
		var global_tilemap_path = ProjectSettings.globalize_path(tilemap_path)
		var tilemap_result = _export_tilemap(scene_root, global_tilemap_path)
		if tilemap_result.success:
			node_offset = tilemap_result.node_offset
		elif tilemap_result.error != "skipped":
			has_error = true

	# Export Decorators
	if EXPORT_DECORATORS:
		var decorator_path = export_base + "/decorator.json"
		var global_decorator_path = ProjectSettings.globalize_path(decorator_path)
		var decorator_result = _export_decorators(scene_root, global_decorator_path, node_offset)
		if not decorator_result.success and decorator_result.error != "skipped":
			has_error = true

	scene_root.queue_free()

	if has_error:
		print("")
		print("Export completed with errors")
		quit(1)
	else:
		print("")
		print("Export completed successfully!")
		quit(0)


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
