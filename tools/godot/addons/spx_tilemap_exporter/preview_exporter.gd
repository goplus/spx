@tool
extends RefCounted
## TileMap-only PNG export shared by the editor plugin and command line.
## Use explicit preloads so a fresh project needs no global script class cache.

class ExportResult:
	var success: bool = false
	var error: String = ""
	var export_path: String = ""
	var image_size: Vector2i = Vector2i.ZERO


static func export_scene_as_png_async(scene_root: Node, viewport_parent: Node) -> ExportResult:
	var result = ExportResult.new()
	if not scene_root:
		result.error = "No scene is currently open"
		return result
	if DisplayServer.get_name() == "headless":
		result.error = "Preview export requires a rendering display"
		return result

	var copy = create_tilemap_only_copy(scene_root)
	if not copy:
		result.error = "Failed to duplicate scene"
		return result

	var viewport = SubViewport.new()
	viewport.disable_3d = true
	viewport.transparent_bg = true
	viewport.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	viewport.render_target_clear_mode = SubViewport.CLEAR_MODE_ALWAYS
	viewport_parent.add_child(viewport)
	viewport.add_child(copy)
	var bounds = calc_tilemap_bounds(find_tilemap_layers(copy))
	if not bounds.has_area():
		viewport.queue_free()
		result.error = "No TileMapLayer content found in scene"
		return result

	# Round outward and move the canvas, including scenes with a plain Node root.
	var origin = bounds.position.floor()
	viewport.size = Vector2i(bounds.end.ceil() - origin)
	viewport.canvas_transform = Transform2D(0.0, -origin)

	var scene_name = scene_root.scene_file_path.get_file().get_basename()
	if scene_name.is_empty():
		scene_name = scene_root.name
	var export_dir = "res://_export/" + scene_name
	var global_dir = ProjectSettings.globalize_path(export_dir)
	var err = DirAccess.make_dir_recursive_absolute(global_dir)
	if err != OK:
		viewport.queue_free()
		result.error = "Failed to create export directory: " + global_dir
		return result

	# Wait for actual draw completion instead of a fixed wall-clock delay.
	await RenderingServer.frame_post_draw
	await RenderingServer.frame_post_draw
	var image = viewport.get_texture().get_image()
	viewport.queue_free()
	if not image or image.is_empty():
		result.error = "Failed to capture viewport image"
		return result

	result.export_path = export_dir + "/preview.png"
	err = image.save_png(ProjectSettings.globalize_path(result.export_path))
	if err != OK:
		result.error = "Failed to save PNG: error code " + str(err)
		return result
	result.success = true
	result.image_size = image.get_size()
	print("Preview PNG saved to: ", result.export_path)
	return result


static func calc_tilemap_bounds(layers: Array[TileMapLayer]) -> Rect2:
	var total_rect = Rect2()
	for layer in layers:
		if not layer.enabled or not layer.is_visible_in_tree():
			continue
		var layer_rect = calc_single_layer_bounds(layer)
		if layer_rect.has_area():
			total_rect = merge_rects(total_rect, layer.get_global_transform() * layer_rect)
	return total_rect


static func calc_single_layer_bounds(layer: TileMapLayer) -> Rect2:
	if not layer or not layer.tile_set:
		return Rect2()
	var bounds = Rect2()
	# A diagonal of the used cell rectangle is insufficient for isometric maps.
	# Include each cell's rendered texture extent, including atlas texture origins.
	for cell in layer.get_used_cells():
		var size = Vector2(layer.tile_set.tile_size)
		var offset = Vector2.ZERO
		var source_id = layer.get_cell_source_id(cell)
		if layer.tile_set.has_source(source_id):
			var source = layer.tile_set.get_source(source_id)
			if source is TileSetAtlasSource:
				var coords = layer.get_cell_atlas_coords(cell)
				size = Vector2(source.get_tile_texture_region(coords).size)
				var data = layer.get_cell_tile_data(cell)
				if data:
					offset = Vector2(data.texture_origin)
					if data.transpose != layer.is_cell_transposed(cell):
						size = Vector2(size.y, size.x)
		var center = layer.map_to_local(cell) - offset
		bounds = merge_rects(bounds, Rect2(center - size / 2.0, size))
	return bounds


static func merge_rects(a: Rect2, b: Rect2) -> Rect2:
	if not a.has_area():
		return b
	if not b.has_area():
		return a
	return a.merge(b)


## Find all TileMapLayer nodes in the scene tree
static func find_tilemap_layers(node: Node) -> Array[TileMapLayer]:
	var layers: Array[TileMapLayer] = []
	_find_tilemap_layers_recursive(node, layers)
	return layers


static func _find_tilemap_layers_recursive(node: Node, layers: Array[TileMapLayer]) -> void:
	if node is TileMapLayer:
		layers.append(node)
	elif node is TileMap:
		for child in node.get_children(true):
			if child is TileMapLayer:
				layers.append(child)

	for child in node.get_children():
		_find_tilemap_layers_recursive(child, layers)


## Create a duplicate of the scene with only TileMapLayer nodes (all others removed)
## Returns the cleaned scene copy, or null on failure
## Set debug_log=true to print scene tree before/after cleanup
static func create_tilemap_only_copy(scene_root: Node, debug_log: bool = false) -> Node:
	var copy = scene_root.duplicate(0)
	if not copy:
		return null

	if debug_log:
		print("=== Original Scene Tree (before cleanup) ===")
		print_scene_tree(copy, 0)

	# Remove all non-TileMapLayer nodes
	remove_non_tilemap_nodes(copy, debug_log)

	if debug_log:
		print("=== Cleaned Scene Tree (after cleanup) ===")
		print_scene_tree(copy, 0)

	return copy


## Print the scene tree for debugging
static func print_scene_tree(node: Node, indent: int) -> void:
	var indent_str = "  ".repeat(indent)
	var node_type = node.get_class()
	var visible_str = ""
	if node is CanvasItem:
		visible_str = " [visible=%s]" % str(node.visible)
	print("%s%s (%s)%s" % [indent_str, node.name, node_type, visible_str])

	for child in node.get_children():
		print_scene_tree(child, indent + 1)


## Remove all nodes that are not TileMapLayer or necessary parent containers
static func remove_non_tilemap_nodes(root: Node, debug_log: bool = false) -> void:
	# First, collect all TileMapLayer nodes and their ancestor paths
	var nodes_to_keep: Dictionary = {}
	_collect_tilemap_ancestors(root, nodes_to_keep)

	if debug_log:
		print("Nodes to keep: %d" % nodes_to_keep.size())
		for node in nodes_to_keep.keys():
			print("  - %s (%s)" % [node.name, node.get_class()])

	# Remove all nodes not in the keep list
	_delete_unwanted_nodes(root, nodes_to_keep, debug_log)


static func _collect_tilemap_ancestors(node: Node, nodes_to_keep: Dictionary) -> bool:
	var has_tilemap_descendant = false

	if node is TileMapLayer or node is TileMap:
		has_tilemap_descendant = true

	for child in node.get_children():
		if _collect_tilemap_ancestors(child, nodes_to_keep):
			has_tilemap_descendant = true

	if has_tilemap_descendant:
		nodes_to_keep[node] = true

	return has_tilemap_descendant


static func _delete_unwanted_nodes(node: Node, nodes_to_keep: Dictionary, debug_log: bool) -> void:
	var children = node.get_children()
	for child in children:
		if nodes_to_keep.has(child):
			_delete_unwanted_nodes(child, nodes_to_keep, debug_log)
		else:
			if debug_log:
				print("Deleting: %s (%s)" % [child.name, child.get_class()])
			node.remove_child(child)
			child.free()
