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

#include "SpineSkeletonFileResource.h"
#ifdef SPINE_GODOT_EXTENSION
#include <godot_cpp/core/error_macros.hpp>
#include <godot_cpp/classes/file_access.hpp>
#else
#if VERSION_MAJOR > 3
#include "core/error/error_list.h"
#include "core/error/error_macros.h"
#include "core/io/file_access.h"
#else
#include "core/error_list.h"
#include "core/error_macros.h"
#include "core/os/file_access.h"
#endif
#endif
#include <spine/Json.h>
#include <spine/Version.h>
#include <spine/Extension.h>
#include <cstdint>
#include <cstring>

void SpineSkeletonFileResource::_bind_methods() {
	ClassDB::bind_method(D_METHOD("load_from_file", "path"), &SpineSkeletonFileResource::load_from_file);
}

static bool checkVersion(const char *version, size_t length) {
	const size_t prefix_length = sizeof(SPINE_VERSION_STRING) - 1;
	if (!version || length < prefix_length || std::memcmp(version, SPINE_VERSION_STRING, prefix_length) != 0) return false;
	// Match this major/minor version, not another version such as 4.20.
	return length == prefix_length || (version[prefix_length] == '.' && length > prefix_length + 1);
}

static bool checkJson(const char *jsonData) {
	spine::Json json(jsonData);
	spine::Json *skeleton = spine::Json::getItem(&json, "skeleton");
	if (!skeleton) return false;
	const char *version = spine::Json::getString(skeleton, "spine", 0);
	if (!version) return false;

	return checkVersion(version, std::strlen(version));
}

static bool checkBinary(const char *binaryData, size_t length) {
	// The header contains an eight-byte hash and a varint string length.
	// Validate the length before reading or allocating anything from the file.
	if (!binaryData || length < 9) return false;
	const unsigned char *cursor = reinterpret_cast<const unsigned char *>(binaryData) + 8;
	const unsigned char *end = reinterpret_cast<const unsigned char *>(binaryData) + length;
	uint32_t string_length = 0;
	for (unsigned int index = 0; index < 5; ++index) {
		if (cursor == end) return false;
		const unsigned char byte = *cursor++;
		if (index == 4 && (byte & 0xF0) != 0) return false;
		string_length |= uint32_t(byte & 0x7F) << (index * 7);
		if ((byte & 0x80) == 0) {
			// Spine encodes string bytes plus one; zero and one are null/empty.
			if (string_length <= 1 || size_t(string_length - 1) > size_t(end - cursor)) return false;
			const size_t version_length = string_length - 1;
			if (std::memchr(cursor, '\0', version_length) != nullptr) return false;
			return checkVersion(reinterpret_cast<const char *>(cursor), version_length);
		}
	}
	return false;
}

Error SpineSkeletonFileResource::load_from_file(const String &path) {
	Error error = OK;
	if (path.ends_with(".spjson") || path.ends_with(".spine-json") || path.ends_with(".json")) {
		String loaded_json;
#ifdef SPINE_GODOT_EXTENSION
		loaded_json = FileAccess::get_file_as_string(path);
		if (SSIZE(loaded_json) == 0) return ERR_INVALID_DATA;
#else
		loaded_json = FileAccess::get_file_as_string(path, &error);
		if (error != OK) return error;
#endif
		if (!checkJson(loaded_json.utf8().ptr())) return ERR_INVALID_DATA;
		json = loaded_json;
		binary.resize(0);
	} else {
#ifdef SPINE_GODOT_EXTENSION
		PackedByteArray loaded_binary = FileAccess::get_file_as_bytes(path);
		if (loaded_binary.size() == 0) return ERR_INVALID_DATA;
#else
#if VERSION_MAJOR > 3
		Vector<uint8_t> loaded_binary = FileAccess::get_file_as_bytes(path, &error);
#else
		Vector<uint8_t> loaded_binary = FileAccess::get_file_as_array(path, &error);
#endif
		if (error != OK) return error;
#endif
		if (!checkBinary((const char *) loaded_binary.ptr(), loaded_binary.size())) return ERR_INVALID_DATA;
		binary = loaded_binary;
		json = String();
	}
	return error;
}

Error SpineSkeletonFileResource::save_to_file(const String &path) {
	Error error;
#ifdef SPINE_GODOT_EXTENSION
	Ref<FileAccess> file = FileAccess::open(path, FileAccess::WRITE);
	if (!file.is_valid()) return ERR_FILE_CANT_OPEN;
#else
#if VERSION_MAJOR > 3
	Ref<FileAccess> file = FileAccess::open(path, FileAccess::WRITE, &error);
	if (error != OK) return error;
#else
	FileAccess *file = FileAccess::open(path, FileAccess::WRITE, &error);
	if (error != OK) {
		if (file) file->close();
		return error;
	}
#endif
#endif
	if (!is_binary())
		file->store_string(json);
	else
		file->store_buffer(binary.ptr(), binary.size());
#if VERSION_MAJOR > 3
	file->flush();
#else
	file->close();
#endif
	return OK;
}

#ifndef SPINE_GODOT_EXTENSION
#if VERSION_MAJOR > 3
Error SpineSkeletonFileResource::copy_from(const Ref<Resource> &p_resource) {
	auto error = Resource::copy_from(p_resource);
	if (error != OK) return error;
	const Ref<SpineSkeletonFileResource> &spineFile = static_cast<const Ref<SpineSkeletonFileResource> &>(p_resource);
	this->json = spineFile->json;
	this->binary = spineFile->binary;
	emit_signal(SNAME("skeleton_file_changed"));
	return OK;
}
#endif
#endif

#ifdef SPINE_GODOT_EXTENSION
Variant SpineSkeletonFileResourceFormatLoader::_load(const String &path, const String &original_path, bool use_sub_threads, int32_t cache_mode) {
#else
#if VERSION_MAJOR > 3
RES SpineSkeletonFileResourceFormatLoader::load(const String &path, const String &original_path, Error *error, bool use_sub_threads, float *progress, CacheMode cache_mode) {
#else
#if VERSION_MINOR > 5
RES SpineSkeletonFileResourceFormatLoader::load(const String &path, const String &original_path, Error *error, bool no_subresource_cache) {
#else
RES SpineSkeletonFileResourceFormatLoader::load(const String &path, const String &original_path, Error *error) {
#endif
#endif
#endif
	Ref<SpineSkeletonFileResource> skeleton_file = memnew(SpineSkeletonFileResource);
	Error load_error = skeleton_file->load_from_file(path);
#ifdef SPINE_GODOT_EXTENSION
	if (load_error != OK) return load_error;
#else
	if (error) *error = load_error;
	if (load_error != OK) return Ref<SpineSkeletonFileResource>();
#endif
	return skeleton_file;
}

#ifdef SPINE_GODOT_EXTENSION
PackedStringArray SpineSkeletonFileResourceFormatLoader::_get_recognized_extensions() {
	PackedStringArray extensions;
	extensions.push_back("spjson");
	extensions.push_back("spskel");
	return extensions;
}
#else
void SpineSkeletonFileResourceFormatLoader::get_recognized_extensions(List<String> *extensions) const {
	extensions->push_back("spjson");
	extensions->push_back("spskel");
}
#endif

#ifdef SPINE_GODOT_EXTENSION
String SpineSkeletonFileResourceFormatLoader::_get_resource_type(const String &path) {
#else
String SpineSkeletonFileResourceFormatLoader::get_resource_type(const String &path) const {
#endif
	return path.ends_with(".spjson") || path.ends_with(".spskel") || path.ends_with(".spine-json") || path.ends_with(".skel") ? "SpineSkeletonFileResource" : "";
}

#ifdef SPINE_GODOT_EXTENSION
bool SpineSkeletonFileResourceFormatLoader::_handles_type(const StringName &type) {
#else
bool SpineSkeletonFileResourceFormatLoader::handles_type(const String &type) const {
#endif
	return type == StringName("SpineSkeletonFileResource") || ClassDB::is_parent_class(type, "SpineSkeletonFileResource");
}

#ifdef SPINE_GODOT_EXTENSION
Error SpineSkeletonFileResourceFormatSaver::_save(const Ref<Resource> &resource, const String &path, uint32_t flags) {
#else
#if VERSION_MAJOR > 3
Error SpineSkeletonFileResourceFormatSaver::save(const RES &resource, const String &path, uint32_t flags) {
#else
Error SpineSkeletonFileResourceFormatSaver::save(const String &path, const RES &resource, uint32_t flags) {
#endif
#endif
	Ref<SpineSkeletonFileResource> res = resource;
	Error error = res->save_to_file(path);
	return error;
}

#ifdef SPINE_GODOT_EXTENSION
PackedStringArray SpineSkeletonFileResourceFormatSaver::_get_recognized_extensions(const Ref<Resource> &resource) {
	PackedStringArray extensions;
	if (Object::cast_to<SpineSkeletonFileResource>(*resource)) {
		extensions.push_back("spjson");
		extensions.push_back("spskel");
	}
	return extensions;
}
#else
void SpineSkeletonFileResourceFormatSaver::get_recognized_extensions(const RES &resource, List<String> *p_extensions) const {
	if (Object::cast_to<SpineSkeletonFileResource>(*resource)) {
		p_extensions->push_back("spjson");
		p_extensions->push_back("spskel");
	}
}
#endif

#ifdef SPINE_GODOT_EXTENSION
bool SpineSkeletonFileResourceFormatSaver::_recognize(const Ref<Resource> &resource) {
#else
bool SpineSkeletonFileResourceFormatSaver::recognize(const RES &resource) const {
#endif
	return Object::cast_to<SpineSkeletonFileResource>(*resource) != nullptr;
}
