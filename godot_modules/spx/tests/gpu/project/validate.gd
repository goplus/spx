extends SceneTree

func _initialize():
    call_deferred("validate")

func validate():
    print("PEN_GPU_DRIVER ", DisplayServer.get_name(), " / ", RenderingServer.get_current_rendering_method(), " / ", RenderingServer.get_video_adapter_name())
    var runner = ClassDB.instantiate("PenValidation")
    runner.position = Vector2(64, 64)
    root.add_child(runner)
    await process_frame
    var failures = runner.run_tests(OS.get_environment("SPX_PEN_SHADER_PATH"), OS.get_environment("SPX_PEN_OUTPUT_DIR"))
    var background = ColorRect.new()
    background.color = Color.WHITE
    background.size = Vector2(128, 128)
    root.add_child(background)
    root.move_child(background, 0)
    runner.prepare_display()
    await process_frame
    await RenderingServer.frame_post_draw
    var image = root.get_texture().get_image()
    image.save_png(OS.get_environment("SPX_PEN_OUTPUT_DIR").path_join("premultiplied_display.png"))
    var pixel = image.get_pixel(64, 64)
    var display_ok = abs(pixel.r - 1.0) < 0.025 and abs(pixel.g - 0.5) < 0.025 and abs(pixel.b - 0.5) < 0.025
    print("PEN_DISPLAY_RESULT pixel=", pixel, " passed=", display_ok)
    if not display_ok:
        failures += 1
    if failures == 0:
        print("PEN_GPU_DONE")
    runner.queue_free()
    quit(0 if failures == 0 else 1)
