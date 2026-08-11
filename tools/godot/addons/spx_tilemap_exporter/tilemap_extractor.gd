@tool
extends RefCounted
## TileMap data extractor for SPX JSON format export
##
## This class extracts TileMapLayer, TileSet, and physics data from Godot nodes
## and exports them to a JSON format compatible with SPX runtime loading.

class_name TileMapExtractor

## Export result structure
class ExportResult:
	var success: bool = false
	var error: String = ""
	var layer_count: int = 0
	var texture_count: int = 0


## Main export function
## textures_subdir: subdirectory name for textures (e.g., "tilemap" for _export/scene/tilemap/)
func export_tilemap(layers: Array[TileMapLayer], export_path: String, textures_subdir: String = "textures") -> ExportResult:
	var result = ExportResult.new()

	if layers.is_empty():
		result.error = "No TileMapLayer nodes provided"
		return result

	# Get the first layer's TileSet (they should all share the same TileSet)
	var tileset: TileSet = null
	for layer in layers:
		if layer.tile_set:
			tileset = layer.tile_set
			break

	if not tileset:
		result.error = "No TileSet found in any TileMapLayer"
		return result

	# Calculate bounds and center offset to position tilemap at world origin (0, 0)
	var tile_size = tileset.tile_size
	var bounds = _calculate_tilemap_bounds(layers, tile_size)
	var center_offset_tiles = _calculate_center_offset(bounds)

	# Convert tile offset to pixel offset
	var pixel_offset_x = center_offset_tiles.x * tile_size.x
	var pixel_offset_y = center_offset_tiles.y * tile_size.y
	print("TileMap bounds: ", bounds)
	print("TileMap center offset (tiles): ", center_offset_tiles)
	print("TileMap center offset (pixels): (%d, %d)" % [pixel_offset_x, pixel_offset_y])

	# Get export directory
	var export_dir = export_path.get_base_dir()

	# Build the JSON data structure
	var json_data: Dictionary = {
		"version": 1,
		"name": export_path.get_file().get_basename(),
		"node_offset": [pixel_offset_x, pixel_offset_y],
		"tileset": _extract_tileset(tileset, export_dir, textures_subdir, result),
		"layers": _extract_layers(layers)
	}

	# Write JSON file
	var json_string = JSON.stringify(json_data, "\t")
	var file = FileAccess.open(export_path, FileAccess.WRITE)
	if not file:
		result.error = "Failed to create file: " + export_path
		return result

	file.store_string(json_string)
	file.close()

	result.success = true
	result.layer_count = layers.size()
	return result


# ============================================================================
# TileSet Extraction
# ============================================================================

func _extract_tileset(tileset: TileSet, export_dir: String, textures_subdir: String, result: ExportResult) -> Dictionary:
	var data: Dictionary = {
		"tile_size": [tileset.tile_size.x, tileset.tile_size.y],
		"tile_shape": _get_tile_shape_string(tileset.tile_shape),
		"physics_layers": _extract_physics_layers(tileset),
		"sources": _extract_sources(tileset, export_dir, textures_subdir, result)
	}
	return data


func _get_tile_shape_string(shape: TileSet.TileShape) -> String:
	match shape:
		TileSet.TILE_SHAPE_SQUARE:
			return "square"
		TileSet.TILE_SHAPE_ISOMETRIC:
			return "isometric"
		TileSet.TILE_SHAPE_HALF_OFFSET_SQUARE:
			return "half_offset_square"
		TileSet.TILE_SHAPE_HEXAGON:
			return "hexagon"
		_:
			return "square"


func _extract_physics_layers(tileset: TileSet) -> Array:
	var layers: Array = []
	var physics_count = tileset.get_physics_layers_count()

	for i in physics_count:
		var layer_data: Dictionary = {
			"collision_layer": tileset.get_physics_layer_collision_layer(i),
			"collision_mask": tileset.get_physics_layer_collision_mask(i)
		}
		layers.append(layer_data)

	return layers


func _extract_sources(tileset: TileSet, export_dir: String, textures_subdir: String, result: ExportResult) -> Array:
	var sources: Array = []
	var source_count = tileset.get_source_count()

	for i in source_count:
		var source_id = tileset.get_source_id(i)
		var source = tileset.get_source(source_id)

		# Only support TileSetAtlasSource
		if source is TileSetAtlasSource:
			var source_data = _extract_atlas_source(source as TileSetAtlasSource, source_id, tileset, export_dir, textures_subdir, result)
			sources.append(source_data)
		else:
			push_warning("Skipping non-atlas source with ID: %d" % source_id)

	return sources


func _extract_atlas_source(source: TileSetAtlasSource, source_id: int, tileset: TileSet, export_dir: String, textures_subdir: String, result: ExportResult) -> Dictionary:
	var texture = source.get_texture()
	var texture_filename = ""

	if texture:
		texture_filename = _copy_texture(texture, export_dir, textures_subdir, result)

	var data: Dictionary = {
		"id": source_id,
		"type": "atlas",
		"texture": texture_filename,
		"texture_region_size": [source.texture_region_size.x, source.texture_region_size.y],
		"margins": [source.margins.x, source.margins.y],
		"separation": [source.separation.x, source.separation.y],
		"tiles": _extract_tiles(source, tileset)
	}

	return data


# ============================================================================
# Texture Copy
# ============================================================================

func _copy_texture(texture: Texture2D, export_dir: String, textures_subdir: String, result: ExportResult) -> String:
	var texture_path = texture.resource_path
	if texture_path.is_empty():
		push_warning("Texture has no resource path, skipping copy")
		return ""

	var filename = texture_path.get_file()

	# Create textures subdirectory
	var textures_dir = export_dir.path_join(textures_subdir)
	if not DirAccess.dir_exists_absolute(textures_dir):
		var err = DirAccess.make_dir_recursive_absolute(textures_dir)
		if err != OK:
			push_warning("Failed to create textures directory: %s (error: %d)" % [textures_dir, err])
			return ""

	var dest_path = textures_dir.path_join(filename)

	# Get the global path for the source texture
	var source_global_path = ProjectSettings.globalize_path(texture_path)

	# Check if file already exists at destination
	if FileAccess.file_exists(dest_path):
		# File already copied (maybe from another source using same texture)
		return textures_subdir.path_join(filename)

	# Copy the file
	var err = DirAccess.copy_absolute(source_global_path, dest_path)
	if err != OK:
		push_warning("Failed to copy texture: %s -> %s (error: %d)" % [source_global_path, dest_path, err])
		return textures_subdir.path_join(filename)  # Still return the path even if copy failed

	result.texture_count += 1
	return textures_subdir.path_join(filename)


# ============================================================================
# Tile Extraction (including physics/collision polygons)
# ============================================================================

func _extract_tiles(source: TileSetAtlasSource, tileset: TileSet) -> Array:
	var tiles: Array = []
	var tiles_count = source.get_tiles_count()

	for i in tiles_count:
		var atlas_coords = source.get_tile_id(i)
		var tile_data = source.get_tile_data(atlas_coords, 0)

		if not tile_data:
			continue

		var tile_dict: Dictionary = {
			"atlas_coords": [atlas_coords.x, atlas_coords.y],
			"size_in_atlas": [
				source.get_tile_size_in_atlas(atlas_coords).x,
				source.get_tile_size_in_atlas(atlas_coords).y
			]
		}

		# Extract physics/collision data
		var physics_data = _extract_tile_physics(tile_data, tileset)
		if not physics_data.is_empty():
			tile_dict["physics"] = physics_data

		tiles.append(tile_dict)

	return tiles


func _extract_tile_physics(tile_data: TileData, tileset: TileSet) -> Array:
	var physics_array: Array = []
	var physics_layer_count = tileset.get_physics_layers_count()

	for layer_id in physics_layer_count:
		var poly_count = tile_data.get_collision_polygons_count(layer_id)
		if poly_count == 0:
			continue

		var layer_physics: Dictionary = {
			"layer": layer_id,
			"polygons": []
		}

		for poly_idx in poly_count:
			var points: PackedVector2Array = tile_data.get_collision_polygon_points(layer_id, poly_idx)
			if points.is_empty():
				continue

			# Convert to flat array [x1, y1, x2, y2, ...]
			var flat_array: Array = []
			for point in points:
				flat_array.append(point.x)
				flat_array.append(point.y)

			layer_physics["polygons"].append(flat_array)

		if not layer_physics["polygons"].is_empty():
			physics_array.append(layer_physics)

	return physics_array


# ============================================================================
# TileMap Centering - Calculate bounds and apply offset to center at origin
# ============================================================================

## Calculate the bounding box of all tiles across all layers in world coordinates (tile units)
## Takes into account each layer's global position (transform offset)
func _calculate_tilemap_bounds(layers: Array[TileMapLayer], tile_size: Vector2i) -> Rect2i:
	var min_x: int = 0x7FFFFFFF  # INT32_MAX
	var max_x: int = -0x80000000  # INT32_MIN
	var min_y: int = 0x7FFFFFFF
	var max_y: int = -0x80000000
	var has_tiles: bool = false

	for layer in layers:
		# Convert layer global position (pixels) to tile offset
		var global_pos = layer.position
		var layer_offset_x: int = floori(global_pos.x / tile_size.x)
		var layer_offset_y: int = floori(global_pos.y / tile_size.y)

		var used_cells = layer.get_used_cells()
		for cell in used_cells:
			has_tiles = true
			# Add layer offset to get world tile coordinates
			var world_x = cell.x + layer_offset_x
			var world_y = cell.y + layer_offset_y
			min_x = mini(min_x, world_x)
			max_x = maxi(max_x, world_x)
			min_y = mini(min_y, world_y)
			max_y = maxi(max_y, world_y)

	if not has_tiles:
		return Rect2i(0, 0, 0, 0)

	return Rect2i(min_x, min_y, max_x - min_x + 1, max_y - min_y + 1)


## Calculate the offset needed to center the tilemap at origin (0, 0)
func _calculate_center_offset(bounds: Rect2i) -> Vector2i:
	if bounds.size == Vector2i.ZERO:
		return Vector2i.ZERO

	# Calculate center of the bounds and return negative offset to center at origin
	var center_x: int = bounds.position.x + bounds.size.x / 2
	var center_y: int = bounds.position.y + bounds.size.y / 2
	return Vector2i(-center_x, -center_y)


# ============================================================================
# TileMapLayer Extraction
# ============================================================================

func _extract_layers(layers: Array[TileMapLayer]) -> Array:
	var layers_data: Array = []

	for layer in layers:
		var layer_data = _extract_layer(layer)
		layers_data.append(layer_data)

	return layers_data


func _extract_layer(layer: TileMapLayer) -> Dictionary:
	# Get tile map data as binary array and encode to Base64
	var bytes: PackedByteArray = layer.get_tile_map_data_as_array()
	var base64_data: String = Marshalls.raw_to_base64(bytes)

	var data: Dictionary = {
		"name": layer.name,
		"z_index": layer.z_index,
		"offset": [layer.position.x, layer.position.y],
		"enabled": layer.enabled,
		"tile_map_data": base64_data
	}

	return data
