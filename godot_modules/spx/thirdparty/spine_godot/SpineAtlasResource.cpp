/******************************************************************************
 * Spine Runtimes License Agreement
 * Last updated April 5, 2025. Replaces all prior versions.
 *
 * Copyright (c) 2013-2025, Esoteric Software LLC
 *
 * Integration of the Spine Runtimes into software or otherwise creating
 * derivative works of the Spine Runtimes is permitted under the terms and
 * conditions of Section 2 of the Spine Editor License Agreement:
 * http://esotericsoftware.com/spine-editor-license
 *
 * Otherwise, it is permitted to integrate the Spine Runtimes into software
 * or otherwise create derivative works of the Spine Runtimes (collectively,
 * "Products"), provided that each user of the Products must obtain their own
 * Spine Editor license and redistribution of the Products in any form must
 * include this license and copyright notice.
 *
 * THE SPINE RUNTIMES ARE PROVIDED BY ESOTERIC SOFTWARE LLC "AS IS" AND ANY
 * EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
 * DISCLAIMED. IN NO EVENT SHALL ESOTERIC SOFTWARE LLC BE LIABLE FOR ANY
 * DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES,
 * BUSINESS INTERRUPTION, OR LOSS OF USE, DATA, OR PROFITS) HOWEVER CAUSED AND
 * ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF
 * THE SPINE RUNTIMES, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 *****************************************************************************/

#include "SpineAtlasResource.h"
#include "SpineRendererObject.h"

#ifdef SPINE_GODOT_EXTENSION
#include <godot_cpp/classes/json.hpp>
#include <godot_cpp/classes/texture.hpp>
#include <godot_cpp/classes/file_access.hpp>
#include <godot_cpp/classes/image.hpp>
#include <godot_cpp/classes/image_texture.hpp>
#else
#include "core/io/json.h"
#include "scene/resources/texture.h"
#if VERSION_MAJOR > 3
#include "core/io/image.h"
#include "scene/resources/image_texture.h"
#else
#include "core/image.h"
#endif
#endif

#ifdef TOOLS_ENABLED
#ifdef SPINE_GODOT_EXTENSION
#include <godot_cpp/classes/editor_file_system.hpp>
#else
#if (VERSION_MAJOR >= 4 && VERSION_MINOR >= 5)
#include "editor/file_system/editor_file_system.h"
#else
#include "editor/editor_file_system.h"
#endif
#endif
#endif

#include <spine/TextureLoader.h>
#include "core/io/dir_access.h"

class GodotSpineTextureLoader : public spine::TextureLoader {

	Array *textures;
	Array *normal_maps;
	Array *specular_maps;
	String normal_map_prefix;
	String specular_map_prefix;
	String atlas_directory;
	bool failed = false;

public:
	GodotSpineTextureLoader(Array *_textures, Array *_normal_maps, Array *_specular_maps, const String &normal_map_prefix, const String &specular_map_prefix, bool is_importing, const String &directory) : textures(_textures), normal_maps(_normal_maps), specular_maps(_specular_maps), normal_map_prefix(normal_map_prefix), specular_map_prefix(specular_map_prefix), atlas_directory(directory.simplify_path()) {
		Ref<DirAccess> access = DirAccess::open(directory.is_empty() ? String(".") : directory);
		if (access.is_valid()) atlas_directory = access->get_current_dir();
	}

	bool has_failed() const { return failed; }

	bool is_safe_path(const String &path) {
		if (path.contains("\\")) return false;
		for (const String &part : path.split("/")) {
			if (part == "..") return false;
		}
		const String base = atlas_directory.trim_suffix("/") + "/";
		const String normalized = path.simplify_path();
		if (!normalized.begins_with(base)) return false;
		const String relative = normalized.substr(base.length());
		if (relative.is_empty() || relative.contains(":")) return false;
		String directory = atlas_directory;
		for (const String &part : relative.split("/")) {
			Ref<DirAccess> access = DirAccess::open(directory);
			if (access.is_valid() && access->is_link(part)) return false;
			directory = directory.path_join(part);
		}
		return true;
	}

	static bool fix_path(String &path) {
		const String prefix = "res:/";
		auto i = path.find(prefix);
		if (i == -1) {
			return false;
		}

		auto sub_str_pos = i + SSIZE(prefix) - 1;
		auto res = path.substr(sub_str_pos);
		if (!EMPTY(res)) {
			if (res[0] != '/') {
				path = prefix + String("/") + res;
			} else {
				path = prefix + res;
			}
		}
		return true;
	}

#if VERSION_MAJOR > 3
	Ref<Texture2D> get_texture_from_image(const String &path, bool is_resource) {
		if (!is_safe_path(path)) {
			failed = true;
			return Ref<Texture2D>();
		}
		Error error = OK;
		if (is_resource) {
#ifdef SPINE_GODOT_EXTENSION
			return ResourceLoader::get_singleton()->load(path, "", ResourceLoader::CACHE_MODE_REUSE);
#else
			Ref<Texture2D> texture = ResourceLoader::load(path, "", ResourceFormatLoader::CACHE_MODE_IGNORE, &error);
			if (texture.is_valid()) return texture;
			// SPX project assets may be mounted without Godot import metadata.
			Ref<Image> image = Image::load_from_file(path);
			return image.is_valid() ? Ref<Texture2D>(ImageTexture::create_from_image(image)) : Ref<Texture2D>();
#endif
		} else {
			Ref<Image> img;
			img.instantiate();
			img = img->load_from_file(path);
			return img.is_valid() ? Ref<Texture2D>(ImageTexture::create_from_image(img)) : Ref<Texture2D>();
		}
	}
#else
	Ref<Texture> get_texture_from_image(const String &path, bool is_resource) {
		Error error = OK;
		if (is_resource) {
			return ResourceLoader::load(path, "", false, &error);
		} else {
			Vector<uint8_t> buf = FileAccess::get_file_as_array(path, &error);
			if (error == OK) {
				Ref<Image> img;
				INSTANTIATE(img);
				img->load(path);

				Ref<ImageTexture> texture;
				INSTANTIATE(texture);
				texture->create_from_image(img);
				return texture;
			}
			return Ref<Texture>();
		}
	}
#endif

	void import_image_resource(const String &path) {
#if VERSION_MAJOR > 4
#ifdef TOOLS_ENABLED
		// Required when importing into editor by e.g. drag & drop. The .png files
		// of the atlas might not have been imported yet.
		// See https://github.com/EsotericSoftware/spine-runtimes/issues/2385
		if (is_importing) {
			HashMap<StringName, Variant> custom_options;
			Dictionary generator_parameters;
			EditorFileSystem::get_singleton()->reimport_append(path, custom_options, "", generator_parameters);
		}
#endif
#endif
	}

	void load(spine::AtlasPage &page, const spine::String &path) override {
		// The directory is trusted configuration; validate the atlas-controlled
		// page name before joining it, then open the canonical joined path.
		String page_name = String::utf8(page.name.buffer());
		bool valid_page = !page_name.is_empty() && !page_name.is_absolute_path() &&
				!page_name.contains(":") && !page_name.contains("\\");
		for (const String &part : page_name.split("/")) {
			if (part == "..") valid_page = false;
		}
		String fixed_path = atlas_directory.path_join(page_name);
		if (!valid_page || !is_safe_path(fixed_path)) {
			failed = true;
			page.texture = memnew(SpineRendererObject);
			return;
		}
		bool is_resource = fix_path(fixed_path);

		import_image_resource(fixed_path);

#if VERSION_MAJOR > 3
		Ref<Texture2D> texture = get_texture_from_image(fixed_path, is_resource);
#else
		Ref<Texture> texture = get_texture_from_image(fixed_path, is_resource);
#endif
		if (!texture.is_valid()) {
			failed = true;
			ERR_PRINT(vformat("Can't load texture: \"%s\"", fixed_path));
			auto renderer_object = memnew(SpineRendererObject);
			renderer_object->texture = Ref<Texture>(nullptr);
			renderer_object->normal_map = Ref<Texture>(nullptr);
			renderer_object->specular_map = Ref<Texture>(nullptr);
			page.texture = (void *) renderer_object;
			return;
		}

		textures->append(texture);
		auto renderer_object = memnew(SpineRendererObject);
		renderer_object->texture = texture;
		renderer_object->normal_map = Ref<Texture>(nullptr);
		renderer_object->specular_map = Ref<Texture>(nullptr);

		String normal_map_path = vformat("%s/%s_%s", fixed_path.get_base_dir(), normal_map_prefix, fixed_path.get_file());
		String specular_map_path = vformat("%s/%s_%s", fixed_path.get_base_dir(), specular_map_prefix, fixed_path.get_file());
		is_resource = fix_path(normal_map_path);
		is_resource = fix_path(specular_map_path);
#if SPINE_GODOT_EXTENSION
		if (ResourceLoader::get_singleton()->exists(normal_map_path)) {
			import_image_resource(normal_map_path);
			Ref<Texture> normal_map = get_texture_from_image(normal_map_path, is_resource);
			normal_maps->append(normal_map);
			renderer_object->normal_map = normal_map;
		}

		if (ResourceLoader::get_singleton()->exists(specular_map_path)) {
			import_image_resource(specular_map_path);
			Ref<Texture> specular_map = get_texture_from_image(specular_map_path, is_resource);
			specular_maps->append(specular_map);
			renderer_object->specular_map = specular_map;
		}
#else
		if (ResourceLoader::exists(normal_map_path)) {
			import_image_resource(normal_map_path);
			Ref<Texture> normal_map = get_texture_from_image(normal_map_path, is_resource);
			normal_maps->append(normal_map);
			renderer_object->normal_map = normal_map;
		}

		if (ResourceLoader::exists(specular_map_path)) {
			import_image_resource(specular_map_path);
			Ref<Texture> specular_map = get_texture_from_image(specular_map_path, is_resource);
			specular_maps->append(specular_map);
			renderer_object->specular_map = specular_map;
		}
#endif

#if VERSION_MAJOR > 3
		renderer_object->canvas_texture.instantiate();
		renderer_object->canvas_texture->set_diffuse_texture(renderer_object->texture);
		renderer_object->canvas_texture->set_normal_texture(renderer_object->normal_map);
		renderer_object->canvas_texture->set_specular_texture(renderer_object->specular_map);
#endif

		page.texture = (void *) renderer_object;
		page.width = texture->get_width();
		page.height = texture->get_height();
	}

	void unload(void *data) override {
		auto renderer_object = (SpineRendererObject *) data;
		if (renderer_object->texture.is_valid()) renderer_object->texture.unref();
		if (renderer_object->normal_map.is_valid()) renderer_object->normal_map.unref();
		if (renderer_object->specular_map.is_valid()) renderer_object->specular_map.unref();
#if VERSION_MAJOR > 3
		if (renderer_object->canvas_texture.is_valid()) renderer_object->canvas_texture.unref();
#endif
		memdelete(renderer_object);
	}
};

void SpineAtlasResource::_bind_methods() {
	ClassDB::bind_method(D_METHOD("load_from_atlas_file", "path"), &SpineAtlasResource::load_from_atlas_file);
	ClassDB::bind_method(D_METHOD("get_source_path"), &SpineAtlasResource::get_source_path);
	ClassDB::bind_method(D_METHOD("get_textures"), &SpineAtlasResource::get_textures);
	ClassDB::bind_method(D_METHOD("get_normal_maps"), &SpineAtlasResource::get_normal_maps);
	ClassDB::bind_method(D_METHOD("get_specular_maps"), &SpineAtlasResource::get_specular_maps);

	ADD_PROPERTY(PropertyInfo(Variant::STRING, "source_path"), "", "get_source_path");
	ADD_PROPERTY(PropertyInfo(Variant::ARRAY, "textures"), "", "get_textures");
	ADD_PROPERTY(PropertyInfo(Variant::ARRAY, "normal_maps"), "", "get_normal_maps");
	ADD_PROPERTY(PropertyInfo(Variant::ARRAY, "specular_maps"), "", "get_specular_maps");
}

SpineAtlasResource::SpineAtlasResource() : atlas(nullptr), texture_loader(nullptr), normal_map_prefix("n"), specular_map_prefix("s") {
}

SpineAtlasResource::~SpineAtlasResource() {
	delete atlas;
	delete texture_loader;
}

void SpineAtlasResource::clear() {
	delete atlas;
	atlas = nullptr;
	delete texture_loader;
	texture_loader = nullptr;
	textures.clear();
	normal_maps.clear();
	specular_maps.clear();
}

Array SpineAtlasResource::get_textures() {
	return textures;
}

Array SpineAtlasResource::get_normal_maps() {
	return normal_maps;
}

Array SpineAtlasResource::get_specular_maps() {
	return specular_maps;
}

String SpineAtlasResource::get_source_path() {
	return source_path;
}

Error SpineAtlasResource::load_from_atlas_file(const String &path) {
	return load_from_atlas_file_internal(path, false);
}

Error SpineAtlasResource::load_from_atlas_file_internal(const String &path, bool is_importing) {
	source_path = path;
#ifdef SPINE_GODOT_EXTENSION
	atlas_data = FileAccess::get_file_as_string(path);
	if (SSIZE(atlas_data) == 0) return ERR_FILE_UNRECOGNIZED;
#else
	Error err;
	atlas_data = FileAccess::get_file_as_string(path, &err);
	if (err != OK) return err;
#endif

	clear();
	texture_loader = new GodotSpineTextureLoader(&textures, &normal_maps, &specular_maps, normal_map_prefix, specular_map_prefix, is_importing, source_path.get_base_dir());
	auto atlas_utf8 = atlas_data.utf8();
	auto dir_utf8 = source_path.get_base_dir().utf8();
	atlas = new spine::Atlas(atlas_utf8.ptr(), atlas_utf8.length(), dir_utf8.ptr(), texture_loader);
	if (atlas && !((GodotSpineTextureLoader *) texture_loader)->has_failed()) return OK;

	clear();
	return ERR_FILE_UNRECOGNIZED;
}

Error SpineAtlasResource::load_from_file(const String &path, const String &original_path) {
	Error error;
#ifdef SPINE_GODOT_EXTENSION
	String json_string = FileAccess::get_file_as_string(path);
	if (SSIZE(json_string) == 0) return ERR_FILE_UNRECOGNIZED;
#else
	String json_string = FileAccess::get_file_as_string(path, &error);
	if (error != OK) return error;
#endif

#if VERSION_MAJOR > 3
	JSON *json = memnew(JSON);
	error = json->parse(json_string);
	if (error != OK) {
		memdelete(json);
		return error;
	}
	Variant result = json->get_data();
	memdelete(json);
#else
	String error_string;
	int error_line;
	Variant result;
	error = JSON::parse(json_string, result, error_string, error_line);
	if (error != OK) return error;
#endif

	if (result.get_type() != Variant::DICTIONARY) return ERR_INVALID_DATA;
	Dictionary content = result;
	for (const char *key : { "source_path", "atlas_data", "normal_texture_prefix", "specular_texture_prefix" }) {
		if (!content.has(key) || content[key].get_type() != Variant::STRING) return ERR_INVALID_DATA;
	}
	const String imported_source = content["source_path"];
	// Imported metadata must not redirect texture reads to arbitrary filesystem
	// locations. ResourceLoader supplies the trusted pre-remap atlas path.
	if (!imported_source.begins_with("res://") || imported_source.get_extension().to_lower() != "atlas" || imported_source.contains("\\") ||
			imported_source.substr(6).contains(":") || imported_source != imported_source.simplify_path()) return ERR_INVALID_DATA;
	for (const String &part : imported_source.substr(6).split("/")) {
		if (part == "..") return ERR_INVALID_DATA;
	}
	if (!original_path.is_empty() && imported_source != original_path) return ERR_INVALID_DATA;
	String directory = "res://";
	for (const String &part : imported_source.substr(6).get_base_dir().split("/", false)) {
		Ref<DirAccess> access = DirAccess::open(directory);
		if (access.is_valid() && access->is_link(part)) return ERR_INVALID_DATA;
		directory = directory.path_join(part);
	}
	source_path = imported_source;
	atlas_data = content["atlas_data"];
	normal_map_prefix = content["normal_texture_prefix"];
	specular_map_prefix = content["specular_texture_prefix"];

	clear();
	texture_loader = new GodotSpineTextureLoader(&textures, &normal_maps, &specular_maps, normal_map_prefix, specular_map_prefix, false, source_path.get_base_dir());
	auto utf8 = atlas_data.utf8();
	auto dir_utf8 = source_path.get_base_dir().utf8();
	atlas = new spine::Atlas(utf8.ptr(), utf8.size(), dir_utf8.ptr(), texture_loader);
	if (atlas && !((GodotSpineTextureLoader *) texture_loader)->has_failed()) return OK;

	clear();
	return ERR_FILE_UNRECOGNIZED;
}

Error SpineAtlasResource::save_to_file(const String &path) {
	Error err;
#if VERSION_MAJOR > 3
#if SPINE_GODOT_EXTENSION
	Ref<FileAccess> file = FileAccess::open(path, FileAccess::WRITE);
	if (file.is_null()) return ERR_FILE_UNRECOGNIZED;
#else
	Ref<FileAccess> file = FileAccess::open(path, FileAccess::WRITE, &err);
	if (err != OK) return err;
#endif
#else
	FileAccess *file = FileAccess::open(path, FileAccess::WRITE, &err);
	if (err != OK) {
		if (file) file->close();
		return err;
	}
#endif

	Dictionary content;
	content["source_path"] = source_path;
	content["atlas_data"] = atlas_data;
	content["normal_texture_prefix"] = normal_map_prefix;
	content["specular_texture_prefix"] = specular_map_prefix;
#if VERSION_MAJOR > 3
	JSON *json = memnew(JSON);
	file->store_string(json->stringify(content));
	file->flush();
	memdelete(json);
#else
	file->store_string(JSON::print(content));
	file->close();
#endif
	return OK;
}

#ifndef SPINE_GODOT_EXTENSION
#if VERSION_MAJOR > 3
Error SpineAtlasResource::copy_from(const Ref<Resource> &p_resource) {
	auto error = Resource::copy_from(p_resource);
	if (error != OK) return error;

	const Ref<SpineAtlasResource> &spineAtlas = static_cast<const Ref<SpineAtlasResource> &>(p_resource);
	this->clear();
	this->atlas = spineAtlas->atlas;
	this->texture_loader = spineAtlas->texture_loader;
	spineAtlas->clear_native_data();

	this->source_path = spineAtlas->source_path;
	this->atlas_data = spineAtlas->atlas_data;
	this->normal_map_prefix = spineAtlas->normal_map_prefix;
	this->specular_map_prefix = spineAtlas->specular_map_prefix;
	this->textures = spineAtlas->textures;
	this->normal_maps = spineAtlas->normal_maps;
	this->specular_maps = spineAtlas->specular_maps;
	emit_signal(SNAME("skeleton_file_changed"));

	return OK;
}
#endif
#endif

#ifdef SPINE_GODOT_EXTENSION
Variant SpineAtlasResourceFormatLoader::_load(const String &path, const String &original_path, bool use_sub_threads, int32_t cache_mode) {
#else
#if VERSION_MAJOR > 3
RES SpineAtlasResourceFormatLoader::load(const String &path, const String &original_path, Error *error, bool use_sub_threads, float *progress, CacheMode cache_mode) {
#else
#if VERSION_MINOR > 5
RES SpineAtlasResourceFormatLoader::load(const String &path, const String &original_path, Error *error, bool p_no_subresource_cache) {
#else
RES SpineAtlasResourceFormatLoader::load(const String &path, const String &original_path, Error *error) {
#endif
#endif
#endif
	Ref<SpineAtlasResource> atlas = memnew(SpineAtlasResource);
	const Error load_error = atlas->load_from_file(path, !original_path.is_empty() && original_path != path ? original_path : String());
#ifndef SPINE_GODOT_EXTENSION
	if (error) *error = load_error;
#endif
	if (load_error != OK) return Ref<Resource>();
	return atlas;
}


#ifdef SPINE_GODOT_EXTENSION
PackedStringArray SpineAtlasResourceFormatLoader::_get_recognized_extensions() {
	PackedStringArray extensions;
	extensions.push_back("spatlas");
	return extensions;
}
#else
void SpineAtlasResourceFormatLoader::get_recognized_extensions(List<String> *extensions) const {
	const char atlas_ext[] = "spatlas";
	if (!extensions->find(atlas_ext))
		extensions->push_back(atlas_ext);
}
#endif

#ifdef SPINE_GODOT_EXTENSION
String SpineAtlasResourceFormatLoader::_get_resource_type(const String &path) {
#else
String SpineAtlasResourceFormatLoader::get_resource_type(const String &path) const {
#endif
	return path.ends_with("spatlas") || path.ends_with(".atlas") ? "SpineAtlasResource" : "";
}

#ifdef SPINE_GODOT_EXTENSION
bool SpineAtlasResourceFormatLoader::_handles_type(const StringName &type) {
#else
bool SpineAtlasResourceFormatLoader::handles_type(const String &type) const {
#endif
	return type == StringName("SpineAtlasResource") || ClassDB::is_parent_class(type, "SpineAtlasResource");
}

#ifdef SPINE_GODOT_EXTENSION
Error SpineAtlasResourceFormatSaver::_save(const Ref<Resource> &resource, const String &path, uint32_t flags) {
#else
#if VERSION_MAJOR > 3
Error SpineAtlasResourceFormatSaver::save(const RES &resource, const String &path, uint32_t flags) {
#else
Error SpineAtlasResourceFormatSaver::save(const String &path, const RES &resource, uint32_t flags) {
#endif
#endif
	Ref<SpineAtlasResource> res = resource;
	return res->save_to_file(path);
}

#ifdef SPINE_GODOT_EXTENSION
PackedStringArray SpineAtlasResourceFormatSaver::_get_recognized_extensions(const Ref<Resource> &resource) {
	PackedStringArray extensions;
	if (Object::cast_to<SpineAtlasResource>(*resource)) {
		extensions.push_back("spatlas");
	}
	return extensions;
}
#else
void SpineAtlasResourceFormatSaver::get_recognized_extensions(const RES &resource, List<String> *extensions) const {
	if (Object::cast_to<SpineAtlasResource>(*resource))
		extensions->push_back("spatlas");
}
#endif

#ifdef SPINE_GODOT_EXTENSION
bool SpineAtlasResourceFormatSaver::_recognize(const RES &resource) {
#else
bool SpineAtlasResourceFormatSaver::recognize(const RES &resource) const {
#endif
	return Object::cast_to<SpineAtlasResource>(*resource) != nullptr;
}
