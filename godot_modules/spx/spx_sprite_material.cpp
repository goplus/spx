/**************************************************************************/
/*  spx_sprite_material.cpp                                               */
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
/* MERCHANTABILITY AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR  */
/* COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, */
/* WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT */
/* OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN  */
/* THE SOFTWARE.                                                          */
/**************************************************************************/

#include "spx_sprite.h"

#include "core/io/resource_loader.h"
#include "scene/2d/animated_sprite_2d.h"
#include "scene/resources/material.h"
#include "scene/resources/shader.h"

#include "spx_abi.h"

void SpxSprite::set_material_shader(GdString p_path) {
	Ref<Shader> shader = ResourceLoader::load(SpxStr(p_path));
	if (shader.is_null()) {
		print_line(String("load spx_sprite_shader failed: ") + SpxStr(p_path));
		return;
	}

	Ref<ShaderMaterial> material = _get_shader_material(__func__, true);
	if (material.is_null()) {
		return;
	}

	material->set_shader(shader);
	anim2d->set_texture_repeat(TEXTURE_REPEAT_ENABLED);
}

GdString SpxSprite::get_material_shader() {
	if (anim2d == nullptr) {
		return nullptr;
	}

	Ref<ShaderMaterial> material = anim2d->get_material();
	if (material.is_null() || material->get_shader().is_null()) {
		return nullptr;
	}

	return SpxReturnStr(material->get_shader()->get_path());
}

void SpxSprite::set_color(GdColor p_color) {
	if (anim2d == nullptr) {
		return;
	}

	Ref<ShaderMaterial> material = anim2d->get_material();
	if (material.is_valid() && material->get_shader().is_valid()) {
		material->set_shader_parameter("color", p_color);
		return;
	}

	anim2d->set_self_modulate(p_color);
}

GdColor SpxSprite::get_color() {
	if (anim2d == nullptr) {
		return GdColor();
	}

	Ref<ShaderMaterial> material = anim2d->get_material();
	if (material.is_valid() && material->get_shader().is_valid()) {
		return material->get_shader_parameter("color");
	}

	return anim2d->get_self_modulate();
}

void SpxSprite::set_material_params(GdString p_effect, GdFloat p_amount) {
	Ref<ShaderMaterial> material = _get_shader_material(__func__);
	if (material.is_null()) {
		return;
	}

	material->set_shader_parameter(SpxStr(p_effect), p_amount);
}

GdFloat SpxSprite::get_material_params(GdString p_effect) {
	Ref<ShaderMaterial> material = _get_shader_material(__func__);
	if (material.is_null()) {
		return 0;
	}

	return material->get_shader_parameter(SpxStr(p_effect));
}

void SpxSprite::set_material_params_vec4(GdString p_effect, GdVec4 p_vec4) {
	Ref<ShaderMaterial> material = _get_shader_material(__func__);
	if (material.is_null()) {
		return;
	}

	material->set_shader_parameter(SpxStr(p_effect), p_vec4);
}

void SpxSprite::set_uv_remap(const GdVec4 &p_uv_remap) {
	Ref<ShaderMaterial> material = _get_shader_material(__func__);
	if (material.is_null()) {
		return;
	}

	material->set_shader_parameter(SNAME("uv_remap"), p_uv_remap);
}

GdVec4 SpxSprite::get_material_params_vec4(GdString p_effect) {
	Ref<ShaderMaterial> material = _get_shader_material(__func__);
	if (material.is_null()) {
		return GdVec4();
	}

	return material->get_shader_parameter(SpxStr(p_effect));
}

void SpxSprite::set_material_params_color(GdString p_effect, GdColor p_color) {
	Ref<ShaderMaterial> material = _get_shader_material(__func__);
	if (material.is_null()) {
		return;
	}

	material->set_shader_parameter(SpxStr(p_effect), p_color);
}

GdColor SpxSprite::get_material_params_color(GdString p_effect) {
	Ref<ShaderMaterial> material = _get_shader_material(__func__);
	if (material.is_null()) {
		return GdColor();
	}

	return material->get_shader_parameter(SpxStr(p_effect));
}

Ref<ShaderMaterial> SpxSprite::_get_shader_material(const char *p_context, bool p_create_if_missing) {
	if (anim2d == nullptr) {
		print_line(String(p_context) + " failed, AnimatedSprite2D is missing.");
		return Ref<ShaderMaterial>();
	}

	Ref<ShaderMaterial> material = anim2d->get_material();
	if (material.is_null() && p_create_if_missing) {
		material.instantiate();
		anim2d->set_material(material);
	}

	if (material.is_null()) {
		print_line(String(p_context) + " failed, the material and shader have not been set, please initialize the shader first!");
		return Ref<ShaderMaterial>();
	}

	return material;
}
