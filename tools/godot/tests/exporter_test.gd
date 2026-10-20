extends SceneTree

const TileExtractor = preload("res://addons/spx_tilemap_exporter/tilemap_extractor.gd")
const Decorations = preload("res://addons/spx_tilemap_exporter/decorator_extractor.gd")
var failures: Array[String] = []

func check(condition: bool, message: String) -> void:
	if not condition:
		failures.append(message)
		printerr(message)

func _init() -> void:
	call_deferred("run")

func run() -> void:
	test_rejected_inputs()
	test_tilemap_round_trip()
	test_decorator_coordinates()
	if failures.is_empty():
		print("Exporter contracts passed")
	quit(0 if failures.is_empty() else 1)

func test_rejected_inputs() -> void:
	var extractor = TileExtractor.new()
	var layers: Array[TileMapLayer] = []
	var path = "res://rejected.json"
	var result = extractor.export_tilemap(layers, path)
	check(not result.success and result.error == "No TileMapLayer nodes provided", "empty layers must fail")
	var layer = TileMapLayer.new()
	layers.append(layer)
	result = extractor.export_tilemap(layers, path)
	check(not result.success and result.error == "No TileSet found in any TileMapLayer", "missing tileset must fail")
	check(extractor._calculate_tilemap_bounds(layers, Vector2i(16, 32)) == Rect2i(), "empty cells have empty bounds")
	check(extractor._calculate_center_offset(Rect2i()) == Vector2i.ZERO, "empty bounds have no offset")
	layer.free()
	var decorations = Decorations.new()
	var decoration_result = decorations.export_decorators(null, path)
	check(not decoration_result.success and decoration_result.error == "Scene root is null", "null scene must fail")
	var scene = Node2D.new()
	decoration_result = decorations.export_decorators(scene, path)
	check(not decoration_result.success and decoration_result.error == "No decorators found in scene", "empty scene must fail")
	check(not FileAccess.file_exists(path), "invalid input must not create output")
	scene.free()

func test_tilemap_round_trip() -> void:
	var extractor = TileExtractor.new()
	var tileset = TileSet.new()
	tileset.tile_size = Vector2i(16, 32)
	tileset.tile_shape = TileSet.TILE_SHAPE_ISOMETRIC
	tileset.add_physics_layer()
	tileset.set_physics_layer_collision_layer(0, 5)
	tileset.set_physics_layer_collision_mask(0, 10)
	var atlas = TileSetAtlasSource.new()
	atlas.texture = fixture_texture()
	atlas.create_tile(Vector2i.ZERO)
	tileset.add_source(atlas, 7)
	var layer = TileMapLayer.new()
	layer.name = "Negative terrain"
	layer.tile_set = tileset
	layer.position = Vector2(-0.5, 31.5)
	layer.z_index = -3
	layer.enabled = false
	layer.set_cell(Vector2i(-2, -1), 7, Vector2i.ZERO)
	layer.set_cell(Vector2i(1, 2), 7, Vector2i.ZERO)
	var layers: Array[TileMapLayer] = [layer]
	# Negative sub-tile positions must floor to -1, not truncate to zero.
	check(extractor._calculate_tilemap_bounds(layers, tileset.tile_size) == Rect2i(-3, -1, 4, 4), "signed bounds and inclusive extents")
	check(extractor._calculate_center_offset(Rect2i(-3, -1, 4, 4)) == Vector2i(1, -1), "even-sized signed center")
	check(extractor._calculate_center_offset(Rect2i(-3, -2, 3, 3)) == Vector2i(2, 1), "odd-sized signed center")
	var result = extractor.export_tilemap(layers, "res://terrain.json")
	check(result.success and result.layer_count == 1, "tilemap export succeeds")
	var data = JSON.parse_string(FileAccess.get_file_as_string("res://terrain.json"))
	check(data.name == "terrain" and data.version == 1, "tilemap identity")
	check(data.node_offset == [16.0, -32.0], "pixel offset uses each tile dimension")
	check(data.tileset.tile_size == [16.0, 32.0] and data.tileset.tile_shape == "isometric", "tileset metadata")
	check(data.tileset.sources[0].id == 7 and data.tileset.sources[0].texture == "textures/fixture_texture.tres", "atlas ID and relative texture path preserved")
	check(result.texture_count == 1 and FileAccess.get_file_as_bytes("res://textures/fixture_texture.tres") == FileAccess.get_file_as_bytes("res://fixture_texture.tres"), "texture copied byte for byte")
	check(data.tileset.physics_layers == [{"collision_layer": 5.0, "collision_mask": 10.0}], "physics masks preserved")
	var exported = data.layers[0]
	check(exported.name == "Negative terrain" and exported.z_index == -3 and not exported.enabled, "layer metadata preserved")
	check(exported.offset == [-0.5, 31.5], "fractional layer offset preserved")
	var restored = TileMapLayer.new()
	restored.tile_set = tileset
	restored.set_tile_map_data_from_array(Marshalls.base64_to_raw(exported.tile_map_data))
	check(restored.get_used_cells().size() == 2, "serialized cells can be decoded by Godot")
	for cell in [Vector2i(-2, -1), Vector2i(1, 2)]:
		check(restored.get_cell_source_id(cell) == 7 and restored.get_cell_atlas_coords(cell) == Vector2i.ZERO, "cell coordinates and source survive round trip")
	check(restored.get_cell_source_id(Vector2i.ZERO) == -1, "empty cell remains empty")
	restored.free()
	layer.free()

func test_decorator_coordinates() -> void:
	var scene = Node2D.new()
	root.add_child(scene)
	scene.position = Vector2(10, 20)
	var sprite = Sprite2D.new()
	sprite.name = "Marker"
	sprite.texture = fixture_texture()
	sprite.position = Vector2(-3, 4)
	sprite.rotation_degrees = -90
	sprite.scale = Vector2(-2, 3)
	sprite.z_index = -2
	scene.add_child(sprite)
	var ignored = Sprite2D.new()
	ignored.name = "_ignore_marker"
	ignored.texture = sprite.texture
	scene.add_child(ignored)
	var extractor = Decorations.new()
	var result = extractor.export_decorators(scene, "res://decorators.json", Vector2(5, -7))
	check(result.success and result.decorator_count == 1, "ignored decorators are excluded")
	var data = JSON.parse_string(FileAccess.get_file_as_string("res://decorators.json"))
	var item = data.decorators[0]
	check(item.name == "Marker" and item.position == {"x": 12.0, "y": -17.0}, "global position plus offset, then Y inversion")
	check(is_zero_approx(item.rotation), "rotation converts to degrees plus 90")
	check(item.scale == {"x": -2.0, "y": 3.0} and item.z_index == -2, "signed scale and draw order survive export")
	check(not item.has("collider_type"), "absent collider is omitted")
	scene.free()

func fixture_texture() -> Texture2D:
	var path = "res://fixture_texture.tres"
	if not FileAccess.file_exists(path):
		check(ResourceSaver.save(GradientTexture2D.new(), path) == OK, "save fixture texture")
	return load(path)
