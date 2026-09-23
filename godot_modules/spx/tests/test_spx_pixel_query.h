/**************************************************************************/
/*  test_spx_pixel_query.h                                                    */
/**************************************************************************/
/*                         This file is part of:                          */
/*                             GODOT ENGINE                               */
/*                        https://godotengine.org                         */
/**************************************************************************/
/* Copyright (c) 2014-present Godot Engine contributors (see AUTHORS.md). */
/* Copyright (c) 2007-2014 Juan Linietsky, Ariel Manzur.                  */
/*                                                                        */
/* Permission is hereby granted, free of charge, to any person obtaining  */
/* a copy of this software and associated documentation files (the        */
/* "Software"), to deal in the Software without restriction, including    */
/* without limitation the rights to use, copy, modify, merge, publish,    */
/* distribute, sublicense, and/or sell copies of the Software, and to     */
/* permit persons to whom the Software is furnished to do so, subject to  */
/* the following conditions:                                              */
/*                                                                        */
/* The above copyright notice and this permission notice shall be         */
/* included in all copies or substantial portions of the Software.        */
/*                                                                        */
/* THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,        */
/* EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF     */
/* MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. */
/* IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY   */
/* CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,   */
/* TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE      */
/* SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.                 */
/**************************************************************************/

#ifndef TEST_SPX_PIXEL_QUERY_H
#define TEST_SPX_PIXEL_QUERY_H

#include "../spx_image_texture.h"
#include "../spx_pixel_query.h"
#include "scene/resources/atlas_texture.h"
#include "scene/resources/image_texture.h"
#include "tests/test_macros.h"

#include <algorithm>

namespace TestSpxPixelQuery {

TEST_CASE("[SPX] Pixel sampling keeps world-center origin at larger steps") {
	const Rect2i centers = SpxPixelQuery::pixel_centers(Rect2(-0.6, -0.6, 3.2, 3.2));
	Vector<Vector2> visited;
	CHECK_FALSE(SpxPixelQuery::any_pixel_center(centers, 2, [&](Vector2 point) {
		visited.push_back(point);
		return false;
	}));
	REQUIRE(visited.size() == 4);
	CHECK(visited[0] == Vector2(-0.5, -0.5));
	CHECK(visited[1] == Vector2(-0.5, 1.5));
	CHECK(visited[2] == Vector2(1.5, -0.5));
	CHECK(visited[3] == Vector2(1.5, 1.5));
}

TEST_CASE("[SPX] Pixel sampling applies flips after inverse transforms") {
	SpxPixelQuery::Snapshot snapshot;
	snapshot.image = Image::create_empty(2, 1, false, Image::FORMAT_RGBA8);
	snapshot.image->set_pixel(0, 0, Color(1, 0, 0, 1));
	snapshot.image->set_pixel(1, 0, Color(0, 1, 0, 1));
	snapshot.image_size = Vector2i(2, 1);
	snapshot.inverse_transform = Transform2D(0, Vector2(-10, 0));
	Color color;
	CHECK(SpxPixelQuery::sample(snapshot, Vector2(10.5, 0.5), color));
	CHECK(color == Color(1, 0, 0, 1));
	snapshot.flip_h = true;
	snapshot.collision_alpha_scale = 0.5;
	CHECK(SpxPixelQuery::sample_premultiplied(snapshot, Vector2(10.5, 0.5), color));
	CHECK(color == Color(0, 0.5, 0, 0.5));
	CHECK_FALSE(SpxPixelQuery::sample(snapshot, Vector2(12.5, 0.5), color));
}

static SpxPixelQuery::Layer solid_layer(const Color &p_color, SpxPixelQuery::Layer::Order p_order) {
	SpxPixelQuery::Layer layer;
	layer.order = p_order;
	layer.pixel_query.image = Image::create_empty(1, 1, false, Image::FORMAT_RGBAF);
	layer.pixel_query.image->fill(p_color);
	layer.pixel_query.image_size = Vector2i(1, 1);
	layer.pixel_query.bounds = Rect2(0, 0, 1, 1);
	layer.pixel_query.local_rect = layer.pixel_query.bounds;
	return layer;
}

TEST_CASE("[SceneTree][SPX] Pixel image cache tracks reloads and atlas changes") {
	Ref<Image> red = Image::create_empty(1, 1, false, Image::FORMAT_RGBA8);
	red->fill(Color(1, 0, 0));
	Ref<Image> blue = Image::create_empty(1, 1, false, Image::FORMAT_RGBA8);
	blue->fill(Color(0, 0, 1));
	Ref<ImageTexture> texture = SpxImageTexture::create_from_image(red);

	SpxPixelQuery::Snapshot first;
	first.texture = texture;
	SpxPixelQuery::Snapshot shared;
	shared.texture = texture;
	SpxPixelQuery::ImageCache cache;
	REQUIRE(SpxPixelQuery::load_image(first, &cache));
	REQUIRE(SpxPixelQuery::load_image(shared, &cache));
	CHECK(cache.size() == 1);
	CHECK(first.image == shared.image);
	CHECK(first.image->get_pixel(0, 0).r == 1.0f);

	SpxImageTexture::replace_image(texture, blue);
	SpxPixelQuery::Snapshot reloaded;
	reloaded.texture = texture;
	REQUIRE(SpxPixelQuery::load_image(reloaded, &cache));
	CHECK(cache.size() == 1);
	CHECK(reloaded.image != first.image);
	CHECK(reloaded.image->get_pixel(0, 0).b == 1.0f);
	CHECK(first.image->get_pixel(0, 0).r == 1.0f);

	Ref<AtlasTexture> atlas;
	atlas.instantiate();
	atlas->set_atlas(texture);
	atlas->set_region(Rect2(0, 0, 1, 1));
	SpxPixelQuery::Snapshot atlas_query;
	atlas_query.texture = atlas;
	REQUIRE(SpxPixelQuery::load_image(atlas_query, &cache));
	CHECK(cache.size() == 1);
	CHECK(atlas_query.image->get_pixel(0, 0).b == 1.0f);
	Ref<Image> strip = Image::create_empty(2, 1, false, Image::FORMAT_RGBA8);
	strip->fill(Color(0, 0, 1));
	strip->set_pixel(1, 0, Color(0, 1, 0));
	SpxImageTexture::replace_image(texture, strip);
	atlas->set_region(Rect2(1, 0, 1, 1));
	SpxPixelQuery::Snapshot moved_region;
	moved_region.texture = atlas;
	REQUIRE(SpxPixelQuery::load_image(moved_region, &cache));
	CHECK(cache.size() == 1);
	CHECK(moved_region.image->get_pixel(0, 0).g == 1.0f);
	const int refs = texture->get_reference_count();
	cache.clear();
	CHECK(texture->get_reference_count() == refs - 1);
}

TEST_CASE("[SceneTree][SPX] Pixel image cache bypasses unversioned textures") {
	class CpuTexture : public ImageTexture {
	public:
		Ref<Image> image;
		Ref<Image> get_image() const override { return image; }
	};
	Ref<CpuTexture> texture;
	texture.instantiate();
	texture->image = Image::create_empty(1, 1, false, Image::FORMAT_RGBA8);
	texture->image->fill(Color(1, 0, 0));
	SpxPixelQuery::ImageCache cache;
	SpxPixelQuery::Snapshot first;
	first.texture = texture;
	REQUIRE(SpxPixelQuery::load_image(first, &cache));
	CHECK(cache.empty());
	texture->image = Image::create_empty(1, 1, false, Image::FORMAT_RGBA8);
	texture->image->fill(Color(0, 0, 1));
	SpxPixelQuery::Snapshot second;
	second.texture = texture;
	REQUIRE(SpxPixelQuery::load_image(second, &cache));
	CHECK(cache.empty());
	CHECK(second.image->get_pixel(0, 0).b == 1.0f);
	CHECK(first.image->get_pixel(0, 0).r == 1.0f);
}

TEST_CASE("[SPX] Scene colors blend premultiplied pen pixels between sprites and backdrop") {
	using SpxPixelQuery::Layer;
	Layer backdrop = solid_layer(Color(0, 1, 0, 1), Layer::BACKDROP);
	backdrop.tree_index = 100;
	Layer pen = solid_layer(Color(0.5, 0, 0, 0.5), Layer::PEN);
	pen.pixel_query.image_premultiplied = true;
	Layer sprite = solid_layer(Color(0, 0, 1, 1), Layer::SPRITE);
	std::vector<Layer> layers{backdrop, pen, sprite};
	auto sort_layers = [&]() {
		std::sort(layers.begin(), layers.end(), [](const Layer &a, const Layer &b) {
			return a.in_front_of(b);
		});
	};
	sort_layers();
	CHECK(layers[0].order == Layer::SPRITE);
	CHECK(layers[1].order == Layer::PEN);
	CHECK(layers[2].order == Layer::BACKDROP);
	CHECK(SpxPixelQuery::composite(layers, Vector2(0.5, 0.5)) == Color(0, 0, 1, 1));

	layers[0].pixel_query.collision_alpha_scale = 0.25;
	CHECK(SpxPixelQuery::composite(layers, Vector2(0.5, 0.5)) == Color(0.375, 0.375, 0.25, 1));
	layers[0].z_index = -1;
	sort_layers();
	CHECK(SpxPixelQuery::composite(layers, Vector2(0.5, 0.5)) == Color(0.5, 0.5, 0, 1));
}

TEST_CASE("[SPX] Scene colors keep transparent and tied layer ordering") {
	using SpxPixelQuery::Layer;
	Layer transparent = solid_layer(Color(1, 0, 0, 0), Layer::SPRITE);
	transparent.z_index = 2;
	Layer half = solid_layer(Color(0, 0, 1, 0.5), Layer::SPRITE);
	half.z_index = 1;
	Layer backdrop = solid_layer(Color(0, 1, 0, 1), Layer::BACKDROP);
	std::vector<Layer> layers{ transparent, half, backdrop };
	CHECK(SpxPixelQuery::composite(layers, Vector2(0.5, 0.5)) == Color(0, 0.5, 0.5, 1));

	transparent.pixel_query.image->fill(Color(1, 0, 0, 1));
	CHECK(SpxPixelQuery::composite(layers, Vector2(0.5, 0.5)) == Color(1, 0, 0, 1));

	layers[0].pixel_query.image->fill(Color(0, 0, 0, 0));
	layers[1].pixel_query.image->fill(Color(0, 0, 0, 0));
	layers[2].pixel_query.image->fill(Color(0, 0, 0, 0));
	CHECK(SpxPixelQuery::composite(layers, Vector2(0.5, 0.5)) == Color(1, 1, 1, 0));

	Layer first = solid_layer(Color(1, 0, 0, 1), Layer::SPRITE);
	Layer last = solid_layer(Color(0, 0, 1, 1), Layer::SPRITE);
	first.tree_index = 1;
	last.tree_index = 2;
	layers = { first, last };
	std::sort(layers.begin(), layers.end(), [](const Layer &a, const Layer &b) {
		return a.in_front_of(b);
	});
	CHECK(SpxPixelQuery::composite(layers, Vector2(0.5, 0.5)) == Color(0, 0, 1, 1));
}

TEST_CASE("[SPX] Premultiplied snapshots apply opacity once and retain world transforms") {
	SpxPixelQuery::Snapshot snapshot = solid_layer(Color(0.5, 0, 0, 0.5), SpxPixelQuery::Layer::PEN).pixel_query;
	snapshot.image_premultiplied = true;
	snapshot.inverse_transform = Transform2D(0, Vector2(-10, -20));
	snapshot.collision_alpha_scale = 0.5;
	Color color;
	CHECK(SpxPixelQuery::sample_premultiplied(snapshot, Vector2(10.5, 20.5), color));
	CHECK(color == Color(0.25, 0, 0, 0.25));
	CHECK_FALSE(SpxPixelQuery::sample_premultiplied(snapshot, Vector2(0.5, 0.5), color));
}

} // namespace TestSpxPixelQuery

#endif // TEST_SPX_PIXEL_QUERY_H
