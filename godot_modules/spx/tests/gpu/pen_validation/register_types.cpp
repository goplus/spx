#include "register_types.h"

#include "../../../spx_pen_surface.h"
#include "../../../spx_pixel_query.h"
#include "core/io/file_access.h"
#include "core/object/class_db.h"
#include "core/string/print_string.h"
#include "scene/2d/animated_sprite_2d.h"
#include "scene/main/scene_tree.h"
#include "scene/main/window.h"
#include "scene/resources/image_texture.h"
#include "scene/resources/material.h"
#include "scene/resources/shader.h"
#include "servers/rendering_server.h"

class PenValidation : public Node2D {
	GDCLASS(PenValidation, Node2D);

	String output_dir;
	int failures = 0;
	int checks = 0;
	SpxPenSurface *surface = nullptr;

	void check(bool p_ok, const String &p_message) {
		checks++;
		if (!p_ok) {
			failures++;
			print_error("PEN_GPU_FAIL " + p_message);
		}
	}

	Ref<Image> capture(const String &p_case) {
		SpxPixelQuery::Snapshot snapshot;
		check(surface->capture(Rect2(-64, -64, 128, 128), snapshot), p_case + ": capture");
		check(snapshot.image.is_valid() && !snapshot.image->is_empty(), p_case + ": image");
		if (snapshot.image.is_valid()) {
			snapshot.image->save_png(output_dir.path_join(p_case + ".png"));
		}
		return snapshot.image;
	}

	void pixel(const Ref<Image> &p_image, Vector2i p_world, const Color &p_expected, const String &p_label) {
		if (p_image.is_null() || p_image->is_empty()) {
			return;
		}
		const Color actual = p_image->get_pixel(p_world.x + p_image->get_width() / 2, p_world.y + p_image->get_height() / 2);
		const bool matches = Math::abs(actual.r - p_expected.r) < 0.025 &&
				Math::abs(actual.g - p_expected.g) < 0.025 &&
				Math::abs(actual.b - p_expected.b) < 0.025 &&
				Math::abs(actual.a - p_expected.a) < 0.025;
		check(matches, p_label + ": actual=" + actual.to_html(true) + " expected=" + p_expected.to_html(true));
	}

	AnimatedSprite2D *sprite(const Ref<Image> &p_image, Vector2 p_position = Vector2(), const Ref<Material> &p_material = Ref<Material>()) {
		Ref<SpriteFrames> frames;
		frames.instantiate();
		frames->add_frame("default", ImageTexture::create_from_image(p_image));
		AnimatedSprite2D *result = memnew(AnimatedSprite2D);
		result->set_sprite_frames(frames);
		result->set_material(p_material);
		result->set_position(p_position);
		add_child(result);
		return result;
	}

	AnimatedSprite2D *sprite(const Color &p_color, Vector2 p_position) {
		Ref<Image> image = Image::create_empty(16, 16, false, Image::FORMAT_RGBA8);
		image->fill(Color(1, 1, 1, 1));
		Ref<Shader> shader;
		shader.instantiate();
		shader->set_code("shader_type canvas_item; render_mode unshaded; uniform vec4 tint: source_color = vec4(1.0); void fragment() { COLOR = texture(TEXTURE, UV) * tint; }");
		Ref<ShaderMaterial> material;
		material.instantiate();
		material->set_shader(shader);
		material->set_shader_parameter("tint", p_color);
		return sprite(image, p_position, material);
	}

protected:
	static void _bind_methods() {
		ClassDB::bind_method(D_METHOD("run_tests", "shader_path", "output_dir"), &PenValidation::run_tests);
		ClassDB::bind_method(D_METHOD("prepare_display"), &PenValidation::prepare_display);
	}

public:
	void prepare_display() {
		surface = memnew(SpxPenSurface);
		surface->initialize(Size2i(64, 64));
		add_child(surface);
		surface->draw_line(Vector2(-16, 0), Vector2(16, 0), 12, Color(1, 0, 0, 0.5), true);
		surface->flush();
	}

	int run_tests(const String &p_shader_path, const String &p_output_dir) {
		output_dir = p_output_dir;
		failures = checks = 0;
		surface = memnew(SpxPenSurface);
		surface->initialize(Size2i(64, 64));
		add_child(surface);
		pixel(capture("empty"), Vector2i(), Color(0, 0, 0, 0), "new surface transparent");

		AnimatedSprite2D *red = sprite(Color(1, 0, 0), Vector2(-16, 0));
		AnimatedSprite2D *blue = sprite(Color(0, 0, 1), Vector2(16, 0));
		surface->draw_stamp(red);
		surface->flush();
		Ref<ShaderMaterial> red_material = red->get_material();
		memdelete(red);
		red_material->set_shader_parameter("tint", Color(1, 1, 0));
		surface->draw_line(Vector2(-4, 0), Vector2(4, 0), 8, Color(0, 1, 0), true);
		surface->flush();
		surface->draw_stamp(blue);
		Ref<ShaderMaterial> blue_material = blue->get_material();
		blue_material->set_shader_parameter("tint", Color(1, 1, 0));
		Ref<Image> image = capture("materials_and_flush");
		pixel(image, Vector2i(-16, 0), Color(1, 0, 0), "first stamp retains resources across flushes after its source is deleted");
		pixel(image, Vector2i(0, 0), Color(0, 1, 0), "line stays independent of stamp material");
		pixel(image, Vector2i(16, 0), Color(0, 0, 1), "last stamp retains its material snapshot");
		memdelete(blue);

		surface->clear();
		red = sprite(Color(1, 0, 0), Vector2());
		blue = sprite(Color(0, 0, 1, 0.5), Vector2());
		surface->draw_stamp(red);
		surface->draw_line(Vector2(-8, 0), Vector2(8, 0), 8, Color(0, 1, 0), true);
		surface->draw_stamp(blue);
		memdelete(red);
		memdelete(blue);
		image = capture("ordering_and_blend");
		pixel(image, Vector2i(), Color(0, 0.5, 0.5, 1), "stamp line stamp order and translucent blending");
		pixel(image, Vector2i(0, 6), Color(0.5, 0, 0.5, 1), "transparent stamp blends over first stamp");

		surface->clear();
		surface->flush();
		pixel(capture("clear"), Vector2i(), Color(0, 0, 0, 0), "clear is visible synchronously");
		surface->draw_line(Vector2(-20, 0), Vector2(12, 0), 4, Color(1, 0, 0, 0.5), true);
		surface->flush();
		surface->flush();
		surface->draw_line(Vector2(-12, 0), Vector2(20, 0), 4, Color(0, 0, 1, 0.5), true);
		surface->flush();
		image = capture("repeated_flush");
		pixel(image, Vector2i(-16, 0), Color(0.5, 0, 0, 0.5), "repeated flush retains first unrendered line exactly once");
		pixel(image, Vector2i(16, 0), Color(0, 0, 0.5, 0.5), "repeated flush retains second line exactly once");
		pixel(image, Vector2i(), Color(0.25, 0, 0.5, 0.75), "appended translucent lines blend in submission order");

		surface->clear();
		surface->draw_line(Vector2(-8, 0), Vector2(8, 0), 8, Color(1, 0, 0, 0.5), true);
		image = capture("clear_draw");
		pixel(image, Vector2i(), Color(0.5, 0, 0, 0.5), "clear then draw preserves new pixels and premultiplied alpha");
		pixel(image, Vector2i(16, 0), Color(0, 0, 0, 0), "clear then draw removes old pixels");
		surface->draw_line(Vector2(-8, 16), Vector2(8, 16), 8, Color(0, 0, 1, 0.5), true);
		surface->flush();
		image = capture("next_batch");
		pixel(image, Vector2i(), Color(0.5, 0, 0, 0.5), "new batch preserves rendered pixels without replaying their commands");
		pixel(image, Vector2i(0, 16), Color(0, 0, 0.5, 0.5), "new batch renders after the previous batch completes");
		surface->clear();
		surface->draw_line(Vector2(-8, 0), Vector2(8, 0), 8, Color(1, 1, 1), true);
		surface->flush();
		surface->clear();
		pixel(capture("clear_pending"), Vector2i(), Color(0, 0, 0, 0), "clear discards queued commands");

		surface->draw_line(Vector2(-8, -10), Vector2(8, -10), 4, Color(1, 1, 1), true);
		surface->flush();
		surface->draw_line(Vector2(-8, 10), Vector2(8, 10), 4, Color(1, 1, 1), true);
		surface->set_canvas_size(Size2i(96, 32));
		surface->draw_line(Vector2(-40, 0), Vector2(40, 0), 4, Color(0, 1, 0), true);
		image = capture("resize");
		check(image.is_valid() && image->get_size() == Vector2i(96, 32), "resize updates GPU target dimensions");
		pixel(image, Vector2i(36, 0), Color(0, 1, 0), "resize draws beyond prior canvas bounds");
		pixel(image, Vector2i(36, 10), Color(0, 0, 0, 0), "resize clears untouched pixels");
		pixel(image, Vector2i(0, -10), Color(0, 0, 0, 0), "resize discards submitted commands before rendering");
		pixel(image, Vector2i(0, 10), Color(0, 0, 0, 0), "resize discards pending commands");

		surface->set_canvas_size(Size2i(64, 64));
		Ref<Image> split_image = Image::create_empty(16, 16, false, Image::FORMAT_RGBA8);
		split_image->fill(Color(1, 0, 0));
		split_image->fill_rect(Rect2i(8, 0, 8, 16), Color(0, 0, 1));
		AnimatedSprite2D *split_sprite = sprite(split_image);
		split_sprite->set_offset(Vector2(4, 2));
		split_sprite->set_flip_h(true);
		surface->draw_stamp(split_sprite);
		image = capture("flipped_pivot");
		pixel(image, Vector2i(-2, 2), Color(0, 0, 1), "horizontal flip preserves costume offset");
		pixel(image, Vector2i(10, 2), Color(1, 0, 0), "horizontal flip mirrors texture without moving bounds");
		surface->clear();
		split_sprite->set_flip_h(false);
		split_sprite->set_scale(Vector2(-1, 1));
		split_sprite->set_rotation(Math_PI / 2);
		surface->draw_stamp(split_sprite);
		image = capture("rotated_negative_scale");
		pixel(image, Vector2i(-2, 2), Color(1, 0, 0), "rotated negative scale preserves first texture half");
		pixel(image, Vector2i(-2, -10), Color(0, 0, 1), "rotated negative scale preserves second texture half");
		memdelete(split_sprite);

		surface->clear();
		Ref<Image> effect_image = Image::create_empty(16, 16, false, Image::FORMAT_RGBA8);
		effect_image->fill(Color(0.5, 0, 0));
		Ref<Shader> effect_shader;
		effect_shader.instantiate();
		effect_shader->set_code(FileAccess::get_file_as_string(p_shader_path));
		Ref<ShaderMaterial> effect_material;
		effect_material.instantiate();
		effect_material->set_shader(effect_shader);
		effect_material->set_shader_parameter("color_amount", 1.0 / 3.0);
		effect_material->set_shader_parameter("brightness_amount", 0.2);
		effect_material->set_shader_parameter("alpha_amount", 0.5);
		AnimatedSprite2D *effect_sprite = sprite(effect_image, Vector2(), effect_material);
		surface->draw_stamp(effect_sprite);
		effect_material->set_shader_parameter("alpha_amount", 1.0);
		memdelete(effect_sprite);
		image = capture("spx_effect_shader");
		pixel(image, Vector2i(), Color(0.1, 0.35, 0.1, 0.5), "actual SPX hue brightness and ghost effects are frozen in stamp");
		SpxPixelQuery::Snapshot cached;
		check(surface->capture(Rect2(-64, -64, 128, 128), cached), "cached capture succeeds");
		check(cached.image == image, "unchanged pen reuses readback image");
		SpxPixelQuery::Layer effect_layer;
		effect_layer.pixel_query = cached;
		const Color composed = SpxPixelQuery::composite({ effect_layer }, get_global_position() + Vector2(0.5, 0.5));
		check(Math::abs(composed.r - 0.6) < 0.025 && Math::abs(composed.g - 0.85) < 0.025 && Math::abs(composed.b - 0.6) < 0.025,
				"sensing composites real premultiplied GPU stamp over white without double alpha");

		SubViewport *target = Object::cast_to<SubViewport>(surface->get_node_or_null(NodePath("pen_render_target")));
		check(target != nullptr, "pen render target is available for deferred rendering checks");
		if (target != nullptr) {
			RenderingServer *server = RenderingServer::get_singleton();
			const Rect2 outside(surface->get_global_position() + Vector2(128, 128), Vector2(2, 2));
			SpxPixelQuery::Snapshot skipped = cached;
			surface->clear();
			surface->draw_line(Vector2(-8, 0), Vector2(8, 0), 8, Color(1, 0, 0), true);
			surface->flush();
			check(!surface->capture(outside, skipped), "non-overlapping query skips submitted pen pixels");
			check(skipped.image.is_null(), "skipped capture clears any previous snapshot image");
			check(server->viewport_get_update_mode(target->get_viewport_rid()) == RS::VIEWPORT_UPDATE_ONCE,
					"non-overlapping query leaves submitted commands unrendered");
			image = capture("deferred_readback");
			pixel(image, Vector2i(), Color(1, 0, 0), "overlapping query renders the retained commands");

			surface->clear();
			surface->draw_line(Vector2(-8, 16), Vector2(8, 16), 8, Color(0, 0, 1), true);
			check(!surface->capture(outside, skipped), "non-overlapping query skips pending clear and new commands");
			check(skipped.image.is_null(), "non-overlapping query does not return stale cached pixels after clear");
			check(server->viewport_get_update_mode(target->get_viewport_rid()) == RS::VIEWPORT_UPDATE_DISABLED,
					"non-overlapping query does not flush pending clear and new commands");
			image = capture("deferred_clear_draw");
			pixel(image, Vector2i(), Color(0, 0, 0, 0), "later overlapping query applies the pending clear");
			pixel(image, Vector2i(0, 16), Color(0, 0, 1), "later overlapping query preserves commands queued after clear");

			surface->clear();
			surface->set_position(Vector2(160, 80));
			surface->set_scale(Vector2(2, 0.5));
			surface->draw_line(Vector2(-31, 0), Vector2(31, 0), 8, Color(1, 0, 1), true);
			surface->flush();
			const Vector2 world_center = surface->get_global_position();
			check(!surface->capture(Rect2(world_center + Vector2(65, 0), Vector2(2, 2)), skipped),
					"translated and scaled pen rejects queries beyond its world bounds");
			check(skipped.image.is_null(), "transformed non-overlapping query skips readback");
			check(skipped.bounds == Rect2(world_center - Vector2(64, 16), Vector2(128, 32)),
					"skipped snapshot exposes translated and nonuniformly scaled world bounds");
			check(server->viewport_get_update_mode(target->get_viewport_rid()) == RS::VIEWPORT_UPDATE_ONCE,
					"transformed non-overlapping query leaves rendering pending");
			SpxPixelQuery::Snapshot transformed;
			check(surface->capture(Rect2(world_center + Vector2(60, 0), Vector2(2, 2)), transformed),
					"query in the scaled extension of the pen bounds triggers capture");
			Color transformed_color;
			check(transformed.image.is_valid() && SpxPixelQuery::sample_premultiplied(transformed, world_center + Vector2(60.5, 0.25), transformed_color),
					"transformed snapshot samples retained pixels in world coordinates");
			check(Math::abs(transformed_color.r - 1) < 0.025 && Math::abs(transformed_color.g) < 0.025 &&
							Math::abs(transformed_color.b - 1) < 0.025 && Math::abs(transformed_color.a - 1) < 0.025,
					"translated and scaled readback preserves the queued line color");
		}

		memdelete(surface);
		surface = nullptr;
		print_line(vformat("PEN_GPU_RESULT checks=%d failures=%d", checks, failures));
		return failures;
	}
};

void initialize_pen_validation_module(ModuleInitializationLevel p_level) {
	if (p_level == MODULE_INITIALIZATION_LEVEL_SCENE) {
		ClassDB::register_class<PenValidation>();
	}
}

void uninitialize_pen_validation_module(ModuleInitializationLevel p_level) {
}
