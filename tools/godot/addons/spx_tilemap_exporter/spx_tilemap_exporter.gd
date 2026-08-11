@tool
extends EditorPlugin

const TileMapExtractor = preload("res://addons/spx_tilemap_exporter/tilemap_extractor.gd")
const DecoratorExtractor = preload("res://addons/spx_tilemap_exporter/decorator_extractor.gd")

var _tilemap_extractor: TileMapExtractor
var _decorator_extractor: DecoratorExtractor


func _enter_tree() -> void:
	_tilemap_extractor = TileMapExtractor.new()
	_decorator_extractor = DecoratorExtractor.new()
	add_tool_menu_item("SPX Export TileMap...", _on_export_tilemap_pressed)
	add_tool_menu_item("SPX Export Decorators...", _on_export_decorators_pressed)
	add_tool_menu_item("SPX Export All...", _on_export_all_pressed)


func _exit_tree() -> void:
	remove_tool_menu_item("SPX Export TileMap...")
	remove_tool_menu_item("SPX Export Decorators...")
	remove_tool_menu_item("SPX Export All...")
	if _tilemap_extractor:
		_tilemap_extractor = null
	if _decorator_extractor:
		_decorator_extractor = null


# ============================================================================
# Export Path Helpers
# ============================================================================

## Get the export base directory: res://_export/{scene_name}/
func _get_export_base_dir(scene_root: Node) -> String:
	var scene_name = scene_root.scene_file_path.get_file().get_basename()
	if scene_name.is_empty():
		scene_name = scene_root.name
	return "res://_export/" + scene_name


## Ensure export directory exists
func _ensure_export_dir(export_dir: String) -> bool:
	var global_path = ProjectSettings.globalize_path(export_dir)
	if not DirAccess.dir_exists_absolute(global_path):
		var err = DirAccess.make_dir_recursive_absolute(global_path)
		if err != OK:
			push_error("Failed to create export directory: %s (error: %d)" % [global_path, err])
			return false
	return true


# ============================================================================
# TileMap Export
# ============================================================================

func _on_export_tilemap_pressed() -> void:
	var edited_scene = get_editor_interface().get_edited_scene_root()
	if not edited_scene:
		_show_error("No scene is currently open.")
		return

	var layers = _find_tilemap_layers(edited_scene)
	if layers.is_empty():
		_show_error("No TileMapLayer or TileMap nodes found in the current scene.")
		return

	var has_tileset = false
	for layer in layers:
		if layer.tile_set:
			has_tileset = true
			break

	if not has_tileset:
		_show_error("No TileSet found in TileMapLayer nodes.")
		return

	_export_tilemap_only(edited_scene, layers)


func _export_tilemap_only(scene_root: Node, layers: Array[TileMapLayer]) -> void:
	var export_base = _get_export_base_dir(scene_root)
	if not _ensure_export_dir(export_base):
		_show_error("Failed to create export directory: " + export_base)
		return

	var export_path = export_base + "/tilemap.json"
	var global_export_path = ProjectSettings.globalize_path(export_path)

	var result = _tilemap_extractor.export_tilemap(layers, global_export_path, "tilemap")

	if result.success:
		_show_info("TileMap Export successful!\n\nExported to: %s\nLayers: %d\nTextures copied: %d" % [
			export_path,
			result.layer_count,
			result.texture_count
		])
	else:
		_show_error("TileMap Export failed:\n" + result.error)


# ============================================================================
# Decorator Export
# ============================================================================

func _on_export_decorators_pressed() -> void:
	var edited_scene = get_editor_interface().get_edited_scene_root()
	if not edited_scene:
		_show_error("No scene is currently open.")
		return

	_export_decorators_only(edited_scene)


func _export_decorators_only(scene_root: Node) -> void:
	var export_base = _get_export_base_dir(scene_root)
	if not _ensure_export_dir(export_base):
		_show_error("Failed to create export directory: " + export_base)
		return

	# Calculate node_offset from tilemap bounds if available
	var node_offset = Vector2.ZERO
	var layers = _find_tilemap_layers(scene_root)
	if not layers.is_empty():
		node_offset = _calculate_tilemap_offset(layers)

	var export_path = export_base + "/decorator.json"
	var global_export_path = ProjectSettings.globalize_path(export_path)

	var result = _decorator_extractor.export_decorators(scene_root, global_export_path, node_offset, "decorator")

	if result.success:
		_show_info("Decorator Export successful!\n\nExported to: %s\nDecorators: %d\nTextures copied: %d" % [
			export_path,
			result.decorator_count,
			result.texture_count
		])
	else:
		_show_error("Decorator Export failed:\n" + result.error)


# ============================================================================
# Export All (TileMap + Decorators)
# ============================================================================

func _on_export_all_pressed() -> void:
	var edited_scene = get_editor_interface().get_edited_scene_root()
	if not edited_scene:
		_show_error("No scene is currently open.")
		return

	_export_all(edited_scene)


func _export_all(scene_root: Node) -> void:
	var export_base = _get_export_base_dir(scene_root)
	if not _ensure_export_dir(export_base):
		_show_error("Failed to create export directory: " + export_base)
		return

	var messages: Array[String] = []
	var has_error = false

	# Export TileMap
	var layers = _find_tilemap_layers(scene_root)
	var node_offset = Vector2.ZERO

	if not layers.is_empty():
		var tilemap_path = export_base + "/tilemap.json"
		var global_tilemap_path = ProjectSettings.globalize_path(tilemap_path)

		var tilemap_result = _tilemap_extractor.export_tilemap(layers, global_tilemap_path, "tilemap")
		if tilemap_result.success:
			messages.append("TileMap: %d layers, %d textures" % [tilemap_result.layer_count, tilemap_result.texture_count])
			node_offset = _calculate_tilemap_offset(layers)
		else:
			messages.append("TileMap failed: " + tilemap_result.error)
			has_error = true
	else:
		messages.append("TileMap: No layers found (skipped)")

	# Export Decorators
	var decorator_path = export_base + "/decorator.json"
	var global_decorator_path = ProjectSettings.globalize_path(decorator_path)

	var decorator_result = _decorator_extractor.export_decorators(scene_root, global_decorator_path, node_offset, "decorator")

	if decorator_result.success:
		messages.append("Decorators: %d items, %d textures" % [decorator_result.decorator_count, decorator_result.texture_count])
	else:
		if decorator_result.error == "No decorators found in scene":
			messages.append("Decorators: None found (skipped)")
		else:
			messages.append("Decorators failed: " + decorator_result.error)
			has_error = true

	# Show result
	var title = "Export Complete" if not has_error else "Export Completed with Errors"
	var message = "Export Results:\n\n" + "\n".join(messages)

	if not has_error:
		message += "\n\nFiles exported to:\n" + export_base

	if has_error:
		_show_error(message)
	else:
		_show_info(message)


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
		for child in node.get_children(true):
			if child is TileMapLayer:
				layers.append(child)

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


func _show_error(message: String) -> void:
	var dialog = AcceptDialog.new()
	dialog.title = "SPX Export Error"
	dialog.dialog_text = message
	dialog.confirmed.connect(dialog.queue_free)
	dialog.canceled.connect(dialog.queue_free)
	get_editor_interface().get_base_control().add_child(dialog)
	dialog.popup_centered()


func _show_info(message: String) -> void:
	var dialog = AcceptDialog.new()
	dialog.title = "SPX Export"
	dialog.dialog_text = message
	dialog.confirmed.connect(dialog.queue_free)
	dialog.canceled.connect(dialog.queue_free)
	get_editor_interface().get_base_control().add_child(dialog)
	dialog.popup_centered()
