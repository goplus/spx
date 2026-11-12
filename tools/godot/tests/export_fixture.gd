extends SceneTree
## A self-contained project fixture; no resource import or script cache needed.

func _init() -> void:
	if "--verify-preview" in OS.get_cmdline_user_args():
		verify_preview()
		return
	var image = Image.create_empty(16, 16, false, Image.FORMAT_RGBA8)
	image.fill(Color.RED)
	ResourceSaver.save(ImageTexture.create_from_image(image), "res://tile.tres")
	image.fill(Color.BLUE)
	ResourceSaver.save(ImageTexture.create_from_image(image), "res://decorator.tres")
	var scene = Node.new()
	scene.name = "Level"
	var container = Node2D.new()
	container.position = Vector2(-12, -20)
	scene.add_child(container)
	container.owner = scene
	var layer = TileMapLayer.new()
	layer.tile_set = TileSet.new()
	layer.tile_set.tile_size = Vector2i(16, 16)
	var atlas = TileSetAtlasSource.new()
	atlas.texture = load("res://tile.tres")
	atlas.create_tile(Vector2i.ZERO)
	layer.tile_set.add_source(atlas, 0)
	layer.set_cell(Vector2i.ZERO, 0, Vector2i.ZERO)
	layer.set_cell(Vector2i(2, 0), 0, Vector2i.ZERO)
	container.add_child(layer)
	layer.owner = scene
	var decoration = Sprite2D.new()
	decoration.name = "Decoration"
	decoration.texture = load("res://decorator.tres")
	decoration.position = Vector2(8, 8)
	container.add_child(decoration)
	decoration.owner = scene
	var packed = PackedScene.new()
	packed.pack(scene)
	DirAccess.make_dir_recursive_absolute("res://levels")
	ResourceSaver.save(packed, "res://levels/terrain.tscn")
	scene.free()
	quit()


func verify_preview() -> void:
	var image = Image.load_from_file("res://_export/terrain/preview.png")
	if not image or image.get_size() != Vector2i(48, 16):
		printerr("ERROR: Unexpected preview size")
		quit(1)
		return
	if image.get_pixel(8, 8) != Color.RED or image.get_pixel(40, 8) != Color.RED:
		printerr("ERROR: Preview tiles are missing or a decorator was rendered")
		quit(1)
		return
	if image.get_pixel(24, 8).a != 0.0:
		printerr("ERROR: Preview background must be transparent")
		quit(1)
		return
	print("Preview pixels passed")
	quit()
